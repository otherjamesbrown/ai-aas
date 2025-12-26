# Implementation Plan: GPU Deployment Mode Migration

**Feature Branch**: `030-gpu-deployment-mode`
**Date**: 2025-12-17
**Spec**: [spec.md](./spec.md)
**Impact Analysis**: [impact.md](./impact.md)
**Type**: Migration

## Summary

Replace implicit nodeSelector-based deployment mode selection with explicit runtime-aware logic. GPU runtimes (tensorrt-llm, triton, vllm+GPU) will default to RawDeployment mode, while CPU workloads retain Knative Serverless capabilities.

## Technical Context

- **Language/Framework**: Go 1.22, Kubernetes Operator SDK
- **Dependencies**: KServe v0.11+, Knative Serving v1.12+
- **Storage**: No database changes (CRD only)
- **Testing**: Go testing, manual integration tests in development cluster

## Constitution Compliance

| Principle | Compliance |
|-----------|------------|
| API-First | CRD fields validated via OpenAPI schema |
| GitOps | All changes via git, ArgoCD syncs |
| Explicit over Implicit | Core goal of this migration |
| Backward Compatible | New fields are optional |

## Project Structure

### Files to Add

| File | What |
|------|------|
| `operators/ai-model-operator/internal/kserve/hpa.go` | HPA builder for RawDeployment |
| `scripts/cleanup-stale-revisions.sh` | One-time cleanup script |
| `docs/runbooks/gpu-deployment-mode.md` | New runbook |

### Files to Modify

| File | What |
|------|------|
| `operators/ai-model-operator/api/v1alpha1/aimodel_types.go` | Add DeploymentMode field |
| `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go` | Add DeploymentMode, AutoscalingSpec |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | Add WithDeploymentMode(), update Build() |
| `operators/ai-model-operator/controllers/aimodel_controller.go` | Add determineDeploymentMode() |
| `operators/ai-model-operator/internal/kserve/inferenceservice_test.go` | Add deployment mode tests |

### Files to Remove (Phase 5)

| File | Lines | What |
|------|-------|------|
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 243-249 | Implicit nodeSelector logic in Build() |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 430-435 | Implicit nodeSelector logic in BuildContainerBased() |

## Phases

### Phase 1: Prepare (Backward Compatible)

**Risk**: LOW | **Rollback**: Remove new fields, regenerate CRDs

| Task | File | Action |
|------|------|--------|
| T001 | `api/v1alpha1/aimodel_types.go` | Add DeploymentMode field to AIModelSpec |
| T002 | `api/v1alpha1/modelrecipe_types.go` | Add DeploymentMode field to ModelRecipeSpec |
| T003 | `api/v1alpha1/modelrecipe_types.go` | Add AutoscalingSpec struct |
| T004 | `operators/ai-model-operator/` | Run `make generate && make manifests` |
| T005 | Deploy | Apply updated CRDs to development cluster |

**Validation**:
- [ ] CRDs accept new field without errors
- [ ] Existing AIModels/Recipes continue working
- [ ] `kubectl explain aimodel.spec.deploymentMode` shows field

### Phase 2: Implement

**Risk**: MEDIUM | **Rollback**: Revert operator code

| Task | File | Action |
|------|------|--------|
| T006 | `internal/kserve/inferenceservice.go` | Add `deploymentMode` field to builder struct |
| T007 | `internal/kserve/inferenceservice.go` | Add `WithDeploymentMode()` builder method |
| T008 | `controllers/aimodel_controller.go` | Implement `determineDeploymentMode()` function |
| T009 | `internal/kserve/inferenceservice.go` | Update `Build()` to use explicit mode (keep implicit fallback) |
| T010 | `internal/kserve/inferenceservice.go` | Update `BuildContainerBased()` to use explicit mode |
| T011 | `controllers/aimodel_controller.go` | Update builder calls to use `WithDeploymentMode()` |
| T012 | `internal/kserve/hpa.go` | Add HPA builder (new file) |
| T013 | `internal/kserve/inferenceservice_test.go` | Add `TestInferenceServiceBuilder_WithDeploymentMode` |
| T014 | `internal/kserve/inferenceservice_test.go` | Add `TestInferenceServiceBuilder_Build_RawDeployment` |
| T015 | `controllers/aimodel_controller_test.go` | Add `TestDetermineDeploymentMode_*` tests |
| T016 | `operators/ai-model-operator/` | Run `go test ./...` |

**Validation**:
- [ ] All unit tests pass
- [ ] Local operator testing with sample AIModel
- [ ] Explicit mode override works

### Phase 3: Migrate

**Risk**: HIGH | **Rollback**: Set deploymentMode: Serverless on affected models

| Task | File | Action |
|------|------|--------|
| T017 | Deploy | Deploy updated operator to development cluster |
| T018 | Verify | Check GPU models use RawDeployment annotation |
| T019 | Verify | Check InferenceServices created successfully |
| T020 | Verify | Check pods scheduled to GPU nodes |
| T021 | Verify | Test inference requests work |
| T022 | ai-aas-config | Update GPU recipes with explicit `deploymentMode: RawDeployment` |

**Validation**:
- [ ] `kubectl get isvc -o yaml | grep deploymentMode` shows RawDeployment for GPU
- [ ] Pods running on GPU nodes
- [ ] Inference requests return valid responses

### Phase 4: Documentation

**Risk**: LOW | **Rollback**: Revert doc changes

| Task | File | Action |
|------|------|--------|
| T023 | `docs/operators/ai-model-operator.md` | Add deploymentMode documentation |
| T024 | `docs/runbooks/gpu-deployment-mode.md` | Create new runbook |
| T025 | `docs/platform/vllm-best-practices.md` | Update with RawDeployment guidance |
| T026 | `operators/ai-model-operator/README.md` | Document new CRD fields |
| T027 | `context/operator-developer/agents.md` | Add deployment mode patterns/anti-patterns (CRITICAL) |

**Context Update Required** (T027):
```yaml
# Add to patterns section:
deployment_mode:
  rule: Use explicit deploymentMode, not implicit nodeSelector inference
  options:
    - Serverless: Knative, scale-to-zero (CPU workloads)
    - RawDeployment: Standard K8s Deployment (GPU workloads)
  default_by_runtime:
    tensorrt-llm: RawDeployment
    triton: RawDeployment
    vllm_with_gpu: RawDeployment
    vllm_cpu: Serverless
    tgi: Serverless
```

### Phase 5: Cleanup

**Risk**: LOW | **Rollback**: N/A (implicit logic already unused)

| Task | File | Action |
|------|------|--------|
| T028 | `internal/kserve/inferenceservice.go` | Remove implicit nodeSelector logic from Build() |
| T029 | `internal/kserve/inferenceservice.go` | Remove implicit nodeSelector logic from BuildContainerBased() |
| T030 | `scripts/cleanup-stale-revisions.sh` | Create cleanup script |
| T031 | Development | Run cleanup script (dry-run first) |
| T032 | Verify | Confirm stale revisions cleaned up |

**Validation**:
- [ ] No implicit mode selection in code
- [ ] Stale revisions cleaned up
- [ ] All tests still pass

## Rollback Plan

| Phase | Strategy | Time |
|-------|----------|------|
| Phase 1 | Remove CRD fields, regenerate, redeploy | 15 min |
| Phase 2 | Revert operator image | 5 min |
| Phase 3 | Set `deploymentMode: Serverless` on models | 10 min |
| Phase 4 | Revert doc changes | 5 min |
| Phase 5 | N/A (cleanup only) | - |

## Success Criteria

1. All GPU runtimes (tensorrt-llm, triton, vllm+GPU) use RawDeployment by default
2. No Knative admission webhook rejections for GPU workloads
3. Explicit `deploymentMode` field available in CRDs
4. Unit tests cover mode selection logic
5. Documentation updated including agent context

## Dependencies

```yaml
blockers:
  - None (self-contained migration)

external:
  - KServe v0.11+ (already installed)
  - Knative Serving v1.12+ (already installed)

internal:
  - ai-aas-config repo (Phase 3 recipe updates)
```

## Research Decisions Summary

| Question | Decision | Rationale |
|----------|----------|-----------|
| HPA automatic or opt-in? | Opt-in via autoscaling config | Explicit control, avoid accidental costs |
| Update existing recipes? | Yes, add explicit deploymentMode | Clarity, matches migration goal |
| Block Serverless for GPU? | Warn but allow | Flexibility, can tighten later |
| GPU metrics for HPA? | CPU-only initially | Simpler, defer GPU metrics |
| Traffic splitting? | Not in scope | Full cutover sufficient for MVP |

See [research.md](./research.md) for full decision details.

## Next Step

`/speckit.tasks 030-gpu-deployment-mode` - Generate task beads from this plan
