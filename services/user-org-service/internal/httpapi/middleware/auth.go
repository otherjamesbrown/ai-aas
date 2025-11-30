// Package middleware provides HTTP middleware for authentication and authorization.
//
// Purpose:
//
//	This package implements middleware for validating OAuth2 Bearer tokens,
//	extracting user context, and enforcing authentication on protected routes.
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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ory/fosite"
	"go.uber.org/zap"

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
// Supports both API key authentication and OAuth2/Fosite authentication.
// Returns 401 Unauthorized if token is missing, invalid, or expired.
func RequireAuth(rt *bootstrap.Runtime, logger *zap.Logger) func(http.Handler) http.Handler {
	if rt == nil {
		logger.Warn("Runtime not available, auth middleware will reject all requests")
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "authentication not configured", http.StatusInternalServerError)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = "unknown"
			}

			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			authPrefix := ""
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) > 0 {
					authPrefix = parts[0]
				}
			}
			logger.Debug("RequireAuth: checking authorization header",
				zap.String("path", r.URL.Path),
				zap.String("request_id", requestID),
				zap.Bool("has_auth_header", authHeader != ""),
				zap.String("auth_header_prefix", authPrefix))

			if authHeader == "" {
				logger.Warn("RequireAuth: missing authorization header",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID),
					zap.String("origin", r.Header.Get("Origin")),
					zap.String("user_agent", r.Header.Get("User-Agent")))
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Parse "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				logger.Warn("RequireAuth: invalid authorization header format",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID),
					zap.String("auth_header_prefix", parts[0]))
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			if token == "" {
				logger.Warn("RequireAuth: empty bearer token",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID))
				http.Error(w, "empty bearer token", http.StatusUnauthorized)
				return
			}

			tokenPrefix := token
			if len(token) > 10 {
				tokenPrefix = token[:10] + "..."
			}
			logger.Debug("RequireAuth: validating token",
				zap.String("path", r.URL.Path),
				zap.String("request_id", requestID),
				zap.Int("token_length", len(token)),
				zap.String("token_prefix", tokenPrefix))

			// Try API key authentication first (check api_keys table)
			authenticated, authCtx := tryAPIKeyAuth(ctx, rt, token, requestID, logger, r.URL.Path)
			if authenticated {
				logger.Debug("RequireAuth: API key authentication successful",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID))
				next.ServeHTTP(w, r.WithContext(authCtx))
				return
			}

			// Fall back to OAuth2/Fosite authentication
			if rt.Provider == nil {
				logger.Warn("RequireAuth: OAuth provider not available and API key auth failed",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID))
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Validate token using Fosite's introspection
			// IntrospectToken validates the token signature and returns session info
			_, accessRequester, err := rt.Provider.IntrospectToken(ctx, token, fosite.AccessToken, &oauth.Session{})
			if err != nil {
				logger.Warn("RequireAuth: token validation failed",
					zap.Error(err),
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID),
					zap.String("error_type", fmt.Sprintf("%T", err)))
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Check if token is active (accessRequester is nil if token is invalid)
			if accessRequester == nil {
				logger.Warn("RequireAuth: token is not active",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID))
				http.Error(w, "token is not active", http.StatusUnauthorized)
				return
			}

			logger.Debug("RequireAuth: OAuth token validated, extracting session",
				zap.String("path", r.URL.Path),
				zap.String("request_id", requestID))

			// Extract session information
			session, ok := accessRequester.GetSession().(*oauth.Session)
			if !ok {
				sessionType := fmt.Sprintf("%T", accessRequester.GetSession())
				logger.Warn("RequireAuth: invalid session type",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID),
					zap.String("session_type", sessionType))
				http.Error(w, "invalid session", http.StatusUnauthorized)
				return
			}

			// Extract user ID and org ID
			userIDStr := session.UserID
			if userIDStr == "" {
				userIDStr = session.Subject // Fallback to subject if user_id not set
			}

			logger.Debug("RequireAuth: extracted session info",
				zap.String("path", r.URL.Path),
				zap.String("request_id", requestID),
				zap.String("user_id", userIDStr),
				zap.String("org_id", session.OrgID),
				zap.String("subject", session.Subject))

			if userIDStr == "" {
				logger.Warn("RequireAuth: session missing user ID",
					zap.String("path", r.URL.Path),
					zap.String("request_id", requestID),
					zap.String("subject", session.Subject))
				http.Error(w, "invalid session: missing user ID", http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				logger.Warn("invalid user ID format",
					zap.Error(err),
					zap.String("user_id", userIDStr),
					zap.String("path", r.URL.Path))
				http.Error(w, "invalid session: invalid user ID", http.StatusUnauthorized)
				return
			}

			var orgID uuid.UUID
			if session.OrgID != "" {
				orgID, err = uuid.Parse(session.OrgID)
				if err != nil {
					logger.Warn("invalid org ID format",
						zap.Error(err),
						zap.String("org_id", session.OrgID),
						zap.String("path", r.URL.Path))
					http.Error(w, "invalid session: invalid org ID", http.StatusUnauthorized)
					return
				}
			}

			// Store authenticated context in request
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, OrgIDKey, orgID)
			ctx = context.WithValue(ctx, SessionKey, session)

			logger.Debug("RequireAuth: authentication successful, proceeding to handler",
				zap.String("path", r.URL.Path),
				zap.String("request_id", requestID),
				zap.String("user_id", userID.String()),
				zap.String("org_id", orgID.String()))

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

// HasScope checks if the authenticated user has a specific scope.
// Returns false if not authenticated or scope not found.
func HasScope(ctx context.Context, scope string) bool {
	session := GetSession(ctx)
	if session == nil || session.GrantedScopes == nil {
		return false
	}
	for _, s := range session.GrantedScopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the authenticated user has any of the specified scopes.
// Returns false if not authenticated or no matching scope found.
func HasAnyScope(ctx context.Context, scopes ...string) bool {
	for _, scope := range scopes {
		if HasScope(ctx, scope) {
			return true
		}
	}
	return false
}

// tryAPIKeyAuth attempts to authenticate using the API key from the api_keys table.
// Returns (true, context) if authentication succeeds, (false, nil) otherwise.
func tryAPIKeyAuth(ctx context.Context, rt *bootstrap.Runtime, token string, requestID string, logger *zap.Logger, path string) (bool, context.Context) {
	if rt.Postgres == nil {
		return false, nil
	}

	// Compute SHA-256 fingerprint of the API key
	hash := sha256.Sum256([]byte(token))
	fingerprint := base64.URLEncoding.EncodeToString(hash[:])
	fingerprint = strings.TrimRight(fingerprint, "=")

	logger.Debug("RequireAuth: trying API key authentication",
		zap.String("path", path),
		zap.String("request_id", requestID),
		zap.String("fingerprint", fingerprint[:min(20, len(fingerprint))]+"..."))

	// Look up API key in database by fingerprint
	var apiKeyID uuid.UUID
	var orgID uuid.UUID
	var userID uuid.UUID
	var status string
	var expiresAt *time.Time
	var scopes []string

	err := rt.Postgres.Pool().QueryRow(ctx, `
		SELECT api_key_id, org_id, principal_id, status, expires_at, scopes
		FROM api_keys
		WHERE fingerprint = $1 AND principal_type = 'user'
	`, fingerprint).Scan(&apiKeyID, &orgID, &userID, &status, &expiresAt, &scopes)

	if err != nil {
		if err.Error() != "no rows in result set" {
			logger.Debug("RequireAuth: API key lookup failed",
				zap.Error(err),
				zap.String("path", path),
				zap.String("request_id", requestID))
		}
		return false, nil
	}

	// Check if API key is active
	if status != "active" {
		logger.Debug("RequireAuth: API key not active",
			zap.String("path", path),
			zap.String("request_id", requestID),
			zap.String("status", status))
		return false, nil
	}

	// Check if API key has expired
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		logger.Debug("RequireAuth: API key expired",
			zap.String("path", path),
			zap.String("request_id", requestID),
			zap.Time("expires_at", *expiresAt))
		return false, nil
	}

	// Store authenticated context in request
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, OrgIDKey, orgID)

	// Create a session for compatibility with existing code
	session := &oauth.Session{
		UserID:        userID.String(),
		OrgID:         orgID.String(),
		GrantedScopes: scopes,
	}
	ctx = context.WithValue(ctx, SessionKey, session)

	logger.Info("RequireAuth: API key authentication successful",
		zap.String("path", path),
		zap.String("request_id", requestID),
		zap.String("user_id", userID.String()),
		zap.String("org_id", orgID.String()),
		zap.String("api_key_id", apiKeyID.String()),
		zap.Strings("scopes", scopes))

	return true, ctx
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
