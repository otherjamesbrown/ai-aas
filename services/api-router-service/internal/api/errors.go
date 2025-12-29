// Package api provides centralized error handling for the API Router Service.
//
// Purpose:
//
//	This package provides service-specific error handling utilities that work
//	with the shared error codes from github.com/ai-aas/shared-go/errors.
//	It provides ErrorBuilder for creating error responses with trace context,
//	and helper functions for writing error responses to HTTP ResponseWriter.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"

	sharedErrors "github.com/ai-aas/shared-go/errors"
)

// Re-export shared error codes for backward compatibility.
// This allows existing code to continue using api.ErrCodeXxx while migrating.
const (
	// Authentication errors (401)
	ErrCodeUnauthorized  = sharedErrors.ErrCodeUnauthorized
	ErrCodeInvalidAPIKey = sharedErrors.ErrCodeInvalidAPIKey
	ErrCodeAuthInvalid   = sharedErrors.ErrCodeAuthInvalid

	// Authorization errors (403)
	ErrCodeForbidden    = sharedErrors.ErrCodeForbidden
	ErrCodeAccessDenied = sharedErrors.ErrCodeAccessDenied // Model access denied (Spec 022)

	// Validation errors (400)
	ErrCodeInvalidRequest  = sharedErrors.ErrCodeInvalidRequest
	ErrCodeMissingField    = sharedErrors.ErrCodeMissingField
	ErrCodeValidationError = sharedErrors.ErrCodeValidationError

	// Rate limiting (429)
	ErrCodeRateLimitExceeded = sharedErrors.ErrCodeRateLimitExceeded

	// Budget/quota (402)
	ErrCodeBudgetExceeded = sharedErrors.ErrCodeBudgetExceeded
	ErrCodeQuotaExceeded  = sharedErrors.ErrCodeQuotaExceeded

	// Backend errors (502, 503, 504)
	ErrCodeBackendUnavailable = sharedErrors.ErrCodeBackendUnavailable
	ErrCodeBackendTimeout     = sharedErrors.ErrCodeBackendTimeout
	ErrCodeBackendError       = sharedErrors.ErrCodeBackendError

	// Routing errors (500, 503)
	ErrCodeNoBackendAvailable = sharedErrors.ErrCodeNoBackendAvailable
	ErrCodeRoutingError       = sharedErrors.ErrCodeRoutingError

	// Not found (404)
	ErrCodeNotFound        = sharedErrors.ErrCodeNotFound
	ErrCodeRequestNotFound = sharedErrors.ErrCodeRequestNotFound

	// Internal errors (500, 503)
	ErrCodeInternalError      = sharedErrors.ErrCodeInternalError
	ErrCodeServiceUnavailable = sharedErrors.ErrCodeServiceUnavailable
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
		Error:             err.Error(),
		Code:              code,
		RetryAfterSeconds: retryAfter,
		LimitContext:      limitContext,
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
// This function delegates to the shared errors package.
func GetHTTPStatus(code string) int {
	return sharedErrors.GetHTTPStatus(code)
}

// MapError maps an internal error to an error code and HTTP status.
// This function uses typed error checking first, then falls back to string matching
// for errors that don't implement the APIError interface.
func MapError(err error) (code string, statusCode int) {
	if err == nil {
		return ErrCodeInternalError, http.StatusInternalServerError
	}

	// First, try to extract error code from typed APIError using errors.As
	// This handles both direct APIError instances and wrapped errors (e.g., fmt.Errorf("%w", apiErr))
	// This is more robust than string matching as it works with error wrapping
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code, GetHTTPStatus(apiErr.Code)
	}

	// Fallback to string matching for errors that don't implement typed errors
	// This is less robust but necessary for compatibility with third-party errors
	errStr := err.Error()
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
	response := builder.BuildError(r.Context(), err, code)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Encode JSON response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Fallback to plain text if JSON encoding fails
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
	}
}

// WriteLimitError writes a limit error response to the HTTP response writer.
func WriteLimitError(w http.ResponseWriter, r *http.Request, builder *ErrorBuilder, err error, code string, retryAfter *int, limitContext map[string]interface{}) {
	statusCode := GetHTTPStatus(code)
	response := builder.BuildLimitError(r.Context(), err, code, retryAfter, limitContext)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Encode JSON response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Fallback to plain text if JSON encoding fails
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
	}
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

// Is implements the errors.Is interface for error comparison.
// This allows errors.Is(err, target) to work correctly.
func (e *APIError) Is(target error) bool {
	if target == nil {
		return false
	}
	t, ok := target.(*APIError)
	if !ok {
		return false
	}
	// Compare by code, allowing nil code to match any code
	if t.Code != "" && e.Code != "" {
		return t.Code == e.Code
	}
	// If target has no code specified, match by message
	return t.Message == e.Message || (t.Message == "" && e.Code == t.Code)
}

// IsAPIError checks if an error is an APIError.
func IsAPIError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr)
}

// GetErrorCode extracts the error code from an error.
func GetErrorCode(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
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
