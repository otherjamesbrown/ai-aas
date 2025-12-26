package triton

import (
	"net/http"
	"testing"
)

func TestMapTritonError(t *testing.T) {
	tests := []struct {
		name           string
		tritonStatus   string
		details        string
		wantHTTPStatus int
		wantErrorType  string
	}{
		{
			name:           "unavailable",
			tritonStatus:   TritonStatusUnavailable,
			details:        "backend not ready",
			wantHTTPStatus: http.StatusServiceUnavailable,
			wantErrorType:  OpenAIErrorTypeServiceUnavailable,
		},
		{
			name:           "invalid argument",
			tritonStatus:   TritonStatusInvalidArg,
			details:        "invalid tensor shape",
			wantHTTPStatus: http.StatusBadRequest,
			wantErrorType:  OpenAIErrorTypeInvalidRequest,
		},
		{
			name:           "not found",
			tritonStatus:   TritonStatusNotFound,
			details:        "model not found",
			wantHTTPStatus: http.StatusNotFound,
			wantErrorType:  OpenAIErrorTypeModelNotFound,
		},
		{
			name:           "resource exhausted",
			tritonStatus:   TritonStatusResourceExhausted,
			details:        "GPU memory full",
			wantHTTPStatus: http.StatusTooManyRequests,
			wantErrorType:  OpenAIErrorTypeRateLimit,
		},
		{
			name:           "deadline exceeded",
			tritonStatus:   TritonStatusDeadlineExceeded,
			details:        "timeout after 30s",
			wantHTTPStatus: http.StatusGatewayTimeout,
			wantErrorType:  OpenAIErrorTypeTimeout,
		},
		{
			name:           "internal error",
			tritonStatus:   TritonStatusInternal,
			details:        "CUDA error",
			wantHTTPStatus: http.StatusInternalServerError,
			wantErrorType:  OpenAIErrorTypeInternal,
		},
		{
			name:           "unknown error",
			tritonStatus:   "SOME_UNKNOWN_ERROR",
			details:        "something went wrong",
			wantHTTPStatus: http.StatusInternalServerError,
			wantErrorType:  OpenAIErrorTypeInternal,
		},
		{
			name:           "status in error message",
			tritonStatus:   "Error: UNAVAILABLE - backend not ready",
			details:        "",
			wantHTTPStatus: http.StatusServiceUnavailable,
			wantErrorType:  OpenAIErrorTypeServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err, httpStatus := MapTritonError(tt.tritonStatus, tt.details)

			if httpStatus != tt.wantHTTPStatus {
				t.Errorf("MapTritonError() httpStatus = %d, want %d", httpStatus, tt.wantHTTPStatus)
			}

			if err.Type != tt.wantErrorType {
				t.Errorf("MapTritonError() error type = %s, want %s", err.Type, tt.wantErrorType)
			}

			if err.Message == "" {
				t.Error("MapTritonError() error message should not be empty")
			}

			// Details should be preserved if provided
			if tt.details != "" && err.TritonDetails != tt.details {
				t.Errorf("MapTritonError() triton_details = %s, want %s", err.TritonDetails, tt.details)
			}
		})
	}
}

func TestMapHTTPStatusToTritonError(t *testing.T) {
	tests := []struct {
		name           string
		httpStatus     int
		body           string
		wantHTTPStatus int
		wantErrorType  string
	}{
		{
			name:           "service unavailable",
			httpStatus:     http.StatusServiceUnavailable,
			body:           "backend unavailable",
			wantHTTPStatus: http.StatusServiceUnavailable,
			wantErrorType:  OpenAIErrorTypeServiceUnavailable,
		},
		{
			name:           "bad request",
			httpStatus:     http.StatusBadRequest,
			body:           "invalid input",
			wantHTTPStatus: http.StatusBadRequest,
			wantErrorType:  OpenAIErrorTypeInvalidRequest,
		},
		{
			name:           "not found",
			httpStatus:     http.StatusNotFound,
			body:           "model not found",
			wantHTTPStatus: http.StatusNotFound,
			wantErrorType:  OpenAIErrorTypeModelNotFound,
		},
		{
			name:           "too many requests",
			httpStatus:     http.StatusTooManyRequests,
			body:           "rate limited",
			wantHTTPStatus: http.StatusTooManyRequests,
			wantErrorType:  OpenAIErrorTypeRateLimit,
		},
		{
			name:           "gateway timeout",
			httpStatus:     http.StatusGatewayTimeout,
			body:           "timeout",
			wantHTTPStatus: http.StatusGatewayTimeout,
			wantErrorType:  OpenAIErrorTypeTimeout,
		},
		{
			name:           "request timeout",
			httpStatus:     http.StatusRequestTimeout,
			body:           "timeout",
			wantHTTPStatus: http.StatusGatewayTimeout,
			wantErrorType:  OpenAIErrorTypeTimeout,
		},
		{
			name:           "internal server error",
			httpStatus:     http.StatusInternalServerError,
			body:           "internal error",
			wantHTTPStatus: http.StatusInternalServerError,
			wantErrorType:  OpenAIErrorTypeInternal,
		},
		{
			name:           "unknown 5xx error",
			httpStatus:     http.StatusBadGateway,
			body:           "bad gateway",
			wantHTTPStatus: http.StatusInternalServerError,
			wantErrorType:  OpenAIErrorTypeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err, httpStatus := MapHTTPStatusToTritonError(tt.httpStatus, tt.body)

			if httpStatus != tt.wantHTTPStatus {
				t.Errorf("MapHTTPStatusToTritonError() httpStatus = %d, want %d", httpStatus, tt.wantHTTPStatus)
			}

			if err.Type != tt.wantErrorType {
				t.Errorf("MapHTTPStatusToTritonError() error type = %s, want %s", err.Type, tt.wantErrorType)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		httpStatus int
		want       bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusNotFound, false},
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, true},
		{http.StatusTooManyRequests, true},
		{http.StatusGatewayTimeout, true},
		{http.StatusRequestTimeout, true},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.httpStatus), func(t *testing.T) {
			got := IsRetryableError(tt.httpStatus)
			if got != tt.want {
				t.Errorf("IsRetryableError(%d) = %v, want %v", tt.httpStatus, got, tt.want)
			}
		})
	}
}

func TestNormalizeTritonStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"UNAVAILABLE", TritonStatusUnavailable},
		{"unavailable", TritonStatusUnavailable},
		{"  INVALID_ARG  ", TritonStatusInvalidArg},
		{"Error: RESOURCE_EXHAUSTED - out of memory", TritonStatusResourceExhausted},
		{"some random error", TritonStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeTritonStatus(tt.input)
			if got != tt.want {
				t.Errorf("normalizeTritonStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
