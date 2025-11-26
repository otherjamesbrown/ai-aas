// Package public provides authentication proxy handlers.
//
// This file implements a reverse proxy for authentication endpoints (/v1/auth/*)
// that forwards requests to the user-org-service.

package public

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// HandleAuthProxy proxies all /v1/auth/* requests to the user-org-service.
// This handler acts as a reverse proxy, forwarding requests as-is and returning the response.
func (h *Handler) HandleAuthProxy(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "auth.proxy")
	defer span.End()

	// Get the user-org-service URL from handler config
	userOrgServiceURL := h.getUserOrgServiceURL()
	if userOrgServiceURL == "" {
		h.logger.Error("user-org-service URL not configured")
		http.Error(w, "Service configuration error", http.StatusInternalServerError)
		return
	}

	// Build the target URL by replacing the path
	// Request path is like "/v1/auth/login", we want to forward to user-org-service with same path
	targetURL := userOrgServiceURL + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	h.logger.Debug("proxying auth request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("target", targetURL),
	)

	// Read the request body into memory to avoid chunked encoding issues
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error("failed to read request body", zap.Error(err))
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
	}

	// Create the proxy request with the body buffer
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		h.logger.Error("failed to create proxy request", zap.Error(err))
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
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

	// Set timeout for the proxy request
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Send the request to user-org-service
	resp, err := h.httpClient.Do(proxyReq.WithContext(reqCtx))
	if err != nil {
		h.logger.Error("proxy request failed",
			zap.String("target", targetURL),
			zap.Error(err),
		)
		http.Error(w, "Authentication service unavailable", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Copy response headers
	for name, values := range resp.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
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

	h.logger.Debug("auth proxy request completed",
		zap.String("path", r.URL.Path),
		zap.Int("status", resp.StatusCode),
		zap.Int64("bytes", written),
	)
}

// getUserOrgServiceURL returns the URL for the user-org-service.
// This can be configured via environment variable or handler field.
func (h *Handler) getUserOrgServiceURL() string {
	// Check if explicitly configured in handler (for testing)
	if h.userOrgServiceURL != "" {
		return h.userOrgServiceURL
	}

	// Default to Kubernetes service DNS name
	// This is set via USER_ORG_SERVICE_URL environment variable in values-development.yaml
	// Format: http://service-name.namespace.svc.cluster.local:port
	return "http://user-org-service-development-user-org-service.user-org-service.svc.cluster.local:8081"
}

// isHopByHopHeader checks if a header is hop-by-hop and should not be proxied.
// Based on RFC 2616 Section 13.5.1
func isHopByHopHeader(name string) bool {
	hopByHopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	return hopByHopHeaders[strings.Title(strings.ToLower(name))]
}
