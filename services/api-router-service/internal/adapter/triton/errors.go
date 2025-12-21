package triton

import (
	"net/http"
	"strings"
)

// Triton error status codes
const (
	TritonStatusUnavailable       = "UNAVAILABLE"
	TritonStatusInvalidArg        = "INVALID_ARG"
	TritonStatusNotFound          = "NOT_FOUND"
	TritonStatusResourceExhausted = "RESOURCE_EXHAUSTED"
	TritonStatusDeadlineExceeded  = "DEADLINE_EXCEEDED"
	TritonStatusInternal          = "INTERNAL"
	TritonStatusUnknown           = "UNKNOWN"
)

// OpenAI error types
const (
	OpenAIErrorTypeInvalidRequest    = "invalid_request_error"
	OpenAIErrorTypeRateLimit         = "rate_limit_exceeded"
	OpenAIErrorTypeServiceUnavailable = "service_unavailable"
	OpenAIErrorTypeTimeout           = "timeout"
	OpenAIErrorTypeModelNotFound     = "model_not_found"
	OpenAIErrorTypeInternal          = "internal_error"
)

// tritonToOpenAIErrorMap maps Triton status codes to OpenAI error types and HTTP status codes
var tritonToOpenAIErrorMap = map[string]struct {
	HTTPStatus int
	ErrorType  string
}{
	TritonStatusUnavailable:       {http.StatusServiceUnavailable, OpenAIErrorTypeServiceUnavailable},
	TritonStatusInvalidArg:        {http.StatusBadRequest, OpenAIErrorTypeInvalidRequest},
	TritonStatusNotFound:          {http.StatusNotFound, OpenAIErrorTypeModelNotFound},
	TritonStatusResourceExhausted: {http.StatusTooManyRequests, OpenAIErrorTypeRateLimit},
	TritonStatusDeadlineExceeded:  {http.StatusGatewayTimeout, OpenAIErrorTypeTimeout},
	TritonStatusInternal:          {http.StatusInternalServerError, OpenAIErrorTypeInternal},
	TritonStatusUnknown:           {http.StatusInternalServerError, OpenAIErrorTypeInternal},
}

// MapTritonError maps a Triton error to an OpenAI error response and HTTP status code.
// The tritonStatus can be a Triton status code or an error message containing the status.
// Returns the OpenAI error structure and the appropriate HTTP status code.
func MapTritonError(tritonStatus string, details string) (*OpenAIError, int) {
	// Normalize status - extract status code from error message if needed
	status := normalizeTritonStatus(tritonStatus)

	// Look up error mapping
	mapping, ok := tritonToOpenAIErrorMap[status]
	if !ok {
		// Default to internal error for unknown statuses
		mapping = tritonToOpenAIErrorMap[TritonStatusUnknown]
	}

	// Build user-friendly message
	message := buildErrorMessage(status, details)

	return &OpenAIError{
		Message:       message,
		Type:          mapping.ErrorType,
		Code:          status,
		TritonDetails: details,
	}, mapping.HTTPStatus
}

// normalizeTritonStatus extracts the Triton status code from an error string.
func normalizeTritonStatus(status string) string {
	status = strings.TrimSpace(status)
	status = strings.ToUpper(status)

	// Check if it's already a known status code
	for knownStatus := range tritonToOpenAIErrorMap {
		if status == knownStatus {
			return status
		}
		// Check if the status is contained in the error message
		if strings.Contains(status, knownStatus) {
			return knownStatus
		}
	}

	return TritonStatusUnknown
}

// buildErrorMessage creates a user-friendly error message.
func buildErrorMessage(status string, details string) string {
	switch status {
	case TritonStatusUnavailable:
		return "The model backend is temporarily unavailable. Please try again later."
	case TritonStatusInvalidArg:
		if details != "" {
			return "Invalid request: " + details
		}
		return "Invalid request parameters."
	case TritonStatusNotFound:
		return "The requested model was not found."
	case TritonStatusResourceExhausted:
		return "The model is currently overloaded. Please try again later."
	case TritonStatusDeadlineExceeded:
		return "The request timed out. Please try again with a smaller input."
	case TritonStatusInternal:
		return "An internal error occurred in the model backend."
	default:
		return "An unexpected error occurred."
	}
}

// MapHTTPStatusToTritonError maps an HTTP status code to a Triton-style error.
// This is useful for handling HTTP errors from the Triton backend.
func MapHTTPStatusToTritonError(httpStatus int, body string) (*OpenAIError, int) {
	var tritonStatus string

	switch {
	case httpStatus == http.StatusServiceUnavailable:
		tritonStatus = TritonStatusUnavailable
	case httpStatus == http.StatusBadRequest:
		tritonStatus = TritonStatusInvalidArg
	case httpStatus == http.StatusNotFound:
		tritonStatus = TritonStatusNotFound
	case httpStatus == http.StatusTooManyRequests:
		tritonStatus = TritonStatusResourceExhausted
	case httpStatus == http.StatusGatewayTimeout || httpStatus == http.StatusRequestTimeout:
		tritonStatus = TritonStatusDeadlineExceeded
	case httpStatus >= 500:
		tritonStatus = TritonStatusInternal
	default:
		tritonStatus = TritonStatusUnknown
	}

	return MapTritonError(tritonStatus, body)
}

// IsRetryableError returns true if the error is retryable.
func IsRetryableError(httpStatus int) bool {
	switch httpStatus {
	case http.StatusServiceUnavailable,
		http.StatusTooManyRequests,
		http.StatusGatewayTimeout,
		http.StatusRequestTimeout:
		return true
	default:
		return false
	}
}
