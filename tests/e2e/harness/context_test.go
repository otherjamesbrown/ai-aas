package harness

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupRedisTokenCounters_NoClient tests that cleanup is safe when Redis client is nil
func TestCleanupRedisTokenCounters_NoClient(t *testing.T) {
	ctx := &Context{
		RedisClient: nil,
	}

	err := ctx.CleanupRedisTokenCounters()
	assert.NoError(t, err, "cleanup should succeed with nil client")
}

// TestCleanupRedisTokenCounters_Integration tests Redis token cleanup with a real Redis instance
// This test requires Redis to be running at localhost:6379
func TestCleanupRedisTokenCounters_Integration(t *testing.T) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for testing to avoid conflicts
	})
	defer client.Close()

	// Test if Redis is available
	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(testCtx).Err(); err != nil {
		t.Skipf("Redis not available, skipping test: %v", err)
		return
	}

	// Setup: Create some token counter keys
	ctx := context.Background()
	testKeys := []string{
		"ratelimit:tokens:org123:daily",
		"ratelimit:tokens:org456:monthly",
		"ratelimit:tokens:user789:hourly",
	}

	for _, key := range testKeys {
		err := client.Set(ctx, key, "1000", 1*time.Hour).Err()
		require.NoError(t, err, "failed to set test key %s", key)
	}

	// Also create a non-token key to verify it's not deleted
	nonTokenKey := "some:other:key"
	err := client.Set(ctx, nonTokenKey, "value", 1*time.Hour).Err()
	require.NoError(t, err, "failed to set non-token key")

	// Create test context with Redis client
	testContext := &Context{
		RedisClient: client,
	}

	// Execute cleanup
	err = testContext.CleanupRedisTokenCounters()
	require.NoError(t, err, "cleanup should succeed")

	// Verify token keys are deleted
	for _, key := range testKeys {
		exists, err := client.Exists(ctx, key).Result()
		require.NoError(t, err, "failed to check existence of %s", key)
		assert.Equal(t, int64(0), exists, "token key %s should be deleted", key)
	}

	// Verify non-token key still exists
	exists, err := client.Exists(ctx, nonTokenKey).Result()
	require.NoError(t, err, "failed to check existence of non-token key")
	assert.Equal(t, int64(1), exists, "non-token key should not be deleted")

	// Cleanup test keys
	client.Del(ctx, nonTokenKey)
}

// TestCleanupRedisTokenCounters_NoKeys tests cleanup when no token keys exist
func TestCleanupRedisTokenCounters_NoKeys(t *testing.T) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for testing
	})
	defer client.Close()

	// Test if Redis is available
	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(testCtx).Err(); err != nil {
		t.Skipf("Redis not available, skipping test: %v", err)
		return
	}

	// Ensure no token keys exist (cleanup from previous tests)
	ctx := context.Background()
	keys, err := client.Keys(ctx, "ratelimit:tokens:*").Result()
	require.NoError(t, err)
	if len(keys) > 0 {
		client.Del(ctx, keys...)
	}

	// Create test context
	testContext := &Context{
		RedisClient: client,
	}

	// Execute cleanup - should succeed with no keys to delete
	err = testContext.CleanupRedisTokenCounters()
	assert.NoError(t, err, "cleanup should succeed even with no keys")
}
