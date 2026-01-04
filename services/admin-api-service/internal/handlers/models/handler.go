// Package models provides HTTP handlers for model management operations.
// Implements the /models/* endpoints per specs/020-model-management/contracts/admin-api.yaml
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Error variables for handler error mapping
var (
	ErrModelNotFound    = errors.New("model not found")
	ErrModelDeployed    = errors.New("model has active deployments")
	ErrModelNameExists  = errors.New("model name already exists")
	ErrInvalidModelName = errors.New("invalid model name format")
)

// Handler handles model management HTTP requests
type Handler struct {
	service Service
}

// Service defines the interface for model management operations
type Service interface {
	// Registry operations
	ListModels(opts ListModelsOptions) ([]Model, error)
	GetModel(name string) (*Model, error)
	AddModel(req AddModelRequest) (*Model, error)
	RemoveModel(name string, force bool) error
	RenameModel(name string, req RenameModelRequest) (*RenameModelResponse, error)

	// Cache operations
	GetModelCache(name string) ([]CacheEntry, error)
	PullModel(name string, opts PullOptions) (*PullJob, error)
	VerifyCache(name string, version string) (*VerifyResult, error)

	// Pull job operations
	ListPullJobs(modelName string) ([]PullJob, error)
	GetPullJob(modelName, jobID string) (*PullJob, error)
	CancelPullJob(jobID string) error

	// Credentials operations
	ListCredentials() ([]Credential, error)
	SetCredential(credType string, value string, metadata map[string]interface{}) error
	TestCredential(credType string) (*CredentialTestResult, error)
	DeleteCredential(credType string) error

	// Deployment operations
	ListDeployments(opts ListDeploymentsOptions) ([]Deployment, error)
	GetDeployment(modelName, environment string) (*Deployment, error)
	CreateDeployment(req CreateDeploymentRequest) (*Deployment, error)
	UpdateDeploymentStatus(modelName, environment string, req UpdateDeploymentStatusRequest) error
	DeleteDeployment(modelName, environment string) error
}

// NewHandler creates a new models handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the model management routes
// Note: This handler is mounted at /v1/models, so paths here are relative to that
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Model registry endpoints
	r.Get("/", h.ListModels)
	r.Post("/", h.AddModel)
	r.Get("/{name}", h.GetModel)
	r.Delete("/{name}", h.DeleteModel)
	r.Post("/{name}/rename", h.RenameModel)

	// Cache endpoints
	r.Get("/{name}/cache", h.GetModelCache)
	r.Post("/{name}/pull", h.PullModel)
	r.Get("/{name}/pull", h.ListPullJobs)
	r.Get("/{name}/pull/{job_id}", h.GetPullJob)
	r.Delete("/{name}/pull/{job_id}", h.CancelPullJob)
	r.Post("/{name}/cache/verify", h.VerifyCache)

	// Credentials endpoints (mounted under /v1/models but logically separate)
	r.Get("/credentials", h.ListCredentials)
	r.Post("/credentials", h.SetCredential)
	r.Post("/credentials/{type}/test", h.TestCredential)
	r.Delete("/credentials/{type}", h.DeleteCredential)

	// Deployment endpoints (mounted under /v1/models but logically separate)
	r.Get("/deployments", h.ListDeployments)
	r.Post("/deployments", h.CreateDeployment)
	r.Get("/deployments/{model_name}/{environment}", h.GetDeployment)
	r.Put("/deployments/{model_name}/{environment}", h.UpdateDeploymentStatus)
	r.Delete("/deployments/{model_name}/{environment}", h.DeleteDeployment)
}

// ListModels handles GET /models
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	opts := ListModelsOptions{
		Cached:      r.URL.Query().Get("cached") == "true",
		Deployed:    r.URL.Query().Get("deployed") == "true",
		Orphaned:    r.URL.Query().Get("orphaned") == "true",
		Environment: r.URL.Query().Get("environment"),
	}

	models, err := h.service.ListModels(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list models", err)
		return
	}

	writeJSON(w, http.StatusOK, models)
}

// GetModel handles GET /models/{name}
func (h *Handler) GetModel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	model, err := h.service.GetModel(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "model not found", err)
		return
	}

	writeJSON(w, http.StatusOK, model)
}

// AddModel handles POST /models
func (h *Handler) AddModel(w http.ResponseWriter, r *http.Request) {
	var req AddModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	model, err := h.service.AddModel(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add model", err)
		return
	}

	writeJSON(w, http.StatusCreated, model)
}

// DeleteModel handles DELETE /models/{name}
func (h *Handler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	force := r.URL.Query().Get("force") == "true"

	if err := h.service.RemoveModel(name, force); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove model", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetModelCache handles GET /models/{name}/cache
func (h *Handler) GetModelCache(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	entries, err := h.service.GetModelCache(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "model or cache not found", err)
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

// PullModel handles POST /models/{name}/pull
func (h *Handler) PullModel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var opts PullOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		// Empty body is OK, use defaults
		opts = PullOptions{Revision: "main"}
	}

	job, err := h.service.PullModel(name, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start pull", err)
		return
	}

	writeJSON(w, http.StatusAccepted, job)
}

// ListPullJobs handles GET /models/{name}/pull
func (h *Handler) ListPullJobs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	jobs, err := h.service.ListPullJobs(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pull jobs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"jobs": jobs,
	})
}

// GetPullJob handles GET /models/{name}/pull/{job_id}
func (h *Handler) GetPullJob(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	jobID := chi.URLParam(r, "job_id")

	job, err := h.service.GetPullJob(name, jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "pull job not found", err)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// CancelPullJob handles DELETE /models/{name}/pull/{job_id}
func (h *Handler) CancelPullJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")

	err := h.service.CancelPullJob(jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel pull job", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VerifyCache handles POST /models/{name}/cache/verify
func (h *Handler) VerifyCache(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := r.URL.Query().Get("version")

	result, err := h.service.VerifyCache(name, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify cache", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListCredentials handles GET /credentials
func (h *Handler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := h.service.ListCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list credentials", err)
		return
	}

	writeJSON(w, http.StatusOK, creds)
}

// SetCredential handles POST /credentials
func (h *Handler) SetCredential(w http.ResponseWriter, r *http.Request) {
	var req SetCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := h.service.SetCredential(req.Type, req.Value, req.Metadata); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set credential", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestCredential handles POST /credentials/{type}/test
func (h *Handler) TestCredential(w http.ResponseWriter, r *http.Request) {
	credType := chi.URLParam(r, "type")

	result, err := h.service.TestCredential(credType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to test credential", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteCredential handles DELETE /credentials/{type}
func (h *Handler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	credType := chi.URLParam(r, "type")

	if err := h.service.DeleteCredential(credType); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete credential", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RenameModel handles POST /models/{name}/rename
func (h *Handler) RenameModel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req RenameModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate request
	if req.NewName == "" {
		writeError(w, http.StatusBadRequest, "new_name is required", fmt.Errorf("new_name cannot be empty"))
		return
	}

	result, err := h.service.RenameModel(name, req)
	if err != nil {
		// Map specific errors to appropriate HTTP status codes
		switch {
		case errors.Is(err, ErrModelNotFound):
			writeError(w, http.StatusNotFound, "model not found", err)
		case errors.Is(err, ErrModelDeployed):
			writeError(w, http.StatusConflict, "cannot rename model with active deployments", err)
		case errors.Is(err, ErrModelNameExists):
			writeError(w, http.StatusConflict, "model name already exists", err)
		case errors.Is(err, ErrInvalidModelName):
			writeError(w, http.StatusBadRequest, "invalid model name format", err)
		default:
			writeError(w, http.StatusInternalServerError, "failed to rename model", err)
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListDeployments handles GET /deployments
func (h *Handler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	opts := ListDeploymentsOptions{
		Environment: r.URL.Query().Get("environment"),
		ModelName:   r.URL.Query().Get("model_name"),
		Status:      r.URL.Query().Get("status"),
	}

	if enabledStr := r.URL.Query().Get("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		opts.Enabled = &enabled
	}

	deployments, err := h.service.ListDeployments(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deployments", err)
		return
	}

	writeJSON(w, http.StatusOK, deployments)
}

// GetDeployment handles GET /deployments/{model_name}/{environment}
func (h *Handler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	environment := chi.URLParam(r, "environment")

	deployment, err := h.service.GetDeployment(modelName, environment)
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found", err)
		return
	}

	writeJSON(w, http.StatusOK, deployment)
}

// CreateDeployment handles POST /deployments
func (h *Handler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req CreateDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	deployment, err := h.service.CreateDeployment(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create deployment", err)
		return
	}

	writeJSON(w, http.StatusCreated, deployment)
}

// UpdateDeploymentStatus handles PUT /deployments/{model_name}/{environment}
func (h *Handler) UpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	environment := chi.URLParam(r, "environment")

	var req UpdateDeploymentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if err := h.service.UpdateDeploymentStatus(modelName, environment, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update deployment status", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteDeployment handles DELETE /deployments/{model_name}/{environment}
func (h *Handler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	environment := chi.URLParam(r, "environment")

	if err := h.service.DeleteDeployment(modelName, environment); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete deployment", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"details": err.Error(),
	})
}
