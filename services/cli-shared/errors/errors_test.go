package errors

import (
	"testing"
)

func TestCLIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CLIError
		contains []string
	}{
		{
			name: "message only",
			err: &CLIError{
				Code:    ErrCodeOperationFailed,
				Message: "Test error",
			},
			contains: []string{"Test error"},
		},
		{
			name: "message with details",
			err: &CLIError{
				Code:    ErrCodeOperationFailed,
				Message: "Test error",
				Details: "additional info",
			},
			contains: []string{"Test error", "additional info"},
		},
		{
			name: "message with suggestion",
			err: &CLIError{
				Code:       ErrCodeOperationFailed,
				Message:    "Test error",
				Suggestion: "Try this",
			},
			contains: []string{"Test error", "Suggestion:", "Try this"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, s := range tt.contains {
				if !containsString(errMsg, s) {
					t.Errorf("expected error message to contain %q, got %q", s, errMsg)
				}
			}
		})
	}
}

func TestNewServiceUnavailableError(t *testing.T) {
	err := NewServiceUnavailableError("test-service", "http://test:8080")

	if err.Code != ErrCodeServiceUnavailable {
		t.Errorf("expected code %s, got %s", ErrCodeServiceUnavailable, err.Code)
	}
	if err.ExitCode != ExitServiceError {
		t.Errorf("expected exit code %d, got %d", ExitServiceError, err.ExitCode)
	}
	if err.Suggestion == "" {
		t.Error("expected non-empty suggestion")
	}
}

func TestNewAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("token expired")

	if err.Code != ErrCodeAuthenticationFailed {
		t.Errorf("expected code %s, got %s", ErrCodeAuthenticationFailed, err.Code)
	}
	if err.ExitCode != ExitAuthError {
		t.Errorf("expected exit code %d, got %d", ExitAuthError, err.ExitCode)
	}
}

func TestNewAuthorizationError(t *testing.T) {
	err := NewAuthorizationError("delete", "user")

	if err.Code != ErrCodeAuthorizationFailed {
		t.Errorf("expected code %s, got %s", ErrCodeAuthorizationFailed, err.Code)
	}
	if err.ExitCode != ExitAuthError {
		t.Errorf("expected exit code %d, got %d", ExitAuthError, err.ExitCode)
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("invalid email", "use format user@domain.com")

	if err.Code != ErrCodeValidationFailed {
		t.Errorf("expected code %s, got %s", ErrCodeValidationFailed, err.Code)
	}
	if err.ExitCode != ExitUsageError {
		t.Errorf("expected exit code %d, got %d", ExitUsageError, err.ExitCode)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("user", "john@example.com")

	if err.Code != ErrCodeNotFound {
		t.Errorf("expected code %s, got %s", ErrCodeNotFound, err.Code)
	}
	if err.ExitCode != ExitNotFound {
		t.Errorf("expected exit code %d, got %d", ExitNotFound, err.ExitCode)
	}
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("API key", "key-123")

	if err.Code != ErrCodeConflict {
		t.Errorf("expected code %s, got %s", ErrCodeConflict, err.Code)
	}
	if err.ExitCode != ExitConflict {
		t.Errorf("expected exit code %d, got %d", ExitConflict, err.ExitCode)
	}
}

func TestWithDetails(t *testing.T) {
	original := NewOperationError("failed", "try again")
	modified := original.WithDetails("specific reason")

	if modified.Details != "specific reason" {
		t.Errorf("expected details %q, got %q", "specific reason", modified.Details)
	}
	// Original should be unchanged
	if original.Details != "failed" {
		t.Errorf("original was modified")
	}
}

func TestWithSuggestion(t *testing.T) {
	original := NewOperationError("failed", "try again")
	modified := original.WithSuggestion("do something else")

	if modified.Suggestion != "do something else" {
		t.Errorf("expected suggestion %q, got %q", "do something else", modified.Suggestion)
	}
}

func TestIsCLIError(t *testing.T) {
	cliErr := NewOperationError("test", "suggestion")
	if !IsCLIError(cliErr) {
		t.Error("expected IsCLIError to return true for CLIError")
	}

	stdErr := &testError{msg: "standard error"}
	if IsCLIError(stdErr) {
		t.Error("expected IsCLIError to return false for standard error")
	}
}

func TestGetExitCode(t *testing.T) {
	cliErr := NewNotFoundError("resource", "id")
	if code := GetExitCode(cliErr); code != ExitNotFound {
		t.Errorf("expected exit code %d, got %d", ExitNotFound, code)
	}

	stdErr := &testError{msg: "standard error"}
	if code := GetExitCode(stdErr); code != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, code)
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
