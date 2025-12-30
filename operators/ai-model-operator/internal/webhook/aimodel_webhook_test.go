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

package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

func TestAIModelValidator_Validate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = aimodelv1alpha1.AddToScheme(scheme)

	tests := []struct {
		name            string
		aimodel         *aimodelv1alpha1.AIModel
		nodes           []corev1.Node
		wantErr         bool
		errContains     string
		wantWarnings    int
		warningContains string
	}{
		{
			name: "no GPU nodes - validation skipped",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
				},
			},
			nodes:   []corev1.Node{},
			wantErr: false,
		},
		{
			name: "GPU nodes with taint - missing toleration rejected",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Value:  "true",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "gpu-workload=true:NoSchedule",
		},
		{
			name: "GPU nodes with taint - toleration present allowed",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
					Tolerations: []corev1.Toleration{
						{
							Key:      "gpu-workload",
							Operator: corev1.TolerationOpEqual,
							Value:    "true",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Value:  "true",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("16"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GPU nodes with taint - Exists operator covers taint",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
					Tolerations: []corev1.Toleration{
						{
							Key:      "gpu-workload",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Value:  "true",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("16"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "CPU request exceeds smallest GPU node - warning",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("20"),
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "gpu-workload",
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("16"),
						},
					},
				},
			},
			wantErr:         false,
			wantWarnings:    1,
			warningContains: "exceeds smallest GPU node",
		},
		{
			name: "PreferNoSchedule taint - not required",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "prefer-gpu",
								Effect: corev1.TaintEffectPreferNoSchedule,
							},
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("16"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple GPU nodes with different taints - all must be covered",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
					Tolerations: []corev1.Toleration{
						{
							Key:      "gpu-workload",
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-2",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "high-memory",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "high-memory:NoSchedule",
		},
		{
			name: "toleration with empty effect matches all effects",
			aimodel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					ModelName: "llama-7b",
					ModelID:   "meta-llama/Llama-2-7b-hf",
					Tolerations: []corev1.Toleration{
						{
							Key:      "gpu-workload",
							Operator: corev1.TolerationOpExists,
							// Empty effect matches all effects
						},
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpu-node-1",
						Labels: map[string]string{
							"nvidia.com/gpu.present": "true",
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "gpu-workload",
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("16"),
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with nodes
			objs := make([]runtime.Object, len(tt.nodes))
			for i := range tt.nodes {
				objs[i] = &tt.nodes[i]
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			validator := &AIModelValidator{
				Client: fakeClient,
			}

			warnings, err := validator.validate(context.Background(), tt.aimodel)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}

			if tt.wantWarnings > 0 {
				assert.Len(t, warnings, tt.wantWarnings)
				if tt.warningContains != "" {
					assert.Contains(t, warnings[0], tt.warningContains)
				}
			} else {
				assert.Empty(t, warnings)
			}
		})
	}
}

func TestTolerationMatchesTaint(t *testing.T) {
	validator := &AIModelValidator{}

	tests := []struct {
		name       string
		toleration corev1.Toleration
		taint      corev1.Taint
		want       bool
	}{
		{
			name: "exact match with Equal operator",
			toleration: corev1.Toleration{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			},
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: true,
		},
		{
			name: "key mismatch",
			toleration: corev1.Toleration{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			},
			taint: corev1.Taint{
				Key:    "other-key",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: false,
		},
		{
			name: "value mismatch",
			toleration: corev1.Toleration{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			},
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Value:  "false",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: false,
		},
		{
			name: "effect mismatch",
			toleration: corev1.Toleration{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			},
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Value:  "true",
				Effect: corev1.TaintEffectNoExecute,
			},
			want: false,
		},
		{
			name: "Exists operator matches without value",
			toleration: corev1.Toleration{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Value:  "any-value",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: true,
		},
		{
			name: "empty effect in toleration matches any effect",
			toleration: corev1.Toleration{
				Key:      "gpu-workload",
				Operator: corev1.TolerationOpExists,
			},
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: true,
		},
		{
			name: "wildcard toleration with empty key",
			toleration: corev1.Toleration{
				Operator: corev1.TolerationOpExists,
			},
			taint: corev1.Taint{
				Key:    "any-key",
				Value:  "any-value",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.tolerationMatchesTaint(tt.toleration, tt.taint)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTaint(t *testing.T) {
	tests := []struct {
		name  string
		taint corev1.Taint
		want  string
	}{
		{
			name: "taint with value",
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: "gpu-workload=true:NoSchedule",
		},
		{
			name: "taint without value",
			taint: corev1.Taint{
				Key:    "gpu-workload",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: "gpu-workload:NoSchedule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTaint(tt.taint)
			assert.Equal(t, tt.want, got)
		})
	}
}
