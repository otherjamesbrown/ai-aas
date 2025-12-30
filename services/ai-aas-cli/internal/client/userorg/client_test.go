// Package userorg provides tests for the user-org client.
package userorg

import (
	"errors"
	"testing"
)

func TestUserConflictError(t *testing.T) {
	email := "test@example.com"
	err := &UserConflictError{Email: email}

	// Verify error message
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}

	expectedMsg := "user with email test@example.com already exists"
	if errMsg != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, errMsg)
	}
}

func TestIsUserConflictError(t *testing.T) {
	// Test with UserConflictError
	conflictErr := &UserConflictError{Email: "test@example.com"}
	if !IsUserConflictError(conflictErr) {
		t.Error("IsUserConflictError should return true for UserConflictError")
	}

	// Test with wrapped UserConflictError
	wrappedErr := errors.New("wrapped error")
	if IsUserConflictError(wrappedErr) {
		t.Error("IsUserConflictError should return false for non-UserConflictError")
	}

	// Test with nil
	if IsUserConflictError(nil) {
		t.Error("IsUserConflictError should return false for nil error")
	}
}
