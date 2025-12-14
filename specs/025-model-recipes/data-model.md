# Data Model: Model Recipes

**Feature**: 025-model-recipes
**Date**: 2025-12-14

## Custom Resource Definitions

### ModelRecipe CRD

```go
// ModelRecipeSpec defines the desired state of ModelRecipe
type ModelRecipeSpec struct {
    // ModelID is the unique identifier for the model (e.g., HuggingFace model ID)
    // +kubebuilder:validation:MinLength=1
    ModelID string `json:"modelID"`

    // DisplayName is the human-readable name for the model
    // +optional
    DisplayName string `json:"displayName,omitempty"`

    // Description provides additional context about the model
    // +optional
    Description string `json:"description,omitempty"`

    // Runtime specifies the inference runtime
    // +kubebuilder:validation:Enum=vllm;triton;tgi
    // +kubebuilder:default=vllm
    Runtime string `json:"runtime"`

    // Image is the container image for the runtime
    // If not specified, uses default image for the runtime
    // +optional
    Image string `json:"image,omitempty"`

    // Resources specifies compute requirements
    Resources RecipeResources `json:"resources"`

    // RuntimeArgs contains runtime-specific configuration
    // +optional
    RuntimeArgs RuntimeArgsSpec `json:"runtimeArgs,omitempty"`

    // Scheduling contains pod scheduling configuration
    // +optional
    Scheduling SchedulingSpec `json:"scheduling,omitempty"`

    // HealthCheck contains health check configuration
    // +optional
    HealthCheck HealthCheckSpec `json:"healthCheck,omitempty"`

    // Metadata contains model-specific metadata
    // +optional
    Metadata ModelMetadata `json:"metadata,omitempty"`
}

// RecipeResources defines compute resource requirements
type RecipeResources struct {
    // GPU specifies GPU requirements
    GPU GPUResources `json:"gpu"`

    // CPU specifies CPU requirements
    // +optional
    CPU CPUResources `json:"cpu,omitempty"`

    // Memory specifies memory requirements
    // +optional
    Memory MemoryResources `json:"memory,omitempty"`
}

// GPUResources defines GPU requirements
type GPUResources struct {
    // Vendor is the GPU vendor (nvidia, amd, intel)
    // +kubebuilder:validation:Enum=nvidia;amd;intel
    // +kubebuilder:default=nvidia
    Vendor string `json:"vendor,omitempty"`

    // Model is the specific GPU model for baseline (e.g., rtx4000-ada, a100-40gb)
    // +optional
    Model string `json:"model,omitempty"`

    // Count is the number of GPUs required
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:default=1
    Count int32 `json:"count"`

    // MinMemoryGB is the minimum GPU memory required in GB
    // +optional
    MinMemoryGB int32 `json:"minMemoryGB,omitempty"`
}

// CPUResources defines CPU requirements
type CPUResources struct {
    // Requests is the CPU request
    Requests string `json:"requests,omitempty"`
    // Limits is the CPU limit
    Limits string `json:"limits,omitempty"`
}

// MemoryResources defines memory requirements
type MemoryResources struct {
    // Requests is the memory request
    Requests string `json:"requests,omitempty"`
    // Limits is the memory limit
    Limits string `json:"limits,omitempty"`
}

// RuntimeArgsSpec contains runtime-specific configuration
type RuntimeArgsSpec struct {
    // VLLM contains vLLM-specific configuration
    // +optional
    VLLM *VLLMArgs `json:"vllm,omitempty"`

    // Triton contains Triton-specific configuration
    // +optional
    Triton *TritonArgs `json:"triton,omitempty"`

    // TGI contains TGI-specific configuration
    // +optional
    TGI *TGIArgs `json:"tgi,omitempty"`
}

// VLLMArgs contains vLLM-specific configuration
type VLLMArgs struct {
    // DType is the data type for model weights
    // +kubebuilder:validation:Enum=auto;float16;bfloat16;float32
    // +kubebuilder:default=auto
    DType string `json:"dtype,omitempty"`

    // MaxModelLen is the maximum sequence length
    // +optional
    MaxModelLen int32 `json:"maxModelLen,omitempty"`

    // GPUMemoryUtilization is the fraction of GPU memory to use (0.0-1.0)
    // +optional
    GPUMemoryUtilization string `json:"gpuMemoryUtilization,omitempty"`

    // TrustRemoteCode allows execution of custom model code
    // +optional
    TrustRemoteCode bool `json:"trustRemoteCode,omitempty"`

    // TokenizerMode specifies the tokenizer mode
    // +optional
    TokenizerMode string `json:"tokenizerMode,omitempty"`

    // ExtraArgs contains additional command-line arguments
    // +optional
    ExtraArgs []string `json:"extraArgs,omitempty"`
}

// TritonArgs contains Triton-specific configuration
type TritonArgs struct {
    // Backend is the Triton backend type
    // +kubebuilder:validation:Enum=python;tensorrt;onnxruntime;pytorch
    Backend string `json:"backend"`

    // ModelRepository is the S3/GCS path to the model repository
    // +optional
    ModelRepository string `json:"modelRepository,omitempty"`

    // InstanceGroup defines GPU instance configuration
    // +optional
    InstanceGroup []TritonInstanceGroup `json:"instanceGroup,omitempty"`

    // DynamicBatching contains batching configuration
    // +optional
    DynamicBatching *TritonDynamicBatching `json:"dynamicBatching,omitempty"`

    // InputConfig defines input tensor configuration
    // +optional
    InputConfig []TritonTensorConfig `json:"inputConfig,omitempty"`

    // OutputConfig defines output tensor configuration
    // +optional
    OutputConfig []TritonTensorConfig `json:"outputConfig,omitempty"`
}

// TritonInstanceGroup defines GPU instance configuration
type TritonInstanceGroup struct {
    Kind  string `json:"kind"`
    Count int32  `json:"count"`
}

// TritonDynamicBatching defines batching configuration
type TritonDynamicBatching struct {
    MaxBatchSize              int32 `json:"maxBatchSize,omitempty"`
    MaxQueueDelayMicroseconds int64 `json:"maxQueueDelayMicroseconds,omitempty"`
}

// TritonTensorConfig defines tensor configuration
type TritonTensorConfig struct {
    Name     string  `json:"name"`
    DataType string  `json:"dataType"`
    Dims     []int32 `json:"dims"`
}

// TGIArgs contains TGI-specific configuration
type TGIArgs struct {
    // Quantize specifies quantization method
    // +kubebuilder:validation:Enum=;bitsandbytes;gptq;awq
    // +optional
    Quantize string `json:"quantize,omitempty"`

    // MaxInputLength is the maximum input sequence length
    // +optional
    MaxInputLength int32 `json:"maxInputLength,omitempty"`

    // MaxTotalTokens is the maximum total tokens (input + output)
    // +optional
    MaxTotalTokens int32 `json:"maxTotalTokens,omitempty"`

    // MaxBatchPrefillTokens is the maximum batch size for prefill
    // +optional
    MaxBatchPrefillTokens int32 `json:"maxBatchPrefillTokens,omitempty"`

    // NumShard is the number of shards for multi-GPU
    // +optional
    NumShard int32 `json:"numShard,omitempty"`

    // DisableFlashAttention disables flash attention
    // +optional
    DisableFlashAttention bool `json:"disableFlashAttention,omitempty"`
}

// SchedulingSpec contains pod scheduling configuration
type SchedulingSpec struct {
    // Tolerations for the pod
    // +optional
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

    // NodeSelector for the pod
    // +optional
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`

    // Affinity for the pod
    // +optional
    Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// HealthCheckSpec contains health check configuration
type HealthCheckSpec struct {
    // StartupProbeSeconds is the startup probe timeout
    // +kubebuilder:default=300
    StartupProbeSeconds int32 `json:"startupProbeSeconds,omitempty"`

    // LivenessPath is the path for liveness checks
    // +kubebuilder:default=/health
    LivenessPath string `json:"livenessPath,omitempty"`

    // ReadinessPath is the path for readiness checks
    // +kubebuilder:default=/health
    ReadinessPath string `json:"readinessPath,omitempty"`
}

// ModelMetadata contains model-specific metadata
type ModelMetadata struct {
    // Parameters is the model parameter count (e.g., "7B", "70B")
    // +optional
    Parameters string `json:"parameters,omitempty"`

    // ContextLength is the maximum context length
    // +optional
    ContextLength int32 `json:"contextLength,omitempty"`

    // Architecture is the model architecture
    // +optional
    Architecture string `json:"architecture,omitempty"`

    // License is the model license
    // +optional
    License string `json:"license,omitempty"`

    // SourceURL is the URL to the model source (e.g., HuggingFace)
    // +optional
    SourceURL string `json:"sourceURL,omitempty"`
}

// ModelRecipeStatus defines the observed state of ModelRecipe
type ModelRecipeStatus struct {
    // Conditions represent the latest available observations
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // DeploymentCount is the number of AIModels using this recipe
    DeploymentCount int32 `json:"deploymentCount,omitempty"`

    // LastValidated is when the recipe was last validated
    LastValidated *metav1.Time `json:"lastValidated,omitempty"`
}
```

### Updated AIModel CRD (additions)

```go
// AIModelSpec additions for recipe support
type AIModelSpec struct {
    // ... existing fields ...

    // RecipeRef references a ModelRecipe to use as the base configuration
    // +optional
    RecipeRef *RecipeReference `json:"recipeRef,omitempty"`

    // Overrides allows overriding specific recipe fields
    // These are deep-merged with the recipe configuration
    // +optional
    Overrides *RecipeOverrides `json:"overrides,omitempty"`
}

// RecipeReference identifies a ModelRecipe
type RecipeReference struct {
    // Name is the name of the ModelRecipe
    Name string `json:"name"`

    // Namespace is the namespace of the ModelRecipe
    // Defaults to ai-model-system if not specified
    // +optional
    Namespace string `json:"namespace,omitempty"`
}

// RecipeOverrides allows overriding recipe fields
type RecipeOverrides struct {
    // Runtime overrides the recipe runtime
    // +optional
    Runtime string `json:"runtime,omitempty"`

    // Image overrides the recipe image
    // +optional
    Image string `json:"image,omitempty"`

    // Replicas overrides the replica configuration
    // +optional
    Replicas *ReplicaOverrides `json:"replicas,omitempty"`

    // Resources overrides resource requirements
    // +optional
    Resources *RecipeResources `json:"resources,omitempty"`

    // RuntimeArgs overrides runtime-specific args
    // +optional
    RuntimeArgs *RuntimeArgsSpec `json:"runtimeArgs,omitempty"`

    // Scheduling overrides scheduling configuration
    // +optional
    Scheduling *SchedulingSpec `json:"scheduling,omitempty"`
}

// ReplicaOverrides for AIModel
type ReplicaOverrides struct {
    Min *int32 `json:"min,omitempty"`
    Max *int32 `json:"max,omitempty"`
}
```

## Database Schema

### recipes table (Admin API)

```sql
CREATE TABLE recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    description TEXT,
    model_id VARCHAR(255) NOT NULL,
    runtime VARCHAR(50) NOT NULL DEFAULT 'vllm',
    spec JSONB NOT NULL,  -- Full recipe spec as JSON
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_recipes_runtime ON recipes(runtime);
CREATE INDEX idx_recipes_model_id ON recipes(model_id);
```

## Labels

Standard labels for ModelRecipe resources:

```yaml
labels:
  ai.ai-aas.io/model-family: mistral    # Model family (mistral, llama, qwen)
  ai.ai-aas.io/model-size: 7b           # Model size (7b, 13b, 70b)
  ai.ai-aas.io/task: text-generation    # Task type (text-generation, image-captioning)
  ai.ai-aas.io/runtime: vllm            # Runtime (vllm, triton, tgi)
```
