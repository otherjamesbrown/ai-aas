// Package engines provides HTTP handlers for inference engine management operations.
package engines

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/services/engines"
)

// Handler handles inference engine HTTP requests
type Handler struct {
	service *engines.Service
}

// NewHandler creates a new engines handler
func NewHandler(service *engines.Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the engine management routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Engine endpoints
	r.Get("/engines", h.ListEngines)
	r.Get("/engines/{name}", h.GetEngine)

	// Version endpoints
	r.Get("/engines/{name}/versions", h.ListVersions)
	r.Post("/engines/{name}/versions", h.AddVersion)
	r.Post("/engines/{name}/versions/{version}/set-default", h.SetDefaultVersion)
	r.Delete("/engines/{name}/versions/{version}", h.DeleteVersion)

	// Config endpoints
	r.Get("/engine-configs", h.ListConfigs)
	r.Post("/engine-configs", h.CreateConfig)
	r.Get("/engine-configs/{name}", h.GetConfig)
	r.Put("/engine-configs/{name}", h.UpdateConfig)
	r.Delete("/engine-configs/{name}", h.DeleteConfig)
}

// ListEngines handles GET /engines
func (h *Handler) ListEngines(w http.ResponseWriter, r *http.Request) {
	engineList, err := h.service.ListEngines(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list engines", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"engines": engineList,
	})
}

// GetEngine handles GET /engines/{name}
func (h *Handler) GetEngine(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	engine, err := h.service.GetEngine(r.Context(), name)
	if errors.Is(err, engines.ErrEngineNotFound) {
		writeError(w, http.StatusNotFound, "engine not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get engine", err)
		return
	}

	writeJSON(w, http.StatusOK, engine)
}

// ListVersions handles GET /engines/{name}/versions
func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	versions, err := h.service.ListVersions(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list versions", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"versions": versions,
	})
}

// AddVersionRequest is the request body for adding a version
type AddVersionRequest struct {
	Version        string  `json:"version"`
	ContainerImage string  `json:"container_image"`
	SetDefault     bool    `json:"set_default"`
	MinGPUMemoryGB *int    `json:"min_gpu_memory_gb,omitempty"`
	ReleaseNotes   *string `json:"release_notes,omitempty"`
}

// AddVersion handles POST /engines/{name}/versions
func (h *Handler) AddVersion(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req AddVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Version == "" || req.ContainerImage == "" {
		writeError(w, http.StatusBadRequest, "version and container_image are required", nil)
		return
	}

	version, err := h.service.AddVersion(r.Context(), engines.AddVersionRequest{
		EngineName:     name,
		Version:        req.Version,
		ContainerImage: req.ContainerImage,
		SetDefault:     req.SetDefault,
		MinGPUMemoryGB: req.MinGPUMemoryGB,
		ReleaseNotes:   req.ReleaseNotes,
	})
	if errors.Is(err, engines.ErrEngineNotFound) {
		writeError(w, http.StatusNotFound, "engine not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add version", err)
		return
	}

	writeJSON(w, http.StatusCreated, version)
}

// SetDefaultVersion handles POST /engines/{name}/versions/{version}/set-default
func (h *Handler) SetDefaultVersion(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	err := h.service.SetDefaultVersion(r.Context(), name, version)
	if errors.Is(err, engines.ErrEngineNotFound) {
		writeError(w, http.StatusNotFound, "engine not found", err)
		return
	}
	if errors.Is(err, engines.ErrVersionNotFound) {
		writeError(w, http.StatusNotFound, "version not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set default version", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteVersion handles DELETE /engines/{name}/versions/{version}
func (h *Handler) DeleteVersion(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	err := h.service.DeleteVersion(r.Context(), name, version)
	if errors.Is(err, engines.ErrEngineNotFound) {
		writeError(w, http.StatusNotFound, "engine not found", err)
		return
	}
	if errors.Is(err, engines.ErrVersionNotFound) {
		writeError(w, http.StatusNotFound, "version not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete version", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListConfigs handles GET /engine-configs
func (h *Handler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.service.ListConfigs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list configs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configs": configs,
	})
}

// GetConfig handles GET /engine-configs/{name}
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	config, err := h.service.GetConfig(r.Context(), name)
	if errors.Is(err, engines.ErrConfigNotFound) {
		writeError(w, http.StatusNotFound, "config not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get config", err)
		return
	}

	writeJSON(w, http.StatusOK, config)
}

// CreateConfigRequest is the request body for creating a config
type CreateConfigRequest struct {
	Name                  string                 `json:"name"`
	EngineName            string                 `json:"engine"`
	Version               *string                `json:"version,omitempty"`
	Description           *string                `json:"description,omitempty"`
	GPUCount              int                    `json:"gpu_count"`
	GPUType               *string                `json:"gpu_type,omitempty"`
	MemoryGB              int                    `json:"memory_gb"`
	CPUCores              int                    `json:"cpu_cores"`
	EngineArgs            map[string]interface{} `json:"engine_args,omitempty"`
	MinReplicas           int                    `json:"min_replicas"`
	MaxReplicas           int                    `json:"max_replicas"`
	StartupTimeoutSeconds int                    `json:"startup_timeout_seconds"`
	IsDefault             bool                   `json:"is_default"`
}

// CreateConfig handles POST /engine-configs
func (h *Handler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var req CreateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Name == "" || req.EngineName == "" {
		writeError(w, http.StatusBadRequest, "name and engine are required", nil)
		return
	}

	// Set defaults
	if req.GPUCount == 0 {
		req.GPUCount = 1
	}
	if req.MemoryGB == 0 {
		req.MemoryGB = 16
	}
	if req.CPUCores == 0 {
		req.CPUCores = 4
	}
	if req.MinReplicas == 0 {
		req.MinReplicas = 1
	}
	if req.MaxReplicas == 0 {
		req.MaxReplicas = 1
	}
	if req.StartupTimeoutSeconds == 0 {
		req.StartupTimeoutSeconds = 900
	}

	config, err := h.service.CreateConfig(r.Context(), engines.CreateConfigRequest{
		Name:                  req.Name,
		EngineName:            req.EngineName,
		Version:               req.Version,
		Description:           req.Description,
		GPUCount:              req.GPUCount,
		GPUType:               req.GPUType,
		MemoryGB:              req.MemoryGB,
		CPUCores:              req.CPUCores,
		EngineArgs:            req.EngineArgs,
		MinReplicas:           req.MinReplicas,
		MaxReplicas:           req.MaxReplicas,
		StartupTimeoutSeconds: req.StartupTimeoutSeconds,
		IsDefault:             req.IsDefault,
	})
	if errors.Is(err, engines.ErrEngineNotFound) {
		writeError(w, http.StatusNotFound, "engine not found", err)
		return
	}
	if errors.Is(err, engines.ErrVersionNotFound) {
		writeError(w, http.StatusNotFound, "version not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create config", err)
		return
	}

	writeJSON(w, http.StatusCreated, config)
}

// UpdateConfigRequest is the request body for updating a config
type UpdateConfigRequest struct {
	Description           *string                `json:"description,omitempty"`
	GPUCount              *int                   `json:"gpu_count,omitempty"`
	GPUType               *string                `json:"gpu_type,omitempty"`
	MemoryGB              *int                   `json:"memory_gb,omitempty"`
	CPUCores              *int                   `json:"cpu_cores,omitempty"`
	EngineArgs            map[string]interface{} `json:"engine_args,omitempty"`
	MinReplicas           *int                   `json:"min_replicas,omitempty"`
	MaxReplicas           *int                   `json:"max_replicas,omitempty"`
	StartupTimeoutSeconds *int                   `json:"startup_timeout_seconds,omitempty"`
	IsDefault             *bool                  `json:"is_default,omitempty"`
}

// UpdateConfig handles PUT /engine-configs/{name}
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	config, err := h.service.UpdateConfig(r.Context(), name, engines.UpdateConfigRequest{
		Description:           req.Description,
		GPUCount:              req.GPUCount,
		GPUType:               req.GPUType,
		MemoryGB:              req.MemoryGB,
		CPUCores:              req.CPUCores,
		EngineArgs:            req.EngineArgs,
		MinReplicas:           req.MinReplicas,
		MaxReplicas:           req.MaxReplicas,
		StartupTimeoutSeconds: req.StartupTimeoutSeconds,
		IsDefault:             req.IsDefault,
	})
	if errors.Is(err, engines.ErrConfigNotFound) {
		writeError(w, http.StatusNotFound, "config not found", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update config", err)
		return
	}

	writeJSON(w, http.StatusOK, config)
}

// DeleteConfig handles DELETE /engine-configs/{name}
func (h *Handler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	err := h.service.DeleteConfig(r.Context(), name)
	if errors.Is(err, engines.ErrConfigNotFound) {
		writeError(w, http.StatusNotFound, "config not found", err)
		return
	}
	if errors.Is(err, engines.ErrConfigInUse) {
		writeError(w, http.StatusConflict, "config is in use by deployments", err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete config", err)
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
	errDetails := ""
	if err != nil {
		errDetails = err.Error()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"details": errDetails,
	})
}
