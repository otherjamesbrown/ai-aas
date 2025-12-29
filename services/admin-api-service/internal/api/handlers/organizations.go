package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/service"
	"go.uber.org/zap"
)

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// OrganizationHandler handles organization HTTP requests
type OrganizationHandler struct {
	svc    *service.OrganizationService
	logger *zap.Logger
}

// NewOrganizationHandler creates a new organization handler
func NewOrganizationHandler(svc *service.OrganizationService, logger *zap.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		svc:    svc,
		logger: logger,
	}
}

// Create handles POST /v1/organizations
func (h *OrganizationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var create domain.OrganizationCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate
	errors := h.validateCreate(&create)
	if len(errors) > 0 {
		httputil.WriteValidationError(w, r, errors)
		return
	}

	org, err := h.svc.Create(r.Context(), &create)
	if err != nil {
		if _, ok := err.(*service.ConflictError); ok {
			httputil.WriteConflict(w, r, err.Error())
			return
		}
		h.logger.Error("failed to create organization", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, org)
}

// List handles GET /v1/organizations
func (h *OrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	params := domain.OrganizationListParams{
		Limit:  parseIntParam(r.URL.Query().Get("limit"), 100),
		Offset: parseIntParam(r.URL.Query().Get("offset"), 0),
	}

	response, err := h.svc.List(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to list organizations", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/organizations/{org_id}
func (h *OrganizationHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "org_id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "org_id", Message: "Invalid organization ID format"},
		})
		return
	}

	org, err := h.svc.Get(r.Context(), orgID)
	if err != nil {
		h.logger.Error("failed to get organization", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if org == nil {
		httputil.WriteNotFound(w, r, "Organization", orgIDStr)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, org)
}

// Update handles PATCH /v1/organizations/{org_id}
func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "org_id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		httputil.WriteValidationError(w, r, []httputil.ValidationError{
			{Field: "org_id", Message: "Invalid organization ID format"},
		})
		return
	}

	var update domain.OrganizationUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		httputil.WriteError(w, r, http.StatusBadRequest, httputil.ErrorTypeValidation,
			"Invalid Request Body", "Failed to parse JSON: "+err.Error())
		return
	}

	// Validate
	errors := h.validateUpdate(&update)
	if len(errors) > 0 {
		httputil.WriteValidationError(w, r, errors)
		return
	}

	org, err := h.svc.Update(r.Context(), orgID, &update)
	if err != nil {
		h.logger.Error("failed to update organization", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	if org == nil {
		httputil.WriteNotFound(w, r, "Organization", orgIDStr)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, org)
}

func (h *OrganizationHandler) validateCreate(create *domain.OrganizationCreate) []httputil.ValidationError {
	var errors []httputil.ValidationError

	if create.Slug == "" {
		errors = append(errors, httputil.ValidationError{Field: "slug", Message: "slug is required"})
	} else if len(create.Slug) > 100 {
		errors = append(errors, httputil.ValidationError{Field: "slug", Message: "slug must be 100 characters or less"})
	} else if !slugRegex.MatchString(create.Slug) {
		errors = append(errors, httputil.ValidationError{Field: "slug", Message: "slug must start with a letter and contain only lowercase letters, numbers, and hyphens"})
	}

	if create.DisplayName == "" {
		errors = append(errors, httputil.ValidationError{Field: "display_name", Message: "display_name is required"})
	} else if len(create.DisplayName) > 255 {
		errors = append(errors, httputil.ValidationError{Field: "display_name", Message: "display_name must be 255 characters or less"})
	}

	if create.PlanTier != "" && !domain.IsValidPlanTier(create.PlanTier) {
		errors = append(errors, httputil.ValidationError{Field: "plan_tier", Message: "Invalid plan_tier. Must be one of: free, starter, enterprise"})
	}

	return errors
}

func (h *OrganizationHandler) validateUpdate(update *domain.OrganizationUpdate) []httputil.ValidationError {
	var errors []httputil.ValidationError

	if update.DisplayName != nil && len(*update.DisplayName) > 255 {
		errors = append(errors, httputil.ValidationError{Field: "display_name", Message: "display_name must be 255 characters or less"})
	}

	if update.PlanTier != nil && !domain.IsValidPlanTier(*update.PlanTier) {
		errors = append(errors, httputil.ValidationError{Field: "plan_tier", Message: "Invalid plan_tier. Must be one of: free, starter, enterprise"})
	}

	if update.Status != nil && !domain.IsValidOrgStatus(*update.Status) {
		errors = append(errors, httputil.ValidationError{Field: "status", Message: "Invalid status. Must be one of: active, suspended, deleted"})
	}

	return errors
}
