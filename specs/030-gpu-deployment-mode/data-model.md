# Data Model: GPU Deployment Mode Migration

**Date**: 2025-12-17
**Spec**: [spec.md](./spec.md)

## CRD Changes

### AIModelSpec (Modified)

```yaml
# operators/ai-model-operator/api/v1alpha1/aimodel_types.go
type: AIModelSpec
changes: ADD field
backward_compatible: true

fields:
  deploymentMode:
    type: string
    required: false
    validation:
      enum: [Serverless, RawDeployment]
    default: "" # Empty = use runtime-based inference
    description: |
      Override deployment mode. If set, takes precedence over recipe.
      - Serverless: Knative-based with scale-to-zero (CPU workloads)
      - RawDeployment: Standard K8s Deployment (GPU workloads)
    kubebuilder_markers:
      - "+kubebuilder:validation:Enum=Serverless;RawDeployment"
      - "+optional"
```

### ModelRecipeSpec (Modified)

```yaml
# operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go
type: ModelRecipeSpec
changes: ADD fields
backward_compatible: true

fields:
  deploymentMode:
    type: string
    required: false
    validation:
      enum: [Serverless, RawDeployment]
    default: "" # Empty = use runtime-based inference
    description: |
      Specifies how models using this recipe should be deployed.
      If not specified, determined automatically based on runtime and GPU requirements.
    kubebuilder_markers:
      - "+kubebuilder:validation:Enum=Serverless;RawDeployment"
      - "+optional"

  autoscaling:
    type: AutoscalingSpec (pointer)
    required: false
    default: null
    description: |
      Autoscaling configuration for RawDeployment mode.
      Only applies when deploymentMode is RawDeployment.
    kubebuilder_markers:
      - "+optional"
```

### AutoscalingSpec (New)

```yaml
# operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go
type: AutoscalingSpec
changes: NEW type

fields:
  enabled:
    type: bool
    required: false
    default: false
    description: Enable HPA for this recipe
    kubebuilder_markers:
      - "+kubebuilder:default=false"

  minReplicas:
    type: int32
    required: false
    default: 1
    validation:
      minimum: 1
    description: Minimum number of replicas
    kubebuilder_markers:
      - "+kubebuilder:validation:Minimum=1"
      - "+kubebuilder:default=1"

  maxReplicas:
    type: int32
    required: false
    default: 1
    validation:
      minimum: 1
    description: Maximum number of replicas
    kubebuilder_markers:
      - "+kubebuilder:validation:Minimum=1"
      - "+kubebuilder:default=1"

  targetCPUUtilization:
    type: int32
    required: false
    default: 70
    validation:
      minimum: 1
      maximum: 100
    description: Target CPU utilization percentage for HPA
    kubebuilder_markers:
      - "+kubebuilder:validation:Minimum=1"
      - "+kubebuilder:validation:Maximum=100"
      - "+kubebuilder:default=70"

  scaleDownStabilization:
    type: int32
    required: false
    default: 300
    validation:
      minimum: 0
    description: Scale down stabilization window in seconds
    kubebuilder_markers:
      - "+kubebuilder:validation:Minimum=0"
      - "+kubebuilder:default=300"
```

## InferenceServiceBuilder (Internal)

```yaml
# operators/ai-model-operator/internal/kserve/inferenceservice.go
type: InferenceServiceBuilder (struct)
changes: ADD field

fields:
  deploymentMode:
    type: string
    default: "" # Empty = Serverless for backward compatibility
    description: |
      Explicit deployment mode passed from controller.
      Used to set serving.kserve.io/deploymentMode annotation.
```

## Deployment Mode Selection Logic

```yaml
# Hierarchy (first match wins):
selection_order:
  1: AIModel.Spec.DeploymentMode (if set)
  2: Recipe.Spec.DeploymentMode (if set)
  3: Runtime-based default:
      tensorrt-llm: RawDeployment
      triton: RawDeployment
      vllm (GPU > 0): RawDeployment
      vllm (GPU = 0): Serverless
      tgi: Serverless
      default: Serverless
```

## Example CRs

### AIModel with Explicit Override

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3-8b-test
  namespace: system
spec:
  modelName: llama-3-8b-test
  modelID: meta-llama/Meta-Llama-3-8B-Instruct
  runtime: vllm
  deploymentMode: RawDeployment  # Explicit override
  resources:
    requests:
      nvidia.com/gpu: "1"
```

### ModelRecipe with Autoscaling

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: llama-3-8b-vllm
  namespace: ai-model-system
spec:
  modelID: meta-llama/Meta-Llama-3-8B-Instruct
  runtime: vllm
  deploymentMode: RawDeployment  # Explicit for GPU
  resources:
    gpu:
      count: 1
      vendor: nvidia
  autoscaling:
    enabled: true
    minReplicas: 1
    maxReplicas: 3
    targetCPUUtilization: 70
    scaleDownStabilization: 300
```

### ModelRecipe for CPU (Serverless)

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: sentence-transformer-cpu
  namespace: ai-model-system
spec:
  modelID: sentence-transformers/all-MiniLM-L6-v2
  runtime: vllm
  deploymentMode: Serverless  # Explicit for CPU, enables scale-to-zero
  resources:
    cpu:
      requests: "2"
      limits: "4"
    memory:
      requests: "4Gi"
      limits: "8Gi"
    gpu:
      count: 0  # No GPU
```

## Validation Rules

```yaml
validation:
  deploymentMode:
    - enum: [Serverless, RawDeployment, ""]
    - warning_if: deploymentMode == "Serverless" AND gpu.count > 0
      message: "Serverless mode with GPU may have scheduling issues"

  autoscaling:
    - if: autoscaling.enabled == true
      then: autoscaling.maxReplicas >= autoscaling.minReplicas
    - if: autoscaling.enabled == true AND deploymentMode == "Serverless"
      then: warning("Autoscaling config ignored in Serverless mode")
```

## Migration Notes

```yaml
migration:
  existing_aimodels:
    - No changes required (backward compatible)
    - deploymentMode defaults to runtime inference
    - Behavior unchanged

  existing_recipes:
    - No changes required (backward compatible)
    - Recommend adding explicit deploymentMode for clarity
    - Phase 3 task in implementation plan

  crd_upgrade:
    - make generate
    - make manifests
    - Deploy updated CRDs (backward compatible)
```
