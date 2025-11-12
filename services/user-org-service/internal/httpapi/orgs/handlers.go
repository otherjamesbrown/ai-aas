// Package orgs provides HTTP handlers for organization lifecycle management.
//
// Purpose:
//   This package implements REST API handlers for organization CRUD operations,
//   including creation, retrieval, updates, and status management. Handlers
//   enforce authorization, validate input, and emit audit events for all
//   state changes.
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
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts organization routes beneath /v1/orgs.
// Users routes should be registered separately after this call.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger zerolog.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	router.Route("/v1/orgs", func(r chi.Router) {
		r.Post("/", handler.CreateOrg)
		r.Get("/", handler.ListOrgs)
		// Register {orgId} routes - these must be registered before users routes
		// to ensure GET /v1/orgs/{orgId} matches correctly
		r.Get("/{orgId}", handler.GetOrg)
		r.Patch("/{orgId}", handler.UpdateOrg)
	})
}

// Handler serves organization management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  zerolog.Logger
}

// CreateOrgRequest represents the payload for creating an organization.
type CreateOrgRequest struct {
	Name              string            `json:"name"`
	Slug              string            `json:"slug"`
	BillingOwnerEmail string            `json:"billingOwnerEmail,omitempty"`
	Declarative       *DeclarativeConfig `json:"declarative,omitempty"`
	Metadata          map[string]any    `json:"metadata,omitempty"`
}

// DeclarativeConfig represents declarative GitOps configuration.
type DeclarativeConfig struct {
	Enabled  bool   `json:"enabled"`
	RepoURL  string `json:"repoUrl,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

// UpdateOrgRequest represents the payload for updating an organization.
type UpdateOrgRequest struct {
	DisplayName   *string            `json:"displayName,omitempty"`
	Status        *string            `json:"status,omitempty"`
	BudgetPolicyID *string           `json:"budgetPolicyId,omitempty"`
	Declarative   *DeclarativeConfig `json:"declarative,omitempty"`
	Metadata      map[string]any     `json:"metadata,omitempty"`
}

// OrganizationResponse represents an organization in API responses.
type OrganizationResponse struct {
	OrgID     string            `json:"orgId"`
	Name      string            `json:"name"`
	Slug      string            `json:"slug"`
	Status    string            `json:"status"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	CreatedAt string            `json:"createdAt"`
	UpdatedAt string            `json:"updatedAt"`
}

// CreateOrg handles POST /v1/orgs - Create a new organization.
// Validates slug uniqueness and creates the organization with default settings.
func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn().Err(err).Msg("invalid request payload")
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Slug == "" {
		http.Error(w, "name and slug are required", http.StatusBadRequest)
		return
	}

	// Normalize slug (lowercase, no spaces)
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		http.Error(w, "slug cannot be empty", http.StatusBadRequest)
		return
	}

	// Check if org with slug already exists
	_, err := h.runtime.Postgres.GetOrgBySlug(ctx, slug)
	if err == nil {
		http.Error(w, "organization with this slug already exists", http.StatusConflict)
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
		ID:                uuid.New(),
		Slug:              slug,
		Name:              req.Name,
		Status:            "active",
		BillingOwnerUserID: billingOwnerID,
		DeclarativeMode:   declarativeMode,
		DeclarativeRepoURL: declarativeRepoURL,
		DeclarativeBranch: declarativeBranch,
		Metadata:          req.Metadata,
	}

	org, err := h.runtime.Postgres.CreateOrg(ctx, params)
	if err != nil {
		h.logger.Error().Err(err).Str("slug", slug).Msg("failed to create organization")
		http.Error(w, "failed to create organization", http.StatusInternalServerError)
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
		h.logger.Error().Err(err).Msg("failed to encode response")
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
			http.Error(w, "organization not found", http.StatusNotFound)
			return
		}
		h.logger.Error().Err(err).Str("orgId", orgIDParam).Msg("failed to get organization")
		http.Error(w, "failed to retrieve organization", http.StatusInternalServerError)
		return
	}

	resp := toOrgResponse(org)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode response")
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
			http.Error(w, "organization not found", http.StatusNotFound)
			return
		}
		h.logger.Error().Err(err).Str("orgId", orgIDParam).Msg("failed to get organization for update")
		http.Error(w, "failed to retrieve organization", http.StatusInternalServerError)
		return
	}

	var req UpdateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn().Err(err).Msg("invalid request payload")
		http.Error(w, "invalid request payload", http.StatusBadRequest)
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
			http.Error(w, "invalid status", http.StatusBadRequest)
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
			http.Error(w, "organization was modified concurrently", http.StatusConflict)
			return
		}
		h.logger.Error().Err(err).Str("orgId", existingOrg.ID.String()).Msg("failed to update organization")
		http.Error(w, "failed to update organization", http.StatusInternalServerError)
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
		h.logger.Error().Err(err).Msg("failed to encode response")
	}
}

// ListOrgs handles GET /v1/orgs - List organizations.
// TODO: Add pagination, filtering, and authorization checks.
func (h *Handler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	// Placeholder - full implementation requires pagination and auth
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]OrganizationResponse{})
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

