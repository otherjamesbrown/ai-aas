// Package public provides admin API proxy handlers.
//
// This file implements a reverse proxy for admin endpoints that forwards
// requests to the user-org-service with JWT authentication.

package public

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// HandleAdminProxy proxies admin API requests to the user-org-service.
// This handler acts as a reverse proxy for portal admin operations.
// It forwards requests as-is with JWT authentication via Authorization header.
//
// Proxied endpoints:
//   - /organizations/me/* - Organization profile and API keys
//   - /members/* - Team member management
//   - /feature-flags - Feature flag configuration
//   - /support/* - Support operations like impersonation
func (h *Handler) HandleAdminProxy(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "admin.proxy")
	defer span.End()

	// Get the user-org-service URL from handler config
	userOrgServiceURL := h.getUserOrgServiceURL()
	if userOrgServiceURL == "" {
		h.logger.Error("user-org-service URL not configured")
		http.Error(w, `{"error":"Service configuration error","code":"CONFIG_ERROR"}`, http.StatusInternalServerError)
		return
	}

	// Verify JWT authentication is present
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.logger.Debug("admin proxy request missing Authorization header",
			zap.String("path", r.URL.Path),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Authorization required","code":"AUTH_REQUIRED"}`))
		return
	}

	// Build the target URL - map the path to user-org-service's API
	// The portal calls /organizations/me/api-keys
	// User-org-service has convenience routes at /organizations/me/* (no /v1 prefix)
	// These routes are registered directly in the router for frontend compatibility
	path := r.URL.Path
	// Don't modify paths in noV1PrefixPaths - they're already correct
	// Only add /v1 prefix for other paths that need it
	if len(path) > 0 && path[0] == '/' && !hasV1Prefix(path) && !shouldSkipV1Prefix(path) {
		path = "/v1" + path
	}

	targetURL := userOrgServiceURL + path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	h.logger.Debug("proxying admin request",
		zap.String("method", r.Method),
		zap.String("original_path", r.URL.Path),
		zap.String("target", targetURL),
	)

	// Read the request body into memory to avoid chunked encoding issues
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error("failed to read request body", zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"Failed to read request body","code":"INTERNAL_ERROR"}`))
			return
		}
		defer r.Body.Close()
	}

	// Create the proxy request with the body buffer
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		h.logger.Error("failed to create proxy request", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Failed to create proxy request","code":"INTERNAL_ERROR"}`))
		return
	}

	// Set Content-Length explicitly to match the body size
	proxyReq.ContentLength = int64(len(bodyBytes))

	// Copy headers from original request
	for name, values := range r.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Ensure Authorization header is forwarded
	proxyReq.Header.Set("Authorization", authHeader)

	// Set timeout for the proxy request
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send the request to user-org-service
	resp, err := h.httpClient.Do(proxyReq.WithContext(reqCtx))
	if err != nil {
		h.logger.Error("admin proxy request failed",
			zap.String("target", targetURL),
			zap.Error(err),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"Admin service unavailable","code":"SERVICE_UNAVAILABLE"}`))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Copy response headers
	for name, values := range resp.Header {
		// Skip hop-by-hop and CORS headers (CORS is handled by api-router middleware)
		if shouldSkipHeader(name) {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Ensure Content-Type is set
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	// Copy the response body
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		h.logger.Error("failed to copy response body",
			zap.Error(err),
			zap.Int64("bytes_written", written),
		)
		return
	}

	h.logger.Debug("admin proxy request completed",
		zap.String("path", r.URL.Path),
		zap.Int("status", resp.StatusCode),
		zap.Int64("bytes", written),
	)
}

// HandleFeatureFlags returns default feature flags.
// This is a stub endpoint that returns sensible defaults when feature flags
// are not configured via a dedicated service.
func (h *Handler) HandleFeatureFlags(w http.ResponseWriter, r *http.Request) {
	_, span := h.tracer.Start(r.Context(), "feature-flags")
	defer span.End()

	// Return default feature flags
	// These are reasonable defaults for the portal
	defaultFlags := map[string]bool{
		"budget-controls":    true,
		"impersonation":      false, // Disabled by default for security
		"usage-insights":     true,
		"api-key-management": true,
		"member-invites":     true,
		"dark-mode":          true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(defaultFlags)
}

// HandleImpersonationStatus returns the current impersonation status.
// Returns empty/not-found when impersonation is not active.
func (h *Handler) HandleImpersonationStatus(w http.ResponseWriter, r *http.Request) {
	_, span := h.tracer.Start(r.Context(), "impersonation.status")
	defer span.End()

	// For now, impersonation is not implemented - return 404
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"No active impersonation session","code":"NOT_FOUND"}`))
}

// noV1PrefixPaths lists path prefixes that should NOT have /v1 prepended.
// These routes are registered directly in user-org-service without the /v1 prefix.
// To add new exceptions, simply append to this slice.
var noV1PrefixPaths = []string{
	"/organizations/me",
}

// hasV1Prefix checks if a path already has the /v1 prefix
func hasV1Prefix(path string) bool {
	return len(path) >= 3 && path[:3] == "/v1"
}

// shouldSkipV1Prefix checks if a path matches any prefix in noV1PrefixPaths.
// Returns true if the path should NOT have /v1 prepended.
func shouldSkipV1Prefix(path string) bool {
	for _, prefix := range noV1PrefixPaths {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
