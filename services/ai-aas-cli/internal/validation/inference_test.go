// Package validation provides model validation functionality.
package validation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInferenceValidator_ValidateEndpoint_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": "test-123",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "test-model",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "Hello!"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	validator := NewInferenceValidator()
	result := validator.ValidateEndpoint(context.Background(), server.URL, "test-api-key")

	assert.True(t, result.Passed)
	assert.NotEmpty(t, result.ResponseTime)
}

func TestInferenceValidator_ValidateEndpoint_Timeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	validator := NewInferenceValidator()
	validator.Timeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result := validator.ValidateEndpoint(ctx, server.URL, "test-api-key")

	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "timeout")
}

func TestInferenceValidator_ValidateEndpoint_InvalidResponse(t *testing.T) {
	// Create server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	validator := NewInferenceValidator()
	result := validator.ValidateEndpoint(context.Background(), server.URL, "test-api-key")

	assert.False(t, result.Passed)
}

func TestInferenceValidator_ValidateEndpoint_HTTPError(t *testing.T) {
	// Create server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	validator := NewInferenceValidator()
	result := validator.ValidateEndpoint(context.Background(), server.URL, "test-api-key")

	assert.False(t, result.Passed)
	assert.Equal(t, 500, result.StatusCode)
}

func TestInferenceResult_String(t *testing.T) {
	result := &InferenceResult{
		Passed:       true,
		StatusCode:   200,
		ResponseTime: 150 * time.Millisecond,
		Message:      "Success",
	}

	str := result.String()
	assert.Contains(t, str, "PASS")
	assert.Contains(t, str, "150ms")
}

func TestNewInferenceValidator(t *testing.T) {
	validator := NewInferenceValidator()

	require.NotNil(t, validator)
	assert.Equal(t, DefaultInferenceTimeout, validator.Timeout)
	assert.NotNil(t, validator.Client)
}
