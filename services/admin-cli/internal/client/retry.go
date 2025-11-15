// Package client provides retry logic with exponential backoff for API clients.
//
// Purpose:
//
//	Handle transient failures (network timeouts, 5xx errors) with exponential backoff
//	retry strategy. Max 3 attempts with delays of 1s, 2s, 4s. Provides configurable
//	max attempts and timeout values.
//
// Dependencies:
//   - context: Timeout and cancellation
//   - time: Exponential backoff delays
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-009 (retry logic with exponential backoff)
//   - specs/009-admin-cli/spec.md#NFR-006 (retry handles transient failures: max 3 attempts, 1s/2s/4s delays)
//
package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxAttempts  int           // Max retry attempts (default: 3)
	InitialDelay time.Duration // Initial delay before first retry (default: 1s)
	MaxDelay     time.Duration // Maximum delay between retries (default: 4s)
	Timeout      time.Duration // Overall operation timeout
}

// DefaultRetryConfig returns default retry configuration per NFR-006.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     4 * time.Second,
		Timeout:      30 * time.Second,
	}
}

// DoWithRetry executes an HTTP request with retry logic.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, config RetryConfig) (*http.Response, error) {
	if config.MaxAttempts == 0 {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Execute request
		resp, err := client.Do(req)
		if err == nil && isSuccess(resp.StatusCode) {
			return resp, nil
		}

		// Check if error is retriable
		if err != nil && !isRetriableError(err) {
			return nil, err
		}

		if resp != nil && !isRetriableStatus(resp.StatusCode) {
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d: %v", resp.StatusCode, err)
		}

		// Don't wait after last attempt
		if attempt < config.MaxAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Exponential backoff: 1s, 2s, 4s
				delay = time.Duration(min(int64(delay)*2, int64(config.MaxDelay)))
			}
		}
	}

	return nil, fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// isRetriableError checks if an error is retriable.
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retriable
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}

// isRetriableStatus checks if an HTTP status code is retriable.
func isRetriableStatus(statusCode int) bool {
	// 5xx errors and 429 (Too Many Requests) are retriable
	return statusCode >= 500 || statusCode == http.StatusTooManyRequests
}

// isSuccess checks if an HTTP status code indicates success.
func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

