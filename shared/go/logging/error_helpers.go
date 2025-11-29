package logging

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"github.com/ai-aas/shared-go/errors"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ErrorLogger provides structured error logging with automatic context extraction.
type ErrorLogger struct {
	logger *Logger
}

// NewErrorLogger creates an ErrorLogger wrapper around the base logger.
func NewErrorLogger(logger *Logger) *ErrorLogger {
	return &ErrorLogger{logger: logger}
}

// LogError logs an error with full context including stack trace location.
func (el *ErrorLogger) LogError(ctx context.Context, err error, msg string, fields ...zap.Field) {
	if err == nil {
		return
	}

	// Get caller info for stack trace context
	_, file, line, ok := runtime.Caller(1)
	if ok {
		fields = append(fields, zap.String("caller_file", file), zap.Int("caller_line", line))
	}

	// Extract trace context
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		spanCtx := span.SpanContext()
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	// Handle shared error type
	if sharedErr, ok := err.(*errors.Error); ok {
		fields = append(fields,
			zap.String("error_code", sharedErr.Code),
			zap.String("error_message", sharedErr.Message),
		)
		if sharedErr.Detail != "" {
			fields = append(fields, zap.String("error_detail", sharedErr.Detail))
		}
		if sharedErr.RequestID != "" {
			fields = append(fields, zap.String("request_id", sharedErr.RequestID))
		}
	} else {
		fields = append(fields, zap.Error(err))
	}

	el.logger.Error(msg, fields...)
}

// LogWarn logs a warning with context.
func (el *ErrorLogger) LogWarn(ctx context.Context, msg string, fields ...zap.Field) {
	// Extract trace context
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		spanCtx := span.SpanContext()
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	el.logger.Warn(msg, fields...)
}

// LogHTTPError logs an HTTP error response with request context.
func (el *ErrorLogger) LogHTTPError(ctx context.Context, r *http.Request, statusCode int, err error, fields ...zap.Field) {
	fields = append(fields,
		zap.Int("status_code", statusCode),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("remote_addr", r.RemoteAddr),
	)

	// Add user agent
	if ua := r.Header.Get("User-Agent"); ua != "" {
		fields = append(fields, zap.String("user_agent", ua))
	}

	// Determine log level based on status code
	msg := "HTTP error response"
	if statusCode >= 500 {
		el.LogError(ctx, err, msg, fields...)
	} else if statusCode >= 400 {
		fields = append(fields, zap.Error(err))
		el.LogWarn(ctx, msg, fields...)
	}
}

// LogPanic logs a panic with full recovery context.
func (el *ErrorLogger) LogPanic(ctx context.Context, recovered interface{}, stack []byte, fields ...zap.Field) {
	fields = append(fields,
		zap.Any("panic_value", recovered),
		zap.ByteString("stack_trace", stack),
	)

	// Extract trace context
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		spanCtx := span.SpanContext()
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	el.logger.Error("panic recovered", fields...)
}

// LogDatabaseError logs database operation errors with query context.
func (el *ErrorLogger) LogDatabaseError(ctx context.Context, operation string, err error, fields ...zap.Field) {
	fields = append(fields,
		zap.String("db_operation", operation),
	)
	el.LogError(ctx, err, "database error", fields...)
}

// LogExternalServiceError logs errors from external service calls.
func (el *ErrorLogger) LogExternalServiceError(ctx context.Context, serviceName, endpoint string, statusCode int, err error, fields ...zap.Field) {
	fields = append(fields,
		zap.String("external_service", serviceName),
		zap.String("external_endpoint", endpoint),
		zap.Int("external_status_code", statusCode),
	)
	el.LogError(ctx, err, "external service error", fields...)
}

// LogAuthError logs authentication/authorization errors.
func (el *ErrorLogger) LogAuthError(ctx context.Context, authType, reason string, fields ...zap.Field) {
	fields = append(fields,
		zap.String("auth_type", authType),
		zap.String("auth_failure_reason", reason),
	)
	el.LogWarn(ctx, "authentication/authorization failure", fields...)
}

// LogValidationError logs input validation errors.
func (el *ErrorLogger) LogValidationError(ctx context.Context, field, issue string, fields ...zap.Field) {
	fields = append(fields,
		zap.String("validation_field", field),
		zap.String("validation_issue", issue),
	)
	el.LogWarn(ctx, "validation error", fields...)
}

// RecoverMiddleware returns a middleware that recovers from panics and logs them.
func (el *ErrorLogger) RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				stack := make([]byte, 4096)
				n := runtime.Stack(stack, false)
				stack = stack[:n]

				el.LogPanic(r.Context(), recovered, stack,
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
				)

				// Return 500 error
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"error":"internal server error","code":"INTERNAL"}`)))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
