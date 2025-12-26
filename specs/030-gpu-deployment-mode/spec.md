# Spec 030: GPU Workload Deployment Mode Migration

## Overview

Migrate GPU-based ML model deployments from KServe Serverless mode (Knative) to RawDeployment mode (standard Kubernetes Deployments) to resolve fundamental incompatibilities between Knative Serving and GPU workloads.

## Problem Statement

The current platform uses KServe with Knative Serving for model deployments. While this works well for CPU-based inference, GPU workloads encounter critical issues:

1. **Knative rejects nodeSelector** - GPU workloads require scheduling to specific GPU nodes, but Knative's admission webhook rejects `nodeSelector` in pod specs
2. **Single-port container restriction** - Knative requires exactly one container port, but inference servers like Triton need multiple (HTTP, gRPC, metrics)
3. **Revision accumulation** - Knative creates immutable revisions that hold GPU resources, causing resource exhaustion
4. **Expensive cold starts** - Scale-to-zero is counterproductive when model loading takes 5-10 minutes

### Current Workaround

The operator has implicit logic to use RawDeployment when `nodeSelector` is present:

```go
// operators/ai-model-operator/internal/kserve/inferenceservice.go:243-248
deploymentMode := "Serverless"
if len(b.nodeSelector) > 0 {
    deploymentMode = "RawDeployment"
}
```

This workaround is:
- **Implicit** - behavior depends on nodeSelector presence, not intent
- **Undocumented** - operators don't know why mode switches
- **Inconsistent** - same recipe can behave differently

## Goals

1. Make deployment mode selection **explicit and runtime-aware**
2. Default GPU runtimes to RawDeployment mode
3. Preserve Knative/Serverless for CPU workloads where beneficial
4. Implement compensating controls for lost Knative capabilities
5. Clean up legacy Knative revisions holding GPU resources

## Non-Goals

- Removing Knative from the cluster entirely (still useful for CPU workloads)
- Implementing custom Knative modifications
- Building a custom autoscaler (future work)

---

## Background

### KServe Deployment Modes

KServe supports two deployment modes via the `serving.kserve.io/deploymentMode` annotation:

| Mode | Underlying Resources | Autoscaling | Scale-to-Zero |
|------|---------------------|-------------|---------------|
| `Serverless` | Knative Service → Knative Revision → Pod | Knative Pod Autoscaler (KPA) | Yes |
| `RawDeployment` | Deployment + Service → Pod | Kubernetes HPA (manual) | No |

### Why Knative Was Chosen Initially

Knative Serving provides valuable capabilities for serverless workloads:

| Capability | Description | ML Use Case |
|------------|-------------|-------------|
| Scale-to-zero | Terminate pods when idle | Save costs during off-hours |
| Autoscaling | Scale based on concurrency/RPS | Handle traffic spikes |
| Revision management | Immutable deployment snapshots | Model version rollbacks |
| Traffic splitting | Route traffic % to revisions | A/B test models |
| Request buffering | Queue during scale-up | Handle cold start latency |

### Why Knative Fails for GPUs

| Knative Assumption | GPU Reality | Impact |
|--------------------|-------------|--------|
| Fast cold starts (~seconds) | Slow starts (~5-10 minutes) | Timeouts, poor UX |
| Stateless containers | Stateful (model in VRAM) | Can't freely restart |
| Single port | Multiple ports needed | Triton incompatible |
| Generic scheduling | Requires nodeSelector | Admission rejection |
| Cheap scaling | Expensive GPU allocation | Scale-to-zero wastes time |

---

## Architecture

### Current State

```
┌─────────────┐    ┌─────────────────┐    ┌──────────────────┐    ┌─────────┐
│ API Router  │───▶│ KServe          │───▶│ Knative Service  │───▶│ Pod     │
└─────────────┘    │ InferenceService│    │ + Revision       │    │ (GPU)   │
                   └─────────────────┘    └──────────────────┘    └─────────┘
                                                   │
                                          Issue: nodeSelector rejected
                                          Issue: single port only
                                          Issue: revisions accumulate
```

### Target State

```
GPU Workloads (tensorrt-llm, triton, vllm+GPU):
┌─────────────┐    ┌─────────────────┐    ┌──────────────────┐    ┌─────────┐
│ API Router  │───▶│ KServe          │───▶│ K8s Deployment   │───▶│ Pod     │
└─────────────┘    │ InferenceService│    │ + Service        │    │ (GPU)   │
                   │ (RawDeployment) │    └──────────────────┘    └─────────┘
                   └─────────────────┘             │
                                          ┌────────┴────────┐
                                          │ HPA (optional)  │
                                          └─────────────────┘

CPU Workloads (embeddings, classifiers):
┌─────────────┐    ┌─────────────────┐    ┌──────────────────┐    ┌─────────┐
│ API Router  │───▶│ KServe          │───▶│ Knative Service  │───▶│ Pod     │
└─────────────┘    │ InferenceService│    │ (scale-to-zero)  │    │ (CPU)   │
                   │ (Serverless)    │    └──────────────────┘    └─────────┘
                   └─────────────────┘
```

### Deployment Mode Decision Logic

```go
func determineDeploymentMode(recipe *ModelRecipeSpec) string {
    // Explicit override in recipe takes precedence
    if recipe.DeploymentMode != "" {
        return recipe.DeploymentMode
    }

    // GPU runtimes default to RawDeployment
    switch recipe.Runtime {
    case "tensorrt-llm", "triton":
        return "RawDeployment"
    case "vllm", "tgi":
        if recipe.Resources.GPU.Count > 0 {
            return "RawDeployment"
        }
        return "Serverless"
    default:
        return "Serverless"
    }
}
```

---

## Implementation

### Phase 1: Schema Changes

#### 1.1 Add DeploymentMode to ModelRecipeSpec

**File**: `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`

```go
type ModelRecipeSpec struct {
    // ... existing fields ...

    // DeploymentMode specifies how the model should be deployed.
    // Options: "Serverless" (Knative), "RawDeployment" (standard K8s)
    // If not specified, determined automatically based on runtime and GPU requirements.
    // +kubebuilder:validation:Enum=Serverless;RawDeployment
    // +optional
    DeploymentMode string `json:"deploymentMode,omitempty"`
}
```

#### 1.2 Add DeploymentMode to AIModelSpec

**File**: `operators/ai-model-operator/api/v1alpha1/aimodel_types.go`

```go
type AIModelSpec struct {
    // ... existing fields ...

    // DeploymentMode override. If set, takes precedence over recipe.
    // +kubebuilder:validation:Enum=Serverless;RawDeployment
    // +optional
    DeploymentMode string `json:"deploymentMode,omitempty"`
}
```

#### 1.3 Regenerate CRDs

```bash
cd operators/ai-model-operator
make generate
make manifests
```

### Phase 2: Operator Changes

#### 2.1 Implement Deployment Mode Selection

**File**: `operators/ai-model-operator/controllers/aimodel_controller.go`

Add new function for deployment mode determination:

```go
func (r *AIModelReconciler) determineDeploymentMode(
    aimodel *aiv1alpha1.AIModel,
    recipe *aiv1alpha1.ModelRecipeSpec,
) string {
    // 1. AIModel explicit override
    if aimodel.Spec.DeploymentMode != "" {
        return aimodel.Spec.DeploymentMode
    }

    // 2. Recipe explicit setting
    if recipe != nil && recipe.DeploymentMode != "" {
        return recipe.DeploymentMode
    }

    // 3. Runtime-based defaults
    runtime := aimodel.Spec.Runtime
    if recipe != nil {
        runtime = recipe.Runtime
    }

    switch runtime {
    case "tensorrt-llm", "triton":
        return "RawDeployment"
    case "vllm", "tgi":
        // Check if GPU is requested
        gpuCount := int32(0)
        if recipe != nil && recipe.Resources.GPU.Count > 0 {
            gpuCount = recipe.Resources.GPU.Count
        }
        if aimodel.Spec.Resources.GPU != "" {
            // Parse GPU count from aimodel spec
            gpuCount = parseGPUCount(aimodel.Spec.Resources.GPU)
        }
        if gpuCount > 0 {
            return "RawDeployment"
        }
        return "Serverless"
    default:
        return "Serverless"
    }
}
```

#### 2.2 Update InferenceServiceBuilder

**File**: `operators/ai-model-operator/internal/kserve/inferenceservice.go`

Replace implicit nodeSelector check with explicit mode:

```go
type InferenceServiceBuilder struct {
    // ... existing fields ...
    deploymentMode string  // NEW: explicit deployment mode
}

func (b *InferenceServiceBuilder) WithDeploymentMode(mode string) *InferenceServiceBuilder {
    b.deploymentMode = mode
    return b
}

func (b *InferenceServiceBuilder) Build() *servingv1beta1.InferenceService {
    // Use explicit mode instead of inferring from nodeSelector
    mode := b.deploymentMode
    if mode == "" {
        mode = "Serverless"  // Default for backward compatibility
    }

    annotations := map[string]string{
        "serving.kserve.io/deploymentMode": mode,
    }
    // ... rest of build logic
}
```

#### 2.3 Update Controller to Use New Logic

**File**: `operators/ai-model-operator/controllers/aimodel_controller.go`

In the reconciliation loop:

```go
// Determine deployment mode
deploymentMode := r.determineDeploymentMode(aimodel, recipeSpec)

// Log the decision for observability
log.Info("Deployment mode determined",
    "mode", deploymentMode,
    "runtime", runtime,
    "hasGPU", gpuCount > 0,
    "reason", deploymentModeReason)

// Build InferenceService with explicit mode
isvcBuilder := kserve.NewInferenceServiceBuilder().
    WithName(aimodel.Name).
    WithNamespace(aimodel.Namespace).
    WithDeploymentMode(deploymentMode).  // NEW
    // ... other builder calls
```

### Phase 3: Compensating Controls

#### 3.1 Horizontal Pod Autoscaler for GPU Workloads

For RawDeployment mode, create optional HPA support:

**File**: `operators/ai-model-operator/internal/kserve/hpa.go` (new)

```go
func (b *InferenceServiceBuilder) BuildHPA() *autoscalingv2.HorizontalPodAutoscaler {
    if b.deploymentMode != "RawDeployment" || b.minReplicas == b.maxReplicas {
        return nil  // No HPA needed
    }

    return &autoscalingv2.HorizontalPodAutoscaler{
        ObjectMeta: metav1.ObjectMeta{
            Name:      b.name + "-hpa",
            Namespace: b.namespace,
        },
        Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
            ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
                APIVersion: "apps/v1",
                Kind:       "Deployment",
                Name:       b.name + "-predictor",
            },
            MinReplicas: &b.minReplicas,
            MaxReplicas: b.maxReplicas,
            Metrics: []autoscalingv2.MetricSpec{
                {
                    Type: autoscalingv2.ResourceMetricSourceType,
                    Resource: &autoscalingv2.ResourceMetricSource{
                        Name: corev1.ResourceCPU,
                        Target: autoscalingv2.MetricTarget{
                            Type:               autoscalingv2.UtilizationMetricValueType,
                            AverageUtilization: pointer.Int32(70),
                        },
                    },
                },
            },
        },
    }
}
```

#### 3.2 Add Autoscaling Config to Recipe

**File**: `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`

```go
type AutoscalingSpec struct {
    // MinReplicas is the minimum number of replicas
    // +kubebuilder:default=1
    MinReplicas int32 `json:"minReplicas,omitempty"`

    // MaxReplicas is the maximum number of replicas
    // +kubebuilder:default=1
    MaxReplicas int32 `json:"maxReplicas,omitempty"`

    // TargetCPUUtilization is the target CPU utilization for HPA
    // +kubebuilder:default=70
    TargetCPUUtilization int32 `json:"targetCPUUtilization,omitempty"`

    // ScaleDownStabilization is the stabilization window for scale down (seconds)
    // +kubebuilder:default=300
    ScaleDownStabilization int32 `json:"scaleDownStabilization,omitempty"`
}

type ModelRecipeSpec struct {
    // ... existing fields ...

    // Autoscaling configuration for RawDeployment mode
    // +optional
    Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
}
```

### Phase 4: Knative Revision Cleanup

#### 4.1 Configure Knative GC

**File**: `infra/k8s/knative/config-gc.yaml` (new)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-gc
  namespace: knative-serving
data:
  # Aggressively clean up old revisions
  retain-since-create-time: "1h"
  retain-since-last-active-time: "1h"
  min-non-active-revisions: "0"
  max-non-active-revisions: "1"
```

#### 4.2 One-time Cleanup Script

**File**: `scripts/cleanup-stale-revisions.sh` (new)

```bash
#!/bin/bash
# Cleanup stale Knative revisions that are holding GPU resources

set -euo pipefail

KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
DRY_RUN="${DRY_RUN:-true}"

echo "Finding stale revisions..."

# Get all revisions that are not receiving traffic
kubectl get revisions -A -o json | jq -r '
  .items[] |
  select(.status.conditions[]? | select(.type=="Active" and .status=="False")) |
  "\(.metadata.namespace)/\(.metadata.name)"
' | while read revision; do
    namespace=$(echo $revision | cut -d'/' -f1)
    name=$(echo $revision | cut -d'/' -f2)

    echo "Found stale revision: $revision"

    if [[ "$DRY_RUN" == "false" ]]; then
        echo "  Deleting..."
        kubectl delete revision "$name" -n "$namespace"
    else
        echo "  (dry-run, use DRY_RUN=false to delete)"
    fi
done
```

### Phase 5: Documentation

#### 5.1 Update Model Recipe Documentation

Document the new `deploymentMode` field and when to use each mode.

#### 5.2 Create Runbook

**File**: `docs/runbooks/gpu-deployment-mode.md`

Document:
- How deployment mode selection works
- When to override the default
- Troubleshooting GPU scheduling issues
- HPA configuration for GPU workloads

---

## Migration Plan

### Step 1: Deploy Schema Changes

1. Update CRDs with new `deploymentMode` field
2. Deploy updated operator
3. Existing deployments continue working (backward compatible)

### Step 2: Apply Knative GC Configuration

1. Apply `config-gc.yaml` to Knative Serving namespace
2. Verify GC is cleaning up old revisions

### Step 3: Clean Up Legacy Revisions

1. Run cleanup script in dry-run mode
2. Review revisions to be deleted
3. Run cleanup script with `DRY_RUN=false`

### Step 4: Update Existing GPU Recipes

Add explicit `deploymentMode: RawDeployment` to existing GPU recipes for clarity:

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: llama-3.1-8b-instruct-trtllm
spec:
  runtime: tensorrt-llm
  deploymentMode: RawDeployment  # Explicit (would be inferred anyway)
  # ...
```

### Step 5: Validate

1. Deploy a new GPU model
2. Verify it uses RawDeployment mode
3. Verify nodeSelector is applied correctly
4. Verify multi-port containers work (Triton)

---

## Testing Strategy

### Unit Tests

1. `TestDetermineDeploymentMode` - verify mode selection logic
2. `TestInferenceServiceBuilder_RawDeployment` - verify correct annotations
3. `TestHPABuilder` - verify HPA creation for RawDeployment

### Integration Tests

1. Deploy tensorrt-llm model → verify RawDeployment
2. Deploy vllm with GPU → verify RawDeployment
3. Deploy vllm without GPU → verify Serverless
4. Deploy with explicit override → verify override works

### E2E Tests

1. Full deployment cycle for GPU model
2. Verify inference works end-to-end
3. Verify autoscaling (if HPA configured)

---

## Rollback Plan

If issues arise:

1. **Immediate**: Set `deploymentMode: Serverless` on affected recipes
2. **Short-term**: Revert operator to previous version
3. **Investigation**: Check operator logs for mode selection issues

The changes are backward compatible - existing deployments continue working.

---

## Success Criteria

1. All GPU runtimes (`tensorrt-llm`, `triton`, `vllm+GPU`) use RawDeployment by default
2. No Knative admission webhook rejections for GPU workloads
3. Multi-port ClusterServingRuntimes (Triton) work correctly
4. Stale revisions are cleaned up within 1 hour
5. CPU workloads continue using Serverless mode with scale-to-zero

---

## Open Questions

1. **Custom metrics for GPU HPA?** - Should we implement GPU utilization-based autoscaling?
2. **Warm pool?** - Should we maintain warm standby replicas for faster scaling?
3. **Traffic splitting without Knative?** - How to do canary releases with RawDeployment?

---

## References

- [KServe Deployment Modes](https://kserve.github.io/website/latest/admin/serverless/serverless/)
- [Knative Serving Configuration](https://knative.dev/docs/serving/configuration/)
- [Kubernetes HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- Spec 029: TensorRT-LLM/Triton Support
- `specs/029-triton-tensorrt-llm/issues-to-be-addressed.md`
