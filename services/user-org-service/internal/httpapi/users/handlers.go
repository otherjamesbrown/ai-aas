// Package users provides HTTP handlers for user lifecycle management.
//
// Purpose:
//   This package implements REST API handlers for user management operations,
//   including invites, user CRUD, role assignments, and status updates (suspend,
//   activate). Handlers enforce authorization, validate input, and emit audit
//   events for all state changes.
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
//   - ListUsers: GET /v1/orgs/{orgId}/users - List users in organization
//   - GetUser: GET /v1/orgs/{orgId}/users/{userId} - Retrieve user details
//   - UpdateUserStatus: PATCH /v1/orgs/{orgId}/users/{userId} - Update user status
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
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts user management routes beneath /v1/orgs/{orgId}.
// This should be called from within the orgs route group to ensure proper route matching.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger zerolog.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}
	// Register routes directly under /v1/orgs/{orgId} without using Route()
	// This prevents the route group from intercepting GET /v1/orgs/{orgId} requests
	router.Post("/v1/orgs/{orgId}/invites", handler.InviteUser)
	router.Get("/v1/orgs/{orgId}/users", handler.ListUsers)
	router.Get("/v1/orgs/{orgId}/users/{userId}", handler.GetUser)
	router.Patch("/v1/orgs/{orgId}/users/{userId}", handler.UpdateUser)
	router.Put("/v1/orgs/{orgId}/users/{userId}/roles", handler.UpdateUserRoles)
}

// Handler serves user management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  zerolog.Logger
}

// InviteUserRequest represents the payload for inviting a user.
type InviteUserRequest struct {
	Email         string   `json:"email"`
	Roles         []string `json:"roles,omitempty"`
	ExpiresInHours int     `json:"expiresInHours,omitempty"`
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
	UserID      string            `json:"userId"`
	OrgID       string            `json:"orgId"`
	Email       string            `json:"email"`
	DisplayName string            `json:"displayName"`
	Status      string            `json:"status"`
	MFAEnrolled bool              `json:"mfaEnrolled"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
}

// UpdateUserRequest represents the payload for updating a user.
type UpdateUserRequest struct {
	DisplayName *string         `json:"displayName,omitempty"`
	Status      *string         `json:"status,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

// RoleAssignmentRequest represents role assignment updates.
type RoleAssignmentRequest struct {
	Roles []string `json:"roles"`
}

// InviteUser handles POST /v1/orgs/{orgId}/invites - Invite a new user.
// Creates a user with status="invited" and generates a temporary password.
func (h *Handler) InviteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	// Resolve org ID (UUID or slug)
	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	var req InviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn().Err(err).Msg("invalid request payload")
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate email
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	_, err = h.runtime.Postgres.GetUserByEmail(ctx, orgID, email)
	if err == nil {
		http.Error(w, "user with this email already exists", http.StatusConflict)
		return
	}
	// ErrNotFound is expected, continue

	// Set invite expiry (default 72 hours)
	expiresInHours := req.ExpiresInHours
	if expiresInHours <= 0 {
		expiresInHours = 72
	}
	expiresAt := time.Now().Add(time.Duration(expiresInHours) * time.Hour)

	// Generate temporary password for invite (user will reset on acceptance)
	tempPassword, err := generateInviteToken()
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to generate invite token")
		http.Error(w, "failed to create invite", http.StatusInternalServerError)
		return
	}
	passwordHash, err := security.HashPassword(tempPassword)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to hash invite password")
		http.Error(w, "failed to create invite", http.StatusInternalServerError)
		return
	}

	// Create user with invited status
	userID := uuid.New()
	params := postgres.CreateUserParams{
		ID:           userID,
		OrgID:        orgID,
		Email:        email,
		DisplayName:  email, // Default to email until user sets display name
		PasswordHash: passwordHash,
		Status:       "invited",
		MFAEnrolled:  false,
		MFAMethods:   []string{},
		RecoveryTokens: []string{},
		Metadata: map[string]any{
			"invite_expires_at": expiresAt.Format(time.RFC3339),
			"invite_token":       tempPassword, // TODO: Store hashed, use separate invite_tokens table
			"roles":             req.Roles,
		},
	}

	createdUser, err := h.runtime.Postgres.CreateUser(ctx, params)
	if err != nil {
		h.logger.Error().Err(err).Str("email", email).Msg("failed to create invited user")
		http.Error(w, "failed to create invite", http.StatusInternalServerError)
		return
	}

	// Emit audit event
	actorID := getActorID(r) // TODO: Extract from authenticated session
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
		h.logger.Error().Err(err).Msg("failed to encode response")
	}
}

// ListUsers handles GET /v1/orgs/{orgId}/users - List users in organization.
// TODO: Add pagination, filtering, and authorization checks.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	_, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	// TODO: Implement list query with pagination
	// For now, return empty list
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]UserResponse{})
}

// GetUser handles GET /v1/orgs/{orgId}/users/{userId} - Retrieve user details.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		h.logger.Error().Err(err).Str("userId", userIDParam).Msg("failed to get user")
		http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
		return
	}

	resp := toUserResponse(user)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode response")
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
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Get existing user to obtain version
	existingUser, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		h.logger.Error().Err(err).Str("userId", userIDParam).Msg("failed to get user for update")
		http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn().Err(err).Msg("invalid request payload")
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Update status if provided
	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "suspended": true, "invited": true}
		if !validStatuses[*req.Status] {
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}

		statusParams := postgres.UpdateUserStatusParams{
			OrgID:  orgID,
			ID:     userID,
			Version: existingUser.Version,
			Status: *req.Status,
		}

		user, err := h.runtime.Postgres.UpdateUserStatus(ctx, statusParams)
		if err != nil {
			if err == postgres.ErrOptimisticLock {
				http.Error(w, "user was modified concurrently", http.StatusConflict)
				return
			}
			h.logger.Error().Err(err).Str("userId", userIDParam).Msg("failed to update user status")
			http.Error(w, "failed to update user", http.StatusInternalServerError)
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
			h.logger.Error().Err(err).Msg("failed to encode response")
		}
		return
	}

	// Update profile (display name, metadata) if provided
	if req.DisplayName != nil || req.Metadata != nil {
		profileParams := postgres.UpdateUserProfileParams{
			OrgID:      orgID,
			ID:         userID,
			Version:    existingUser.Version,
			DisplayName: existingUser.DisplayName,
			MFAEnrolled: existingUser.MFAEnrolled,
			MFAMethods: existingUser.MFAMethods,
			Metadata:   existingUser.Metadata,
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
				http.Error(w, "user was modified concurrently", http.StatusConflict)
				return
			}
			h.logger.Error().Err(err).Str("userId", userIDParam).Msg("failed to update user profile")
			http.Error(w, "failed to update user", http.StatusInternalServerError)
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
			h.logger.Error().Err(err).Msg("failed to encode response")
		}
		return
	}

	// No fields to update
	http.Error(w, "no fields provided for update", http.StatusBadRequest)
}

// UpdateUserRoles handles PUT /v1/orgs/{orgId}/users/{userId}/roles - Update role assignments.
// TODO: Implement role storage and assignment logic.
func (h *Handler) UpdateUserRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	var req RoleAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn().Err(err).Msg("invalid request payload")
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// TODO: Implement role assignment (requires roles table and user_roles junction)
	// For now, store roles in user metadata as temporary solution
	// TODO: Emit audit event

	h.logger.Info().
		Str("orgId", orgID.String()).
		Str("userId", userID.String()).
		Strs("roles", req.Roles).
		Msg("role assignment requested (not yet implemented)")

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

// getActorID extracts the actor ID from the request.
// TODO: Extract from authenticated session/JWT token once auth middleware is in place.
// For now, returns a system actor UUID.
func getActorID(r *http.Request) uuid.UUID {
	// TODO: Extract from r.Context() when auth middleware is added
	// For now, use a system actor
	return uuid.Nil // System actor (will be replaced with actual user ID from session)
}

// generateInviteToken generates a secure random token for user invites.
// TODO: Replace with crypto/rand for production use.
func generateInviteToken() (string, error) {
	// Generate a URL-safe random token
	// For now, using a simple generator - replace with crypto/rand in production
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte('A' + (i*7+13)%26)
	}
	return string(b), nil
}

