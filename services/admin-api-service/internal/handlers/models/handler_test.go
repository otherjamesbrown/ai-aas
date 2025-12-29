package models

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// mockService implements the Service interface for testing
type mockService struct {
	models       []Model
	credentials  []Credential
	cacheEntries []CacheEntry
	err          error
}

func (m *mockService) ListModels(opts ListModelsOptions) ([]Model, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.models, nil
}

func (m *mockService) GetModel(name string) (*Model, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, model := range m.models {
		if model.Name == name {
			return &model, nil
		}
	}
	return nil, ErrModelNotFound
}

func (m *mockService) AddModel(req AddModelRequest) (*Model, error) {
	if m.err != nil {
		return nil, m.err
	}
	model := Model{
		ID:           "test-id",
		Name:         req.Name,
		HFModelID:    req.HFModelID,
		RequiresAuth: req.RequiresAuth,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.models = append(m.models, model)
	return &model, nil
}

func (m *mockService) RemoveModel(name string, force bool) error {
	if m.err != nil {
		return m.err
	}
	for i, model := range m.models {
		if model.Name == name {
			m.models = append(m.models[:i], m.models[i+1:]...)
			return nil
		}
	}
	return testErrModelNotFound
}

func (m *mockService) RenameModel(name string, req RenameModelRequest) (*RenameModelResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i, model := range m.models {
		if model.Name == name {
			m.models[i].Name = req.NewName
			return &RenameModelResponse{
				OldName:       name,
				NewName:       req.NewName,
				CacheMigrated: req.MigrateCache,
			}, nil
		}
	}
	return nil, testErrModelNotFound
}

func (m *mockService) GetModelCache(name string) ([]CacheEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cacheEntries, nil
}

func (m *mockService) PullModel(name string, opts PullOptions) (*PullJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &PullJob{
		ID:        "job-123",
		ModelName: name,
		Status:    "pending",
		StartedAt: time.Now(),
	}, nil
}

func (m *mockService) ListPullJobs(modelName string) ([]PullJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []PullJob{}, nil
}

func (m *mockService) GetPullJob(modelName, jobID string) (*PullJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &PullJob{
		ID:        jobID,
		ModelName: modelName,
		Status:    "completed",
		StartedAt: time.Now(),
	}, nil
}

func (m *mockService) CancelPullJob(jobID string) error {
	return m.err
}

func (m *mockService) VerifyCache(name string, version string) (*VerifyResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &VerifyResult{
		Valid:         true,
		FilesChecked:  10,
		ChecksumMatch: true,
		VerifiedAt:    time.Now(),
	}, nil
}

func (m *mockService) ListCredentials() ([]Credential, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.credentials, nil
}

func (m *mockService) SetCredential(credType string, value string, metadata map[string]interface{}) error {
	if m.err != nil {
		return m.err
	}
	m.credentials = append(m.credentials, Credential{
		Type:      credType,
		Masked:    "****configured****",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	return nil
}

func (m *mockService) TestCredential(credType string) (*CredentialTestResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &CredentialTestResult{
		Valid:   true,
		Message: "Credential is valid",
	}, nil
}

func (m *mockService) DeleteCredential(credType string) error {
	if m.err != nil {
		return m.err
	}
	for i, cred := range m.credentials {
		if cred.Type == credType {
			m.credentials = append(m.credentials[:i], m.credentials[i+1:]...)
			return nil
		}
	}
	return testErrCredentialNotFound
}

// Deployment operations (stub implementations for testing)
func (m *mockService) ListDeployments(opts ListDeploymentsOptions) ([]Deployment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []Deployment{}, nil
}

func (m *mockService) GetDeployment(modelName, environment string) (*Deployment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &Deployment{
		ModelName:   modelName,
		Environment: environment,
		Status:      "ready",
	}, nil
}

func (m *mockService) CreateDeployment(req CreateDeploymentRequest) (*Deployment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &Deployment{
		ModelName:   req.ModelName,
		Environment: req.Environment,
		Status:      "deploying",
	}, nil
}

func (m *mockService) UpdateDeploymentStatus(modelName, environment string, req UpdateDeploymentStatusRequest) error {
	return m.err
}

func (m *mockService) DeleteDeployment(modelName, environment string) error {
	return m.err
}

// Sentinel errors for mock (use different names from handler.go)
var testErrModelNotFound = &modelError{"model not found"}
var testErrCredentialNotFound = &modelError{"credential not found"}

type modelError struct {
	msg string
}

func (e *modelError) Error() string { return e.msg }

// Test setup helpers
func setupRouter(svc Service) *chi.Mux {
	r := chi.NewRouter()
	h := NewHandler(svc)
	h.RegisterRoutes(r)
	return r
}

// Model endpoint tests

func TestListModels(t *testing.T) {
	svc := &mockService{
		models: []Model{
			{ID: "1", Name: "llama-3-8b", HFModelID: "meta-llama/Llama-3-8B"},
			{ID: "2", Name: "mistral-7b", HFModelID: "mistralai/Mistral-7B-v0.1"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var models []Model
	if err := json.NewDecoder(w.Body).Decode(&models); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestListModelsWithFilters(t *testing.T) {
	svc := &mockService{models: []Model{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/models?cached=true&deployed=true&environment=development", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetModel(t *testing.T) {
	svc := &mockService{
		models: []Model{
			{ID: "1", Name: "llama-3-8b", HFModelID: "meta-llama/Llama-3-8B"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/models/llama-3-8b", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var model Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if model.Name != "llama-3-8b" {
		t.Errorf("expected name llama-3-8b, got %s", model.Name)
	}
}

func TestGetModelNotFound(t *testing.T) {
	svc := &mockService{models: []Model{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/models/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAddModel(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	body := `{"name": "test-model", "hf_model_id": "test/model", "requires_auth": true}`
	req := httptest.NewRequest("POST", "/models", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var model Model
	if err := json.NewDecoder(w.Body).Decode(&model); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if model.Name != "test-model" {
		t.Errorf("expected name test-model, got %s", model.Name)
	}
}

func TestAddModelInvalidBody(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	req := httptest.NewRequest("POST", "/models", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteModel(t *testing.T) {
	svc := &mockService{
		models: []Model{
			{ID: "1", Name: "llama-3-8b"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("DELETE", "/models/llama-3-8b", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestDeleteModelWithForce(t *testing.T) {
	svc := &mockService{
		models: []Model{
			{ID: "1", Name: "llama-3-8b"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("DELETE", "/models/llama-3-8b?force=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// Cache endpoint tests

func TestGetModelCache(t *testing.T) {
	svc := &mockService{
		cacheEntries: []CacheEntry{
			{ID: "1", Version: "abc123", Status: "ready"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/models/llama-3-8b/cache", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var entries []CacheEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 cache entry, got %d", len(entries))
	}
}

func TestPullModel(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	body := `{"revision": "main"}`
	req := httptest.NewRequest("POST", "/models/llama-3-8b/pull", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	var job PullJob
	if err := json.NewDecoder(w.Body).Decode(&job); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if job.ModelName != "llama-3-8b" {
		t.Errorf("expected model name llama-3-8b, got %s", job.ModelName)
	}
}

func TestPullModelEmptyBody(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	req := httptest.NewRequest("POST", "/models/llama-3-8b/pull", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}
}

func TestVerifyCache(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	req := httptest.NewRequest("POST", "/models/llama-3-8b/cache/verify?version=abc123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result VerifyResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Error("expected cache to be valid")
	}
}

// Credentials endpoint tests

func TestListCredentials(t *testing.T) {
	svc := &mockService{
		credentials: []Credential{
			{Type: "huggingface", Masked: "****configured****"},
			{Type: "s3", Masked: "****configured****"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/credentials", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var creds []Credential
	if err := json.NewDecoder(w.Body).Decode(&creds); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(creds) != 2 {
		t.Errorf("expected 2 credentials, got %d", len(creds))
	}
}

func TestSetCredential(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	body := `{"type": "huggingface", "value": "hf_secret_token"}`
	req := httptest.NewRequest("POST", "/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetCredentialInvalidBody(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	req := httptest.NewRequest("POST", "/credentials", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestTestCredential(t *testing.T) {
	svc := &mockService{}
	r := setupRouter(svc)

	req := httptest.NewRequest("POST", "/credentials/huggingface/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result CredentialTestResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Error("expected credential to be valid")
	}
}

func TestDeleteCredential(t *testing.T) {
	svc := &mockService{
		credentials: []Credential{
			{Type: "huggingface"},
		},
	}
	r := setupRouter(svc)

	req := httptest.NewRequest("DELETE", "/credentials/huggingface", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// Content-Type tests

func TestResponseContentType(t *testing.T) {
	svc := &mockService{models: []Model{}}
	r := setupRouter(svc)

	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}
