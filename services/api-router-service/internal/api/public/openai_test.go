package public

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOpenAICompletionRequest_Unmarshal_Stream tests that the Stream field
// is correctly unmarshaled from JSON (aas-tv7d)
func TestOpenAICompletionRequest_Unmarshal_Stream(t *testing.T) {
	tests := []struct {
		name           string
		jsonPayload    string
		expectedStream bool
	}{
		{
			name:           "stream true",
			jsonPayload:    `{"model":"gpt-4","prompt":"test","stream":true}`,
			expectedStream: true,
		},
		{
			name:           "stream false",
			jsonPayload:    `{"model":"gpt-4","prompt":"test","stream":false}`,
			expectedStream: false,
		},
		{
			name:           "stream omitted (defaults to false)",
			jsonPayload:    `{"model":"gpt-4","prompt":"test"}`,
			expectedStream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req OpenAICompletionRequest
			err := json.Unmarshal([]byte(tt.jsonPayload), &req)
			if err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if req.Stream != tt.expectedStream {
				t.Errorf("Stream field mismatch: got %v, expected %v", req.Stream, tt.expectedStream)
			}
		})
	}
}

// TestOpenAICompletionRequest_Decode_AfterMiddleware tests that the Stream field
// is correctly decoded after the request body has been buffered by middleware (aas-tv7d)
func TestOpenAICompletionRequest_Decode_AfterMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		jsonPayload    string
		expectedStream bool
	}{
		{
			name:           "stream true with middleware",
			jsonPayload:    `{"model":"gpt-4","prompt":"test","stream":true}`,
			expectedStream: true,
		},
		{
			name:           "stream false with middleware",
			jsonPayload:    `{"model":"gpt-4","prompt":"test","stream":false}`,
			expectedStream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate BodyBufferMiddleware behavior
			body := []byte(tt.jsonPayload)

			// Middleware reads body
			bodyReader := io.NopCloser(bytes.NewReader(body))

			// Middleware restores body
			req := httptest.NewRequest("POST", "/v1/completions", bodyReader)
			req.Body = io.NopCloser(bytes.NewReader(body))

			// Handler decodes body
			var openAIReq OpenAICompletionRequest
			err := json.NewDecoder(req.Body).Decode(&openAIReq)
			if err != nil {
				t.Fatalf("failed to decode: %v", err)
			}

			if openAIReq.Stream != tt.expectedStream {
				t.Errorf("Stream field mismatch after middleware: got %v, expected %v", openAIReq.Stream, tt.expectedStream)
			}
		})
	}
}

// TestBodyBufferMiddleware_PreservesStreamField tests that BodyBufferMiddleware
// doesn't interfere with the Stream field (aas-tv7d)
func TestBodyBufferMiddleware_PreservesStreamField(t *testing.T) {
	jsonPayload := `{"model":"gpt-4","prompt":"test","stream":true,"max_tokens":100}`

	// Create a test handler that checks if stream field is preserved
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var openAIReq OpenAICompletionRequest
		err := json.NewDecoder(r.Body).Decode(&openAIReq)
		if err != nil {
			t.Fatalf("failed to decode in handler: %v", err)
		}

		// Verify the stream field is set correctly
		if !openAIReq.Stream {
			t.Errorf("Stream field should be true, got false")
		}

		// Verify other fields are also correct
		if openAIReq.Model != "gpt-4" {
			t.Errorf("Model mismatch: got %s, expected gpt-4", openAIReq.Model)
		}
		if openAIReq.Prompt != "test" {
			t.Errorf("Prompt mismatch: got %s, expected test", openAIReq.Prompt)
		}
		if openAIReq.MaxTokens != 100 {
			t.Errorf("MaxTokens mismatch: got %d, expected 100", openAIReq.MaxTokens)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Wrap with BodyBufferMiddleware
	middleware := BodyBufferMiddleware(1024 * 1024) // 1MB max
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewBufferString(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestBodyBufferMiddleware_StreamFieldWithUnmarshal tests that using json.Unmarshal
// with buffered body (aas-wc6u fix) correctly preserves the Stream field
func TestBodyBufferMiddleware_StreamFieldWithUnmarshal(t *testing.T) {
	jsonPayload := `{"model":"gpt-4","prompt":"test","stream":true,"max_tokens":100}`

	// Create a test handler that uses the buffered body from context
	// This matches the pattern used in HandleOpenAICompletions after aas-wc6u fix
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var openAIReq OpenAICompletionRequest

		// Use buffered body from context (aas-wc6u fix)
		if bufferedBody := r.Context().Value(BufferedBodyKey); bufferedBody != nil {
			bodyBytes, ok := bufferedBody.([]byte)
			if !ok {
				t.Fatalf("buffered body is not []byte")
			}
			if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
				t.Fatalf("failed to unmarshal buffered body: %v", err)
			}
		} else {
			t.Fatal("buffered body not found in context")
		}

		// Verify the stream field is set correctly
		if !openAIReq.Stream {
			t.Errorf("Stream field should be true, got false")
		}

		// Verify other fields are also correct
		if openAIReq.Model != "gpt-4" {
			t.Errorf("Model mismatch: got %s, expected gpt-4", openAIReq.Model)
		}
		if openAIReq.Prompt != "test" {
			t.Errorf("Prompt mismatch: got %s, expected test", openAIReq.Prompt)
		}
		if openAIReq.MaxTokens != 100 {
			t.Errorf("MaxTokens mismatch: got %d, expected 100", openAIReq.MaxTokens)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Wrap with BodyBufferMiddleware
	middleware := BodyBufferMiddleware(1024 * 1024) // 1MB max
	handler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewBufferString(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
