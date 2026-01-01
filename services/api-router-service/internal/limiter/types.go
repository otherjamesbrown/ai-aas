// Package limiter provides rate limiting and budget enforcement for API requests.
//
// Purpose:
//
//	This package defines shared types for rate limiting and token quota enforcement.
//
// Requirements Reference:
//   - specs/035-token-rate-limits/spec.md
package limiter

import (
	"time"
)

// WindowStatus represents the status of a single rate limit window.
type WindowStatus struct {
	Window    string    `json:"window"`    // "1h", "24h", "7d"
	Limit     int64     `json:"limit"`     // Token limit for this window
	Used      int64     `json:"used"`      // Tokens consumed in this window
	Remaining int64     `json:"remaining"` // Tokens remaining
	Percentage float64  `json:"percentage"` // Percentage used (0-100)
	ResetsAt  time.Time `json:"resets_at"` // When this window resets
}

// RateLimitStatus represents the rate limit status for a user.
type RateLimitStatus struct {
	Allowed        bool           `json:"allowed"`                   // Whether the request is allowed
	Windows        []WindowStatus `json:"windows"`                   // All windows with their status
	BlockingWindow *WindowStatus  `json:"blocking_window,omitempty"` // Which window is blocking (if any)
}

// RecordUsageRequest represents a request to record token usage.
type RecordUsageRequest struct {
	UserID         string `json:"user_id"`
	Tokens         int64  `json:"tokens"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}
