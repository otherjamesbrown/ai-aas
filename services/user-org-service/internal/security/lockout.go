// Package security provides lockout tracking and enforcement.
//
// Purpose:
//   This package implements account lockout tracking using Redis to count
//   failed authentication attempts and enforce lockout policies.
//
// Dependencies:
//   - github.com/redis/go-redis/v9: Redis client for tracking attempts
//   - internal/config: Lockout configuration (max attempts, duration, window)
//
// Key Responsibilities:
//   - TrackFailedAttempt: Increment failed attempt counter for a user
//   - CheckLockout: Determine if user should be locked out
//   - RecordLockout: Set lockout_until timestamp in database
//   - ClearAttempts: Reset failed attempt counter on successful login
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-008 (Account Lockout)
//
package security

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// LockoutTracker tracks failed authentication attempts and enforces lockout policies.
type LockoutTracker struct {
	client *redis.Client
	cfg    LockoutConfig
}

// LockoutConfig contains lockout policy configuration.
type LockoutConfig struct {
	MaxAttempts      int           // Maximum failed attempts before lockout
	LockoutDuration  time.Duration // Duration of lockout
	WindowDuration   time.Duration // Time window for counting attempts
}

// NewLockoutTracker creates a new lockout tracker.
func NewLockoutTracker(client *redis.Client, cfg LockoutConfig) *LockoutTracker {
	return &LockoutTracker{
		client: client,
		cfg:    cfg,
	}
}

// key returns the Redis key for tracking failed attempts for a user (by email or userID).
func (t *LockoutTracker) key(identifier string) string {
	return fmt.Sprintf("lockout:attempts:%s", identifier)
}

// TrackFailedAttempt increments the failed attempt counter for a user (by email or userID).
// Returns the current count and whether lockout should be triggered.
func (t *LockoutTracker) TrackFailedAttempt(ctx context.Context, identifier string) (int, bool, error) {
	if t.client == nil {
		// No Redis client, skip tracking (graceful degradation)
		return 0, false, nil
	}

	key := t.key(identifier)
	
	// Increment counter with expiration set to window duration
	pipe := t.client.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, t.cfg.WindowDuration)
	results, err := pipe.Exec(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("lockout tracker: failed to increment counter: %w", err)
	}

	count := results[0].(*redis.IntCmd).Val()
	shouldLockout := count >= int64(t.cfg.MaxAttempts)
	
	return int(count), shouldLockout, nil
}

// GetFailedAttemptCount returns the current failed attempt count for a user (by email or userID).
func (t *LockoutTracker) GetFailedAttemptCount(ctx context.Context, identifier string) (int, error) {
	if t.client == nil {
		return 0, nil
	}

	key := t.key(identifier)
	count, err := t.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lockout tracker: failed to get count: %w", err)
	}
	return count, nil
}

// ClearAttempts resets the failed attempt counter for a user (called on successful login).
// Clears both email-based and userID-based keys.
func (t *LockoutTracker) ClearAttempts(ctx context.Context, email string, userID uuid.UUID) error {
	if t.client == nil {
		return nil
	}

	// Clear both email and userID keys (in case we tracked by either)
	pipe := t.client.TxPipeline()
	pipe.Del(ctx, t.key(email))
	pipe.Del(ctx, t.key(userID.String()))
	_, err := pipe.Exec(ctx)
	return err
}

// CalculateLockoutUntil calculates the lockout expiration time.
func (t *LockoutTracker) CalculateLockoutUntil() time.Time {
	return time.Now().Add(t.cfg.LockoutDuration)
}

