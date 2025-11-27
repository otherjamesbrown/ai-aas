package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
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
			endpoint := normalizeEndpoint(r)

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

// normalizeEndpoint uses chi's route pattern for low-cardinality metric labels
func normalizeEndpoint(r *http.Request) string {
	// Use chi's routing pattern to get a low-cardinality endpoint label
	routeCtx := chi.RouteContext(r.Context())
	if routeCtx != nil {
		if pattern := routeCtx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	// Fallback for routes not found, to prevent high cardinality
	return "unknown"
}
