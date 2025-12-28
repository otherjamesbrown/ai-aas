// Package orgs provides HTTP handlers for organization lifecycle management.
//
// Purpose:
//
//	This package implements REST API handlers for organization CRUD operations,
//	including creation, retrieval, updates, and status management. Handlers
//	enforce authorization, validate input, and emit audit events for all
//	state changes.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store, config)
//   - internal/storage/postgres: Data access layer
//
// Key Responsibilities:
//   - CreateOrg: POST /v1/orgs - Create new organization
//   - GetOrg: GET /v1/orgs/{orgId} - Retrieve organization by ID or slug
//   - UpdateOrg: PATCH /v1/orgs/{orgId} - Update organization metadata
//   - ListOrgs: GET /v1/orgs - List organizations (future: pagination)
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User & Organization Management)
//   - specs/005-user-org-service/spec.md#FR-001 (Organization Lifecycle)
//   - specs/005-user-org-service/contracts/user-org-service.openapi.yaml
//
// Debugging Notes:
//   - Organization lookups support both UUID and slug (slug preferred for human-readable APIs)
//   - Optimistic locking prevents concurrent update conflicts (returns 409 Conflict)
//   - Soft deletes are enforced (deleted_at IS NULL)
//   - Status transitions: pending -> active -> suspended -> active or pending_delete
//
// Thread Safety:
//   - Handler methods are safe for concurrent use (stateless, uses runtime dependencies)
//
// Error Handling:
//   - Invalid UUID/slug returns 400 Bad Request
//   - Not found returns 404 Not Found
//   - Optimistic lock conflicts return 409 Conflict
//   - Database errors return 500 Internal Server Error
package orgs

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts organization routes beneath /v1/orgs.
// Users routes should be registered separately after this call.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	router.Route("/v1/orgs", func(r chi.Router) {
		// Org creation typically requires platform admin scope
		r.With(middleware.RequireAdminScope("org:admin")).Post("/", handler.CreateOrg)
		r.With(middleware.RequireAdminScope("org:read", "org:admin")).Get("/", handler.ListOrgs)
		// Register {orgId} routes - these must be registered before users routes
		// to ensure GET /v1/orgs/{orgId} matches correctly
		r.With(middleware.RequireAdminScope("org:read", "org:admin")).Get("/{orgId}", handler.GetOrg)
		r.With(middleware.RequireAdminScope("org:write", "org:admin")).Patch("/{orgId}", handler.UpdateOrg)
		r.With(middleware.RequireAdminScope("org:admin")).Delete("/{orgId}", handler.DeleteOrg)
	})

	// Frontend-friendly convenience routes (no /v1 prefix)
	// These resolve org from authenticated context
	router.With(middleware.RequireAdminScope("org:read", "org:admin")).Get("/organizations/me", handler.GetOrgForMe)
	router.With(middleware.RequireAdminScope("org:write", "org:admin")).Patch("/organizations/me", handler.UpdateOrgForMe)
}

// Handler serves organization management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// CreateOrgRequest represents the payload for creating an organization.
type CreateOrgRequest struct {
	Name              string             `json:"name"`
	Slug              string             `json:"slug"`
	BillingOwnerEmail string             `json:"billingOwnerEmail,omitempty"`
	Declarative       *DeclarativeConfig `json:"declarative,omitempty"`
	Metadata          map[string]any     `json:"metadata,omitempty"`
}

// DeclarativeConfig represents declarative GitOps configuration.
type DeclarativeConfig struct {
	Enabled bool   `json:"enabled"`
	RepoURL string `json:"repoUrl,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

// UpdateOrgRequest represents the payload for updating an organization.
type UpdateOrgRequest struct {
	DisplayName    *string            `json:"displayName,omitempty"`
	Status         *string            `json:"status,omitempty"`
	BudgetPolicyID *string            `json:"budgetPolicyId,omitempty"`
	Declarative    *DeclarativeConfig `json:"declarative,omitempty"`
	Metadata       map[string]any     `json:"metadata,omitempty"`
}

// OrganizationResponse represents an organization in API responses.
type OrganizationResponse struct {
	OrgID     string         `json:"orgId"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

// CreateOrg handles POST /v1/orgs - Create a new organization.
// Validates slug uniqueness and creates the organization with default settings.
func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Validate required fields
	if req.Name == "" || req.Slug == "" {
		httputil.WriteBadRequest(w, r, "name and slug are required")
		return
	}

	// Normalize slug (lowercase, no spaces)
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		httputil.WriteBadRequest(w, r, "slug cannot be empty")
		return
	}

	// Check if org with slug already exists
	_, err := h.runtime.Postgres.GetOrgBySlug(ctx, slug)
	if err == nil {
		httputil.WriteConflict(w, r, "organization with this slug already exists")
		return
	}
	// ErrNotFound is expected, continue

	// TODO: Lookup billing owner user by email if provided
	var billingOwnerID *uuid.UUID

	declarativeMode := "disabled"
	var declarativeRepoURL, declarativeBranch *string
	if req.Declarative != nil && req.Declarative.Enabled {
		declarativeMode = "enabled"
		if req.Declarative.RepoURL != "" {
			declarativeRepoURL = &req.Declarative.RepoURL
		}
		if req.Declarative.Branch != "" {
			declarativeBranch = &req.Declarative.Branch
		}
	}

	params := postgres.CreateOrgParams{
		ID:                 uuid.New(),
		Slug:               slug,
		Name:               req.Name,
		Status:             "active",
		BillingOwnerUserID: billingOwnerID,
		DeclarativeMode:    declarativeMode,
		DeclarativeRepoURL: declarativeRepoURL,
		DeclarativeBranch:  declarativeBranch,
		Metadata:           req.Metadata,
	}

	org, err := h.runtime.Postgres.CreateOrg(ctx, params)
	if err != nil {
		h.logger.Error("failed to create organization", zap.Error(err), zap.String("slug", slug))
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	actorID := getActorID(r) // TODO: Extract from authenticated session
	event := audit.BuildEvent(org.ID, actorID, audit.ActorTypeSystem, audit.ActionOrgCreate, audit.TargetTypeOrg, &org.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"slug": org.Slug,
		"name": org.Name,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	resp := toOrgResponse(org)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// GetOrg handles GET /v1/orgs/{orgId} - Retrieve organization by ID or slug.
func (h *Handler) GetOrg(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	var org postgres.Org
	var err error

	// Try parsing as UUID first, then fall back to slug lookup
	if orgID, err := uuid.Parse(orgIDParam); err == nil {
		org, err = h.runtime.Postgres.GetOrg(ctx, orgID)
	} else {
		// Treat as slug
		org, err = h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
	}

	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "organization", "")
			return
		}
		h.logger.Error("failed to get organization", zap.Error(err), zap.String("orgId", orgIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	resp := toOrgResponse(org)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// UpdateOrg handles PATCH /v1/orgs/{orgId} - Update organization metadata.
// Uses optimistic locking to prevent concurrent update conflicts.
func (h *Handler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	// Get existing org to obtain current version
	var existingOrg postgres.Org
	var err error
	if orgID, err := uuid.Parse(orgIDParam); err == nil {
		existingOrg, err = h.runtime.Postgres.GetOrg(ctx, orgID)
	} else {
		existingOrg, err = h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
	}

	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "organization", "")
			return
		}
		h.logger.Error("failed to get organization for update", zap.Error(err), zap.String("orgId", orgIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	var req UpdateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Build update params (only include fields that are provided)
	params := postgres.UpdateOrgParams{
		ID:      existingOrg.ID,
		Version: existingOrg.Version,
		Name:    existingOrg.Name, // Default to existing
		Status:  existingOrg.Status,
	}

	if req.DisplayName != nil {
		params.Name = *req.DisplayName
	}
	if req.Status != nil {
		// Validate status
		validStatuses := map[string]bool{"active": true, "suspended": true}
		if !validStatuses[*req.Status] {
			httputil.WriteBadRequest(w, r, "invalid status")
			return
		}
		params.Status = *req.Status
	}
	if req.BudgetPolicyID != nil {
		if policyID, err := uuid.Parse(*req.BudgetPolicyID); err == nil {
			params.BudgetPolicyID = &policyID
		}
	}

	// Handle declarative config updates
	if req.Declarative != nil {
		if req.Declarative.Enabled {
			params.DeclarativeMode = "enabled"
			if req.Declarative.RepoURL != "" {
				params.DeclarativeRepoURL = &req.Declarative.RepoURL
			}
			if req.Declarative.Branch != "" {
				params.DeclarativeBranch = &req.Declarative.Branch
			}
		} else {
			params.DeclarativeMode = "disabled"
		}
	}

	// Merge metadata if provided
	if req.Metadata != nil {
		params.Metadata = req.Metadata
	} else {
		params.Metadata = existingOrg.Metadata
	}

	org, err := h.runtime.Postgres.UpdateOrg(ctx, params)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			httputil.WriteConflict(w, r, "organization was modified concurrently")
			return
		}
		h.logger.Error("failed to update organization", zap.Error(err), zap.String("orgId", existingOrg.ID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	actorID := getActorID(r) // TODO: Extract from authenticated session
	action := audit.ActionOrgUpdate
	if req.Status != nil && *req.Status == "suspended" {
		action = audit.ActionOrgSuspend
	}
	event := audit.BuildEvent(org.ID, actorID, audit.ActorTypeSystem, action, audit.TargetTypeOrg, &org.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"previous_status": existingOrg.Status,
		"new_status":      org.Status,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	resp := toOrgResponse(org)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// ListOrgs handles GET /v1/orgs - List organizations.
// Supports pagination via ?limit=N&offset=M query parameters.
// Note: This is a master admin endpoint - normal users should only see their own org.
func (h *Handler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	orgs, err := h.runtime.Postgres.ListOrgs(ctx, limit, offset)
	if err != nil {
		h.logger.Error("failed to list organizations", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Convert to response format
	resp := make([]OrganizationResponse, 0, len(orgs))
	for _, org := range orgs {
		resp = append(resp, toOrgResponse(org))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// DeleteOrg handles DELETE /v1/orgs/{orgId} - Delete (soft-delete) an organization.
// This is a destructive operation that requires master admin privileges.
func (h *Handler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
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

	// Perform soft delete
	if err := h.runtime.Postgres.DeleteOrg(ctx, orgID); err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "organization", "")
			return
		}
		h.logger.Error("failed to delete organization", zap.Error(err), zap.String("orgId", orgID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	actorID := getActorID(r)
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionOrgDelete, audit.TargetTypeOrg, &orgID)
	event = audit.BuildEventFromRequest(event, r)
	_ = h.runtime.Audit.Emit(ctx, event)

	h.logger.Info("organization deleted",
		zap.String("org_id", orgID.String()),
		zap.String("actor_id", actorID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// toOrgResponse converts a postgres.Org to an OrganizationResponse.
func toOrgResponse(org postgres.Org) OrganizationResponse {
	return OrganizationResponse{
		OrgID:     org.ID.String(),
		Name:      org.Name,
		Slug:      org.Slug,
		Status:    org.Status,
		Metadata:  org.Metadata,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// getActorID extracts the actor ID from the request context.
// Requires RequireAuth middleware to be applied to the route.
func getActorID(r *http.Request) uuid.UUID {
	return middleware.GetUserID(r.Context())
}

// GetOrgForMe handles GET /organizations/me - Get the current user's organization.
// Resolves the organization from the authenticated context.
func (h *Handler) GetOrgForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get org ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	if orgID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "organization not found in context")
		return
	}

	org, err := h.runtime.Postgres.GetOrg(ctx, orgID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "organization", "")
			return
		}
		h.logger.Error("failed to get organization", zap.Error(err), zap.String("orgId", orgID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	resp := toOrgResponse(org)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// UpdateOrgForMe handles PATCH /organizations/me - Update the current user's organization.
// Resolves the organization from the authenticated context.
func (h *Handler) UpdateOrgForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get org ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	if orgID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "organization not found in context")
		return
	}

	// Get existing org to obtain current version
	existingOrg, err := h.runtime.Postgres.GetOrg(ctx, orgID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "organization", "")
			return
		}
		h.logger.Error("failed to get organization for update", zap.Error(err), zap.String("orgId", orgID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	var req UpdateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Build update params (only include fields that are provided)
	params := postgres.UpdateOrgParams{
		ID:      existingOrg.ID,
		Version: existingOrg.Version,
		Name:    existingOrg.Name, // Default to existing
		Status:  existingOrg.Status,
	}

	if req.DisplayName != nil {
		params.Name = *req.DisplayName
	}
	if req.Status != nil {
		// Validate status
		validStatuses := map[string]bool{"active": true, "suspended": true}
		if !validStatuses[*req.Status] {
			httputil.WriteBadRequest(w, r, "invalid status")
			return
		}
		params.Status = *req.Status
	}
	if req.BudgetPolicyID != nil {
		if policyID, err := uuid.Parse(*req.BudgetPolicyID); err == nil {
			params.BudgetPolicyID = &policyID
		}
	}

	// Handle declarative config updates
	if req.Declarative != nil {
		if req.Declarative.Enabled {
			params.DeclarativeMode = "enabled"
			if req.Declarative.RepoURL != "" {
				params.DeclarativeRepoURL = &req.Declarative.RepoURL
			}
			if req.Declarative.Branch != "" {
				params.DeclarativeBranch = &req.Declarative.Branch
			}
		} else {
			params.DeclarativeMode = "disabled"
		}
	}

	// Merge metadata if provided
	if req.Metadata != nil {
		params.Metadata = req.Metadata
	} else {
		params.Metadata = existingOrg.Metadata
	}

	org, err := h.runtime.Postgres.UpdateOrg(ctx, params)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			httputil.WriteConflict(w, r, "organization was modified concurrently")
			return
		}
		h.logger.Error("failed to update organization", zap.Error(err), zap.String("orgId", existingOrg.ID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	actorID := getActorID(r)
	action := audit.ActionOrgUpdate
	if req.Status != nil && *req.Status == "suspended" {
		action = audit.ActionOrgSuspend
	}
	event := audit.BuildEvent(org.ID, actorID, audit.ActorTypeSystem, action, audit.TargetTypeOrg, &org.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"previous_status": existingOrg.Status,
		"new_status":      org.Status,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	resp := toOrgResponse(org)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}
