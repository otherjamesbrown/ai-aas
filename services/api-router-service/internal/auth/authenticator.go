// Package auth provides authentication and authorization for API requests.
//
// Purpose:
//
//	This package implements API key authentication and optional HMAC signature
//	verification for inference requests. It validates credentials and extracts
//	organization context for downstream processing.
//
// Dependencies:
//   - user-org-service: For API key validation (can be stubbed initially)
//
// Key Responsibilities:
//   - Validate API keys from X-API-Key header
//   - Verify HMAC signatures if provided
//   - Extract organization and principal context
//   - Handle revocation and expiration checks
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#FR-001 (Credential validation)
package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ai-aas/shared-go/auth/apikey"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthenticatedContext is an alias to the shared apikey.AuthenticatedContext type.
// This maintains backward compatibility while using the shared implementation.
type AuthenticatedContext = apikey.AuthenticatedContext

// Authenticator handles API key authentication.
type Authenticator struct {
	logger          *zap.Logger
	userOrgURL      string                       // URL to user-org-service for key validation
	httpClient      *http.Client                 // HTTP client for user-org-service requests
	validationCache map[string]*cachedValidation // Simple in-memory cache (key: fingerprint, value: validation result)
}

// cachedValidation stores a cached validation result with expiration.
type cachedValidation struct {
	result    *AuthenticatedContext
	expiresAt time.Time
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(logger *zap.Logger, userOrgURL string, timeout time.Duration) *Authenticator {
	return &Authenticator{
		logger:          logger,
		userOrgURL:      strings.TrimSuffix(userOrgURL, "/"),
		httpClient:      &http.Client{Timeout: timeout},
		validationCache: make(map[string]*cachedValidation),
	}
}

// Authenticate validates the API key from the request headers.
// Returns authenticated context or an error.
func (a *Authenticator) Authenticate(r *http.Request) (*AuthenticatedContext, error) {
	key := apikey.ExtractAPIKey(r)
	if key == "" {
		return nil, fmt.Errorf("missing X-API-Key header")
	}

	// Validate API key against user-org-service
	ctx, err := a.validateAPIKey(key)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Verify HMAC signature if provided
	if sig := r.Header.Get("X-HMAC-Signature"); sig != "" {
		if err := a.verifyHMAC(r, ctx, sig); err != nil {
			return nil, fmt.Errorf("HMAC verification failed: %w", err)
		}
	}

	return ctx, nil
}

// validateAPIKey validates an API key by calling user-org-service.
// Falls back to stub validation for dev/test keys if user-org-service is unavailable.
func (a *Authenticator) validateAPIKey(key string) (*AuthenticatedContext, error) {
	// Check cache first (compute fingerprint for cache key)
	fingerprint := apikey.ComputeFingerprintHex(key)
	if cached, ok := a.validationCache[fingerprint]; ok {
		if time.Now().Before(cached.expiresAt) {
			a.logger.Debug("API key validation cache hit", zap.String("fingerprint", fingerprint[:8]))
			return cached.result, nil
		}
		// Cache expired, remove it
		delete(a.validationCache, fingerprint)
		a.logger.Debug("API key validation cache expired", zap.String("fingerprint", fingerprint[:8]))
	}

	// Fallback to stub for dev/test keys (for local development)
	if strings.HasPrefix(key, "dev-") || strings.HasPrefix(key, "test-") {
		a.logger.Debug("using stub validator for dev/test key", zap.String("prefix", key[:5]))
		return a.validateAPIKeyStub(key)
	}

	// Validate against user-org-service
	if a.userOrgURL == "" {
		return nil, fmt.Errorf("user-org-service URL not configured")
	}

	// Extract org ID from key if possible (for optimization)
	// For now, we'll try without org_id first, then with org_id if available
	reqBody := map[string]string{
		"apiKeySecret": key,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", a.userOrgURL+"/v1/auth/validate-api-key", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		// If user-org-service is unavailable, fall back to stub for dev keys
		a.logger.Warn("user-org-service unavailable, falling back to stub validation", zap.Error(err))
		if strings.HasPrefix(key, "dev-") || strings.HasPrefix(key, "test-") {
			return a.validateAPIKeyStub(key)
		}
		return nil, fmt.Errorf("user-org-service unavailable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user-org-service returned status %d: %s", resp.StatusCode, string(body))
	}

	var validationResp struct {
		Valid          bool     `json:"valid"`
		APIKeyID       string   `json:"apiKeyId"`
		OrganizationID string   `json:"organizationId"`
		PrincipalID    string   `json:"principalId"`
		PrincipalType  string   `json:"principalType"`
		Scopes         []string `json:"scopes"`
		Status         string   `json:"status"`
		Message        string   `json:"message"`
		// Model access control (Spec 022)
		ModelAccessMode string   `json:"modelAccessMode,omitempty"` // "restricted" or "auto_grant"
		GrantedModels   []string `json:"grantedModels,omitempty"`   // Only populated for restricted mode
		// HMAC secret for request signing verification
		HMACSecret string `json:"hmacSecret,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&validationResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !validationResp.Valid {
		return nil, fmt.Errorf("invalid API key: %s", validationResp.Message)
	}

	// Build authenticated context
	ctx := &AuthenticatedContext{
		APIKeyID:        validationResp.APIKeyID,
		OrganizationID:  validationResp.OrganizationID,
		PrincipalID:     validationResp.PrincipalID,
		PrincipalType:   validationResp.PrincipalType,
		Scopes:          validationResp.Scopes,
		ModelAccessMode: validationResp.ModelAccessMode,
		GrantedModels:   validationResp.GrantedModels,
		HMACSecret:      validationResp.HMACSecret,
	}

	// Cache the result for 1 minute
	a.validationCache[fingerprint] = &cachedValidation{
		result:    ctx,
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	return ctx, nil
}

// validateAPIKeyStub validates an API key using a stub implementation for dev/test keys.
func (a *Authenticator) validateAPIKeyStub(apiKey string) (*AuthenticatedContext, error) {
	// Stub implementation for development
	// Accepts keys starting with "dev-" or "test-"
	if strings.HasPrefix(apiKey, "dev-") || strings.HasPrefix(apiKey, "test-") {
		// Extract org ID from key format: dev-{org-id}-{key-id}
		parts := strings.Split(apiKey, "-")
		orgID := "00000000-0000-0000-0000-000000000001" // Default dev org
		if len(parts) >= 3 {
			orgID = parts[1]
		}

		return &AuthenticatedContext{
			APIKeyID:        uuid.New().String(),
			OrganizationID:  orgID,
			PrincipalID:     uuid.New().String(),
			PrincipalType:   "service_account",
			Scopes:          []string{"inference:read"},
			ModelAccessMode: "auto_grant", // Default to auto_grant for dev/test keys
			GrantedModels:   []string{},   // Empty - all models allowed
		}, nil
	}

	return nil, fmt.Errorf("invalid API key format")
}

// verifyHMAC verifies an HMAC signature of the request payload.
// The request body should be buffered by BodyBufferMiddleware before calling this function.
func (a *Authenticator) verifyHMAC(r *http.Request, authCtx *AuthenticatedContext, signature string) error {
	// Get buffered body from context (set by BodyBufferMiddleware)
	var body []byte
	if bufferedBody := r.Context().Value("buffered_body"); bufferedBody != nil {
		if b, ok := bufferedBody.([]byte); ok {
			body = b
		}
	}

	// If no buffered body in context, try to read from request body
	// (fallback for cases where middleware isn't used)
	if len(body) == 0 && r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		// Restore body for downstream handlers
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	if len(body) == 0 {
		return fmt.Errorf("request body is empty")
	}

	// Get HMAC secret from authenticated context (returned by user-org-service validation)
	if authCtx.HMACSecret == "" {
		return fmt.Errorf("HMAC verification requested but no HMAC secret configured for this API key")
	}
	secret := []byte(authCtx.HMACSecret)

	// Compute HMAC
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("HMAC signature mismatch")
	}

	return nil
}

// IsRevoked checks if an API key is revoked.
// TODO: Implement with user-org-service integration.
func (a *Authenticator) IsRevoked(apiKeyID string) (bool, error) {
	// Stub: always return false
	return false, nil
}

// IsExpired checks if an API key is expired.
// TODO: Implement with user-org-service integration.
func (a *Authenticator) IsExpired(apiKeyID string) (bool, error) {
	// Stub: always return false
	return false, nil
}

// UpdateLastUsed updates the last used timestamp for an API key.
// TODO: Implement with user-org-service integration.
func (a *Authenticator) UpdateLastUsed(apiKeyID string) error {
	// Stub: no-op
	return nil
}
