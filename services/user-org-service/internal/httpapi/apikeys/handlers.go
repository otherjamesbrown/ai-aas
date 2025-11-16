// Package apikeys provides HTTP handlers for API key lifecycle management.
//
// Purpose:
//
//	This package implements REST API handlers for API key operations:
//	issuing keys for service accounts, revoking keys, and managing key metadata.
//	Handlers enforce authorization, validate input, emit audit events, and
//	integrate with Vault Transit for secret encryption.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store, config, Redis)
//   - internal/storage/postgres: Data access layer
//   - internal/security: Cryptographic utilities for fingerprint generation
//
// Key Responsibilities:
//   - IssueAPIKey: POST /v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys - Issue new key
//   - RevokeAPIKey: DELETE /v1/orgs/{orgId}/api-keys/{apiKeyId} - Revoke a key
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-004 (API Key Lifecycle)
//   - specs/005-user-org-service/contracts/user-org-service.openapi.yaml
//
// Debugging Notes:
//   - API keys are displayed once on creation (secret never stored in DB)
//   - Fingerprints are SHA-256 hashes of the secret (for identification)
//   - Vault Transit encrypts secret material (stub implementation for now)
//   - Revocation propagates to Redis for fast revocation checks
//   - Optimistic locking prevents concurrent revocation conflicts
//
// Thread Safety:
//   - Handler methods are safe for concurrent use (stateless, uses runtime dependencies)
//
// Error Handling:
//   - Invalid UUID returns 400 Bad Request
//   - Not found returns 404 Not Found
//   - Optimistic lock conflicts return 409 Conflict
//   - Database errors return 500 Internal Server Error
package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/metrics"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts API key routes beneath /v1/orgs/{orgId}.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}

	router.Post("/v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys", handler.IssueAPIKey)
	router.Post("/v1/orgs/{orgId}/users/{userId}/api-keys", handler.IssueUserAPIKey)
	router.Get("/v1/orgs/{orgId}/api-keys", handler.ListAPIKeys)
	router.Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.Patch("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.UpdateAPIKey)
	router.Post("/v1/orgs/{orgId}/api-keys/{apiKeyId}/rotate", handler.RotateAPIKey)
	router.Post("/v1/orgs/{orgId}/api-keys/{apiKeyId}/revoke", handler.RevokeAPIKey)
	router.Delete("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.RevokeAPIKey)

	// Register convenience routes for /organizations/me/* (frontend-friendly)
	router.Post("/organizations/me/api-keys", handler.IssueUserAPIKeyForMe)
	router.Get("/organizations/me/api-keys", handler.ListAPIKeysForMe)
	router.Get("/organizations/me/api-keys/{apiKeyId}", handler.GetAPIKeyForMe)
	router.Patch("/organizations/me/api-keys/{apiKeyId}", handler.UpdateAPIKeyForMe)
	router.Post("/organizations/me/api-keys/{apiKeyId}/rotate", handler.RotateAPIKeyForMe)
	router.Post("/organizations/me/api-keys/{apiKeyId}/revoke", handler.RevokeAPIKeyForMe)
	router.Delete("/organizations/me/api-keys/{apiKeyId}", handler.RevokeAPIKeyForMe)
}

// Handler serves API key lifecycle endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// IssueAPIKeyRequest represents the payload for issuing an API key.
type IssueAPIKeyRequest struct {
	DisplayName   string         `json:"display_name,omitempty"`
	Scopes        []string       `json:"scopes,omitempty"`
	ExpiresInDays *int           `json:"expiresInDays,omitempty"`
	Annotations   map[string]any `json:"annotations,omitempty"`
}

// IssuedAPIKeyResponse represents an issued API key (secret shown once).
type IssuedAPIKeyResponse struct {
	APIKeyID    string  `json:"apiKeyId"`
	Secret      string  `json:"secret"`
	Fingerprint string  `json:"fingerprint"`
	Status      string  `json:"status"`
	ExpiresAt   *string `json:"expiresAt,omitempty"`
}

// IssueAPIKey handles POST /v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys.
// Generates a secure random secret, computes fingerprint, stores metadata in DB,
// encrypts secret via Vault Transit, and returns the secret once.
func (h *Handler) IssueAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	serviceAccountIDParam := chi.URLParam(r, "serviceAccountId")

	// Parse org ID (UUID or slug)
	var orgID uuid.UUID
	var err error
	if orgID, err = uuid.Parse(orgIDParam); err != nil {
		// Try as slug
		org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
		if err != nil {
			if err == postgres.ErrNotFound {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			http.Error(w, "failed to resolve organization", http.StatusInternalServerError)
			return
		}
		orgID = org.ID
	}

	// Parse service account ID
	serviceAccountID, err := uuid.Parse(serviceAccountIDParam)
	if err != nil {
		http.Error(w, "invalid service account ID", http.StatusBadRequest)
		return
	}

	// Verify service account exists and belongs to org
	serviceAccount, err := h.runtime.Postgres.GetServiceAccountByID(ctx, serviceAccountID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "service account not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get service account", zap.Error(err), zap.String("serviceAccountId", serviceAccountID.String()))
		http.Error(w, "failed to retrieve service account", http.StatusInternalServerError)
		return
	}

	// Verify service account belongs to org
	if serviceAccount.OrgID != orgID {
		http.Error(w, "service account not found", http.StatusNotFound)
		return
	}

	// Verify service account is active
	if serviceAccount.Status != "active" {
		http.Error(w, "service account is not active", http.StatusBadRequest)
		return
	}

	var req IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Generate secure random secret (32 bytes = 256 bits)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		h.logger.Error("failed to generate secret", zap.Error(err))
		http.Error(w, "failed to generate API key", http.StatusInternalServerError)
		return
	}

	// Encode secret as base64url (URL-safe, no padding)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	// Compute fingerprint (SHA-256 hash of secret)
	fingerprintHash := sha256.Sum256([]byte(secret))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Encrypt secret via Vault Transit (stub for now)
	encryptedSecret, err := h.encryptSecret(ctx, secret)
	if err != nil {
		h.logger.Error("failed to encrypt secret", zap.Error(err))
		http.Error(w, "failed to encrypt API key", http.StatusInternalServerError)
		return
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		exp := time.Now().UTC().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &exp
	}

	// Prepare annotations (include display_name if provided)
	annotations := req.Annotations
	if annotations == nil {
		annotations = make(map[string]any)
	}
	if req.DisplayName != "" {
		annotations["display_name"] = req.DisplayName
	}

	// Create API key record in database
	params := postgres.CreateAPIKeyParams{
		OrgID:         orgID,
		PrincipalType: postgres.PrincipalTypeServiceAccount,
		PrincipalID:   serviceAccountID,
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
		Annotations:   annotations,
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("serviceAccountId", serviceAccountID.String()))
		http.Error(w, "failed to create API key", http.StatusInternalServerError)
		return
	}

	// Store encrypted secret in Vault (async, best-effort)
	// TODO: Store encryptedSecret in Vault Transit with key ID = apiKey.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.storeEncryptedSecret(ctx, apiKey.ID, encryptedSecret)
	}()

	// Emit audit event
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionAPIKeyIssue, audit.TargetTypeAPIKey, &apiKey.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"principal_type": "service_account",
		"principal_id":   serviceAccountID.String(),
		"fingerprint":    fingerprint,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Record API key issuance
	metrics.RecordAPIKeyIssued()

	// Build response (secret shown once)
	resp := IssuedAPIKeyResponse{
		APIKeyID:    apiKey.ID.String(),
		Secret:      secret, // Only time secret is returned
		Fingerprint: fingerprint,
		Status:      apiKey.Status,
	}
	if apiKey.ExpiresAt != nil {
		expStr := apiKey.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expStr
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// RevokeAPIKey handles DELETE /v1/orgs/{orgId}/api-keys/{apiKeyId}.
// Marks the key as revoked in the database and propagates revocation to Redis.
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	apiKeyIDParam := chi.URLParam(r, "apiKeyId")

	// Parse org ID (UUID or slug)
	var orgID uuid.UUID
	var err error
	if orgID, err = uuid.Parse(orgIDParam); err != nil {
		// Try as slug
		org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
		if err != nil {
			if err == postgres.ErrNotFound {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			http.Error(w, "failed to resolve organization", http.StatusInternalServerError)
			return
		}
		orgID = org.ID
	}

	// Parse API key ID
	apiKeyID, err := uuid.Parse(apiKeyIDParam)
	if err != nil {
		http.Error(w, "invalid API key ID", http.StatusBadRequest)
		return
	}

	// Get existing key to obtain version for optimistic locking
	apiKey, err := h.runtime.Postgres.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "API key not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get API key", zap.Error(err), zap.String("apiKeyId", apiKeyID.String()))
		http.Error(w, "failed to retrieve API key", http.StatusInternalServerError)
		return
	}

	// Verify key belongs to org
	if apiKey.OrgID != orgID {
		http.Error(w, "API key not found", http.StatusNotFound)
		return
	}

	// Check if already revoked
	if apiKey.Status == "revoked" || apiKey.RevokedAt != nil {
		http.Error(w, "API key already revoked", http.StatusConflict)
		return
	}

	// Revoke key in database
	revokedAt := time.Now().UTC()
	_, err = h.runtime.Postgres.RevokeAPIKey(ctx, postgres.RevokeAPIKeyParams{
		ID:        apiKey.ID,
		Version:   apiKey.Version,
		Status:    "revoked",
		RevokedAt: revokedAt,
	}, orgID)
	if err != nil {
		if err == postgres.ErrOptimisticLock {
			http.Error(w, "API key was modified concurrently", http.StatusConflict)
			return
		}
		h.logger.Error("failed to revoke API key", zap.Error(err), zap.String("apiKeyId", apiKeyID.String()))
		http.Error(w, "failed to revoke API key", http.StatusInternalServerError)
		return
	}

	// Propagate revocation to Redis for fast revocation checks
	if h.runtime.Redis != nil {
		revocationKey := fmt.Sprintf("api_key:revoked:%s", apiKey.Fingerprint)
		// Store with TTL matching key expiration (or 1 year if no expiration)
		ttl := 365 * 24 * time.Hour
		if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.After(time.Now()) {
			ttl = time.Until(*apiKey.ExpiresAt)
		}
		if err := h.runtime.Redis.Set(ctx, revocationKey, "1", ttl).Err(); err != nil {
			h.logger.Warn("failed to propagate revocation to Redis", zap.Error(err), zap.String("fingerprint", apiKey.Fingerprint))
			// Non-fatal: continue even if Redis propagation fails
		}
	}

	// Emit audit event
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionAPIKeyRevoke, audit.TargetTypeAPIKey, &apiKey.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"fingerprint": apiKey.Fingerprint,
		"revoked_at":  revokedAt.Format(time.RFC3339),
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Record API key revocation
	metrics.RecordAPIKeyRevoked()

	w.WriteHeader(http.StatusNoContent)
}

// encryptSecret encrypts the secret using Vault Transit (stub implementation).
// TODO: Integrate with Hashicorp Vault Transit engine for production.
func (h *Handler) encryptSecret(ctx context.Context, secret string) (string, error) {
	// Stub: return base64-encoded secret (in production, use Vault Transit)
	// This allows the code to compile and run, but secrets are not properly encrypted
	h.logger.Warn("using stub Vault Transit encryption - secrets are not encrypted")
	return base64.StdEncoding.EncodeToString([]byte(secret)), nil
}

// storeEncryptedSecret stores the encrypted secret in Vault (stub implementation).
// TODO: Store encrypted secret in Vault Transit with proper key management.
func (h *Handler) storeEncryptedSecret(ctx context.Context, keyID uuid.UUID, encryptedSecret string) error {
	// Stub: log the operation (in production, store in Vault Transit)
	h.logger.Debug("stub: storing encrypted secret in Vault", zap.String("keyId", keyID.String()))
	return nil
}

// IssueUserAPIKey handles POST /v1/orgs/{orgId}/users/{userId}/api-keys.
// Similar to IssueAPIKey but for user principals instead of service accounts.
func (h *Handler) IssueUserAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	// Parse org ID (UUID or slug)
	var orgID uuid.UUID
	var err error
	if orgID, err = uuid.Parse(orgIDParam); err != nil {
		// Try as slug
		org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
		if err != nil {
			if err == postgres.ErrNotFound {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			http.Error(w, "failed to resolve organization", http.StatusInternalServerError)
			return
		}
		orgID = org.ID
	}

	// Parse user ID
	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Verify user exists and belongs to org
	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get user", zap.Error(err), zap.String("userId", userID.String()))
		http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
		return
	}

	// Verify user belongs to org
	if user.OrgID != orgID {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Verify user is active
	if user.Status != "active" {
		http.Error(w, "user is not active", http.StatusBadRequest)
		return
	}

	var req IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Generate secure random secret (32 bytes = 256 bits)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		h.logger.Error("failed to generate secret", zap.Error(err))
		http.Error(w, "failed to generate API key", http.StatusInternalServerError)
		return
	}

	// Encode secret as base64url (URL-safe, no padding)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	// Compute fingerprint (SHA-256 hash of secret)
	fingerprintHash := sha256.Sum256([]byte(secret))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Encrypt secret via Vault Transit (stub for now)
	encryptedSecret, err := h.encryptSecret(ctx, secret)
	if err != nil {
		h.logger.Error("failed to encrypt secret", zap.Error(err))
		http.Error(w, "failed to encrypt API key", http.StatusInternalServerError)
		return
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		exp := time.Now().UTC().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &exp
	}

	// Create API key record in database
	params := postgres.CreateAPIKeyParams{
		OrgID:         orgID,
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   userID,
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
		Annotations:   req.Annotations,
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("userId", userID.String()))
		http.Error(w, "failed to create API key", http.StatusInternalServerError)
		return
	}

	// Store encrypted secret in Vault (async, best-effort)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.storeEncryptedSecret(ctx, apiKey.ID, encryptedSecret)
	}()

	// Emit audit event
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionAPIKeyIssue, audit.TargetTypeAPIKey, &apiKey.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"principal_type": "user",
		"principal_id":   userID.String(),
		"fingerprint":    fingerprint,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Record API key issuance
	metrics.RecordAPIKeyIssued()

	// Build response (secret shown once)
	resp := IssuedAPIKeyResponse{
		APIKeyID:    apiKey.ID.String(),
		Secret:      secret, // Only time secret is returned
		Fingerprint: fingerprint,
		Status:      apiKey.Status,
	}
	if apiKey.ExpiresAt != nil {
		expStr := apiKey.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expStr
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// ListAPIKeys handles GET /v1/orgs/{orgId}/api-keys - List all API keys for the organization.
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement listing API keys
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// GetAPIKey handles GET /v1/orgs/{orgId}/api-keys/{apiKeyId} - Get API key details.
func (h *Handler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement getting API key
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// UpdateAPIKey handles PATCH /v1/orgs/{orgId}/api-keys/{apiKeyId} - Update API key metadata.
func (h *Handler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement updating API key
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// RotateAPIKey handles POST /v1/orgs/{orgId}/api-keys/{apiKeyId}/rotate - Rotate API key.
func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement rotating API key
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Convenience handlers for /organizations/me/* routes that resolve org/user from auth context

// IssueUserAPIKeyForMe handles POST /organizations/me/api-keys - Create API key for current user.
func (h *Handler) IssueUserAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get org ID and user ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	userID := middleware.GetUserID(ctx)
	
	if orgID == uuid.Nil {
		http.Error(w, "organization not found in context", http.StatusUnauthorized)
		return
	}
	if userID == uuid.Nil {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}
	
	// Verify user exists and belongs to org
	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get user", zap.Error(err), zap.String("userId", userID.String()))
		http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
		return
	}

	// Verify user is active
	if user.Status != "active" {
		http.Error(w, "user is not active", http.StatusBadRequest)
		return
	}

	var req IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Generate secure random secret (32 bytes = 256 bits)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		h.logger.Error("failed to generate secret", zap.Error(err))
		http.Error(w, "failed to generate API key", http.StatusInternalServerError)
		return
	}

	// Encode secret as base64url (URL-safe, no padding)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	// Compute fingerprint (SHA-256 hash of secret)
	fingerprintHash := sha256.Sum256([]byte(secret))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Encrypt secret via Vault Transit (stub for now)
	encryptedSecret, err := h.encryptSecret(ctx, secret)
	if err != nil {
		h.logger.Error("failed to encrypt secret", zap.Error(err))
		http.Error(w, "failed to encrypt API key", http.StatusInternalServerError)
		return
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		exp := time.Now().UTC().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &exp
	}

	// Prepare annotations (include display_name if provided)
	annotations := req.Annotations
	if annotations == nil {
		annotations = make(map[string]any)
	}
	if req.DisplayName != "" {
		annotations["display_name"] = req.DisplayName
	}

	// Create API key record in database
	params := postgres.CreateAPIKeyParams{
		OrgID:         orgID,
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   userID,
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
		Annotations:   annotations,
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("userId", userID.String()))
		http.Error(w, "failed to create API key", http.StatusInternalServerError)
		return
	}

	// Store encrypted secret in Vault (async, best-effort)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.storeEncryptedSecret(ctx, apiKey.ID, encryptedSecret)
	}()

	// Emit audit event
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionAPIKeyIssue, audit.TargetTypeAPIKey, &apiKey.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"principal_type": "user",
		"principal_id":   userID.String(),
		"fingerprint":    fingerprint,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Record API key issuance
	metrics.RecordAPIKeyIssued()

	// Build response (secret shown once)
	resp := IssuedAPIKeyResponse{
		APIKeyID:    apiKey.ID.String(),
		Secret:      secret, // Only time secret is returned
		Fingerprint: fingerprint,
		Status:      apiKey.Status,
	}
	if apiKey.ExpiresAt != nil {
		expStr := apiKey.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expStr
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// ListAPIKeysForMe handles GET /organizations/me/api-keys - List API keys for current user.
func (h *Handler) ListAPIKeysForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get org ID and user ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	userID := middleware.GetUserID(ctx)
	
	if orgID == uuid.Nil {
		http.Error(w, "organization not found in context", http.StatusUnauthorized)
		return
	}
	if userID == uuid.Nil {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}
	
	// List API keys for this user
	apiKeys, err := h.runtime.Postgres.ListAPIKeysForPrincipal(ctx, orgID, postgres.PrincipalTypeUser, userID)
	if err != nil {
		h.logger.Error("failed to list API keys", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("userId", userID.String()))
		http.Error(w, "failed to list API keys", http.StatusInternalServerError)
		return
	}
	
	// Convert to response format (without secrets)
	type APIKeyResponse struct {
		APIKeyID    string   `json:"apiKeyId"`
		Fingerprint string   `json:"fingerprint"`
		Status      string   `json:"status"`
		Scopes      []string `json:"scopes"`
		IssuedAt    string   `json:"issuedAt"`
		ExpiresAt   *string  `json:"expiresAt,omitempty"`
		LastUsedAt  *string  `json:"lastUsedAt,omitempty"`
	}
	
	responses := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		responses[i] = APIKeyResponse{
			APIKeyID:    key.ID.String(),
			Fingerprint: key.Fingerprint,
			Status:      key.Status,
			Scopes:      key.Scopes,
			IssuedAt:    key.IssuedAt.Format(time.RFC3339),
		}
		if key.ExpiresAt != nil {
			expStr := key.ExpiresAt.Format(time.RFC3339)
			responses[i].ExpiresAt = &expStr
		}
		if key.LastUsedAt != nil {
			usedStr := key.LastUsedAt.Format(time.RFC3339)
			responses[i].LastUsedAt = &usedStr
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// GetAPIKeyForMe handles GET /organizations/me/api-keys/{apiKeyId} - Get API key for current user.
func (h *Handler) GetAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiKeyIDParam := chi.URLParam(r, "apiKeyId")
	
	// Get org ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	if orgID == uuid.Nil {
		http.Error(w, "organization not found in context", http.StatusUnauthorized)
		return
	}
	
	// Parse API key ID
	apiKeyID, err := uuid.Parse(apiKeyIDParam)
	if err != nil {
		http.Error(w, "invalid API key ID", http.StatusBadRequest)
		return
	}
	
	// Get API key
	apiKey, err := h.runtime.Postgres.GetAPIKeyByID(ctx, apiKeyID)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "API key not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get API key", zap.Error(err), zap.String("apiKeyId", apiKeyID.String()))
		http.Error(w, "failed to retrieve API key", http.StatusInternalServerError)
		return
	}
	
	// Verify key belongs to org
	if apiKey.OrgID != orgID {
		http.Error(w, "API key not found", http.StatusNotFound)
		return
	}
	
	// Build response (without secret)
	type APIKeyResponse struct {
		APIKeyID    string   `json:"apiKeyId"`
		Fingerprint string   `json:"fingerprint"`
		Status      string   `json:"status"`
		Scopes      []string `json:"scopes"`
		IssuedAt    string   `json:"issuedAt"`
		ExpiresAt   *string  `json:"expiresAt,omitempty"`
		LastUsedAt  *string  `json:"lastUsedAt,omitempty"`
	}
	
	resp := APIKeyResponse{
		APIKeyID:    apiKey.ID.String(),
		Fingerprint: apiKey.Fingerprint,
		Status:      apiKey.Status,
		Scopes:      apiKey.Scopes,
		IssuedAt:    apiKey.IssuedAt.Format(time.RFC3339),
	}
	if apiKey.ExpiresAt != nil {
		expStr := apiKey.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expStr
	}
	if apiKey.LastUsedAt != nil {
		usedStr := apiKey.LastUsedAt.Format(time.RFC3339)
		resp.LastUsedAt = &usedStr
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// UpdateAPIKeyForMe handles PATCH /organizations/me/api-keys/{apiKeyId} - Update API key for current user.
func (h *Handler) UpdateAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement updating API key
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// RotateAPIKeyForMe handles POST /organizations/me/api-keys/{apiKeyId}/rotate - Rotate API key for current user.
func (h *Handler) RotateAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement rotating API key
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// RevokeAPIKeyForMe handles POST/DELETE /organizations/me/api-keys/{apiKeyId}/revoke - Revoke API key for current user.
func (h *Handler) RevokeAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiKeyIDParam := chi.URLParam(r, "apiKeyId")
	
	// Get org ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	if orgID == uuid.Nil {
		http.Error(w, "organization not found in context", http.StatusUnauthorized)
		return
	}
	
	// Call the main revoke handler with resolved org ID
	r.URL.Path = fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID.String(), apiKeyIDParam)
	h.RevokeAPIKey(w, r)
}
