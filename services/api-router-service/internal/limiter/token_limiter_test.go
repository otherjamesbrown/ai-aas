package limiter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTokenLimiter_CheckLimit(t *testing.T) {
	logger := zap.NewNop()

	t.Run("allowed when under limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/internal/v1/users/")
			assert.Contains(t, r.URL.Path, "/rate-limit")

			resp := rateLimitResponse{
				Allowed: true,
				Windows: []windowResponse{
					{
						Window:     "1h",
						Limit:      10000,
						Used:       3500,
						Remaining:  6500,
						Percentage: 35.0,
						ResetsAt:   time.Now().Add(30 * time.Minute).Format(time.RFC3339),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		status, err := limiter.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status.Allowed)
		assert.Len(t, status.Windows, 1)
		assert.Equal(t, "1h", status.Windows[0].Window)
		assert.Equal(t, int64(10000), status.Windows[0].Limit)
		assert.Equal(t, int64(3500), status.Windows[0].Used)
		assert.Equal(t, int64(6500), status.Windows[0].Remaining)
		assert.Nil(t, status.BlockingWindow)
	})

	t.Run("blocked when limit exceeded", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resetTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
			resp := rateLimitResponse{
				Allowed: false,
				Windows: []windowResponse{
					{
						Window:     "1h",
						Limit:      10000,
						Used:       10000,
						Remaining:  0,
						Percentage: 100.0,
						ResetsAt:   resetTime,
					},
				},
				BlockingWindow: &windowResponse{
					Window:     "1h",
					Limit:      10000,
					Used:       10000,
					Remaining:  0,
					Percentage: 100.0,
					ResetsAt:   resetTime,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		status, err := limiter.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.False(t, status.Allowed)
		assert.NotNil(t, status.BlockingWindow)
		assert.Equal(t, "1h", status.BlockingWindow.Window)
		assert.Equal(t, int64(10000), status.BlockingWindow.Limit)
	})

	t.Run("graceful degradation when service unavailable", func(t *testing.T) {
		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: "http://localhost:1", // Invalid port
			Timeout:  100 * time.Millisecond,
			Logger:   logger,
		})

		status, err := limiter.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status.Allowed, "should allow request when service unavailable")
	})

	t.Run("graceful degradation when no endpoint configured", func(t *testing.T) {
		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: "",
			Logger:   logger,
		})

		status, err := limiter.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status.Allowed, "should allow request when no endpoint configured")
	})

	t.Run("graceful degradation on 404 user not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		status, err := limiter.CheckLimit(context.Background(), "unknown-user")
		require.NoError(t, err)
		assert.True(t, status.Allowed, "should allow request when user not found")
	})

	t.Run("graceful degradation on 500 error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		status, err := limiter.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status.Allowed, "should allow request on 500 error")
	})

	t.Run("includes M2M API key in request", func(t *testing.T) {
		var receivedAPIKey string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAPIKey = r.Header.Get("X-API-Key")
			resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint:  server.URL,
			M2MAPIKey: "m2m-secret-key",
			Logger:    logger,
		})

		_, err := limiter.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, "m2m-secret-key", receivedAPIKey)
	})
}

func TestTokenLimiter_RecordUsage(t *testing.T) {
	logger := zap.NewNop()

	t.Run("records usage successfully", func(t *testing.T) {
		var receivedRequest RecordUsageRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/internal/v1/usage/tokens", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			err := json.NewDecoder(r.Body).Decode(&receivedRequest)
			require.NoError(t, err)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		err := limiter.RecordUsage(context.Background(), "user-123", 500)
		require.NoError(t, err)
		assert.Equal(t, "user-123", receivedRequest.UserID)
		assert.Equal(t, int64(500), receivedRequest.Tokens)
	})

	t.Run("skips recording when no endpoint configured", func(t *testing.T) {
		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: "",
			Logger:   logger,
		})

		err := limiter.RecordUsage(context.Background(), "user-123", 500)
		require.NoError(t, err)
	})

	t.Run("skips recording zero tokens", func(t *testing.T) {
		serverCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		err := limiter.RecordUsage(context.Background(), "user-123", 0)
		require.NoError(t, err)
		assert.False(t, serverCalled, "should not call server for zero tokens")
	})

	t.Run("skips recording negative tokens", func(t *testing.T) {
		serverCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serverCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		err := limiter.RecordUsage(context.Background(), "user-123", -100)
		require.NoError(t, err)
		assert.False(t, serverCalled, "should not call server for negative tokens")
	})

	t.Run("returns error on server failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		err := limiter.RecordUsage(context.Background(), "user-123", 500)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")
	})

	t.Run("includes M2M API key in request", func(t *testing.T) {
		var receivedAPIKey string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAPIKey = r.Header.Get("X-API-Key")
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint:  server.URL,
			M2MAPIKey: "m2m-secret-key",
			Logger:    logger,
		})

		err := limiter.RecordUsage(context.Background(), "user-123", 500)
		require.NoError(t, err)
		assert.Equal(t, "m2m-secret-key", receivedAPIKey)
	})
}

func TestTokenLimiter_IsConfigured(t *testing.T) {
	t.Run("returns true when endpoint configured", func(t *testing.T) {
		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: "http://localhost:8080",
		})
		assert.True(t, limiter.IsConfigured())
	})

	t.Run("returns false when no endpoint", func(t *testing.T) {
		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: "",
		})
		assert.False(t, limiter.IsConfigured())
	})
}
