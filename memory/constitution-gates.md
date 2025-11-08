# Constitution Gates (Enforceable Checks)

All implementation plans and specs MUST explicitly satisfy these gates or document approved deviations with a remediation plan and timeline.

## API‑First
- OpenAPI spec present for all new/changed endpoints.
- UI/CLI remain client‑only (no business logic).
- OpenAI compatibility for `/v1/chat/completions` with streaming (SSE).

## Statelessness & Boundaries
- No reliance on in‑process state across requests.
- Persistent state in PostgreSQL; cache in Redis; async work via RabbitMQ.
- No shared databases across services; clear single‑responsibility per service.

## Async Non‑Critical Work
- Non‑critical operations (analytics, logging) removed from critical path.
- Queue consumers are idempotent and resilient to retries.

## Security by Default
- AuthN on every request; AuthZ via RBAC middleware on every action.
- Secrets never in Git; API keys SHA‑256; passwords bcrypt.
- NetworkPolicies enforce zero‑trust; TLS via Ingress + cert‑manager.
- Supply‑chain and static analysis in CI: CodeQL, gitleaks, trivy, hadolint, tflint, tfsec, golangci‑lint.

## Declarative Ops & GitOps
- Terraform for cloud; Helm for apps; ArgoCD for reconciliation.
- Git is source of truth; no manual applies in production.
- Hybrid mode: policies in Git, secrets/runtime via API; drift detection + restore.

## Observability
- `/health`, `/ready`, `/metrics` for every service.
- Logs (JSON), metrics (Prometheus/Mimir), traces (OpenTelemetry→Tempo).
- Required dashboards for system, org usage, model performance, infra health.

## Testing
- Unit tests (≥80% for business logic) and integration tests with Testcontainers (no DB mocks).
- E2E for primary journeys in CI; full regression nightly.
- Contract tests for public APIs; RFC7807 error semantics.

## Performance
- Inference TTFB: P50 <100ms, P95 <300ms (initial).
- Management API: P99 <500ms (initial).
- Provide profiling plan if targets are not yet met.

## Documentation & Contracts
- OpenAPI for all endpoints; schema docs for DB tables.
- Architecture docs and runbooks updated alongside changes.


