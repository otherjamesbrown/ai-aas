package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestInvalidateCache verifies that InvalidateCache removes entries from the cache
func TestInvalidateCache(t *testing.T) {
	logger := zaptest.NewLogger(t)
	authenticator := NewAuthenticator(logger, "http://localhost", 2*time.Second)

	// Add a test entry to the cache
	fingerprint := "test-fingerprint-123"
	authenticator.validationCache[fingerprint] = &cachedValidation{
		result:    &AuthenticatedContext{APIKeyID: "test-key"},
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	// Verify entry exists
	authenticator.cacheMutex.RLock()
	_, exists := authenticator.validationCache[fingerprint]
	authenticator.cacheMutex.RUnlock()
	assert.True(t, exists, "cache entry should exist before invalidation")

	// Invalidate the cache entry
	authenticator.InvalidateCache(fingerprint)

	// Verify entry is removed
	authenticator.cacheMutex.RLock()
	_, exists = authenticator.validationCache[fingerprint]
	authenticator.cacheMutex.RUnlock()
	assert.False(t, exists, "cache entry should be removed after invalidation")
}

// TestInvalidateCacheNonExistent verifies that invalidating non-existent entries doesn't panic
func TestInvalidateCacheNonExistent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	authenticator := NewAuthenticator(logger, "http://localhost", 2*time.Second)

	// Invalidate a non-existent entry (should not panic)
	assert.NotPanics(t, func() {
		authenticator.InvalidateCache("non-existent-fingerprint")
	})
}

// TestCacheInvalidationSubscriber verifies that the Redis subscriber invalidates cache on message
func TestCacheInvalidationSubscriber(t *testing.T) {
	logger := zaptest.NewLogger(t)
	authenticator := NewAuthenticator(logger, "http://localhost", 2*time.Second)

	// Start miniredis (in-memory Redis for testing)
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	// Add a test entry to the cache
	fingerprint := "test-fingerprint-456"
	authenticator.validationCache[fingerprint] = &cachedValidation{
		result:    &AuthenticatedContext{APIKeyID: "test-key-456"},
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	// Start the subscriber in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscriberStarted := make(chan struct{})
	go func() {
		close(subscriberStarted)
		_ = authenticator.StartCacheInvalidationSubscriber(ctx, redisClient)
	}()

	// Wait for subscriber to start
	<-subscriberStarted
	time.Sleep(100 * time.Millisecond) // Give subscriber time to connect

	// Verify entry exists before publishing
	authenticator.cacheMutex.RLock()
	_, exists := authenticator.validationCache[fingerprint]
	authenticator.cacheMutex.RUnlock()
	require.True(t, exists, "cache entry should exist before invalidation")

	// Publish invalidation event
	err := redisClient.Publish(ctx, "apikey:invalidate", fingerprint).Err()
	require.NoError(t, err, "publishing invalidation event should succeed")

	// Wait for message to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify entry is removed
	authenticator.cacheMutex.RLock()
	_, exists = authenticator.validationCache[fingerprint]
	authenticator.cacheMutex.RUnlock()
	assert.False(t, exists, "cache entry should be removed after invalidation message")
}

// TestCacheInvalidationSubscriberNilRedis verifies graceful handling when Redis is nil
func TestCacheInvalidationSubscriberNilRedis(t *testing.T) {
	logger := zaptest.NewLogger(t)
	authenticator := NewAuthenticator(logger, "http://localhost", 2*time.Second)

	ctx := context.Background()

	// Should return without error when Redis client is nil
	err := authenticator.StartCacheInvalidationSubscriber(ctx, nil)
	assert.NoError(t, err, "should handle nil Redis client gracefully")
}
