// Package errors provides standardized error handling across all ai-aas services.
//
// This file defines error codes matching the OpenAPI specification and provides
// HTTP status code mapping for consistent API responses.
package errors

import "net/http"

// Authentication errors (401)
const (
	// ErrCodeUnauthorized indicates the request lacks valid authentication.
	ErrCodeUnauthorized = "UNAUTHORIZED"
	// ErrCodeInvalidAPIKey indicates the provided API key is invalid or expired.
	ErrCodeInvalidAPIKey = "INVALID_API_KEY"
	// ErrCodeAuthInvalid indicates authentication failed for another reason.
	ErrCodeAuthInvalid = "AUTH_INVALID"
)

// Authorization errors (403)
const (
	// ErrCodeForbidden indicates the authenticated user lacks permission.
	ErrCodeForbidden = "FORBIDDEN"
	// ErrCodeAccessDenied indicates model access was denied (Spec 022).
	ErrCodeAccessDenied = "ACCESS_DENIED"
)

// Validation errors (400)
const (
	// ErrCodeInvalidRequest indicates the request format or content is invalid.
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	// ErrCodeMissingField indicates a required field is missing.
	ErrCodeMissingField = "MISSING_FIELD"
	// ErrCodeValidationError indicates one or more fields failed validation.
	ErrCodeValidationError = "VALIDATION_ERROR"
)

// Rate limiting (429)
const (
	// ErrCodeRateLimitExceeded indicates too many requests were made.
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
)

// Budget/quota errors (402)
const (
	// ErrCodeBudgetExceeded indicates the organization's budget was exceeded.
	ErrCodeBudgetExceeded = "BUDGET_EXCEEDED"
	// ErrCodeQuotaExceeded indicates a usage quota was exceeded.
	ErrCodeQuotaExceeded = "QUOTA_EXCEEDED"
)

// Backend errors (502, 503, 504)
const (
	// ErrCodeBackendUnavailable indicates the backend service is not available.
	ErrCodeBackendUnavailable = "BACKEND_UNAVAILABLE"
	// ErrCodeBackendTimeout indicates the backend service timed out.
	ErrCodeBackendTimeout = "BACKEND_TIMEOUT"
	// ErrCodeBackendError indicates the backend returned an error.
	ErrCodeBackendError = "BACKEND_ERROR"
)

// Routing errors (500, 503)
const (
	// ErrCodeNoBackendAvailable indicates no backend is available to handle the request.
	ErrCodeNoBackendAvailable = "NO_BACKEND_AVAILABLE"
	// ErrCodeRoutingError indicates an error occurred during request routing.
	ErrCodeRoutingError = "ROUTING_ERROR"
)

// Resource errors (404, 409)
const (
	// ErrCodeNotFound indicates the requested resource was not found.
	ErrCodeNotFound = "NOT_FOUND"
	// ErrCodeRequestNotFound indicates the specific request was not found.
	ErrCodeRequestNotFound = "REQUEST_NOT_FOUND"
	// ErrCodeConflict indicates a resource conflict (e.g., duplicate).
	ErrCodeConflict = "CONFLICT"
)

// Internal errors (500, 503)
const (
	// ErrCodeInternalError indicates an unexpected internal error occurred.
	ErrCodeInternalError = "INTERNAL_ERROR"
	// ErrCodeServiceUnavailable indicates the service is temporarily unavailable.
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// httpStatusMap maps error codes to HTTP status codes.
var httpStatusMap = map[string]int{
	// Authentication errors (401)
	ErrCodeUnauthorized:  http.StatusUnauthorized,
	ErrCodeInvalidAPIKey: http.StatusUnauthorized,
	ErrCodeAuthInvalid:   http.StatusUnauthorized,

	// Authorization errors (403)
	ErrCodeForbidden:    http.StatusForbidden,
	ErrCodeAccessDenied: http.StatusForbidden,

	// Validation errors (400)
	ErrCodeInvalidRequest:  http.StatusBadRequest,
	ErrCodeMissingField:    http.StatusBadRequest,
	ErrCodeValidationError: http.StatusBadRequest,

	// Rate limiting (429)
	ErrCodeRateLimitExceeded: http.StatusTooManyRequests,

	// Budget/quota (402)
	ErrCodeBudgetExceeded: http.StatusPaymentRequired,
	ErrCodeQuotaExceeded:  http.StatusPaymentRequired,

	// Backend errors (502, 503, 504)
	ErrCodeBackendUnavailable: http.StatusServiceUnavailable,
	ErrCodeBackendTimeout:     http.StatusGatewayTimeout,
	ErrCodeBackendError:       http.StatusBadGateway,

	// Routing errors (500, 503)
	ErrCodeNoBackendAvailable: http.StatusServiceUnavailable,
	ErrCodeRoutingError:       http.StatusInternalServerError,

	// Resource errors (404, 409)
	ErrCodeNotFound:        http.StatusNotFound,
	ErrCodeRequestNotFound: http.StatusNotFound,
	ErrCodeConflict:        http.StatusConflict,

	// Internal errors (500, 503)
	ErrCodeInternalError:      http.StatusInternalServerError,
	ErrCodeServiceUnavailable: http.StatusServiceUnavailable,
}

// GetHTTPStatus maps an error code to an HTTP status code.
func GetHTTPStatus(code string) int {
	if status, ok := httpStatusMap[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// IsRetryable returns true if the error code indicates a retryable condition.
// Clients should use exponential backoff when retrying these errors.
func IsRetryable(code string) bool {
	switch code {
	case ErrCodeBackendUnavailable,
		ErrCodeBackendTimeout,
		ErrCodeNoBackendAvailable,
		ErrCodeServiceUnavailable,
		ErrCodeRateLimitExceeded:
		return true
	default:
		return false
	}
}

// IsClientError returns true if the error code indicates a client error (4xx).
func IsClientError(code string) bool {
	status := GetHTTPStatus(code)
	return status >= 400 && status < 500
}

// IsServerError returns true if the error code indicates a server error (5xx).
func IsServerError(code string) bool {
	status := GetHTTPStatus(code)
	return status >= 500 && status < 600
}
