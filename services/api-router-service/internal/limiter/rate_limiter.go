// Package limiter provides rate limiting and budget enforcement for API requests.
//
// Purpose:
//   This package implements Redis-backed token bucket rate limiting and budget
//   checking to enforce fair usage and prevent over-spending.
//
// Dependencies:
//   - github.com/redis/go-redis/v9: Redis client for token bucket storage
//
// Key Responsibilities:
//   - Token bucket rate limiting per organization and per API key
//   - Configurable RPS and burst size
//   - Thread-safe operations
//   - Fast sub-millisecond checks
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-002 (Enforce budgets and safe usage)
//
package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimiter implements token bucket rate limiting using Redis.
type RateLimiter struct {
	client    *redis.Client
	logger    *zap.Logger
	defaultRPS int
	burstSize  int
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(client *redis.Client, logger *zap.Logger, defaultRPS, burstSize int) *RateLimiter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RateLimiter{
		client:     client,
		logger:     logger,
		defaultRPS: defaultRPS,
		burstSize:  burstSize,
	}
}

// CheckResult represents the result of a rate limit check.
type CheckResult struct {
	Allowed      bool
	RetryAfter   time.Duration
	Remaining    int
	Limit        int
}

// CheckOrganization checks if a request from an organization is allowed.
// Returns CheckResult with allowed status and retry information.
func (r *RateLimiter) CheckOrganization(ctx context.Context, orgID string) (*CheckResult, error) {
	return r.check(ctx, fmt.Sprintf("rate_limit:org:%s", orgID), r.defaultRPS, r.burstSize)
}

// CheckAPIKey checks if a request from an API key is allowed.
// Returns CheckResult with allowed status and retry information.
func (r *RateLimiter) CheckAPIKey(ctx context.Context, apiKeyID string, rps, burst int) (*CheckResult, error) {
	// Use provided limits or fall back to defaults
	if rps <= 0 {
		rps = r.defaultRPS
	}
	if burst <= 0 {
		burst = r.burstSize
	}
	return r.check(ctx, fmt.Sprintf("rate_limit:key:%s", apiKeyID), rps, burst)
}

// check performs the token bucket check using Redis.
// Implements token bucket algorithm:
// - Each bucket has a capacity (burst size)
// - Tokens refill at rate RPS per second
// - Each request consumes one token
// - If no tokens available, request is denied
func (r *RateLimiter) check(ctx context.Context, key string, rps, burst int) (*CheckResult, error) {
	now := time.Now()
	nowUnix := now.Unix()
	
	// Calculate refill interval (seconds per token)
	refillInterval := float64(1) / float64(rps)
	
	// Use Redis Lua script for atomic token bucket operation
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local refill_interval = tonumber(ARGV[2])
		local burst = tonumber(ARGV[3])
		
		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1]) or burst
		local last_refill = tonumber(bucket[2]) or now
		
		-- Calculate tokens to add based on time elapsed
		local elapsed = now - last_refill
		local tokens_to_add = math.floor(elapsed / refill_interval)
		
		-- Refill tokens (cap at burst size)
		tokens = math.min(burst, tokens + tokens_to_add)
		
		-- Check if request is allowed
		if tokens >= 1 then
			tokens = tokens - 1
			redis.call('HSET', key, 'tokens', tokens, 'last_refill', now)
			redis.call('EXPIRE', key, 3600) -- Expire after 1 hour of inactivity
			return {1, tokens, burst} -- allowed=1, remaining, limit
		else
			-- Calculate retry after (time until next token available)
			local time_until_next = refill_interval - (elapsed % refill_interval)
			redis.call('HSET', key, 'tokens', tokens, 'last_refill', now)
			redis.call('EXPIRE', key, 3600)
			return {0, tokens, burst, time_until_next} -- allowed=0, remaining, limit, retry_after
		end
	`
	
	result, err := r.client.Eval(ctx, script, []string{key}, nowUnix, refillInterval, burst).Result()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, create it with full bucket
			tokens := burst - 1
			r.client.HSet(ctx, key, "tokens", tokens, "last_refill", nowUnix)
			r.client.Expire(ctx, key, time.Hour)
			return &CheckResult{
				Allowed:    true,
				Remaining:  tokens,
				Limit:      burst,
			}, nil
		}
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}
	
	// Parse result from Lua script
	results, ok := result.([]interface{})
	if !ok || len(results) < 3 {
		return nil, fmt.Errorf("unexpected rate limit result format")
	}
	
	allowed := results[0].(int64) == 1
	remaining := int(results[1].(int64))
	limit := int(results[2].(int64))
	
	checkResult := &CheckResult{
		Allowed:   allowed,
		Remaining: remaining,
		Limit:     limit,
	}
	
	// If denied, calculate retry after
	if !allowed && len(results) >= 4 {
		retryAfterSeconds := results[3].(float64)
		checkResult.RetryAfter = time.Duration(retryAfterSeconds * float64(time.Second))
	}
	
	return checkResult, nil
}

// Reset resets the rate limit for a given key (useful for testing).
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

