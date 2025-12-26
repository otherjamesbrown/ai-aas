package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type contextKey string

const (
	// APIKeyContextKey is the context key for the authenticated API key ID
	APIKeyContextKey contextKey = "api_key_id"
)

// APIKeyValidator validates API keys
type APIKeyValidator interface {
	ValidateKey(ctx context.Context, key string) (keyID string, valid bool, err error)
}

// Auth creates an authentication middleware
func Auth(validator APIKeyValidator, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			// Expect "Bearer <key>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}

			apiKey := parts[1]
			if apiKey == "" {
				writeUnauthorized(w, "empty API key")
				return
			}

			// Validate the API key
			keyID, valid, err := validator.ValidateKey(r.Context(), apiKey)
			if err != nil {
				logger.Error("API key validation error", zap.Error(err))
				writeUnauthorized(w, "authentication failed")
				return
			}

			if !valid {
				writeUnauthorized(w, "invalid or revoked API key")
				return
			}

			// Add key ID to context
			ctx := context.WithValue(r.Context(), APIKeyContextKey, keyID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIKeyID retrieves the API key ID from context
func GetAPIKeyID(ctx context.Context) string {
	if keyID, ok := ctx.Value(APIKeyContextKey).(string); ok {
		return keyID
	}
	return ""
}

// ConstantTimeCompare performs a constant-time string comparison
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"type":"https://docs.otherjamesbrown.com/errors/unauthorized","title":"Unauthorized","status":401,"detail":"` + message + `"}`))
}

