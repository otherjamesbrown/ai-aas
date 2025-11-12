// Package middleware provides HTTP middleware for authentication and authorization.
//
// Purpose:
//   This package implements middleware for validating OAuth2 Bearer tokens,
//   extracting user context, and enforcing authentication on protected routes.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router middleware
//   - github.com/google/uuid: UUID parsing for user/org IDs
//   - github.com/ory/fosite: OAuth2 token validation
//   - internal/bootstrap: Runtime dependencies (OAuth provider)
//   - internal/oauth: Session type with user/org context
//
// Key Responsibilities:
//   - Extract Bearer token from Authorization header
//   - Validate token using Fosite provider
//   - Extract user ID, org ID, and scopes from session
//   - Store authenticated context in request context
//   - Return 401 Unauthorized for invalid/missing tokens
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-005 (OAuth2 Support)
//   - specs/005-user-org-service/spec.md#FR-002 (Multi-tenant Isolation)
//
// Debugging Notes:
//   - Middleware extracts token from "Authorization: Bearer <token>" header
//   - Token signature is validated against oauth_sessions table
//   - Session includes org_id and user_id for multi-tenant isolation
//   - Context key is "auth.user" for extracting authenticated user
//
// Thread Safety:
//   - Middleware is stateless and safe for concurrent use
//
// Error Handling:
//   - Missing token returns 401 Unauthorized
//   - Invalid token returns 401 Unauthorized
//   - Expired token returns 401 Unauthorized
//   - Database errors return 500 Internal Server Error
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/ory/fosite"
	"github.com/rs/zerolog"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
)

// ContextKey is the type for context keys.
type ContextKey string

const (
	// UserIDKey is the context key for authenticated user ID.
	UserIDKey ContextKey = "auth.user_id"
	// OrgIDKey is the context key for authenticated organization ID.
	OrgIDKey ContextKey = "auth.org_id"
	// SessionKey is the context key for the full OAuth session.
	SessionKey ContextKey = "auth.session"
)

// AuthenticatedUser contains information about the authenticated user.
type AuthenticatedUser struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Scopes []string
}

// RequireAuth creates middleware that validates Bearer tokens and extracts user context.
// Returns 401 Unauthorized if token is missing, invalid, or expired.
func RequireAuth(rt *bootstrap.Runtime, logger zerolog.Logger) func(http.Handler) http.Handler {
	if rt == nil || rt.Provider == nil {
		logger.Warn().Msg("OAuth provider not available, auth middleware will reject all requests")
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "authentication not configured", http.StatusInternalServerError)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug().Str("path", r.URL.Path).Msg("missing authorization header")
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Parse "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				logger.Debug().Str("path", r.URL.Path).Msg("invalid authorization header format")
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			if token == "" {
				logger.Debug().Str("path", r.URL.Path).Msg("empty bearer token")
				http.Error(w, "empty bearer token", http.StatusUnauthorized)
				return
			}

			// Validate token using Fosite's introspection
			// IntrospectToken validates the token signature and returns session info
			_, accessRequester, err := rt.Provider.IntrospectToken(ctx, token, fosite.AccessToken, &oauth.Session{})
			if err != nil {
				logger.Debug().Err(err).Str("path", r.URL.Path).Msg("token validation failed")
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Check if token is active (accessRequester is nil if token is invalid)
			if accessRequester == nil {
				logger.Debug().Str("path", r.URL.Path).Msg("token is not active")
				http.Error(w, "token is not active", http.StatusUnauthorized)
				return
			}

			// Extract session information
			session, ok := accessRequester.GetSession().(*oauth.Session)
			if !ok {
				logger.Warn().Str("path", r.URL.Path).Msg("invalid session type")
				http.Error(w, "invalid session", http.StatusUnauthorized)
				return
			}

			// Extract user ID and org ID
			userIDStr := session.UserID
			if userIDStr == "" {
				userIDStr = session.Subject // Fallback to subject if user_id not set
			}

			if userIDStr == "" {
				logger.Warn().Str("path", r.URL.Path).Msg("session missing user ID")
				http.Error(w, "invalid session: missing user ID", http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				logger.Warn().Err(err).Str("user_id", userIDStr).Str("path", r.URL.Path).Msg("invalid user ID format")
				http.Error(w, "invalid session: invalid user ID", http.StatusUnauthorized)
				return
			}

			var orgID uuid.UUID
			if session.OrgID != "" {
				orgID, err = uuid.Parse(session.OrgID)
				if err != nil {
					logger.Warn().Err(err).Str("org_id", session.OrgID).Str("path", r.URL.Path).Msg("invalid org ID format")
					http.Error(w, "invalid session: invalid org ID", http.StatusUnauthorized)
					return
				}
			}

			// Store authenticated context in request
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, OrgIDKey, orgID)
			ctx = context.WithValue(ctx, SessionKey, session)

			// Continue to next handler with authenticated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the authenticated user ID from the request context.
// Returns uuid.Nil if not authenticated (should not happen if RequireAuth middleware is used).
func GetUserID(ctx context.Context) uuid.UUID {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return userID
}

// GetOrgID extracts the authenticated organization ID from the request context.
// Returns uuid.Nil if not authenticated or org ID not set.
func GetOrgID(ctx context.Context) uuid.UUID {
	orgID, ok := ctx.Value(OrgIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return orgID
}

// GetSession extracts the full OAuth session from the request context.
// Returns nil if not authenticated.
func GetSession(ctx context.Context) *oauth.Session {
	session, ok := ctx.Value(SessionKey).(*oauth.Session)
	if !ok {
		return nil
	}
	return session
}

// GetAuthenticatedUser extracts all authenticated user information from context.
func GetAuthenticatedUser(ctx context.Context) *AuthenticatedUser {
	session := GetSession(ctx)
	if session == nil {
		return nil
	}

	scopes := make([]string, 0)
	if session.GrantedScopes != nil {
		scopes = session.GrantedScopes
	}

	return &AuthenticatedUser{
		UserID: GetUserID(ctx),
		OrgID:  GetOrgID(ctx),
		Scopes: scopes,
	}
}

