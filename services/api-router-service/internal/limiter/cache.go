// Package limiter provides rate limiting and budget enforcement for API requests.
//
// Purpose:
//
//	This file implements an in-memory cache for rate limit status to minimize
//	calls to user-org-service for hot-path rate limit checks.
//
// Dependencies:
//   - TokenLimiter for fetching rate limits from user-org-service
//
// Key Responsibilities:
//   - Cache user rate limit status with configurable TTL
//   - Thread-safe access for concurrent requests
//   - Automatic cache invalidation on TTL expiry
//   - Optional background refresh for frequently accessed entries
//
// Requirements Reference:
//   - specs/035-token-rate-limits/spec.md#FR-015 (API Router caches user's effective policy)
//   - specs/035-token-rate-limits/spec.md (60-second cache TTL)
package limiter

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// RateLimitCache caches rate limit status per user.
type RateLimitCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
	logger  *zap.Logger
	limiter *TokenLimiter

	// singleflight prevents thundering herd for concurrent requests
	sf singleflight.Group

	// For background refresh
	stopCh   chan struct{}
	stopped  bool
	stopOnce sync.Once
}

type cacheEntry struct {
	status    *RateLimitStatus
	expiresAt time.Time
	fetchedAt time.Time
}

// RateLimitCacheConfig configures the RateLimitCache.
type RateLimitCacheConfig struct {
	TTL     time.Duration  // Cache entry TTL (default: 60s)
	Logger  *zap.Logger
	Limiter *TokenLimiter
}

// NewRateLimitCache creates a new rate limit cache.
func NewRateLimitCache(cfg RateLimitCacheConfig) *RateLimitCache {
	if cfg.TTL == 0 {
		cfg.TTL = 60 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	cache := &RateLimitCache{
		entries: make(map[string]*cacheEntry),
		ttl:     cfg.TTL,
		logger:  cfg.Logger,
		limiter: cfg.Limiter,
		stopCh:  make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// CheckLimit returns the cached rate limit status for a user, or fetches it if not cached.
// Uses singleflight to prevent thundering herd when multiple concurrent requests
// need to fetch the same user's rate limit status.
func (c *RateLimitCache) CheckLimit(ctx context.Context, userID string) (*RateLimitStatus, error) {
	// Check cache first
	c.mu.RLock()
	entry, ok := c.entries[userID]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		c.logger.Debug("rate limit cache hit",
			zap.String("user_id", userID),
			zap.Duration("age", time.Since(entry.fetchedAt)),
		)
		return entry.status, nil
	}

	// Cache miss or expired - fetch from limiter using singleflight
	c.logger.Debug("rate limit cache miss",
		zap.String("user_id", userID),
	)

	// Use singleflight to deduplicate concurrent requests for the same user
	result, err, _ := c.sf.Do(userID, func() (interface{}, error) {
		// Double-check cache in case another goroutine just populated it
		c.mu.RLock()
		entry, ok := c.entries[userID]
		c.mu.RUnlock()

		if ok && time.Now().Before(entry.expiresAt) {
			return entry.status, nil
		}

		return c.fetchAndCache(ctx, userID)
	})

	if err != nil {
		return nil, err
	}

	return result.(*RateLimitStatus), nil
}

// fetchAndCache fetches rate limit from the limiter and caches it.
func (c *RateLimitCache) fetchAndCache(ctx context.Context, userID string) (*RateLimitStatus, error) {
	status, err := c.limiter.CheckLimit(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	c.mu.Lock()
	c.entries[userID] = &cacheEntry{
		status:    status,
		expiresAt: now.Add(c.ttl),
		fetchedAt: now,
	}
	c.mu.Unlock()

	c.logger.Debug("rate limit cached",
		zap.String("user_id", userID),
		zap.Bool("allowed", status.Allowed),
		zap.Duration("ttl", c.ttl),
	)

	return status, nil
}

// Invalidate removes a user's entry from the cache.
// Call this after recording usage to ensure the next check fetches fresh data.
func (c *RateLimitCache) Invalidate(userID string) {
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()

	c.logger.Debug("rate limit cache invalidated",
		zap.String("user_id", userID),
	)
}

// InvalidateAll clears the entire cache.
func (c *RateLimitCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()

	c.logger.Debug("rate limit cache cleared")
}

// Size returns the number of entries in the cache.
func (c *RateLimitCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// cleanupLoop periodically removes expired entries from the cache.
func (c *RateLimitCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2) // Cleanup at half the TTL interval
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes expired entries from the cache.
func (c *RateLimitCache) cleanup() {
	now := time.Now()
	expired := 0

	c.mu.Lock()
	for userID, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, userID)
			expired++
		}
	}
	c.mu.Unlock()

	if expired > 0 {
		c.logger.Debug("rate limit cache cleanup",
			zap.Int("expired_entries", expired),
			zap.Int("remaining_entries", c.Size()),
		)
	}
}

// Stop stops the background cleanup goroutine.
func (c *RateLimitCache) Stop() {
	c.stopOnce.Do(func() {
		c.stopped = true
		close(c.stopCh)
	})
}

// GetCachedStatus returns the cached status for a user without fetching.
// Returns nil if not cached or expired.
func (c *RateLimitCache) GetCachedStatus(userID string) *RateLimitStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[userID]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.status
}

// Refresh fetches fresh rate limit status and updates the cache.
// This is useful for background refresh patterns.
func (c *RateLimitCache) Refresh(ctx context.Context, userID string) (*RateLimitStatus, error) {
	return c.fetchAndCache(ctx, userID)
}

// CachedTokenLimiter wraps TokenLimiter with caching.
// This provides a drop-in replacement that adds caching to CheckLimit.
type CachedTokenLimiter struct {
	*TokenLimiter
	cache *RateLimitCache
}

// NewCachedTokenLimiter creates a new cached token limiter.
func NewCachedTokenLimiter(limiter *TokenLimiter, cacheTTL time.Duration, logger *zap.Logger) *CachedTokenLimiter {
	cache := NewRateLimitCache(RateLimitCacheConfig{
		TTL:     cacheTTL,
		Logger:  logger,
		Limiter: limiter,
	})

	return &CachedTokenLimiter{
		TokenLimiter: limiter,
		cache:        cache,
	}
}

// CheckLimit returns the cached rate limit status for a user.
func (c *CachedTokenLimiter) CheckLimit(ctx context.Context, userID string) (*RateLimitStatus, error) {
	return c.cache.CheckLimit(ctx, userID)
}

// RecordUsage records token usage and invalidates the cache for the user.
func (c *CachedTokenLimiter) RecordUsage(ctx context.Context, userID string, tokens int64) error {
	err := c.TokenLimiter.RecordUsage(ctx, userID, tokens)
	if err == nil {
		// Invalidate cache after successful recording so next check gets fresh data
		c.cache.Invalidate(userID)
	}
	return err
}

// Stop stops the cache cleanup goroutine.
func (c *CachedTokenLimiter) Stop() {
	c.cache.Stop()
}

// Cache returns the underlying cache for inspection/testing.
func (c *CachedTokenLimiter) Cache() *RateLimitCache {
	return c.cache
}
