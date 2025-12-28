// Package bootstrapkeys provides HTTP handlers for bootstrap key lifecycle management.
//
// Purpose:
//
//	Bootstrap keys are one-time use tokens that allow organization administrators
//	to initialize their ai-aas-org CLI. Keys have a 7-day expiry by default.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store, config, Redis)
//   - internal/storage/postgres: Data access layer
//
// Key Responsibilities:
//   - CreateBootstrapKey: POST /v1/bootstrap-keys - Create new key
//   - ListBootstrapKeys: GET /v1/bootstrap-keys - List keys (with optional org filter)
//   - RevokeBootstrapKey: POST /v1/bootstrap-keys/{keyId}/revoke - Revoke a key
//   - RedeemBootstrapKey: POST /v1/bootstrap-keys/redeem - Redeem key for org access
//
// Requirements Reference:
//   - specs/033-org-admin-cli/spec.md (Org admin CLI onboarding)
package bootstrapkeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// TokenPrefix is the prefix for bootstrap key tokens.
const TokenPrefix = "bsk_"

// APIKeyPrefix is the prefix for API key tokens.
const APIKeyPrefix = "ai-aas_"

// DefaultExpiryDays is the default expiry in days for bootstrap keys.
const DefaultExpiryDays = 7

// RegisterRoutes mounts bootstrap key routes.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}

	// Protected routes (require platform admin auth)
	router.Post("/v1/bootstrap-keys", handler.CreateBootstrapKey)
	router.Get("/v1/bootstrap-keys", handler.ListBootstrapKeys)
	router.Post("/v1/bootstrap-keys/{keyId}/revoke", handler.RevokeBootstrapKey)
}

// RegisterPublicRoutes mounts unauthenticated bootstrap key routes.
func RegisterPublicRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}

	// Public route - redeem doesn't require prior auth (the key IS the auth)
	router.Post("/v1/bootstrap-keys/redeem", handler.RedeemBootstrapKey)
}

// Handler serves bootstrap key lifecycle endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// CreateBootstrapKeyRequest represents the payload for creating a bootstrap key.
type CreateBootstrapKeyRequest struct {
	OrgID         string `json:"orgId"`
	ExpiresInDays int    `json:"expiresInDays,omitempty"` // Default 7 days
	Notes         string `json:"notes,omitempty"`
}

// BootstrapKeyCreatedResponse represents a newly created bootstrap key.
type BootstrapKeyCreatedResponse struct {
	KeyID     string `json:"keyId"`
	Token     string `json:"token"` // bsk_xxxx - shown only once
	OrgID     string `json:"orgId"`
	OrgName   string `json:"orgName,omitempty"`
	ExpiresAt string `json:"expiresAt"`
}

// BootstrapKeyResponse represents a bootstrap key in list responses.
type BootstrapKeyResponse struct {
	KeyID      string  `json:"keyId"`
	OrgID      string  `json:"orgId"`
	OrgName    string  `json:"orgName,omitempty"`
	Status     string  `json:"status"` // active, revoked, expired, redeemed
	Notes      string  `json:"notes,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  string  `json:"expiresAt"`
	RedeemedAt *string `json:"redeemedAt,omitempty"`
	RedeemedBy *string `json:"redeemedBy,omitempty"`
}

// CreateBootstrapKey handles POST /v1/bootstrap-keys.
func (h *Handler) CreateBootstrapKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateBootstrapKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate org ID
	if req.OrgID == "" {
		http.Error(w, "orgId is required", http.StatusBadRequest)
		return
	}

	// Parse org ID (UUID or slug)
	var orgID uuid.UUID
	var orgName string
	var err error
	if orgID, err = uuid.Parse(req.OrgID); err != nil {
		// Try as slug
		org, err := h.runtime.Postgres.GetOrgBySlug(ctx, req.OrgID)
		if err != nil {
			if err == postgres.ErrNotFound {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			h.logger.Error("failed to resolve organization", zap.Error(err), zap.String("orgId", req.OrgID))
			http.Error(w, "failed to resolve organization", http.StatusInternalServerError)
			return
		}
		orgID = org.ID
		orgName = org.Name
	} else {
		// Fetch org to get name
		org, err := h.runtime.Postgres.GetOrg(ctx, orgID)
		if err != nil {
			if err == postgres.ErrNotFound {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			h.logger.Error("failed to get organization", zap.Error(err), zap.String("orgId", orgID.String()))
			http.Error(w, "failed to get organization", http.StatusInternalServerError)
			return
		}
		orgName = org.Name
	}

	// Set default expiry
	expiresInDays := req.ExpiresInDays
	if expiresInDays <= 0 {
		expiresInDays = DefaultExpiryDays
	}

	// Generate secure random token (32 bytes = 256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		h.logger.Error("failed to generate token", zap.Error(err))
		http.Error(w, "failed to generate bootstrap key", http.StatusInternalServerError)
		return
	}

	// Encode token as base64url with bsk_ prefix
	tokenRaw := base64.RawURLEncoding.EncodeToString(tokenBytes)
	token := TokenPrefix + tokenRaw

	// Compute fingerprint (SHA-256 hash of full token including prefix)
	fingerprintHash := sha256.Sum256([]byte(token))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Calculate expiration
	expiresAt := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)

	// Create bootstrap key record in database
	params := postgres.CreateBootstrapKeyParams{
		OrgID:       orgID,
		Fingerprint: fingerprint,
		Notes:       req.Notes,
		ExpiresAt:   expiresAt,
	}

	key, err := h.runtime.Postgres.CreateBootstrapKey(ctx, params)
	if err != nil {
		h.logger.Error("failed to create bootstrap key", zap.Error(err), zap.String("orgId", orgID.String()))
		http.Error(w, "failed to create bootstrap key", http.StatusInternalServerError)
		return
	}

	// Emit audit event
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(orgID, actorID, audit.ActorTypeUser, audit.ActionBootstrapKeyCreate, audit.TargetTypeBootstrapKey, &key.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"fingerprint": fingerprint,
		"expires_at":  expiresAt.Format(time.RFC3339),
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Build response (token shown once)
	resp := BootstrapKeyCreatedResponse{
		KeyID:     key.KeyID,
		Token:     token,
		OrgID:     orgID.String(),
		OrgName:   orgName,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// ListBootstrapKeys handles GET /v1/bootstrap-keys.
func (h *Handler) ListBootstrapKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Optional org filter
	orgIDParam := r.URL.Query().Get("orgId")
	var orgFilter *uuid.UUID

	if orgIDParam != "" {
		orgID, err := uuid.Parse(orgIDParam)
		if err != nil {
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
			orgFilter = &org.ID
		} else {
			orgFilter = &orgID
		}
	}

	// List bootstrap keys
	keys, err := h.runtime.Postgres.ListBootstrapKeys(ctx, orgFilter)
	if err != nil {
		h.logger.Error("failed to list bootstrap keys", zap.Error(err))
		http.Error(w, "failed to list bootstrap keys", http.StatusInternalServerError)
		return
	}

	// Build response
	type ListResponse struct {
		Keys []BootstrapKeyResponse `json:"keys"`
	}

	responses := make([]BootstrapKeyResponse, len(keys))
	for i, key := range keys {
		status := key.Status
		if status == "active" && time.Now().After(key.ExpiresAt) {
			status = "expired"
		}

		responses[i] = BootstrapKeyResponse{
			KeyID:     key.KeyID,
			OrgID:     key.OrgID.String(),
			OrgName:   key.OrgName,
			Status:    status,
			Notes:     key.Notes,
			CreatedAt: key.CreatedAt.Format(time.RFC3339),
			ExpiresAt: key.ExpiresAt.Format(time.RFC3339),
		}
		if key.RedeemedAt != nil {
			redeemedAt := key.RedeemedAt.Format(time.RFC3339)
			responses[i].RedeemedAt = &redeemedAt
		}
		if key.RedeemedBy != "" {
			responses[i].RedeemedBy = &key.RedeemedBy
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ListResponse{Keys: responses}); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// RevokeBootstrapKey handles POST /v1/bootstrap-keys/{keyId}/revoke.
func (h *Handler) RevokeBootstrapKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	keyIDParam := chi.URLParam(r, "keyId")

	// Get key by key ID
	key, err := h.runtime.Postgres.GetBootstrapKeyByKeyID(ctx, keyIDParam)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "bootstrap key not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get bootstrap key", zap.Error(err), zap.String("keyId", keyIDParam))
		http.Error(w, "failed to get bootstrap key", http.StatusInternalServerError)
		return
	}

	// Check if already revoked or redeemed
	if key.Status != "active" {
		http.Error(w, "bootstrap key cannot be revoked (already "+key.Status+")", http.StatusConflict)
		return
	}

	// Revoke the key
	if err := h.runtime.Postgres.RevokeBootstrapKey(ctx, key.ID); err != nil {
		h.logger.Error("failed to revoke bootstrap key", zap.Error(err), zap.String("keyId", keyIDParam))
		http.Error(w, "failed to revoke bootstrap key", http.StatusInternalServerError)
		return
	}

	// Emit audit event
	actorID := middleware.GetUserID(r.Context())
	event := audit.BuildEvent(key.OrgID, actorID, audit.ActorTypeUser, audit.ActionBootstrapKeyRevoke, audit.TargetTypeBootstrapKey, &key.ID)
	event = audit.BuildEventFromRequest(event, r)
	_ = h.runtime.Audit.Emit(ctx, event)

	w.WriteHeader(http.StatusNoContent)
}

// RedeemBootstrapKeyRequest represents the payload for redeeming a bootstrap key.
type RedeemBootstrapKeyRequest struct {
	Token string `json:"token"` // bsk_xxxx
}

// RedeemBootstrapKeyResponse represents the response from redeeming a bootstrap key.
type RedeemBootstrapKeyResponse struct {
	OrgID       string `json:"orgId"`
	OrgName     string `json:"orgName"`
	APIEndpoint string `json:"apiEndpoint"`
	APIKey      string `json:"apiKey"`
}

// RedeemBootstrapKey handles POST /v1/bootstrap-keys/redeem.
// This is called by ai-aas-org init to exchange a bootstrap key for org access.
func (h *Handler) RedeemBootstrapKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req RedeemBootstrapKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("invalid request payload", zap.Error(err))
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	// Validate token format
	if len(req.Token) < len(TokenPrefix) || req.Token[:len(TokenPrefix)] != TokenPrefix {
		http.Error(w, "invalid bootstrap key format", http.StatusBadRequest)
		return
	}

	// Compute fingerprint from token
	fingerprintHash := sha256.Sum256([]byte(req.Token))
	fingerprint := base64.RawURLEncoding.EncodeToString(fingerprintHash[:])

	// Look up key by fingerprint
	key, err := h.runtime.Postgres.GetBootstrapKeyByFingerprint(ctx, fingerprint)
	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "invalid or expired bootstrap key", http.StatusUnauthorized)
			return
		}
		h.logger.Error("failed to get bootstrap key", zap.Error(err))
		http.Error(w, "failed to validate bootstrap key", http.StatusInternalServerError)
		return
	}

	// Check if key is valid
	if key.Status != "active" {
		http.Error(w, "bootstrap key is no longer valid ("+key.Status+")", http.StatusUnauthorized)
		return
	}
	if time.Now().After(key.ExpiresAt) {
		http.Error(w, "bootstrap key has expired", http.StatusUnauthorized)
		return
	}

	// Get org details
	org, err := h.runtime.Postgres.GetOrg(ctx, key.OrgID)
	if err != nil {
		h.logger.Error("failed to get organization", zap.Error(err), zap.String("orgId", key.OrgID.String()))
		http.Error(w, "failed to get organization", http.StatusInternalServerError)
		return
	}

	// Generate API key token for org admin
	tokenBytes := make([]byte, 32) // 256 bits of entropy
	if _, err := rand.Read(tokenBytes); err != nil {
		h.logger.Error("failed to generate API key token", zap.Error(err))
		http.Error(w, "failed to generate API key", http.StatusInternalServerError)
		return
	}

	// Encode token as base64url with ai-aas_ prefix
	tokenRaw := base64.RawURLEncoding.EncodeToString(tokenBytes)
	apiKeyToken := APIKeyPrefix + tokenRaw

	// Compute fingerprint (SHA-256 hash of full token including prefix)
	apiKeyFingerprintHash := sha256.Sum256([]byte(apiKeyToken))
	apiKeyFingerprint := base64.RawURLEncoding.EncodeToString(apiKeyFingerprintHash[:])

	// Create API key record for org admin (90-day expiry)
	apiKeyExpiry := time.Now().UTC().Add(90 * 24 * time.Hour)
	apiKeyParams := postgres.CreateAPIKeyParams{
		OrgID:         key.OrgID,
		PrincipalType: postgres.PrincipalTypeUser,
		PrincipalID:   uuid.Nil, // System-generated key (no specific user)
		Notes:         "Org admin key (generated via bootstrap)",
		Fingerprint:   apiKeyFingerprint,
		Status:        "active",
		Scopes:        []string{"org:admin", "org:read", "org:write", "user:manage", "apikey:manage"},
		ExpiresAt:     &apiKeyExpiry,
		Annotations:   map[string]any{"bootstrap_key_id": key.KeyID},
	}

	apiKey, err := h.runtime.Postgres.CreateAPIKey(ctx, apiKeyParams)
	if err != nil {
		h.logger.Error("failed to create API key", zap.Error(err), zap.String("orgId", key.OrgID.String()))
		http.Error(w, "failed to create API key", http.StatusInternalServerError)
		return
	}

	// Mark bootstrap key as redeemed (after API key created successfully)
	if err := h.runtime.Postgres.RedeemBootstrapKey(ctx, key.ID, apiKey.KeyID); err != nil {
		h.logger.Error("failed to redeem bootstrap key", zap.Error(err))
		// API key was created but we couldn't mark bootstrap key as redeemed
		// This is a partial failure state - log and continue
	}

	// Emit audit event for bootstrap key redemption
	event := audit.BuildEvent(key.OrgID, uuid.Nil, audit.ActorTypeSystem, audit.ActionBootstrapKeyRedeem, audit.TargetTypeBootstrapKey, &key.ID)
	event = audit.BuildEventFromRequest(event, r)
	event.Metadata = map[string]any{
		"api_key_id": apiKey.KeyID,
	}
	_ = h.runtime.Audit.Emit(ctx, event)

	// Emit audit event for API key creation
	apiKeyEvent := audit.BuildEvent(key.OrgID, uuid.Nil, audit.ActorTypeSystem, audit.ActionAPIKeyIssue, audit.TargetTypeAPIKey, &apiKey.ID)
	apiKeyEvent = audit.BuildEventFromRequest(apiKeyEvent, r)
	apiKeyEvent.Metadata = map[string]any{
		"bootstrap_key_id": key.KeyID,
		"scopes":           apiKeyParams.Scopes,
	}
	_ = h.runtime.Audit.Emit(ctx, apiKeyEvent)

	h.logger.Info("bootstrap key redeemed, org admin API key created",
		zap.String("bootstrapKeyId", key.KeyID),
		zap.String("apiKeyId", apiKey.KeyID),
		zap.String("orgId", key.OrgID.String()))

	// Build response
	resp := RedeemBootstrapKeyResponse{
		OrgID:       org.ID.String(),
		OrgName:     org.Name,
		APIEndpoint: h.runtime.Config.OIDCBaseURL,
		APIKey:      apiKeyToken, // Return the actual API key token (shown once)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}
