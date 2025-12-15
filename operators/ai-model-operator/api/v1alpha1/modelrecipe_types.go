/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	// +kubebuilder:validation:Enum=vllm;triton;tensorrt-llm;tgi
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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=modelrecipes,scope=Namespaced,singular=modelrecipe
//+kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.displayName",description="Display name of the model"
//+kubebuilder:printcolumn:name="Runtime",type="string",JSONPath=".spec.runtime",description="Inference runtime (vllm, triton, tgi)"
//+kubebuilder:printcolumn:name="GPU",type="integer",JSONPath=".spec.resources.gpu.count",description="Number of GPUs required"
//+kubebuilder:printcolumn:name="Deployments",type="integer",JSONPath=".status.deploymentCount",description="Number of AIModels using this recipe"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ModelRecipe is the Schema for the modelrecipes API
type ModelRecipe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelRecipeSpec   `json:"spec,omitempty"`
	Status ModelRecipeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModelRecipeList contains a list of ModelRecipe
type ModelRecipeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelRecipe `json:"items"`
}
