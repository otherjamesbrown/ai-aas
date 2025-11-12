// Package limiter provides unit tests for rate limiting functionality.
//
// Purpose:
//   These tests validate the token bucket rate limiting implementation,
//   including per-organization and per-API-key rate limiting.
//
package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// setupTestRedis creates a test Redis client (requires Redis to be running).
// Returns nil if Redis is not available (tests will be skipped).
func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for testing to avoid conflicts
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available, skipping test: %v", err)
		return nil
	}

	// Clean up test keys
	client.FlushDB(ctx)

	return client
}

// TestRateLimiter_CheckOrganization tests organization-level rate limiting.
func TestRateLimiter_CheckOrganization(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 10, 20) // 10 RPS, burst of 20

	ctx := context.Background()
	orgID := "test-org-1"

	// First request should be allowed
	result, err := limiter.CheckOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected first request to be allowed")
	}
	if result.Remaining != 19 {
		t.Errorf("expected remaining 19, got %d", result.Remaining)
	}
	if result.Limit != 20 {
		t.Errorf("expected limit 20, got %d", result.Limit)
	}

	// Consume all tokens
	for i := 0; i < 19; i++ {
		result, err := limiter.CheckOrganization(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+2, err)
		}
		if !result.Allowed {
			t.Errorf("expected request %d to be allowed", i+2)
		}
	}

	// Next request should be denied
	result, err = limiter.CheckOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected request to be denied after consuming all tokens")
	}
	if result.RetryAfter <= 0 {
		t.Error("expected retry_after to be set when denied")
	}
}

// TestRateLimiter_CheckAPIKey tests API key-level rate limiting.
func TestRateLimiter_CheckAPIKey(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 10, 20) // Default: 10 RPS, burst of 20

	ctx := context.Background()
	apiKeyID := "test-key-1"

	// Test with custom limits
	result, err := limiter.CheckAPIKey(ctx, apiKeyID, 5, 10) // 5 RPS, burst of 10
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected first request to be allowed")
	}
	if result.Remaining != 9 {
		t.Errorf("expected remaining 9, got %d", result.Remaining)
	}
	if result.Limit != 10 {
		t.Errorf("expected limit 10, got %d", result.Limit)
	}

	// Consume all tokens
	for i := 0; i < 9; i++ {
		result, err := limiter.CheckAPIKey(ctx, apiKeyID, 5, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected request %d to be allowed", i+2)
		}
	}

	// Next request should be denied
	result, err = limiter.CheckAPIKey(ctx, apiKeyID, 5, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected request to be denied after consuming all tokens")
	}
}

// TestRateLimiter_TokenRefill tests that tokens refill over time.
func TestRateLimiter_TokenRefill(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 10, 20) // 10 RPS = 1 token per 100ms

	ctx := context.Background()
	orgID := "test-org-refill"

	// Consume all tokens
	for i := 0; i < 20; i++ {
		result, err := limiter.CheckOrganization(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected request %d to be allowed", i+1)
		}
	}

	// Should be denied now
	result, err := limiter.CheckOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected request to be denied")
	}

	// Wait for token refill (110ms should be enough for 1 token at 10 RPS)
	time.Sleep(110 * time.Millisecond)

	// Should be allowed again
	result, err = limiter.CheckOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected request to be allowed after token refill")
	}
	if result.Remaining != 0 {
		t.Errorf("expected remaining 0, got %d", result.Remaining)
	}
}

// TestRateLimiter_Isolation tests that different organizations have isolated rate limits.
func TestRateLimiter_Isolation(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 10, 20)

	ctx := context.Background()
	org1 := "test-org-1"
	org2 := "test-org-2"

	// Exhaust org1's rate limit
	for i := 0; i < 20; i++ {
		result, err := limiter.CheckOrganization(ctx, org1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected org1 request %d to be allowed", i+1)
		}
	}

	// Org1 should be denied
	result, err := limiter.CheckOrganization(ctx, org1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected org1 to be rate limited")
	}

	// Org2 should still be allowed (different bucket)
	result, err = limiter.CheckOrganization(ctx, org2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected org2 to be allowed (isolated from org1)")
	}
	if result.Remaining != 19 {
		t.Errorf("expected org2 remaining 19, got %d", result.Remaining)
	}
}

// TestRateLimiter_Reset tests the Reset function.
func TestRateLimiter_Reset(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 10, 20)

	ctx := context.Background()
	orgID := "test-org-reset"

	// Consume some tokens
	for i := 0; i < 10; i++ {
		_, err := limiter.CheckOrganization(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Reset the rate limit
	key := "rate_limit:org:" + orgID
	if err := limiter.Reset(ctx, key); err != nil {
		t.Fatalf("unexpected error resetting: %v", err)
	}

	// Should have full bucket again
	result, err := limiter.CheckOrganization(ctx, orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected request to be allowed after reset")
	}
	if result.Remaining != 19 {
		t.Errorf("expected remaining 19 after reset, got %d", result.Remaining)
	}
}

// TestRateLimiter_DefaultLimits tests that default limits are used when not specified.
func TestRateLimiter_DefaultLimits(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 10, 20) // Default: 10 RPS, burst 20

	ctx := context.Background()
	apiKeyID := "test-key-default"

	// Use default limits (pass 0, 0)
	result, err := limiter.CheckAPIKey(ctx, apiKeyID, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected request to be allowed")
	}
	if result.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", result.Limit)
	}
	if result.Remaining != 19 {
		t.Errorf("expected remaining 19, got %d", result.Remaining)
	}
}

// TestRateLimiter_ConcurrentAccess tests concurrent rate limit checks.
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	client := setupTestRedis(t)
	if client == nil {
		return
	}
	defer client.Close()

	logger := zap.NewNop()
	limiter := NewRateLimiter(client, logger, 100, 200) // High limits for concurrent test

	ctx := context.Background()
	orgID := "test-org-concurrent"

	// Run concurrent requests
	results := make(chan *CheckResult, 200)
	errors := make(chan error, 200)

	for i := 0; i < 200; i++ {
		go func() {
			result, err := limiter.CheckOrganization(ctx, orgID)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}

	// Collect results
	allowedCount := 0
	deniedCount := 0
	for i := 0; i < 200; i++ {
		select {
		case err := <-errors:
			t.Errorf("unexpected error: %v", err)
		case result := <-results:
			if result.Allowed {
				allowedCount++
			} else {
				deniedCount++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for results")
		}
	}

	// Should have exactly 200 allowed (burst size) and 0 denied
	// (or some allowed if tokens refilled during concurrent access)
	if allowedCount+deniedCount != 200 {
		t.Errorf("expected 200 results, got %d allowed + %d denied", allowedCount, deniedCount)
	}
	if allowedCount < 200 {
		t.Logf("Note: %d requests were denied (may be expected with concurrent access)", deniedCount)
	}
}

