package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/pashagolub/pgxmock/v4"
)

// TestRecipeRepository_Create_Success tests successful recipe creation
func TestRecipeRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
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

	// Expect the INSERT query
	mock.ExpectExec(`INSERT INTO recipes`).
		WithArgs(pgxmock.AnyArg(), create.Name, create.DisplayName, create.Description,
			create.ModelID, create.Runtime, create.Spec, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Create_DuplicateName_Error tests that creating a recipe with a duplicate name fails
func TestRecipeRepository_Create_DuplicateName_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)

	create := &domain.RecipeCreate{
		Name:    "llama-7b-vllm",
		ModelID: "meta-llama/Llama-2-7b-hf",
		Runtime: "vllm",
		Spec:    spec,
	}

	// Simulate duplicate key violation error
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
	mock.ExpectExec(`INSERT INTO recipes`).
		WithArgs(pgxmock.AnyArg(), create.Name, create.DisplayName, create.Description,
			create.ModelID, create.Runtime, create.Spec, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(pgErr)

	_, err = repo.Create(ctx, create)
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Get_Success tests successful retrieval of a recipe by name
func TestRecipeRepository_Get_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	expectedID := uuid.New()
	now := time.Now().UTC()

	// Expect the SELECT query
	rows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(expectedID, "llama-7b-vllm", "Llama 7B with vLLM", "Test recipe", "meta-llama/Llama-2-7b-hf", "vllm", spec, now, now)

	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE name`).
		WithArgs("llama-7b-vllm").
		WillReturnRows(rows)

	recipe, err := repo.Get(ctx, "llama-7b-vllm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if recipe == nil {
		t.Fatal("expected recipe to be non-nil")
	}
	if recipe.ID != expectedID {
		t.Errorf("expected ID %s, got %s", expectedID, recipe.ID)
	}
	if recipe.Name != "llama-7b-vllm" {
		t.Errorf("expected name llama-7b-vllm, got %s", recipe.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Get_NotFound_Error tests that getting a non-existent recipe returns nil
func TestRecipeRepository_Get_NotFound_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	// Expect query to return no rows
	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE name`).
		WithArgs("non-existent-recipe").
		WillReturnError(pgx.ErrNoRows)

	recipe, err := repo.Get(ctx, "non-existent-recipe")
	if err != nil {
		t.Fatalf("expected no error for not found, got %v", err)
	}
	if recipe != nil {
		t.Error("expected recipe to be nil for not found")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_List_All tests listing all recipes
func TestRecipeRepository_List_All(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	now := time.Now().UTC()

	// Expect count query
	countRows := pgxmock.NewRows([]string{"count"}).AddRow(3)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM recipes`).
		WillReturnRows(countRows)

	// Expect data query
	dataRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(uuid.New(), "llama-7b-vllm", "Llama 7B", "", "model1", "vllm", spec, now, now).
		AddRow(uuid.New(), "llama-7b-triton", "Llama 7B Triton", "", "model1", "triton", spec, now, now).
		AddRow(uuid.New(), "mistral-7b-vllm", "Mistral 7B", "", "model2", "vllm", spec, now, now)

	mock.ExpectQuery(`SELECT (.+) FROM recipes ORDER BY created_at DESC LIMIT`).
		WithArgs(100, 0).
		WillReturnRows(dataRows)

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_List_FilterByRuntime tests filtering recipes by runtime
func TestRecipeRepository_List_FilterByRuntime(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	now := time.Now().UTC()

	// Expect count query with runtime filter
	countRows := pgxmock.NewRows([]string{"count"}).AddRow(2)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM recipes WHERE runtime`).
		WithArgs("vllm").
		WillReturnRows(countRows)

	// Expect data query with runtime filter
	dataRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(uuid.New(), "llama-7b-vllm", "Llama 7B", "", "model1", "vllm", spec, now, now).
		AddRow(uuid.New(), "mistral-7b-vllm", "Mistral 7B", "", "model2", "vllm", spec, now, now)

	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE runtime = \$1 ORDER BY created_at DESC LIMIT`).
		WithArgs("vllm", 100, 0).
		WillReturnRows(dataRows)

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_List_Pagination tests pagination functionality
func TestRecipeRepository_List_Pagination(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	now := time.Now().UTC()

	// First page - expect count query
	countRows := pgxmock.NewRows([]string{"count"}).AddRow(5)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM recipes`).
		WillReturnRows(countRows)

	// First page - expect data query
	dataRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(uuid.New(), "recipe-a", "", "", "model1", "vllm", spec, now, now).
		AddRow(uuid.New(), "recipe-b", "", "", "model1", "vllm", spec, now, now)

	mock.ExpectQuery(`SELECT (.+) FROM recipes ORDER BY created_at DESC LIMIT`).
		WithArgs(2, 0).
		WillReturnRows(dataRows)

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Update_Success tests successful recipe update
func TestRecipeRepository_Update_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	existingID := uuid.New()
	createdAt := time.Now().UTC().Add(-1 * time.Hour)
	updatedAt := time.Now().UTC()

	// Expect Get query first
	getRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(existingID, "llama-7b-vllm", "Llama 7B", "Original description", "meta-llama/Llama-2-7b-hf", "vllm", spec, createdAt, createdAt)

	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE name`).
		WithArgs("llama-7b-vllm").
		WillReturnRows(getRows)

	// Expect UPDATE query
	newDisplayName := "Llama 7B Updated"
	newDescription := "Updated description"
	newRuntime := "triton"
	newSpec := json.RawMessage(`{"runtime": "triton"}`)

	updateRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(existingID, "llama-7b-vllm", newDisplayName, newDescription, "meta-llama/Llama-2-7b-hf", newRuntime, newSpec, createdAt, updatedAt)

	mock.ExpectQuery(`UPDATE recipes SET (.+) WHERE name`).
		WithArgs(newDisplayName, newDescription, newRuntime, newSpec, "llama-7b-vllm").
		WillReturnRows(updateRows)

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
	if !updated.UpdatedAt.After(createdAt) {
		t.Error("expected updated_at to be after created_at")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Update_PartialUpdate tests partial update of a recipe
func TestRecipeRepository_Update_PartialUpdate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	existingID := uuid.New()
	createdAt := time.Now().UTC().Add(-1 * time.Hour)
	updatedAt := time.Now().UTC()

	// Expect Get query first
	getRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(existingID, "llama-7b-vllm", "Llama 7B", "Original description", "meta-llama/Llama-2-7b-hf", "vllm", spec, createdAt, createdAt)

	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE name`).
		WithArgs("llama-7b-vllm").
		WillReturnRows(getRows)

	// Expect UPDATE query with only display_name
	newDisplayName := "Llama 7B Updated"
	updateRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(existingID, "llama-7b-vllm", newDisplayName, "Original description", "meta-llama/Llama-2-7b-hf", "vllm", spec, createdAt, updatedAt)

	mock.ExpectQuery(`UPDATE recipes SET display_name = \$1 WHERE name`).
		WithArgs(newDisplayName, "llama-7b-vllm").
		WillReturnRows(updateRows)

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
	if updated.Description != "Original description" {
		t.Errorf("expected description unchanged: Original description, got %s", updated.Description)
	}
	if updated.Runtime != "vllm" {
		t.Errorf("expected runtime unchanged: vllm, got %s", updated.Runtime)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Update_NotFound_Error tests updating a non-existent recipe
func TestRecipeRepository_Update_NotFound_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	// Expect Get query to return no rows
	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE name`).
		WithArgs("non-existent-recipe").
		WillReturnError(pgx.ErrNoRows)

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Delete_Success tests successful recipe deletion
func TestRecipeRepository_Delete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	// Expect DELETE query
	mock.ExpectExec(`DELETE FROM recipes WHERE name`).
		WithArgs("llama-7b-vllm").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(ctx, "llama-7b-vllm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_Delete_NotFound_Error tests deleting a non-existent recipe
func TestRecipeRepository_Delete_NotFound_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	// Expect DELETE query that affects 0 rows
	mock.ExpectExec(`DELETE FROM recipes WHERE name`).
		WithArgs("non-existent-recipe").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.Delete(ctx, "non-existent-recipe")
	if err != pgx.ErrNoRows {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_List_DefaultPagination tests default pagination values
func TestRecipeRepository_List_DefaultPagination(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
	ctx := context.Background()

	spec := json.RawMessage(`{"runtime": "vllm"}`)
	now := time.Now().UTC()

	// Expect count query
	countRows := pgxmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM recipes`).
		WillReturnRows(countRows)

	// Expect data query with default limit of 100
	dataRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(uuid.New(), "test-recipe", "", "", "model1", "vllm", spec, now, now)

	mock.ExpectQuery(`SELECT (.+) FROM recipes ORDER BY created_at DESC LIMIT`).
		WithArgs(100, 0).
		WillReturnRows(dataRows)

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRecipeRepository_SpecJSONMarshaling tests that spec is properly stored and retrieved as JSON
func TestRecipeRepository_SpecJSONMarshaling(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	db := &DB{pool: mock}
	repo := NewRecipeRepository(db)
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

	// Expect CREATE query
	mock.ExpectExec(`INSERT INTO recipes`).
		WithArgs(pgxmock.AnyArg(), create.Name, create.DisplayName, create.Description,
			create.ModelID, create.Runtime, complexSpec, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	created, err := repo.Create(ctx, create)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Verify spec is stored correctly
	if string(created.Spec) != string(complexSpec) {
		t.Errorf("spec mismatch:\nexpected: %s\ngot: %s", string(complexSpec), string(created.Spec))
	}

	// Expect GET query
	now := time.Now().UTC()
	getRows := pgxmock.NewRows([]string{"id", "name", "display_name", "description", "model_id", "runtime", "spec", "created_at", "updated_at"}).
		AddRow(created.ID, "llama-7b-complex", "", "", "meta-llama/Llama-2-7b-hf", "vllm", complexSpec, now, now)

	mock.ExpectQuery(`SELECT (.+) FROM recipes WHERE name`).
		WithArgs("llama-7b-complex").
		WillReturnRows(getRows)

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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
