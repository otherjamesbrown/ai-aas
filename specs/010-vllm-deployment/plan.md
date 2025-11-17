# Implementation Plan: Model Inference Deployment

**Branch**: `010-vllm-deployment` | **Date**: 2025-01-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/010-vllm-deployment/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Deploy production-ready vLLM inference engines on GPU nodes with health/status visibility, predictable endpoints, and model registration for routing. Implement safe rollout/rollback practices with environment separation. The solution uses Helm charts for declarative deployment, ArgoCD for GitOps reconciliation, and extends the existing model registry for routing configuration.

## Technical Context

**Language/Version**: Python 3.11+ (vLLM runtime), Go 1.21+ (management tooling), YAML (Helm charts)  
**Primary Dependencies**: 
- vLLM (Python inference engine)
- Kubernetes (LKE cluster)
- Helm 3.x (deployment charts)
- ArgoCD (GitOps reconciliation)
- PostgreSQL (model registry, existing `model_registry_entries` table)
- Redis (health check caching, optional)

**Storage**: 
- PostgreSQL: Model deployment metadata, routing configuration (extends `model_registry_entries`)
- Kubernetes: Deployment state, ConfigMaps, Secrets
- Git: Helm chart definitions (source of truth)

**Testing**: 
- Helm chart linting (`helm lint`)
- Kubernetes manifest validation (`kubectl apply --dry-run`)
- Integration tests: Deploy test model, verify readiness, test completion endpoint
- E2E tests: Full deployment → registration → routing flow

**Target Platform**: 
- Kubernetes (LKE) on GPU node pool (`g1-gpu-rtx6000`)
- Linux containers (vLLM Python runtime)
- Namespace: `system` (GPU workloads) or environment-specific namespaces

**Project Type**: Infrastructure/DevOps (Helm charts, Kubernetes manifests, management APIs)

**Performance Goals**: 
- Deployment readiness: ≤10 minutes (95th percentile) per SC-001
- Test completion response: ≤3 seconds (95th percentile) per SC-002
- Registration propagation: ≤2 minutes (95th percentile) per SC-003
- Rollback completion: ≤5 minutes per SC-004

**Constraints**: 
- GPU node pool capacity (2 nodes initially)
- Model initialization time (large models may take 5-10 minutes)
- Network policies must allow API Router → vLLM pod communication
- Environment separation (development/staging/production namespaces)

**Scale/Scope**: 
- Initial: 2-5 model deployments per environment
- Target: Support 10+ concurrent model deployments
- Model sizes: 7B to 70B parameters (GPU memory constraints)
- Concurrent inference requests: 10-100 per model instance

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### API-First Interfaces ✓
- **Status**: PASS
- **Evidence**: Management APIs for deployment/registration will be defined in OpenAPI specs (`specs/010-vllm-deployment/contracts/`)
- **Inference API**: vLLM exposes OpenAI-compatible `/v1/chat/completions` endpoint (constitution requirement met)
- **UI/CLI**: Admin CLI and web UI will be thin clients calling management APIs (no business logic)

### Stateless Microservices & Boundaries ✓
- **Status**: PASS
- **Evidence**: 
  - vLLM pods are stateless (model loaded in memory, no persistent state)
  - Deployment metadata in PostgreSQL (`model_registry_entries` table)
  - Health check caching in Redis (optional, non-critical)
  - No shared databases; each service maintains its own state

### Async Non-Critical Work ✓
- **Status**: PASS
- **Evidence**: 
  - Deployment status updates can be async (non-blocking)
  - Health check aggregation off critical inference path
  - Analytics/usage tracking already async via RabbitMQ (existing pattern)

### Security by Default ✓
- **Status**: PASS
- **Evidence**: 
  - NetworkPolicies required for API Router → vLLM pod communication
  - TLS via Ingress (existing pattern)
  - Secrets in Kubernetes Secrets (not Git)
  - RBAC for deployment/registration operations (via existing middleware)

### Declarative Infrastructure & GitOps ✓
- **Status**: PASS
- **Evidence**: 
  - Helm charts in Git (`infra/helm/charts/vllm-deployment/` or similar)
  - ArgoCD reconciliation (existing GitOps pattern)
  - Environment profiles for configuration (`configs/environments/*.yaml`)
  - No manual `kubectl apply` in production

### Observability ✓
- **Status**: PASS
- **Evidence**: 
  - `/health` and `/ready` endpoints (vLLM provides these)
  - `/metrics` endpoint (Prometheus metrics from vLLM)
  - Logs via existing Loki integration
  - Dashboards for model deployment status (Grafana)

### Testing ✓
- **Status**: PASS
- **Evidence**: 
  - Integration tests with Testcontainers for PostgreSQL/Redis
  - E2E tests: Deploy → Register → Route → Verify
  - Helm chart validation (`helm lint`, `kubectl apply --dry-run`)
  - No DB mocks (use real dependencies)

### Performance ✓
- **Status**: PASS
- **Evidence**: 
  - Success criteria define measurable targets (SC-001 through SC-004)
  - Inference TTFB targets align with constitution (P50 <100ms, P95 <300ms)
  - Management API targets: Registration ≤2 minutes (well under P99 <500ms)

### Documentation & Contracts ✓
- **Status**: PASS
- **Evidence**: 
  - OpenAPI specs for management APIs (Phase 1 deliverable)
  - Helm chart documentation
  - Runbooks for deployment/rollback procedures
  - Architecture docs updated

**Overall Gate Status**: ✅ ALL GATES PASS

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
infra/
├── helm/
│   └── charts/
│       └── vllm-deployment/
│           ├── Chart.yaml
│           ├── values.yaml
│           ├── values-development.yaml
│           ├── values-staging.yaml
│           ├── values-production.yaml
│           └── templates/
│               ├── deployment.yaml
│               ├── service.yaml
│               ├── configmap.yaml
│               ├── networkpolicy.yaml
│               ├── servicemonitor.yaml
│               └── ingress.yaml (optional, for direct access)

services/
└── model-deployment-service/ (optional, for management APIs)
    ├── cmd/
    │   └── model-deployment-service/
    │       └── main.go
    ├── internal/
    │   ├── api/
    │   │   └── handler.go
    │   ├── deployment/
    │   │   ├── helm.go
    │   │   └── kubernetes.go
    │   ├── registry/
    │   │   └── repository.go
    │   └── health/
    │       └── checker.go
    ├── deployments/
    │   └── helm/
    │       └── model-deployment-service/
    └── test/
        └── integration/

db/
└── migrations/
    └── operational/
        └── YYYYMMDDHHMMSS_add_deployment_metadata.up.sql
```

**Structure Decision**: 
- **Helm Charts**: Primary deliverable for vLLM deployment (`infra/helm/charts/vllm-deployment/`)
- **Optional Management Service**: If management APIs are needed beyond admin-cli, create `services/model-deployment-service/` following existing service patterns
- **Database Migrations**: Extend `model_registry_entries` table with deployment metadata (endpoint URLs, health status, environment)
- **Testing**: Integration tests in service test directory, E2E tests in `test/e2e/` or service-specific test directories

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations - all gates passed. No complexity justification needed.

---

## Phase 0 & Phase 1 Complete

**Phase 0 (Research)**: ✅ Complete
- `research.md` generated with all technical decisions and alternatives

**Phase 1 (Design)**: ✅ Complete
- `data-model.md` generated with entity definitions and schema changes
- `contracts/model-deployment-api.yaml` generated with OpenAPI specification
- `quickstart.md` generated with deployment and operations guide
- Agent context updated with new technology (Python 3.11+, vLLM, Helm)

**Next Steps**: 
- Phase 2: Run `/speckit.tasks` to generate implementation task breakdown
- Implementation: Follow tasks in order, starting with database migrations and Helm chart creation
