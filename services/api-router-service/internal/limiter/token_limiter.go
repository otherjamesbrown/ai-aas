// Package limiter provides rate limiting and budget enforcement for API requests.
//
// Purpose:
//
//	This file implements TokenLimiter which calls user-org-service to check
//	token rate limits and record usage for users.
//
// Dependencies:
//   - user-org-service internal endpoints for rate limit checks and usage recording
//
// Key Responsibilities:
//   - Check token rate limits before processing requests
//   - Record token usage after request completion
//   - Handle user-org-service unavailability gracefully
//   - Return structured rate limit status with window details
//
// Requirements Reference:
//   - specs/035-token-rate-limits/spec.md#FR-014 (Enforcement at API Router)
//   - specs/035-token-rate-limits/spec.md#FR-015 (Caching)
//   - specs/035-token-rate-limits/spec.md#FR-016 (Pre-request rejection)
//   - specs/035-token-rate-limits/spec.md#FR-017 (Post-request recording)
package limiter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// TokenLimiter checks token rate limits and records usage via user-org-service.
type TokenLimiter struct {
	endpoint   string
	timeout    time.Duration
	logger     *zap.Logger
	client     *http.Client
	m2mAPIKey  string // M2M API key for internal service-to-service calls
}

// TokenLimiterConfig configures the TokenLimiter.
type TokenLimiterConfig struct {
	Endpoint  string        // Base URL of user-org-service
	Timeout   time.Duration // HTTP request timeout
	Logger    *zap.Logger
	M2MAPIKey string // API key for M2M authentication
}

// NewTokenLimiter creates a new token limiter.
func NewTokenLimiter(cfg TokenLimiterConfig) *TokenLimiter {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &TokenLimiter{
		endpoint:  cfg.Endpoint,
		timeout:   cfg.Timeout,
		logger:    cfg.Logger,
		m2mAPIKey: cfg.M2MAPIKey,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// rateLimitResponse represents the response from user-org-service rate limit check.
type rateLimitResponse struct {
	Allowed        bool             `json:"allowed"`
	Windows        []windowResponse `json:"windows"`
	BlockingWindow *windowResponse  `json:"blocking_window,omitempty"`
}

type windowResponse struct {
	Window     string  `json:"window"`
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percentage float64 `json:"percentage"`
	ResetsAt   string  `json:"resets_at"`
}

// CheckLimit checks if a user is within their token rate limits.
// Returns RateLimitStatus with allowed status and window details.
// On error (service unavailable), returns allowed=true for graceful degradation.
func (l *TokenLimiter) CheckLimit(ctx context.Context, userID string) (*RateLimitStatus, error) {
	// If no endpoint configured, allow all requests (graceful degradation)
	if l.endpoint == "" {
		l.logger.Debug("token limiter not configured, allowing request",
			zap.String("user_id", userID),
		)
		return &RateLimitStatus{
			Allowed: true,
			Windows: []WindowStatus{},
		}, nil
	}

	url := fmt.Sprintf("%s/internal/v1/users/%s/rate-limit", l.endpoint, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create rate limit check request: %w", err)
	}

	// Add M2M authentication
	if l.m2mAPIKey != "" {
		req.Header.Set("X-API-Key", l.m2mAPIKey)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		l.logger.Warn("user-org-service unavailable, allowing request (graceful degradation)",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		// Graceful degradation: allow request if user-org-service unavailable
		return &RateLimitStatus{
			Allowed: true,
			Windows: []WindowStatus{},
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle non-200 responses
	if resp.StatusCode == http.StatusNotFound {
		// User not found - allow request (might be a new user or service account)
		l.logger.Debug("user not found in rate limit check, allowing request",
			zap.String("user_id", userID),
		)
		return &RateLimitStatus{
			Allowed: true,
			Windows: []WindowStatus{},
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		l.logger.Warn("user-org-service returned error, allowing request (graceful degradation)",
			zap.String("user_id", userID),
			zap.Int("status", resp.StatusCode),
		)
		// Graceful degradation
		return &RateLimitStatus{
			Allowed: true,
			Windows: []WindowStatus{},
		}, nil
	}

	var rlResp rateLimitResponse
	if err := json.NewDecoder(resp.Body).Decode(&rlResp); err != nil {
		return nil, fmt.Errorf("decode rate limit response: %w", err)
	}

	status := &RateLimitStatus{
		Allowed: rlResp.Allowed,
		Windows: make([]WindowStatus, len(rlResp.Windows)),
	}

	// Convert windows
	for i, w := range rlResp.Windows {
		resetsAt, _ := time.Parse(time.RFC3339, w.ResetsAt)
		status.Windows[i] = WindowStatus{
			Window:     w.Window,
			Limit:      w.Limit,
			Used:       w.Used,
			Remaining:  w.Remaining,
			Percentage: w.Percentage,
			ResetsAt:   resetsAt,
		}
	}

	// Convert blocking window if present
	if rlResp.BlockingWindow != nil {
		resetsAt, _ := time.Parse(time.RFC3339, rlResp.BlockingWindow.ResetsAt)
		status.BlockingWindow = &WindowStatus{
			Window:     rlResp.BlockingWindow.Window,
			Limit:      rlResp.BlockingWindow.Limit,
			Used:       rlResp.BlockingWindow.Used,
			Remaining:  rlResp.BlockingWindow.Remaining,
			Percentage: rlResp.BlockingWindow.Percentage,
			ResetsAt:   resetsAt,
		}
	}

	return status, nil
}

// RecordUsage reports token usage to user-org-service.
// This should be called asynchronously after a request completes.
// Errors are logged but not returned (fire-and-forget).
func (l *TokenLimiter) RecordUsage(ctx context.Context, userID string, tokens int64) error {
	// If no endpoint configured, skip recording
	if l.endpoint == "" {
		return nil
	}

	// Skip if no tokens to record
	if tokens <= 0 {
		return nil
	}

	url := fmt.Sprintf("%s/internal/v1/usage/tokens", l.endpoint)

	body := RecordUsageRequest{
		UserID: userID,
		Tokens: tokens,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		l.logger.Error("failed to marshal usage request",
			zap.Error(err),
			zap.String("user_id", userID),
			zap.Int64("tokens", tokens),
		)
		return fmt.Errorf("marshal usage request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create usage record request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add M2M authentication
	if l.m2mAPIKey != "" {
		req.Header.Set("X-API-Key", l.m2mAPIKey)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		l.logger.Warn("failed to record token usage (user-org-service unavailable)",
			zap.Error(err),
			zap.String("user_id", userID),
			zap.Int64("tokens", tokens),
		)
		return fmt.Errorf("record usage request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		l.logger.Warn("failed to record token usage (unexpected status)",
			zap.Int("status", resp.StatusCode),
			zap.String("user_id", userID),
			zap.Int64("tokens", tokens),
		)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	l.logger.Debug("token usage recorded",
		zap.String("user_id", userID),
		zap.Int64("tokens", tokens),
	)

	return nil
}

// IsConfigured returns true if the TokenLimiter is configured with an endpoint.
func (l *TokenLimiter) IsConfigured() bool {
	return l.endpoint != ""
}
