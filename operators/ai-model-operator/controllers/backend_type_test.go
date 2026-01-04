package controllers

import (
	"testing"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetermineBackendType(t *testing.T) {
	tests := []struct {
		name                string
		aiModel             *aimodelv1alpha1.AIModel
		expectedBackendType string
	}{
		{
			name: "tensorrt-llm runtime returns triton",
			aiModel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "tensorrt-llm",
				},
			},
			expectedBackendType: "triton",
		},
		{
			name: "triton runtime returns triton",
			aiModel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "triton",
				},
			},
			expectedBackendType: "triton",
		},
		{
			name: "vllm runtime returns openai",
			aiModel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "vllm",
				},
			},
			expectedBackendType: "openai",
		},
		{
			name: "tgi runtime returns openai",
			aiModel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "tgi",
				},
			},
			expectedBackendType: "openai",
		},
		{
			name: "empty runtime defaults to openai (vllm default)",
			aiModel: &aimodelv1alpha1.AIModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "system",
				},
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "",
				},
			},
			expectedBackendType: "openai",
		},
		{
			name: "unknown runtime defaults to openai",
			aiModel: &aimodelv1alpha1.AIModel{
				Spec: aimodelv1alpha1.AIModelSpec{
					Runtime: "unknown-runtime",
				},
			},
			expectedBackendType: "openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineBackendType(tt.aiModel)
			assert.Equal(t, tt.expectedBackendType, result)
		})
	}
}
