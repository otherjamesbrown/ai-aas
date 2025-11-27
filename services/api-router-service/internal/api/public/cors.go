package public

import (
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// CORSConfig holds the configuration for CORS middleware.
type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	MaxAge           int
	AllowCredentials bool
	Logger           *zap.Logger
}

// CORSMiddleware returns a middleware that handles CORS preflight and actual requests.
// This middleware should be registered FIRST on the router to handle OPTIONS requests
// before any other middleware processes the request.
func CORSMiddleware(cfg CORSConfig) func(http.Handler) http.Handler {
	// Pre-compute allowed origins map for O(1) lookup
	allowedOriginsMap := make(map[string]bool)
	allowAllOrigins := false
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			allowAllOrigins = true
			break
		}
		allowedOriginsMap[origin] = true
	}

	// Pre-compute header strings
	allowedMethodsStr := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeadersStr := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeadersStr := strings.Join(cfg.ExposedHeaders, ", ")
	maxAgeStr := strconv.Itoa(cfg.MaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			originAllowed := false
			if origin != "" {
				if allowAllOrigins {
					originAllowed = true
				} else if allowedOriginsMap[origin] {
					originAllowed = true
				}
			}

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				if originAllowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", allowedMethodsStr)
					w.Header().Set("Access-Control-Allow-Headers", allowedHeadersStr)
					w.Header().Set("Access-Control-Max-Age", maxAgeStr)
					if cfg.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
					if exposedHeadersStr != "" {
						w.Header().Set("Access-Control-Expose-Headers", exposedHeadersStr)
					}

					if cfg.Logger != nil {
						cfg.Logger.Debug("CORS preflight handled",
							zap.String("origin", origin),
							zap.String("method", r.Header.Get("Access-Control-Request-Method")),
							zap.String("headers", r.Header.Get("Access-Control-Request-Headers")),
						)
					}
				} else if cfg.Logger != nil && origin != "" {
					cfg.Logger.Warn("CORS preflight rejected: origin not allowed",
						zap.String("origin", origin),
						zap.Strings("allowed_origins", cfg.AllowedOrigins),
					)
				}

				// Always return 204 for OPTIONS to avoid confusing clients
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Handle actual request - add CORS headers if origin is allowed
			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if exposedHeadersStr != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeadersStr)
				}
			}

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// ParseCORSOrigins parses a comma-separated list of origins.
func ParseCORSOrigins(origins string) []string {
	if origins == "" {
		return nil
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// ParseCORSMethods parses a comma-separated list of HTTP methods.
func ParseCORSMethods(methods string) []string {
	if methods == "" {
		return []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	parts := strings.Split(methods, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, strings.ToUpper(part))
		}
	}
	return result
}

// ParseCORSHeaders parses a comma-separated list of headers.
func ParseCORSHeaders(headers string) []string {
	if headers == "" {
		return nil
	}
	parts := strings.Split(headers, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
