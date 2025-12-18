package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"go.uber.org/zap"
)

// mockRecipeRepository implements RecipeRepositoryInterface for testing
type mockRecipeRepository struct {
	createFunc func(ctx context.Context, create *domain.RecipeCreate) (*domain.Recipe, error)
	getFunc    func(ctx context.Context, name string) (*domain.Recipe, error)
	listFunc   func(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error)
	updateFunc func(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error)
	deleteFunc func(ctx context.Context, name string) error
}

func (m *mockRecipeRepository) Create(ctx context.Context, create *domain.RecipeCreate) (*domain.Recipe, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, create)
	}
	return nil, errors.New("createFunc not implemented")
}

func (m *mockRecipeRepository) Get(ctx context.Context, name string) (*domain.Recipe, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, name)
	}
	return nil, errors.New("getFunc not implemented")
}

func (m *mockRecipeRepository) List(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, params)
	}
	return nil, errors.New("listFunc not implemented")
}

func (m *mockRecipeRepository) Update(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, name, update)
	}
	return nil, errors.New("updateFunc not implemented")
}

func (m *mockRecipeRepository) Delete(ctx context.Context, name string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, name)
	}
	return errors.New("deleteFunc not implemented")
}

// TestRecipeService_CreateRecipe_Success tests successful recipe creation
func TestRecipeService_CreateRecipe_Success(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	spec := map[string]interface{}{
		"runtime": "vllm",
		"resources": map[string]interface{}{
			"gpu": map[string]interface{}{
				"vendor":      "nvidia",
				"count":       1,
				"minMemoryGB": 16,
			},
		},
	}

	input := &CreateRecipeInput{
		Name:        "mistral-7b-instruct-v03",
		DisplayName: "Mistral 7B Instruct v0.3",
		Description: "Mistral 7B instruction-tuned model",
		ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
		Runtime:     "vllm",
		Spec:        spec,
	}

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			// Recipe doesn't exist yet
			return nil, nil
		},
		createFunc: func(ctx context.Context, create *domain.RecipeCreate) (*domain.Recipe, error) {
			now := time.Now().UTC()
			return &domain.Recipe{
				ID:          uuid.New(),
				Name:        create.Name,
				DisplayName: create.DisplayName,
				Description: create.Description,
				ModelID:     create.ModelID,
				Runtime:     create.Runtime,
				Spec:        create.Spec,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	recipe, err := svc.CreateRecipe(ctx, input)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if recipe.Name != input.Name {
		t.Errorf("expected name %s, got %s", input.Name, recipe.Name)
	}
	if recipe.Runtime != input.Runtime {
		t.Errorf("expected runtime %s, got %s", input.Runtime, recipe.Runtime)
	}
	if recipe.RecipeID == uuid.Nil {
		t.Error("expected non-nil recipe ID")
	}
}

// TestRecipeService_CreateRecipe_ValidationError tests validation errors
func TestRecipeService_CreateRecipe_ValidationError(t *testing.T) {
	tests := []struct {
		name        string
		input       *CreateRecipeInput
		expectedErr error
	}{
		{
			name: "missing name",
			input: &CreateRecipeInput{
				Runtime: "vllm",
			},
			expectedErr: ErrMissingRequiredField,
		},
		{
			name: "missing runtime",
			input: &CreateRecipeInput{
				Name: "test-recipe",
			},
			expectedErr: ErrMissingRequiredField,
		},
		{
			name: "invalid runtime",
			input: &CreateRecipeInput{
				Name:    "test-recipe",
				Runtime: "invalid",
			},
			expectedErr: ErrInvalidRuntime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := zap.NewNop()
			mockRepo := &mockRecipeRepository{}

			svc := NewRecipeService(mockRepo, nil, logger)
			_, err := svc.CreateRecipe(ctx, tt.input)

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// TestRecipeService_CreateRecipe_AlreadyExists tests duplicate recipe creation
func TestRecipeService_CreateRecipe_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	input := &CreateRecipeInput{
		Name:    "test-recipe",
		Runtime: "vllm",
	}

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			// Recipe already exists
			return &domain.Recipe{
				ID:      uuid.New(),
				Name:    name,
				Runtime: "vllm",
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	_, err := svc.CreateRecipe(ctx, input)

	if !errors.Is(err, ErrRecipeAlreadyExists) {
		t.Errorf("expected ErrRecipeAlreadyExists, got %v", err)
	}
}

// TestRecipeService_GetRecipe_Success tests successful recipe retrieval
func TestRecipeService_GetRecipe_Success(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	expectedID := uuid.New()
	now := time.Now().UTC()
	spec := json.RawMessage(`{"runtime": "vllm"}`)

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:          expectedID,
				Name:        "test-recipe",
				DisplayName: "Test Recipe",
				Runtime:     "vllm",
				Spec:        spec,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	recipe, err := svc.GetRecipe(ctx, "test-recipe")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if recipe.RecipeID != expectedID {
		t.Errorf("expected ID %s, got %s", expectedID, recipe.RecipeID)
	}
	if recipe.Name != "test-recipe" {
		t.Errorf("expected name test-recipe, got %s", recipe.Name)
	}
}

// TestRecipeService_GetRecipe_NotFound tests recipe not found
func TestRecipeService_GetRecipe_NotFound(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return nil, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	_, err := svc.GetRecipe(ctx, "nonexistent")

	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

// TestRecipeService_ListRecipes_All tests listing all recipes
func TestRecipeService_ListRecipes_All(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	now := time.Now().UTC()

	mockRepo := &mockRecipeRepository{
		listFunc: func(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error) {
			return &domain.RecipeListResponse{
				Recipes: []domain.Recipe{
					{ID: uuid.New(), Name: "recipe1", Runtime: "vllm", ModelID: "model1", Spec: spec, CreatedAt: now, UpdatedAt: now},
					{ID: uuid.New(), Name: "recipe2", Runtime: "triton", ModelID: "model2", Spec: spec, CreatedAt: now, UpdatedAt: now},
					{ID: uuid.New(), Name: "recipe3", Runtime: "vllm", ModelID: "model1", Spec: spec, CreatedAt: now, UpdatedAt: now},
				},
				Pagination: domain.Pagination{
					Total:  3,
					Limit:  1000,
					Offset: 0,
				},
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	result, err := svc.ListRecipes(ctx, &ListRecipesFilters{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Total != 3 {
		t.Errorf("expected 3 recipes, got %d", result.Total)
	}
	if len(result.Recipes) != 3 {
		t.Errorf("expected 3 recipes in list, got %d", len(result.Recipes))
	}
}

// TestRecipeService_ListRecipes_FilterByRuntime tests filtering by runtime
func TestRecipeService_ListRecipes_FilterByRuntime(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	now := time.Now().UTC()

	mockRepo := &mockRecipeRepository{
		listFunc: func(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error) {
			// Verify runtime filter is passed correctly
			if params.Runtime != "vllm" {
				t.Errorf("expected runtime filter 'vllm', got '%s'", params.Runtime)
			}

			return &domain.RecipeListResponse{
				Recipes: []domain.Recipe{
					{ID: uuid.New(), Name: "vllm1", Runtime: "vllm", Spec: spec, CreatedAt: now, UpdatedAt: now},
					{ID: uuid.New(), Name: "vllm2", Runtime: "vllm", Spec: spec, CreatedAt: now, UpdatedAt: now},
				},
				Pagination: domain.Pagination{
					Total:  2,
					Limit:  1000,
					Offset: 0,
				},
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	result, err := svc.ListRecipes(ctx, &ListRecipesFilters{Runtime: "vllm"})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 vllm recipes, got %d", result.Total)
	}

	// Verify all returned recipes are vllm
	for _, recipe := range result.Recipes {
		if recipe.Runtime != "vllm" {
			t.Errorf("expected runtime vllm, got %s", recipe.Runtime)
		}
	}
}

// TestRecipeService_UpdateRecipe_Success tests successful recipe update
func TestRecipeService_UpdateRecipe_Success(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	now := time.Now().UTC()
	spec := json.RawMessage(`{"runtime": "vllm"}`)

	newDisplayName := "Updated Name"
	newDescription := "Updated Description"

	mockRepo := &mockRecipeRepository{
		updateFunc: func(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:          uuid.New(),
				Name:        name,
				DisplayName: *update.DisplayName,
				Description: *update.Description,
				Runtime:     "vllm",
				Spec:        spec,
				CreatedAt:   now.Add(-1 * time.Hour),
				UpdatedAt:   now,
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)

	input := &UpdateRecipeInput{
		DisplayName: &newDisplayName,
		Description: &newDescription,
	}

	updated, err := svc.UpdateRecipe(ctx, "test-recipe", input)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if updated.DisplayName != newDisplayName {
		t.Errorf("expected display name %s, got %s", newDisplayName, updated.DisplayName)
	}
	if updated.Description != newDescription {
		t.Errorf("expected description %s, got %s", newDescription, updated.Description)
	}
}

// TestRecipeService_UpdateRecipe_NotFound tests updating non-existent recipe
func TestRecipeService_UpdateRecipe_NotFound(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		updateFunc: func(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error) {
			return nil, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)

	newName := "Updated"
	input := &UpdateRecipeInput{
		DisplayName: &newName,
	}

	_, err := svc.UpdateRecipe(ctx, "nonexistent", input)

	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

// TestRecipeService_DeleteRecipe_Success tests successful recipe deletion
func TestRecipeService_DeleteRecipe_Success(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:      uuid.New(),
				Name:    name,
				Runtime: "vllm",
			}, nil
		},
		deleteFunc: func(ctx context.Context, name string) error {
			return nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	err := svc.DeleteRecipe(ctx, "test-recipe")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestRecipeService_DeleteRecipe_NotFound tests deleting non-existent recipe
func TestRecipeService_DeleteRecipe_NotFound(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return nil, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	err := svc.DeleteRecipe(ctx, "nonexistent")

	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

// TestRecipeService_ValidateRecipe_Valid tests valid recipe validation
func TestRecipeService_ValidateRecipe_Valid(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockRepo := &mockRecipeRepository{}

	svc := NewRecipeService(mockRepo, nil, logger)

	spec := map[string]interface{}{
		"runtime": "vllm",
		"resources": map[string]interface{}{
			"gpu": map[string]interface{}{
				"count": 1,
			},
		},
	}

	result, err := svc.ValidateRecipe(ctx, spec)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got false with errors: %+v", result.Errors)
	}
	if result.Runtime != "vllm" {
		t.Errorf("expected runtime vllm, got %s", result.Runtime)
	}
}

// TestRecipeService_ValidateRecipe_Invalid tests invalid recipe validation
func TestRecipeService_ValidateRecipe_Invalid(t *testing.T) {
	tests := []struct {
		name          string
		spec          map[string]interface{}
		expectedError string
	}{
		{
			name:          "missing runtime",
			spec:          map[string]interface{}{},
			expectedError: "runtime is required",
		},
		{
			name: "invalid runtime",
			spec: map[string]interface{}{
				"runtime": "invalid",
			},
			expectedError: "runtime must be one of",
		},
		{
			name: "missing resources",
			spec: map[string]interface{}{
				"runtime": "vllm",
			},
			expectedError: "resources field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := zap.NewNop()
			mockRepo := &mockRecipeRepository{}

			svc := NewRecipeService(mockRepo, nil, logger)
			result, err := svc.ValidateRecipe(ctx, tt.spec)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid {
				t.Error("expected valid=false, got true")
			}

			if len(result.Errors) == 0 {
				t.Error("expected validation errors, got none")
			}
		})
	}
}

// TestRecipeService_ListRecipeDeployments_Success tests listing deployments
func TestRecipeService_ListRecipeDeployments_Success(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:      uuid.New(),
				Name:    name,
				Runtime: "vllm",
			}, nil
		},
	}

	// Mock K8s client
	mockK8s := &mockK8sClient{
		deployments: []K8sDeployment{
			{Name: "model1", Namespace: "default", Environment: "dev"},
			{Name: "model2", Namespace: "default", Environment: "dev"},
		},
	}

	svc := NewRecipeService(mockRepo, mockK8s, logger)
	deployments, err := svc.ListRecipeDeployments(ctx, "test-recipe")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(deployments) != 2 {
		t.Errorf("expected 2 deployments, got %d", len(deployments))
	}
}

// TestRecipeService_ListRecipeDeployments_NoK8sClient tests with no k8s client
func TestRecipeService_ListRecipeDeployments_NoK8sClient(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:      uuid.New(),
				Name:    name,
				Runtime: "vllm",
			}, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	deployments, err := svc.ListRecipeDeployments(ctx, "test-recipe")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(deployments) != 0 {
		t.Errorf("expected empty deployments, got %d", len(deployments))
	}
}

// TestRecipeService_ListRecipeDeployments_RecipeNotFound tests with non-existent recipe
func TestRecipeService_ListRecipeDeployments_RecipeNotFound(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return nil, nil
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	_, err := svc.ListRecipeDeployments(ctx, "nonexistent")

	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

// TestRecipeService_DeleteRecipe_RepositoryError tests repository error handling
func TestRecipeService_DeleteRecipe_RepositoryError(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	mockRepo := &mockRecipeRepository{
		getFunc: func(ctx context.Context, name string) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:      uuid.New(),
				Name:    name,
				Runtime: "vllm",
			}, nil
		},
		deleteFunc: func(ctx context.Context, name string) error {
			return pgx.ErrNoRows
		},
	}

	svc := NewRecipeService(mockRepo, nil, logger)
	err := svc.DeleteRecipe(ctx, "test-recipe")

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// mockK8sClient implements K8sClient for testing
type mockK8sClient struct {
	deployments []K8sDeployment
	err         error
}

func (m *mockK8sClient) ListAIModelsByRecipe(ctx context.Context, recipeName string) ([]K8sDeployment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.deployments, nil
}
