// Package modelaccess provides HTTP handlers for user-level model access control.
//
// Purpose:
//
//	This package implements REST API handlers for managing per-user model access
//	within organizations. Org admins can set access modes (restricted/auto_grant)
//	and grant/revoke specific model access to users.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store)
//   - internal/storage/postgres: Data access layer
//
// Key Responsibilities:
//   - GetUserModelAccess: GET /v1/orgs/{orgId}/users/{userId}/model-access
//   - SetUserAccessMode: PUT /v1/orgs/{orgId}/users/{userId}/model-access/mode
//   - ListUserModelGrants: GET /v1/orgs/{orgId}/users/{userId}/model-access/grants
//   - GrantModelAccess: POST /v1/orgs/{orgId}/users/{userId}/model-access/grants
//   - GrantAllCurrentModels: POST /v1/orgs/{orgId}/users/{userId}/model-access/grants/all-current
//   - RevokeModelAccess: DELETE /v1/orgs/{orgId}/users/{userId}/model-access/grants/{modelName}
//
// Requirements Reference:
//   - specs/022-user-model-access-control/spec.md
//   - specs/022-user-model-access-control/contracts/openapi.yaml
package modelaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts model access routes beneath /v1/orgs/{orgId}/users/{userId}.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	// Legacy detailed routes
	router.Get("/v1/orgs/{orgId}/users/{userId}/model-access", handler.GetUserModelAccess)
	router.Put("/v1/orgs/{orgId}/users/{userId}/model-access/mode", handler.SetUserAccessMode)
	router.Get("/v1/orgs/{orgId}/users/{userId}/model-access/grants", handler.ListUserModelGrants)
	router.Post("/v1/orgs/{orgId}/users/{userId}/model-access/grants", handler.GrantModelAccess)
	router.Post("/v1/orgs/{orgId}/users/{userId}/model-access/grants/all-current", handler.GrantAllCurrentModels)
	router.Delete("/v1/orgs/{orgId}/users/{userId}/model-access/grants/{modelName}", handler.RevokeModelAccess)

	// Simpler routes for CLI (UC-USR-005, UC-USR-006, UC-USR-007)
	router.Get("/v1/orgs/{orgId}/users/{userId}/models", handler.ListUserModelsSimple)
	router.Post("/v1/orgs/{orgId}/users/{userId}/models", handler.GrantModelAccessSimple)
	router.Post("/v1/orgs/{orgId}/users/{userId}/models/all", handler.GrantAllModelsSimple)
	router.Delete("/v1/orgs/{orgId}/users/{userId}/models/{modelId}", handler.RevokeModelAccessSimple)
}

// Handler serves model access management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// UserModelAccessResponse represents the user's model access configuration.
type UserModelAccessResponse struct {
	UserID          string          `json:"userId"`
	OrgID           string          `json:"orgId"`
	AccessMode      string          `json:"accessMode"`
	GrantedModels   []ModelGrantDTO `json:"grantedModels"`
	AvailableModels []string        `json:"availableModels,omitempty"`
}

// ModelGrantDTO represents a model grant in API responses.
type ModelGrantDTO struct {
	GrantID   string  `json:"grantId"`
	ModelName string  `json:"modelName"`
	GrantedAt string  `json:"grantedAt"`
	GrantedBy *string `json:"grantedBy,omitempty"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

// SetAccessModeRequest represents the request body for setting access mode.
type SetAccessModeRequest struct {
	AccessMode string `json:"accessMode"` // "restricted" or "auto_grant"
}

// GrantModelRequest represents the request body for granting model access.
type GrantModelRequest struct {
	ModelName string  `json:"modelName"`
	ExpiresAt *string `json:"expiresAt,omitempty"` // ISO 8601 timestamp
}

// ModelGrantsListResponse represents the response for listing grants.
type ModelGrantsListResponse struct {
	Grants []ModelGrantDTO `json:"grants"`
}

// BulkGrantResponse represents the response for granting all current models.
type BulkGrantResponse struct {
	GrantedCount int      `json:"grantedCount"`
	Models       []string `json:"models"`
	GrantedAt    string   `json:"grantedAt"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// GetUserModelAccess handles GET /v1/orgs/{orgId}/users/{userId}/model-access
func (h *Handler) GetUserModelAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Verify caller has access to this org (must be org member or admin)
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "you do not have access to this organization")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	info, err := h.runtime.Postgres.GetUserModelAccess(ctx, orgID, targetUserID)
	if err != nil {
		h.logger.Error("failed to get user model access", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve model access")
		return
	}

	resp := UserModelAccessResponse{
		UserID:        targetUserID.String(),
		OrgID:         orgID.String(),
		AccessMode:    info.AccessMode,
		GrantedModels: make([]ModelGrantDTO, 0, len(info.GrantedModels)),
	}

	for _, g := range info.GrantedModels {
		resp.GrantedModels = append(resp.GrantedModels, toModelGrantDTO(g))
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// SetUserAccessMode handles PUT /v1/orgs/{orgId}/users/{userId}/model-access/mode
func (h *Handler) SetUserAccessMode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for modifying user access mode
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to modify user access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	var req SetAccessModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid request payload")
		return
	}

	// Validate access mode
	if req.AccessMode != "restricted" && req.AccessMode != "auto_grant" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Access Mode", "access_mode must be 'restricted' or 'auto_grant'")
		return
	}

	actorID := middleware.GetUserID(ctx)

	_, err = h.runtime.Postgres.SetUserAccessMode(ctx, orgID, targetUserID, req.AccessMode)
	if err != nil {
		h.logger.Error("failed to set user access mode", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to update access mode")
		return
	}

	h.logger.Info("user access mode changed",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.String("access_mode", req.AccessMode),
		zap.String("actor_id", actorID.String()),
	)

	// Return updated model access info
	info, err := h.runtime.Postgres.GetUserModelAccess(ctx, orgID, targetUserID)
	if err != nil {
		h.logger.Error("failed to get user model access after update", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve updated model access")
		return
	}

	resp := UserModelAccessResponse{
		UserID:        targetUserID.String(),
		OrgID:         orgID.String(),
		AccessMode:    info.AccessMode,
		GrantedModels: make([]ModelGrantDTO, 0, len(info.GrantedModels)),
	}

	for _, g := range info.GrantedModels {
		resp.GrantedModels = append(resp.GrantedModels, toModelGrantDTO(g))
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// ListUserModelGrants handles GET /v1/orgs/{orgId}/users/{userId}/model-access/grants
func (h *Handler) ListUserModelGrants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "you do not have access to this organization")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	grants, err := h.runtime.Postgres.ListModelGrants(ctx, orgID, targetUserID)
	if err != nil {
		h.logger.Error("failed to list model grants", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve grants")
		return
	}

	resp := ModelGrantsListResponse{
		Grants: make([]ModelGrantDTO, 0, len(grants)),
	}
	for _, g := range grants {
		resp.Grants = append(resp.Grants, toModelGrantDTO(g))
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// GrantModelAccess handles POST /v1/orgs/{orgId}/users/{userId}/model-access/grants
func (h *Handler) GrantModelAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for granting model access
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to grant model access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	var req GrantModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid request payload")
		return
	}

	if req.ModelName == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "model_name is required")
		return
	}

	// Parse expires_at if provided
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Date", "expires_at must be a valid ISO 8601 timestamp")
			return
		}
		if t.Before(time.Now()) {
			h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Date", "expires_at must be in the future")
			return
		}
		expiresAt = &t
	}

	// Get the actor (admin granting access)
	actorID := middleware.GetUserID(ctx)

	params := postgres.CreateModelGrantParams{
		OrgID:     orgID,
		UserID:    targetUserID,
		ModelName: req.ModelName,
		GrantedBy: &actorID,
		ExpiresAt: expiresAt,
	}

	grant, err := h.runtime.Postgres.CreateModelGrant(ctx, params)
	if err != nil {
		if err == postgres.ErrConflict {
			h.writeError(w, http.StatusConflict, "conflict", "Grant Already Exists", "user already has access to this model")
			return
		}
		h.logger.Error("failed to create model grant", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to create grant")
		return
	}

	h.logger.Info("model access granted",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.String("model_name", req.ModelName),
		zap.String("grant_id", grant.ID.String()),
		zap.String("actor_id", actorID.String()),
		zap.Bool("has_expiry", expiresAt != nil),
	)

	h.writeJSON(w, http.StatusCreated, toModelGrantDTO(grant))
}

// GrantAllCurrentModels handles POST /v1/orgs/{orgId}/users/{userId}/model-access/grants/all-current
func (h *Handler) GrantAllCurrentModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for bulk granting model access
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to grant model access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	actorID := middleware.GetUserID(ctx)

	// Read models from request body
	var requestBody struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid JSON payload")
		return
	}

	models := requestBody.Models
	if len(models) == 0 {
		// No models to grant
		h.writeError(w, http.StatusBadRequest, "bad_request", "No Models", "no models provided; include 'models' array in request body")
		return
	}

	count, err := h.runtime.Postgres.GrantAllCurrentModels(ctx, orgID, targetUserID, &actorID, models)
	if err != nil {
		h.logger.Error("failed to grant all current models", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to grant models")
		return
	}

	h.logger.Info("bulk model access granted",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.Int("granted_count", count),
		zap.Int("requested_count", len(models)),
		zap.Strings("models", models),
		zap.String("actor_id", actorID.String()),
	)

	resp := BulkGrantResponse{
		GrantedCount: count,
		Models:       models,
		GrantedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// RevokeModelAccess handles DELETE /v1/orgs/{orgId}/users/{userId}/model-access/grants/{modelName}
func (h *Handler) RevokeModelAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")
	modelName := chi.URLParam(r, "modelName")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for revoking model access
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to revoke model access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	actorID := middleware.GetUserID(ctx)

	err = h.runtime.Postgres.DeleteModelGrant(ctx, orgID, targetUserID, modelName)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Grant Not Found", "user does not have a grant for this model")
			return
		}
		h.logger.Error("failed to revoke model grant", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to revoke grant")
		return
	}

	h.logger.Info("model access revoked",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.String("model_name", modelName),
		zap.String("actor_id", actorID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

func (h *Handler) resolveOrgID(ctx context.Context, orgIDParam string) (uuid.UUID, error) {
	if orgID, err := uuid.Parse(orgIDParam); err == nil {
		_, err := h.runtime.Postgres.GetOrg(ctx, orgID)
		return orgID, err
	}
	org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func (h *Handler) resolveUserID(ctx context.Context, orgID uuid.UUID, userIDParam string) (uuid.UUID, error) {
	// Try parsing as UUID first
	if userID, err := uuid.Parse(userIDParam); err == nil {
		// Verify user exists in org
		_, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
		return userID, err
	}
	// Try as email
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, userIDParam)
	if err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
}

func (h *Handler) requireOrgAccess(ctx context.Context, orgID uuid.UUID) error {
	// Get authenticated user's org from context
	authOrgID := middleware.GetOrgID(ctx)
	userID := middleware.GetUserID(ctx)

	// Service accounts with admin scope (or wildcard "*" scope) can access any org
	// This allows platform admins to manage all orgs
	if middleware.HasAnyScope(ctx, "admin", "*") {
		return nil
	}

	// If user is in the same org, allow access
	if authOrgID == orgID {
		return nil
	}

	// Otherwise, verify user is a member of the target org
	return h.runtime.Postgres.ValidateUserOrgMembership(ctx, userID, orgID)
}

// requireOrgAdmin checks if the authenticated user has admin privileges for the org.
// Admin privileges are granted via scopes: "org:admin", "model-access:admin", or "admin".
// Returns nil if authorized, error otherwise.
func (h *Handler) requireOrgAdmin(ctx context.Context, orgID uuid.UUID) error {
	// First check org access
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		return err
	}

	// Then check for admin scope (including wildcard scope)
	if middleware.HasAnyScope(ctx, "org:admin", "model-access:admin", "admin", "*") {
		return nil
	}

	// Check if user is the billing owner of the org (implicit admin)
	org, err := h.runtime.Postgres.GetOrg(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get org: %w", err)
	}

	userID := middleware.GetUserID(ctx)
	if org.BillingOwnerUserID != nil && *org.BillingOwnerUserID == userID {
		return nil
	}

	return fmt.Errorf("admin privileges required")
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, errType, title, detail string) {
	resp := ErrorResponse{
		Type:   "https://api.ai-aas.io/errors/" + errType,
		Title:  title,
		Status: status,
		Detail: detail,
	}
	h.writeJSON(w, status, resp)
}

func toModelGrantDTO(g postgres.UserModelGrant) ModelGrantDTO {
	dto := ModelGrantDTO{
		GrantID:   g.ID.String(),
		ModelName: g.ModelName,
		GrantedAt: g.GrantedAt.Format(time.RFC3339),
	}
	if g.GrantedBy != nil {
		s := g.GrantedBy.String()
		dto.GrantedBy = &s
	}
	if g.ExpiresAt != nil {
		s := g.ExpiresAt.Format(time.RFC3339)
		dto.ExpiresAt = &s
	}
	return dto
}

// ---------------------------------------------------------------------------
// Simple API routes for CLI compatibility
// ---------------------------------------------------------------------------

// UserModelAccessSimple represents a user's access to a model (simple format for CLI).
type UserModelAccessSimple struct {
	UserID    string `json:"user_id"`
	ModelID   string `json:"model_id"`
	ModelName string `json:"model_name"`
	GrantedAt string `json:"granted_at"`
	GrantedBy string `json:"granted_by,omitempty"`
}

// GrantModelRequestSimple represents the request body for granting model access (simple format).
type GrantModelRequestSimple struct {
	ModelID string `json:"model_id"`
}

// ListUserModelsSimple handles GET /v1/orgs/{orgId}/users/{userId}/models
// Returns a simple array of models (for CLI compatibility).
func (h *Handler) ListUserModelsSimple(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "you do not have access to this organization")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	grants, err := h.runtime.Postgres.ListModelGrants(ctx, orgID, targetUserID)
	if err != nil {
		h.logger.Error("failed to list model grants", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve model access")
		return
	}

	// Convert to simple format
	models := make([]UserModelAccessSimple, 0, len(grants))
	for _, g := range grants {
		model := UserModelAccessSimple{
			UserID:    targetUserID.String(),
			ModelID:   g.ModelName, // Use model_name as model_id for now
			ModelName: g.ModelName,
			GrantedAt: g.GrantedAt.Format(time.RFC3339),
		}
		if g.GrantedBy != nil {
			model.GrantedBy = g.GrantedBy.String()
		}
		models = append(models, model)
	}

	// Return wrapped response for CLI
	resp := map[string]interface{}{
		"models": models,
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// GrantModelAccessSimple handles POST /v1/orgs/{orgId}/users/{userId}/models
// Grants access to a specific model (simple format for CLI).
func (h *Handler) GrantModelAccessSimple(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for granting model access
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to grant model access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	var req GrantModelRequestSimple
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid request payload")
		return
	}

	if req.ModelID == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "model_id is required")
		return
	}

	// Get the actor (admin granting access)
	actorID := middleware.GetUserID(ctx)

	params := postgres.CreateModelGrantParams{
		OrgID:     orgID,
		UserID:    targetUserID,
		ModelName: req.ModelID, // Use model_id as model_name
		GrantedBy: &actorID,
		ExpiresAt: nil,
	}

	grant, err := h.runtime.Postgres.CreateModelGrant(ctx, params)
	if err != nil {
		if err == postgres.ErrConflict {
			h.writeError(w, http.StatusConflict, "conflict", "Grant Already Exists", "user already has access to this model")
			return
		}
		h.logger.Error("failed to create model grant", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to create grant")
		return
	}

	h.logger.Info("model access granted (simple API)",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.String("model_name", req.ModelID),
		zap.String("grant_id", grant.ID.String()),
		zap.String("actor_id", actorID.String()),
	)

	// Return the grant in simple format
	model := UserModelAccessSimple{
		UserID:    targetUserID.String(),
		ModelID:   grant.ModelName,
		ModelName: grant.ModelName,
		GrantedAt: grant.GrantedAt.Format(time.RFC3339),
	}
	if grant.GrantedBy != nil {
		model.GrantedBy = grant.GrantedBy.String()
	}

	h.writeJSON(w, http.StatusCreated, model)
}

// GrantAllModelsSimple handles POST /v1/orgs/{orgId}/users/{userId}/models/all
// Grants access to all available models (simple format for CLI).
func (h *Handler) GrantAllModelsSimple(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for bulk granting model access
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to grant model access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	actorID := middleware.GetUserID(ctx)

	// For "all" mode, we need to fetch available models
	// This is a placeholder - in production, this should fetch from routing policies or model registry
	// For now, we'll just set the access mode to "auto_grant"
	_, err = h.runtime.Postgres.SetUserAccessMode(ctx, orgID, targetUserID, "auto_grant")
	if err != nil {
		h.logger.Error("failed to set user access mode to auto_grant", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to grant all models access")
		return
	}

	h.logger.Info("all models access granted (simple API)",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.String("actor_id", actorID.String()),
	)

	// Return success response
	resp := map[string]interface{}{
		"message":     "User granted access to all models",
		"access_mode": "auto_grant",
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// RevokeModelAccessSimple handles DELETE /v1/orgs/{orgId}/users/{userId}/models/{modelId}
// Revokes access to a specific model (simple format for CLI).
func (h *Handler) RevokeModelAccessSimple(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")
	modelID := chi.URLParam(r, "modelId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Require admin privileges for revoking model access
	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required to revoke model access")
		return
	}

	targetUserID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found in organization")
		return
	}

	actorID := middleware.GetUserID(ctx)

	err = h.runtime.Postgres.DeleteModelGrant(ctx, orgID, targetUserID, modelID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Grant Not Found", "user does not have a grant for this model")
			return
		}
		h.logger.Error("failed to revoke model grant", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to revoke grant")
		return
	}

	h.logger.Info("model access revoked (simple API)",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", targetUserID.String()),
		zap.String("model_id", modelID),
		zap.String("actor_id", actorID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
