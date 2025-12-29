package middleware

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const (
	// APIKeyContextKey is the context key for the authenticated API key ID
	APIKeyContextKey contextKey = "api_key_id"
	// OrgIDContextKey is the context key for the organization ID (for org API keys)
	OrgIDContextKey contextKey = "org_id"
	// IsMasterKeyContextKey is the context key indicating if the master key was used
	IsMasterKeyContextKey contextKey = "is_master_key"
)

// APIKeyValidator validates API keys
type APIKeyValidator interface {
	ValidateKey(ctx context.Context, key string) (keyID string, valid bool, err error)
}

// MultiKeyValidationResult contains the result of multi-key validation
type MultiKeyValidationResult struct {
	Valid    bool
	KeyID    string
	OrgID    *uuid.UUID // nil for master key
	IsMaster bool
	Message  string
}

// MultiKeyValidator validates both master and organization API keys
type MultiKeyValidator interface {
	ValidateKey(ctx context.Context, key string) (*MultiKeyValidationResult, error)
}

// Auth creates an authentication middleware using the legacy single-key validator
func Auth(validator APIKeyValidator, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			// Expect "Bearer <key>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}

			apiKey := parts[1]
			if apiKey == "" {
				writeUnauthorized(w, "empty API key")
				return
			}

			// Validate the API key
			keyID, valid, err := validator.ValidateKey(r.Context(), apiKey)
			if err != nil {
				logger.Error("API key validation error", zap.Error(err))
				writeUnauthorized(w, "authentication failed")
				return
			}

			if !valid {
				writeUnauthorized(w, "invalid or revoked API key")
				return
			}

			// Add key ID to context
			ctx := context.WithValue(r.Context(), APIKeyContextKey, keyID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MultiAuth creates an authentication middleware that supports both master and org API keys
func MultiAuth(validator MultiKeyValidator, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			// Expect "Bearer <key>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}

			apiKey := parts[1]
			if apiKey == "" {
				writeUnauthorized(w, "empty API key")
				return
			}

			// Validate the API key
			result, err := validator.ValidateKey(r.Context(), apiKey)
			if err != nil {
				logger.Error("API key validation error", zap.Error(err))
				writeUnauthorized(w, "authentication failed")
				return
			}

			if !result.Valid {
				msg := "invalid or revoked API key"
				if result.Message != "" {
					msg = result.Message
				}
				writeUnauthorized(w, msg)
				return
			}

			// Add authentication info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, APIKeyContextKey, result.KeyID)
			ctx = context.WithValue(ctx, IsMasterKeyContextKey, result.IsMaster)
			if result.OrgID != nil {
				ctx = context.WithValue(ctx, OrgIDContextKey, *result.OrgID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIKeyID retrieves the API key ID from context
func GetAPIKeyID(ctx context.Context) string {
	if keyID, ok := ctx.Value(APIKeyContextKey).(string); ok {
		return keyID
	}
	return ""
}

// GetOrgID retrieves the organization ID from context (returns nil UUID if not set)
func GetOrgID(ctx context.Context) *uuid.UUID {
	if orgID, ok := ctx.Value(OrgIDContextKey).(uuid.UUID); ok {
		return &orgID
	}
	return nil
}

// IsMasterKey returns true if the request was authenticated with the master key
func IsMasterKey(ctx context.Context) bool {
	if isMaster, ok := ctx.Value(IsMasterKeyContextKey).(bool); ok {
		return isMaster
	}
	return false
}

// ConstantTimeCompare performs a constant-time string comparison
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"type":"https://docs.otherjamesbrown.com/errors/unauthorized","title":"Unauthorized","status":401,"detail":"` + message + `"}`))
}

// MasterAndOrgKeyValidator validates both master admin API key and organization API keys
type MasterAndOrgKeyValidator struct {
	masterKey         string
	userOrgServiceURL string
	httpClient        *http.Client
	logger            *zap.Logger
}

// NewMasterAndOrgKeyValidator creates a new multi-key validator
func NewMasterAndOrgKeyValidator(masterKey, userOrgServiceURL string, logger *zap.Logger) *MasterAndOrgKeyValidator {
	return &MasterAndOrgKeyValidator{
		masterKey:         masterKey,
		userOrgServiceURL: userOrgServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// ValidateKey validates an API key, checking master key first then org API keys
func (v *MasterAndOrgKeyValidator) ValidateKey(ctx context.Context, key string) (*MultiKeyValidationResult, error) {
	// Check if it's the master key first (constant-time comparison)
	if subtle.ConstantTimeCompare([]byte(key), []byte(v.masterKey)) == 1 {
		return &MultiKeyValidationResult{
			Valid:    true,
			KeyID:    "master-admin-key",
			IsMaster: true,
			OrgID:    nil,
		}, nil
	}

	// If user-org-service URL not configured, only master key works
	if v.userOrgServiceURL == "" {
		return &MultiKeyValidationResult{
			Valid:   false,
			Message: "invalid API key",
		}, nil
	}

	// Validate as org API key via user-org-service
	return v.validateOrgAPIKey(ctx, key)
}

// validateAPIKeyRequest is the request payload for user-org-service validation
type validateAPIKeyRequest struct {
	APIKeySecret string `json:"apiKeySecret"`
}

// validateAPIKeyResponse is the response from user-org-service validation
type validateAPIKeyResponse struct {
	Valid          bool     `json:"valid"`
	APIKeyID       string   `json:"apiKeyId,omitempty"`
	OrganizationID string   `json:"organizationId,omitempty"`
	PrincipalID    string   `json:"principalId,omitempty"`
	PrincipalType  string   `json:"principalType,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	Status         string   `json:"status,omitempty"`
	Message        string   `json:"message,omitempty"`
}

func (v *MasterAndOrgKeyValidator) validateOrgAPIKey(ctx context.Context, key string) (*MultiKeyValidationResult, error) {
	// Prepare request
	reqBody := validateAPIKeyRequest{
		APIKeySecret: key,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := strings.TrimSuffix(v.userOrgServiceURL, "/") + "/v1/auth/validate-api-key"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := v.httpClient.Do(req)
	if err != nil {
		v.logger.Warn("failed to validate API key via user-org-service",
			zap.Error(err),
			zap.String("url", url))
		return nil, fmt.Errorf("failed to contact user-org-service: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var validationResp validateAPIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&validationResp); err != nil {
		return nil, fmt.Errorf("failed to decode validation response: %w", err)
	}

	if !validationResp.Valid {
		return &MultiKeyValidationResult{
			Valid:   false,
			Message: validationResp.Message,
		}, nil
	}

	// Parse organization ID
	orgID, err := uuid.Parse(validationResp.OrganizationID)
	if err != nil {
		v.logger.Error("invalid organization ID from user-org-service",
			zap.String("org_id", validationResp.OrganizationID),
			zap.Error(err))
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}

	return &MultiKeyValidationResult{
		Valid:    true,
		KeyID:    validationResp.APIKeyID,
		OrgID:    &orgID,
		IsMaster: false,
	}, nil
}
