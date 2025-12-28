// Package middleware provides HTTP middleware for authentication and authorization.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
)

// tryAPIKeyAuth attempts to authenticate using the API key from the api_keys table.
// Returns (true, context) if authentication succeeds, (false, nil) otherwise.
// Supports both user and service_account principal types.
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
	// Support both user and service_account principal types
	var apiKeyID uuid.UUID
	var orgID uuid.UUID
	var principalID uuid.UUID
	var principalType string
	var status string
	var expiresAt *time.Time
	var scopes []string

	err := rt.Postgres.Pool().QueryRow(ctx, `
		SELECT api_key_id, org_id, principal_id, principal_type, status, expires_at, scopes
		FROM api_keys
		WHERE fingerprint = $1 AND principal_type IN ('user', 'service_account')
	`, fingerprint).Scan(&apiKeyID, &orgID, &principalID, &principalType, &status, &expiresAt, &scopes)

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
	// For service accounts, principalID is the service account ID
	// For users, principalID is the user ID
	ctx = context.WithValue(ctx, UserIDKey, principalID)
	ctx = context.WithValue(ctx, OrgIDKey, orgID)

	// Create a session for compatibility with existing code
	session := &oauth.Session{
		UserID:        principalID.String(),
		OrgID:         orgID.String(),
		GrantedScopes: scopes,
	}
	ctx = context.WithValue(ctx, SessionKey, session)

	logger.Info("RequireAuth: API key authentication successful",
		zap.String("path", path),
		zap.String("request_id", requestID),
		zap.String("principal_id", principalID.String()),
		zap.String("principal_type", principalType),
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
