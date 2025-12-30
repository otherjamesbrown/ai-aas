// Package serviceaccounts provides service account management endpoints.
//
// Purpose:
//
//	This package implements service account CRUD operations for organizations.
//	Service accounts are used to issue API keys for programmatic access.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router
//   - internal/bootstrap: Runtime dependencies
//   - internal/storage/postgres: Service account data access
//
// Key Responsibilities:
//   - CreateServiceAccount: POST /v1/orgs/{orgId}/service-accounts - Create new service account
//   - GetServiceAccount: GET /v1/orgs/{orgId}/service-accounts/{serviceAccountId} - Get service account
//   - ListServiceAccounts: GET /v1/orgs/{orgId}/service-accounts - List service accounts
//   - UpdateServiceAccount: PATCH /v1/orgs/{orgId}/service-accounts/{serviceAccountId} - Update service account
//   - DeleteServiceAccount: DELETE /v1/orgs/{orgId}/service-accounts/{serviceAccountId} - Delete service account
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-004 (API Key Lifecycle)
package serviceaccounts

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts service account routes beneath /v1/orgs/{orgId}.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	// Service account management requires apikey:manage or org:admin scope
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/v1/orgs/{orgId}/service-accounts", handler.CreateServiceAccount)
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/v1/orgs/{orgId}/service-accounts/{serviceAccountId}", handler.GetServiceAccount)
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/v1/orgs/{orgId}/service-accounts", handler.ListServiceAccounts)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Patch("/v1/orgs/{orgId}/service-accounts/{serviceAccountId}", handler.UpdateServiceAccount)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Delete("/v1/orgs/{orgId}/service-accounts/{serviceAccountId}", handler.DeleteServiceAccount)
}

// Handler serves service account management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// CreateServiceAccountRequest represents the payload for creating a service account.
type CreateServiceAccountRequest struct {
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// CreateServiceAccount handles POST /v1/orgs/{orgId}/service-accounts.
func (h *Handler) CreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	// Parse org ID (UUID or slug)
	var orgID uuid.UUID
	var err error
	if orgID, err = uuid.Parse(orgIDParam); err != nil {
		// Try as slug
		org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
		if err != nil {
			if err == postgres.ErrNotFound {
				httputil.WriteNotFound(w, r, "organization", "")
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			httputil.WriteInternalError(w, r)
			return
		}
		orgID = org.ID
	}

	var req CreateServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	if req.Name == "" {
		httputil.WriteBadRequest(w, r, "name is required")
		return
	}

	// Create service account
	params := postgres.CreateServiceAccountParams{
		OrgID:       orgID,
		Name:        req.Name,
		Description: req.Description,
		Status:      "active",
		Metadata:    req.Metadata,
	}

	serviceAccount, err := h.runtime.Postgres.CreateServiceAccount(ctx, params)
	if err != nil {
		h.logger.Error("failed to create service account", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("name", req.Name))
		httputil.WriteInternalError(w, r)
		return
	}

	response := map[string]any{
		"serviceAccountId": serviceAccount.ID.String(),
		"orgId":            serviceAccount.OrgID.String(),
		"name":             serviceAccount.Name,
		"status":           serviceAccount.Status,
		"createdAt":        serviceAccount.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if serviceAccount.Description != nil {
		response["description"] = *serviceAccount.Description
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetServiceAccount handles GET /v1/orgs/{orgId}/service-accounts/{serviceAccountId}.
func (h *Handler) GetServiceAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	serviceAccountIDParam := chi.URLParam(r, "serviceAccountId")

	// Parse org ID
	var orgID uuid.UUID
	var err error
	if orgID, err = uuid.Parse(orgIDParam); err != nil {
		org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
		if err != nil {
			httputil.WriteNotFound(w, r, "organization", "")
			return
		}
		orgID = org.ID
	}

	// Parse service account ID
	serviceAccountID, err := uuid.Parse(serviceAccountIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid service account ID")
		return
	}

	// Get service account
	serviceAccount, err := h.runtime.Postgres.GetServiceAccountByID(ctx, serviceAccountID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "service account", "")
			return
		}
		h.logger.Error("failed to get service account", zap.Error(err), zap.String("serviceAccountId", serviceAccountID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify service account belongs to org
	if serviceAccount.OrgID != orgID {
		httputil.WriteNotFound(w, r, "service account", "")
		return
	}

	response := map[string]any{
		"serviceAccountId": serviceAccount.ID.String(),
		"orgId":            serviceAccount.OrgID.String(),
		"name":             serviceAccount.Name,
		"status":           serviceAccount.Status,
		"createdAt":        serviceAccount.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updatedAt":        serviceAccount.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if serviceAccount.Description != nil {
		response["description"] = *serviceAccount.Description
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ListServiceAccounts handles GET /v1/orgs/{orgId}/service-accounts.
func (h *Handler) ListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement list with pagination
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]map[string]any{})
}

// UpdateServiceAccount handles PATCH /v1/orgs/{orgId}/service-accounts/{serviceAccountId}.
func (h *Handler) UpdateServiceAccount(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement update
	httputil.WriteNotImplemented(w, r, "not implemented")
}

// DeleteServiceAccount handles DELETE /v1/orgs/{orgId}/service-accounts/{serviceAccountId}.
func (h *Handler) DeleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement soft delete
	httputil.WriteNotImplemented(w, r, "not implemented")
}
