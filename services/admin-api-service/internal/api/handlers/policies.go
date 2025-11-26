package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/middleware"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
	"go.uber.org/zap"
)

// PolicyHandler handles routing policy HTTP requests
type PolicyHandler struct {
	svc    *service.PolicyService
	logger *zap.Logger
}

// NewPolicyHandler creates a new policy handler
func NewPolicyHandler(svc *service.PolicyService, logger *zap.Logger) *PolicyHandler {
	return &PolicyHandler{
		svc:    svc,
		logger: logger,
	}
}

// Create handles POST /v1/routing/policies
func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var create domain.PolicyCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate
	errors := h.validateCreate(&create)
	if len(errors) > 0 {
		api.WriteValidationError(w, r, errors)
		return
	}

	createdBy := middleware.GetAPIKeyID(r.Context())
	policy, err := h.svc.Create(r.Context(), &create, createdBy)
	if err != nil {
		h.logger.Error("failed to create policy", zap.Error(err))
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Validation Failed", err.Error())
		return
	}

	api.WriteJSON(w, http.StatusCreated, policy)
}

// List handles GET /v1/routing/policies
func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	params := domain.PolicyListParams{
		Model:          r.URL.Query().Get("model"),
		OrganizationID: r.URL.Query().Get("organization_id"),
		Limit:          parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset:         parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	if enabledStr := r.URL.Query().Get("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		params.Enabled = &enabled
	}

	response, err := h.svc.List(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to list policies", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/routing/policies/{policy_id}
func (h *PolicyHandler) Get(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policy_id")

	policy, err := h.svc.Get(r.Context(), policyID)
	if err != nil {
		h.logger.Error("failed to get policy", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if policy == nil {
		api.WriteNotFound(w, r, "Policy", policyID)
		return
	}

	api.WriteJSON(w, http.StatusOK, policy)
}

// Update handles PATCH /v1/routing/policies/{policy_id}
func (h *PolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policy_id")

	var update domain.PolicyUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate
	errors := h.validateUpdate(&update)
	if len(errors) > 0 {
		api.WriteValidationError(w, r, errors)
		return
	}

	updatedBy := middleware.GetAPIKeyID(r.Context())
	policy, err := h.svc.Update(r.Context(), policyID, &update, updatedBy)
	if err != nil {
		h.logger.Error("failed to update policy", zap.Error(err))
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Validation Failed", err.Error())
		return
	}

	if policy == nil {
		api.WriteNotFound(w, r, "Policy", policyID)
		return
	}

	api.WriteJSON(w, http.StatusOK, policy)
}

// Delete handles DELETE /v1/routing/policies/{policy_id}
func (h *PolicyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policy_id")

	// Check if policy exists
	policy, err := h.svc.Get(r.Context(), policyID)
	if err != nil {
		h.logger.Error("failed to check policy", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if policy == nil {
		api.WriteNotFound(w, r, "Policy", policyID)
		return
	}

	if err := h.svc.Delete(r.Context(), policyID); err != nil {
		h.logger.Error("failed to delete policy", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Activate handles POST /v1/routing/policies/{policy_id}/activate
func (h *PolicyHandler) Activate(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policy_id")
	updatedBy := middleware.GetAPIKeyID(r.Context())

	response, err := h.svc.Activate(r.Context(), policyID, updatedBy)
	if err != nil {
		h.logger.Error("failed to activate policy", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if response == nil {
		api.WriteNotFound(w, r, "Policy", policyID)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// Deactivate handles POST /v1/routing/policies/{policy_id}/deactivate
func (h *PolicyHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policy_id")
	updatedBy := middleware.GetAPIKeyID(r.Context())

	response, err := h.svc.Deactivate(r.Context(), policyID, updatedBy)
	if err != nil {
		h.logger.Error("failed to deactivate policy", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	if response == nil {
		api.WriteNotFound(w, r, "Policy", policyID)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// Sync handles GET /v1/routing/policies/sync
func (h *PolicyHandler) Sync(w http.ResponseWriter, r *http.Request) {
	sinceVersion := parseIntParam(r.URL.Query().Get("since_version"), 0)
	environment := r.URL.Query().Get("environment")

	response, err := h.svc.Sync(r.Context(), sinceVersion, environment)
	if err != nil {
		h.logger.Error("failed to sync policies", zap.Error(err))
		api.WriteInternalError(w, r)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// Validate handles POST /v1/routing/policies/validate
func (h *PolicyHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var create domain.PolicyCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, api.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	response := h.svc.Validate(r.Context(), &create)
	api.WriteJSON(w, http.StatusOK, response)
}

func (h *PolicyHandler) validateCreate(create *domain.PolicyCreate) []api.ValidationError {
	var errors []api.ValidationError

	if create.Model == "" {
		errors = append(errors, api.ValidationError{Field: "model", Message: "model is required"})
	}

	if len(create.Backends) == 0 {
		errors = append(errors, api.ValidationError{Field: "backends", Message: "at least one backend is required"})
	} else if len(create.Backends) > 10 {
		errors = append(errors, api.ValidationError{Field: "backends", Message: "maximum 10 backends allowed"})
	}

	if !domain.ValidateBackendWeights(create.Backends) && len(create.Backends) > 0 {
		errors = append(errors, api.ValidationError{Field: "backends", Message: "backend weights must sum to 100"})
	}

	if create.FailoverThreshold != 0 && (create.FailoverThreshold < 1 || create.FailoverThreshold > 10) {
		errors = append(errors, api.ValidationError{Field: "failover_threshold", Message: "failover_threshold must be between 1 and 10"})
	}

	return errors
}

func (h *PolicyHandler) validateUpdate(update *domain.PolicyUpdate) []api.ValidationError {
	var errors []api.ValidationError

	if update.Backends != nil {
		if len(update.Backends) == 0 {
			errors = append(errors, api.ValidationError{Field: "backends", Message: "at least one backend is required"})
		} else if len(update.Backends) > 10 {
			errors = append(errors, api.ValidationError{Field: "backends", Message: "maximum 10 backends allowed"})
		} else if !domain.ValidateBackendWeights(update.Backends) {
			errors = append(errors, api.ValidationError{Field: "backends", Message: "backend weights must sum to 100"})
		}
	}

	if update.FailoverThreshold != nil && (*update.FailoverThreshold < 1 || *update.FailoverThreshold > 10) {
		errors = append(errors, api.ValidationError{Field: "failover_threshold", Message: "failover_threshold must be between 1 and 10"})
	}

	return errors
}

