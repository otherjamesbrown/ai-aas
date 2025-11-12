// Package api provides centralized error handling for the API Router Service.
//
// Purpose:
//   This package provides a centralized error catalog with consistent error codes,
//   response formatting, and HTTP status code mapping across all API endpoints.
//
// Error Codes:
//   Error codes match the OpenAPI specification and follow a consistent naming
//   convention. Each error code maps to a specific HTTP status code.
//
package api

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// Error codes matching OpenAPI spec
const (
	// Authentication errors (401)
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeInvalidAPIKey   = "INVALID_API_KEY"
	ErrCodeAuthInvalid     = "AUTH_INVALID"

	// Authorization errors (403)
	ErrCodeForbidden = "FORBIDDEN"

	// Validation errors (400)
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	ErrCodeMissingField   = "MISSING_FIELD"
	ErrCodeValidationError = "VALIDATION_ERROR"

	// Rate limiting (429)
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"

	// Budget/quota (402)
	ErrCodeBudgetExceeded = "BUDGET_EXCEEDED"
	ErrCodeQuotaExceeded  = "QUOTA_EXCEEDED"

	// Backend errors (502, 503, 504)
	ErrCodeBackendUnavailable = "BACKEND_UNAVAILABLE"
	ErrCodeBackendTimeout     = "BACKEND_TIMEOUT"
	ErrCodeBackendError       = "BACKEND_ERROR"

	// Routing errors (500, 503)
	ErrCodeNoBackendAvailable = "NO_BACKEND_AVAILABLE"
	ErrCodeRoutingError       = "ROUTING_ERROR"

	// Not found (404)
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeRequestNotFound = "REQUEST_NOT_FOUND"

	// Internal errors (500, 503)
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// ErrorResponse represents a standard error response matching OpenAPI spec.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	TraceID string `json:"trace_id,omitempty"`
}

// LimitErrorResponse represents a limit error response with additional context.
type LimitErrorResponse struct {
	Error             string                 `json:"error"`
	Code              string                 `json:"code"`
	TraceID           string                 `json:"trace_id,omitempty"`
	RetryAfterSeconds *int                   `json:"retry_after_seconds,omitempty"`
	LimitContext      map[string]interface{} `json:"limit_context,omitempty"`
}

// ErrorBuilder provides methods for building error responses.
type ErrorBuilder struct {
	tracer trace.Tracer
}

// NewErrorBuilder creates a new error builder.
func NewErrorBuilder(tracer trace.Tracer) *ErrorBuilder {
	return &ErrorBuilder{
		tracer: tracer,
	}
}

// BuildError creates an ErrorResponse from an error and code.
func (b *ErrorBuilder) BuildError(ctx context.Context, err error, code string) *ErrorResponse {
	response := &ErrorResponse{
		Error: err.Error(),
		Code:  code,
	}

	// Add trace ID if available
	if b.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			response.TraceID = span.SpanContext().TraceID().String()
		}
	}

	return response
}

// BuildLimitError creates a LimitErrorResponse with limit context.
func (b *ErrorBuilder) BuildLimitError(ctx context.Context, err error, code string, retryAfter *int, limitContext map[string]interface{}) *LimitErrorResponse {
	response := &LimitErrorResponse{
		Error:        err.Error(),
		Code:         code,
		RetryAfterSeconds: retryAfter,
		LimitContext: limitContext,
	}

	// Add trace ID if available
	if b.tracer != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			response.TraceID = span.SpanContext().TraceID().String()
		}
	}

	return response
}

// GetHTTPStatus maps an error code to an HTTP status code.
func GetHTTPStatus(code string) int {
	switch code {
	// Authentication errors
	case ErrCodeUnauthorized, ErrCodeInvalidAPIKey, ErrCodeAuthInvalid:
		return http.StatusUnauthorized

	// Authorization errors
	case ErrCodeForbidden:
		return http.StatusForbidden

	// Validation errors
	case ErrCodeInvalidRequest, ErrCodeMissingField, ErrCodeValidationError:
		return http.StatusBadRequest

	// Rate limiting
	case ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests

	// Budget/quota
	case ErrCodeBudgetExceeded, ErrCodeQuotaExceeded:
		return http.StatusPaymentRequired

	// Backend errors
	case ErrCodeBackendUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeBackendTimeout:
		return http.StatusGatewayTimeout
	case ErrCodeBackendError:
		return http.StatusBadGateway

	// Routing errors
	case ErrCodeNoBackendAvailable:
		return http.StatusServiceUnavailable
	case ErrCodeRoutingError:
		return http.StatusInternalServerError

	// Not found
	case ErrCodeNotFound, ErrCodeRequestNotFound:
		return http.StatusNotFound

	// Internal errors
	case ErrCodeInternalError:
		return http.StatusInternalServerError
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable

	default:
		return http.StatusInternalServerError
	}
}

// MapError maps an internal error to an error code and HTTP status.
// This function provides intelligent error mapping based on error content.
func MapError(err error) (code string, statusCode int) {
	if err == nil {
		return ErrCodeInternalError, http.StatusInternalServerError
	}

	errStr := err.Error()

	// Map common error patterns to error codes
	switch {
	case contains(errStr, "authentication") || contains(errStr, "unauthorized") || contains(errStr, "invalid api key"):
		return ErrCodeUnauthorized, http.StatusUnauthorized
	case contains(errStr, "forbidden") || contains(errStr, "permission"):
		return ErrCodeForbidden, http.StatusForbidden
	case contains(errStr, "validation") || contains(errStr, "invalid request") || contains(errStr, "missing"):
		return ErrCodeInvalidRequest, http.StatusBadRequest
	case contains(errStr, "rate limit") || contains(errStr, "too many requests"):
		return ErrCodeRateLimitExceeded, http.StatusTooManyRequests
	case contains(errStr, "budget") || contains(errStr, "quota"):
		return ErrCodeBudgetExceeded, http.StatusPaymentRequired
	case contains(errStr, "backend unavailable") || contains(errStr, "no backend"):
		return ErrCodeBackendUnavailable, http.StatusServiceUnavailable
	case contains(errStr, "timeout"):
		return ErrCodeBackendTimeout, http.StatusGatewayTimeout
	case contains(errStr, "not found"):
		return ErrCodeNotFound, http.StatusNotFound
	case contains(errStr, "routing"):
		return ErrCodeRoutingError, http.StatusInternalServerError
	default:
		return ErrCodeInternalError, http.StatusInternalServerError
	}
}

// WriteError writes an error response to the HTTP response writer.
func WriteError(w http.ResponseWriter, r *http.Request, builder *ErrorBuilder, err error, code string) {
	statusCode := GetHTTPStatus(code)
	builder.BuildError(r.Context(), err, code)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// Note: Error encoding is handled by the caller or middleware
	// This function just sets up the response structure
}

// WriteLimitError writes a limit error response to the HTTP response writer.
func WriteLimitError(w http.ResponseWriter, r *http.Request, builder *ErrorBuilder, err error, code string, retryAfter *int, limitContext map[string]interface{}) {
	statusCode := GetHTTPStatus(code)
	builder.BuildLimitError(r.Context(), err, code, retryAfter, limitContext)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// Note: Error encoding is handled by the caller or middleware
	// This function just sets up the response structure
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// NewError creates a new error with a specific error code.
func NewError(code, message string) error {
	return &APIError{
		Code:    code,
		Message: message,
	}
}

// APIError represents an API error with a specific code.
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

// IsAPIError checks if an error is an APIError.
func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

// GetErrorCode extracts the error code from an error.
func GetErrorCode(err error) string {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code
	}
	code, _ := MapError(err)
	return code
}

// WrapError wraps an error with an error code.
func WrapError(err error, code string) error {
	if err == nil {
		return nil
	}
	return &APIError{
		Code:    code,
		Message: err.Error(),
	}
}

