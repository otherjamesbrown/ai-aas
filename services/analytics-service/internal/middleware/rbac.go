// Package middleware provides HTTP middleware for the analytics service.
//
// Purpose:
//   This package provides RBAC middleware that integrates with shared/go/auth
//   to enforce role-based access control on analytics API endpoints.
//
// Dependencies:
//   - github.com/otherjamesbrown/ai-aas/shared/go/auth: Shared authorization middleware
//   - go.uber.org/zap: Structured logging
package middleware

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/otherjamesbrown/ai-aas/shared/go/auth"
	"go.uber.org/zap"
)

// RBACConfig holds configuration for RBAC middleware.
type RBACConfig struct {
	Logger *zap.Logger
	// EnableRBAC controls whether RBAC is enforced (default: true)
	// Set to false for development/testing
	EnableRBAC bool
}

// analyticsPolicy defines role-based access policies for analytics endpoints.
// Paths use {id} placeholder for UUIDs (normalized from actual request paths).
var analyticsPolicy = map[string][]string{
	// Usage API
	"GET:/analytics/v1/orgs/{id}/usage": {
		"analytics:usage:read",
		"admin",
	},
	// Reliability API
	"GET:/analytics/v1/orgs/{id}/reliability": {
		"analytics:reliability:read",
		"admin",
	},
	// Export API - Create
	"POST:/analytics/v1/orgs/{id}/exports": {
		"analytics:exports:create",
		"admin",
	},
	// Export API - List
	"GET:/analytics/v1/orgs/{id}/exports": {
		"analytics:exports:read",
		"admin",
	},
	// Export API - Get
	"GET:/analytics/v1/orgs/{id}/exports/{id}": {
		"analytics:exports:read",
		"admin",
	},
	// Export API - Download
	"GET:/analytics/v1/orgs/{id}/exports/{id}/download": {
		"analytics:exports:download",
		"admin",
	},
}

// buildPolicyEngine creates an auth.Engine from the analytics policy.
func buildPolicyEngine() (*auth.Engine, error) {
	policy := auth.Policy{
		Rules: analyticsPolicy,
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	return auth.LoadPolicy(strings.NewReader(string(policyJSON)))
}

// uuidRegex matches UUIDs in paths
var uuidRegex = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizePath normalizes request paths by replacing UUIDs with placeholders.
// This allows the policy engine to match routes with path parameters.
func normalizePath(path string) string {
	// Replace UUIDs with {id} placeholder
	normalized := uuidRegex.ReplaceAllString(path, "{id}")
	return normalized
}

// RBAC creates RBAC middleware for analytics endpoints.
// If EnableRBAC is false, it returns a no-op middleware (for development).
func RBAC(cfg RBACConfig) func(http.Handler) http.Handler {
	if !cfg.EnableRBAC {
		cfg.Logger.Warn("RBAC middleware is disabled - all requests will be allowed")
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	engine, err := buildPolicyEngine()
	if err != nil {
		cfg.Logger.Fatal("failed to build RBAC policy engine", zap.Error(err))
	}

	// Use header-based actor extraction (can be extended to support token-based)
	extractor := auth.HeaderExtractor

	// Wrap the shared auth middleware with path normalization
	return func(next http.Handler) http.Handler {
		baseMiddleware := auth.Middleware(engine, extractor)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Normalize path before passing to middleware
			// Create a new request with normalized path for policy matching
			normalizedPath := normalizePath(r.URL.Path)
			
			// The auth middleware uses r.Method + ":" + r.URL.Path
			// We need to temporarily modify the path for policy lookup
			originalPath := r.URL.Path
			r.URL.Path = normalizedPath
			
			// Call the base middleware
			baseMiddleware(next).ServeHTTP(w, r)
			
			// Restore original path (though it's probably not needed after ServeHTTP)
			r.URL.Path = originalPath
		})
	}
}

