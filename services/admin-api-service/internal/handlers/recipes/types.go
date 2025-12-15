// Package recipes provides HTTP handlers for recipe management operations.
package recipes

import (
	"errors"
	"time"
)

// Error variables for handler error mapping
var (
	ErrRecipeNotFound   = errors.New("recipe not found")
	ErrRecipeNameExists = errors.New("recipe name already exists")
	ErrInvalidRecipeName = errors.New("invalid recipe name format")
	ErrRecipeInUse      = errors.New("recipe is in use by deployments")
)

// Service defines the interface for recipe management operations
type Service interface {
	ListRecipes(opts ListRecipesOptions) ([]Recipe, error)
	GetRecipe(name string) (*Recipe, error)
	CreateRecipe(req CreateRecipeRequest) (*Recipe, error)
	UpdateRecipe(name string, req UpdateRecipeRequest) (*Recipe, error)
	DeleteRecipe(name string) error
	ValidateRecipe(req ValidateRecipeRequest) (*ValidationResult, error)
	ListRecipeDeployments(name string) ([]DeploymentReference, error)
}

// ListRecipesOptions contains options for listing recipes
type ListRecipesOptions struct {
	Runtime string // Filter by runtime (vllm, triton, tgi)
}

// Recipe represents a model recipe in the database
type Recipe struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName,omitempty"`
	Description string                 `json:"description,omitempty"`
	ModelID     string                 `json:"modelID"`
	Runtime     string                 `json:"runtime"`
	Spec        map[string]interface{} `json:"spec"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// CreateRecipeRequest represents a request to create a new recipe
type CreateRecipeRequest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName,omitempty"`
	Description string                 `json:"description,omitempty"`
	ModelID     string                 `json:"modelID"`
	Runtime     string                 `json:"runtime"`
	Spec        map[string]interface{} `json:"spec"`
}

// UpdateRecipeRequest represents a request to update an existing recipe
type UpdateRecipeRequest struct {
	DisplayName *string                 `json:"displayName,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Spec        map[string]interface{}  `json:"spec,omitempty"`
}

// ValidateRecipeRequest represents a request to validate a recipe
type ValidateRecipeRequest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName,omitempty"`
	Description string                 `json:"description,omitempty"`
	ModelID     string                 `json:"modelID"`
	Runtime     string                 `json:"runtime"`
	Spec        map[string]interface{} `json:"spec"`
}

// RecipeResponse represents a recipe in API responses
type RecipeResponse struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName,omitempty"`
	Description string                 `json:"description,omitempty"`
	ModelID     string                 `json:"modelID"`
	Runtime     string                 `json:"runtime"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
	CreatedAt   time.Time              `json:"createdAt,omitempty"`
	UpdatedAt   time.Time              `json:"updatedAt,omitempty"`
}

// ListRecipesResponse represents the response for listing recipes
type ListRecipesResponse struct {
	Recipes []RecipeResponse `json:"recipes"`
}

// ValidationResult represents the result of recipe validation
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// DeploymentReference represents a deployment using a recipe
type DeploymentReference struct {
	ModelName   string `json:"modelName"`
	Environment string `json:"environment"`
	Namespace   string `json:"namespace,omitempty"`
}

// ListDeploymentsResponse represents the response for listing deployments
type ListDeploymentsResponse struct {
	Deployments []DeploymentReference `json:"deployments"`
}
