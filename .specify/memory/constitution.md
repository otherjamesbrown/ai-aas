# AI-AAS Platform Constitution

## Core Principles

### I. API-First Interfaces
All functionality MUST be exposed via documented REST APIs first (OpenAPI required). Web UI and CLI are thin clients and MUST NOT contain business logic. The inference API MUST be 100% OpenAI-compatible for `/v1/chat/completions` with streaming. Management APIs MUST return JSON, use standard HTTP status codes, RFC7807 error format, ISO-8601 timestamps, and cursor-based pagination.

### II. Stateless Microservices & Async Non-Critical Paths
Services MUST be independently deployable with single responsibility. No shared state in service memory; persistent state in PostgreSQL, cache in Redis, queues in RabbitMQ. Critical path (inference) MUST be ultra-low latency; non-critical work (analytics, logging) MUST be asynchronous and idempotent. Inter-service communication via REST or queues; no shared databases between services.

### III. Security by Default
Every request MUST be authenticated; every action MUST be authorized via RBAC middleware. API keys hashed (SHA-256); passwords bcrypt; secrets never stored in Git. Kubernetes NetworkPolicies enforce zero-trust boundaries. TLS termination via Ingress + cert-manager; HSTS and security headers enforced. Supply chain and static analysis: CodeQL, gitleaks, trivy, hadolint, tflint, tfsec, golangci-lint.

### IV. Declarative Infrastructure & GitOps (NON-NEGOTIABLE)
Cloud infra via Terraform; apps via Helm; ArgoCD manages cluster state. Git is the source of truth. No manual `kubectl apply`/`terraform apply` in production. Environment profiles (`configs/environments/*.yaml`) centralize configuration management. Hybrid GitOps: Git-managed (org policies, observability config, model deployments); API-managed (secrets, runtime data).

### V. Observability, Testing & Performance SLOs
Logs (Loki), metrics (Prometheus/Mimir), traces (OpenTelemetry→Tempo), dashboards (Grafana) are mandatory. Health endpoints: `/health`, `/ready`; metrics at `/metrics`. Testing: Unit tests (≥80% coverage), integration tests with Testcontainers (no DB mocks), E2E tests for critical paths. Performance targets: Inference TTFB P50 <100ms, P95 <300ms; management API P99 <500ms.

## Technology Standards

**Backend**: Go 1.21+, chi/gorilla; multi-stage Docker builds; `go vet`, `staticcheck`, `golangci-lint`.

**Datastores & Infra**: PostgreSQL 15+ (TimescaleDB for analytics), Redis 7+, RabbitMQ 3.12+, Kubernetes (LKE).

**Inference**: vLLM on GPU node pools; OpenAI-compatible routing via API Router; model registry in PostgreSQL with Redis caching.

**Frontend**: React 18 + TypeScript, Vite, TailwindCSS + shadcn/ui; served via Nginx.

**CLI**: Go + Cobra; single binary releases via GitHub Releases.

**CI/CD**: GitHub Actions → GHCR → ArgoCD auto-sync; GitOps for vLLM, observability, and platform config.

**Logging**: All Go services MUST use `shared/go/logging` package with zap backend. Structured JSON logging with standardized fields (`service`, `environment`, `trace_id`, `request_id`, `user_id`, `org_id`).

## Development Workflow & Quality Gates

### Code Quality Requirements
- **Go**: `gofmt`/`goimports`, `golangci-lint`, `go vet`, `staticcheck`, `govulncheck`
- **Frontend**: ESLint (strict), Prettier, TypeScript strict (no `any`)
- **Infra**: `terraform fmt/validate`, `tflint`, `tfsec`; Dockerfiles with `hadolint`
- **Security scans**: `gitleaks`, `trivy`; CodeQL in CI

### Git Workflow
Trunk-based development; short-lived branches; CI-gated PRs; at least one review; squash-merge.

### Deployment Progression
Models and services progress: **Development → Staging → Production**. Never skip staging—it serves as the final validation gate. Production requires manual ArgoCD sync; development auto-syncs.

### Task Naming Convention
Format: `T-S{spec_number}-P{phase_number}-{task_number}` (e.g., `T-S006-P01-001`)

## Constitution Gates

All implementation plans MUST explicitly pass these gates:

| Gate | Requirement |
|------|-------------|
| API-First | OpenAPI present; UI/CLI client-only |
| Statelessness | No in-process state; state in Postgres/Redis/RabbitMQ |
| Async Non-Critical | Analytics/logging off critical path; idempotent consumers |
| Security | AuthN/Z, secrets handling, SAST/DAST, NetworkPolicies, TLS |
| GitOps/Declarative | Terraform/Helm/ArgoCD with Git as source of truth |
| Observability | Health, metrics, logs, traces, dashboards defined |
| Testing | Unit/integration/E2E coverage; no DB mocks |
| Performance | SLO adherence or profiling plan provided |

## Governance

- **Authority**: This constitution supersedes other practices for architectural and operational mandates.
- **Compliance**: All PRs and reviews MUST verify constitution gates; deviations require documented justification and time-boxed remediation tasks.
- **Amendments**: Propose via PR with rationale, impact, and migration plan. Record changes in `memory/CHANGELOG.md`.
- **Versioning**: Semantic versioning (MAJOR: incompatible rewrites, MINOR: new principles, PATCH: clarifications).
- **Reviews**: Periodic compliance reviews ensure gates remain testable and enforced.

**Version**: 1.4.2 | **Ratified**: 2025-11-06 | **Last Amended**: 2025-01-27
