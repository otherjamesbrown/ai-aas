# User & Organization Service

This service manages tenant identity, access, budgeting, and declarative configuration for the AI AAS platform. It exposes REST APIs for interactive administrators and service accounts, a reconciliation worker for Git-backed declarative workflows, and streams audit events for compliance.

## Architecture Overview

- **cmd/admin-api**: Serves REST endpoints for auth, org/user lifecycle, roles, budgets, policy evaluation, and audit exports.
- **cmd/reconciler**: Processes declarative updates from Git repositories, detects drift, and reconciles state.
- **internal/** packages (to be implemented):
  - `authn`, `authz`: Authentication (Fosite + MFA) and policy enforcement (OPA/Rego).
  - `budgets`, `apikeys`, `orgs`: Domain logic for budget policies, credentials, and entity lifecycle.
  - `declarative`: Git clients, diffing, reconciliation orchestration.
  - `storage`: Postgres repositories, Redis caching, Vault integrations.
  - `audit`, `telemetry`: Kafka emitters, metrics, tracing.
- **configs/**: Helm chart and Kustomize overlays (`development`, `staging`, `production`) for Kubernetes deployment.
- **contracts/**: OpenAPI definition (`specs/005-user-org-service/contracts/user-org-service.openapi.yaml`) driving contract tests.

## Local Development

```sh
# Start dependencies (Postgres, Redis, Kafka, Vault) once infrastructure tasks provide helpers.
make services/user-org-service/dev-up

# Run admin API + reconciler locally (in separate shells or via `run` target).
make -C services/user-org-service run

# Execute unit tests and linters.
make -C services/user-org-service test
make -C services/user-org-service lint

# Run policy and contract checks (placeholders until implemented).
make -C services/user-org-service opa-test
make -C services/user-org-service contract-test
```

### Database Migrations

```sh
export USER_ORG_DATABASE_URL="postgres://user:pass@localhost:5432/user_org?sslmode=disable"

make -C services/user-org-service migrate       # apply migrations
make -C services/user-org-service rollback      # roll back the latest migration
make -C services/user-org-service schema-drift  # show migration status
make -C services/user-org-service sqlc-generate # regenerate sqlc models
```

## CI Workflow

`.github/workflows/user-org-service.yml` runs on push/PR:

1. `gofmt`, `go vet`, `go test ./services/user-org-service/...`
2. `make lint` (repository-wide linters)
3. Optional OPA policy tests and OpenAPI lint (skipped until artifacts exist)
4. k6 smoke test placeholder via `scripts/service/test_new_service.sh`

## Deployment

```sh
# Helm chart
helm install user-org-service ./services/user-org-service/configs/helm -n user-org-service

# Kustomize overlays
kustomize build services/user-org-service/configs/kustomize/overlays/development | kubectl apply -f -
```

Update values and overlays with environment-specific configuration (secrets, Kafka brokers, Postgres DSNs) as later tasks implement backing services.

## References

- Spec & plan: `specs/005-user-org-service/`
- Data model: `specs/005-user-org-service/data-model.md`
- Quickstart: `specs/005-user-org-service/quickstart.md`
- Contracts: `specs/005-user-org-service/contracts/user-org-service.openapi.yaml`


