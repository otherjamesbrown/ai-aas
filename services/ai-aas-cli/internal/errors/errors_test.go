// Package errors provides tests for error handling.
package errors

import (
	"strings"
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

func TestUserConflictError(t *testing.T) {
	email := "test@example.com"
	err := NewUserConflictError(email)
	if err == nil {
		t.Fatal("NewUserConflictError() returned nil")
	}

	if err.Code != ErrCodeResourceConflict {
		t.Errorf("expected ErrCodeResourceConflict, got %s", err.Code)
	}

	if err.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", err.ExitCode)
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}

	// Log the actual error message for manual verification
	t.Logf("Error message:\n%s", errMsg)

	// Verify the error message includes the email
	if !strings.Contains(errMsg, email) {
		t.Errorf("error message should contain email %s, got: %s", email, errMsg)
	}

	// Verify the error message suggests --upsert
	if !strings.Contains(errMsg, "--upsert") {
		t.Errorf("error message should suggest --upsert flag, got: %s", errMsg)
	}
}
