package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Recipe represents a model recipe entity
type Recipe struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	ModelID     string          `json:"model_id"`
	Runtime     string          `json:"runtime"`
	Spec        json.RawMessage `json:"spec"` // Full recipe spec as JSON
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// RecipeCreate represents a request to create a recipe
type RecipeCreate struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	ModelID     string          `json:"model_id"`
	Runtime     string          `json:"runtime"`
	Spec        json.RawMessage `json:"spec"`
}

// RecipeUpdate represents a partial recipe update
type RecipeUpdate struct {
	DisplayName *string          `json:"display_name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Runtime     *string          `json:"runtime,omitempty"`
	Spec        *json.RawMessage `json:"spec,omitempty"`
}

// RecipeListParams represents query parameters for listing recipes
type RecipeListParams struct {
	Runtime string // Filter by runtime (vllm, triton, tgi)
	Limit   int
	Offset  int
}

// RecipeListResponse represents a paginated list of recipes
type RecipeListResponse struct {
	Recipes    []Recipe   `json:"recipes"`
	Pagination Pagination `json:"pagination"`
}

// Valid runtimes
var ValidRuntimes = []string{"vllm", "triton", "tgi"}

// IsValidRuntime checks if a runtime is valid
func IsValidRuntime(runtime string) bool {
	for _, r := range ValidRuntimes {
		if r == runtime {
			return true
		}
	}
	return false
}
