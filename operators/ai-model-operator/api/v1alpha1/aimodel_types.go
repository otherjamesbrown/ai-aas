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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AIModelSpec defines the desired state of AIModel
type AIModelSpec struct {
	// INSERT CUSTOM FIELDS - desired state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file

	// +kubebuilder:validation:MinLength=1
	// ModelName is the name of the AI model.
	ModelName string `json:"modelName"`

	// ModelID is the unique identifier for the model, typically a hash or UUID.
	ModelID string `json:"modelID"`

	// S3Bucket is the S3 bucket where model artifacts are stored.
	S3Bucket string `json:"s3Bucket"`

	// S3Key is the S3 key (path) to the model artifacts.
	S3Key string `json:"s3Key"`

	// +kubebuilder:validation:Minimum=0
	// Replicas is the number of vLLM instances to deploy.
	Replicas *int32 `json:"replicas,omitempty"`

	// Enabled determines if the model deployment should be active.
	// If false, associated resources will be scaled down or deleted.
	Enabled bool `json:"enabled,omitempty"`
}

// AIModelStatus defines the observed state of AIModel
type AIModelStatus struct {
	// INSERT CUSTOM FIELDS - observed state of cluster
	// Important: Run "make generate" to regenerate code after modifying this file

	// Conditions represent the latest available observations of an object's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase indicates the current phase of the AIModel deployment (e.g., Pending, Downloading, Deploying, Ready, Failed).
	Phase string `json:"phase,omitempty"`

	// VLLMDeploymentName is the name of the associated vLLM Deployment.
	VLLMDeploymentName string `json:"vllmDeploymentName,omitempty"`

	// VLLMServiceName is the name of the associated vLLM Service.
	VLLMServiceName string `json:"vllmServiceName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=aimodels,scope=Namespaced,singular=aimodel
//+kubebuilder:printcolumn:name="Model Name",type="string",JSONPath=".spec.modelName",description="Name of the AI Model"
//+kubebuilder:printcolumn:name="Model ID",type="string",JSONPath=".spec.modelID",description="ID of the AI Model"
//+kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled",description="Is the model deployment enabled?"
//+kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="Number of vLLM replicas"
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

func init() {
	SchemeBuilder.Register(&AIModel{}, &AIModelList{})
}