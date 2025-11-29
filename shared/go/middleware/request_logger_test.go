package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestRequestLogger(t *testing.T) {
	// Create a test logger (nop logger for quiet tests)
	logger := zap.NewNop()
	defer logger.Sync()

	tests := []struct {
		name           string
		path           string
		method         string
		config         RequestLoggerConfig
		expectedStatus int
		shouldSkip     bool
	}{
		{
			name:           "normal request",
			path:           "/api/v1/users",
			method:         "GET",
			config:         DefaultRequestLoggerConfig(),
			expectedStatus: http.StatusOK,
			shouldSkip:     false,
		},
		{
			name:           "health check skipped",
			path:           "/healthz",
			method:         "GET",
			config:         DefaultRequestLoggerConfig(),
			expectedStatus: http.StatusOK,
			shouldSkip:     true,
		},
		{
			name:           "readiness check skipped",
			path:           "/v1/status/readyz",
			method:         "GET",
			config:         DefaultRequestLoggerConfig(),
			expectedStatus: http.StatusOK,
			shouldSkip:     true,
		},
		{
			name:           "POST request",
			path:           "/api/v1/resources",
			method:         "POST",
			config:         DefaultRequestLoggerConfig(),
			expectedStatus: http.StatusCreated,
			shouldSkip:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.expectedStatus)
				w.Write([]byte("OK"))
			})

			// Apply middleware
			middleware := RequestLogger(logger, tt.config)
			wrapped := middleware(handler)

			// Create request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			// Execute
			wrapped.ServeHTTP(rec, req)

			// Check status
			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.WriteHeader(http.StatusCreated)

		if rw.statusCode != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rw.statusCode)
		}
	})

	t.Run("captures bytes written", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		n, err := rw.Write([]byte("Hello, World!"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if n != 13 {
			t.Errorf("expected 13 bytes written, got %d", n)
		}

		if rw.bytesWritten != 13 {
			t.Errorf("expected bytesWritten 13, got %d", rw.bytesWritten)
		}
	})

	t.Run("defaults to 200 if WriteHeader not called", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		rw.Write([]byte("test"))

		if rw.statusCode != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rw.statusCode)
		}
	})
}

func TestFilterHeaders(t *testing.T) {
	sensitiveSet := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
	}

	headers := http.Header{
		"Authorization": []string{"Bearer token123"},
		"X-API-Key":     []string{"secret-key"},
		"Content-Type":  []string{"application/json"},
		"Accept":        []string{"*/*"},
	}

	result := filterHeaders(headers, sensitiveSet)

	if result["Authorization"] != "[REDACTED]" {
		t.Errorf("expected Authorization to be redacted, got %s", result["Authorization"])
	}

	if result["X-API-Key"] != "[REDACTED]" {
		t.Errorf("expected X-API-Key to be redacted, got %s", result["X-API-Key"])
	}

	if result["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type to be application/json, got %s", result["Content-Type"])
	}

	if result["Accept"] != "*/*" {
		t.Errorf("expected Accept to be */*, got %s", result["Accept"])
	}
}

func TestResponseWriter_WriteHeaderOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// First call should set status
	rw.WriteHeader(http.StatusCreated)
	if rw.statusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rw.statusCode)
	}

	// Second call should be ignored
	rw.WriteHeader(http.StatusBadRequest)
	if rw.statusCode != http.StatusCreated {
		t.Errorf("expected status to remain %d, got %d", http.StatusCreated, rw.statusCode)
	}
}

func TestResponseWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Flush should not panic - httptest.ResponseRecorder implements Flusher
	rw.Flush()
}

func TestResponseWriter_Unwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	unwrapped := rw.Unwrap()
	if unwrapped != rec {
		t.Error("Unwrap() did not return the underlying ResponseWriter")
	}
}

func TestRequestLogger_StatusCodeLevels(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	tests := []struct {
		name       string
		statusCode int
	}{
		{"5xx error", http.StatusInternalServerError},
		{"4xx error", http.StatusBadRequest},
		{"2xx success", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			middleware := RequestLogger(logger, DefaultRequestLoggerConfig())
			wrapped := middleware(handler)

			req := httptest.NewRequest("GET", "/api/test", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			if rec.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, rec.Code)
			}
		})
	}
}

func TestRequestLogger_WithQueryString(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestLogger(logger, DefaultRequestLoggerConfig())
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/api/test?foo=bar&baz=qux", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRequestLogger_WithRequestHeaders(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	config := DefaultRequestLoggerConfig()
	config.LogRequestHeaders = true

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequestLogger(logger, config)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestDefaultRequestLoggerConfig(t *testing.T) {
	config := DefaultRequestLoggerConfig()

	// Check skip paths
	if len(config.SkipPaths) == 0 {
		t.Error("expected SkipPaths to be non-empty")
	}

	// Check sensitive headers
	if len(config.SensitiveHeaders) == 0 {
		t.Error("expected SensitiveHeaders to be non-empty")
	}

	// Verify specific defaults
	foundHealthz := false
	for _, p := range config.SkipPaths {
		if p == "/healthz" {
			foundHealthz = true
			break
		}
	}
	if !foundHealthz {
		t.Error("expected /healthz in SkipPaths")
	}

	foundAuth := false
	for _, h := range config.SensitiveHeaders {
		if h == "Authorization" {
			foundAuth = true
			break
		}
	}
	if !foundAuth {
		t.Error("expected Authorization in SensitiveHeaders")
	}
}

func TestFilterHeaders_EmptyHeader(t *testing.T) {
	sensitiveSet := map[string]bool{
		"authorization": true,
	}

	headers := http.Header{
		"Empty-Header": []string{},
	}

	result := filterHeaders(headers, sensitiveSet)

	// Empty header should not be included
	if _, ok := result["Empty-Header"]; ok {
		t.Error("expected empty header to not be included")
	}
}
