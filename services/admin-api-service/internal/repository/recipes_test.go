package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

// mockRecipeRepository is a mock implementation for testing
// This will be replaced by the real implementation in T017
type mockRecipeRepository struct {
	recipes map[string]*domain.Recipe
	err     error
}

func newMockRecipeRepository() *mockRecipeRepository {
	return &mockRecipeRepository{
		recipes: make(map[string]*domain.Recipe),
	}
}

func (m *mockRecipeRepository) Create(ctx context.Context, create *domain.RecipeCreate) (*domain.Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Check for duplicate name
	if _, exists := m.recipes[create.Name]; exists {
		return nil, errors.New("duplicate key value violates unique constraint")
	}

	now := time.Now().UTC()
	recipe := &domain.Recipe{
		ID:          uuid.New(),
		Name:        create.Name,
		DisplayName: create.DisplayName,
		Description: create.Description,
		ModelID:     create.ModelID,
		Runtime:     create.Runtime,
		Spec:        create.Spec,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.recipes[recipe.Name] = recipe
	return recipe, nil
}

func (m *mockRecipeRepository) Get(ctx context.Context, name string) (*domain.Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}

	recipe, exists := m.recipes[name]
	if !exists {
		return nil, nil
	}

	return recipe, nil
}

func (m *mockRecipeRepository) List(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Collect all recipes
	all := make([]*domain.Recipe, 0, len(m.recipes))
	for _, recipe := range m.recipes {
		// Apply runtime filter if specified
		if params.Runtime != "" && recipe.Runtime != params.Runtime {
			continue
		}
		all = append(all, recipe)
	}

	total := len(all)

	// Apply pagination
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	start := params.Offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	recipes := all[start:end]

	// Convert to non-pointer slice
	result := make([]domain.Recipe, len(recipes))
	for i, r := range recipes {
		result[i] = *r
	}

	response := &domain.RecipeListResponse{
		Recipes: result,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if end < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

func (m *mockRecipeRepository) Update(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}

	recipe, exists := m.recipes[name]
	if !exists {
		return nil, nil
	}

	// Apply updates
	if update.DisplayName != nil {
		recipe.DisplayName = *update.DisplayName
	}
	if update.Description != nil {
		recipe.Description = *update.Description
	}
	if update.Runtime != nil {
		recipe.Runtime = *update.Runtime
	}
	if update.Spec != nil {
		recipe.Spec = *update.Spec
	}

	recipe.UpdatedAt = time.Now().UTC()
	return recipe, nil
}

func (m *mockRecipeRepository) Delete(ctx context.Context, name string) error {
	if m.err != nil {
		return m.err
	}

	if _, exists := m.recipes[name]; !exists {
		return pgx.ErrNoRows
	}

	delete(m.recipes, name)
	return nil
}

// TestRecipeRepository_Create_Success tests successful recipe creation
func TestRecipeRepository_Create_Success(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	spec := json.RawMessage(`{
		"modelID": "meta-llama/Llama-2-7b-hf",
		"runtime": "vllm",
		"resources": {
			"gpu": {
				"vendor": "nvidia",
				"count": 1,
				"minMemoryGB": 16
			}
		}
	}`)

	create := &domain.RecipeCreate{
		Name:        "llama-7b-vllm",
		DisplayName: "Llama 7B with vLLM",
		Description: "Llama 7B model using vLLM runtime",
		ModelID:     "meta-llama/Llama-2-7b-hf",
		Runtime:     "vllm",
		Spec:        spec,
	}

	recipe, err := repo.Create(ctx, create)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if recipe == nil {
		t.Fatal("expected recipe to be non-nil")
	}
	if recipe.Name != create.Name {
		t.Errorf("expected name %s, got %s", create.Name, recipe.Name)
	}
	if recipe.DisplayName != create.DisplayName {
		t.Errorf("expected display_name %s, got %s", create.DisplayName, recipe.DisplayName)
	}
	if recipe.ModelID != create.ModelID {
		t.Errorf("expected model_id %s, got %s", create.ModelID, recipe.ModelID)
	}
	if recipe.Runtime != create.Runtime {
		t.Errorf("expected runtime %s, got %s", create.Runtime, recipe.Runtime)
	}
	if recipe.ID == uuid.Nil {
		t.Error("expected non-zero UUID for ID")
	}
	if recipe.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
	if recipe.UpdatedAt.IsZero() {
		t.Error("expected non-zero updated_at")
	}
}

// TestRecipeRepository_Create_DuplicateName_Error tests that creating a recipe with a duplicate name fails
func TestRecipeRepository_Create_DuplicateName_Error(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)

	create := &domain.RecipeCreate{
		Name:    "llama-7b-vllm",
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Spec:    spec,
	}

	// Create first recipe
	_, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("first create should succeed, got error: %v", err)
	}

	// Attempt to create duplicate
	_, err = repo.Create(ctx, create)
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

// TestRecipeRepository_Get_Success tests successful retrieval of a recipe by name
func TestRecipeRepository_Get_Success(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)

	create := &domain.RecipeCreate{
		Name:        "llama-7b-vllm",
		DisplayName: "Llama 7B with vLLM",
		Description: "Test recipe",
		ModelID:     "meta-llama/Llama-2-7b-hf",
		Runtime:     "vllm",
		Spec:        spec,
	}

	created, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Get the recipe
	recipe, err := repo.Get(ctx, "llama-7b-vllm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if recipe == nil {
		t.Fatal("expected recipe to be non-nil")
	}
	if recipe.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, recipe.ID)
	}
	if recipe.Name != created.Name {
		t.Errorf("expected name %s, got %s", created.Name, recipe.Name)
	}
}

// TestRecipeRepository_Get_NotFound_Error tests that getting a non-existent recipe returns nil
func TestRecipeRepository_Get_NotFound_Error(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	recipe, err := repo.Get(ctx, "non-existent-recipe")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
	if recipe != nil {
		t.Error("expected recipe to be nil for not found")
	}
}

// TestRecipeRepository_List_All tests listing all recipes
func TestRecipeRepository_List_All(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	// Create multiple recipes
	recipes := []struct {
		name    string
		runtime string
	}{
		{"llama-7b-vllm", "vllm"},
		{"llama-7b-triton", "triton"},
		{"mistral-7b-vllm", "vllm"},
	}

	for _, r := range recipes {
		spec := json.RawMessage(`{"runtime": "` + r.runtime + `"}`)
		create := &domain.RecipeCreate{
			Name:    r.name,
			ModelID: "test-model",
			Runtime: r.runtime,
			Spec:    spec,
		}
		if _, err := repo.Create(ctx, create); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// List all recipes
	params := domain.RecipeListParams{
		Limit:  100,
		Offset: 0,
	}

	response, err := repo.List(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response == nil {
		t.Fatal("expected response to be non-nil")
	}
	if len(response.Recipes) != 3 {
		t.Errorf("expected 3 recipes, got %d", len(response.Recipes))
	}
	if response.Pagination.Total != 3 {
		t.Errorf("expected total 3, got %d", response.Pagination.Total)
	}
	if response.Pagination.NextOffset != nil {
		t.Error("expected no next offset for single page")
	}
}

// TestRecipeRepository_List_FilterByRuntime tests filtering recipes by runtime
func TestRecipeRepository_List_FilterByRuntime(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	// Create recipes with different runtimes
	recipes := []struct {
		name    string
		runtime string
	}{
		{"llama-7b-vllm", "vllm"},
		{"llama-7b-triton", "triton"},
		{"mistral-7b-vllm", "vllm"},
	}

	for _, r := range recipes {
		spec := json.RawMessage(`{"runtime": "` + r.runtime + `"}`)
		create := &domain.RecipeCreate{
			Name:    r.name,
			ModelID: "test-model",
			Runtime: r.runtime,
			Spec:    spec,
		}
		if _, err := repo.Create(ctx, create); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// Filter by vllm runtime
	params := domain.RecipeListParams{
		Runtime: "vllm",
		Limit:   100,
		Offset:  0,
	}

	response, err := repo.List(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(response.Recipes) != 2 {
		t.Errorf("expected 2 vllm recipes, got %d", len(response.Recipes))
	}
	for _, recipe := range response.Recipes {
		if recipe.Runtime != "vllm" {
			t.Errorf("expected runtime vllm, got %s", recipe.Runtime)
		}
	}
}

// TestRecipeRepository_List_Pagination tests pagination functionality
func TestRecipeRepository_List_Pagination(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	// Create 5 recipes
	for i := 0; i < 5; i++ {
		spec := json.RawMessage(`{"runtime": "vllm"}`)
		create := &domain.RecipeCreate{
			Name:    "recipe-" + string(rune('a'+i)),
			ModelID: "test-model",
			Runtime: "vllm",
			Spec:    spec,
		}
		if _, err := repo.Create(ctx, create); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}

	// Get first page (limit 2)
	params := domain.RecipeListParams{
		Limit:  2,
		Offset: 0,
	}

	response, err := repo.List(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(response.Recipes) != 2 {
		t.Errorf("expected 2 recipes in first page, got %d", len(response.Recipes))
	}
	if response.Pagination.Total != 5 {
		t.Errorf("expected total 5, got %d", response.Pagination.Total)
	}
	if response.Pagination.NextOffset == nil {
		t.Error("expected next offset for first page")
	} else if *response.Pagination.NextOffset != 2 {
		t.Errorf("expected next offset 2, got %d", *response.Pagination.NextOffset)
	}

	// Get second page
	params.Offset = 2
	response, err = repo.List(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(response.Recipes) != 2 {
		t.Errorf("expected 2 recipes in second page, got %d", len(response.Recipes))
	}
}

// TestRecipeRepository_Update_Success tests successful recipe update
func TestRecipeRepository_Update_Success(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)

	create := &domain.RecipeCreate{
		Name:        "llama-7b-vllm",
		DisplayName: "Llama 7B",
		Description: "Original description",
		ModelID:     "meta-llama/Llama-2-7b-hf",
		Runtime:     "vllm",
		Spec:        spec,
	}

	created, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Sleep briefly to ensure updated_at is different
	time.Sleep(1 * time.Millisecond)

	// Update the recipe
	newDisplayName := "Llama 7B Updated"
	newDescription := "Updated description"
	newRuntime := "triton"
	newSpec := json.RawMessage(`{"runtime": "triton"}`)

	update := &domain.RecipeUpdate{
		DisplayName: &newDisplayName,
		Description: &newDescription,
		Runtime:     &newRuntime,
		Spec:        &newSpec,
	}

	updated, err := repo.Update(ctx, "llama-7b-vllm", update)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated recipe to be non-nil")
	}
	if updated.DisplayName != newDisplayName {
		t.Errorf("expected display_name %s, got %s", newDisplayName, updated.DisplayName)
	}
	if updated.Description != newDescription {
		t.Errorf("expected description %s, got %s", newDescription, updated.Description)
	}
	if updated.Runtime != newRuntime {
		t.Errorf("expected runtime %s, got %s", newRuntime, updated.Runtime)
	}
	// UpdatedAt should be >= CreatedAt (may be equal due to millisecond precision)
	if updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Error("expected updated_at to be >= created_at")
	}
}

// TestRecipeRepository_Update_PartialUpdate tests partial update of a recipe
func TestRecipeRepository_Update_PartialUpdate(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)

	create := &domain.RecipeCreate{
		Name:        "llama-7b-vllm",
		DisplayName: "Llama 7B",
		Description: "Original description",
		ModelID:     "meta-llama/Llama-2-7b-hf",
		Runtime:     "vllm",
		Spec:        spec,
	}

	_, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Update only display name
	newDisplayName := "Llama 7B Updated"
	update := &domain.RecipeUpdate{
		DisplayName: &newDisplayName,
	}

	updated, err := repo.Update(ctx, "llama-7b-vllm", update)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.DisplayName != newDisplayName {
		t.Errorf("expected display_name %s, got %s", newDisplayName, updated.DisplayName)
	}
	// Verify other fields are unchanged
	if updated.Description != create.Description {
		t.Errorf("expected description unchanged: %s, got %s", create.Description, updated.Description)
	}
	if updated.Runtime != create.Runtime {
		t.Errorf("expected runtime unchanged: %s, got %s", create.Runtime, updated.Runtime)
	}
}

// TestRecipeRepository_Update_NotFound_Error tests updating a non-existent recipe
func TestRecipeRepository_Update_NotFound_Error(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	newDisplayName := "Updated Name"
	update := &domain.RecipeUpdate{
		DisplayName: &newDisplayName,
	}

	recipe, err := repo.Update(ctx, "non-existent-recipe", update)
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
	if recipe != nil {
		t.Error("expected recipe to be nil for not found")
	}
}

// TestRecipeRepository_Delete_Success tests successful recipe deletion
func TestRecipeRepository_Delete_Success(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)

	create := &domain.RecipeCreate{
		Name:    "llama-7b-vllm",
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Spec:    spec,
	}

	_, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Delete the recipe
	err = repo.Delete(ctx, "llama-7b-vllm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it's deleted
	recipe, err := repo.Get(ctx, "llama-7b-vllm")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if recipe != nil {
		t.Error("expected recipe to be nil after deletion")
	}
}

// TestRecipeRepository_Delete_NotFound_Error tests deleting a non-existent recipe
func TestRecipeRepository_Delete_NotFound_Error(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	err := repo.Delete(ctx, "non-existent-recipe")
	if err != pgx.ErrNoRows {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}
}

// TestRecipeRepository_List_DefaultPagination tests default pagination values
func TestRecipeRepository_List_DefaultPagination(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	// Create one recipe
	spec := json.RawMessage(`{"runtime": "vllm"}`)
	create := &domain.RecipeCreate{
		Name:    "test-recipe",
		ModelID: "test-model",
		Runtime: "vllm",
		Spec:    spec,
	}
	if _, err := repo.Create(ctx, create); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// List with zero limit (should default to 100)
	params := domain.RecipeListParams{
		Limit:  0,
		Offset: 0,
	}

	response, err := repo.List(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.Pagination.Limit != 100 {
		t.Errorf("expected default limit 100, got %d", response.Pagination.Limit)
	}

	// List with limit > 500 (should be capped at 100)
	params.Limit = 1000
	response, err = repo.List(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.Pagination.Limit != 100 {
		t.Errorf("expected capped limit 100, got %d", response.Pagination.Limit)
	}
}

// TestRecipeRepository_SpecJSONMarshaling tests that spec is properly stored and retrieved as JSON
func TestRecipeRepository_SpecJSONMarshaling(t *testing.T) {
	repo := newMockRecipeRepository()
	ctx := context.Background()

	complexSpec := json.RawMessage(`{
		"modelID": "meta-llama/Llama-2-7b-hf",
		"runtime": "vllm",
		"resources": {
			"gpu": {
				"vendor": "nvidia",
				"count": 1,
				"minMemoryGB": 16
			},
			"cpu": {
				"requests": "4",
				"limits": "8"
			},
			"memory": {
				"requests": "16Gi",
				"limits": "32Gi"
			}
		},
		"runtimeArgs": {
			"vllm": {
				"dtype": "float16",
				"maxModelLen": 32768,
				"gpuMemoryUtilization": "0.9"
			}
		}
	}`)

	create := &domain.RecipeCreate{
		Name:    "llama-7b-complex",
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Spec:    complexSpec,
	}

	created, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Verify spec is stored correctly
	if string(created.Spec) != string(complexSpec) {
		t.Errorf("spec mismatch:\nexpected: %s\ngot: %s", string(complexSpec), string(created.Spec))
	}

	// Get and verify spec is retrieved correctly
	recipe, err := repo.Get(ctx, "llama-7b-complex")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if string(recipe.Spec) != string(complexSpec) {
		t.Errorf("retrieved spec mismatch:\nexpected: %s\ngot: %s", string(complexSpec), string(recipe.Spec))
	}

	// Verify it can be unmarshaled back to a struct
	var specMap map[string]interface{}
	if err := json.Unmarshal(recipe.Spec, &specMap); err != nil {
		t.Fatalf("failed to unmarshal spec: %v", err)
	}

	// Verify some nested fields
	if resources, ok := specMap["resources"].(map[string]interface{}); ok {
		if gpu, ok := resources["gpu"].(map[string]interface{}); ok {
			if vendor, ok := gpu["vendor"].(string); !ok || vendor != "nvidia" {
				t.Errorf("expected gpu vendor 'nvidia', got %v", gpu["vendor"])
			}
		} else {
			t.Error("expected gpu in resources")
		}
	} else {
		t.Error("expected resources in spec")
	}
}
