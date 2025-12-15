// Package recipe provides commands for managing model recipes.
package recipe

// Recipe represents a model recipe from the API
type Recipe struct {
	ID          string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string                 `json:"name" yaml:"name"`
	DisplayName string                 `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	ModelID     string                 `json:"modelID,omitempty" yaml:"modelID,omitempty"`
	Runtime     string                 `json:"runtime" yaml:"runtime"`
	Spec        map[string]interface{} `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// RecipesResponse is the API response for listing recipes
type RecipesResponse struct {
	Recipes []Recipe `json:"recipes" yaml:"recipes"`
}

// RecipeClient interface for interacting with the recipes API
type RecipeClient interface {
	GetRecipe(name string) (*Recipe, error)
}
