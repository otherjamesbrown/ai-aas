# Impact Analysis: GPU Workload Deployment Mode Migration

**Spec**: [spec.md](./spec.md)
**Analyzed**: 2025-12-17
**Type**: Migration

## Summary

This migration replaces implicit nodeSelector-based deployment mode selection with explicit runtime-aware logic. GPU runtimes (tensorrt-llm, triton, vllm+GPU) will default to RawDeployment mode, while CPU workloads retain Knative Serverless capabilities. The change affects the ai-model-operator's InferenceServiceBuilder and CRD types.

## Impact Signals

| Signal from Spec | Search Pattern | Findings |
|------------------|----------------|----------|
| "migrate from Knative to RawDeployment" | `deploymentMode\|Serverless\|RawDeployment` | 46 files (most are docs/specs) |
| "implicit nodeSelector logic" | `nodeSelector` in operators/ | 24 matches in 4 files |
| "clean up Knative revisions" | `knative\|Knative` | 77 files |
| "InferenceServiceBuilder changes" | `InferenceServiceBuilder` | 4 files |

## Affected Components

```yaml
components:
  operators/ai-model-operator/api/v1alpha1/:
    files: 2
    risk: HIGH
    changes: [MODIFY]
    reason: CRD changes require migration strategy

  operators/ai-model-operator/internal/kserve/:
    files: 2
    risk: HIGH
    changes: [MODIFY, ADD]
    reason: Core deployment mode logic

  operators/ai-model-operator/controllers/:
    files: 2
    risk: MEDIUM
    changes: [MODIFY]
    reason: Uses InferenceServiceBuilder

  infra/k8s/knative-serving/:
    files: 4
    risk: LOW
    changes: [MODIFY]
    reason: GC config already exists, may need tuning

  docs/:
    files: 8+
    risk: LOW
    changes: [UPDATE]
    reason: Documentation needs sync
```

## Detailed Findings

### REMOVE

| File | Lines | What | Risk | Notes |
|------|-------|------|------|-------|
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 243-249 | Implicit nodeSelector → RawDeployment logic in `Build()` | MEDIUM | Replace with explicit mode parameter |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 430-435 | Implicit nodeSelector → RawDeployment logic in `BuildContainerBased()` | MEDIUM | Replace with explicit mode parameter |

### MODIFY

| File | Lines | What | Risk | Notes |
|------|-------|------|------|-------|
| `operators/ai-model-operator/api/v1alpha1/aimodel_types.go` | 28-133 | Add `DeploymentMode` field to `AIModelSpec` | HIGH | CRD change, backward compatible |
| `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go` | 24-66 | Add `DeploymentMode` field to `ModelRecipeSpec` | HIGH | CRD change, backward compatible |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 27-52 | Add `deploymentMode` field to `InferenceServiceBuilder` struct | MEDIUM | Internal change |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 207-367 | Update `Build()` to use explicit deployment mode | MEDIUM | Core logic change |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | 403-598 | Update `BuildContainerBased()` to use explicit deployment mode | MEDIUM | Core logic change |
| `operators/ai-model-operator/controllers/aimodel_controller.go` | 1081-1310 | Add `determineDeploymentMode()` function, update builder calls | MEDIUM | Reconciler logic |
| `infra/k8s/knative-serving/config-gc.yaml` | 1-30 | Review/tune GC settings for GPU cleanup | LOW | Already configured |

### ADD

| File | What | Risk | Notes |
|------|------|------|-------|
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | `WithDeploymentMode()` builder method | LOW | New fluent API method |
| `operators/ai-model-operator/internal/kserve/hpa.go` | HPA builder for RawDeployment (optional) | MEDIUM | New file, compensating control |
| `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go` | `AutoscalingSpec` struct | MEDIUM | New type for HPA config |
| `scripts/cleanup-stale-revisions.sh` | One-time cleanup script | LOW | Operational script |
| `docs/runbooks/gpu-deployment-mode.md` | New runbook | LOW | Documentation |

### UPDATE (Tests)

| File | What |
|------|------|
| `operators/ai-model-operator/internal/kserve/inferenceservice_test.go` | Add `TestDetermineDeploymentMode`, update builder tests |
| `operators/ai-model-operator/controllers/aimodel_controller_test.go` | Add tests for deployment mode selection |

### UPDATE (Human Documentation - `/docs/`)

| File | What | Audience |
|------|------|----------|
| `operators/ai-model-operator/README.md` | Document `deploymentMode` field | Developers |
| `docs/operators/ai-model-operator.md` | Add deployment mode documentation | Operators |
| `docs/operators/ai-model-operator-guide.md` | Update deployment guide | Operators |
| `docs/platform/vllm-best-practices.md` | Update with RawDeployment guidance | Platform users |
| `docs/runbooks/gpu-deployment-mode.md` | New runbook for GPU deployments | On-call engineers |

### UPDATE (Agent Context - `/context/`) - CRITICAL

**Purpose**: Loaded into AI agent context. NOT for human consumption.

| File | What | Agent |
|------|------|-------|
| `context/operator-developer/agents.md` | Add deployment mode patterns and anti-patterns | operator-developer |

**Required Context Updates**:

```yaml
# Add to context/operator-developer/agents.md patterns section:
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
  builder_method: WithDeploymentMode(mode string)
```

```go
// Add to context/operator-developer/agents.md anti-patterns section:
// WRONG: Implicit deployment mode from nodeSelector
deploymentMode := "Serverless"
if len(b.nodeSelector) > 0 {
    deploymentMode = "RawDeployment"
}

// WRONG: Not setting explicit deployment mode
builder := kserve.NewInferenceServiceBuilder().
    WithNodeSelector(nodeSelector)  // Mode inferred implicitly!

// Correct approach is documented in patterns section
```

**Context Format Rules**:
- YAML/bullets only, no prose
- Keywords: MUST, NEVER, ALWAYS
- WRONG examples only in anti-patterns
- Max 200 lines for rules type

## Migration Order

```yaml
phase_1_prepare:
  description: "Add new CRD fields (backward compatible)"
  tasks:
    - Add DeploymentMode field to AIModelSpec
    - Add DeploymentMode field to ModelRecipeSpec
    - Add AutoscalingSpec to ModelRecipeSpec (optional)
    - Run make generate && make manifests
    - Deploy updated CRDs
  risk: LOW
  rollback: "Remove new fields from types, regenerate CRDs"
  validation:
    - CRDs accept new field
    - Existing AIModels/Recipes continue working
    - No validation errors

phase_2_implement:
  description: "Implement new deployment mode logic"
  tasks:
    - Add deploymentMode field to InferenceServiceBuilder struct
    - Add WithDeploymentMode() builder method
    - Implement determineDeploymentMode() in controller
    - Update Build() to use explicit mode (keep implicit as fallback)
    - Update BuildContainerBased() to use explicit mode
    - Add HPA builder (optional)
    - Add unit tests for new logic
  risk: MEDIUM
  rollback: "Revert operator code, old logic still present"
  validation:
    - Unit tests pass
    - Local operator testing with sample AIModels
    - Explicit mode overrides work

phase_3_migrate:
  description: "Deploy and validate new behavior"
  tasks:
    - Deploy updated operator to development
    - Verify GPU models use RawDeployment
    - Verify CPU models use Serverless (if any)
    - Update existing recipes with explicit deploymentMode (optional but recommended)
    - Monitor for issues
  risk: HIGH
  rollback: "Set deploymentMode: Serverless on affected models, or revert operator"
  validation:
    - All GPU models have RawDeployment annotation
    - InferenceServices created successfully
    - Pods scheduled to GPU nodes correctly
    - Inference requests working

phase_4_documentation:
  description: "Update human docs and agent context"
  tasks:
    - Update docs/operators/ai-model-operator.md
    - Add docs/runbooks/gpu-deployment-mode.md
    - Update docs/platform/vllm-best-practices.md
    - Update context/operator-developer/agents.md (CRITICAL - agent context)
  risk: LOW
  rollback: "Revert doc changes"
  validation:
    - Human docs cover deploymentMode usage
    - Agent context has patterns and anti-patterns

phase_5_cleanup:
  description: "Remove deprecated code, clean up resources"
  tasks:
    - Remove implicit nodeSelector logic from Build()
    - Remove implicit nodeSelector logic from BuildContainerBased()
    - Run cleanup script for stale Knative revisions
  risk: LOW
  rollback: "N/A - implicit logic already unused at this point"
  validation:
    - No implicit mode selection in code
    - Stale revisions cleaned up
```

## Dependencies

```yaml
external_dependencies:
  - name: KServe
    version: v0.11+
    notes: Must support serving.kserve.io/deploymentMode annotation

  - name: Knative Serving
    version: v1.12+
    notes: Retained for CPU workloads, GC configured

internal_dependencies:
  - name: AIModel CRD
    change: MODIFY (add field)
    backward_compatible: true

  - name: ModelRecipe CRD
    change: MODIFY (add field)
    backward_compatible: true

  - name: ai-model-operator controller
    change: MODIFY (logic update)
    depends_on: [AIModel CRD, InferenceServiceBuilder]

  - name: InferenceServiceBuilder
    change: MODIFY (add method)
    depended_by: [aimodel_controller]
```

## Test Coverage

| Component | Current Tests | Needs Update |
|-----------|---------------|--------------|
| `inferenceservice_test.go` | 16 tests | Yes - add deployment mode tests, update builder tests |
| `aimodel_controller_test.go` | Exists | Yes - add determineDeploymentMode tests |
| `status_test.go` | Exists | No change needed |

### New Tests Required

```go
// inferenceservice_test.go
func TestInferenceServiceBuilder_WithDeploymentMode(t *testing.T)
func TestInferenceServiceBuilder_Build_RawDeployment(t *testing.T)
func TestInferenceServiceBuilder_Build_Serverless(t *testing.T)
func TestInferenceServiceBuilder_BuildContainerBased_RawDeployment(t *testing.T)

// aimodel_controller_test.go (or new file)
func TestDetermineDeploymentMode_TensorRTLLM(t *testing.T)
func TestDetermineDeploymentMode_Triton(t *testing.T)
func TestDetermineDeploymentMode_VLLMWithGPU(t *testing.T)
func TestDetermineDeploymentMode_VLLMWithoutGPU(t *testing.T)
func TestDetermineDeploymentMode_ExplicitOverride(t *testing.T)
```

## Rollback Plan

| Phase | Rollback Strategy | Time to Execute |
|-------|-------------------|-----------------|
| Phase 1 | Remove new CRD fields, regenerate, redeploy CRDs | 15 min |
| Phase 2 | Revert operator image to previous version | 5 min |
| Phase 3 | Set `deploymentMode: Serverless` on affected AIModels | 10 min |
| Phase 4 | Revert documentation changes | 5 min |
| Phase 5 | No rollback needed (cleanup only) | N/A |

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| CRD field breaks existing AIModels | Field is optional with no default, existing CRs unaffected |
| New logic selects wrong mode | Keep implicit fallback during Phase 2, explicit override available |
| GPU pods fail to schedule | RawDeployment allows nodeSelector (current implicit behavior preserved) |
| Knative revisions accumulate | GC already configured, cleanup script provided |
| HPA doesn't scale correctly | HPA is optional, can be disabled per-model |

## Open Questions

1. **Should HPA be created automatically?** - The spec proposes optional HPA for RawDeployment. Should it be opt-in via recipe config or automatic when min != max replicas?

2. **Explicit mode for existing recipes?** - Should we update all existing GPU recipes to have explicit `deploymentMode: RawDeployment` for clarity, even though it would be inferred?

3. **Knative retention for GPU?** - Should we keep Knative Serverless as an option for GPU workloads (with warnings), or block it entirely via validation?

## Next Step

`/speckit.plan 030-gpu-deployment-mode` - Plan will incorporate migration phases from this analysis.
