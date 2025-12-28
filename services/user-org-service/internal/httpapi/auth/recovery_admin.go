// Package auth provides admin endpoints for recovery request approval.
//
// Purpose:
//
//	This package implements admin endpoints for approving/rejecting recovery requests
//	when RECOVERY_REQUIRES_ADMIN_APPROVAL is enabled.
package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httputil"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// ApproveRecoveryRequest represents the payload for approving a recovery request.
type ApproveRecoveryRequest struct {
	Token string `json:"token"` // Recovery token to approve
	Email string `json:"email"` // User email
	OrgID string `json:"org_id,omitempty"`
}

// RejectRecoveryRequest represents the payload for rejecting a recovery request.
type RejectRecoveryRequest struct {
	Token  string `json:"token"` // Recovery token to reject
	Email  string `json:"email"` // User email
	OrgID  string `json:"org_id,omitempty"`
	Reason string `json:"reason,omitempty"` // Optional rejection reason
}

// ApproveRecovery handles POST /v1/auth/recover/approve.
// Approves a pending recovery request, allowing the user to reset their password.
func (h *Handler) ApproveRecovery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.runtime.Config.RecoveryRequiresAdminApproval {
		httputil.WriteBadRequest(w, r, "admin approval not required")
		return
	}

	// Get admin actor ID from context (set by auth middleware)
	actorID := middleware.GetUserID(r.Context())
	if actorID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "unauthorized")
		return
	}

	var req ApproveRecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	if req.Token == "" || req.Email == "" {
		httputil.WriteBadRequest(w, r, "token and email are required")
		return
	}

	// Resolve org ID
	var orgID uuid.UUID
	var err error
	if req.OrgID != "" {
		if orgID, err = uuid.Parse(req.OrgID); err != nil {
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
			if err != nil {
				httputil.WriteNotFound(w, r, "organization", "")
				return
			}
			orgID = org.ID
		}
	} else {
		httputil.WriteBadRequest(w, r, "org_id is required")
		return
	}

	// Find user by email
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, req.Email)
	if err != nil {
		httputil.WriteNotFound(w, r, "user", "")
		return
	}

	// Find and approve the recovery token
	updatedTokens, found := h.approveRecoveryToken(user.RecoveryTokens, req.Token, actorID)
	if !found {
		httputil.WriteNotFound(w, r, "recovery token", "")
		return
	}

	// Update user with approved token
	_, err = h.runtime.Postgres.UpdateUserRecoveryTokens(ctx, orgID, user.ID, user.Version, updatedTokens)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			httputil.WriteConflict(w, r, "user was modified concurrently")
			return
		}
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionRecoveryApprove, audit.TargetTypeUser, &user.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"email": req.Email,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Recovery request approved",
	})
}

// RejectRecovery handles POST /v1/auth/recover/reject.
// Rejects a pending recovery request, marking the token as rejected.
func (h *Handler) RejectRecovery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.runtime.Config.RecoveryRequiresAdminApproval {
		httputil.WriteBadRequest(w, r, "admin approval not required")
		return
	}

	// Get admin actor ID from context (set by auth middleware)
	actorID := middleware.GetUserID(r.Context())
	if actorID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "unauthorized")
		return
	}

	var req RejectRecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	if req.Token == "" || req.Email == "" {
		httputil.WriteBadRequest(w, r, "token and email are required")
		return
	}

	// Resolve org ID
	var orgID uuid.UUID
	var err error
	if req.OrgID != "" {
		if orgID, err = uuid.Parse(req.OrgID); err != nil {
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
			if err != nil {
				httputil.WriteNotFound(w, r, "organization", "")
				return
			}
			orgID = org.ID
		}
	} else {
		httputil.WriteBadRequest(w, r, "org_id is required")
		return
	}

	// Find user by email
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, req.Email)
	if err != nil {
		httputil.WriteNotFound(w, r, "user", "")
		return
	}

	// Find and reject the recovery token (mark as used to invalidate it)
	updatedTokens, found := h.rejectRecoveryToken(user.RecoveryTokens, req.Token)
	if !found {
		httputil.WriteNotFound(w, r, "recovery token", "")
		return
	}

	// Update user with rejected token
	_, err = h.runtime.Postgres.UpdateUserRecoveryTokens(ctx, orgID, user.ID, user.Version, updatedTokens)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			httputil.WriteConflict(w, r, "user was modified concurrently")
			return
		}
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionRecoveryReject, audit.TargetTypeUser, &user.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"email":  req.Email,
		"reason": req.Reason,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Recovery request rejected",
	})
}

// approveRecoveryToken finds a recovery token and marks it as approved.
func (h *Handler) approveRecoveryToken(tokens []string, token string, approvedBy uuid.UUID) ([]string, bool) {
	result := make([]string, 0, len(tokens))
	found := false
	for _, tokenStr := range tokens {
		var tokenData map[string]interface{}
		if err := json.Unmarshal([]byte(tokenStr), &tokenData); err != nil {
			result = append(result, tokenStr)
			continue
		}

		// Check if this is the token we're looking for
		hash, ok := tokenData["hash"].(string)
		if !ok {
			result = append(result, tokenStr)
			continue
		}
		valid, err := security.VerifyPassword(token, hash)
		if err == nil && valid && !found {
			// Found the token - approve it
			tokenData["status"] = "approved"
			tokenData["approved_at"] = time.Now().UTC().Format(time.RFC3339)
			tokenData["approved_by"] = approvedBy.String()
			found = true
		}

		// Re-marshal token data
		tokenJSON, _ := json.Marshal(tokenData)
		result = append(result, string(tokenJSON))
	}
	return result, found
}

// rejectRecoveryToken finds a recovery token and marks it as used (rejected).
func (h *Handler) rejectRecoveryToken(tokens []string, token string) ([]string, bool) {
	result := make([]string, 0, len(tokens))
	found := false
	for _, tokenStr := range tokens {
		var tokenData map[string]interface{}
		if err := json.Unmarshal([]byte(tokenStr), &tokenData); err != nil {
			result = append(result, tokenStr)
			continue
		}

		// Check if this is the token we're looking for
		hash, ok := tokenData["hash"].(string)
		if !ok {
			result = append(result, tokenStr)
			continue
		}
		valid, err := security.VerifyPassword(token, hash)
		if err == nil && valid && !found {
			// Found the token - mark as used (rejected)
			tokenData["used"] = true
			tokenData["status"] = "rejected"
			found = true
		}

		// Re-marshal token data
		tokenJSON, _ := json.Marshal(tokenData)
		result = append(result, string(tokenJSON))
	}
	return result, found
}
