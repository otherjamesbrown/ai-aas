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
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httputil"
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

	// API key management requires apikey:manage or org:admin scope
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys", handler.IssueAPIKey)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/v1/orgs/{orgId}/users/{userId}/api-keys", handler.IssueUserAPIKey)
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/v1/orgs/{orgId}/api-keys", handler.ListAPIKeys)
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.GetAPIKey)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Patch("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.UpdateAPIKey)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/v1/orgs/{orgId}/api-keys/{apiKeyId}/rotate", handler.RotateAPIKey)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/v1/orgs/{orgId}/api-keys/{apiKeyId}/revoke", handler.RevokeAPIKey)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Delete("/v1/orgs/{orgId}/api-keys/{apiKeyId}", handler.RevokeAPIKey)

	// Register convenience routes for /organizations/me/* (frontend-friendly)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/organizations/me/api-keys", handler.IssueUserAPIKeyForMe)
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/organizations/me/api-keys", handler.ListAPIKeysForMe)
	router.With(middleware.RequireAdminScope(logger, "org:read", "org:admin")).Get("/organizations/me/api-keys/{apiKeyId}", handler.GetAPIKeyForMe)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Patch("/organizations/me/api-keys/{apiKeyId}", handler.UpdateAPIKeyForMe)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/organizations/me/api-keys/{apiKeyId}/rotate", handler.RotateAPIKeyForMe)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/organizations/me/api-keys/{apiKeyId}/revoke", handler.RevokeAPIKeyForMe)
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Delete("/organizations/me/api-keys/{apiKeyId}", handler.RevokeAPIKeyForMe)

	// API key inspection endpoint (debugging/support) - requires admin scope
	router.With(middleware.RequireAdminScope(logger, "apikey:manage", "org:admin")).Post("/v1/api-keys/inspect", handler.InspectAPIKey)
}

// Handler serves API key lifecycle endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// findAPIKey looks up an API key by either UUID or KeyID (e.g., "ak_abc123").
// This centralizes the lookup logic used by multiple handlers.
func (h *Handler) findAPIKey(ctx context.Context, orgID uuid.UUID, keyIdentifier string) (postgres.APIKey, error) {
	apiKeyUUID, err := uuid.Parse(keyIdentifier)
	if err != nil {
		// Not a UUID - assume it's a KeyID
		return h.runtime.Postgres.GetAPIKeyByKeyID(ctx, orgID, keyIdentifier)
	}
	// It's a UUID
	return h.runtime.Postgres.GetAPIKeyByID(ctx, apiKeyUUID)
}

// TokenPrefix is the prefix for all AI-AAS API tokens.
const TokenPrefix = "ai-aas_"

// IssueAPIKeyRequest represents the payload for issuing an API key.
type IssueAPIKeyRequest struct {
	Notes         string         `json:"notes,omitempty"` // Human-readable description (like GitHub token name)
	Scopes        []string       `json:"scopes,omitempty"`
	ExpiresInDays *int           `json:"expiresInDays,omitempty"`
	Annotations   map[string]any `json:"annotations,omitempty"`
}

// IssuedAPIKeyResponse represents an issued API key (token shown once).
type IssuedAPIKeyResponse struct {
	KeyID       string  `json:"keyId"`       // Short, unique identifier for the key
	Token       string  `json:"token"`       // The secret token (ai-aas_xxx...) - shown only once
	Fingerprint string  `json:"fingerprint"` // Hash of token for identification
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
				httputil.WriteNotFound(w, r, "organization", "")
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			httputil.WriteInternalError(w, r)
			return
		}
		orgID = org.ID
	}

	// Parse service account ID
	serviceAccountID, err := uuid.Parse(serviceAccountIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid service account ID")
		return
	}

	// Verify service account exists and belongs to org
	serviceAccount, err := h.runtime.Postgres.GetServiceAccountByID(ctx, serviceAccountID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "service account", "")
			return
		}
		h.logger.Error("failed to get service account", zap.Error(err), zap.String("serviceAccountId", serviceAccountID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify service account belongs to org
	if serviceAccount.OrgID != orgID {
		httputil.WriteNotFound(w, r, "service account", "")
		return
	}

	// Verify service account is active
	if serviceAccount.Status != "active" {
		httputil.WriteBadRequest(w, r, "service account is not active")
		return
	}

	var req IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Generate secure random token (32 bytes = 256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		h.logger.Error("failed to generate token", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Encode token as base64url with ai-aas_ prefix
	tokenRaw := base64.RawURLEncoding.EncodeToString(tokenBytes)
	token := TokenPrefix + tokenRaw

	// Compute fingerprint (SHA-256 hash of full token including prefix)
	fingerprintHash := sha256.Sum256([]byte(token))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Encrypt token via Vault Transit (stub for now)
	encryptedSecret, err := h.encryptSecret(ctx, token)
	if err != nil {
		h.logger.Error("failed to encrypt token", zap.Error(err))
		httputil.WriteInternalError(w, r)
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
		PrincipalType: postgres.PrincipalTypeServiceAccount,
		PrincipalID:   serviceAccountID,
		Notes:         req.Notes,
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
		Annotations:   req.Annotations,
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("serviceAccountId", serviceAccountID.String()))
		httputil.WriteInternalError(w, r)
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

	// Build response (token shown once)
	resp := IssuedAPIKeyResponse{
		KeyID:       apiKey.KeyID,
		Token:       token,
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
				httputil.WriteNotFound(w, r, "organization", "")
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			httputil.WriteInternalError(w, r)
			return
		}
		orgID = org.ID
	}

	// Look up API key by UUID or KeyID
	apiKey, err := h.findAPIKey(ctx, orgID, apiKeyIDParam)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "API key", "")
			return
		}
		h.logger.Error("failed to get API key", zap.Error(err), zap.String("keyIdentifier", apiKeyIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify key belongs to org
	if apiKey.OrgID != orgID {
		httputil.WriteNotFound(w, r, "API key", "")
		return
	}

	// Check if already revoked
	if apiKey.Status == "revoked" || apiKey.RevokedAt != nil {
		httputil.WriteConflict(w, r, "API key already revoked")
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
			httputil.WriteConflict(w, r, "API key was modified concurrently")
			return
		}
		h.logger.Error("failed to revoke API key", zap.Error(err), zap.String("keyIdentifier", apiKeyIDParam))
		httputil.WriteInternalError(w, r)
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
				httputil.WriteNotFound(w, r, "organization", "")
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", orgIDParam))
			httputil.WriteInternalError(w, r)
			return
		}
		orgID = org.ID
	}

	// Parse user ID
	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		httputil.WriteBadRequest(w, r, "invalid user ID")
		return
	}

	// Verify user exists and belongs to org
	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "user", "")
			return
		}
		h.logger.Error("failed to get user", zap.Error(err), zap.String("userId", userID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify user belongs to org
	if user.OrgID != orgID {
		httputil.WriteNotFound(w, r, "user", "")
		return
	}

	// Verify user is active
	if user.Status != "active" {
		httputil.WriteBadRequest(w, r, "user is not active")
		return
	}

	var req IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Generate secure random token (32 bytes = 256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		h.logger.Error("failed to generate token", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Encode token as base64url with ai-aas_ prefix
	tokenRaw := base64.RawURLEncoding.EncodeToString(tokenBytes)
	token := TokenPrefix + tokenRaw

	// Compute fingerprint (SHA-256 hash of full token including prefix)
	fingerprintHash := sha256.Sum256([]byte(token))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Encrypt token via Vault Transit (stub for now)
	encryptedSecret, err := h.encryptSecret(ctx, token)
	if err != nil {
		h.logger.Error("failed to encrypt token", zap.Error(err))
		httputil.WriteInternalError(w, r)
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
		Notes:         req.Notes,
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
		Annotations:   req.Annotations,
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("userId", userID.String()))
		httputil.WriteInternalError(w, r)
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

	// Build response (token shown once)
	resp := IssuedAPIKeyResponse{
		KeyID:       apiKey.KeyID,
		Token:       token,
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
// Supports pagination via ?limit=N&offset=M query parameters.
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
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

	// List API keys for this organization
	apiKeys, err := h.runtime.Postgres.ListAPIKeysForOrg(ctx, orgID, limit, offset)
	if err != nil {
		h.logger.Error("failed to list API keys", zap.Error(err), zap.String("orgId", orgID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Convert to response format (without secrets)
	type APIKeyResponse struct {
		KeyID         string   `json:"keyId"`
		Notes         string   `json:"notes,omitempty"`
		PrincipalType string   `json:"principalType"`
		PrincipalID   string   `json:"principalId"`
		Fingerprint   string   `json:"fingerprint"`
		Status        string   `json:"status"`
		Scopes        []string `json:"scopes"`
		IssuedAt      string   `json:"issuedAt"`
		ExpiresAt     *string  `json:"expiresAt,omitempty"`
		LastUsedAt    *string  `json:"lastUsedAt,omitempty"`
	}

	responses := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		responses[i] = APIKeyResponse{
			KeyID:         key.KeyID,
			Notes:         key.Notes,
			PrincipalType: string(key.PrincipalType),
			PrincipalID:   key.PrincipalID.String(),
			Fingerprint:   key.Fingerprint,
			Status:        key.Status,
			Scopes:        key.Scopes,
			IssuedAt:      key.IssuedAt.Format(time.RFC3339),
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

// GetAPIKey handles GET /v1/orgs/{orgId}/api-keys/{apiKeyId} - Get API key details.
func (h *Handler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement getting API key
	httputil.WriteNotImplemented(w, r, "not implemented")
}

// UpdateAPIKey handles PATCH /v1/orgs/{orgId}/api-keys/{apiKeyId} - Update API key metadata.
func (h *Handler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement updating API key
	httputil.WriteNotImplemented(w, r, "not implemented")
}

// RotateAPIKey handles POST /v1/orgs/{orgId}/api-keys/{apiKeyId}/rotate - Rotate API key.
func (h *Handler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement rotating API key
	httputil.WriteNotImplemented(w, r, "not implemented")
}

// Convenience handlers for /organizations/me/* routes that resolve org/user from auth context

// IssueUserAPIKeyForMe handles POST /organizations/me/api-keys - Create API key for current user.
func (h *Handler) IssueUserAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get org ID and user ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	userID := middleware.GetUserID(ctx)

	if orgID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "organization not found in context")
		return
	}
	if userID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "user not found in context")
		return
	}

	// Verify user exists and belongs to org
	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "user", "")
			return
		}
		h.logger.Error("failed to get user", zap.Error(err), zap.String("userId", userID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify user is active
	if user.Status != "active" {
		httputil.WriteBadRequest(w, r, "user is not active")
		return
	}

	var req IssueAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Generate secure random token (32 bytes = 256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		h.logger.Error("failed to generate token", zap.Error(err))
		httputil.WriteInternalError(w, r)
		return
	}

	// Encode token as base64url with ai-aas_ prefix
	tokenRaw := base64.RawURLEncoding.EncodeToString(tokenBytes)
	token := TokenPrefix + tokenRaw

	// Compute fingerprint (SHA-256 hash of full token including prefix)
	fingerprintHash := sha256.Sum256([]byte(token))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Encrypt token via Vault Transit (stub for now)
	encryptedSecret, err := h.encryptSecret(ctx, token)
	if err != nil {
		h.logger.Error("failed to encrypt token", zap.Error(err))
		httputil.WriteInternalError(w, r)
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
		Notes:         req.Notes,
		Fingerprint:   fingerprint,
		Status:        "active",
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
		Annotations:   req.Annotations,
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("userId", userID.String()))
		httputil.WriteInternalError(w, r)
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

	// Build response (token shown once)
	resp := IssuedAPIKeyResponse{
		KeyID:       apiKey.KeyID,
		Token:       token,
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
		httputil.WriteUnauthorized(w, r, "organization not found in context")
		return
	}
	if userID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "user not found in context")
		return
	}

	// List API keys for this user
	apiKeys, err := h.runtime.Postgres.ListAPIKeysForPrincipal(ctx, orgID, postgres.PrincipalTypeUser, userID)
	if err != nil {
		h.logger.Error("failed to list API keys", zap.Error(err), zap.String("orgId", orgID.String()), zap.String("userId", userID.String()))
		httputil.WriteInternalError(w, r)
		return
	}

	// Convert to response format (without secrets)
	type APIKeyResponse struct {
		KeyID       string   `json:"keyId"`
		Notes       string   `json:"notes,omitempty"`
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
			KeyID:       key.KeyID,
			Notes:       key.Notes,
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
		httputil.WriteUnauthorized(w, r, "organization not found in context")
		return
	}

	// Look up API key by UUID or KeyID
	apiKey, err := h.findAPIKey(ctx, orgID, apiKeyIDParam)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "API key", "")
			return
		}
		h.logger.Error("failed to get API key", zap.Error(err), zap.String("keyIdentifier", apiKeyIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify key belongs to org
	if apiKey.OrgID != orgID {
		httputil.WriteNotFound(w, r, "API key", "")
		return
	}

	// Build response (without secret)
	type APIKeyResponse struct {
		KeyID       string   `json:"keyId"`
		Notes       string   `json:"notes,omitempty"`
		Fingerprint string   `json:"fingerprint"`
		Status      string   `json:"status"`
		Scopes      []string `json:"scopes"`
		IssuedAt    string   `json:"issuedAt"`
		ExpiresAt   *string  `json:"expiresAt,omitempty"`
		LastUsedAt  *string  `json:"lastUsedAt,omitempty"`
	}

	resp := APIKeyResponse{
		KeyID:       apiKey.KeyID,
		Notes:       apiKey.Notes,
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
	httputil.WriteNotImplemented(w, r, "not implemented")
}

// RotateAPIKeyForMe handles POST /organizations/me/api-keys/{apiKeyId}/rotate - Rotate API key for current user.
func (h *Handler) RotateAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement rotating API key
	httputil.WriteNotImplemented(w, r, "not implemented")
}

// RevokeAPIKeyForMe handles POST/DELETE /organizations/me/api-keys/{apiKeyId}/revoke - Revoke API key for current user.
func (h *Handler) RevokeAPIKeyForMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiKeyIDParam := chi.URLParam(r, "apiKeyId")

	// Get org ID from authenticated context
	orgID := middleware.GetOrgID(ctx)
	if orgID == uuid.Nil {
		httputil.WriteUnauthorized(w, r, "organization not found in context")
		return
	}

	// Look up API key by UUID or KeyID
	apiKey, err := h.findAPIKey(ctx, orgID, apiKeyIDParam)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "API key", "")
			return
		}
		h.logger.Error("failed to get API key", zap.Error(err), zap.String("keyIdentifier", apiKeyIDParam))
		httputil.WriteInternalError(w, r)
		return
	}

	// Verify key belongs to org
	if apiKey.OrgID != orgID {
		httputil.WriteNotFound(w, r, "API key", "")
		return
	}

	// Check if already revoked
	if apiKey.Status == "revoked" || apiKey.RevokedAt != nil {
		httputil.WriteConflict(w, r, "API key already revoked")
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
			httputil.WriteConflict(w, r, "API key was modified concurrently")
			return
		}
		h.logger.Error("failed to revoke API key", zap.Error(err), zap.String("keyIdentifier", apiKeyIDParam))
		httputil.WriteInternalError(w, r)
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

// InspectAPIKeyRequest represents the payload for inspecting an API key.
type InspectAPIKeyRequest struct {
	Key string `json:"key"` // The full API key secret (ai-aas_xxx...)
}

// InspectAPIKeyResponse represents the details returned when inspecting an API key.
type InspectAPIKeyResponse struct {
	KeyID         string   `json:"keyId"`
	OrgID         string   `json:"orgId"`
	OrgSlug       string   `json:"orgSlug"`
	PrincipalType string   `json:"principalType"`
	PrincipalID   string   `json:"principalId"`
	Status        string   `json:"status"`
	Scopes        []string `json:"scopes"`
	CreatedAt     string   `json:"createdAt"`
	ExpiresAt     *string  `json:"expiresAt,omitempty"`
	LastUsedAt    *string  `json:"lastUsedAt,omitempty"`
}

// InspectAPIKey handles POST /v1/api-keys/inspect.
// Accepts an API key secret and returns key details for debugging/support.
// This endpoint requires admin scope (apikey:manage or org:admin).
func (h *Handler) InspectAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = "unknown"
	}

	// Parse request body
	var req InspectAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		httputil.WriteBadRequest(w, r, "invalid request payload")
		return
	}

	// Validate key is provided
	if req.Key == "" {
		httputil.WriteBadRequest(w, r, "key is required")
		return
	}

	// Compute fingerprint from the provided key (same as auth middleware)
	hash := sha256.Sum256([]byte(req.Key))
	fingerprint := base64.URLEncoding.EncodeToString(hash[:])
	fingerprint = strings.TrimRight(fingerprint, "=")

	h.logger.Info("API key inspect request",
		zap.String("request_id", requestID),
		zap.String("fingerprint", fingerprint[:min(20, len(fingerprint))]+"..."))

	// Look up API key by fingerprint (across all organizations)
	apiKey, err := h.runtime.Postgres.GetAPIKeyByFingerprintAnyOrg(ctx, fingerprint)
	if err != nil {
		if err == postgres.ErrNotFound {
			httputil.WriteNotFound(w, r, "API key", "")
			return
		}
		h.logger.Error("failed to get API key by fingerprint", zap.Error(err), zap.String("request_id", requestID))
		httputil.WriteInternalError(w, r)
		return
	}

	// Get organization details to include slug
	org, err := h.runtime.Postgres.GetOrg(ctx, apiKey.OrgID)
	if err != nil {
		if err == postgres.ErrNotFound {
			// API key exists but org doesn't - data inconsistency
			h.logger.Error("API key references non-existent org",
				zap.String("api_key_id", apiKey.ID.String()),
				zap.String("org_id", apiKey.OrgID.String()),
				zap.String("request_id", requestID))
			httputil.WriteInternalError(w, r)
			return
		}
		h.logger.Error("failed to get organization", zap.Error(err), zap.String("request_id", requestID))
		httputil.WriteInternalError(w, r)
		return
	}

	// Emit audit event (log all inspect requests for security)
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(apiKey.OrgID, actorID, audit.ActorTypeUser, audit.ActionAPIKeyIssue, audit.TargetTypeAPIKey, &apiKey.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"action":      "inspect",
		"fingerprint": fingerprint,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Build response (do NOT include the secret)
	resp := InspectAPIKeyResponse{
		KeyID:         apiKey.KeyID,
		OrgID:         apiKey.OrgID.String(),
		OrgSlug:       org.Slug,
		PrincipalType: string(apiKey.PrincipalType),
		PrincipalID:   apiKey.PrincipalID.String(),
		Status:        apiKey.Status,
		Scopes:        apiKey.Scopes,
		CreatedAt:     apiKey.CreatedAt.Format(time.RFC3339),
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
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}
