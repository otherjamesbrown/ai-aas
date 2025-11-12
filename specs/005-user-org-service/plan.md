# Implementation Plan: User & Organization Service

**Branch**: `005-user-org-service-upgrade` | **Date**: 2025-11-11 | **Spec**: `/specs/005-user-org-service/spec.md`  
**Input**: Feature specification from `/specs/005-user-org-service/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Deliver a Go-based User & Organization Service that authenticates interactive users and automation, enforces role- and budget-driven authorization, and reconciles declarative org configuration with drift detection. The service issues and revokes credentials, streams tamper-evident audit logs, exposes PDP/PEP contracts for downstream services, and provides Git-backed declarative reconciliation that converges target state within the defined SLOs.

## Technical Context

**Language/Version**: Go 1.21, GNU Make 4.x, OpenAPI 3.1 contracts  
**Primary Dependencies**: `go-chi/chi` HTTP router, `ory/fosite` for OAuth2/OIDC flows, `go-oidc` for SSO federation, `ory/ladon` (policy engine) backed by Rego policies, `open-policy-agent/opa` SDK for PDP evaluations, `github.com/rs/zerolog` for structured logging, OpenTelemetry Go SDK, `go-git` for declarative repo ingestion, `segmentio/kafka-go` for audit/event fan-out  
**Storage**: PostgreSQL (tenant-aware relational schema), Redis (session/token cache), Kafka topic (`audit.identity`) for immutable event stream, S3-compatible object storage for audit exports  
**Testing**: `go test ./...`, contract tests with `dredd` against exported OpenAPI, load/regression testing via `k6`, policy regression harness using `conftest`/`opa test`  
**Target Platform**: Linode LKE Kubernetes clusters (dev/staging/prod) managed by spec `001-infrastructure`; service runs behind platform ingress with mTLS between services  
**Project Type**: Backend microservice with background reconciliation worker and admin API  
**Performance Goals**: Authorization checks ≤50ms P95, API key issuance/revocation ≤1s P95, reconciliation of declarative commits ≤2 minutes for 95% of commits, audit export of 30-day window ≤2 minutes  
**Constraints**: Must emit structured logs/traces per constitution, enforce MFA for OrgOwner role, operate via GitOps (no manual state changes), preserve 400-day audit retention, support hard-stop budget enforcement with dual-approval overrides, zero plaintext secrets at rest  
**Scale/Scope**: Target 20k organizations, 200k interactive users, 500k service accounts/API keys, 10k authorization checks/minute sustained with burst handling up to 50k/minute

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **API-First & Contract Driven**: Plan includes OpenAPI 3.1 contracts for admin/auth/PDP endpoints plus policy evaluation harness; downstream services integrate via documented interfaces.  
- **Security by Default**: MFA enforcement, encrypted secrets via KMS, tamper-evident audit pipeline, OPA-based policy evaluation, and rate limiting satisfy security gate.  
- **Observability**: OpenTelemetry traces, Prometheus metrics, and Kafka audit stream cover metrics/logs/traces requirements with dashboards defined in quickstart deliverables.  
- **GitOps & Declarative Operations**: Declarative reconciliation via Git-managed config and immutable audit ensures compliance with GitOps gate; all mutable state changes flow through APIs or reconciler.  
- **Testing & Quality**: Unit tests, policy regression (`opa test`), contract tests (`dredd`), and load tests (`k6`) provide automated coverage across primary flows and exception handling.  
- **Performance & Resiliency**: Defined SLOs for auth latency, reconciliation throughput, and failover behavior align with constitution performance expectations.  
No waivers required.

## Project Structure

### Documentation (this feature)

```text
specs/005-user-org-service/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── user-org-service.openapi.yaml
└── tasks.md            # generated later via /speckit.tasks
```

### Source Code (repository root)

```text
services/user-org-service/
├── cmd/
│   ├── admin-api/              # REST/OIDC entrypoint
│   └── reconciler/             # Git declarative reconciler
├── internal/
│   ├── authn/                  # interactive auth, MFA, session issuing
│   ├── authz/                  # OPA integration, policy evaluation cache
│   ├── budgets/                # budget policy enforcement + overrides
│   ├── declarative/            # git client, diffing, reconciliation workflows
│   ├── orgs/                   # org/user/service-account lifecycle logic
│   ├── apikeys/                # key issuance, rotation, fingerprinting
│   ├── audit/                  # structured logging, Kafka producers
│   ├── storage/                # Postgres repositories, Redis cache adapters
│   └── telemetry/              # metrics, tracing
├── migrations/                 # goose/sqlc migrations and seed data
├── pkg/
│   ├── api/                    # shared API models + validation helpers
│   └── tokens/                 # JWT/JWS helper utilities
├── policies/                   # Rego policies, tests, and bundles
├── contracts/                  # generated OpenAPI + JSON schemas
├── configs/
│   ├── helm/                   # Helm chart for deployment
│   └── kustomize/              # overlays per environment
├── Makefile                    # build/test/lint targets
└── tools/
    ├── opa/                    # policy regression harness
    └── scripts/                # maintenance, drift triage, audit export helpers

tests/
├── contract/user-org-service/  # Dredd/Postman suites
├── integration/user-org-service/
├── policy/                     # opa test suites
└── load/user-org-service/      # k6 scenarios
```

**Structure Decision**: Implement as a single Go microservice with distinct binaries for admin API and declarative reconciler, sharing internal packages. Policies, migrations, and Helm/kustomize manifests stay co-located under the service directory to keep GitOps alignment. Tests mirror spec expectations with contract, integration, policy, and load harnesses under `tests/`.

## Complexity Tracking

No constitution violations introduced; the split binaries and policy submodule exist to satisfy separation of concerns (API vs reconciliation) and policy testing requirements without exceeding governance thresholds.
