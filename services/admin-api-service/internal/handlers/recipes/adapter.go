// Package recipes provides HTTP handlers for recipe management operations.
package recipes

import (
	"context"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
)

// ServiceAdapter adapts the service.RecipeService to the handler's Service interface
type ServiceAdapter struct {
	svc service.RecipeService
	ctx context.Context
}

// NewServiceAdapter creates a new service adapter
func NewServiceAdapter(svc service.RecipeService) *ServiceAdapter {
	return &ServiceAdapter{
		svc: svc,
		ctx: context.Background(),
	}
}

// WithContext returns a new adapter with the given context
func (a *ServiceAdapter) WithContext(ctx context.Context) *ServiceAdapter {
	return &ServiceAdapter{
		svc: a.svc,
		ctx: ctx,
	}
}

// ListRecipes lists all recipes with optional filtering
func (a *ServiceAdapter) ListRecipes(opts ListRecipesOptions) ([]Recipe, error) {
	filters := &service.ListRecipesFilters{
		Runtime: opts.Runtime,
	}

	resp, err := a.svc.ListRecipes(a.ctx, filters)
	if err != nil {
		return nil, err
	}

	recipes := make([]Recipe, len(resp.Recipes))
	for i, r := range resp.Recipes {
		recipes[i] = Recipe{
			ID:          r.RecipeID.String(),
			Name:        r.Name,
			DisplayName: r.DisplayName,
			Description: r.Description,
			ModelID:     r.ModelID,
			Runtime:     r.Runtime,
			Spec:        r.Spec,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		}
	}

	return recipes, nil
}

// GetRecipe retrieves a recipe by name
func (a *ServiceAdapter) GetRecipe(name string) (*Recipe, error) {
	resp, err := a.svc.GetRecipe(a.ctx, name)
	if err != nil {
		return nil, err
	}

	return &Recipe{
		ID:          resp.RecipeID.String(),
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		ModelID:     resp.ModelID,
		Runtime:     resp.Runtime,
		Spec:        resp.Spec,
		CreatedAt:   resp.CreatedAt,
		UpdatedAt:   resp.UpdatedAt,
	}, nil
}

// CreateRecipe creates a new recipe
func (a *ServiceAdapter) CreateRecipe(req CreateRecipeRequest) (*Recipe, error) {
	input := &service.CreateRecipeInput{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		ModelID:     req.ModelID,
		Runtime:     req.Runtime,
		Spec:        req.Spec,
	}

	resp, err := a.svc.CreateRecipe(a.ctx, input)
	if err != nil {
		return nil, err
	}

	return &Recipe{
		ID:          resp.RecipeID.String(),
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		ModelID:     resp.ModelID,
		Runtime:     resp.Runtime,
		Spec:        resp.Spec,
		CreatedAt:   resp.CreatedAt,
		UpdatedAt:   resp.UpdatedAt,
	}, nil
}

// UpdateRecipe updates an existing recipe
func (a *ServiceAdapter) UpdateRecipe(name string, req UpdateRecipeRequest) (*Recipe, error) {
	input := &service.UpdateRecipeInput{
		DisplayName: req.DisplayName,
		Description: req.Description,
		Spec:        req.Spec,
	}

	resp, err := a.svc.UpdateRecipe(a.ctx, name, input)
	if err != nil {
		return nil, err
	}

	return &Recipe{
		ID:          resp.RecipeID.String(),
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		ModelID:     resp.ModelID,
		Runtime:     resp.Runtime,
		Spec:        resp.Spec,
		CreatedAt:   resp.CreatedAt,
		UpdatedAt:   resp.UpdatedAt,
	}, nil
}

// DeleteRecipe deletes a recipe by name
func (a *ServiceAdapter) DeleteRecipe(name string) error {
	return a.svc.DeleteRecipe(a.ctx, name)
}

// ValidateRecipe validates a recipe specification
func (a *ServiceAdapter) ValidateRecipe(req ValidateRecipeRequest) (*ValidationResult, error) {
	// Convert request to spec map for validation
	spec := req.Spec

	result, err := a.svc.ValidateRecipe(a.ctx, spec)
	if err != nil {
		return nil, err
	}

	// Convert ValidationError structs to strings
	errors := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		if e.Field != "" {
			errors[i] = e.Field + ": " + e.Message
		} else {
			errors[i] = e.Message
		}
	}

	return &ValidationResult{
		Valid:  result.Valid,
		Errors: errors,
	}, nil
}

// ListRecipeDeployments lists all deployments using a specific recipe
func (a *ServiceAdapter) ListRecipeDeployments(name string) ([]DeploymentReference, error) {
	deployments, err := a.svc.ListRecipeDeployments(a.ctx, name)
	if err != nil {
		return nil, err
	}

	// Convert to handler DeploymentReference
	result := make([]DeploymentReference, len(deployments))
	for i, d := range deployments {
		result[i] = DeploymentReference{
			ModelName:   d.ModelName,
			Environment: d.Environment,
			Namespace:   d.Namespace,
		}
	}

	return result, nil
}
