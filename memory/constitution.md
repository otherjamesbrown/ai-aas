<!--
Sync Impact Report
- Version change: Template → 1.4.0
- Modified principles:
  - New: API‑First Interfaces
  - New: Stateless Microservices & Async Non‑Critical Paths
  - New: Security by Default
  - New: Declarative Infrastructure & GitOps
  - New: Observability, Testing, and Performance SLOs
- Added sections:
  - Technology Standards & Non‑Negotiables
  - Development Workflow & Quality Gates (incl. Constitution Gates)
  - Governance (versioning, amendments, compliance review)
- Removed sections: none
- Templates requiring updates:
  - .specify/templates/plan-template.md → Constitution Check gates (⚠ pending)
  - .specify/templates/spec-template.md → Acceptance & API standards note (⚠ pending)
  - .specify/templates/tasks-template.md → Cross-cutting security/observability tasks (⚠ pending)
- Deferred TODOs:
  - RATIFICATION_DATE: confirm original adoption date
-->

# Inference-as-a-Service (IAS) Constitution

## Core Principles

### 1. API‑First Interfaces
- All functionality MUST be exposed via documented REST APIs first (OpenAPI required).
- Web UI and CLI are thin clients and MUST NOT contain business logic.
- The inference API MUST be 100% OpenAI‑compatible for `/v1/chat/completions` with streaming.
- Management APIs MUST return JSON, use standard HTTP status codes, RFC7807 error format, ISO‑8601 timestamps, and cursor‑based pagination.

Rationale: A single, well‑defined surface enables client choice, automation, testing, and governance.

### 2. Stateless Microservices & Async Non‑Critical Paths
- Services MUST be independently deployable with single responsibility.
- No shared state in service memory; persistent state in PostgreSQL, cache in Redis, queues in RabbitMQ.
- Critical path (inference) MUST be ultra‑low latency; non‑critical work (analytics, logging) MUST be asynchronous and idempotent. **When analytics pipelines reuse operational data stores (e.g., guardrail workflows that seed analytics from operational tables), this coupling MUST be documented in runbooks and automated checks MUST guard against schema drift.**
- Inter‑service communication via REST or queues; no shared databases between services.

Rationale: Statelessness and clear boundaries enable horizontal scale, resilience, and predictable latency.

### 3. Security by Default
- Every request MUST be authenticated; every action MUST be authorized via RBAC middleware.
- API keys hashed (SHA‑256); passwords bcrypt; secrets never stored in Git.
- Kubernetes NetworkPolicies enforce zero‑trust boundaries.
- TLS termination via Ingress + cert‑manager; HSTS and security headers enforced.
- Supply chain and static analysis: CodeQL, `gitleaks`, `trivy`, `hadolint`, `tflint`, `tfsec`, `golangci‑lint`.

Rationale: A secure default posture reduces blast radius and operational risk.

### 4. Declarative Infrastructure & GitOps
- Cloud infra via Terraform; apps via Helm; ArgoCD manages cluster state.
- Git is the source of truth. No manual `kubectl apply`/`terraform apply` in production.
- Hybrid GitOps:
  - Git‑managed: org policies (budgets, rate limits, allowed models, memberships), observability config, model deployments.
  - API‑managed: secrets and runtime data (API keys, logs, audit records).
- Reconciliation logs drift and restores Git‑declared state; emergency override is controlled and auditable.

Rationale: Declarative ops improve repeatability, auditability, and recovery.

### 5. Observability, Testing, and Performance SLOs
- Logs (Loki), metrics (Prometheus/Mimir), traces (OpenTelemetry→Tempo), dashboards (Grafana) are mandatory.
- Health endpoints: `/health`, `/ready`; metrics at `/metrics`.
- Testing discipline:
  - Unit tests for business logic (≥80% coverage target).
  - Integration tests with real deps via Testcontainers (no DB mocks).
  - E2E tests for primary user journeys in CI; full suite nightly.
- Performance targets (initial):
  - Inference TTFB P50 <100ms, P95 <300ms; management API P99 <500ms.

Rationale: Measurability and automated feedback ensure reliability and velocity.

## Technology Standards & Non‑Negotia​bles

- Backend: Go 1.21+, chi/gorilla; multi‑stage Docker builds; `go vet`, `staticcheck`, `golangci‑lint`.
- Datastores & Infra: PostgreSQL 15+ (TimescaleDB for analytics), Redis 7+, RabbitMQ 3.12+, Kubernetes (LKE).
- Inference: vLLM on GPU node pools; OpenAI‑compatible routing via API Router; model registry in PostgreSQL with Redis caching.
- Frontend: React 18 + TypeScript, Vite, TailwindCSS + shadcn/ui; served via Nginx.
- CLI: Go + Cobra; single binary releases via GitHub Releases.
- CI/CD: GitHub Actions → GHCR → ArgoCD auto‑sync; GitOps for vLLM, observability, and platform config.
- Ingress & TLS: Nginx Ingress + cert‑manager (Let’s Encrypt), HTTPS redirect, CORS, security headers, IP rate‑limits.
- NetworkPolicies: default deny; explicit egress to PostgreSQL/Redis/RabbitMQ/DNS; controlled egress for GitOps/HuggingFace.
- Documentation: OpenAPI for all endpoints; architecture docs; deployment/runbooks; contracts in `specs/*/contracts/`.

## Development Workflow & Quality Gates

- Code quality:
  - Go: `gofmt`/`goimports`, `golangci‑lint`, `go vet`, `staticcheck`, `govulncheck`.
  - Frontend: ESLint (strict), Prettier, TypeScript strict (no `any`).
  - Infra: `terraform fmt/validate`, `tflint`, `tfsec`; Dockerfiles with `hadolint`.
  - Security scans: `gitleaks`, `trivy`; CodeQL in CI.
- Git workflow: trunk‑based; short‑lived branches; CI‑gated PRs; at least one review; squash‑merge.
- Testing:
  - Unit, integration (Testcontainers), and E2E (happy path in CI; nightly regression).
  - No DB mocks; verify audit logging, auth failures, budget enforcement, RBAC constraints.
- API standards:
  - JSON responses; RFC7807 errors; UUID IDs; streaming SSE for inference; OpenAI‑format error semantics.

### Constitution Gates (for plans/specs)
All implementation plans MUST explicitly pass these gates:
- API‑First: OpenAPI present; UI/CLI client‑only.
- Statelessness: no in‑process state relied on; state in Postgres/Redis/RabbitMQ.
- Async Non‑Critical: analytics/logging off critical path; idempotent consumers.
- Security: authN/Z, secrets handling, SAST/DAST, NetworkPolicies, TLS/Ingress.
- GitOps/Declarative: Terraform/Helm/ArgoCD with Git as source of truth.
- Observability: health, metrics, logs, traces, dashboards defined.
- Testing: unit/integration/E2E coverage appropriate; no DB mocks.
- Performance: demonstrate SLO adherence or provide profiling plan.

## Governance

- Authority: This constitution supersedes other practices for architectural and operational mandates.
- Compliance: All PRs and reviews MUST verify constitution gates; deviations require a documented justification and time‑boxed remediation tasks.
- Amendments:
  - Propose changes via PR with rationale, impact, and migration plan.
  - Update version per semantic versioning:
    - MAJOR: incompatible rewrites of mandates.
    - MINOR: new principles/sections or significant expansions.
    - PATCH: clarifications, non‑semantic edits.
  - Record changes in `memory/CHANGELOG.md` and a dated entry in `memory/updates/`.
- Versioning & Dates:
  - Tag constitutional milestones (`constitution-vX.Y[.Z]`).
  - Keep specs aligned via targeted spec bumps and CHANGELOGs when contracts change.
- Reviews: Periodic compliance reviews ensure gates remain testable, enforced, and automated where possible.

**Version**: 1.4.0 | **Ratified**: 2025-11-06 | **Last Amended**: 2025-11-07

