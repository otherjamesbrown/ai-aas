package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
	"go.uber.org/zap"
)

// ModelHandler handles model registry HTTP requests
type ModelHandler struct {
	svc    *service.ModelRegistryService
	logger *zap.Logger
}

// NewModelHandler creates a new model handler
func NewModelHandler(svc *service.ModelRegistryService, logger *zap.Logger) *ModelHandler {
	return &ModelHandler{
		svc:    svc,
		logger: logger,
	}
}

// Register handles POST /v1/registry/models
func (h *ModelHandler) Register(w http.ResponseWriter, r *http.Request) {
	var reg domain.ModelRegistration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate required fields
	errors := h.validateRegistration(&reg)
	if len(errors) > 0 {
		api.WriteValidationError(w, r, errors)
		return
	}

	model, created, err := h.svc.Register(r.Context(), &reg)
	if err != nil {
		h.logger.Error("failed to register model", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	api.WriteJSON(w, status, model)
}

// List handles GET /v1/registry/models
func (h *ModelHandler) List(w http.ResponseWriter, r *http.Request) {
	params := domain.ModelListParams{
		Environment: r.URL.Query().Get("environment"),
		Status:      r.URL.Query().Get("status"),
		Limit:       parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset:      parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	// Validate environment
	if params.Environment != "" && !domain.IsValidEnvironment(params.Environment) {
		api.WriteValidationError(w, r, []api.ValidationError{
			{Field: "environment", Message: "Invalid environment. Must be one of: development, staging, production"},
		})
		return
	}

	// Validate status
	if params.Status != "" && !domain.IsValidStatus(params.Status) {
		api.WriteValidationError(w, r, []api.ValidationError{
			{Field: "status", Message: "Invalid status. Must be one of: ready, degraded, offline, pending"},
		})
		return
	}

	response, err := h.svc.List(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to list models", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/registry/models/{model_name}
func (h *ModelHandler) Get(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	environment := r.URL.Query().Get("environment")

	if environment == "" {
		api.WriteValidationError(w, r, []api.ValidationError{
			{Field: "environment", Message: "environment query parameter is required"},
		})
		return
	}

	model, err := h.svc.Get(r.Context(), modelName, environment)
	if err != nil {
		h.logger.Error("failed to get model", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if model == nil {
		api.WriteNotFound(w, r, "Model", modelName)
		return
	}

	api.WriteJSON(w, http.StatusOK, model)
}

// Update handles PATCH /v1/registry/models/{model_name}
func (h *ModelHandler) Update(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	environment := r.URL.Query().Get("environment")

	if environment == "" {
		api.WriteValidationError(w, r, []api.ValidationError{
			{Field: "environment", Message: "environment query parameter is required"},
		})
		return
	}

	var update domain.ModelUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate status if provided
	if update.DeploymentStatus != nil && !domain.IsValidStatus(*update.DeploymentStatus) {
		api.WriteValidationError(w, r, []api.ValidationError{
			{Field: "deployment_status", Message: "Invalid status. Must be one of: ready, degraded, offline, pending"},
		})
		return
	}

	model, err := h.svc.Update(r.Context(), modelName, environment, &update)
	if err != nil {
		h.logger.Error("failed to update model", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if model == nil {
		api.WriteNotFound(w, r, "Model", modelName)
		return
	}

	api.WriteJSON(w, http.StatusOK, model)
}

// Delete handles DELETE /v1/registry/models/{model_name}
func (h *ModelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model_name")
	environment := r.URL.Query().Get("environment")

	if environment == "" {
		api.WriteValidationError(w, r, []api.ValidationError{
			{Field: "environment", Message: "environment query parameter is required"},
		})
		return
	}

	// Check if model exists first
	model, err := h.svc.Get(r.Context(), modelName, environment)
	if err != nil {
		h.logger.Error("failed to check model", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if model == nil {
		api.WriteNotFound(w, r, "Model", modelName)
		return
	}

	if err := h.svc.Delete(r.Context(), modelName, environment); err != nil {
		h.logger.Error("failed to delete model", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ModelHandler) validateRegistration(reg *domain.ModelRegistration) []api.ValidationError {
	var errors []api.ValidationError

	if reg.ModelName == "" {
		errors = append(errors, api.ValidationError{Field: "model_name", Message: "model_name is required"})
	} else if len(reg.ModelName) > 255 {
		errors = append(errors, api.ValidationError{Field: "model_name", Message: "model_name must be 255 characters or less"})
	}

	if reg.DeploymentEndpoint == "" {
		errors = append(errors, api.ValidationError{Field: "deployment_endpoint", Message: "deployment_endpoint is required"})
	}

	if reg.DeploymentEnvironment == "" {
		errors = append(errors, api.ValidationError{Field: "deployment_environment", Message: "deployment_environment is required"})
	} else if !domain.IsValidEnvironment(reg.DeploymentEnvironment) {
		errors = append(errors, api.ValidationError{Field: "deployment_environment", Message: "Invalid environment. Must be one of: development, staging, production"})
	}

	if reg.DeploymentNamespace == "" {
		errors = append(errors, api.ValidationError{Field: "deployment_namespace", Message: "deployment_namespace is required"})
	}

	if reg.DeploymentStatus != "" && !domain.IsValidStatus(reg.DeploymentStatus) {
		errors = append(errors, api.ValidationError{Field: "deployment_status", Message: "Invalid status. Must be one of: ready, degraded, offline, pending"})
	}

	return errors
}

func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

