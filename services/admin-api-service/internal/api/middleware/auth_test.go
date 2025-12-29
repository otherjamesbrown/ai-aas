package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockMultiKeyValidator is a mock implementation of MultiKeyValidator for testing
type mockMultiKeyValidator struct {
	result *MultiKeyValidationResult
	err    error
}

func (m *mockMultiKeyValidator) ValidateKey(ctx context.Context, key string) (*MultiKeyValidationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestMultiAuth_MissingAuthHeader(t *testing.T) {
	logger := zap.NewNop()
	validator := &mockMultiKeyValidator{}

	handler := MultiAuth(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMultiAuth_InvalidAuthFormat(t *testing.T) {
	logger := zap.NewNop()
	validator := &mockMultiKeyValidator{}

	handler := MultiAuth(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	tests := []struct {
		name       string
		authHeader string
	}{
		{"no bearer prefix", "some-key"},
		{"basic auth", "Basic dXNlcjpwYXNz"},
		{"empty bearer", "Bearer "},
		{"bearer only", "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})
	}
}

func TestMultiAuth_InvalidKey(t *testing.T) {
	logger := zap.NewNop()
	validator := &mockMultiKeyValidator{
		result: &MultiKeyValidationResult{
			Valid:   false,
			Message: "invalid API key",
		},
	}

	handler := MultiAuth(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMultiAuth_MasterKeyValid(t *testing.T) {
	logger := zap.NewNop()
	validator := &mockMultiKeyValidator{
		result: &MultiKeyValidationResult{
			Valid:    true,
			KeyID:    "master-admin-key",
			IsMaster: true,
			OrgID:    nil,
		},
	}

	var capturedCtx context.Context
	handler := MultiAuth(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer master-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify context values
	if keyID := GetAPIKeyID(capturedCtx); keyID != "master-admin-key" {
		t.Errorf("expected key ID 'master-admin-key', got '%s'", keyID)
	}
	if !IsMasterKey(capturedCtx) {
		t.Error("expected IsMasterKey to be true")
	}
	if orgID := GetOrgID(capturedCtx); orgID != nil {
		t.Errorf("expected nil org ID for master key, got %v", orgID)
	}
}

func TestMultiAuth_OrgKeyValid(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.New()
	validator := &mockMultiKeyValidator{
		result: &MultiKeyValidationResult{
			Valid:    true,
			KeyID:    "org-api-key-123",
			IsMaster: false,
			OrgID:    &orgID,
		},
	}

	var capturedCtx context.Context
	handler := MultiAuth(validator, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer org-api-key")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify context values
	if keyID := GetAPIKeyID(capturedCtx); keyID != "org-api-key-123" {
		t.Errorf("expected key ID 'org-api-key-123', got '%s'", keyID)
	}
	if IsMasterKey(capturedCtx) {
		t.Error("expected IsMasterKey to be false for org key")
	}
	if ctxOrgID := GetOrgID(capturedCtx); ctxOrgID == nil || *ctxOrgID != orgID {
		t.Errorf("expected org ID %v, got %v", orgID, ctxOrgID)
	}
}

func TestMasterAndOrgKeyValidator_MasterKey(t *testing.T) {
	masterKey := "test-master-key-12345"
	validator := NewMasterAndOrgKeyValidator(masterKey, "", zap.NewNop())

	result, err := validator.ValidateKey(context.Background(), masterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid result for master key")
	}
	if !result.IsMaster {
		t.Error("expected IsMaster to be true")
	}
	if result.KeyID != "master-admin-key" {
		t.Errorf("expected key ID 'master-admin-key', got '%s'", result.KeyID)
	}
	if result.OrgID != nil {
		t.Errorf("expected nil org ID, got %v", result.OrgID)
	}
}

func TestMasterAndOrgKeyValidator_InvalidKey_NoUserOrgService(t *testing.T) {
	masterKey := "test-master-key-12345"
	validator := NewMasterAndOrgKeyValidator(masterKey, "", zap.NewNop())

	result, err := validator.ValidateKey(context.Background(), "wrong-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for wrong key")
	}
	if result.Message != "invalid API key" {
		t.Errorf("expected message 'invalid API key', got '%s'", result.Message)
	}
}

func TestMasterAndOrgKeyValidator_OrgKey_ViaUserOrgService(t *testing.T) {
	masterKey := "test-master-key-12345"
	orgID := uuid.New()

	// Create a mock user-org-service
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/validate-api-key" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp := map[string]interface{}{
			"valid":          true,
			"apiKeyId":       "api-key-uuid",
			"organizationId": orgID.String(),
			"principalId":    uuid.New().String(),
			"principalType":  "user",
			"scopes":         []string{"read", "write"},
			"status":         "active",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	validator := NewMasterAndOrgKeyValidator(masterKey, mockServer.URL, zap.NewNop())

	result, err := validator.ValidateKey(context.Background(), "org-api-key-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid result for org key")
	}
	if result.IsMaster {
		t.Error("expected IsMaster to be false for org key")
	}
	if result.KeyID != "api-key-uuid" {
		t.Errorf("expected key ID 'api-key-uuid', got '%s'", result.KeyID)
	}
	if result.OrgID == nil || *result.OrgID != orgID {
		t.Errorf("expected org ID %v, got %v", orgID, result.OrgID)
	}
}

func TestMasterAndOrgKeyValidator_OrgKey_Revoked(t *testing.T) {
	masterKey := "test-master-key-12345"

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"valid":   false,
			"message": "API key is revoked",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	validator := NewMasterAndOrgKeyValidator(masterKey, mockServer.URL, zap.NewNop())

	result, err := validator.ValidateKey(context.Background(), "revoked-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for revoked key")
	}
	if result.Message != "API key is revoked" {
		t.Errorf("expected message 'API key is revoked', got '%s'", result.Message)
	}
}

func TestMasterAndOrgKeyValidator_OrgKey_Expired(t *testing.T) {
	masterKey := "test-master-key-12345"

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"valid":   false,
			"message": "API key is expired",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	validator := NewMasterAndOrgKeyValidator(masterKey, mockServer.URL, zap.NewNop())

	result, err := validator.ValidateKey(context.Background(), "expired-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for expired key")
	}
	if result.Message != "API key is expired" {
		t.Errorf("expected message 'API key is expired', got '%s'", result.Message)
	}
}

func TestGetAPIKeyID_Empty(t *testing.T) {
	ctx := context.Background()
	if keyID := GetAPIKeyID(ctx); keyID != "" {
		t.Errorf("expected empty key ID, got '%s'", keyID)
	}
}

func TestGetOrgID_Nil(t *testing.T) {
	ctx := context.Background()
	if orgID := GetOrgID(ctx); orgID != nil {
		t.Errorf("expected nil org ID, got %v", orgID)
	}
}

func TestIsMasterKey_Default(t *testing.T) {
	ctx := context.Background()
	if IsMasterKey(ctx) {
		t.Error("expected IsMasterKey to be false by default")
	}
}

func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"test", "test", true},
		{"test", "Test", false},
		{"", "", true},
		{"a", "b", false},
		{"longer-string", "longer-string", true},
		{"abc", "abcd", false},
	}

	for _, tt := range tests {
		result := ConstantTimeCompare(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("ConstantTimeCompare(%q, %q) = %v, expected %v", tt.a, tt.b, result, tt.expected)
		}
	}
}
