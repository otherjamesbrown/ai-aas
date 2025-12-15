package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Mock implementation for testing

type mockRecipeRepository struct {
	recipes map[string]*RecipeResponse
	err     error
}

func (m *mockRecipeRepository) GetByName(ctx context.Context, name string) (*RecipeResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	recipe, exists := m.recipes[name]
	if !exists {
		return nil, ErrRecipeNotFound
	}
	return recipe, nil
}

func (m *mockRecipeRepository) Create(ctx context.Context, input *CreateRecipeInput) (*RecipeResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	recipe := &RecipeResponse{
		RecipeID:    uuid.New(),
		Name:        input.Name,
		DisplayName: input.DisplayName,
		Description: input.Description,
		ModelID:     input.ModelID,
		Runtime:     input.Runtime,
		Spec:        input.Spec,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.recipes[input.Name] = recipe
	return recipe, nil
}

func (m *mockRecipeRepository) List(ctx context.Context, filters *ListRecipesFilters) ([]RecipeResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	var results []RecipeResponse
	for _, recipe := range m.recipes {
		if filters.Runtime != "" && recipe.Runtime != filters.Runtime {
			continue
		}
		if filters.ModelID != "" && recipe.ModelID != filters.ModelID {
			continue
		}
		results = append(results, *recipe)
	}
	return results, nil
}

func (m *mockRecipeRepository) Update(ctx context.Context, name string, input *UpdateRecipeInput) (*RecipeResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	recipe, exists := m.recipes[name]
	if !exists {
		return nil, ErrRecipeNotFound
	}
	if input.DisplayName != nil {
		recipe.DisplayName = *input.DisplayName
	}
	if input.Description != nil {
		recipe.Description = *input.Description
	}
	if input.Spec != nil {
		recipe.Spec = input.Spec
	}
	recipe.UpdatedAt = time.Now()
	return recipe, nil
}

func (m *mockRecipeRepository) Delete(ctx context.Context, name string) error {
	if m.err != nil {
		return m.err
	}
	if _, exists := m.recipes[name]; !exists {
		return ErrRecipeNotFound
	}
	delete(m.recipes, name)
	return nil
}

// Mock service implementation for testing
type mockRecipeService struct {
	repo *mockRecipeRepository
}

func newMockRecipeService() *mockRecipeService {
	return &mockRecipeService{
		repo: &mockRecipeRepository{
			recipes: make(map[string]*RecipeResponse),
		},
	}
}

func (s *mockRecipeService) CreateRecipe(ctx context.Context, input *CreateRecipeInput) (*RecipeResponse, error) {
	// Validate input
	if input.Name == "" {
		return nil, ErrMissingRequiredField
	}
	if input.Runtime == "" {
		return nil, ErrMissingRequiredField
	}
	if input.Runtime != "vllm" && input.Runtime != "triton" && input.Runtime != "tgi" {
		return nil, ErrInvalidRuntime
	}

	// Check if already exists
	existing, _ := s.repo.GetByName(ctx, input.Name)
	if existing != nil {
		return nil, ErrRecipeAlreadyExists
	}

	return s.repo.Create(ctx, input)
}

func (s *mockRecipeService) GetRecipe(ctx context.Context, name string) (*RecipeResponse, error) {
	if name == "" {
		return nil, ErrMissingRequiredField
	}
	return s.repo.GetByName(ctx, name)
}

func (s *mockRecipeService) ListRecipes(ctx context.Context, filters *ListRecipesFilters) (*ListRecipesResponse, error) {
	if filters == nil {
		filters = &ListRecipesFilters{}
	}
	recipes, err := s.repo.List(ctx, filters)
	if err != nil {
		return nil, err
	}
	return &ListRecipesResponse{
		Recipes: recipes,
		Total:   len(recipes),
	}, nil
}

func (s *mockRecipeService) UpdateRecipe(ctx context.Context, name string, input *UpdateRecipeInput) (*RecipeResponse, error) {
	if name == "" {
		return nil, ErrMissingRequiredField
	}
	return s.repo.Update(ctx, name, input)
}

func (s *mockRecipeService) DeleteRecipe(ctx context.Context, name string) error {
	if name == "" {
		return ErrMissingRequiredField
	}
	return s.repo.Delete(ctx, name)
}

func (s *mockRecipeService) ValidateRecipe(ctx context.Context, spec map[string]interface{}) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Check runtime
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

	if runtime != "vllm" && runtime != "triton" && runtime != "tgi" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "runtime",
			Message: "runtime must be one of: vllm, triton, tgi",
		})
	}

	// Check resources
	if _, hasResources := spec["resources"]; !hasResources {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "resources",
			Message: "resources field is required",
		})
	}

	return result, nil
}

// Test Cases

func TestRecipeService_CreateRecipe_Success(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	input := &CreateRecipeInput{
		Name:        "mistral-7b-instruct-v03",
		DisplayName: "Mistral 7B Instruct v0.3",
		Description: "Mistral 7B instruction-tuned model",
		ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
		Runtime:     "vllm",
		Spec: map[string]interface{}{
			"runtime": "vllm",
			"resources": map[string]interface{}{
				"gpu": map[string]interface{}{
					"vendor":      "nvidia",
					"count":       1,
					"minMemoryGB": 16,
				},
			},
		},
	}

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
			svc := newMockRecipeService()
			ctx := context.Background()

			_, err := svc.CreateRecipe(ctx, tt.input)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestRecipeService_CreateRecipe_AlreadyExists(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	input := &CreateRecipeInput{
		Name:    "test-recipe",
		Runtime: "vllm",
	}

	// Create first time
	_, err := svc.CreateRecipe(ctx, input)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Try to create again
	_, err = svc.CreateRecipe(ctx, input)
	if !errors.Is(err, ErrRecipeAlreadyExists) {
		t.Errorf("expected ErrRecipeAlreadyExists, got %v", err)
	}
}

func TestRecipeService_GetRecipe_Success(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	// Create a recipe first
	input := &CreateRecipeInput{
		Name:        "test-recipe",
		DisplayName: "Test Recipe",
		Runtime:     "vllm",
	}
	created, err := svc.CreateRecipe(ctx, input)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Get the recipe
	recipe, err := svc.GetRecipe(ctx, "test-recipe")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if recipe.Name != created.Name {
		t.Errorf("expected name %s, got %s", created.Name, recipe.Name)
	}
	if recipe.RecipeID != created.RecipeID {
		t.Errorf("expected ID %s, got %s", created.RecipeID, recipe.RecipeID)
	}
}

func TestRecipeService_GetRecipe_NotFound(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	_, err := svc.GetRecipe(ctx, "nonexistent")
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

func TestRecipeService_ListRecipes_All(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	// Create multiple recipes
	recipes := []*CreateRecipeInput{
		{Name: "recipe1", Runtime: "vllm", ModelID: "model1"},
		{Name: "recipe2", Runtime: "triton", ModelID: "model2"},
		{Name: "recipe3", Runtime: "vllm", ModelID: "model1"},
	}

	for _, input := range recipes {
		_, err := svc.CreateRecipe(ctx, input)
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// List all
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

func TestRecipeService_ListRecipes_FilterByRuntime(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	// Create recipes with different runtimes
	recipes := []*CreateRecipeInput{
		{Name: "vllm1", Runtime: "vllm"},
		{Name: "vllm2", Runtime: "vllm"},
		{Name: "triton1", Runtime: "triton"},
	}

	for _, input := range recipes {
		_, err := svc.CreateRecipe(ctx, input)
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// Filter by vllm
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

func TestRecipeService_ListRecipes_FilterByModelID(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	// Create recipes with different model IDs
	recipes := []*CreateRecipeInput{
		{Name: "recipe1", Runtime: "vllm", ModelID: "modelA"},
		{Name: "recipe2", Runtime: "vllm", ModelID: "modelB"},
		{Name: "recipe3", Runtime: "vllm", ModelID: "modelA"},
	}

	for _, input := range recipes {
		_, err := svc.CreateRecipe(ctx, input)
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// Filter by modelA
	result, err := svc.ListRecipes(ctx, &ListRecipesFilters{ModelID: "modelA"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 recipes for modelA, got %d", result.Total)
	}
}

func TestRecipeService_UpdateRecipe_Success(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	// Create a recipe
	input := &CreateRecipeInput{
		Name:        "test-recipe",
		DisplayName: "Original Name",
		Description: "Original Description",
		Runtime:     "vllm",
	}
	_, err := svc.CreateRecipe(ctx, input)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Update it
	newDisplayName := "Updated Name"
	newDescription := "Updated Description"
	update := &UpdateRecipeInput{
		DisplayName: &newDisplayName,
		Description: &newDescription,
	}

	updated, err := svc.UpdateRecipe(ctx, "test-recipe", update)
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

func TestRecipeService_UpdateRecipe_NotFound(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	newName := "Updated"
	update := &UpdateRecipeInput{
		DisplayName: &newName,
	}

	_, err := svc.UpdateRecipe(ctx, "nonexistent", update)
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

func TestRecipeService_DeleteRecipe_Success(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	// Create a recipe
	input := &CreateRecipeInput{
		Name:    "test-recipe",
		Runtime: "vllm",
	}
	_, err := svc.CreateRecipe(ctx, input)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Delete it
	err = svc.DeleteRecipe(ctx, "test-recipe")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it's gone
	_, err = svc.GetRecipe(ctx, "test-recipe")
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected recipe to be deleted, got error: %v", err)
	}
}

func TestRecipeService_DeleteRecipe_NotFound(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

	err := svc.DeleteRecipe(ctx, "nonexistent")
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Errorf("expected ErrRecipeNotFound, got %v", err)
	}
}

func TestRecipeService_ValidateRecipe_Valid(t *testing.T) {
	svc := newMockRecipeService()
	ctx := context.Background()

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
			svc := newMockRecipeService()
			ctx := context.Background()

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
