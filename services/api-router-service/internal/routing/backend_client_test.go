package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

func TestNewBackendClient(t *testing.T) {
	logger := zaptest.NewLogger(t)
	timeout := 5 * time.Second

	client := NewBackendClient(logger, timeout)

	if client == nil {
		t.Fatal("NewBackendClient returned nil")
	}

	if client.httpClient == nil {
		t.Error("httpClient is nil")
	}

	if client.httpClient.Timeout != timeout {
		t.Errorf("timeout = %v, want %v", client.httpClient.Timeout, timeout)
	}

	if client.logger == nil {
		t.Error("logger is nil")
	}
}

func TestBackendClient_ForwardRequest(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name           string
		backendHandler http.HandlerFunc
		request        *BackendRequest
		wantErr        bool
		wantResponse   *BackendResponse
		errContains    string
	}{
		{
			name: "successful request",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
				}

				resp := map[string]interface{}{
					"text":        "Hello, world!",
					"tokens_used": 10,
					"metadata": map[string]interface{}{
						"model": "test-model",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(resp)
			},
			request: &BackendRequest{
				Prompt:      "Test prompt",
				MaxTokens:   100,
				Temperature: 0.7,
			},
			wantErr: false,
			wantResponse: &BackendResponse{
				Text:       "Hello, world!",
				TokensUsed: 10,
				Metadata: map[string]interface{}{
					"model": "test-model",
				},
			},
		},
		{
			name: "backend returns 500 error",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("internal server error"))
			},
			request: &BackendRequest{
				Prompt: "Test prompt",
			},
			wantErr:     true,
			errContains: "status 500",
		},
		{
			name: "backend returns 404 error",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("not found"))
			},
			request: &BackendRequest{
				Prompt: "Test prompt",
			},
			wantErr:     true,
			errContains: "status 404",
		},
		{
			name: "backend returns invalid JSON",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid json"))
			},
			request: &BackendRequest{
				Prompt: "Test prompt",
			},
			wantErr:     true,
			errContains: "unmarshal response",
		},
		{
			name: "request with all parameters",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				// Verify request body
				var req BackendRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
				}

				if req.Prompt != "Test prompt" {
					t.Errorf("prompt = %s, want 'Test prompt'", req.Prompt)
				}
				if req.MaxTokens != 100 {
					t.Errorf("max_tokens = %d, want 100", req.MaxTokens)
				}
				if req.Temperature != 0.7 {
					t.Errorf("temperature = %f, want 0.7", req.Temperature)
				}

				resp := map[string]interface{}{
					"text":        "Response",
					"tokens_used": 5,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(resp)
			},
			request: &BackendRequest{
				Prompt:      "Test prompt",
				MaxTokens:   100,
				Temperature: 0.7,
				Parameters: map[string]interface{}{
					"top_p": 0.9,
				},
			},
			wantErr: false,
			wantResponse: &BackendResponse{
				Text:       "Response",
				TokensUsed: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(tt.backendHandler)
			defer server.Close()

			// Create client
			client := NewBackendClient(logger, 5*time.Second)

			// Create backend endpoint
			backend := &BackendEndpoint{
				ID:           "test-backend",
				URI:          server.URL,
				ModelVariant: "test-model",
				Timeout:      5 * time.Second,
			}

			// Execute request
			ctx := context.Background()
			resp, err := client.ForwardRequest(ctx, backend, tt.request)

			// Verify error
			if tt.wantErr {
				if err == nil {
					t.Errorf("ForwardRequest() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ForwardRequest() unexpected error: %v", err)
				return
			}

			// Verify response
			if tt.wantResponse != nil {
				if resp.Text != tt.wantResponse.Text {
					t.Errorf("response.Text = %s, want %s", resp.Text, tt.wantResponse.Text)
				}
				if resp.TokensUsed != tt.wantResponse.TokensUsed {
					t.Errorf("response.TokensUsed = %d, want %d", resp.TokensUsed, tt.wantResponse.TokensUsed)
				}
			}
		})
	}
}

func TestBackendClient_ForwardRequest_ContextCancellation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a slow backend that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		resp := map[string]interface{}{
			"text":        "Response",
			"tokens_used": 5,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(logger, 5*time.Second)

	backend := &BackendEndpoint{
		ID:           "test-backend",
		URI:          server.URL,
		ModelVariant: "test-model",
		Timeout:      5 * time.Second,
	}

	request := &BackendRequest{
		Prompt: "Test prompt",
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.ForwardRequest(ctx, backend, request)
	if err == nil {
		t.Error("ForwardRequest() expected timeout error, got nil")
	}
}

func TestBackendClient_HealthCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name           string
		healthPath     string
		backendHandler http.HandlerFunc
		wantErr        bool
		errContains    string
	}{
		{
			name:       "healthy backend with default path",
			healthPath: "",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("path = %s, want /health", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:       "healthy backend with custom path",
			healthPath: "/v2/health/ready",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v2/health/ready" {
					t.Errorf("path = %s, want /v2/health/ready", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:       "health path without leading slash",
			healthPath: "health/status",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health/status" {
					t.Errorf("path = %s, want /health/status", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:       "unhealthy backend - 503",
			healthPath: "/health",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			wantErr:     true,
			errContains: "status 503",
		},
		{
			name:       "unhealthy backend - 500",
			healthPath: "/health",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr:     true,
			errContains: "status 500",
		},
		{
			name:       "backend with trailing slash in URI",
			healthPath: "/health",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("path = %s, want /health", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(tt.backendHandler)
			defer server.Close()

			// Create client
			client := NewBackendClient(logger, 5*time.Second)

			// Create backend endpoint
			uri := server.URL
			if tt.name == "backend with trailing slash in URI" {
				uri += "/"
			}

			backend := &BackendEndpoint{
				ID:         "test-backend",
				URI:        uri,
				Timeout:    5 * time.Second,
				HealthPath: tt.healthPath,
			}

			// Execute health check
			ctx := context.Background()
			err := client.HealthCheck(ctx, backend)

			// Verify error
			if tt.wantErr {
				if err == nil {
					t.Errorf("HealthCheck() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("HealthCheck() unexpected error: %v", err)
			}
		})
	}
}

func TestBackendClient_HealthCheck_ContextCancellation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a slow health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBackendClient(logger, 5*time.Second)

	backend := &BackendEndpoint{
		ID:      "test-backend",
		URI:     server.URL,
		Timeout: 5 * time.Second,
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.HealthCheck(ctx, backend)
	if err == nil {
		t.Error("HealthCheck() expected timeout error, got nil")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)*2 && s[len(s)/2-len(substr)/2:len(s)/2+len(substr)/2+1] == substr ||
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
