# Implementation Plan: KServe Migration

**Branch**: `016-kserve-migration` | **Date**: 2025-11-24 | **Spec**: [specs/016-kserve-migration/spec.md](file:///Users/james/Documents/GitHub/otherjamesbrown/ai-aas/specs/016-kserve-migration/spec.md)
**Input**: Feature specification from `/specs/016-kserve-migration/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Migrate the AI-AAS platform from custom vLLM Helm chart deployments to a standardized KServe-based architecture. This involves deploying KServe/Istio/Knative infrastructure, implementing an "OpenAI-to-KServe Adapter" in the API Router, updating the Model Registry schema for hybrid routing, and verifying autoscaling capabilities using the Load Testing Harness.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.21+ (Services), YAML/Kustomize (Infrastructure)
**Primary Dependencies**: 
- **Infrastructure**: KServe v0.11+, Istio v1.19+, Knative Serving v1.11+
- **Services**: `net/http` (Go stdlib), internal `shared` libraries
**Storage**: PostgreSQL (Model Registry), S3/MinIO (Model Artifacts)
**Testing**: Go `testing` package, `specs/014-load-testing-harness` (Go)
**Target Platform**: Kubernetes (GKE/EKS/Kind) with GPU nodes
**Project Type**: Microservices + Infrastructure
**Performance Goals**: <5s latency for warm inference, <30s cold start, <5ms routing overhead
**Constraints**: Must maintain backward compatibility with OpenAI API clients.
**Scale/Scope**: Support 10-50 concurrent models, scale-to-zero for idle models.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

1.  **Architecture**: Does this align with the microservices architecture? **YES** (Decouples serving from routing).
2.  **Immutability**: Does this respect the immutability of `specs/006` and `specs/010`? **YES** (Uses adapter pattern and new spec for migration).
3.  **Observability**: Does this maintain or improve observability? **YES** (Adds KServe/Istio metrics mapped to standard dashboards).
4.  **Testing**: Is there a verification plan? **YES** (Uses Load Testing Harness).

## Project Structure

### Documentation (this feature)

```text
specs/016-kserve-migration/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
infra/
├── kserve/                  # [NEW] KServe infrastructure manifests
│   ├── base/
│   └── overlays/
├── helm/
│   └── charts/
│       └── kserve-inference/ # [NEW] Helm chart for InferenceService

services/
├── api-router-service/
│   └── internal/
│       └── adapter/         # [NEW] OpenAI-to-KServe protocol adapter
├── admin-cli/
│   └── cmd/
│       └── model/           # [UPDATE] KServe management commands

specs/
└── 016-kserve-migration/    # [EXISTING] Feature specs and plans
```

**Structure Decision**: 
- **Infrastructure**: New `infra/kserve` directory for the control plane setup and a new Helm chart `kserve-inference` for deploying individual models (wrapping the CRD).
- **Services**: `api-router-service` gets a new `adapter` package to handle the protocol translation cleanly. `admin-cli` gets updated commands to manage the new resource types.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |
