// Package middleware provides HTTP middleware for logging and observability.
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ai-aas/shared-go/observability"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// RequestLoggerConfig configures the request logging middleware.
type RequestLoggerConfig struct {
	// SkipPaths excludes paths from logging (e.g., health checks)
	SkipPaths []string
	// SensitiveHeaders won't be logged (headers to redact)
	SensitiveHeaders []string
	// LogRequestHeaders enables logging of request headers
	LogRequestHeaders bool
	// LogResponseHeaders enables logging of response headers
	LogResponseHeaders bool
}

// DefaultRequestLoggerConfig returns sensible defaults.
func DefaultRequestLoggerConfig() RequestLoggerConfig {
	return RequestLoggerConfig{
		SkipPaths: []string{
			"/healthz",
			"/readyz",
			"/metrics",
			"/v1/status/healthz",
			"/v1/status/readyz",
		},
		SensitiveHeaders: []string{
			"Authorization",
			"X-API-Key",
			"Cookie",
			"Set-Cookie",
			"X-Auth-Token",
		},
		LogRequestHeaders:  false,
		LogResponseHeaders: false,
	}
}

// RequestLogger returns middleware that logs HTTP requests and responses.
// It integrates with OpenTelemetry trace context and the observability middleware.
// Accepts *zap.Logger directly for flexibility with different logging setups.
func RequestLogger(logger *zap.Logger, config RequestLoggerConfig) func(http.Handler) http.Handler {
	// Build skip path set for O(1) lookup
	skipSet := make(map[string]bool)
	for _, p := range config.SkipPaths {
		skipSet[p] = true
	}

	// Build sensitive header set for O(1) lookup
	sensitiveSet := make(map[string]bool)
	for _, h := range config.SensitiveHeaders {
		sensitiveSet[strings.ToLower(h)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip configured paths
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Extract request ID from context (set by RequestContextMiddleware)
			requestID, _ := observability.RequestIDFromContext(r.Context())

			// Create logger with trace context
			reqLogger := loggerWithContext(logger, r.Context())
			if requestID != "" {
				reqLogger = reqLogger.With(zap.String("request_id", requestID))
			}

			// Build log fields for request
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("host", r.Host),
			}

			if r.URL.RawQuery != "" {
				fields = append(fields, zap.String("query", r.URL.RawQuery))
			}

			// Optionally log headers
			if config.LogRequestHeaders {
				headers := filterHeaders(r.Header, sensitiveSet)
				if len(headers) > 0 {
					fields = append(fields, zap.Any("request_headers", headers))
				}
			}

			// Log request start at debug level
			reqLogger.Debug("request_started", fields...)

			// Wrap response writer to capture status and size
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Execute handler
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Build response log fields
			responseFields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.statusCode),
				zap.Duration("duration", duration),
				zap.Int64("response_bytes", wrapped.bytesWritten),
			}

			if requestID != "" {
				responseFields = append(responseFields, zap.String("request_id", requestID))
			}

			// Log at appropriate level based on status code
			switch {
			case wrapped.statusCode >= 500:
				reqLogger.Error("request_completed", responseFields...)
			case wrapped.statusCode >= 400:
				reqLogger.Warn("request_completed", responseFields...)
			default:
				reqLogger.Info("request_completed", responseFields...)
			}
		})
	}
}

// filterHeaders returns a map of headers with sensitive values redacted.
func filterHeaders(headers http.Header, sensitiveSet map[string]bool) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if sensitiveSet[strings.ToLower(k)] {
			result[k] = "[REDACTED]"
		} else if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write captures bytes written and ensures header is written.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// Flush implements http.Flusher if the underlying writer supports it.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for compatibility with middleware.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// loggerWithContext returns a logger with OpenTelemetry trace context fields added.
func loggerWithContext(logger *zap.Logger, ctx context.Context) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return logger
	}

	spanCtx := span.SpanContext()
	return logger.With(
		zap.String("trace_id", spanCtx.TraceID().String()),
		zap.String("span_id", spanCtx.SpanID().String()),
	)
}
