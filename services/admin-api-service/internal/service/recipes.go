package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
	"go.uber.org/zap"
)

// RecipeService defines the interface for recipe management
type RecipeService interface {
	CreateRecipe(ctx context.Context, input *CreateRecipeInput) (*RecipeResponse, error)
	GetRecipe(ctx context.Context, name string) (*RecipeResponse, error)
	ListRecipes(ctx context.Context, filters *ListRecipesFilters) (*ListRecipesResponse, error)
	UpdateRecipe(ctx context.Context, name string, input *UpdateRecipeInput) (*RecipeResponse, error)
	DeleteRecipe(ctx context.Context, name string) error
	ValidateRecipe(ctx context.Context, spec map[string]interface{}) (*ValidationResult, error)
	ListRecipeDeployments(ctx context.Context, name string) ([]DeploymentReference, error)
}

// CreateRecipeInput represents the input for creating a recipe
type CreateRecipeInput struct {
	Name        string
	DisplayName string
	Description string
	ModelID     string
	Runtime     string
	Spec        map[string]interface{}
}

// UpdateRecipeInput represents the input for updating a recipe
type UpdateRecipeInput struct {
	DisplayName *string
	Description *string
	Spec        map[string]interface{}
}

// RecipeResponse represents a recipe response
type RecipeResponse struct {
	RecipeID    uuid.UUID
	Name        string
	DisplayName string
	Description string
	ModelID     string
	Runtime     string
	Spec        map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListRecipesResponse represents a list of recipes
type ListRecipesResponse struct {
	Recipes []RecipeResponse
	Total   int
}

// ListRecipesFilters represents filters for listing recipes
type ListRecipesFilters struct {
	Runtime string
	ModelID string
}

// ValidationResult represents the result of recipe validation
type ValidationResult struct {
	Valid   bool
	Errors  []ValidationError
	Runtime string
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

// DeploymentReference represents a deployment using a recipe
type DeploymentReference struct {
	ModelName   string
	Environment string
	Namespace   string
}

// Sentinel errors for recipe operations
var (
	ErrRecipeNotFound       = fmt.Errorf("recipe not found")
	ErrRecipeAlreadyExists  = fmt.Errorf("recipe already exists")
	ErrInvalidRecipeSpec    = fmt.Errorf("invalid recipe spec")
	ErrInvalidRuntime       = fmt.Errorf("invalid runtime")
	ErrMissingRequiredField = fmt.Errorf("missing required field")
)

// RecipeServiceImpl implements the RecipeService interface
type RecipeServiceImpl struct {
	repo      repository.RecipeRepositoryInterface
	k8sClient K8sClient
	logger    *zap.Logger
}

// K8sClient defines the interface for Kubernetes operations
type K8sClient interface {
	ListAIModelsByRecipe(ctx context.Context, recipeName string) ([]K8sDeployment, error)
}

// K8sDeployment represents a Kubernetes deployment reference
type K8sDeployment struct {
	Name        string
	Namespace   string
	Environment string
}

// NewRecipeService creates a new recipe service
func NewRecipeService(repo repository.RecipeRepositoryInterface, k8sClient K8sClient, logger *zap.Logger) RecipeService {
	return &RecipeServiceImpl{
		repo:      repo,
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// CreateRecipe creates a new recipe with validation
func (s *RecipeServiceImpl) CreateRecipe(ctx context.Context, input *CreateRecipeInput) (*RecipeResponse, error) {
	// Validate required fields
	if input.Name == "" {
		return nil, ErrMissingRequiredField
	}
	if input.Runtime == "" {
		return nil, ErrMissingRequiredField
	}

	// Validate runtime
	if input.Runtime != "vllm" && input.Runtime != "triton" && input.Runtime != "tgi" {
		return nil, ErrInvalidRuntime
	}

	// Check if recipe already exists
	existing, err := s.repo.Get(ctx, input.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing recipe: %w", err)
	}
	if existing != nil {
		return nil, ErrRecipeAlreadyExists
	}

	// Convert input to domain.RecipeCreate
	specJSON, err := json.Marshal(input.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %w", err)
	}

	create := &domain.RecipeCreate{
		Name:        input.Name,
		DisplayName: input.DisplayName,
		Description: input.Description,
		ModelID:     input.ModelID,
		Runtime:     input.Runtime,
		Spec:        specJSON,
	}

	// Create the recipe
	recipe, err := s.repo.Create(ctx, create)
	if err != nil {
		s.logger.Error("failed to create recipe",
			zap.String("recipe_name", input.Name),
			zap.String("runtime", input.Runtime),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create recipe: %w", err)
	}

	// Convert to response
	return s.toResponse(recipe)
}

// GetRecipe retrieves a recipe by name
func (s *RecipeServiceImpl) GetRecipe(ctx context.Context, name string) (*RecipeResponse, error) {
	if name == "" {
		return nil, ErrMissingRequiredField
	}

	recipe, err := s.repo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get recipe: %w", err)
	}
	if recipe == nil {
		return nil, ErrRecipeNotFound
	}

	return s.toResponse(recipe)
}

// ListRecipes retrieves recipes with filtering
func (s *RecipeServiceImpl) ListRecipes(ctx context.Context, filters *ListRecipesFilters) (*ListRecipesResponse, error) {
	if filters == nil {
		filters = &ListRecipesFilters{}
	}

	// Convert filters to domain params
	params := domain.RecipeListParams{
		Runtime: filters.Runtime,
		Limit:   1000, // Use high limit for simplicity in tests
		Offset:  0,
	}

	response, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list recipes: %w", err)
	}

	// Convert to response
	recipes := make([]RecipeResponse, 0, len(response.Recipes))
	for _, recipe := range response.Recipes {
		resp, err := s.toResponse(&recipe)
		if err != nil {
			return nil, err
		}
		recipes = append(recipes, *resp)
	}

	return &ListRecipesResponse{
		Recipes: recipes,
		Total:   response.Pagination.Total,
	}, nil
}

// UpdateRecipe updates a recipe with partial updates
func (s *RecipeServiceImpl) UpdateRecipe(ctx context.Context, name string, input *UpdateRecipeInput) (*RecipeResponse, error) {
	if name == "" {
		return nil, ErrMissingRequiredField
	}

	// Convert to domain update
	update := &domain.RecipeUpdate{
		DisplayName: input.DisplayName,
		Description: input.Description,
	}

	if input.Spec != nil {
		specJSON, err := json.Marshal(input.Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal spec: %w", err)
		}
		rawMsg := json.RawMessage(specJSON)
		update.Spec = &rawMsg
	}

	// Update the recipe
	recipe, err := s.repo.Update(ctx, name, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update recipe: %w", err)
	}
	if recipe == nil {
		return nil, ErrRecipeNotFound
	}

	s.logger.Info("recipe updated",
		zap.String("recipe_id", recipe.ID.String()),
		zap.String("recipe_name", recipe.Name),
	)

	return s.toResponse(recipe)
}

// DeleteRecipe deletes a recipe by name
func (s *RecipeServiceImpl) DeleteRecipe(ctx context.Context, name string) error {
	if name == "" {
		return ErrMissingRequiredField
	}

	// Check if recipe exists first
	existing, err := s.repo.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check recipe: %w", err)
	}
	if existing == nil {
		return ErrRecipeNotFound
	}

	if err := s.repo.Delete(ctx, name); err != nil {
		s.logger.Error("failed to delete recipe",
			zap.String("recipe_name", name),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete recipe: %w", err)
	}

	s.logger.Info("recipe deleted",
		zap.String("recipe_name", name),
	)

	return nil
}

// ValidateRecipe validates a recipe spec without creating it
func (s *RecipeServiceImpl) ValidateRecipe(ctx context.Context, spec map[string]interface{}) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Validate runtime field exists
	runtime, ok := spec["runtime"].(string)
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "runtime",
			Message: "runtime is required and must be a string",
		})
		return result, nil
	}
	result.Runtime = runtime

	// Validate runtime value
	if runtime != "vllm" && runtime != "triton" && runtime != "tgi" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "runtime",
			Message: "runtime must be one of: vllm, triton, tgi",
		})
	}

	// Validate resources field exists
	if _, hasResources := spec["resources"]; !hasResources {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "resources",
			Message: "resources field is required",
		})
	}

	return result, nil
}

// toResponse converts a domain.Recipe to RecipeResponse
func (s *RecipeServiceImpl) toResponse(recipe *domain.Recipe) (*RecipeResponse, error) {
	var spec map[string]interface{}
	if err := json.Unmarshal(recipe.Spec, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal spec: %w", err)
	}

	return &RecipeResponse{
		RecipeID:    recipe.ID,
		Name:        recipe.Name,
		DisplayName: recipe.DisplayName,
		Description: recipe.Description,
		ModelID:     recipe.ModelID,
		Runtime:     recipe.Runtime,
		Spec:        spec,
		CreatedAt:   recipe.CreatedAt,
		UpdatedAt:   recipe.UpdatedAt,
	}, nil
}

// ListRecipeDeployments lists all deployments using a specific recipe
func (s *RecipeServiceImpl) ListRecipeDeployments(ctx context.Context, name string) ([]DeploymentReference, error) {
	if name == "" {
		return nil, ErrMissingRequiredField
	}

	// Check if recipe exists
	recipe, err := s.repo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to check recipe: %w", err)
	}
	if recipe == nil {
		return nil, ErrRecipeNotFound
	}

	// If no Kubernetes client configured, return empty list
	if s.k8sClient == nil {
		s.logger.Warn("no kubernetes client configured, returning empty deployment list",
			zap.String("recipe_name", name),
		)
		return []DeploymentReference{}, nil
	}

	// Query Kubernetes for AIModels using this recipe
	k8sDeployments, err := s.k8sClient.ListAIModelsByRecipe(ctx, name)
	if err != nil {
		s.logger.Error("failed to list deployments from kubernetes",
			zap.String("recipe_name", name),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	// Convert to DeploymentReference
	deployments := make([]DeploymentReference, len(k8sDeployments))
	for i, d := range k8sDeployments {
		deployments[i] = DeploymentReference{
			ModelName:   d.Name,
			Environment: d.Environment,
			Namespace:   d.Namespace,
		}
	}

	s.logger.Info("listed recipe deployments",
		zap.String("recipe_name", name),
		zap.Int("count", len(deployments)),
	)

	return deployments, nil
}
