package recipes

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// mockService implements the Service interface for testing
type mockService struct {
	recipes []Recipe
	err     error
}

func (m *mockService) ListRecipes(opts ListRecipesOptions) ([]Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Apply runtime filter if specified
	if opts.Runtime != "" {
		filtered := []Recipe{}
		for _, r := range m.recipes {
			if r.Runtime == opts.Runtime {
				filtered = append(filtered, r)
			}
		}
		return filtered, nil
	}

	return m.recipes, nil
}

func (m *mockService) GetRecipe(name string) (*Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, recipe := range m.recipes {
		if recipe.Name == name {
			return &recipe, nil
		}
	}
	return nil, ErrRecipeNotFound
}

func (m *mockService) CreateRecipe(req CreateRecipeRequest) (*Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}
	recipe := Recipe{
		ID:          "test-recipe-id",
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		ModelID:     req.ModelID,
		Runtime:     req.Runtime,
		Spec:        req.Spec,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.recipes = append(m.recipes, recipe)
	return &recipe, nil
}

func (m *mockService) UpdateRecipe(name string, req UpdateRecipeRequest) (*Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i, recipe := range m.recipes {
		if recipe.Name == name {
			if req.DisplayName != nil {
				m.recipes[i].DisplayName = *req.DisplayName
			}
			if req.Description != nil {
				m.recipes[i].Description = *req.Description
			}
			if req.Spec != nil {
				m.recipes[i].Spec = req.Spec
			}
			m.recipes[i].UpdatedAt = time.Now()
			return &m.recipes[i], nil
		}
	}
	return nil, testErrRecipeNotFound
}

func (m *mockService) DeleteRecipe(name string) error {
	if m.err != nil {
		return m.err
	}
	for i, recipe := range m.recipes {
		if recipe.Name == name {
			m.recipes = append(m.recipes[:i], m.recipes[i+1:]...)
			return nil
		}
	}
	return testErrRecipeNotFound
}

func (m *mockService) ValidateRecipe(req ValidateRecipeRequest) (*ValidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Simple validation: check if required fields are present
	valid := req.Name != "" && req.ModelID != "" && req.Runtime != ""
	errors := []string{}

	if req.Name == "" {
		errors = append(errors, "name is required")
	}
	if req.ModelID == "" {
		errors = append(errors, "modelID is required")
	}
	if req.Runtime == "" {
		errors = append(errors, "runtime is required")
	}

	return &ValidationResult{
		Valid:  valid,
		Errors: errors,
	}, nil
}

func (m *mockService) ListRecipeDeployments(name string) ([]DeploymentReference, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Check if recipe exists
	found := false
	for _, recipe := range m.recipes {
		if recipe.Name == name {
			found = true
			break
		}
	}
	if !found {
		return nil, testErrRecipeNotFound
	}

	// Return empty list for testing
	return []DeploymentReference{}, nil
}

// Sentinel errors for mock (use different names from handler.go)
var testErrRecipeNotFound = &recipeError{"recipe not found"}

type recipeError struct {
	msg string
}

func (e *recipeError) Error() string { return e.msg }

// Test setup helpers
func setupRouter(svc Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewHandler(svc)
	h.RegisterRoutes(r)
	return r
}

// Test cases

func TestRecipeHandler_ListRecipes_Success(t *testing.T) {
	svc := &mockService{
		recipes: []Recipe{
			{
				ID:          "1",
				Name:        "mistral-7b-instruct",
				DisplayName: "Mistral 7B Instruct",
				ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
				Runtime:     "vllm",
				Spec: map[string]interface{}{
					"resources": map[string]interface{}{
						"gpu": map[string]interface{}{
							"count": 1,
						},
					},
				},
			},
			{
				ID:          "2",
				Name:        "llama-3-8b",
				DisplayName: "Llama 3 8B",
				ModelID:     "meta-llama/Llama-3-8B",
				Runtime:     "vllm",
				Spec: map[string]interface{}{
					"resources": map[string]interface{}{
						"gpu": map[string]interface{}{
							"count": 1,
						},
					},
				},
			},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ListRecipesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Recipes) != 2 {
		t.Errorf("expected 2 recipes, got %d", len(response.Recipes))
	}
}

func TestRecipeHandler_ListRecipes_FilterByRuntime(t *testing.T) {
	svc := &mockService{
		recipes: []Recipe{
			{
				ID:      "1",
				Name:    "mistral-7b-vllm",
				Runtime: "vllm",
			},
			{
				ID:      "2",
				Name:    "florence-triton",
				Runtime: "triton",
			},
			{
				ID:      "3",
				Name:    "llama-7b-vllm",
				Runtime: "vllm",
			},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes?runtime=vllm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ListRecipesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Recipes) != 2 {
		t.Errorf("expected 2 recipes with runtime=vllm, got %d", len(response.Recipes))
	}

	for _, recipe := range response.Recipes {
		if recipe.Runtime != "vllm" {
			t.Errorf("expected runtime vllm, got %s", recipe.Runtime)
		}
	}
}

func TestRecipeHandler_GetRecipe_Success(t *testing.T) {
	svc := &mockService{
		recipes: []Recipe{
			{
				ID:          "1",
				Name:        "mistral-7b-instruct",
				DisplayName: "Mistral 7B Instruct",
				ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
				Runtime:     "vllm",
				Spec: map[string]interface{}{
					"resources": map[string]interface{}{
						"gpu": map[string]interface{}{
							"count":  1,
							"vendor": "nvidia",
						},
					},
				},
			},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes/mistral-7b-instruct", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var recipe RecipeResponse
	if err := json.NewDecoder(w.Body).Decode(&recipe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if recipe.Name != "mistral-7b-instruct" {
		t.Errorf("expected name mistral-7b-instruct, got %s", recipe.Name)
	}

	if recipe.Runtime != "vllm" {
		t.Errorf("expected runtime vllm, got %s", recipe.Runtime)
	}
}

func TestRecipeHandler_GetRecipe_NotFound(t *testing.T) {
	svc := &mockService{recipes: []Recipe{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRecipeHandler_CreateRecipe_Success(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	body := `{
		"name": "test-recipe",
		"displayName": "Test Recipe",
		"description": "Test recipe for unit tests",
		"modelID": "test/model",
		"runtime": "vllm",
		"spec": {
			"resources": {
				"gpu": {
					"count": 1,
					"vendor": "nvidia"
				}
			}
		}
	}`
	req := httptest.NewRequest("POST", "/recipes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var recipe RecipeResponse
	if err := json.NewDecoder(w.Body).Decode(&recipe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if recipe.Name != "test-recipe" {
		t.Errorf("expected name test-recipe, got %s", recipe.Name)
	}

	if recipe.Runtime != "vllm" {
		t.Errorf("expected runtime vllm, got %s", recipe.Runtime)
	}
}

func TestRecipeHandler_CreateRecipe_InvalidJSON(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	req := httptest.NewRequest("POST", "/recipes", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRecipeHandler_CreateRecipe_ValidationError(t *testing.T) {
	svc := &mockService{
		err: errors.New("validation error: name is required"),
	}
	r := setupRouter(svc)

	body := `{
		"name": "",
		"modelID": "test/model",
		"runtime": "vllm"
	}`
	req := httptest.NewRequest("POST", "/recipes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestRecipeHandler_UpdateRecipe_Success(t *testing.T) {
	svc := &mockService{
		recipes: []Recipe{
			{
				ID:          "1",
				Name:        "test-recipe",
				DisplayName: "Old Name",
				Runtime:     "vllm",
			},
		},
	}
	r := setupRouter(svc)

	body := `{
		"displayName": "New Name",
		"description": "Updated description"
	}`
	req := httptest.NewRequest("PUT", "/recipes/test-recipe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var recipe RecipeResponse
	if err := json.NewDecoder(w.Body).Decode(&recipe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if recipe.DisplayName != "New Name" {
		t.Errorf("expected displayName 'New Name', got %s", recipe.DisplayName)
	}
}

func TestRecipeHandler_UpdateRecipe_NotFound(t *testing.T) {
	svc := &mockService{recipes: []Recipe{}}
	r := setupRouter(svc)

	body := `{"displayName": "New Name"}`
	req := httptest.NewRequest("PUT", "/recipes/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRecipeHandler_DeleteRecipe_Success(t *testing.T) {
	svc := &mockService{
		recipes: []Recipe{
			{
				ID:   "1",
				Name: "test-recipe",
			},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("DELETE", "/recipes/test-recipe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestRecipeHandler_DeleteRecipe_NotFound(t *testing.T) {
	svc := &mockService{recipes: []Recipe{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("DELETE", "/recipes/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRecipeHandler_ValidateRecipe_Valid(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	body := `{
		"name": "test-recipe",
		"modelID": "test/model",
		"runtime": "vllm",
		"spec": {
			"resources": {
				"gpu": {
					"count": 1
				}
			}
		}
	}`
	req := httptest.NewRequest("POST", "/recipes/test-recipe/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result ValidationResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected recipe to be valid, got invalid with errors: %v", result.Errors)
	}

	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

func TestRecipeHandler_ValidateRecipe_Invalid(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	body := `{
		"name": "",
		"modelID": "",
		"runtime": ""
	}`
	req := httptest.NewRequest("POST", "/recipes/test-recipe/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result ValidationResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected recipe to be invalid")
	}

	if len(result.Errors) == 0 {
		t.Error("expected validation errors, got none")
	}

	// Check that expected errors are present
	expectedErrors := map[string]bool{
		"name is required":    false,
		"modelID is required": false,
		"runtime is required": false,
	}

	for _, err := range result.Errors {
		if _, exists := expectedErrors[err]; exists {
			expectedErrors[err] = true
		}
	}

	for errMsg, found := range expectedErrors {
		if !found {
			t.Errorf("expected error '%s' not found in validation errors", errMsg)
		}
	}
}

func TestRecipeHandler_ListRecipeDeployments_Success(t *testing.T) {
	svc := &mockService{
		recipes: []Recipe{
			{
				ID:   "1",
				Name: "test-recipe",
			},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes/test-recipe/deployments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ListDeploymentsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// For now, expect empty list
	if response.Deployments == nil {
		t.Error("expected deployments to be non-nil")
	}
}

func TestRecipeHandler_ListRecipeDeployments_NotFound(t *testing.T) {
	svc := &mockService{recipes: []Recipe{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes/nonexistent/deployments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestRecipeHandler_ContentType(t *testing.T) {
	svc := &mockService{recipes: []Recipe{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/recipes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}
