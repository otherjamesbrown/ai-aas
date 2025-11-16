package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	handler := setupTestServer(t, func(r chi.Router) {
		r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})
		r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})
	})

	tests := []struct {
		name           string
		origin         string
		method         string
		expectedStatus int
		expectCORS     bool
	}{
		{
			name:           "OPTIONS preflight from localhost:5173",
			origin:         "http://localhost:5173",
			method:         "OPTIONS",
			expectedStatus: http.StatusNoContent,
			expectCORS:     true,
		},
		{
			name:           "GET request from localhost:5173",
			origin:         "http://localhost:5173",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectCORS:     true,
		},
		{
			name:           "POST request from localhost:5173",
			origin:         "http://localhost:5173",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectCORS:     true,
		},
		{
			name:           "OPTIONS from different localhost port",
			origin:         "http://localhost:3000",
			method:         "OPTIONS",
			expectedStatus: http.StatusNoContent,
			expectCORS:     true,
		},
		{
			name:           "Request without origin",
			origin:         "",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectCORS:     false,
		},
		{
			name:           "Request from disallowed origin",
			origin:         "http://evil.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectCORS:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			corsHeader := w.Header().Get("Access-Control-Allow-Origin")
			if tt.expectCORS {
				assert.Equal(t, tt.origin, corsHeader, "CORS header should match origin")
				if tt.method == "OPTIONS" {
					assert.Equal(t, "GET, POST, PUT, PATCH, DELETE, OPTIONS",
						w.Header().Get("Access-Control-Allow-Methods"))
					assert.Equal(t, "true",
						w.Header().Get("Access-Control-Allow-Credentials"))
					assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "X-API-Key")
				}
			} else {
				assert.Empty(t, corsHeader, "CORS header should not be set")
			}
		})
	}
}

func TestCORS_NotFoundResponse(t *testing.T) {
	handler := setupTestServer(t, nil)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errorResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errorResp)
	require.NoError(t, err)
	assert.Equal(t, "route not found", errorResp["error"])
	assert.Equal(t, "GET", errorResp["method"])
	assert.Equal(t, "/nonexistent", errorResp["path"])
}

func TestCORS_MethodNotAllowedResponse(t *testing.T) {
	handler := setupTestServer(t, func(r chi.Router) {
		// Register a route that only accepts GET
		r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// Try POST to a GET-only route
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errorResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &errorResp)
	require.NoError(t, err)
	assert.Equal(t, "method not allowed", errorResp["error"])
	assert.Equal(t, "POST", errorResp["method"])
	assert.Equal(t, "/test", errorResp["path"])
}

func TestCORS_OPTIONSOnNotFound(t *testing.T) {
	handler := setupTestServer(t, nil)

	// OPTIONS request to non-existent route should return 204 with CORS headers
	req := httptest.NewRequest("OPTIONS", "/nonexistent", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OPTIONSOnMethodNotAllowed(t *testing.T) {
	handler := setupTestServer(t, func(r chi.Router) {
		r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// OPTIONS request to route with wrong method should return 204 with CORS headers
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestDebugRoutesEndpoint(t *testing.T) {
	handler := setupTestServer(t, func(r chi.Router) {
		r.Get("/test1", func(w http.ResponseWriter, r *http.Request) {})
		r.Post("/test2", func(w http.ResponseWriter, r *http.Request) {})
	})

	req := httptest.NewRequest("GET", "/debug/routes", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	routes, ok := response["routes"].([]interface{})
	require.True(t, ok, "routes should be an array")

	count, ok := response["count"].(float64)
	require.True(t, ok, "count should be a number")
	assert.Greater(t, int(count), 0, "should have at least one route")

	// Verify some expected routes exist
	routeMap := make(map[string]string)
	for _, route := range routes {
		routeObj := route.(map[string]interface{})
		method := routeObj["method"].(string)
		path := routeObj["route"].(string)
		routeMap[method+" "+path] = path
	}

	// Check for some expected routes
	assert.Contains(t, routeMap, "GET /healthz")
	assert.Contains(t, routeMap, "GET /readyz")
	assert.Contains(t, routeMap, "GET /debug/routes")
	assert.Contains(t, routeMap, "GET /test1")
	assert.Contains(t, routeMap, "POST /test2")
}

func TestRequestLogging(t *testing.T) {
	// This test verifies that the logging middleware doesn't break requests
	handler := setupTestServer(t, func(r chi.Router) {
		r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Request should complete successfully
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestResponseWriter_StatusCodeCapture(t *testing.T) {
	handler := setupTestServer(t, func(r chi.Router) {
		r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		r.Get("/notfound", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		r.Get("/error", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	})

	tests := []struct {
		path           string
		expectedStatus int
	}{
		{"/ok", http.StatusOK},
		{"/notfound", http.StatusNotFound},
		{"/error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestCORS_AllowedHeaders(t *testing.T) {
	handler := setupTestServer(t, func(r chi.Router) {
		r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-API-Key, Content-Type")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowedHeaders, "X-API-Key")
	assert.Contains(t, allowedHeaders, "Content-Type")
	assert.Contains(t, allowedHeaders, "Authorization")
}

// setupTestServer creates a test server with the given route registration function
// Returns the HTTP handler (router) for direct testing
func setupTestServer(t *testing.T, registerRoutes func(chi.Router)) http.Handler {
	logger := zap.NewNop()
	
	srv := New(Options{
		Port:        8081,
		Logger:      logger,
		ServiceName: "test-server",
		Readiness:   func(ctx context.Context) error { return nil },
		RegisterRoutes: registerRoutes,
	})

	return srv.Handler
}
