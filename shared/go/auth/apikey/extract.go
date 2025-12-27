package apikey

import (
	"net/http"
	"strings"
)

// ExtractAPIKey extracts an API key from HTTP request headers.
// It checks the X-API-Key header first, then falls back to Authorization: Bearer.
func ExtractAPIKey(r *http.Request) string {
	// Check X-API-Key header first (preferred)
	if key := r.Header.Get("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}

	// Fallback to Authorization header: Bearer <key>
	return ExtractBearerToken(r)
}

// ExtractBearerToken extracts a Bearer token from the Authorization header.
// Returns an empty string if the header is missing or malformed.
func ExtractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

// HasAPIKey returns true if the request contains an API key
// in either the X-API-Key header or Authorization: Bearer header.
func HasAPIKey(r *http.Request) bool {
	return ExtractAPIKey(r) != ""
}

// HeaderName is the standard API key header name.
const HeaderName = "X-API-Key"

// AuthorizationScheme is the standard Bearer scheme for Authorization header.
const AuthorizationScheme = "Bearer"
