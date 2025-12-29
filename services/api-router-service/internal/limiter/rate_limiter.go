// Package limiter provides rate limiting and budget enforcement for API requests.
//
// Purpose:
//
//	This package implements Redis-backed token bucket rate limiting and budget
//	checking to enforce fair usage and prevent over-spending.
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
	client     *redis.Client
	logger     *zap.Logger
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
	Allowed    bool
	RetryAfter time.Duration
	Remaining  int
	Limit      int
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
	// Use Unix timestamp with fractional seconds for precision (millisecond precision)
	nowUnixFloat := float64(now.UnixNano()) / float64(time.Second)

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

	result, err := r.client.Eval(ctx, script, []string{key}, nowUnixFloat, refillInterval, burst).Result()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, create it with full bucket
			tokens := burst - 1
			r.client.HSet(ctx, key, "tokens", tokens, "last_refill", nowUnixFloat)
			r.client.Expire(ctx, key, time.Hour)
			return &CheckResult{
				Allowed:   true,
				Remaining: tokens,
				Limit:     burst,
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
	if !allowed {
		refillInterval := float64(1) / float64(rps)
		var retryAfterSeconds float64

		if len(results) >= 4 {
			switch v := results[3].(type) {
			case float64:
				retryAfterSeconds = v
			case int64:
				retryAfterSeconds = float64(v)
			default:
				// Unexpected type, log and calculate default retry_after
				r.logger.Warn("unexpected retry_after type from Redis", zap.Any("type", v), zap.Any("value", v))
				retryAfterSeconds = refillInterval
			}
		} else {
			// Lua script didn't return retry_after, calculate it based on refill_interval
			r.logger.Warn("retry_after not returned from Redis Lua script, using default")
			retryAfterSeconds = refillInterval
		}

		// Ensure retry_after is at least one refill_interval (safeguard against 0 or negative values)
		if retryAfterSeconds <= 0 {
			retryAfterSeconds = refillInterval
		}

		checkResult.RetryAfter = time.Duration(retryAfterSeconds * float64(time.Second))
	}

	return checkResult, nil
}

// Reset resets the rate limit for a given key (useful for testing).
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// TokenQuotaLimits represents token usage limits for different time periods.
type TokenQuotaLimits struct {
	Hourly int
	Daily  int
	Weekly int
}

// TokenQuotaStatus represents the current token usage status.
type TokenQuotaStatus struct {
	Allowed      bool
	Exceeded     bool
	Period       string // "hour", "day", or "week"
	CurrentUsage int
	Limit        int
	ResetAt      time.Time
}

// CheckTokenQuota checks if an organization is within their token quota limits.
// Returns TokenQuotaStatus indicating if the request should be allowed.
func (r *RateLimiter) CheckTokenQuota(ctx context.Context, orgID string, limits TokenQuotaLimits) (*TokenQuotaStatus, error) {
	now := time.Now()

	// Check hourly limit
	if limits.Hourly > 0 {
		hourlyKey := tokenKey(orgID, "hour", now)
		hourlyUsage, err := r.client.Get(ctx, hourlyKey).Int()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("get hourly token usage: %w", err)
		}
		if hourlyUsage >= limits.Hourly {
			resetAt := now.Truncate(time.Hour).Add(time.Hour)
			return &TokenQuotaStatus{
				Allowed:      false,
				Exceeded:     true,
				Period:       "hour",
				CurrentUsage: hourlyUsage,
				Limit:        limits.Hourly,
				ResetAt:      resetAt,
			}, nil
		}
	}

	// Check daily limit
	if limits.Daily > 0 {
		dailyKey := tokenKey(orgID, "day", now)
		dailyUsage, err := r.client.Get(ctx, dailyKey).Int()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("get daily token usage: %w", err)
		}
		if dailyUsage >= limits.Daily {
			resetAt := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
			return &TokenQuotaStatus{
				Allowed:      false,
				Exceeded:     true,
				Period:       "day",
				CurrentUsage: dailyUsage,
				Limit:        limits.Daily,
				ResetAt:      resetAt,
			}, nil
		}
	}

	// Check weekly limit
	if limits.Weekly > 0 {
		weeklyKey := tokenKey(orgID, "week", now)
		weeklyUsage, err := r.client.Get(ctx, weeklyKey).Int()
		if err != nil && err != redis.Nil {
			return nil, fmt.Errorf("get weekly token usage: %w", err)
		}
		if weeklyUsage >= limits.Weekly {
			// Reset on Sunday at midnight (start of week)
			daysUntilSunday := (7 - int(now.Weekday())) % 7
			if daysUntilSunday == 0 {
				daysUntilSunday = 7
			}
			resetAt := now.Truncate(24*time.Hour).AddDate(0, 0, daysUntilSunday)
			return &TokenQuotaStatus{
				Allowed:      false,
				Exceeded:     true,
				Period:       "week",
				CurrentUsage: weeklyUsage,
				Limit:        limits.Weekly,
				ResetAt:      resetAt,
			}, nil
		}
	}

	return &TokenQuotaStatus{
		Allowed:  true,
		Exceeded: false,
	}, nil
}

// RecordTokenUsage increments token usage counters for an organization.
// Updates hourly, daily, and weekly counters with appropriate TTLs.
func (r *RateLimiter) RecordTokenUsage(ctx context.Context, orgID string, tokens int) error {
	if tokens <= 0 {
		return nil
	}

	now := time.Now()
	pipe := r.client.Pipeline()

	// Increment hourly counter
	hourlyKey := tokenKey(orgID, "hour", now)
	pipe.IncrBy(ctx, hourlyKey, int64(tokens))
	pipe.Expire(ctx, hourlyKey, time.Hour)

	// Increment daily counter
	dailyKey := tokenKey(orgID, "day", now)
	pipe.IncrBy(ctx, dailyKey, int64(tokens))
	pipe.Expire(ctx, dailyKey, 24*time.Hour)

	// Increment weekly counter
	weeklyKey := tokenKey(orgID, "week", now)
	pipe.IncrBy(ctx, weeklyKey, int64(tokens))
	pipe.Expire(ctx, weeklyKey, 7*24*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("record token usage: %w", err)
	}

	return nil
}

// tokenKey generates a Redis key for token usage tracking.
// The key includes a time bucket based on the period (hour/day/week).
func tokenKey(orgID, period string, t time.Time) string {
	var bucket int64
	switch period {
	case "hour":
		bucket = t.Truncate(time.Hour).Unix()
	case "day":
		bucket = t.Truncate(24 * time.Hour).Unix()
	case "week":
		// Start of week (Sunday)
		startOfWeek := t.AddDate(0, 0, -int(t.Weekday()))
		bucket = startOfWeek.Truncate(24 * time.Hour).Unix()
	default:
		bucket = t.Unix()
	}
	return fmt.Sprintf("ratelimit:tokens:%s:%s:%d", orgID, period, bucket)
}
