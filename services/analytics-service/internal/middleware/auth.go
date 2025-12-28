// Package middleware provides HTTP middleware for the analytics service.
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	Logger            *zap.Logger
	MasterAdminAPIKey string
	// EnableAuth controls whether auth is enforced (default: true)
	// Set to false for development/testing
	EnableAuth bool
}

// Auth creates authentication middleware that validates Bearer tokens
// and sets X-Actor-* headers for downstream RBAC middleware.
//
// This middleware:
// 1. Extracts Bearer token from Authorization header
// 2. Validates against master admin API key
// 3. Sets X-Actor-Subject and X-Actor-Roles headers for RBAC
//
// If EnableAuth is false, it sets admin headers for all requests.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	if !cfg.EnableAuth {
		cfg.Logger.Warn("Auth middleware is disabled - all requests will have admin access")
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Set admin headers for all requests when auth is disabled
				r.Header.Set("X-Actor-Subject", "dev-user")
				r.Header.Set("X-Actor-Roles", "admin")
				next.ServeHTTP(w, r)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}

			// Expect "Bearer <token>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}

			token := parts[1]
			if token == "" {
				writeUnauthorized(w, "empty API key")
				return
			}

			// Validate against master admin API key
			if cfg.MasterAdminAPIKey == "" {
				cfg.Logger.Error("MASTER_ADMIN_API_KEY not configured")
				writeUnauthorized(w, "authentication not configured")
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.MasterAdminAPIKey)) != 1 {
				writeUnauthorized(w, "invalid or revoked API key")
				return
			}

			// Set X-Actor-* headers for downstream RBAC middleware
			r.Header.Set("X-Actor-Subject", "master-admin")
			r.Header.Set("X-Actor-Roles", "admin")

			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + message + `","code":"UNAUTHORIZED"}`))
}
