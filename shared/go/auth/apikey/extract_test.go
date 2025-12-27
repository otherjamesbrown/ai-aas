package apikey

import (
	"net/http/httptest"
	"testing"
)

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		expectedKey string
	}{
		{
			name:        "X-API-Key header",
			headers:     map[string]string{"X-API-Key": "my-api-key-123"},
			expectedKey: "my-api-key-123",
		},
		{
			name:        "X-API-Key with spaces",
			headers:     map[string]string{"X-API-Key": "  my-api-key-123  "},
			expectedKey: "my-api-key-123",
		},
		{
			name:        "Authorization Bearer",
			headers:     map[string]string{"Authorization": "Bearer my-bearer-token"},
			expectedKey: "my-bearer-token",
		},
		{
			name:        "Authorization bearer lowercase",
			headers:     map[string]string{"Authorization": "bearer my-bearer-token"},
			expectedKey: "my-bearer-token",
		},
		{
			name:        "X-API-Key takes precedence",
			headers:     map[string]string{"X-API-Key": "api-key", "Authorization": "Bearer bearer-token"},
			expectedKey: "api-key",
		},
		{
			name:        "no headers",
			headers:     map[string]string{},
			expectedKey: "",
		},
		{
			name:        "empty X-API-Key falls back to Bearer",
			headers:     map[string]string{"X-API-Key": "", "Authorization": "Bearer fallback-token"},
			expectedKey: "fallback-token",
		},
		{
			name:        "invalid Authorization format",
			headers:     map[string]string{"Authorization": "Basic credentials"},
			expectedKey: "",
		},
		{
			name:        "Authorization without scheme",
			headers:     map[string]string{"Authorization": "just-a-token"},
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			got := ExtractAPIKey(r)
			if got != tt.expectedKey {
				t.Errorf("ExtractAPIKey() = %q, want %q", got, tt.expectedKey)
			}
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		expectedKey string
	}{
		{
			name:        "valid Bearer token",
			authHeader:  "Bearer my-token-123",
			expectedKey: "my-token-123",
		},
		{
			name:        "Bearer lowercase",
			authHeader:  "bearer my-token-123",
			expectedKey: "my-token-123",
		},
		{
			name:        "Bearer mixed case",
			authHeader:  "BEARER my-token-123",
			expectedKey: "my-token-123",
		},
		{
			name:        "Bearer with extra spaces",
			authHeader:  "Bearer   my-token-123  ",
			expectedKey: "my-token-123",
		},
		{
			name:        "Basic auth",
			authHeader:  "Basic dXNlcjpwYXNz",
			expectedKey: "",
		},
		{
			name:        "empty header",
			authHeader:  "",
			expectedKey: "",
		},
		{
			name:        "just Bearer",
			authHeader:  "Bearer",
			expectedKey: "",
		},
		{
			name:        "Bearer with empty token",
			authHeader:  "Bearer ",
			expectedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				r.Header.Set("Authorization", tt.authHeader)
			}

			got := ExtractBearerToken(r)
			if got != tt.expectedKey {
				t.Errorf("ExtractBearerToken() = %q, want %q", got, tt.expectedKey)
			}
		})
	}
}

func TestHasAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "has X-API-Key",
			headers:  map[string]string{"X-API-Key": "key"},
			expected: true,
		},
		{
			name:     "has Bearer token",
			headers:  map[string]string{"Authorization": "Bearer token"},
			expected: true,
		},
		{
			name:     "no key",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name:     "empty X-API-Key",
			headers:  map[string]string{"X-API-Key": ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			got := HasAPIKey(r)
			if got != tt.expected {
				t.Errorf("HasAPIKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if HeaderName != "X-API-Key" {
		t.Errorf("HeaderName = %q, want %q", HeaderName, "X-API-Key")
	}
	if AuthorizationScheme != "Bearer" {
		t.Errorf("AuthorizationScheme = %q, want %q", AuthorizationScheme, "Bearer")
	}
}
