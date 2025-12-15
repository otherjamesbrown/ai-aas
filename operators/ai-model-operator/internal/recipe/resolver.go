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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	aimodelv1alpha1 "github.com/ai-aas/ai-model-operator/api/v1alpha1"
)

const (
	// DefaultRecipeNamespace is the default namespace for ModelRecipes
	DefaultRecipeNamespace = "ai-model-system"
)

// Resolver resolves ModelRecipe references to actual ModelRecipe resources
type Resolver struct {
	client client.Client
}

// NewResolver creates a new recipe resolver
func NewResolver(client client.Client) *Resolver {
	return &Resolver{
		client: client,
	}
}

// ResolveRecipe resolves a RecipeReference to a ModelRecipe
func (r *Resolver) ResolveRecipe(ctx context.Context, ref *aimodelv1alpha1.RecipeReference) (*aimodelv1alpha1.ModelRecipe, error) {
	// Handle nil reference
	if ref == nil {
		return nil, fmt.Errorf("recipe reference is nil")
	}

	// Handle empty name
	if ref.Name == "" {
		return nil, fmt.Errorf("recipe name is empty")
	}

	// Default namespace to ai-model-system if empty
	namespace := ref.Namespace
	if namespace == "" {
		namespace = DefaultRecipeNamespace
	}

	// Fetch the ModelRecipe from Kubernetes
	recipe := &aimodelv1alpha1.ModelRecipe{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      ref.Name,
		Namespace: namespace,
	}, recipe)

	if err != nil {
		return nil, fmt.Errorf("recipe %s not found: %w", ref.Name, err)
	}

	return recipe, nil
}
