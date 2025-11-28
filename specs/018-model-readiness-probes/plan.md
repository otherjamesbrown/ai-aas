# Implementation Plan: Model Readiness Probes for KServe InferenceServices

**Branch**: `018-model-readiness-probes` | **Date**: 2025-11-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/018-model-readiness-probes/spec.md`

## Summary

This feature implements HTTP-based health probes (startup, readiness, and liveness) for KServe InferenceService deployments to ensure pods only receive traffic after vLLM models are fully loaded into GPU memory, eliminating timeout errors during autoscaling and cold starts. The implementation uses vLLM's native `/health` endpoint with model-size-specific timeout configurations.

## Technical Context

**Language/Version**: YAML (Kubernetes manifests), KServe v0.11+, Knative Serving v1.x  
**Primary Dependencies**: KServe, Knative Serving, vLLM (v0.6.x - v0.10.x), Kubernetes 1.28+  
**Storage**: N/A (declarative configuration only)  
**Testing**: kubectl validation, GitOps deployment verification, manual E2E testing  
**Target Platform**: Kubernetes cluster (Linode LKE) with GPU nodes  
**Project Type**: Infrastructure/Configuration  
**Performance Goals**: Zero `context deadline exceeded` errors during autoscaling, <5s first request latency with warm replicas  
**Constraints**: Startup probe timeout must accommodate 95th percentile model loading times (15 min for 20B, 30 min for 70B)  
**Scale/Scope**: 3-4 InferenceService deployments (gpt-oss-20b, mistral-7b-instruct, llama-2-7b)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The project constitution is not yet populated with specific gates. General principles apply:

| Principle | Status | Notes |
|-----------|--------|-------|
| Spec-Driven Development | ✅ PASS | Spec exists at `specs/018-model-readiness-probes/spec.md` |
| GitOps Deployment | ✅ PASS | All changes via ArgoCD, manifests in `infra/k8s/kserve/` |
| Test Before Production | ✅ PASS | Deploy to development first, validate before staging/production |
| Documentation Required | ✅ PASS | Runbooks and best practices to be updated |

**Gate Status**: ✅ PASS - Proceeding to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/018-model-readiness-probes/
├── plan.md              # This file
├── research.md          # Phase 0 output - probe configuration research
├── data-model.md        # Phase 1 output - N/A (configuration only)
├── quickstart.md        # Phase 1 output - probe configuration guide
├── contracts/           # Phase 1 output - probe configuration templates
│   └── probe-config-templates.yaml
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
infra/k8s/kserve/
├── templates/
│   └── inference-service-vllm-template.yaml  # Probe configuration template
├── models/
│   ├── gpt-oss-20b.yaml           # 20B model InferenceService
│   ├── mistral-7b-instruct.yaml   # 7B model InferenceService
│   └── llama-2-7b.yaml            # 7B model InferenceService
└── base/
    └── inferenceservice-config.yaml

docs/
├── runbooks/
│   └── enable-model-readiness-probes.md  # New runbook (to create)
├── best-practices/
│   └── vllm-deployment-best-practices.md  # Update with probe guidance
└── model-initialization.md               # Existing probe documentation
```

**Structure Decision**: Infrastructure/configuration feature. All changes are to existing YAML manifests and documentation. No new service code required.

## Implementation Status

### Current State Analysis

**IMPORTANT**: Probes are already implemented in all InferenceService manifests:

| Manifest | Startup Probe | Readiness Probe | Liveness Probe | Status |
|----------|--------------|-----------------|----------------|--------|
| gpt-oss-20b.yaml | ✅ Configured (90×10s=15min) | ✅ Configured | ✅ Configured | Complete |
| mistral-7b-instruct.yaml | ✅ Configured (36×10s=6min) | ✅ Configured | ✅ Configured | Complete |
| llama-2-7b.yaml | ✅ Configured (36×10s=6min) | ✅ Configured | ✅ Configured | Complete |
| Template | ✅ Configured (placeholder) | ✅ Configured | ✅ Configured | Complete |

### Remaining Work

1. **Validation**: Confirm probes are working correctly in deployed environments
2. **Documentation**: Create dedicated runbook for probe management
3. **Testing**: Verify behavior during autoscaling events
4. **Monitoring**: Ensure Grafana dashboards reflect probe status

## Complexity Tracking

No violations identified. This is a configuration-only feature with:
- No new services or code
- No database changes
- No API changes
- Minimal risk (additive configuration to existing manifests)
