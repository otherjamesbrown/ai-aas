package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
)

// TestRequireAdminScope_Success tests that requests with required scopes succeed.
func TestRequireAdminScope_Success(t *testing.T) {
	tests := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expectSuccess  bool
	}{
		{
			name:           "user has exact required scope",
			userScopes:     []string{"user:manage"},
			requiredScopes: []string{"user:manage"},
			expectSuccess:  true,
		},
		{
			name:           "user has one of multiple required scopes",
			userScopes:     []string{"org:admin"},
			requiredScopes: []string{"user:manage", "org:admin"},
			expectSuccess:  true,
		},
		{
			name:           "user has admin scope (universal grant)",
			userScopes:     []string{"admin"},
			requiredScopes: []string{"user:manage"},
			expectSuccess:  true,
		},
		{
			name:           "user has wildcard scope (universal grant)",
			userScopes:     []string{"*"},
			requiredScopes: []string{"user:manage"},
			expectSuccess:  true,
		},
		{
			name:           "user has multiple scopes including required",
			userScopes:     []string{"org:read", "user:manage", "apikey:manage"},
			requiredScopes: []string{"user:manage"},
			expectSuccess:  true,
		},
		{
			name:           "user lacks required scope",
			userScopes:     []string{"org:read"},
			requiredScopes: []string{"user:manage"},
			expectSuccess:  false,
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"user:manage"},
			expectSuccess:  false,
		},
		{
			name:           "user has different scope",
			userScopes:     []string{"apikey:manage"},
			requiredScopes: []string{"user:manage", "org:admin"},
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler that should only be called if scope check passes
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Apply scope middleware
			middleware := RequireAdminScope(nil, tt.requiredScopes...)
			handler := middleware(testHandler)

			// Create request with authenticated context
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			session := &oauth.Session{
				GrantedScopes: tt.userScopes,
			}
			ctx := context.WithValue(req.Context(), SessionKey, session)
			req = req.WithContext(ctx)

			// Execute request
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Verify result
			if tt.expectSuccess {
				if !handlerCalled {
					t.Errorf("expected handler to be called but it was not")
				}
				if rr.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", rr.Code)
				}
			} else {
				if handlerCalled {
					t.Errorf("expected handler NOT to be called but it was")
				}
				if rr.Code != http.StatusForbidden {
					t.Errorf("expected status 403 Forbidden, got %d", rr.Code)
				}
			}
		})
	}
}

// TestRequireAdminScope_NoSession tests that middleware returns 403 when no session exists.
func TestRequireAdminScope_NoSession(t *testing.T) {
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAdminScope(nil, "user:manage")
	handler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No session in context

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if handlerCalled {
		t.Errorf("expected handler NOT to be called but it was")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden, got %d", rr.Code)
	}
}

// TestRequireAdminScope_AdminScopeGrantsAll tests that admin and * scopes grant access to all endpoints.
func TestRequireAdminScope_AdminScopeGrantsAll(t *testing.T) {
	tests := []struct {
		name       string
		userScope  string
		shouldPass bool
	}{
		{
			name:       "admin scope grants access",
			userScope:  "admin",
			shouldPass: true,
		},
		{
			name:       "wildcard scope grants access",
			userScope:  "*",
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Test with various specific required scopes
			requiredScopes := [][]string{
				{"user:manage"},
				{"org:admin"},
				{"apikey:manage"},
				{"org:read", "org:write"},
			}

			for _, required := range requiredScopes {
				handlerCalled = false
				middleware := RequireAdminScope(nil, required...)
				handler := middleware(testHandler)

				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				session := &oauth.Session{
					GrantedScopes: []string{tt.userScope},
				}
				ctx := context.WithValue(req.Context(), SessionKey, session)
				req = req.WithContext(ctx)

				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				if !handlerCalled {
					t.Errorf("expected handler to be called with %s scope for required scopes %v", tt.userScope, required)
				}
				if rr.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", rr.Code)
				}
			}
		})
	}
}

// TestHasAnyScope tests the HasAnyScope helper function.
func TestHasAnyScope(t *testing.T) {
	tests := []struct {
		name          string
		userScopes    []string
		checkScopes   []string
		expectedMatch bool
	}{
		{
			name:          "exact match",
			userScopes:    []string{"user:manage"},
			checkScopes:   []string{"user:manage"},
			expectedMatch: true,
		},
		{
			name:          "match one of several",
			userScopes:    []string{"org:read", "user:manage"},
			checkScopes:   []string{"user:manage", "org:admin"},
			expectedMatch: true,
		},
		{
			name:          "no match",
			userScopes:    []string{"org:read"},
			checkScopes:   []string{"user:manage", "org:admin"},
			expectedMatch: false,
		},
		{
			name:          "empty user scopes",
			userScopes:    []string{},
			checkScopes:   []string{"user:manage"},
			expectedMatch: false,
		},
		{
			name:          "admin scope matches",
			userScopes:    []string{"admin"},
			checkScopes:   []string{"admin"},
			expectedMatch: true,
		},
		{
			name:          "wildcard matches",
			userScopes:    []string{"*"},
			checkScopes:   []string{"*"},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &oauth.Session{
				GrantedScopes: tt.userScopes,
			}
			ctx := context.WithValue(context.Background(), SessionKey, session)

			result := HasAnyScope(ctx, tt.checkScopes...)
			if result != tt.expectedMatch {
				t.Errorf("expected HasAnyScope to return %v, got %v", tt.expectedMatch, result)
			}
		})
	}
}

// TestHasScope tests the HasScope helper function.
func TestHasScope(t *testing.T) {
	tests := []struct {
		name          string
		userScopes    []string
		checkScope    string
		expectedMatch bool
	}{
		{
			name:          "exact match",
			userScopes:    []string{"user:manage"},
			checkScope:    "user:manage",
			expectedMatch: true,
		},
		{
			name:          "no match",
			userScopes:    []string{"org:read"},
			checkScope:    "user:manage",
			expectedMatch: false,
		},
		{
			name:          "match in multiple scopes",
			userScopes:    []string{"org:read", "user:manage", "apikey:manage"},
			checkScope:    "user:manage",
			expectedMatch: true,
		},
		{
			name:          "no session",
			userScopes:    nil,
			checkScope:    "user:manage",
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.userScopes != nil {
				session := &oauth.Session{
					GrantedScopes: tt.userScopes,
				}
				ctx = context.WithValue(context.Background(), SessionKey, session)
			} else {
				ctx = context.Background()
			}

			result := HasScope(ctx, tt.checkScope)
			if result != tt.expectedMatch {
				t.Errorf("expected HasScope to return %v, got %v", tt.expectedMatch, result)
			}
		})
	}
}
