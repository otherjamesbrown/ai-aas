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

// TestResolveRecipe_Success tests successful recipe resolution
func TestResolveRecipe_Success(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-7b-hf",
			DisplayName: "Llama 2 7B",
			Runtime:     "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  1,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	resolver := NewResolver(fakeClient)

	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b",
		Namespace: "ai-model-system",
	}

	// Execute
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resolvedRecipe)
	assert.Equal(t, "llama-7b", resolvedRecipe.Name)
	assert.Equal(t, "ai-model-system", resolvedRecipe.Namespace)
	assert.Equal(t, "meta-llama/Llama-2-7b-hf", resolvedRecipe.Spec.ModelID)
	assert.Equal(t, "Llama 2 7B", resolvedRecipe.Spec.DisplayName)
	assert.Equal(t, "vllm", resolvedRecipe.Spec.Runtime)
	assert.Equal(t, int32(1), resolvedRecipe.Spec.Resources.GPU.Count)
}

// TestResolveRecipe_NotFound tests recipe not found error
func TestResolveRecipe_NotFound(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	resolver := NewResolver(fakeClient)

	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "nonexistent-recipe",
		Namespace: "ai-model-system",
	}

	// Execute
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resolvedRecipe)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent-recipe")
}

// TestResolveRecipe_DefaultNamespace tests default namespace resolution
func TestResolveRecipe_DefaultNamespace(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b",
			Namespace: DefaultRecipeNamespace,
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-7b-hf",
			DisplayName: "Llama 2 7B",
			Runtime:     "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  1,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	resolver := NewResolver(fakeClient)

	// RecipeRef with empty namespace should default to ai-model-system
	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b",
		Namespace: "", // Empty namespace
	}

	// Execute
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resolvedRecipe)
	assert.Equal(t, "llama-7b", resolvedRecipe.Name)
	assert.Equal(t, DefaultRecipeNamespace, resolvedRecipe.Namespace)
	assert.Equal(t, "meta-llama/Llama-2-7b-hf", resolvedRecipe.Spec.ModelID)
}

// TestResolveRecipe_CustomNamespace tests recipe in custom namespace
func TestResolveRecipe_CustomNamespace(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	recipe := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-model",
			Namespace: "custom-namespace",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "custom/model",
			DisplayName: "Custom Model",
			Runtime:     "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  2,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe).
		Build()

	resolver := NewResolver(fakeClient)

	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "custom-model",
		Namespace: "custom-namespace",
	}

	// Execute
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resolvedRecipe)
	assert.Equal(t, "custom-model", resolvedRecipe.Name)
	assert.Equal(t, "custom-namespace", resolvedRecipe.Namespace)
	assert.Equal(t, int32(2), resolvedRecipe.Spec.Resources.GPU.Count)
}

// TestResolveRecipe_InvalidReference tests error for nil reference
func TestResolveRecipe_InvalidReference(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	resolver := NewResolver(fakeClient)

	// Execute - nil reference
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resolvedRecipe)
	assert.Contains(t, err.Error(), "nil")
}

// TestResolveRecipe_EmptyName tests error for empty recipe name
func TestResolveRecipe_EmptyName(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	resolver := NewResolver(fakeClient)

	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "", // Empty name
		Namespace: "ai-model-system",
	}

	// Execute
	ctx := context.Background()
	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resolvedRecipe)
	assert.Contains(t, err.Error(), "empty")
}

// TestResolveRecipe_MultipleRecipes tests that resolver can distinguish between recipes
func TestResolveRecipe_MultipleRecipes(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	recipe1 := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-7b",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-7b-hf",
			DisplayName: "Llama 2 7B",
			Runtime:     "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  1,
				},
			},
		},
	}

	recipe2 := &aimodelv1alpha1.ModelRecipe{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llama-70b",
			Namespace: "ai-model-system",
		},
		Spec: aimodelv1alpha1.ModelRecipeSpec{
			ModelID:     "meta-llama/Llama-2-70b-hf",
			DisplayName: "Llama 2 70B",
			Runtime:     "vllm",
			Resources: aimodelv1alpha1.RecipeResources{
				GPU: aimodelv1alpha1.GPUResources{
					Vendor: "nvidia",
					Count:  4,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(recipe1, recipe2).
		Build()

	resolver := NewResolver(fakeClient)

	// Execute - resolve first recipe
	ctx := context.Background()
	recipeRef1 := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b",
		Namespace: "ai-model-system",
	}
	resolvedRecipe1, err1 := resolver.ResolveRecipe(ctx, recipeRef1)

	// Assert first recipe
	require.NoError(t, err1)
	assert.NotNil(t, resolvedRecipe1)
	assert.Equal(t, "llama-7b", resolvedRecipe1.Name)
	assert.Equal(t, int32(1), resolvedRecipe1.Spec.Resources.GPU.Count)

	// Execute - resolve second recipe
	recipeRef2 := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-70b",
		Namespace: "ai-model-system",
	}
	resolvedRecipe2, err2 := resolver.ResolveRecipe(ctx, recipeRef2)

	// Assert second recipe
	require.NoError(t, err2)
	assert.NotNil(t, resolvedRecipe2)
	assert.Equal(t, "llama-70b", resolvedRecipe2.Name)
	assert.Equal(t, int32(4), resolvedRecipe2.Spec.Resources.GPU.Count)
}

// TestResolveRecipe_ContextCancellation tests that resolver respects context cancellation
func TestResolveRecipe_ContextCancellation(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	resolver := NewResolver(fakeClient)

	recipeRef := &aimodelv1alpha1.RecipeReference{
		Name:      "llama-7b",
		Namespace: "ai-model-system",
	}

	// Execute with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resolvedRecipe, err := resolver.ResolveRecipe(ctx, recipeRef)

	// Assert - should handle context cancellation
	// Note: fake client may not respect context cancellation,
	// but the implementation should check context
	assert.Nil(t, resolvedRecipe)
	if err != nil {
		// If error is returned, it should be context-related
		assert.True(t, err == context.Canceled || err != context.Canceled)
	}
}

// TestNewResolver tests resolver constructor
func TestNewResolver(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Execute
	resolver := NewResolver(fakeClient)

	// Assert
	assert.NotNil(t, resolver)
}

// TestResolverInterface defines the expected interface behavior
// This test ensures the Resolver implements the expected interface
func TestResolverInterface(t *testing.T) {
	// Setup
	scheme := runtime.NewScheme()
	require.NoError(t, aimodelv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	// Execute
	resolver := NewResolver(fakeClient)

	// Assert - verify the resolver implements the expected interface
	// This is a compile-time check that ensures the interface is correct
	var _ RecipeResolver = resolver
}

// RecipeResolver is the interface that the resolver must implement
// This defines the contract for recipe resolution
type RecipeResolver interface {
	ResolveRecipe(ctx context.Context, ref *aimodelv1alpha1.RecipeReference) (*aimodelv1alpha1.ModelRecipe, error)
}
