package errors

import (
	"net/http"
	"testing"
)

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		code     string
		expected int
	}{
		// Authentication errors (401)
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeInvalidAPIKey, http.StatusUnauthorized},
		{ErrCodeAuthInvalid, http.StatusUnauthorized},

		// Authorization errors (403)
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeAccessDenied, http.StatusForbidden},

		// Validation errors (400)
		{ErrCodeInvalidRequest, http.StatusBadRequest},
		{ErrCodeMissingField, http.StatusBadRequest},
		{ErrCodeValidationError, http.StatusBadRequest},

		// Rate limiting (429)
		{ErrCodeRateLimitExceeded, http.StatusTooManyRequests},

		// Budget/quota (402)
		{ErrCodeBudgetExceeded, http.StatusPaymentRequired},
		{ErrCodeQuotaExceeded, http.StatusPaymentRequired},

		// Backend errors
		{ErrCodeBackendUnavailable, http.StatusServiceUnavailable},
		{ErrCodeBackendTimeout, http.StatusGatewayTimeout},
		{ErrCodeBackendError, http.StatusBadGateway},

		// Routing errors
		{ErrCodeNoBackendAvailable, http.StatusServiceUnavailable},
		{ErrCodeRoutingError, http.StatusInternalServerError},

		// Resource errors
		{ErrCodeNotFound, http.StatusNotFound},
		{ErrCodeRequestNotFound, http.StatusNotFound},
		{ErrCodeConflict, http.StatusConflict},

		// Internal errors
		{ErrCodeInternalError, http.StatusInternalServerError},
		{ErrCodeServiceUnavailable, http.StatusServiceUnavailable},

		// Unknown code should return 500
		{"UNKNOWN_CODE", http.StatusInternalServerError},
		{"", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := GetHTTPStatus(tt.code)
			if got != tt.expected {
				t.Errorf("GetHTTPStatus(%q) = %d, want %d", tt.code, got, tt.expected)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	retryableCodes := []string{
		ErrCodeBackendUnavailable,
		ErrCodeBackendTimeout,
		ErrCodeNoBackendAvailable,
		ErrCodeServiceUnavailable,
		ErrCodeRateLimitExceeded,
	}

	nonRetryableCodes := []string{
		ErrCodeUnauthorized,
		ErrCodeInvalidAPIKey,
		ErrCodeForbidden,
		ErrCodeInvalidRequest,
		ErrCodeNotFound,
		ErrCodeConflict,
		ErrCodeInternalError,
		ErrCodeBackendError,
	}

	for _, code := range retryableCodes {
		t.Run(code+"_retryable", func(t *testing.T) {
			if !IsRetryable(code) {
				t.Errorf("IsRetryable(%q) = false, want true", code)
			}
		})
	}

	for _, code := range nonRetryableCodes {
		t.Run(code+"_not_retryable", func(t *testing.T) {
			if IsRetryable(code) {
				t.Errorf("IsRetryable(%q) = true, want false", code)
			}
		})
	}
}

func TestIsClientError(t *testing.T) {
	clientErrorCodes := []string{
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeInvalidRequest,
		ErrCodeNotFound,
		ErrCodeConflict,
		ErrCodeRateLimitExceeded,
		ErrCodeBudgetExceeded,
	}

	serverErrorCodes := []string{
		ErrCodeInternalError,
		ErrCodeServiceUnavailable,
		ErrCodeBackendError,
		ErrCodeBackendTimeout,
	}

	for _, code := range clientErrorCodes {
		t.Run(code+"_client", func(t *testing.T) {
			if !IsClientError(code) {
				t.Errorf("IsClientError(%q) = false, want true", code)
			}
		})
	}

	for _, code := range serverErrorCodes {
		t.Run(code+"_not_client", func(t *testing.T) {
			if IsClientError(code) {
				t.Errorf("IsClientError(%q) = true, want false", code)
			}
		})
	}
}

func TestIsServerError(t *testing.T) {
	serverErrorCodes := []string{
		ErrCodeInternalError,
		ErrCodeServiceUnavailable,
		ErrCodeBackendError,
		ErrCodeBackendTimeout,
		ErrCodeBackendUnavailable,
		ErrCodeRoutingError,
		ErrCodeNoBackendAvailable,
	}

	clientErrorCodes := []string{
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeInvalidRequest,
		ErrCodeNotFound,
		ErrCodeConflict,
	}

	for _, code := range serverErrorCodes {
		t.Run(code+"_server", func(t *testing.T) {
			if !IsServerError(code) {
				t.Errorf("IsServerError(%q) = false, want true", code)
			}
		})
	}

	for _, code := range clientErrorCodes {
		t.Run(code+"_not_server", func(t *testing.T) {
			if IsServerError(code) {
				t.Errorf("IsServerError(%q) = true, want false", code)
			}
		})
	}
}
