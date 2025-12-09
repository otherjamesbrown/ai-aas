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

package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

const (
	// vLLMAppLabelKey is the label key for vLLM applications
	vLLMAppLabelKey = "app.kubernetes.io/name"
	// vLLMAppLabelValue is the label value for vLLM applications
	vLLMAppLabelValue = "vllm-inference"
	// vLLMContainerPort is the port vLLM listens on
	vLLMContainerPort = 8000
)

// generateDeployment creates a vLLM Deployment object for the given AIModel.
func generateDeployment(aiModel *aimodelv1alpha1.AIModel) *appsv1.Deployment {
	replicas := int32(1)
	if aiModel.Spec.Replicas != nil {
		replicas = *aiModel.Spec.Replicas
	}

	labels := map[string]string{
		vLLMAppLabelKey: vLLMAppLabelValue,
		"aimodel.ai-aas.io/model-id": aiModel.Spec.ModelID,
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModel.Name + "-vllm-deployment",
			Namespace: aiModel.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "vllm-container",
						Image: aiModel.Spec.Image,
						Ports: []corev1.ContainerPort{{
							ContainerPort: vLLMContainerPort,
							Name:          "http",
						}},
						Env: []corev1.EnvVar{{
							Name:  "MODEL",
							Value: aiModel.Spec.ModelID, // Pass the model ID to the container
						}}, // TODO: Add more environment variables for model path/args
					}},
				},
			},
		},
	}
}

// generateService creates a vLLM Service object for the given AIModel.
func generateService(aiModel *aimodelv1alpha1.AIModel) *corev1.Service {
	labels := map[string]string{
		vLLMAppLabelKey: vLLMAppLabelValue,
		"aimodel.ai-aas.io/model-id": aiModel.Spec.ModelID,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aiModel.Name + "-vllm-service",
			Namespace: aiModel.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(vLLMContainerPort),
				Name:       "http",
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}
