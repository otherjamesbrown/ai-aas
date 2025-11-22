package utils

import (
	"fmt"
	"time"
)

// Retry executes a function with exponential backoff retry logic
func Retry(maxAttempts int, delay time.Duration, multiplier float64, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(float64(delay) * float64(attempt) * multiplier)
			time.Sleep(backoffDelay)
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("max attempts (%d) exceeded: %w", maxAttempts, lastErr)
}

// RetryWithContext executes a function with retry logic and context cancellation
func RetryWithContext(maxAttempts int, delay time.Duration, multiplier float64, fn func() error, shouldRetry func(error) bool) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoffDelay := time.Duration(float64(delay) * float64(attempt) * multiplier)
			time.Sleep(backoffDelay)
		}

		err := fn()
		if err == nil {
			return nil
		}

		if !shouldRetry(err) {
			return err
		}

		lastErr = err
	}

	return fmt.Errorf("max attempts (%d) exceeded: %w", maxAttempts, lastErr)
}

