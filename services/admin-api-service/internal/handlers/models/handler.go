// Package models provides HTTP handlers for model management operations.
// Implements the /models/* endpoints per specs/020-model-management/contracts/admin-api.yaml
package models

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
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

	// Cache operations
	GetModelCache(name string) ([]CacheEntry, error)
	PullModel(name string, opts PullOptions) (*PullJob, error)
	VerifyCache(name string, version string) (*VerifyResult, error)

	// Pull job operations
	ListPullJobs(modelName string) ([]PullJob, error)
	GetPullJob(jobID string) (*PullJob, error)
	CancelPullJob(jobID string) error

	// Credentials operations
	ListCredentials() ([]Credential, error)
	SetCredential(credType string, value string, metadata map[string]interface{}) error
	TestCredential(credType string) (*CredentialTestResult, error)
	DeleteCredential(credType string) error
}

// NewHandler creates a new models handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the model management routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Model registry endpoints
	r.Get("/models", h.ListModels)
	r.Post("/models", h.AddModel)
	r.Get("/models/{name}", h.GetModel)
	r.Delete("/models/{name}", h.DeleteModel)

	// Cache endpoints
	r.Get("/models/{name}/cache", h.GetModelCache)
	r.Post("/models/{name}/pull", h.PullModel)
	r.Get("/models/{name}/pull", h.ListPullJobs)
	r.Get("/models/{name}/pull/{job_id}", h.GetPullJob)
	r.Delete("/models/{name}/pull/{job_id}", h.CancelPullJob)
	r.Post("/models/{name}/cache/verify", h.VerifyCache)

	// Credentials endpoints
	r.Get("/credentials", h.ListCredentials)
	r.Post("/credentials", h.SetCredential)
	r.Post("/credentials/{type}/test", h.TestCredential)
	r.Delete("/credentials/{type}", h.DeleteCredential)
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
	jobID := chi.URLParam(r, "job_id")

	job, err := h.service.GetPullJob(jobID)
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

