// Package auth provides API key validation endpoint.
//
// Purpose:
//
//	This package implements API key validation for external services (e.g., API Router Service).
//	Validates API keys by computing fingerprint and looking up key metadata.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router
//   - internal/bootstrap: Runtime dependencies
//   - internal/storage/postgres: User data access
//   - internal/security: Cryptographic utilities
//
// Key Responsibilities:
//   - ValidateAPIKey: POST /v1/auth/validate-api-key - Validate API key secret
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-004 (API Key Lifecycle)
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// ValidateAPIKeyRequest represents the payload for validating an API key.
type ValidateAPIKeyRequest struct {
	APIKeySecret string `json:"apiKeySecret"`    // The API key secret to validate
	OrgID        string `json:"orgId,omitempty"` // Optional: UUID or slug (helps narrow search)
}

// ValidateAPIKeyResponse represents the response after validating an API key.
type ValidateAPIKeyResponse struct {
	Valid          bool     `json:"valid"`
	APIKeyID       string   `json:"apiKeyId,omitempty"`
	OrganizationID string   `json:"organizationId,omitempty"`
	PrincipalID    string   `json:"principalId,omitempty"`
	PrincipalType  string   `json:"principalType,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	Status         string   `json:"status,omitempty"`
	ExpiresAt      *string  `json:"expiresAt,omitempty"`
	Message        string   `json:"message,omitempty"`
}

// ValidateAPIKey handles POST /v1/auth/validate-api-key.
// Validates an API key by computing its fingerprint and looking up the key in the database.
func (h *Handler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ValidateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	if req.APIKeySecret == "" {
		http.Error(w, "apiKeySecret is required", http.StatusBadRequest)
		return
	}

	// Compute fingerprint from secret (same algorithm as key issuance)
	fingerprintHash := sha256.Sum256([]byte(req.APIKeySecret))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Try to find the key by fingerprint
	// If org_id provided, use it to narrow search; otherwise search across orgs
	var apiKey postgres.APIKey
	var err error

	if req.OrgID != "" {
		// Parse org ID (UUID or slug)
		var orgID uuid.UUID
		if orgID, err = uuid.Parse(req.OrgID); err != nil {
			// Try as slug
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
			if err != nil {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			orgID = org.ID
		}
		apiKey, err = h.runtime.Postgres.GetAPIKeyByFingerprint(ctx, orgID, fingerprint)
	} else {
		// Search across all orgs (less efficient, but supports org-agnostic validation)
		// TODO: Once API Router provides org_id, make this required for security
		apiKey, err = h.runtime.Postgres.GetAPIKeyByFingerprintAnyOrg(ctx, fingerprint)
	}

	if err != nil {
		if err == postgres.ErrNotFound {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ValidateAPIKeyResponse{
				Valid:   false,
				Message: "API key not found",
			})
			return
		}
		http.Error(w, "failed to validate API key", http.StatusInternalServerError)
		return
	}

	// Check if key is revoked
	if apiKey.Status == "revoked" || apiKey.RevokedAt != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ValidateAPIKeyResponse{
			Valid:   false,
			Message: "API key is revoked",
		})
		return
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now().UTC()) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ValidateAPIKeyResponse{
			Valid:   false,
			Message: "API key is expired",
		})
		return
	}

	// Update last_used_at (best-effort, non-blocking)
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.runtime.Postgres.UpdateAPIKeyLastUsed(updateCtx, apiKey.ID, time.Now().UTC())
	}()

	// Build success response
	expiresAtStr := ""
	if apiKey.ExpiresAt != nil {
		expiresAtStr = apiKey.ExpiresAt.Format(time.RFC3339)
	}

	response := ValidateAPIKeyResponse{
		Valid:          true,
		APIKeyID:       apiKey.ID.String(),
		OrganizationID: apiKey.OrgID.String(),
		PrincipalID:    apiKey.PrincipalID.String(),
		PrincipalType:  string(apiKey.PrincipalType),
		Scopes:         apiKey.Scopes,
		Status:         apiKey.Status,
	}
	if expiresAtStr != "" {
		response.ExpiresAt = &expiresAtStr
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
