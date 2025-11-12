# Quickstart: User & Organization Service

**Branch**: `005-user-org-service-upgrade`  
**Date**: 2025-11-11  
**Spec**: `/specs/005-user-org-service/spec.md`

This quickstart explains how to run the User & Organization Service locally, execute core workflows, and verify observability and declarative reconciliation behavior. It assumes repository bootstrap per `000-project-setup` and infrastructure prerequisites from `001-infrastructure`.

## Prerequisites

- Go 1.21.x, GNU Make 4.x, Docker Desktop (or container runtime compatible with Tilt)  
- Access to shared development Postgres, Redis, Vault, and Kafka instances (`make infra-dev-up` from infrastructure spec)  
- `opa` CLI ≥ 0.59, `k6` ≥ 0.46, `dredd` ≥ 14, `psql`, `redis-cli`  
- Environment variables:
  - `POSTGRES_URL`, `REDIS_URL`, `KAFKA_BROKERS`
  - `VAULT_ADDR`, `VAULT_TOKEN` (dev namespace with transit engine enabled)
  - `DECLARATIVE_REPO_URL` (dev Git repo seeded with sample config)
  - `NOTIFY_WEBHOOK_URL` (dev notification sink)
- AWS/Linode object storage credentials with write access to `identity-audit-dev` bucket (for audit exports)

## 1. Bootstrap Local Environment

```sh
make services/user-org-service/dev-up        # spins Postgres schema, Redis cache, Vault policies, Kafka topics
make services/user-org-service/migrate       # applies goose migrations
make services/user-org-service/run           # runs admin API (port 8081) and reconciler (port 8082)
```

Verify health:

```sh
curl -s http://localhost:8081/healthz | jq
curl -s http://localhost:8082/readyz | jq
```

## 2. Seed Demo Data & MFA

```sh
make services/user-org-service/seed-demo
make services/user-org-service/create-demo-mfa USER_EMAIL=admin@example.io
```

The seed script provisions:
- Organization `acme-solar`
- Admin user (`admin@example.io`, TOTP secret printed once)
- Service account `billing-exporter`
- Budget policy (warn 80%, block 100%)
- Declarative repo pointer (branch `main`)

## 3. Exercise Core Flows

### Interactive Auth

```sh
curl -X POST http://localhost:8081/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.io","password":"SuperSecure!123","totp":"123456"}'
# Response includes access token, refresh token, session_id
```

Refresh token:

```sh
curl -X POST http://localhost:8081/v1/auth/refresh \
  -H 'Authorization: Bearer <refresh_token>' \
  -d '{}'
```

### Invite & Role Assignment

```sh
curl -X POST http://localhost:8081/v1/orgs/acme-solar/invites \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"email":"new.user@example.io","roles":["OrgViewer"]}'
```

### Budget Enforcement Drill

Simulate usage above threshold:

```sh
kafkacat -b "$KAFKA_BROKERS" -t billing.usage -P <<'EOF'
{"org_id":"acme-solar","amount":1200,"period":"2025-11","timestamp":"2025-11-11T10:15:00Z"}
EOF
```

Observe alert:

```sh
curl -s http://localhost:8081/v1/budgets/acme-solar/status | jq
```

### API Key Lifecycle

```sh
curl -X POST http://localhost:8081/v1/orgs/acme-solar/service-accounts/billing-exporter/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"scopes":["billing.read"]}'
curl -X DELETE http://localhost:8081/v1/orgs/acme-solar/api-keys/<key_id> \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

## 4. Declarative Reconciliation

1. Clone declarative repo and apply change:

```sh
git clone "$DECLARATIVE_REPO_URL" /tmp/org-declarative
cd /tmp/org-declarative
cp samples/acme-solar/add-billing-admin.yaml orgs/acme-solar/roles.yaml
git commit -am "Add BillingAdmin role to automation"
git push
```

2. Watch reconciliation status:

```sh
watch -n5 curl -s http://localhost:8082/v1/reconcile/status/acme-solar | jq
```

3. Introduce drift manually, verify alert and convergence:

```sh
curl -X PATCH http://localhost:8081/v1/orgs/acme-solar/settings \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"billing_owner_user_id":null}'
```

Reconciler should report drift, revert the field, and emit audit event.

## 5. Observability Checks

- **Metrics**: `curl -s http://localhost:8081/metrics | grep identity_authorization_latency_seconds`  
- **Traces**: Use `otel-cli` or Grafana Tempo to confirm spans `identity.auth.evaluate` and `identity.reconcile.run`.  
- **Audit Stream**: Tail Kafka topic:

```sh
kafkacat -b "$KAFKA_BROKERS" -t audit.identity -C -o -5
```

- **Budget Dashboard**: Load Grafana dashboard `User Org Service / Budget Enforcement` (import JSON from `docs/observability/user-org-service-budget.json`).

## 6. Testing Workflows

```sh
make services/user-org-service/test            # go test ./...
make services/user-org-service/test-policy     # opa test ./policies
make services/user-org-service/test-contract   # dredd ./contracts/user-org-service.openapi.yaml
make services/user-org-service/test-load       # k6 run tests/load/user-org-service/authorize.js
```

CI parity validation:

```sh
make ci FEATURE=005-user-org-service-upgrade   # runs shared pipeline locally or via act
```

## 7. Audit Export & Verification

```sh
curl -X POST http://localhost:8081/v1/audit/export \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"org_id":"acme-solar","from":"2025-10-01T00:00:00Z","to":"2025-11-11T23:59:59Z"}' \
  | jq '.export_url'
```

Download export and verify signature:

```sh
curl -s "$(jq -r '.export_url' export.json)" -o export.ndjson
curl -s "$(jq -r '.signature_url' export.json)" -o export.sig
openssl dgst -sha256 -verify ops/keys/audit.pub -signature export.sig export.ndjson
```

## 8. Cleanup

```sh
make services/user-org-service/dev-down        # stops services, cleans containers
```

Audit exports remain in object storage for compliance; delete manually if using personal sandbox credentials.

