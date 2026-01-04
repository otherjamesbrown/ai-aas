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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AIModelSpec defines the desired state of AIModel
type AIModelSpec struct {
	// INSERT CUSTOM FIELDS - desired state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z]([a-z0-9-]*[a-z0-9])?$`
	// ModelName is the human-readable name of the AI model.
	// Must be DNS-compatible: lowercase alphanumeric and hyphens only, must start
	// with a letter and end with alphanumeric. No periods allowed (use hyphens instead,
	// e.g., "llama-3-1-8b" not "llama-3.1-8b").
	ModelName string `json:"modelName"`

	// ModelID is the unique identifier for the model (e.g., HuggingFace model ID).
	// This is the full internal path used by all internal systems (e.g., "unsloth/gpt-oss-20b").
	ModelID string `json:"modelID"`

	// ExternalName is the name exposed in OpenAI-compatible APIs (/v1/models, /v1/chat/completions).
	// If not specified, derived from ModelID by taking the part after the last "/".
	// Example: modelID "unsloth/gpt-oss-20b" -> externalName "gpt-oss-20b"
	// This allows multiple models with the same base name from different providers to coexist
	// with different external names (e.g., "gpt-oss-20b-unsloth" vs "gpt-oss-20b-openai").
	// Must be unique per environment.
	// +optional
	ExternalName string `json:"externalName,omitempty"`

	// ModelType indicates the type of model (text, vision-language, embedding, audio).
	// This is used for routing and client display purposes.
	// +kubebuilder:validation:Enum=text;vision-language;embedding;audio
	// +optional
	ModelType string `json:"modelType,omitempty"`

	// S3Bucket is the S3 bucket where model artifacts are stored.
	// Optional when TrustRemoteCode is true (model loads directly from HuggingFace).
	// +optional
	S3Bucket string `json:"s3Bucket,omitempty"`

	// S3Key is the S3 key (path) to the model artifacts.
	// Optional when TrustRemoteCode is true (model loads directly from HuggingFace).
	// +optional
	S3Key string `json:"s3Key,omitempty"`

	// Enabled determines if the model deployment should be active.
	// If false, the InferenceService will be scaled to zero replicas.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Runtime specifies the inference runtime to use.
	// +kubebuilder:validation:Enum=vllm;triton;tensorrt-llm;tgi
	// +kubebuilder:default=vllm
	Runtime string `json:"runtime,omitempty"`

	// RuntimeName specifies the name of the ClusterServingRuntime to use.
	// If not specified, the operator will use the default runtime based on the Runtime field.
	// +optional
	RuntimeName string `json:"runtimeName,omitempty"`

	// DeploymentMode specifies how the model should be deployed.
	// If set, overrides recipe and runtime-based defaults.
	// +kubebuilder:validation:Enum=Serverless;RawDeployment
	// +optional
	DeploymentMode string `json:"deploymentMode,omitempty"`

	// MinReplicas is the minimum number of replicas for autoscaling.
	// Set to 0 to enable scale-to-zero.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas for autoscaling.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// Resources specifies the compute resources required by the inference container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector specifies node labels for pod scheduling.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations specifies tolerations for pod scheduling.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// RuntimeArgs specifies additional command-line arguments for the runtime.
	// For vLLM, these are passed to the vllm serve command.
	// +optional
	RuntimeArgs []string `json:"runtimeArgs,omitempty"`

	// RuntimeEnv specifies additional environment variables for the runtime container.
	// +optional
	RuntimeEnv []corev1.EnvVar `json:"runtimeEnv,omitempty"`

	// TrustRemoteCode allows the runtime to execute custom model code from the model repository.
	// Required for models with custom architectures not built into transformers.
	// For vLLM, this adds the --trust-remote-code flag.
	// When true, S3Bucket and S3Key become optional as the model loads directly from HuggingFace.
	// +optional
	TrustRemoteCode bool `json:"trustRemoteCode"`

	// RecipeRef references a ModelRecipe to use as the base configuration
	// +optional
	RecipeRef *RecipeReference `json:"recipeRef,omitempty"`

	// Overrides allows overriding specific recipe fields
	// These are deep-merged with the recipe configuration
	// +optional
	Overrides *RecipeOverrides `json:"overrides,omitempty"`

	// ProbeConfig allows customizing health probe settings for the inference container.
	// +optional
	ProbeConfig *ProbeConfig `json:"probeConfig,omitempty"`

	// DEPRECATED: Use MinReplicas/MaxReplicas instead.
	// Replicas is the number of inference instances to deploy.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// UpdateStrategy determines how updates to the InferenceService are rolled out.
	// RollingUpdate (default): New pods are created before old ones are terminated.
	// Recreate: Old pods are terminated before new ones are created, freeing GPU resources.
	// +kubebuilder:validation:Enum=RollingUpdate;Recreate
	// +kubebuilder:default=RollingUpdate
	// +optional
	UpdateStrategy UpdateStrategyType `json:"updateStrategy,omitempty"`
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

// ProbeConfig allows customizing health probe settings for the inference container
type ProbeConfig struct {
	// InitialDelaySeconds is the number of seconds after the container starts before liveness probes are initiated.
	// This should be set to allow sufficient time for model loading, especially for large models (100B+).
	// Default: 300 (5 minutes)
	// +kubebuilder:validation:Minimum=0
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
}

// UpdateStrategyType defines how model updates are deployed
type UpdateStrategyType string

const (
	// UpdateStrategyRollingUpdate creates new pods before terminating old ones (default)
	UpdateStrategyRollingUpdate UpdateStrategyType = "RollingUpdate"
	// UpdateStrategyRecreate terminates old pods before creating new ones, freeing GPU resources
	UpdateStrategyRecreate UpdateStrategyType = "Recreate"
)

// AIModelPhase represents the current phase of the AIModel deployment.
type AIModelPhase string

const (
	// AIModelPhasePending indicates the AIModel is waiting to be processed.
	AIModelPhasePending AIModelPhase = "Pending"
	// AIModelPhaseValidating indicates the spec is being validated and recipe is being resolved.
	AIModelPhaseValidating AIModelPhase = "Validating"
	// AIModelPhaseScheduling indicates the pod is waiting to be scheduled on a node.
	AIModelPhaseScheduling AIModelPhase = "Scheduling"
	// AIModelPhaseDownloading indicates model artifacts are being downloaded.
	AIModelPhaseDownloading AIModelPhase = "Downloading"
	// AIModelPhaseInitializing indicates the main container is starting and loading the model.
	AIModelPhaseInitializing AIModelPhase = "Initializing"
	// AIModelPhaseDeploying indicates the InferenceService is being created/updated.
	// DEPRECATED: Use more granular phases (Validating, Scheduling, Downloading, Initializing) instead.
	AIModelPhaseDeploying AIModelPhase = "Deploying"
	// AIModelPhaseReady indicates the model is ready to serve inference requests.
	AIModelPhaseReady AIModelPhase = "Ready"
	// AIModelPhaseFailed indicates an error occurred during deployment.
	AIModelPhaseFailed AIModelPhase = "Failed"
	// AIModelPhaseDisabled indicates the model is disabled (scaled to zero).
	AIModelPhaseDisabled AIModelPhase = "Disabled"
	// AIModelPhaseRetryPending indicates a retry is scheduled after a download failure.
	AIModelPhaseRetryPending AIModelPhase = "RetryPending"
)

// Condition type constants for AIModel
const (
	// ConditionTypeRoutingReady indicates whether a routing policy exists for this model,
	// making it accessible via the API router.
	ConditionTypeRoutingReady = "RoutingReady"
)

// PhaseProgress tracks progress within a specific phase.
type PhaseProgress struct {
	// Percentage indicates overall progress (0-100).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	Percentage int `json:"percentage,omitempty"`

	// Downloaded indicates bytes downloaded (e.g., "2.1GB / 8.5GB").
	// Only applicable during Downloading phase.
	// +optional
	Downloaded string `json:"downloaded,omitempty"`

	// ETA indicates estimated time to completion (e.g., "5m30s").
	// +optional
	ETA string `json:"eta,omitempty"`
}

// SchedulingStatus provides information about pod scheduling
type SchedulingStatus struct {
	// Scheduled indicates whether the pod has been scheduled to a node
	Scheduled bool `json:"scheduled"`

	// BlockedReason is the reason why scheduling is blocked (e.g., "Unschedulable", "InsufficientCPU")
	// Only set when Scheduled is false
	// +optional
	BlockedReason string `json:"blockedReason,omitempty"`

	// BlockedMessage is a human-readable message explaining why scheduling is blocked
	// Only set when Scheduled is false
	// +optional
	BlockedMessage string `json:"blockedMessage,omitempty"`

	// NodeName is the name of the node where the pod is scheduled
	// Only set when Scheduled is true
	// +optional
	NodeName string `json:"nodeName,omitempty"`
}

// ProbeStatus provides information about container readiness probe status
type ProbeStatus struct {
	// Ready indicates if the container is currently ready
	Ready bool `json:"ready,omitempty"`

	// FailureCount is number of consecutive probe failures
	FailureCount int32 `json:"failureCount,omitempty"`

	// LastProbeTime is when the last probe was attempted
	// +optional
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`

	// Message provides probe failure details
	// +optional
	Message string `json:"message,omitempty"`
}

// AutoscalerStatus provides information about cluster autoscaler activity
type AutoscalerStatus struct {
	// ScalingUp indicates the cluster autoscaler is actively provisioning new nodes
	ScalingUp bool `json:"scalingUp,omitempty"`

	// ScaleUpReason explains why scaling is happening (e.g., pod resource requirements)
	// +optional
	ScaleUpReason string `json:"scaleUpReason,omitempty"`

	// AtMaxCapacity indicates the autoscaler cannot add more nodes (node pool at max size)
	AtMaxCapacity bool `json:"atMaxCapacity,omitempty"`

	// PendingNodes is the count of nodes currently being provisioned by the autoscaler
	// +optional
	PendingNodes int32 `json:"pendingNodes,omitempty"`
}

// AIModelStatus defines the observed state of AIModel
type AIModelStatus struct {
	// INSERT CUSTOM FIELDS - observed state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file

	// Conditions represent the latest available observations of an object's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase indicates the current phase of the AIModel deployment.
	Phase AIModelPhase `json:"phase,omitempty"`

	// PhaseStartTime is when the current phase started.
	// +optional
	PhaseStartTime *metav1.Time `json:"phaseStartTime,omitempty"`

	// PhaseDuration is the human-readable duration in the current phase (e.g., "5m30s").
	// +optional
	PhaseDuration string `json:"phaseDuration,omitempty"`

	// Progress tracks progress within the current phase.
	// +optional
	Progress *PhaseProgress `json:"progress,omitempty"`

	// InferenceServiceName is the name of the associated KServe InferenceService.
	InferenceServiceName string `json:"inferenceServiceName,omitempty"`

	// InferenceEndpoint is the URL where the model can be accessed for inference.
	InferenceEndpoint string `json:"inferenceEndpoint,omitempty"`

	// ModelType indicates the type of model (text, vision-language, embedding, audio).
	// Reflects the value from spec.modelType.
	ModelType string `json:"modelType,omitempty"`

	// ReadyReplicas is the number of replicas that are ready to serve requests.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// DownloadProgress indicates the progress of model artifact download (0-100).
	// DEPRECATED: Use Progress.Percentage instead.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	DownloadProgress int32 `json:"downloadProgress,omitempty"`

	// LastTransitionTime is the last time the Phase transitioned.
	// DEPRECATED: Use PhaseStartTime instead.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Message provides additional information about the current phase.
	Message string `json:"message,omitempty"`

	// SchedulingStatus provides information about pod scheduling status
	// +optional
	SchedulingStatus *SchedulingStatus `json:"schedulingStatus,omitempty"`

	// AutoscalerStatus provides information about cluster autoscaler activity
	// Only populated when pod is pending and autoscaler events are detected
	// +optional
	AutoscalerStatus *AutoscalerStatus `json:"autoscalerStatus,omitempty"`

	// ProbeStatus provides information about container readiness probe status
	// +optional
	ProbeStatus *ProbeStatus `json:"probeStatus,omitempty"`

	// RetryCount tracks the number of download retry attempts.
	RetryCount int32 `json:"retryCount,omitempty"`

	// LastRetryTime is when the last retry was attempted.
	LastRetryTime *metav1.Time `json:"lastRetryTime,omitempty"`

	// NextRetryTime is when the next retry will be attempted (if in retry state).
	NextRetryTime *metav1.Time `json:"nextRetryTime,omitempty"`

	// DeploymentStartedAt is when the Deploying phase started.
	// Used for timeout detection to prevent stuck deployments.
	DeploymentStartedAt *metav1.Time `json:"deploymentStartedAt,omitempty"`

	// LastFailureReason provides the reason for the last failure.
	// Set when transitioning to Failed phase.
	LastFailureReason string `json:"lastFailureReason,omitempty"`

	// CrashLoopBackOffCount tracks consecutive CrashLoopBackOff occurrences.
	// Reset when container successfully starts. Used to detect stuck deployments.
	CrashLoopBackOffCount int32 `json:"crashLoopBackOffCount,omitempty"`

	// LastAdminAPISyncTime tracks when deployment record was last synced to Admin API.
	// Used to implement periodic sync to handle Admin API downtime or database resets.
	// +optional
	LastAdminAPISyncTime *metav1.Time `json:"lastAdminAPISyncTime,omitempty"`

	// DEPRECATED: Use InferenceServiceName instead.
	VLLMDeploymentName string `json:"vllmDeploymentName,omitempty"`

	// DEPRECATED: Use InferenceEndpoint instead.
	VLLMServiceName string `json:"vllmServiceName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=aimodels,scope=Namespaced,singular=aimodel
//+kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.modelName",description="Name of the AI Model"
//+kubebuilder:printcolumn:name="Runtime",type="string",JSONPath=".spec.runtime",description="Inference runtime (vllm, triton, tgi)"
//+kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled",description="Is the model deployment enabled?"
//+kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas",description="Number of ready replicas"
//+kubebuilder:printcolumn:name="RoutingReady",type="string",JSONPath=".status.conditions[?(@.type=='RoutingReady')].status",description="Routing policy exists",priority=1
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Current phase of the AI Model deployment"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AIModel is the Schema for the aimodels API
type AIModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIModelSpec   `json:"spec,omitempty"`
	Status AIModelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AIModelList contains a list of AIModel
type AIModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIModel `json:"items"`
}
