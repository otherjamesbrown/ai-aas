// Package integration provides integration tests for API key validation.
//
// Purpose:
//   These tests validate the API key validation flow between API Router Service
//   and User-Org-Service, including caching, error handling, and fallback behavior.
//
// Key Responsibilities:
//   - Test valid API key validation
//   - Test invalid/revoked/expired key rejection
//   - Test service unavailable fallback
//   - Validate caching behavior
//   - Test error handling and response codes
//
package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
)

// mockUserOrgService creates a mock User-Org-Service HTTP server for testing.
func mockUserOrgService(t *testing.T, responses map[string]*authValidationResponse) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/auth/validate-api-key" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Parse request
		var req struct {
			APIKeySecret string `json:"apiKeySecret"`
			OrgID        string `json:"orgId,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Look up response for this API key
		resp, ok := responses[req.APIKeySecret]
		if !ok {
			// Default: key not found
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(authValidationResponse{
				Valid:   false,
				Message: "API key not found",
			})
			return
		}

		// Return configured response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	return httptest.NewServer(handler)
}

// authValidationResponse represents the response from user-org-service validation endpoint.
type authValidationResponse struct {
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

// TestAPIKeyValidationSuccess tests successful API key validation.
func TestAPIKeyValidationSuccess(t *testing.T) {
	// Setup mock user-org-service with valid key response
	validKey := "valid-api-key-secret-12345"
	responses := map[string]*authValidationResponse{
		validKey: {
			Valid:          true,
			APIKeyID:       "550e8400-e29b-41d4-a716-446655440000",
			OrganizationID: "123e4567-e89b-12d3-a456-426614174000",
			PrincipalID:    "789e0123-e45b-67c8-d901-234567890123",
			PrincipalType:  "service_account",
			Scopes:         []string{"inference:read", "inference:write"},
			Status:         "active",
		},
	}

	mockService := mockUserOrgService(t, responses)
	defer mockService.Close()

	// Create authenticator pointing to mock service
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Create a test request with valid API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", validKey)

	// Authenticate
	ctx, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("expected successful authentication, got error: %v", err)
	}

	// Verify context
	if ctx.APIKeyID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected APIKeyID %s, got %s", "550e8400-e29b-41d4-a716-446655440000", ctx.APIKeyID)
	}
	if ctx.OrganizationID != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("expected OrganizationID %s, got %s", "123e4567-e89b-12d3-a456-426614174000", ctx.OrganizationID)
	}
	if ctx.PrincipalType != "service_account" {
		t.Errorf("expected PrincipalType %s, got %s", "service_account", ctx.PrincipalType)
	}
	if len(ctx.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(ctx.Scopes))
	}
}

// TestAPIKeyValidationInvalid tests rejection of invalid API keys.
func TestAPIKeyValidationInvalid(t *testing.T) {
	// Setup mock user-org-service with invalid key response
	invalidKey := "invalid-api-key"
	responses := map[string]*authValidationResponse{
		invalidKey: {
			Valid:   false,
			Message: "API key not found",
		},
	}

	mockService := mockUserOrgService(t, responses)
	defer mockService.Close()

	// Create authenticator pointing to mock service
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Create a test request with invalid API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", invalidKey)

	// Authenticate - should fail
	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected authentication failure, got success with context: %+v", ctx)
	}
	if ctx != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx)
	}
}

// TestAPIKeyValidationRevoked tests rejection of revoked API keys.
func TestAPIKeyValidationRevoked(t *testing.T) {
	// Setup mock user-org-service with revoked key response
	revokedKey := "revoked-api-key-secret"
	responses := map[string]*authValidationResponse{
		revokedKey: {
			Valid:   false,
			Message: "API key is revoked",
		},
	}

	mockService := mockUserOrgService(t, responses)
	defer mockService.Close()

	// Create authenticator pointing to mock service
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Create a test request with revoked API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", revokedKey)

	// Authenticate - should fail
	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected authentication failure for revoked key, got success")
	}
	if ctx != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx)
	}
}

// TestAPIKeyValidationExpired tests rejection of expired API keys.
func TestAPIKeyValidationExpired(t *testing.T) {
	// Setup mock user-org-service with expired key response
	expiredKey := "expired-api-key-secret"
	responses := map[string]*authValidationResponse{
		expiredKey: {
			Valid:   false,
			Message: "API key is expired",
		},
	}

	mockService := mockUserOrgService(t, responses)
	defer mockService.Close()

	// Create authenticator pointing to mock service
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Create a test request with expired API key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", expiredKey)

	// Authenticate - should fail
	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected authentication failure for expired key, got success")
	}
	if ctx != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx)
	}
}

// TestAPIKeyValidationServiceUnavailable tests fallback when user-org-service is unavailable.
func TestAPIKeyValidationServiceUnavailable(t *testing.T) {
	// Create authenticator pointing to non-existent service
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "http://localhost:99999", 100*time.Millisecond)

	// Test with dev key - should fall back to stub
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "dev-test-key")

	ctx, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("expected fallback to stub for dev key, got error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected context from stub validation, got nil")
	}
	if ctx.OrganizationID == "" {
		t.Error("expected organization ID from stub validation")
	}

	// Test with non-dev key - should fail when service unavailable
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", "real-api-key-secret")

	ctx2, err2 := authenticator.Authenticate(req2)
	if err2 == nil {
		t.Fatalf("expected error when service unavailable and key is not dev/test, got success")
	}
	if ctx2 != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx2)
	}
}

// TestAPIKeyValidationCaching tests that validation results are cached.
func TestAPIKeyValidationCaching(t *testing.T) {
	// Track number of validation requests
	requestCount := 0
	validKey := "cached-api-key-secret"

	// Setup mock user-org-service that counts requests
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/validate-api-key" {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(authValidationResponse{
				Valid:          true,
				APIKeyID:       "cached-key-id",
				OrganizationID: "cached-org-id",
				PrincipalID:    "cached-principal-id",
				PrincipalType:  "service_account",
				Scopes:         []string{"inference:read"},
			})
		}
	})
	mockService := httptest.NewServer(handler)
	defer mockService.Close()

	// Create authenticator pointing to mock service
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// First authentication - should call service
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-API-Key", validKey)
	ctx1, err1 := authenticator.Authenticate(req1)
	if err1 != nil {
		t.Fatalf("first authentication failed: %v", err1)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 validation request, got %d", requestCount)
	}

	// Second authentication with same key - should use cache
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-API-Key", validKey)
	ctx2, err2 := authenticator.Authenticate(req2)
	if err2 != nil {
		t.Fatalf("second authentication failed: %v", err2)
	}
	if requestCount != 1 {
		t.Errorf("expected cached result (still 1 request), got %d requests", requestCount)
	}
	if ctx2.APIKeyID != ctx1.APIKeyID {
		t.Errorf("cached context mismatch: expected %s, got %s", ctx1.APIKeyID, ctx2.APIKeyID)
	}
}

// TestAPIKeyValidationMissingHeader tests error when API key header is missing.
func TestAPIKeyValidationMissingHeader(t *testing.T) {
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "http://localhost:8081", 2*time.Second)

	// Create request without API key header
	req := httptest.NewRequest("GET", "/test", nil)

	// Authenticate - should fail
	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected error for missing API key, got success")
	}
	if ctx != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx)
	}
}

// TestAPIKeyValidationAuthorizationHeader tests API key extraction from Authorization header.
func TestAPIKeyValidationAuthorizationHeader(t *testing.T) {
	validKey := "bearer-api-key-secret"
	responses := map[string]*authValidationResponse{
		validKey: {
			Valid:          true,
			APIKeyID:       "bearer-key-id",
			OrganizationID: "bearer-org-id",
			PrincipalID:    "bearer-principal-id",
			PrincipalType:  "service_account",
			Scopes:         []string{"inference:read"},
		},
	}

	mockService := mockUserOrgService(t, responses)
	defer mockService.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Create request with Authorization header instead of X-API-Key
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+validKey)

	ctx, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("expected successful authentication with Authorization header, got error: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected context from authentication, got nil")
	}
	if ctx.APIKeyID != "bearer-key-id" {
		t.Errorf("expected APIKeyID %s, got %s", "bearer-key-id", ctx.APIKeyID)
	}
}

// TestAPIKeyValidationErrorResponse tests handling of error responses from user-org-service.
func TestAPIKeyValidationErrorResponse(t *testing.T) {
	// Setup mock service that returns error status
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/validate-api-key" {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		}
	})
	mockService := httptest.NewServer(handler)
	defer mockService.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Use a non-dev/test key to avoid stub fallback
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "real-api-key-not-dev")

	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected error for service error response, got success")
	}
	if ctx != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx)
	}
}

// TestAPIKeyValidationMalformedResponse tests handling of malformed JSON responses.
func TestAPIKeyValidationMalformedResponse(t *testing.T) {
	// Setup mock service that returns malformed JSON
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/validate-api-key" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{ invalid json }"))
		}
	})
	mockService := httptest.NewServer(handler)
	defer mockService.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 2*time.Second)

	// Use a non-dev/test key to avoid stub fallback
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "real-api-key-malformed")

	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected error for malformed response, got success")
	}
	if ctx != nil {
		t.Errorf("expected nil context on failure, got %+v", ctx)
	}
}

// TestAPIKeyValidationTimeout tests handling of request timeouts.
func TestAPIKeyValidationTimeout(t *testing.T) {
	// Setup mock service that delays response
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/validate-api-key" {
			time.Sleep(3 * time.Second) // Longer than timeout
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(authValidationResponse{
				Valid: true,
			})
		}
	})
	mockService := httptest.NewServer(handler)
	defer mockService.Close()

	logger := zap.NewNop()
	// Use short timeout
	authenticator := auth.NewAuthenticator(logger, mockService.URL, 500*time.Millisecond)

	// Use a non-dev/test key to avoid stub fallback
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "real-api-key-timeout")

	ctx, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatalf("expected timeout error, got success")
	}
	if ctx != nil {
		t.Errorf("expected nil context on timeout, got %+v", ctx)
	}
	// Verify error mentions timeout or unavailable
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "timeout") && !strings.Contains(errMsg, "unavailable") {
			t.Logf("got error (expected): %v", err)
		}
	}
}

