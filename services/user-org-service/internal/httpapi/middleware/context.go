// Package middleware provides HTTP middleware for authentication and authorization.
package middleware

import (
	"context"

	"github.com/google/uuid"

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
