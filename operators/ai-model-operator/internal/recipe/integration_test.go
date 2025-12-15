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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

// TestIntegration_FullResolutionFlow tests the complete recipe resolution flow
// including resolving the recipe, merging overrides, and validating the result
func TestIntegration_FullResolutionFlow(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create a ModelRecipe
	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b-vllm",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-7b-hf",
			DisplayName: "Llama 2 7B",
			Description: "Llama 2 7B model with vLLM runtime",
			Runtime:     "vllm",
			Image:       "vllm/vllm-openai:latest",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor:      "nvidia",
					Model:       "rtx4000-ada",
					Count:       1,
					MinMemoryGB: 16,
				},
				CPU: aimodelv1alpha1.CPUResources{
					Requests: "4",
					Limits:   "8",
				},
				Memory: aimodelv1alpha1.MemoryResources{
					Requests: "8Gi",
					Limits:   "16Gi",
				},
			},
			RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
				VLLM: &aimodelv1alpha1.VLLMArgs{
					DType:                "auto",
					MaxModelLen:          4096,
					GPUMemoryUtilization: "0.9",
					TrustRemoteCode:      false,
				},
			},
		},
	}

	// Create fake client with the recipe
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	// Create resolver and validator
	resolver := NewResolver(fakeClient)
	validator := NewValidator()

	// Create recipe reference (as AIModel would have)
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b-vllm",
		Namespace: "ai-model-system",
	}

	// No overrides for this test
	var overrides *aimodelv1alpha1.RecipeOverrides = nil

	// Execute: Resolve recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)
	require.NoError(t, err)
	require.NotNil(t, resolvedRecipe)

	// Execute: Merge with overrides (nil in this case)
	mergedSpec := MergeRecipe(resolvedRecipe.Spec, overrides)

	// Execute: Validate merged spec
	validationResult := validator.Validate(&mergedSpec)

	// Assert: Validation should pass
	assert.True(t, validationResult.Valid, "Validation should pass for merged spec")
	assert.Empty(t, validationResult.Errors)

	// Assert: Merged spec should match original recipe (no overrides applied)
	assert.Equal(t, "meta-llama/Llama-2-7b-hf", mergedSpec.ModelID)
	assert.Equal(t, "vllm", mergedSpec.Runtime)
	assert.Equal(t, "vllm/vllm-openai:latest", mergedSpec.Image)
	assert.Equal(t, int32(1), mergedSpec.Resources.GPU.Count)
	assert.Equal(t, "nvidia", mergedSpec.Resources.GPU.Vendor)
	assert.Equal(t, "4", mergedSpec.Resources.CPU.Requests)
	assert.Equal(t, "8Gi", mergedSpec.Resources.Memory.Requests)
	assert.NotNil(t, mergedSpec.RuntimeArgs.VLLM)
	assert.Equal(t, "auto", mergedSpec.RuntimeArgs.VLLM.DType)
	assert.Equal(t, int32(4096), mergedSpec.RuntimeArgs.VLLM.MaxModelLen)
}

// TestIntegration_OverrideApplication tests that overrides are correctly applied
// and merged with the base recipe
func TestIntegration_OverrideApplication(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create a base ModelRecipe
	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b-vllm",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-7b-hf",
			DisplayName: "Llama 2 7B",
			Runtime:     "vllm",
			Image:       "vllm/vllm-openai:v0.2.0",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor:      "nvidia",
					Count:       1,
					MinMemoryGB: 16,
				},
				CPU: aimodelv1alpha1.CPUResources{
					Requests: "4",
					Limits:   "8",
				},
				Memory: aimodelv1alpha1.MemoryResources{
					Requests: "8Gi",
					Limits:   "16Gi",
				},
			},
			RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
				VLLM: &aimodelv1alpha1.VLLMArgs{
					DType:                "auto",
					MaxModelLen:          4096,
					GPUMemoryUtilization: "0.9",
				},
			},
		},
	}

	// Create fake client with the recipe
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	// Create resolver and validator
	resolver := NewResolver(fakeClient)
	validator := NewValidator()

	// Create recipe reference
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b-vllm",
		Namespace: "ai-model-system",
	}

	// Define overrides (as AIModel would specify)
	overrides := &aimodelv1alpha1.RecipeOverrides{
		Image: "vllm/vllm-openai:v0.3.0", // Override image version
		Resources: &aimodelv1alpha1.RecipeResources{
			GPU: aimodelv1alpha1.GPUResources{
				Count: 2, // Override GPU count
			},
			Memory: aimodelv1alpha1.MemoryResources{
				Requests: "16Gi", // Override memory request
			},
		},
		RuntimeArgs: &aimodelv1alpha1.RuntimeArgsSpec{
			VLLM: &aimodelv1alpha1.VLLMArgs{
				MaxModelLen: 8192, // Override max model length
			},
		},
	}

	// Execute: Resolve recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)
	require.NoError(t, err)
	require.NotNil(t, resolvedRecipe)

	// Execute: Merge with overrides
	mergedSpec := MergeRecipe(resolvedRecipe.Spec, overrides)

	// Execute: Validate merged spec
	validationResult := validator.Validate(&mergedSpec)

	// Assert: Validation should pass
	assert.True(t, validationResult.Valid, "Validation should pass for merged spec with overrides")
	assert.Empty(t, validationResult.Errors)

	// Assert: Override values should take precedence
	assert.Equal(t, "vllm/vllm-openai:v0.3.0", mergedSpec.Image, "Image should be overridden")
	assert.Equal(t, int32(2), mergedSpec.Resources.GPU.Count, "GPU count should be overridden")
	assert.Equal(t, "16Gi", mergedSpec.Resources.Memory.Requests, "Memory requests should be overridden")
	assert.Equal(t, int32(8192), mergedSpec.RuntimeArgs.VLLM.MaxModelLen, "MaxModelLen should be overridden")

	// Assert: Non-overridden values should be preserved from recipe
	assert.Equal(t, "meta-llama/Llama-2-7b-hf", mergedSpec.ModelID, "ModelID should be preserved")
	assert.Equal(t, "vllm", mergedSpec.Runtime, "Runtime should be preserved")
	assert.Equal(t, "nvidia", mergedSpec.Resources.GPU.Vendor, "GPU vendor should be preserved")
	assert.Equal(t, "4", mergedSpec.Resources.CPU.Requests, "CPU requests should be preserved")
	assert.Equal(t, "16Gi", mergedSpec.Resources.Memory.Limits, "Memory limits should be preserved")
	assert.Equal(t, "auto", mergedSpec.RuntimeArgs.VLLM.DType, "DType should be preserved")
	assert.Equal(t, "0.9", mergedSpec.RuntimeArgs.VLLM.GPUMemoryUtilization, "GPU memory utilization should be preserved")
}

// TestIntegration_ValidationCatchesErrors tests that validation catches errors
// in the merged specification
func TestIntegration_ValidationCatchesErrors(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create a ModelRecipe with valid base configuration
	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b-vllm",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID: "meta-llama/Llama-2-7b-hf",
			Runtime: "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  1,
				},
			},
			RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
				VLLM: &aimodelv1alpha1.VLLMArgs{
					DType: "auto",
				},
			},
		},
	}

	// Create fake client with the recipe
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	// Create resolver and validator
	resolver := NewResolver(fakeClient)
	validator := NewValidator()

	// Create recipe reference
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b-vllm",
		Namespace: "ai-model-system",
	}

	// Define invalid overrides (invalid GPU vendor)
	overrides := &aimodelv1alpha1.RecipeOverrides{
		Resources: &aimodelv1alpha1.RecipeResources{
			GPU: aimodelv1alpha1.GPUResources{
				Vendor: "invalid-vendor", // Invalid: must be nvidia, amd, or intel
			},
		},
	}

	// Execute: Resolve recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)
	require.NoError(t, err)
	require.NotNil(t, resolvedRecipe)

	// Execute: Merge with invalid overrides
	mergedSpec := MergeRecipe(resolvedRecipe.Spec, overrides)

	// Execute: Validate merged spec
	validationResult := validator.Validate(&mergedSpec)

	// Assert: Validation should fail
	assert.False(t, validationResult.Valid, "Validation should fail for invalid GPU vendor")
	assert.NotEmpty(t, validationResult.Errors)
	assert.Contains(t, validationResult.ErrorString(), "gpu.vendor must be one of")
}

// TestIntegration_MissingRecipe tests error handling when recipe doesn't exist
func TestIntegration_MissingRecipe(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create fake client WITHOUT any recipes
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Create resolver
	resolver := NewResolver(fakeClient)

	// Create recipe reference for non-existent recipe
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "nonexistent-recipe",
		Namespace: "ai-model-system",
	}

	// Execute: Attempt to resolve non-existent recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert: Should return error
	require.Error(t, err)
	assert.Nil(t, resolvedRecipe)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent-recipe")
}

// TestIntegration_ComplexOverrideScenario tests a complex scenario with multiple
// levels of overrides including runtime args and scheduling
func TestIntegration_ComplexOverrideScenario(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create a comprehensive ModelRecipe
	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-70b-vllm",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-70b-hf",
			DisplayName: "Llama 2 70B",
			Runtime:     "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor:      "nvidia",
					Model:       "a100-40gb",
					Count:       4,
					MinMemoryGB: 40,
				},
				CPU: aimodelv1alpha1.CPUResources{
					Requests: "16",
					Limits:   "32",
				},
				Memory: aimodelv1alpha1.MemoryResources{
					Requests: "64Gi",
					Limits:   "128Gi",
				},
			},
			RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
				VLLM: &aimodelv1alpha1.VLLMArgs{
					DType:                "bfloat16",
					MaxModelLen:          4096,
					GPUMemoryUtilization: "0.95",
					TrustRemoteCode:      false,
					TokenizerMode:        "auto",
				},
			},
			Scheduling: aimodelv1alpha1.SchedulingSpec{
				NodeSelector: map[string]string{
					"gpu-type": "a100",
				},
			},
		},
	}

	// Create fake client with the recipe
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	// Create resolver and validator
	resolver := NewResolver(fakeClient)
	validator := NewValidator()

	// Create recipe reference
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-70b-vllm",
		Namespace: "ai-model-system",
	}

	// Define complex overrides
	overrides := &aimodelv1alpha1.RecipeOverrides{
		Resources: &aimodelv1alpha1.RecipeResources{
			GPU: aimodelv1alpha1.GPUResources{
				Count: 8, // Scale up to 8 GPUs
			},
			Memory: aimodelv1alpha1.MemoryResources{
				Requests: "128Gi", // Increase memory
				Limits:   "256Gi",
			},
		},
		RuntimeArgs: &aimodelv1alpha1.RuntimeArgsSpec{
			VLLM: &aimodelv1alpha1.VLLMArgs{
				MaxModelLen:          8192,  // Increase context length
				GPUMemoryUtilization: "0.90", // Reduce GPU memory utilization
			},
		},
		Scheduling: &aimodelv1alpha1.SchedulingSpec{
			NodeSelector: map[string]string{
				"gpu-type":     "a100", // Preserved
				"storage-type": "nvme", // Added
			},
		},
	}

	// Execute: Resolve recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)
	require.NoError(t, err)
	require.NotNil(t, resolvedRecipe)

	// Execute: Merge with overrides
	mergedSpec := MergeRecipe(resolvedRecipe.Spec, overrides)

	// Execute: Validate merged spec
	validationResult := validator.Validate(&mergedSpec)

	// Assert: Validation should pass
	assert.True(t, validationResult.Valid, "Validation should pass for complex merged spec")
	assert.Empty(t, validationResult.Errors)

	// Assert: Override values are applied
	assert.Equal(t, int32(8), mergedSpec.Resources.GPU.Count)
	assert.Equal(t, "128Gi", mergedSpec.Resources.Memory.Requests)
	assert.Equal(t, "256Gi", mergedSpec.Resources.Memory.Limits)
	assert.Equal(t, int32(8192), mergedSpec.RuntimeArgs.VLLM.MaxModelLen)
	assert.Equal(t, "0.90", mergedSpec.RuntimeArgs.VLLM.GPUMemoryUtilization)

	// Assert: Non-overridden values are preserved
	assert.Equal(t, "meta-llama/Llama-2-70b-hf", mergedSpec.ModelID)
	assert.Equal(t, "nvidia", mergedSpec.Resources.GPU.Vendor)
	assert.Equal(t, "a100-40gb", mergedSpec.Resources.GPU.Model)
	assert.Equal(t, "16", mergedSpec.Resources.CPU.Requests)
	assert.Equal(t, "bfloat16", mergedSpec.RuntimeArgs.VLLM.DType)
	assert.Equal(t, "auto", mergedSpec.RuntimeArgs.VLLM.TokenizerMode)

	// Assert: NodeSelector is merged (not replaced)
	assert.Equal(t, 2, len(mergedSpec.Scheduling.NodeSelector))
	assert.Equal(t, "a100", mergedSpec.Scheduling.NodeSelector["gpu-type"])
	assert.Equal(t, "nvme", mergedSpec.Scheduling.NodeSelector["storage-type"])
}

// TestIntegration_InvalidRuntimeOverride tests validation failure when
// runtime args don't match the specified runtime
func TestIntegration_InvalidRuntimeOverride(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create a vLLM ModelRecipe
	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b-vllm",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID: "meta-llama/Llama-2-7b-hf",
			Runtime: "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  1,
				},
			},
			RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
				VLLM: &aimodelv1alpha1.VLLMArgs{
					DType: "auto",
				},
			},
		},
	}

	// Create fake client with the recipe
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	// Create resolver and validator
	resolver := NewResolver(fakeClient)
	validator := NewValidator()

	// Create recipe reference
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b-vllm",
		Namespace: "ai-model-system",
	}

	// Define invalid overrides (adding TGI args when runtime is vLLM)
	overrides := &aimodelv1alpha1.RecipeOverrides{
		RuntimeArgs: &aimodelv1alpha1.RuntimeArgsSpec{
			TGI: &aimodelv1alpha1.TGIArgs{
				Quantize: "gptq",
			},
		},
	}

	// Execute: Resolve recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)
	require.NoError(t, err)
	require.NotNil(t, resolvedRecipe)

	// Execute: Merge with invalid overrides
	mergedSpec := MergeRecipe(resolvedRecipe.Spec, overrides)

	// Execute: Validate merged spec
	validationResult := validator.Validate(&mergedSpec)

	// Assert: Validation should fail due to runtime mismatch
	assert.False(t, validationResult.Valid, "Validation should fail for runtime args mismatch")
	assert.NotEmpty(t, validationResult.Errors)
	assert.Contains(t, validationResult.ErrorString(), "should not be set when runtime is vllm")
}

// TestIntegration_TritonRecipeWithOverrides tests recipe resolution with
// Triton-specific configuration
func TestIntegration_TritonRecipeWithOverrides(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	// Create a Triton ModelRecipe
	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gpt-triton",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID: "gpt2",
			Runtime: "triton",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  1,
				},
			},
			RuntimeArgs: aimodelv1alpha1.RuntimeArgsSpec{
				Triton: &aimodelv1alpha1.TritonArgs{
					Backend:         "python",
					ModelRepository: "s3://models/gpt2",
					DynamicBatching: &aimodelv1alpha1.TritonDynamicBatching{
						MaxBatchSize:              8,
						MaxQueueDelayMicroseconds: 100,
					},
				},
			},
		},
	}

	// Create fake client with the recipe
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	// Create resolver and validator
	resolver := NewResolver(fakeClient)
	validator := NewValidator()

	// Create recipe reference
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "gpt-triton",
		Namespace: "ai-model-system",
	}

	// Define overrides for Triton
	overrides := &aimodelv1alpha1.RecipeOverrides{
		RuntimeArgs: &aimodelv1alpha1.RuntimeArgsSpec{
			Triton: &aimodelv1alpha1.TritonArgs{
				DynamicBatching: &aimodelv1alpha1.TritonDynamicBatching{
					MaxBatchSize: 16, // Override batch size
				},
			},
		},
	}

	// Execute: Resolve recipe
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)
	require.NoError(t, err)
	require.NotNil(t, resolvedRecipe)

	// Execute: Merge with overrides
	mergedSpec := MergeRecipe(resolvedRecipe.Spec, overrides)

	// Execute: Validate merged spec
	validationResult := validator.Validate(&mergedSpec)

	// Assert: Validation should pass
	assert.True(t, validationResult.Valid, "Validation should pass for Triton recipe")
	assert.Empty(t, validationResult.Errors)

	// Assert: Override is applied
	assert.Equal(t, int32(16), mergedSpec.RuntimeArgs.Triton.DynamicBatching.MaxBatchSize)

	// Assert: Non-overridden values are preserved
	assert.Equal(t, "python", mergedSpec.RuntimeArgs.Triton.Backend)
	assert.Equal(t, "s3://models/gpt2", mergedSpec.RuntimeArgs.Triton.ModelRepository)
	assert.Equal(t, int64(100), mergedSpec.RuntimeArgs.Triton.DynamicBatching.MaxQueueDelayMicroseconds)
}
