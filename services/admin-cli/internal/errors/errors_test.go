// Package errors provides tests for error handling.
package errors

import (
	"testing"
)

func TestCLIError(t *testing.T) {
	err := NewServiceUnavailableError("test-service", "http://test:8081")
	if err == nil {
		t.Fatal("NewServiceUnavailableError() returned nil")
	}

	if err.Code != ErrCodeServiceUnavailable {
		t.Errorf("expected ErrCodeServiceUnavailable, got %s", err.Code)
	}

	if err.ExitCode != 3 {
		t.Errorf("expected exit code 3, got %d", err.ExitCode)
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("token expired")
	if err == nil {
		t.Fatal("NewAuthenticationError() returned nil")
	}

	if err.Code != ErrCodeAuthenticationFailed {
		t.Errorf("expected ErrCodeAuthenticationFailed, got %s", err.Code)
	}

	if err.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", err.ExitCode)
	}
}

