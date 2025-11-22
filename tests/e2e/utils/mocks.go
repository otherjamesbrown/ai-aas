package utils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockBackend represents a mock backend server
type MockBackend struct {
	server   *httptest.Server
	requests []MockRequest
	mu       sync.RWMutex
	handler  http.HandlerFunc
}

// MockRequest represents a captured request
type MockRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

// MockResponse represents a mock response configuration
type MockResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Delay      int // milliseconds
}

// NewMockBackend creates a new mock backend server
func NewMockBackend() *MockBackend {
	mb := &MockBackend{
		requests: []MockRequest{},
	}

	mb.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mb.mu.Lock()
		defer mb.mu.Unlock()

		// Capture request
		body := make([]byte, 0)
		if r.Body != nil {
			body, _ = readBody(r)
		}

		headers := make(map[string]string)
		for k, v := range r.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		mb.requests = append(mb.requests, MockRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: headers,
			Body:    body,
		})

		// Use custom handler if set, otherwise default response
		if mb.handler != nil {
			mb.handler(w, r)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}
	}))

	return mb
}

// SetHandler sets a custom handler for the mock backend
func (mb *MockBackend) SetHandler(handler http.HandlerFunc) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.handler = handler
}

// SetResponse sets a default response for all requests
func (mb *MockBackend) SetResponse(response MockResponse) {
	mb.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}

		// Set status code
		w.WriteHeader(response.StatusCode)

		// Write body
		if len(response.Body) > 0 {
			w.Write(response.Body)
		}
	})
}

// URL returns the mock backend URL
func (mb *MockBackend) URL() string {
	return mb.server.URL
}

// Close shuts down the mock backend
func (mb *MockBackend) Close() {
	mb.server.Close()
}

// GetRequests returns all captured requests
func (mb *MockBackend) GetRequests() []MockRequest {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	result := make([]MockRequest, len(mb.requests))
	copy(result, mb.requests)
	return result
}

// ClearRequests clears captured requests
func (mb *MockBackend) ClearRequests() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.requests = []MockRequest{}
}

// readBody reads request body (helper function)
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// MockModelBackend creates a mock model backend with OpenAI-compatible responses
func MockModelBackend() *MockBackend {
	mb := NewMockBackend()
	mb.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1677652288,
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "This is a mock response"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 9,
					"completion_tokens": 12,
					"total_tokens": 21
				}
			}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	return mb
}

