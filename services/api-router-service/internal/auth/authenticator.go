// Package auth provides authentication and authorization for API requests.
//
// Purpose:
//   This package implements API key authentication and optional HMAC signature
//   verification for inference requests. It validates credentials and extracts
//   organization context for downstream processing.
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
//
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

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthenticatedContext contains authentication and authorization context.
type AuthenticatedContext struct {
	APIKeyID       string
	OrganizationID string
	PrincipalID    string
	PrincipalType  string
	Scopes         []string
}

// Authenticator handles API key authentication.
type Authenticator struct {
	logger          *zap.Logger
	userOrgURL      string        // URL to user-org-service for key validation
	httpClient      *http.Client  // HTTP client for user-org-service requests
	validationCache map[string]*cachedValidation // Simple in-memory cache (key: fingerprint, value: validation result)
}

// cachedValidation stores a cached validation result with expiration.
type cachedValidation struct {
	result      *AuthenticatedContext
	expiresAt   time.Time
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
	apiKey := a.extractAPIKey(r)
	if apiKey == "" {
		return nil, fmt.Errorf("missing X-API-Key header")
	}

	// Validate API key against user-org-service
	ctx, err := a.validateAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Verify HMAC signature if provided
	if sig := r.Header.Get("X-HMAC-Signature"); sig != "" {
		if err := a.verifyHMAC(r, apiKey, sig); err != nil {
			return nil, fmt.Errorf("HMAC verification failed: %w", err)
		}
	}

	return ctx, nil
}

// extractAPIKey extracts the API key from request headers.
func (a *Authenticator) extractAPIKey(r *http.Request) string {
	// Check X-API-Key header first
	if key := r.Header.Get("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}

	// Fallback to Authorization header: Bearer <key>
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

// validateAPIKey validates an API key by calling user-org-service.
// Falls back to stub validation for dev/test keys if user-org-service is unavailable.
func (a *Authenticator) validateAPIKey(apiKey string) (*AuthenticatedContext, error) {
	// Check cache first (compute fingerprint for cache key)
	fingerprint := a.computeFingerprint(apiKey)
	if cached, ok := a.validationCache[fingerprint]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.result, nil
		}
		// Cache expired, remove it
		delete(a.validationCache, fingerprint)
	}

	// Fallback to stub for dev/test keys (for local development)
	if strings.HasPrefix(apiKey, "dev-") || strings.HasPrefix(apiKey, "test-") {
		return a.validateAPIKeyStub(apiKey)
	}

	// Validate against user-org-service
	if a.userOrgURL == "" {
		return nil, fmt.Errorf("user-org-service URL not configured")
	}

	// Extract org ID from key if possible (for optimization)
	// For now, we'll try without org_id first, then with org_id if available
	reqBody := map[string]string{
		"apiKeySecret": apiKey,
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
		if strings.HasPrefix(apiKey, "dev-") || strings.HasPrefix(apiKey, "test-") {
			return a.validateAPIKeyStub(apiKey)
		}
		return nil, fmt.Errorf("user-org-service unavailable: %w", err)
	}
	defer resp.Body.Close()

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
	}

	if err := json.NewDecoder(resp.Body).Decode(&validationResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !validationResp.Valid {
		return nil, fmt.Errorf("invalid API key: %s", validationResp.Message)
	}

	// Build authenticated context
	ctx := &AuthenticatedContext{
		APIKeyID:       validationResp.APIKeyID,
		OrganizationID: validationResp.OrganizationID,
		PrincipalID:    validationResp.PrincipalID,
		PrincipalType:  validationResp.PrincipalType,
		Scopes:         validationResp.Scopes,
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
			APIKeyID:       uuid.New().String(),
			OrganizationID: orgID,
			PrincipalID:    uuid.New().String(),
			PrincipalType:  "service_account",
			Scopes:         []string{"inference:read"},
		}, nil
	}

	return nil, fmt.Errorf("invalid API key format")
}

// computeFingerprint computes the SHA-256 fingerprint of an API key (same algorithm as user-org-service).
func (a *Authenticator) computeFingerprint(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// verifyHMAC verifies an HMAC signature of the request payload.
// The request body should be buffered by BodyBufferMiddleware before calling this function.
func (a *Authenticator) verifyHMAC(r *http.Request, apiKey, signature string) error {
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

	// TODO: Get secret from API key (requires user-org-service integration)
	// For now, use a stub secret. In production, this should fetch the secret
	// associated with the API key from user-org-service.
	secret := []byte("stub-secret") // Placeholder - replace with actual secret retrieval

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

