// Package userorg provides authentication utilities for user-org-service API client.
//
// Purpose:
//
//	Token validation with clock skew tolerance (±5 minutes) to handle system clock drift
//	affecting token validity. Provides clear errors if tokens are invalid due to clock issues.
//
// Dependencies:
//   - time: Time validation with clock skew
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#NFR-011 (token validation with clear error messages)
//   - specs/009-admin-cli/spec.md#Edge-Cases (time synchronization)
//
package userorg

import (
	"errors"
	"time"
)

const (
	// ClockSkewTolerance is the acceptable clock skew for token validation (±5 minutes).
	ClockSkewTolerance = 5 * time.Minute
)

// ValidateToken validates a token expiration with clock skew tolerance.
func ValidateToken(expiresAt time.Time) error {
	now := time.Now()
	skewedExpiry := expiresAt.Add(ClockSkewTolerance)

	if now.After(skewedExpiry) {
		return errors.New("token expired (may be due to clock drift - check system time)")
	}

	// Check if token is too far in the future (also indicates clock issues)
	skewedNow := now.Add(ClockSkewTolerance)
	if expiresAt.After(skewedNow.Add(24 * time.Hour)) {
		return errors.New("token expiration too far in future (check system time)")
	}

	return nil
}

