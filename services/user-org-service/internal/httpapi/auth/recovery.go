// Package auth provides password recovery endpoints.
//
// Purpose:
//
//	This package implements password recovery flows:
//	- Initiate recovery (generate recovery token)
//	- Verify recovery token
//	- Reset password with recovery token
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router
//   - internal/bootstrap: Runtime dependencies
//   - internal/storage/postgres: User data access
//   - internal/security: Password hashing
//
// Key Responsibilities:
//   - InitiateRecovery: POST /v1/auth/recover - Generate recovery token
//   - VerifyRecoveryToken: POST /v1/auth/recover/verify - Verify token validity
//   - ResetPassword: POST /v1/auth/recover/reset - Reset password with token
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-007 (Credential Recovery)
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/metrics"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// InitiateRecoveryRequest represents the payload for initiating recovery.
type InitiateRecoveryRequest struct {
	Email string `json:"email"`
	OrgID string `json:"org_id,omitempty"` // Optional: UUID or slug
}

// InitiateRecoveryResponse represents the response after initiating recovery.
type InitiateRecoveryResponse struct {
	Message string `json:"message"`
	// Token is only returned in development/testing - in production, send via email
	Token string `json:"token,omitempty"`
}

// VerifyRecoveryTokenRequest represents the payload for verifying a recovery token.
type VerifyRecoveryTokenRequest struct {
	Token string `json:"token"`
	Email string `json:"email"` // Email is required to find the user
	OrgID string `json:"org_id,omitempty"`
}

// VerifyRecoveryTokenResponse represents the response after verifying a token.
type VerifyRecoveryTokenResponse struct {
	Valid   bool   `json:"valid"`
	Email   string `json:"email,omitempty"`
	Message string `json:"message,omitempty"`
}

// ResetPasswordRequest represents the payload for resetting password.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	Email       string `json:"email"` // Email is required to find the user
	NewPassword string `json:"newPassword"`
	OrgID       string `json:"org_id,omitempty"`
}

// ResetPasswordResponse represents the response after resetting password.
type ResetPasswordResponse struct {
	Message string `json:"message"`
}

// InitiateRecovery handles POST /v1/auth/recover.
// Generates a recovery token and stores it in the user's recovery_tokens array.
func (h *Handler) InitiateRecovery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req InitiateRecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	// Resolve org ID
	var orgID uuid.UUID
	var err error
	if req.OrgID != "" {
		if orgID, err = uuid.Parse(req.OrgID); err != nil {
			// Try as slug
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
			if err != nil {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			orgID = org.ID
		}
	} else {
		// If no org_id provided, we need to find user first
		// For now, return error - org_id should be provided
		http.Error(w, "org_id is required", http.StatusBadRequest)
		return
	}

	// Find user by email
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, req.Email)
	if err != nil {
		// Don't reveal if user exists (prevent user enumeration)
		// Return success message anyway
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(InitiateRecoveryResponse{
			Message: "If an account exists with this email, a recovery token has been generated",
		})
		return
	}

	// Check if user is active
	if user.Status != "active" {
		// Don't reveal account status
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(InitiateRecoveryResponse{
			Message: "If an account exists with this email, a recovery token has been generated",
		})
		return
	}

	// Generate recovery token (32 bytes, base64url encoded)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "failed to generate recovery token", http.StatusInternalServerError)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	// Hash token for storage (similar to password hashing)
	tokenHash, err := security.HashPassword(token)
	if err != nil {
		http.Error(w, "failed to hash recovery token", http.StatusInternalServerError)
		return
	}

	// Add token to user's recovery_tokens array
	// Tokens expire after 24 hours
	recoveryToken := map[string]interface{}{
		"hash":       tokenHash,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"expires_at": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		"used":       false,
	}

	// Add admin approval status if required
	if h.runtime.Config.RecoveryRequiresAdminApproval {
		recoveryToken["status"] = "pending"
		recoveryToken["approved_at"] = nil
		recoveryToken["approved_by"] = nil
	} else {
		recoveryToken["status"] = "approved" // Auto-approved if admin approval not required
	}

	// Get current recovery tokens
	currentTokens := user.RecoveryTokens
	if currentTokens == nil {
		currentTokens = []string{}
	}

	// Add new token (store as JSON string in array)
	tokenJSON, _ := json.Marshal(recoveryToken)
	newTokens := append(currentTokens, string(tokenJSON))

	// Update user with new recovery token
	_, err = h.runtime.Postgres.UpdateUserRecoveryTokens(ctx, orgID, user.ID, user.Version, newTokens)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			// User was modified concurrently - still return success to prevent enumeration
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(InitiateRecoveryResponse{
				Message: "If an account exists with this email, a recovery token has been generated",
			})
			return
		}
		// Log error (runtime logger available via bootstrap)
		// Note: In production, use structured logging from runtime
		// Still return success to prevent user enumeration
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(InitiateRecoveryResponse{
			Message: "If an account exists with this email, a recovery token has been generated",
		})
		return
	}

	// Emit audit event
	action := audit.ActionRecoveryInitiate
	if h.runtime.Config.RecoveryRequiresAdminApproval {
		action = audit.ActionRecoveryInitiate // Status will be "pending" in metadata
	}
	event := audit.BuildEvent(orgID, user.ID, audit.ActorTypeSystem, action, audit.TargetTypeUser, &user.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"status": recoveryToken["status"],
		"email":  req.Email,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Record recovery attempt
	metrics.RecordRecoveryAttempt("initiate")

	// In development/testing, return token in response
	// In production, send token via email
	devMode := h.runtime.Config.Environment == "development"
	response := InitiateRecoveryResponse{
		Message: "If an account exists with this email, a recovery token has been generated",
	}
	if devMode {
		response.Token = token
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// VerifyRecoveryToken handles POST /v1/auth/recover/verify.
// Verifies that a recovery token is valid and not expired.
func (h *Handler) VerifyRecoveryToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req VerifyRecoveryTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.Email == "" {
		http.Error(w, "token and email are required", http.StatusBadRequest)
		return
	}

	// Resolve org ID
	var orgID uuid.UUID
	var err error
	if req.OrgID != "" {
		if orgID, err = uuid.Parse(req.OrgID); err != nil {
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
			if err != nil {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			orgID = org.ID
		}
	} else {
		http.Error(w, "org_id is required", http.StatusBadRequest)
		return
	}

	// Find user by email
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, req.Email)
	if err != nil {
		// Don't reveal if user exists
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(VerifyRecoveryTokenResponse{
			Valid:   false,
			Message: "Invalid or expired recovery token",
		})
		return
	}

	// Verify token in user's recovery_tokens array
	valid := h.verifyRecoveryTokenInUser(user, req.Token)
	if !valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(VerifyRecoveryTokenResponse{
			Valid:   false,
			Message: "Invalid or expired recovery token",
		})
		return
	}

	// Record recovery verification attempt
	metrics.RecordRecoveryAttempt("verify")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(VerifyRecoveryTokenResponse{
		Valid: true,
		Email: user.Email,
	})
}

// ResetPassword handles POST /v1/auth/recover/reset.
// Resets the user's password using a valid recovery token.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		http.Error(w, "token and newPassword are required", http.StatusBadRequest)
		return
	}

	// Validate password strength
	if len(req.NewPassword) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Resolve org ID
	var orgID uuid.UUID
	var err error
	if req.OrgID != "" {
		if orgID, err = uuid.Parse(req.OrgID); err != nil {
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
			if err != nil {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			orgID = org.ID
		}
	} else {
		http.Error(w, "org_id is required", http.StatusBadRequest)
		return
	}

	// Find user by email
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, req.Email)
	if err != nil {
		// Don't reveal if user exists
		http.Error(w, "invalid or expired recovery token", http.StatusBadRequest)
		return
	}

	// Verify token in user's recovery_tokens array
	if !h.verifyRecoveryTokenInUser(user, req.Token) {
		http.Error(w, "invalid or expired recovery token", http.StatusBadRequest)
		return
	}

	// Hash new password
	passwordHash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	// Update password
	updatedUser, err := h.runtime.Postgres.UpdateUserPasswordHash(ctx, postgres.UpdateUserPasswordHashParams{
		OrgID:        orgID,
		ID:           user.ID,
		Version:      user.Version,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			http.Error(w, "user was modified concurrently", http.StatusConflict)
			return
		}
		http.Error(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	// Mark recovery token as used
	updatedTokens := h.markRecoveryTokenAsUsed(user.RecoveryTokens, req.Token)
	if len(updatedTokens) != len(user.RecoveryTokens) {
		// Token was found and marked as used - update user
		_, err = h.runtime.Postgres.UpdateUserRecoveryTokens(ctx, orgID, updatedUser.ID, updatedUser.Version, updatedTokens)
		if err != nil {
			// Non-fatal: log but don't fail the password reset
			// Token is already used functionally (password changed)
		}
	}

	// Emit audit event
	event := audit.BuildEvent(orgID, updatedUser.ID, audit.ActorTypeSystem, audit.ActionRecoveryComplete, audit.TargetTypeUser, &updatedUser.ID)
	event = audit.BuildEventFromRequest(event, r)
	_ = h.runtime.Audit.Emit(ctx, event)

	// Record recovery reset attempt
	metrics.RecordRecoveryAttempt("reset")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ResetPasswordResponse{
		Message: "Password has been reset successfully",
	})
}

// verifyRecoveryTokenInUser verifies a recovery token against a user's recovery_tokens array.
func (h *Handler) verifyRecoveryTokenInUser(user postgres.User, token string) bool {
	for _, tokenStr := range user.RecoveryTokens {
		var tokenData map[string]interface{}
		if err := json.Unmarshal([]byte(tokenStr), &tokenData); err != nil {
			continue
		}

		// Check if token is used
		if used, ok := tokenData["used"].(bool); ok && used {
			continue
		}

		// Check expiration
		expiresAtStr, ok := tokenData["expires_at"].(string)
		if !ok {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			continue
		}
		if expiresAt.Before(time.Now()) {
			continue
		}

		// Check approval status if admin approval is required
		if h.runtime.Config.RecoveryRequiresAdminApproval {
			status, ok := tokenData["status"].(string)
			if !ok || status != "approved" {
				continue // Token not approved yet
			}
		}

		// Verify token hash
		hash, ok := tokenData["hash"].(string)
		if !ok {
			continue
		}
		valid, err := security.VerifyPassword(token, hash)
		if err == nil && valid {
			return true
		}
	}

	return false
}

// markRecoveryTokenAsUsed marks a recovery token as used in the tokens array.
func (h *Handler) markRecoveryTokenAsUsed(tokens []string, token string) []string {
	result := make([]string, 0, len(tokens))
	for _, tokenStr := range tokens {
		var tokenData map[string]interface{}
		if err := json.Unmarshal([]byte(tokenStr), &tokenData); err != nil {
			result = append(result, tokenStr) // Keep invalid tokens as-is
			continue
		}

		// Check if this is the token we're looking for
		hash, ok := tokenData["hash"].(string)
		if ok {
			valid, err := security.VerifyPassword(token, hash)
			if err == nil && valid {
				// Mark as used
				tokenData["used"] = true
				tokenData["used_at"] = time.Now().UTC().Format(time.RFC3339)
				tokenJSON, _ := json.Marshal(tokenData)
				result = append(result, string(tokenJSON))
				continue
			}
		}

		// Not the token we're looking for - keep as-is
		result = append(result, tokenStr)
	}
	return result
}
