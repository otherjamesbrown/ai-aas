package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestRequireAuthAndAdminScopeChain tests the integration of RequireAuth + RequireAdminScope middleware chain.
// This test validates that the two middleware work together correctly:
// 1. RequireAuth extracts the token and sets session context
// 2. RequireAdminScope reads the session and enforces scopes
func TestRequireAuthAndAdminScopeChain(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		sessionScopes  []string
		requiredScopes []string
		expectStatus   int
		expectHandler  bool
		description    string
	}{
		{
			name:           "no auth header",
			authHeader:     "",
			sessionScopes:  nil,
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusUnauthorized,
			expectHandler:  false,
			description:    "Missing Authorization header should return 401",
		},
		{
			name:           "invalid auth format",
			authHeader:     "InvalidFormat",
			sessionScopes:  nil,
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusUnauthorized,
			expectHandler:  false,
			description:    "Invalid Authorization format should return 401",
		},
		{
			name:           "valid auth but no admin scope",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{"org:read"},
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusForbidden,
			expectHandler:  false,
			description:    "Valid auth without required scope should return 403",
		},
		{
			name:           "valid auth with matching scope",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{"user:manage"},
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusOK,
			expectHandler:  true,
			description:    "Valid auth with matching scope should pass through",
		},
		{
			name:           "valid auth with admin scope (universal grant)",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{"admin"},
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusOK,
			expectHandler:  true,
			description:    "Admin scope should grant access to all endpoints",
		},
		{
			name:           "valid auth with wildcard scope (universal grant)",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{"*"},
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusOK,
			expectHandler:  true,
			description:    "Wildcard scope should grant access to all endpoints",
		},
		{
			name:           "valid auth with one of multiple required scopes",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{"org:admin"},
			requiredScopes: []string{"user:manage", "org:admin"},
			expectStatus:   http.StatusOK,
			expectHandler:  true,
			description:    "Having one of multiple required scopes should grant access",
		},
		{
			name:           "valid auth with multiple scopes including required",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{"org:read", "user:manage", "apikey:manage"},
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusOK,
			expectHandler:  true,
			description:    "Multiple scopes including required should grant access",
		},
		{
			name:           "valid auth with empty scopes",
			authHeader:     "Bearer valid-token",
			sessionScopes:  []string{},
			requiredScopes: []string{"user:manage"},
			expectStatus:   http.StatusForbidden,
			expectHandler:  false,
			description:    "Empty scopes should be denied even with valid auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler that should only be called if both middleware pass
			handlerCalled := false
			finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Create a mock RequireAuth middleware that simulates authentication
			// In real scenarios, RequireAuth would validate tokens against database or OAuth provider
			mockRequireAuth := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					authHeader := r.Header.Get("Authorization")

					// No auth header
					if authHeader == "" {
						w.WriteHeader(http.StatusUnauthorized)
						w.Write([]byte(`{"error":"missing authorization header"}`))
						return
					}

					// Invalid format
					parts := strings.SplitN(authHeader, " ", 2)
					if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
						w.WriteHeader(http.StatusUnauthorized)
						w.Write([]byte(`{"error":"invalid authorization header format"}`))
						return
					}

					token := parts[1]
					if token == "" {
						w.WriteHeader(http.StatusUnauthorized)
						w.Write([]byte(`{"error":"empty bearer token"}`))
						return
					}

					// For this test, any non-empty bearer token is considered valid
					// Create session context with the test scopes
					session := &oauth.Session{
						UserID:        "test-user-id",
						OrgID:         "test-org-id",
						GrantedScopes: tt.sessionScopes,
					}

					ctx := context.WithValue(r.Context(), UserIDKey, "test-user-uuid")
					ctx = context.WithValue(ctx, OrgIDKey, "test-org-uuid")
					ctx = context.WithValue(ctx, SessionKey, session)

					next.ServeHTTP(w, r.WithContext(ctx))
				})
			}

			// Build middleware chain: RequireAuth → RequireAdminScope → handler
			handler := mockRequireAuth(RequireAdminScope(nil, tt.requiredScopes...)(finalHandler))

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Execute request
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Verify status code
			if rr.Code != tt.expectStatus {
				t.Errorf("%s: expected status %d, got %d", tt.description, tt.expectStatus, rr.Code)
			}

			// Verify handler was called (or not)
			if handlerCalled != tt.expectHandler {
				t.Errorf("%s: expected handler called=%v, got %v", tt.description, tt.expectHandler, handlerCalled)
			}
		})
	}
}

// TestRequireAuthAndAdminScopeChain_ContextPropagation tests that context values
// set by RequireAuth are properly propagated through RequireAdminScope to the final handler.
func TestRequireAuthAndAdminScopeChain_ContextPropagation(t *testing.T) {
	// Mock RequireAuth that sets context values
	mockRequireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := &oauth.Session{
				UserID:        "user-123",
				OrgID:         "org-456",
				GrantedScopes: []string{"admin"},
			}

			ctx := context.WithValue(r.Context(), UserIDKey, "test-user-uuid")
			ctx = context.WithValue(ctx, OrgIDKey, "test-org-uuid")
			ctx = context.WithValue(ctx, SessionKey, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Final handler that checks context values
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r.Context())
		if session == nil {
			t.Error("Expected session to be set in context, got nil")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if session.UserID != "user-123" {
			t.Errorf("Expected UserID=user-123, got %s", session.UserID)
		}

		if session.OrgID != "org-456" {
			t.Errorf("Expected OrgID=org-456, got %s", session.OrgID)
		}

		if len(session.GrantedScopes) != 1 || session.GrantedScopes[0] != "admin" {
			t.Errorf("Expected scopes=[admin], got %v", session.GrantedScopes)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("context propagated correctly"))
	})

	// Build middleware chain
	handler := mockRequireAuth(RequireAdminScope(nil, "user:manage")(finalHandler))

	// Execute request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}
