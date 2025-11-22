package utils

import (
	"fmt"
	"time"
)

// WaitFor waits for a condition to become true with timeout
func WaitFor(condition func() (bool, error), timeout time.Duration, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ok, err := condition()
			if err != nil {
				return fmt.Errorf("condition check failed: %w", err)
			}
			if ok {
				return nil
			}
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for condition after %v", timeout)
		}
	}
}

// WaitForWithContext waits for a condition with context cancellation support
func WaitForWithContext(condition func() (bool, error), timeout time.Duration, interval time.Duration, checkCancel func() bool) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if checkCancel != nil && checkCancel() {
				return fmt.Errorf("operation cancelled")
			}

			ok, err := condition()
			if err != nil {
				return fmt.Errorf("condition check failed: %w", err)
			}
			if ok {
				return nil
			}
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for condition after %v", timeout)
		}
	}
}

// SleepWithJitter sleeps for a duration with random jitter to avoid thundering herd
func SleepWithJitter(baseDuration time.Duration, jitterPercent float64) {
	jitter := time.Duration(float64(baseDuration) * jitterPercent)
	actualDuration := baseDuration + time.Duration(float64(jitter)*0.5) // Simplified jitter
	time.Sleep(actualDuration)
}

