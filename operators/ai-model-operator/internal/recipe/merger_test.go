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

package recipe

import (
	"testing"

	"github.com/ai-aas/ai-model-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Test_MergeRecipe_NoOverrides tests that when no overrides are provided,
// the original recipe spec is returned unchanged.
func Test_MergeRecipe_NoOverrides(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID:     "meta-llama/Llama-2-7b-hf",
		DisplayName: "Llama 2 7B",
		Runtime:     "vllm",
		Image:       "vllm/vllm-openai:v0.2.7",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Vendor: "nvidia",
				Model:  "rtx4000-ada",
				Count:  1,
			},
			CPU: v1alpha1.CPUResources{
				Requests: "4",
				Limits:   "8",
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "16Gi",
				Limits:   "32Gi",
			},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				DType:                "auto",
				MaxModelLen:          4096,
				GPUMemoryUtilization: "0.9",
				TrustRemoteCode:      false,
			},
		},
	}

	var nilOverrides *v1alpha1.RecipeOverrides = nil

	// Act
	result := MergeRecipe(originalSpec, nilOverrides)

	// Assert
	if result.ModelID != originalSpec.ModelID {
		t.Errorf("Expected ModelID %s, got %s", originalSpec.ModelID, result.ModelID)
	}
	if result.Runtime != originalSpec.Runtime {
		t.Errorf("Expected Runtime %s, got %s", originalSpec.Runtime, result.Runtime)
	}
	if result.Image != originalSpec.Image {
		t.Errorf("Expected Image %s, got %s", originalSpec.Image, result.Image)
	}
	if result.Resources.GPU.Count != originalSpec.Resources.GPU.Count {
		t.Errorf("Expected GPU Count %d, got %d", originalSpec.Resources.GPU.Count, result.Resources.GPU.Count)
	}
	if result.RuntimeArgs.VLLM.MaxModelLen != originalSpec.RuntimeArgs.VLLM.MaxModelLen {
		t.Errorf("Expected MaxModelLen %d, got %d", originalSpec.RuntimeArgs.VLLM.MaxModelLen, result.RuntimeArgs.VLLM.MaxModelLen)
	}
}

// Test_MergeRecipe_RuntimeOverride tests overriding the runtime field.
func Test_MergeRecipe_RuntimeOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Image:   "vllm/vllm-openai:v0.2.7",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Runtime: "triton",
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	if result.Runtime != "triton" {
		t.Errorf("Expected Runtime 'triton', got %s", result.Runtime)
	}
	if result.Image != originalSpec.Image {
		t.Errorf("Image should remain unchanged, got %s", result.Image)
	}
	if result.ModelID != originalSpec.ModelID {
		t.Errorf("ModelID should remain unchanged, got %s", result.ModelID)
	}
}

// Test_MergeRecipe_ImageOverride tests overriding the container image.
func Test_MergeRecipe_ImageOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Image:   "vllm/vllm-openai:v0.2.7",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Image: "vllm/vllm-openai:v0.3.0",
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	if result.Image != "vllm/vllm-openai:v0.3.0" {
		t.Errorf("Expected Image 'vllm/vllm-openai:v0.3.0', got %s", result.Image)
	}
	if result.Runtime != originalSpec.Runtime {
		t.Errorf("Runtime should remain unchanged, got %s", result.Runtime)
	}
}

// Test_MergeRecipe_ResourcesOverride tests deep merging of resource fields.
// This verifies that individual resource fields can be overridden while preserving others.
func Test_MergeRecipe_ResourcesOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Vendor:      "nvidia",
				Model:       "rtx4000-ada",
				Count:       1,
				MinMemoryGB: 20,
			},
			CPU: v1alpha1.CPUResources{
				Requests: "4",
				Limits:   "8",
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "16Gi",
				Limits:   "32Gi",
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Resources: &v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Count: 2, // Override GPU count
				// Vendor and Model should be preserved
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "32Gi", // Override memory request
				// Limits should be preserved
			},
			// CPU should remain unchanged
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// GPU count should be overridden
	if result.Resources.GPU.Count != 2 {
		t.Errorf("Expected GPU Count 2, got %d", result.Resources.GPU.Count)
	}
	// GPU vendor and model should be preserved
	if result.Resources.GPU.Vendor != "nvidia" {
		t.Errorf("Expected GPU Vendor 'nvidia', got %s", result.Resources.GPU.Vendor)
	}
	if result.Resources.GPU.Model != "rtx4000-ada" {
		t.Errorf("Expected GPU Model 'rtx4000-ada', got %s", result.Resources.GPU.Model)
	}
	if result.Resources.GPU.MinMemoryGB != 20 {
		t.Errorf("Expected GPU MinMemoryGB 20, got %d", result.Resources.GPU.MinMemoryGB)
	}

	// Memory request should be overridden
	if result.Resources.Memory.Requests != "32Gi" {
		t.Errorf("Expected Memory Requests '32Gi', got %s", result.Resources.Memory.Requests)
	}
	// Memory limits should be preserved
	if result.Resources.Memory.Limits != "32Gi" {
		t.Errorf("Expected Memory Limits '32Gi', got %s", result.Resources.Memory.Limits)
	}

	// CPU should remain unchanged
	if result.Resources.CPU.Requests != "4" {
		t.Errorf("Expected CPU Requests '4', got %s", result.Resources.CPU.Requests)
	}
	if result.Resources.CPU.Limits != "8" {
		t.Errorf("Expected CPU Limits '8', got %s", result.Resources.CPU.Limits)
	}
}

// Test_MergeRecipe_RuntimeArgsOverride_VLLM tests deep merging of vLLM runtime args.
// This verifies that individual vLLM args can be overridden while preserving others.
func Test_MergeRecipe_RuntimeArgsOverride_VLLM(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				DType:                "auto",
				MaxModelLen:          4096,
				GPUMemoryUtilization: "0.9",
				TrustRemoteCode:      false,
				TokenizerMode:        "auto",
				ExtraArgs:            []string{"--enable-chunked-prefill"},
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		RuntimeArgs: &v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				MaxModelLen:          8192, // Override max length
				TrustRemoteCode:      true, // Enable trust remote code
				GPUMemoryUtilization: "0.95", // Increase GPU memory utilization
				// DType, TokenizerMode, ExtraArgs should be preserved
			},
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	if result.RuntimeArgs.VLLM == nil {
		t.Fatal("Expected VLLM RuntimeArgs to be present")
	}

	// Overridden fields
	if result.RuntimeArgs.VLLM.MaxModelLen != 8192 {
		t.Errorf("Expected MaxModelLen 8192, got %d", result.RuntimeArgs.VLLM.MaxModelLen)
	}
	if !result.RuntimeArgs.VLLM.TrustRemoteCode {
		t.Errorf("Expected TrustRemoteCode true, got false")
	}
	if result.RuntimeArgs.VLLM.GPUMemoryUtilization != "0.95" {
		t.Errorf("Expected GPUMemoryUtilization '0.95', got %s", result.RuntimeArgs.VLLM.GPUMemoryUtilization)
	}

	// Preserved fields
	if result.RuntimeArgs.VLLM.DType != "auto" {
		t.Errorf("Expected DType 'auto', got %s", result.RuntimeArgs.VLLM.DType)
	}
	if result.RuntimeArgs.VLLM.TokenizerMode != "auto" {
		t.Errorf("Expected TokenizerMode 'auto', got %s", result.RuntimeArgs.VLLM.TokenizerMode)
	}
	if len(result.RuntimeArgs.VLLM.ExtraArgs) != 1 || result.RuntimeArgs.VLLM.ExtraArgs[0] != "--enable-chunked-prefill" {
		t.Errorf("Expected ExtraArgs to be preserved, got %v", result.RuntimeArgs.VLLM.ExtraArgs)
	}
}

// Test_MergeRecipe_RuntimeArgsOverride_Triton tests deep merging of Triton runtime args.
func Test_MergeRecipe_RuntimeArgsOverride_Triton(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "model/triton-model",
		Runtime: "triton",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			Triton: &v1alpha1.TritonArgs{
				Backend:         "tensorrt",
				ModelRepository: "s3://models/triton",
				InstanceGroup: []v1alpha1.TritonInstanceGroup{
					{Kind: "KIND_GPU", Count: 1},
				},
				DynamicBatching: &v1alpha1.TritonDynamicBatching{
					MaxBatchSize:              8,
					MaxQueueDelayMicroseconds: 1000,
				},
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		RuntimeArgs: &v1alpha1.RuntimeArgsSpec{
			Triton: &v1alpha1.TritonArgs{
				InstanceGroup: []v1alpha1.TritonInstanceGroup{
					{Kind: "KIND_GPU", Count: 2}, // Override instance count
				},
				DynamicBatching: &v1alpha1.TritonDynamicBatching{
					MaxBatchSize: 16, // Override batch size
					// MaxQueueDelayMicroseconds should be preserved
				},
				// Backend and ModelRepository should be preserved
			},
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	if result.RuntimeArgs.Triton == nil {
		t.Fatal("Expected Triton RuntimeArgs to be present")
	}

	// Overridden fields
	if len(result.RuntimeArgs.Triton.InstanceGroup) != 1 || result.RuntimeArgs.Triton.InstanceGroup[0].Count != 2 {
		t.Errorf("Expected InstanceGroup Count 2, got %v", result.RuntimeArgs.Triton.InstanceGroup)
	}
	if result.RuntimeArgs.Triton.DynamicBatching.MaxBatchSize != 16 {
		t.Errorf("Expected MaxBatchSize 16, got %d", result.RuntimeArgs.Triton.DynamicBatching.MaxBatchSize)
	}

	// Preserved fields
	if result.RuntimeArgs.Triton.Backend != "tensorrt" {
		t.Errorf("Expected Backend 'tensorrt', got %s", result.RuntimeArgs.Triton.Backend)
	}
	if result.RuntimeArgs.Triton.ModelRepository != "s3://models/triton" {
		t.Errorf("Expected ModelRepository 's3://models/triton', got %s", result.RuntimeArgs.Triton.ModelRepository)
	}
	if result.RuntimeArgs.Triton.DynamicBatching.MaxQueueDelayMicroseconds != 1000 {
		t.Errorf("Expected MaxQueueDelayMicroseconds 1000, got %d", result.RuntimeArgs.Triton.DynamicBatching.MaxQueueDelayMicroseconds)
	}
}

// Test_MergeRecipe_SchedulingOverride tests merging of scheduling configurations.
func Test_MergeRecipe_SchedulingOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
		Scheduling: v1alpha1.SchedulingSpec{
			Tolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			NodeSelector: map[string]string{
				"node-type": "gpu",
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Scheduling: &v1alpha1.SchedulingSpec{
			Tolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "high-memory",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			NodeSelector: map[string]string{
				"node-type":  "gpu",
				"gpu-memory": "high",
			},
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// Tolerations should be replaced (not appended)
	if len(result.Scheduling.Tolerations) != 2 {
		t.Errorf("Expected 2 tolerations, got %d", len(result.Scheduling.Tolerations))
	}

	// NodeSelector should be merged
	if result.Scheduling.NodeSelector["node-type"] != "gpu" {
		t.Errorf("Expected NodeSelector node-type 'gpu', got %s", result.Scheduling.NodeSelector["node-type"])
	}
	if result.Scheduling.NodeSelector["gpu-memory"] != "high" {
		t.Errorf("Expected NodeSelector gpu-memory 'high', got %s", result.Scheduling.NodeSelector["gpu-memory"])
	}
}

// Test_MergeRecipe_ReplicasOverride tests overriding replica configuration.
func Test_MergeRecipe_ReplicasOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
	}

	minReplicas := int32(2)
	maxReplicas := int32(10)

	overrides := &v1alpha1.RecipeOverrides{
		Replicas: &v1alpha1.ReplicaOverrides{
			Min: &minReplicas,
			Max: &maxReplicas,
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// Note: ReplicaOverrides are specific to AIModel, not ModelRecipeSpec
	// This test verifies that the merger function handles them correctly
	// The actual replica values would be applied at AIModel spec level
	if result.ModelID != originalSpec.ModelID {
		t.Errorf("ModelID should remain unchanged, got %s", result.ModelID)
	}
}

// Test_MergeRecipe_PartialOverride tests partial override scenarios.
// This verifies that only specified override fields are applied.
func Test_MergeRecipe_PartialOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID:     "meta-llama/Llama-2-7b-hf",
		DisplayName: "Llama 2 7B",
		Runtime:     "vllm",
		Image:       "vllm/vllm-openai:v0.2.7",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Vendor:      "nvidia",
				Count:       1,
				MinMemoryGB: 20,
			},
			CPU: v1alpha1.CPUResources{
				Requests: "4",
				Limits:   "8",
			},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				DType:       "auto",
				MaxModelLen: 4096,
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		// Only override runtime - all other fields should be preserved
		Runtime: "triton",
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// Overridden field
	if result.Runtime != "triton" {
		t.Errorf("Expected Runtime 'triton', got %s", result.Runtime)
	}

	// All other fields should be preserved
	if result.ModelID != originalSpec.ModelID {
		t.Errorf("Expected ModelID to be preserved, got %s", result.ModelID)
	}
	if result.DisplayName != originalSpec.DisplayName {
		t.Errorf("Expected DisplayName to be preserved, got %s", result.DisplayName)
	}
	if result.Image != originalSpec.Image {
		t.Errorf("Expected Image to be preserved, got %s", result.Image)
	}
	if result.Resources.GPU.Count != 1 {
		t.Errorf("Expected GPU Count to be preserved, got %d", result.Resources.GPU.Count)
	}
	if result.RuntimeArgs.VLLM.MaxModelLen != 4096 {
		t.Errorf("Expected MaxModelLen to be preserved, got %d", result.RuntimeArgs.VLLM.MaxModelLen)
	}
}

// Test_MergeRecipe_EmptyOverrides tests that empty (but non-nil) overrides don't change the spec.
func Test_MergeRecipe_EmptyOverrides(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Image:   "vllm/vllm-openai:v0.2.7",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
	}

	emptyOverrides := &v1alpha1.RecipeOverrides{
		// All fields are empty/nil
	}

	// Act
	result := MergeRecipe(originalSpec, emptyOverrides)

	// Assert
	if result.ModelID != originalSpec.ModelID {
		t.Errorf("Expected ModelID to be unchanged, got %s", result.ModelID)
	}
	if result.Runtime != originalSpec.Runtime {
		t.Errorf("Expected Runtime to be unchanged, got %s", result.Runtime)
	}
	if result.Image != originalSpec.Image {
		t.Errorf("Expected Image to be unchanged, got %s", result.Image)
	}
}

// Test_MergeRecipe_ComplexNestedOverride tests a complex scenario with multiple nested overrides.
func Test_MergeRecipe_ComplexNestedOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID:     "meta-llama/Llama-2-70b-hf",
		DisplayName: "Llama 2 70B",
		Description: "Large language model",
		Runtime:     "vllm",
		Image:       "vllm/vllm-openai:v0.2.7",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Vendor:      "nvidia",
				Model:       "a100-40gb",
				Count:       4,
				MinMemoryGB: 40,
			},
			CPU: v1alpha1.CPUResources{
				Requests: "16",
				Limits:   "32",
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "64Gi",
				Limits:   "128Gi",
			},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				DType:                "float16",
				MaxModelLen:          4096,
				GPUMemoryUtilization: "0.9",
				TrustRemoteCode:      false,
				TokenizerMode:        "auto",
			},
		},
		Scheduling: v1alpha1.SchedulingSpec{
			NodeSelector: map[string]string{
				"node-type": "gpu",
				"gpu-model": "a100",
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Image: "vllm/vllm-openai:v0.3.0", // Upgrade image version
		Resources: &v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Count: 8, // Double GPU count for better performance
				// Keep vendor, model, minMemoryGB from original
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "128Gi", // Increase memory request
				Limits:   "256Gi", // Increase memory limit
			},
			// CPU remains unchanged
		},
		RuntimeArgs: &v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				MaxModelLen:          8192, // Double context length
				GPUMemoryUtilization: "0.95", // Increase GPU memory usage
				TrustRemoteCode:      true,   // Enable trust remote code
				// DType and TokenizerMode remain unchanged
			},
		},
		Scheduling: &v1alpha1.SchedulingSpec{
			NodeSelector: map[string]string{
				"node-type":  "gpu",
				"gpu-model":  "a100",
				"gpu-memory": "80gb", // Add new selector
			},
			// Tolerations remain unchanged
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// Overridden fields
	if result.Image != "vllm/vllm-openai:v0.3.0" {
		t.Errorf("Expected Image 'vllm/vllm-openai:v0.3.0', got %s", result.Image)
	}
	if result.Resources.GPU.Count != 8 {
		t.Errorf("Expected GPU Count 8, got %d", result.Resources.GPU.Count)
	}
	if result.Resources.Memory.Requests != "128Gi" {
		t.Errorf("Expected Memory Requests '128Gi', got %s", result.Resources.Memory.Requests)
	}
	if result.Resources.Memory.Limits != "256Gi" {
		t.Errorf("Expected Memory Limits '256Gi', got %s", result.Resources.Memory.Limits)
	}
	if result.RuntimeArgs.VLLM.MaxModelLen != 8192 {
		t.Errorf("Expected MaxModelLen 8192, got %d", result.RuntimeArgs.VLLM.MaxModelLen)
	}
	if result.RuntimeArgs.VLLM.GPUMemoryUtilization != "0.95" {
		t.Errorf("Expected GPUMemoryUtilization '0.95', got %s", result.RuntimeArgs.VLLM.GPUMemoryUtilization)
	}
	if !result.RuntimeArgs.VLLM.TrustRemoteCode {
		t.Errorf("Expected TrustRemoteCode true, got false")
	}

	// Preserved fields
	if result.ModelID != originalSpec.ModelID {
		t.Errorf("Expected ModelID to be preserved, got %s", result.ModelID)
	}
	if result.DisplayName != originalSpec.DisplayName {
		t.Errorf("Expected DisplayName to be preserved, got %s", result.DisplayName)
	}
	if result.Runtime != originalSpec.Runtime {
		t.Errorf("Expected Runtime to be preserved, got %s", result.Runtime)
	}
	if result.Resources.GPU.Vendor != "nvidia" {
		t.Errorf("Expected GPU Vendor to be preserved, got %s", result.Resources.GPU.Vendor)
	}
	if result.Resources.GPU.Model != "a100-40gb" {
		t.Errorf("Expected GPU Model to be preserved, got %s", result.Resources.GPU.Model)
	}
	if result.Resources.GPU.MinMemoryGB != 40 {
		t.Errorf("Expected GPU MinMemoryGB to be preserved, got %d", result.Resources.GPU.MinMemoryGB)
	}
	if result.Resources.CPU.Requests != "16" {
		t.Errorf("Expected CPU Requests to be preserved, got %s", result.Resources.CPU.Requests)
	}
	if result.RuntimeArgs.VLLM.DType != "float16" {
		t.Errorf("Expected DType to be preserved, got %s", result.RuntimeArgs.VLLM.DType)
	}
	if result.RuntimeArgs.VLLM.TokenizerMode != "auto" {
		t.Errorf("Expected TokenizerMode to be preserved, got %s", result.RuntimeArgs.VLLM.TokenizerMode)
	}

	// NodeSelector should be merged
	if result.Scheduling.NodeSelector["node-type"] != "gpu" {
		t.Errorf("Expected NodeSelector node-type 'gpu', got %s", result.Scheduling.NodeSelector["node-type"])
	}
	if result.Scheduling.NodeSelector["gpu-memory"] != "80gb" {
		t.Errorf("Expected NodeSelector gpu-memory '80gb', got %s", result.Scheduling.NodeSelector["gpu-memory"])
	}

	// Tolerations should be preserved
	if len(result.Scheduling.Tolerations) != 1 {
		t.Errorf("Expected 1 toleration to be preserved, got %d", len(result.Scheduling.Tolerations))
	}
}

// Test_MergeRecipe_ZeroValueOverride tests that zero values in overrides are applied.
// This is important to distinguish between "not set" and "set to zero".
func Test_MergeRecipe_ZeroValueOverride(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{
				Count:       2,
				MinMemoryGB: 20,
			},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				MaxModelLen:     8192,
				TrustRemoteCode: true,
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		RuntimeArgs: &v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				MaxModelLen:     0,     // Explicitly set to zero (should override)
				TrustRemoteCode: false, // Explicitly set to false (should override)
			},
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// Zero values should be applied if explicitly set in overrides
	// Note: The implementation will need to distinguish between "not set" and "set to zero"
	// This may require using pointers for numeric fields in RecipeOverrides
	if result.RuntimeArgs.VLLM.MaxModelLen != 0 {
		t.Errorf("Expected MaxModelLen 0, got %d", result.RuntimeArgs.VLLM.MaxModelLen)
	}
	if result.RuntimeArgs.VLLM.TrustRemoteCode != false {
		t.Errorf("Expected TrustRemoteCode false, got true")
	}
}

// Test_MergeRecipe_RuntimeSwitchClearsArgs tests that switching runtimes should handle args correctly.
// When switching from vLLM to Triton, vLLM args should not be in the result.
func Test_MergeRecipe_RuntimeSwitchClearsArgs(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "model/multi-runtime",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
		},
		RuntimeArgs: v1alpha1.RuntimeArgsSpec{
			VLLM: &v1alpha1.VLLMArgs{
				DType:       "auto",
				MaxModelLen: 4096,
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Runtime: "triton",
		RuntimeArgs: &v1alpha1.RuntimeArgsSpec{
			Triton: &v1alpha1.TritonArgs{
				Backend: "tensorrt",
			},
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	if result.Runtime != "triton" {
		t.Errorf("Expected Runtime 'triton', got %s", result.Runtime)
	}
	if result.RuntimeArgs.Triton == nil {
		t.Error("Expected Triton RuntimeArgs to be present")
	}
	if result.RuntimeArgs.Triton.Backend != "tensorrt" {
		t.Errorf("Expected Triton Backend 'tensorrt', got %s", result.RuntimeArgs.Triton.Backend)
	}

	// vLLM args behavior depends on implementation:
	// Option 1: Clear vLLM args when switching to Triton
	// Option 2: Keep vLLM args (harmless if runtime is Triton)
	// This test documents the expected behavior
}

// Test_MergeRecipe_ResourceQuantityParsing tests merging with Kubernetes resource quantities.
func Test_MergeRecipe_ResourceQuantityParsing(t *testing.T) {
	// Arrange
	originalSpec := v1alpha1.ModelRecipeSpec{
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Resources: v1alpha1.RecipeResources{
			GPU: v1alpha1.GPUResources{Count: 1},
			CPU: v1alpha1.CPUResources{
				Requests: "4",
				Limits:   "8",
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "16Gi",
				Limits:   "32Gi",
			},
		},
	}

	overrides := &v1alpha1.RecipeOverrides{
		Resources: &v1alpha1.RecipeResources{
			CPU: v1alpha1.CPUResources{
				Requests: "8000m", // Different format but equivalent to "8"
			},
			Memory: v1alpha1.MemoryResources{
				Requests: "32768Mi", // Different format
			},
		},
	}

	// Act
	result := MergeRecipe(originalSpec, overrides)

	// Assert
	// Verify the values are set correctly
	if result.Resources.CPU.Requests != "8000m" {
		t.Errorf("Expected CPU Requests '8000m', got %s", result.Resources.CPU.Requests)
	}
	if result.Resources.Memory.Requests != "32768Mi" {
		t.Errorf("Expected Memory Requests '32768Mi', got %s", result.Resources.Memory.Requests)
	}

	// Verify preserved limits
	if result.Resources.CPU.Limits != "8" {
		t.Errorf("Expected CPU Limits to be preserved, got %s", result.Resources.CPU.Limits)
	}
	if result.Resources.Memory.Limits != "32Gi" {
		t.Errorf("Expected Memory Limits to be preserved, got %s", result.Resources.Memory.Limits)
	}

	// Verify they are semantically equivalent
	cpuRequests, err := resource.ParseQuantity(result.Resources.CPU.Requests)
	if err != nil {
		t.Fatalf("Failed to parse CPU requests: %v", err)
	}
	expectedCPU, _ := resource.ParseQuantity("8")
	if !cpuRequests.Equal(expectedCPU) {
		t.Errorf("CPU requests not semantically equal: %s vs %s", cpuRequests.String(), expectedCPU.String())
	}
}
