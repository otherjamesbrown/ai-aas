// Package users provides HTTP handlers for user lifecycle management.
//
// Purpose:
//
//	This package implements REST API handlers for user management operations,
//	including invites, user CRUD, role assignments, and status updates (suspend,
//	activate). Handlers enforce authorization, validate input, and emit audit
//	events for all state changes.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store, config)
//   - internal/storage/postgres: Data access layer
//   - internal/security: Password hashing for temporary invite passwords
//
// Key Responsibilities:
//   - InviteUser: POST /v1/orgs/{orgId}/invites - Create user invite
//   - CreateUser: POST /v1/orgs/{orgId}/users - Create user directly
//   - ListUsers: GET /v1/orgs/{orgId}/users - List users in organization
//   - GetUser: GET /v1/orgs/{orgId}/users/{userId} - Retrieve user details
//   - UpdateUser: PATCH /v1/orgs/{orgId}/users/{userId} - Update user profile/status
//   - DeleteUser: DELETE /v1/orgs/{orgId}/users/{userId} - Soft delete user
//   - UpdateUserRoles: PUT /v1/orgs/{orgId}/users/{userId}/roles - Update role assignments
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User & Organization Management)
//   - specs/005-user-org-service/spec.md#FR-003 (User Invites)
//   - specs/005-user-org-service/contracts/user-org-service.openapi.yaml
//
// Debugging Notes:
//   - Invites create users with status="invited" and temporary password
//   - Invite expiry is 72 hours by default (configurable)
//   - User status transitions: invited -> active -> suspended -> active or deleted
//   - Role assignments require roles table (TODO: implement role storage)
//   - Optimistic locking prevents concurrent update conflicts
//
// Thread Safety:
//   - Handler methods are safe for concurrent use (stateless, uses runtime dependencies)
//
// Error Handling:
//   - Invalid UUID returns 400 Bad Request
//   - Not found returns 404 Not Found
//   - Duplicate email returns 409 Conflict
//   - Optimistic lock conflicts return 409 Conflict
//   - Database errors return 500 Internal Server Error
package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts user management routes beneath /v1/orgs/{orgId}.
// This should be called from within the orgs route group to ensure proper route matching.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	// Register routes directly under /v1/orgs/{orgId} without using Route()
	// This prevents the route group from intercepting GET /v1/orgs/{orgId} requests
	// Admin operations require user:manage or org:admin scope
	router.With(middleware.RequireAdminScope("user:manage", "org:admin")).Post("/v1/orgs/{orgId}/invites", handler.InviteUser)
	router.With(middleware.RequireAdminScope("user:manage", "org:admin")).Post("/v1/orgs/{orgId}/users", handler.CreateUser) // Direct user creation
	router.With(middleware.RequireAdminScope("org:read", "org:admin")).Get("/v1/orgs/{orgId}/users", handler.ListUsers)
	router.With(middleware.RequireAdminScope("org:read", "org:admin")).Get("/v1/orgs/{orgId}/users/{userId}", handler.GetUser)
	router.With(middleware.RequireAdminScope("user:manage", "org:admin")).Patch("/v1/orgs/{orgId}/users/{userId}", handler.UpdateUser)
	router.With(middleware.RequireAdminScope("user:manage", "org:admin")).Delete("/v1/orgs/{orgId}/users/{userId}", handler.DeleteUser)
	router.With(middleware.RequireAdminScope("user:manage", "org:admin")).Put("/v1/orgs/{orgId}/users/{userId}/roles", handler.UpdateUserRoles)
}

// Handler serves user management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// InviteUserRequest represents the payload for inviting a user.
type InviteUserRequest struct {
	Email          string   `json:"email"`
	Roles          []string `json:"roles,omitempty"`
	ExpiresInHours int      `json:"expiresInHours,omitempty"`
}

// InviteResponse represents an invite in API responses.
type InviteResponse struct {
	InviteID  string    `json:"inviteId"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// UserResponse represents a user in API responses.
type UserResponse struct {
	UserID      string         `json:"userId"`
	OrgID       string         `json:"orgId"`
	Email       string         `json:"email"`
	DisplayName string         `json:"displayName"`
	Status      string         `json:"status"`
	MFAEnrolled bool           `json:"mfaEnrolled"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
}

// UpdateUserRequest represents the payload for updating a user.
type UpdateUserRequest struct {
	DisplayName *string        `json:"displayName,omitempty"`
	Status      *string        `json:"status,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// RoleAssignmentRequest represents role assignment updates.
type RoleAssignmentRequest struct {
	Roles []string `json:"roles"`
}

// CreateUserRequest represents the payload for direct user creation.
type CreateUserRequest struct {
	Email          string   `json:"email"`
	DisplayName    string   `json:"displayName,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	ForcePwdChange *bool    `json:"forcePwdChange,omitempty"` // If true, user must change password on first login
}

// CreateUserResponse represents the response for direct user creation.
// Includes the temporary password which is shown only once.
type CreateUserResponse struct {
	UserID            string `json:"userId"`
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	Status            string `json:"status"`
	TemporaryPassword string `json:"temporaryPassword"`
	CreatedAt         string `json:"createdAt"`
}

// InviteUser handles POST /v1/orgs/{orgId}/invites - Invite a new user.
// Creates a user with status="invited" and generates a temporary password.
func (h *Handler) InviteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	// Resolve org ID (UUID or slug)
	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	var req InviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Validate email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		httputil.WriteBadRequest(w, r, "email is required")
		return
	}

	// Check if user already exists
	_, err = h.runtime.Postgres.GetUserByEmail(ctx, orgID, email)
	if err == nil {
		httputil.WriteConflict(w, r, "user with this email already exists")
		return
	}
	// ErrNotFound is expected, continue

	// Set invite expiry (default 72 hours)
	expiresInHours := req.ExpiresInHours
	if expiresInHours <= 0 {
		expiresInHours = 72
	}
	expiresAt := time.Now().Add(time.Duration(expiresInHours) * time.Hour)

	// Generate secure invite token
	inviteToken, err := generateInviteToken()
	if err != nil {
		h.logger.Error("failed to generate invite token", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Hash the invite token for secure storage
	tokenHash, err := security.HashPassword(inviteToken)
	if err != nil {
		h.logger.Error("failed to hash invite token", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Generate temporary password for user (user will reset on acceptance)
	tempPassword, err := generateInviteToken()
	if err != nil {
		h.logger.Error("failed to generate temporary password", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}
	passwordHash, err := security.HashPassword(tempPassword)
	if err != nil {
		h.logger.Error("failed to hash invite password", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Create user with invited status
	userID := uuid.New()
	actorID := getActorID(r)

	// Create user first (uses its own transaction with RLS)
	params := postgres.CreateUserParams{
		ID:             userID,
		OrgID:          orgID,
		Email:          email,
		DisplayName:    email, // Default to email until user sets display name
		PasswordHash:   passwordHash,
		Status:         "invited",
		MFAEnrolled:    false,
		MFAMethods:     []string{},
		RecoveryTokens: []string{},
		Metadata: map[string]any{
			"invite_expires_at": expiresAt.Format(time.RFC3339),
			"roles":             req.Roles,
		},
	}

	createdUser, err := h.runtime.Postgres.CreateUser(ctx, params)
	if err != nil {
		h.logger.Error("failed to create invited user", zap.Error(err), zap.String("email", email))
		httputil.WriteInternalError(w, r)
		return
	}

	// Set auto_grant for admin users
	if containsRole(req.Roles, "admin") {
		_, err := h.runtime.Postgres.SetUserAccessMode(ctx, orgID, createdUser.ID, "auto_grant")
		if err != nil {
			h.logger.Warn("failed to set auto_grant for admin user",
				zap.String("user_id", createdUser.ID.String()),
				zap.Error(err))
			// Don't fail user creation, just log warning
		}
	}

	// Store invite token hash in separate table (in tenant transaction for RLS)
	tx, err := h.runtime.Postgres.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		h.logger.Error("failed to begin transaction for invite token", zap.Error(err), zap.String("email", email))
		httputil.WriteInternalError(w, r)
		return
	}
	defer tx.Rollback(ctx)

	// Set tenant context for RLS
	escapedOrgID := strings.ReplaceAll(orgID.String(), "'", "''")
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.org_id = '%s'", escapedOrgID)); err != nil {
		h.logger.Error("failed to set tenant context", zap.Error(err), zap.String("email", email))
		httputil.WriteInternalError(w, r)
		return
	}

	// Store invite token hash
	_, err = tx.Exec(ctx, `
		INSERT INTO invite_tokens (org_id, user_id, token_hash, expires_at, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5)
	`, orgID, userID, tokenHash, expiresAt, actorID)
	if err != nil {
		h.logger.Error("failed to store invite token", zap.Error(err), zap.String("email", email))
		// User was created but token wasn't - this is a partial failure
		// In production, consider rolling back or marking user for cleanup
		httputil.WriteInternalError(w, r)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("failed to commit invite token transaction", zap.Error(err), zap.String("email", email))
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeSystem, audit.ActionUserInvite, audit.TargetTypeUser, &createdUser.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"email":      email,
		"roles":      req.Roles,
		"expires_at": expiresAt.Format(time.RFC3339),
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// TODO: Send invite email with token

	resp := InviteResponse{
		InviteID:  userID.String(),
		Email:     email,
		Status:    "pending",
		ExpiresAt: expiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// CreateUser handles POST /v1/orgs/{orgId}/users - Create a user directly.
// Creates an active user with a generated temporary password that is returned once.
// This is for admin/CLI use where email invitation is not needed.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	// Resolve org ID (UUID or slug)
	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Validate email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		httputil.WriteBadRequest(w, r, "email is required")
		return
	}

	// Check if user already exists
	_, err = h.runtime.Postgres.GetUserByEmail(ctx, orgID, email)
	if err == nil {
		httputil.WriteConflict(w, r, "user with this email already exists")
		return
	}
	// ErrNotFound is expected, continue

	// Generate secure temporary password (user should change on first login)
	tempPassword, err := generateSecurePassword()
	if err != nil {
		h.logger.Error("failed to generate temporary password", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}
	passwordHash, err := security.HashPassword(tempPassword)
	if err != nil {
		h.logger.Error("failed to hash password", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Set display name (default to email if not provided)
	displayName := req.DisplayName
	if displayName == "" {
		displayName = email
	}

	// Create user with active status
	userID := uuid.New()
	actorID := getActorID(r)

	// Determine if password must be changed on first login
	// Default to false (no forced password change) unless explicitly requested
	passwordMustChange := false
	if req.ForcePwdChange != nil && *req.ForcePwdChange {
		passwordMustChange = true
	}

	params := postgres.CreateUserParams{
		ID:             userID,
		OrgID:          orgID,
		Email:          email,
		DisplayName:    displayName,
		PasswordHash:   passwordHash,
		Status:         "active",
		MFAEnrolled:    false,
		MFAMethods:     []string{},
		RecoveryTokens: []string{},
		Metadata: map[string]any{
			"roles":                req.Roles,
			"password_must_change": passwordMustChange,
			"created_via":          "direct",
		},
	}

	createdUser, err := h.runtime.Postgres.CreateUser(ctx, params)
	if err != nil {
		h.logger.Error("failed to create user", zap.Error(err), zap.String("email", email))
		httputil.WriteInternalError(w, r)
		return
	}

	// Set auto_grant for admin users
	if containsRole(req.Roles, "admin") {
		_, err := h.runtime.Postgres.SetUserAccessMode(ctx, orgID, createdUser.ID, "auto_grant")
		if err != nil {
			h.logger.Warn("failed to set auto_grant for admin user",
				zap.String("user_id", createdUser.ID.String()),
				zap.Error(err))
			// Don't fail user creation, just log warning
		}
	}

	// Emit audit event
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeSystem, audit.ActionUserCreate, audit.TargetTypeUser, &createdUser.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"email":       email,
		"roles":       req.Roles,
		"created_via": "direct",
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	resp := CreateUserResponse{
		UserID:            createdUser.ID.String(),
		Email:             createdUser.Email,
		DisplayName:       createdUser.DisplayName,
		Status:            createdUser.Status,
		TemporaryPassword: tempPassword, // Shown only once
		CreatedAt:         createdUser.CreatedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// ListUsers handles GET /v1/orgs/{orgId}/users - List users in organization.
// TODO: Add pagination and filtering.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	// Authorization: verify user has access to this organization
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.logger.Warn("unauthorized access to list users", zap.Error(err), zap.String("orgId", orgIDParam))
		httputil.WriteForbidden(w, r, "forbidden")
		return
	}

	users, err := h.runtime.Postgres.ListUsersInOrg(ctx, orgID)
	if err != nil {
		h.logger.Error("failed to list users", zap.Error(err), zap.String("orgId", orgIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	// Convert to response format
	var responses []UserResponse
	for _, user := range users {
		responses = append(responses, toUserResponse(user))
	}
	if responses == nil {
		responses = []UserResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// GetUser handles GET /v1/orgs/{orgId}/users/{userId} - Retrieve user details.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid user ID")
		return
	}

	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "user", "")
			return
		}
		h.logger.Error("failed to get user", zap.Error(err), zap.String("userId", userIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	resp := toUserResponse(user)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// UpdateUser handles PATCH /v1/orgs/{orgId}/users/{userId} - Update user metadata.
// Supports updating display name, status, and metadata. Uses optimistic locking.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid user ID")
		return
	}

	// Get existing user to obtain version
	existingUser, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "user", "")
			return
		}
		h.logger.Error("failed to get user for update", zap.Error(err), zap.String("userId", userIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Update status if provided
	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "suspended": true, "invited": true}
		if !validStatuses[*req.Status] {
			httputil.WriteBadRequest(w, r, "invalid status")
			return
		}

		statusParams := postgres.UpdateUserStatusParams{
			OrgID:   orgID,
			ID:      userID,
			Version: existingUser.Version,
			Status:  *req.Status,
		}

		user, err := h.runtime.Postgres.UpdateUserStatus(ctx, statusParams)
		if err != nil {
			if err == postgres.ErrOptimisticLock {
				httputil.WriteConflict(w, r, "user was modified concurrently")
				return
			}
			h.logger.Error("failed to update user status", zap.Error(err), zap.String("userId", userIDParam))
			httputil.WriteInternalError(w, r)
			return
		}

		// Emit audit event
		actorID := getActorID(r)
		action := audit.ActionUserUpdate
		if *req.Status == "suspended" {
			action = audit.ActionUserSuspend
		} else if *req.Status == "active" && existingUser.Status == "suspended" {
			action = audit.ActionUserActivate
		}
		event := audit.BuildEvent(orgID, actorID, audit.ActorTypeSystem, action, audit.TargetTypeUser, &userID)
		event = audit.BuildEventFromRequest(event, r)
		event.Metadata = map[string]any{
			"previous_status": existingUser.Status,
			"new_status":      user.Status,
		}
		_ = h.runtime.Audit.Emit(ctx, event)

		resp := toUserResponse(user)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			h.logger.Error("failed to encode response", zap.Error(err))
		}
		return
	}

	// Update profile (display name, metadata) if provided
	if req.DisplayName != nil || req.Metadata != nil {
		profileParams := postgres.UpdateUserProfileParams{
			OrgID:       orgID,
			ID:          userID,
			Version:     existingUser.Version,
			DisplayName: existingUser.DisplayName,
			MFAEnrolled: existingUser.MFAEnrolled,
			MFAMethods:  existingUser.MFAMethods,
			Metadata:    existingUser.Metadata,
		}

		if req.DisplayName != nil {
			profileParams.DisplayName = *req.DisplayName
		}
		if req.Metadata != nil {
			profileParams.Metadata = req.Metadata
		}

		user, err := h.runtime.Postgres.UpdateUserProfile(ctx, profileParams)
		if err != nil {
			if err == postgres.ErrOptimisticLock {
				httputil.WriteConflict(w, r, "user was modified concurrently")
				return
			}
			h.logger.Error("failed to update user profile", zap.Error(err), zap.String("userId", userIDParam))
			httputil.WriteInternalError(w, r)
			return
		}

		// Emit audit event
		actorID := getActorID(r)
		event := audit.BuildEvent(orgID, actorID, audit.ActorTypeSystem, audit.ActionUserUpdate, audit.TargetTypeUser, &userID)
		event = audit.BuildEventFromRequest(event, r)
		event.Metadata = map[string]any{
			"updated_fields": []string{},
		}
		if req.DisplayName != nil {
			event.Metadata["updated_fields"] = append(event.Metadata["updated_fields"].([]string), "display_name")
		}
		if req.Metadata != nil {
			event.Metadata["updated_fields"] = append(event.Metadata["updated_fields"].([]string), "metadata")
		}
		_ = h.runtime.Audit.Emit(ctx, event)

		resp := toUserResponse(user)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			h.logger.Error("failed to encode response", zap.Error(err))
		}
		return
	}

	// No fields to update
	httputil.WriteBadRequest(w, r, "no fields provided for update")
}

// DeleteUser handles DELETE /v1/orgs/{orgId}/users/{userId} - Soft delete a user.
// This sets deleted_at timestamp and revokes all active API keys for the user.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid user ID")
		return
	}

	// Verify user exists before deletion
	existingUser, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "user", "")
			return
		}
		h.logger.Error("failed to get user for deletion", zap.Error(err), zap.String("userId", userIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	// Authorization: only org admins or system admins can delete users
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.logger.Warn("unauthorized access to delete user", zap.Error(err), zap.String("userId", userIDParam))
		httputil.WriteForbidden(w, r, "forbidden")
		return
	}

	// Soft delete the user
	if err := h.runtime.Postgres.DeleteUser(ctx, orgID, userID); err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "user", "")
			return
		}
		h.logger.Error("failed to delete user", zap.Error(err), zap.String("userId", userIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	// Revoke all active API keys for this user
	apiKeys, err := h.runtime.Postgres.ListAPIKeysForPrincipal(ctx, orgID, postgres.PrincipalTypeUser, userID)
	if err != nil {
		h.logger.Warn("failed to list API keys for deletion", zap.Error(err), zap.String("userId", userIDParam))
		// Don't fail the deletion - user is already deleted
	} else {
		now := time.Now()
		for _, key := range apiKeys {
			if key.RevokedAt == nil {
				_, err := h.runtime.Postgres.RevokeAPIKey(ctx, postgres.RevokeAPIKeyParams{
					ID:        key.ID,
					Version:   key.Version,
					Status:    "revoked",
					RevokedAt: now,
				}, orgID)
				if err != nil {
					h.logger.Warn("failed to revoke API key during user deletion",
						zap.Error(err),
						zap.String("userId", userIDParam),
						zap.String("apiKeyId", key.ID.String()))
					// Continue revoking other keys even if one fails
				}
			}
		}
	}

	// Emit audit event
	actorID := getActorID(r)
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeSystem, audit.ActionUserDelete, audit.TargetTypeUser, &userID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"email":  existingUser.Email,
		"status": existingUser.Status,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Return 204 No Content on successful deletion
	w.WriteHeader(http.StatusNoContent)
}

// UpdateUserRoles handles PUT /v1/orgs/{orgId}/users/{userId}/roles - Update role assignments.
// TODO: Implement role storage and assignment logic.
func (h *Handler) UpdateUserRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		httputil.WriteNotFound(w, r, "organization", "")
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid user ID")
		return
	}

	var req RoleAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// TODO: Implement role assignment (requires roles table and user_roles junction)
	// For now, store roles in user metadata as temporary solution
	// TODO: Emit audit event

	h.logger.Info("role assignment requested (not yet implemented)",
		zap.String("orgId", orgID.String()),
		zap.String("userId", userID.String()),
		zap.Strings("roles", req.Roles))

	_ = ctx // Suppress unused variable warning
	w.WriteHeader(http.StatusNotImplemented)
}

// resolveOrgID resolves an org identifier (UUID or slug) to a UUID.
func (h *Handler) resolveOrgID(ctx context.Context, orgIDParam string) (uuid.UUID, error) {
	if orgID, err := uuid.Parse(orgIDParam); err == nil {
		// Verify org exists
		_, err := h.runtime.Postgres.GetOrg(ctx, orgID)
		return orgID, err
	}
	// Treat as slug
	org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

// toUserResponse converts a postgres.User to a UserResponse.
func toUserResponse(user postgres.User) UserResponse {
	return UserResponse{
		UserID:      user.ID.String(),
		OrgID:       user.OrgID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Status:      user.Status,
		MFAEnrolled: user.MFAEnrolled,
		Metadata:    user.Metadata,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// getActorID extracts the actor ID from the request context.
// Requires RequireAuth middleware to be applied to the route.
func getActorID(r *http.Request) uuid.UUID {
	return middleware.GetUserID(r.Context())
}

// generateInviteToken generates a secure random token for user invites.
// Uses crypto/rand for cryptographically secure random bytes.
func generateInviteToken() (string, error) {
	// Generate 32 bytes of random data
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	// Encode as URL-safe base64 (no padding)
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generateSecurePassword generates a secure random password for direct user creation.
// Returns a human-readable password with mixed case, numbers, and special chars.
func generateSecurePassword() (string, error) {
	// Character sets for password generation
	const (
		lowercase = "abcdefghijkmnopqrstuvwxyz"  // Excluding l for readability
		uppercase = "ABCDEFGHJKLMNPQRSTUVWXYZ"   // Excluding I, O for readability
		digits    = "23456789"                   // Excluding 0, 1 for readability
		special   = "!@#$%^&*"
	)
	allChars := lowercase + uppercase + digits + special

	// Generate 16-character password
	passwordLen := 16
	password := make([]byte, passwordLen)

	// Ensure at least one character from each set
	charSets := []string{lowercase, uppercase, digits, special}
	for i, set := range charSets {
		idx := make([]byte, 1)
		if _, err := rand.Read(idx); err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}
		password[i] = set[int(idx[0])%len(set)]
	}

	// Fill remaining positions with random characters from all sets
	for i := len(charSets); i < passwordLen; i++ {
		idx := make([]byte, 1)
		if _, err := rand.Read(idx); err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}
		password[i] = allChars[int(idx[0])%len(allChars)]
	}

	// Shuffle the password to avoid predictable positions
	for i := passwordLen - 1; i > 0; i-- {
		jByte := make([]byte, 1)
		if _, err := rand.Read(jByte); err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}
		j := int(jByte[0]) % (i + 1)
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

// requireOrgAccess checks if the authenticated user has access to the specified organization.
// Service accounts with admin or wildcard scope can access any org.
// Regular users must be a member of the org.
func (h *Handler) requireOrgAccess(ctx context.Context, orgID uuid.UUID) error {
	// Service accounts with admin scope (or wildcard "*" scope) can access any org
	if middleware.HasAnyScope(ctx, "admin", "*") {
		return nil
	}

	// Get authenticated user's org from context
	authOrgID := middleware.GetOrgID(ctx)
	userID := middleware.GetUserID(ctx)

	// If user is in the same org, allow access
	if authOrgID == orgID {
		return nil
	}

	// Otherwise, verify user is a member of the target org
	return h.runtime.Postgres.ValidateUserOrgMembership(ctx, userID, orgID)
}

// containsRole checks if a role exists in the roles slice (case-insensitive).
func containsRole(roles []string, target string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, target) {
			return true
		}
	}
	return false
}
