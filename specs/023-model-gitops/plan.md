# Implementation Plan: GitOps-Managed AI Models

**Branch**: `023-model-gitops-spec` | **Date**: 2025-12-09 | **Spec**: [link](./spec.md)
**Input**: Feature specification from `/specs/023-model-gitops/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This feature introduces a Kubernetes Operator to manage the lifecycle of AI models via a declarative, GitOps-based workflow. It automates model deployment, artifact caching, and status reporting, replacing the current error-prone manual process. The technical approach involves creating a new Go-based Kubernetes Operator, a Custom Resource Definition (CRD) for `AIModel`, and a model downloader job.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: Kubernetes controller-runtime, client-go, vLLM, ArgoCD
**Storage**: S3-compatible Object Storage (for model artifacts)
**Testing**: Ginkgo/Gomega for controller tests, Testcontainers for integration tests
**Target Platform**: Kubernetes
**Project Type**: Go service (Kubernetes Operator)
**Performance Goals**: Reconcile an `AIModel` resource from creation to "Ready" state in under 15 minutes (dominated by model download time).
**Constraints**: Must integrate with existing vLLM and ArgoCD installations.
**Scale/Scope**: The operator must handle up to 100 concurrent `AIModel` resources.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Requirement | Status | Notes |
|---|---|---|---|
| API-First | OpenAPI present; UI/CLI client-only | **PASS** | The primary API is the Kubernetes API via the `AIModel` CRD. No separate REST API is needed. |
| Statelessness | No in-process state; state in Postgres/Redis/RabbitMQ | **PASS** | The operator is stateless; all state is stored in the Kubernetes etcd via the `AIModel` resource status. |
| Async Non-Critical | Analytics/logging off critical path; idempotent consumers | **PASS** | Model downloading is a long-running, asynchronous job. Reconciliation is idempotent. |
| Security | AuthN/Z, secrets handling, SAST/DAST, NetworkPolicies, TLS | **PASS** | Operator will use a ServiceAccount with RBAC permissions. HF Tokens will be stored in K8s Secrets. |
| GitOps/Declarative | Terraform/Helm/ArgoCD with Git as source of truth | **PASS** | This feature is the embodiment of this principle for model management. The operator will be deployed via Helm. |
| Observability | Health, metrics, logs, traces, dashboards defined | **PASS** | Operator will expose Prometheus metrics for reconciliation status and errors. Structured logging will be used. |
| Testing | Unit/integration/E2E coverage; no DB mocks | **PASS** | Controller tests will use a mock Kubernetes API server. Integration tests will use Testcontainers. |
| Performance | SLO adherence or profiling plan provided | **PASS** | The 15-minute deployment SLO is defined. |

## Project Structure

### Documentation (this feature)

```text
specs/023-model-gitops/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Operator Source Code
services/ai-model-operator/
├── api/v1alpha1/
│   ├── aimodel_types.go
│   └── zz_generated.deepcopy.go
├── internal/controller/
│   ├── aimodel_controller.go
│   └── aimodel_controller_test.go
├── cmd/main.go
├── Dockerfile
└── Makefile

# Model Downloader Utility
services/model-downloader/
├── main.py
└── Dockerfile

# Helm Chart for Operator
infra/helm/charts/ai-model-operator/
├── templates/
│   ├── deployment.yaml
│   └── rbac.yaml
└── Chart.yaml
```

**Structure Decision**: A new Go service, `ai-model-operator`, will be created within the existing `services/` directory. A separate utility service, `model-downloader`, will also be created in `services/` to house the artifact download logic. The operator's Helm chart will be added to the `infra/helm/charts/` directory. This structure aligns with the existing project layout for microservices.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| N/A | N/A | N/A |