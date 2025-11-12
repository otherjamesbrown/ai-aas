// Package auth provides admin endpoints for recovery request approval.
//
// Purpose:
//   This package implements admin endpoints for approving/rejecting recovery requests
//   when RECOVERY_REQUIRES_ADMIN_APPROVAL is enabled.
//
package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
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
	Token string `json:"token"` // Recovery token to reject
	Email string `json:"email"` // User email
	OrgID string `json:"org_id,omitempty"`
	Reason string `json:"reason,omitempty"` // Optional rejection reason
}

// ApproveRecovery handles POST /v1/auth/recover/approve.
// Approves a pending recovery request, allowing the user to reset their password.
func (h *Handler) ApproveRecovery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.runtime.Config.RecoveryRequiresAdminApproval {
		http.Error(w, "admin approval not required", http.StatusBadRequest)
		return
	}

	// Get admin actor ID from context (set by auth middleware)
	actorID := middleware.GetUserID(r.Context())
	if actorID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ApproveRecoveryRequest
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
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Find and approve the recovery token
	updatedTokens, found := h.approveRecoveryToken(user.RecoveryTokens, req.Token, actorID)
	if !found {
		http.Error(w, "recovery token not found or already processed", http.StatusNotFound)
		return
	}

	// Update user with approved token
	_, err = h.runtime.Postgres.UpdateUserRecoveryTokens(ctx, orgID, user.ID, user.Version, updatedTokens)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			http.Error(w, "user was modified concurrently", http.StatusConflict)
			return
		}
		http.Error(w, "failed to update recovery token", http.StatusInternalServerError)
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
		http.Error(w, "admin approval not required", http.StatusBadRequest)
		return
	}

	// Get admin actor ID from context (set by auth middleware)
	actorID := middleware.GetUserID(r.Context())
	if actorID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req RejectRecoveryRequest
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
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Find and reject the recovery token (mark as used to invalidate it)
	updatedTokens, found := h.rejectRecoveryToken(user.RecoveryTokens, req.Token)
	if !found {
		http.Error(w, "recovery token not found or already processed", http.StatusNotFound)
		return
	}

	// Update user with rejected token
	_, err = h.runtime.Postgres.UpdateUserRecoveryTokens(ctx, orgID, user.ID, user.Version, updatedTokens)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			http.Error(w, "user was modified concurrently", http.StatusConflict)
			return
		}
		http.Error(w, "failed to update recovery token", http.StatusInternalServerError)
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

