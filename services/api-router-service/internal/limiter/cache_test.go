package limiter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRateLimitCache_CheckLimit(t *testing.T) {
	logger := zap.NewNop()

	t.Run("caches result on first call", func(t *testing.T) {
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			resp := rateLimitResponse{
				Allowed: true,
				Windows: []windowResponse{
					{
						Window:     "1h",
						Limit:      10000,
						Used:       1000,
						Remaining:  9000,
						Percentage: 10.0,
						ResetsAt:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
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

		cache := NewRateLimitCache(RateLimitCacheConfig{
			TTL:     60 * time.Second,
			Logger:  logger,
			Limiter: limiter,
		})
		defer cache.Stop()

		// First call should hit the server
		status, err := cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status.Allowed)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

		// Second call should use cache
		status2, err := cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status2.Allowed)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "should not call server again")
	})

	t.Run("expires after TTL", func(t *testing.T) {
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		cache := NewRateLimitCache(RateLimitCacheConfig{
			TTL:     50 * time.Millisecond, // Short TTL for testing
			Logger:  logger,
			Limiter: limiter,
		})
		defer cache.Stop()

		// First call
		_, err := cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

		// Wait for TTL to expire
		time.Sleep(60 * time.Millisecond)

		// Second call should hit server again
		_, err = cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})

	t.Run("thread-safe under concurrent access", func(t *testing.T) {
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			time.Sleep(10 * time.Millisecond) // Simulate latency
			resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		cache := NewRateLimitCache(RateLimitCacheConfig{
			TTL:     60 * time.Second,
			Logger:  logger,
			Limiter: limiter,
		})
		defer cache.Stop()

		// Launch many concurrent requests
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := cache.CheckLimit(context.Background(), "user-concurrent")
				assert.NoError(t, err)
			}()
		}
		wg.Wait()

		// Should have minimal server calls due to caching
		// Note: There may be more than 1 call due to race between first fetch and cache update
		calls := atomic.LoadInt32(&callCount)
		assert.LessOrEqual(t, calls, int32(10), "should have minimal server calls, got %d", calls)
	})
}

func TestRateLimitCache_Invalidate(t *testing.T) {
	logger := zap.NewNop()

	t.Run("invalidate forces refetch", func(t *testing.T) {
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		cache := NewRateLimitCache(RateLimitCacheConfig{
			TTL:     60 * time.Second,
			Logger:  logger,
			Limiter: limiter,
		})
		defer cache.Stop()

		// First call
		_, err := cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

		// Second call from cache
		_, err = cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

		// Invalidate
		cache.Invalidate("user-123")

		// Third call should hit server again
		_, err = cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})
}

func TestRateLimitCache_InvalidateAll(t *testing.T) {
	logger := zap.NewNop()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	limiter := NewTokenLimiter(TokenLimiterConfig{
		Endpoint: server.URL,
		Logger:   logger,
	})

	cache := NewRateLimitCache(RateLimitCacheConfig{
		TTL:     60 * time.Second,
		Logger:  logger,
		Limiter: limiter,
	})
	defer cache.Stop()

	// Cache multiple users
	_, _ = cache.CheckLimit(context.Background(), "user-1")
	_, _ = cache.CheckLimit(context.Background(), "user-2")
	_, _ = cache.CheckLimit(context.Background(), "user-3")
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	assert.Equal(t, 3, cache.Size())

	// Clear all
	cache.InvalidateAll()
	assert.Equal(t, 0, cache.Size())

	// Calls should hit server again
	_, _ = cache.CheckLimit(context.Background(), "user-1")
	assert.Equal(t, int32(4), atomic.LoadInt32(&callCount))
}

func TestRateLimitCache_GetCachedStatus(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	limiter := NewTokenLimiter(TokenLimiterConfig{
		Endpoint: server.URL,
		Logger:   logger,
	})

	cache := NewRateLimitCache(RateLimitCacheConfig{
		TTL:     60 * time.Second,
		Logger:  logger,
		Limiter: limiter,
	})
	defer cache.Stop()

	t.Run("returns nil when not cached", func(t *testing.T) {
		status := cache.GetCachedStatus("unknown-user")
		assert.Nil(t, status)
	})

	t.Run("returns cached status", func(t *testing.T) {
		// Populate cache
		_, err := cache.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)

		status := cache.GetCachedStatus("user-123")
		require.NotNil(t, status)
		assert.True(t, status.Allowed)
	})
}

func TestCachedTokenLimiter(t *testing.T) {
	logger := zap.NewNop()

	t.Run("CheckLimit uses cache", func(t *testing.T) {
		var callCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				atomic.AddInt32(&callCount, 1)
				resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		cached := NewCachedTokenLimiter(limiter, 60*time.Second, logger)
		defer cached.Stop()

		// First call
		status, err := cached.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.True(t, status.Allowed)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

		// Second call from cache
		_, err = cached.CheckLimit(context.Background(), "user-123")
		require.NoError(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
	})

	t.Run("RecordUsage invalidates cache", func(t *testing.T) {
		var checkCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				atomic.AddInt32(&checkCount, 1)
				resp := rateLimitResponse{Allowed: true, Windows: []windowResponse{}}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		limiter := NewTokenLimiter(TokenLimiterConfig{
			Endpoint: server.URL,
			Logger:   logger,
		})

		cached := NewCachedTokenLimiter(limiter, 60*time.Second, logger)
		defer cached.Stop()

		// Populate cache
		_, _ = cached.CheckLimit(context.Background(), "user-123")
		assert.Equal(t, int32(1), atomic.LoadInt32(&checkCount))

		// Record usage (should invalidate cache)
		err := cached.RecordUsage(context.Background(), "user-123", 100)
		require.NoError(t, err)

		// Check again - should hit server due to invalidation
		_, _ = cached.CheckLimit(context.Background(), "user-123")
		assert.Equal(t, int32(2), atomic.LoadInt32(&checkCount))
	})
}
