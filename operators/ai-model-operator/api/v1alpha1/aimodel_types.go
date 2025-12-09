package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +groupName=aimodel.ai-aas.io

const (
	// AIModelFinalizer is the finalizer name for AIModel resources
	AIModelFinalizer = "aimodel.ai-aas.io/finalizer"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags.

// AIModelSpec defines the desired state of AIModel
type AIModelSpec struct {
	// INSERT FIE
	// +kubebuilder:validation:MinLength=1
	ModelID string `json:"modelID"`
	// +kubebuilder:validation:MinLength=1
	// +optional
	Revision string `json:"revision,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +kubebuilder:default:=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// AIModelStatus defines the observed state of AIModel
type AIModelStatus struct {
	// INSERT FIELDS HERE (e.g. status detail for the Model)
	// +optional
	Phase string `json:"phase,omitempty"` // e.g., Pending, Downloading, Ready, Failed
	// +optional
	InferenceEndpoint string `json:"inferenceEndpoint,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=aimodels,scope=Namespaced,singular=aimodel,shortName=aim
// +kubebuilder:printcolumn:name="ModelID",type="string",JSONPath=".spec.modelID",description="HuggingFace Model ID"
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".spec.revision",description="Model Revision"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image",description="Inference Image"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled",description="Is Model Enabled"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current status of the AI Model"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.inferenceEndpoint",description="Inference Endpoint URL"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AIModel is the Schema for the aimodels API
type AIModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIModelSpec   `json:"spec,omitempty"`
	Status AIModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object_root=true

// AIModelList contains a list of AIModel
type AIModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AIModel `json:"items"`
}
