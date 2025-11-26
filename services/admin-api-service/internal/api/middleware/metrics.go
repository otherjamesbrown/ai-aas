package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/handlers"
)

// Metrics creates a middleware that records RED metrics
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			endpoint := normalizeEndpoint(r.URL.Path)

			handlers.HTTPRequestsTotal.WithLabelValues(
				r.Method,
				endpoint,
				strconv.Itoa(wrapped.status),
			).Inc()

			handlers.HTTPRequestDuration.WithLabelValues(
				r.Method,
				endpoint,
			).Observe(duration)
		})
	}
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// normalizeEndpoint removes dynamic path segments for consistent metric labels
func normalizeEndpoint(path string) string {
	// Normalize common patterns
	// /v1/registry/models/some-model -> /v1/registry/models/{model_name}
	// /v1/organizations/uuid -> /v1/organizations/{org_id}
	// etc.

	// Simple implementation - in production would use route patterns
	return path
}

