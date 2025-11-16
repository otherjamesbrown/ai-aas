// Package health provides tests for health checking.
package health

import (
	"testing"
	"time"
)

func TestNewChecker(t *testing.T) {
	checker := NewChecker(5 * time.Second)
	if checker == nil {
		t.Fatal("NewChecker() returned nil")
	}

	if checker.timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", checker.timeout)
	}
}

func TestNewCheckerDefault(t *testing.T) {
	checker := NewChecker(0)
	if checker.timeout != 5*time.Second {
		t.Errorf("expected default 5s timeout, got %v", checker.timeout)
	}
}

// Note: Full integration tests for CheckService would require a running service
// These are covered in integration tests (Phase 3)

