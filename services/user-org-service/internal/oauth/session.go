// Package oauth (session.go) defines the custom Fosite session type with
// user-org service specific fields (org_id, user_id).
//
// Purpose:
//   This file extends fosite.DefaultSession to include organization and user
//   identifiers required for multi-tenant authorization. The session is
//   serialized to JSON and stored in the oauth_sessions table, allowing
//   token introspection and authorization decisions to include org context.
//
// Dependencies:
//   - github.com/ory/fosite: DefaultSession base type and Session interface
//
// Key Responsibilities:
//   - Session embeds DefaultSession and adds OrgID, UserID, GrantedScopes
//   - Clone creates a deep copy for Fosite's session handling
//   - GetJWTClaims includes org_id and user_id in JWT token claims
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-005 (OAuth2 Support)
//   - specs/005-user-org-service/spec.md#FR-002 (Multi-tenant Isolation)
//
// Debugging Notes:
//   - Session is serialized to JSONB in oauth_sessions.session_data column
//   - Clone must properly clone the embedded DefaultSession (Fosite requirement)
//   - JWT claims include org_id and user_id for downstream authorization
//   - GrantedScopes tracks which scopes were actually granted (may differ from requested)
//
// Thread Safety:
//   - Session struct is not thread-safe (cloned before use in concurrent contexts)
//
// Error Handling:
//   - Clone returns nil if session is nil (defensive)
package oauth

import (
	"github.com/ory/fosite"
)

// Session extends fosite's DefaultSession with user-org specific metadata.
// This session type is used throughout the OAuth2 flow and stored in the
// oauth_sessions table. The org_id and user_id fields enable multi-tenant
// authorization decisions.
type Session struct {
	fosite.DefaultSession
	OrgID         string   `json:"org_id,omitempty"`         // Organization UUID (required for multi-tenant isolation)
	UserID        string   `json:"user_id,omitempty"`        // User UUID (subject of the OAuth2 session)
	GrantedScopes []string `json:"granted_scopes,omitempty"` // Scopes actually granted (may differ from requested)
}

// Clone returns a deep copy of the session, including user-org specific fields.
// Required by fosite.Session interface. Properly clones the embedded
// DefaultSession and all custom fields to avoid shared state issues.
func (s *Session) Clone() fosite.Session {
	if s == nil {
		return nil
	}

	clone := *s
	if ds, ok := s.DefaultSession.Clone().(*fosite.DefaultSession); ok && ds != nil {
		clone.DefaultSession = *ds
	}
	if len(s.GrantedScopes) > 0 {
		clone.GrantedScopes = append([]string{}, s.GrantedScopes...)
	}
	return &clone
}
