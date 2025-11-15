// Package errors provides structured error types and recovery suggestions.
//
// Purpose:
//
//	Define consistent error types across all CLI commands with recovery suggestions
//	and clear error messages. Enables consistent error handling and user guidance.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#NFR-014 (clear, actionable error messages)
//   - specs/009-admin-cli/spec.md#NFR-023 (error handling consistency)
//
package errors

import (
	"fmt"
)

// ErrorCode represents a standardized error code.
type ErrorCode string

const (
	// ErrCodeServiceUnavailable indicates a required service is unavailable.
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	// ErrCodeAuthenticationFailed indicates authentication failure.
	ErrCodeAuthenticationFailed ErrorCode = "AUTHENTICATION_FAILED"
	// ErrCodeValidationFailed indicates input validation failure.
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	// ErrCodeOperationFailed indicates a general operation failure.
	ErrCodeOperationFailed ErrorCode = "OPERATION_FAILED"
	// ErrCodeUsage indicates incorrect command usage.
	ErrCodeUsage ErrorCode = "USAGE_ERROR"
)

// CLIError represents a structured CLI error with recovery suggestions.
type CLIError struct {
	Code       ErrorCode
	Message    string
	Suggestion string
	Details    string
	ExitCode   int // Exit code for scriptability (NFR-019)
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	msg := e.Message
	if e.Details != "" {
		msg += ": " + e.Details
	}
	if e.Suggestion != "" {
		msg += "\n\nSuggestion: " + e.Suggestion
	}
	return msg
}

// NewServiceUnavailableError creates an error for service unavailability.
func NewServiceUnavailableError(service, endpoint string) *CLIError {
	return &CLIError{
		Code:       ErrCodeServiceUnavailable,
		Message:    fmt.Sprintf("Service '%s' is unavailable", service),
		Details:    fmt.Sprintf("Endpoint: %s", endpoint),
		Suggestion: fmt.Sprintf("Verify '%s' is running and accessible at %s. Check network connectivity and service health.", service, endpoint),
		ExitCode:   3, // NFR-019: 3 = service unavailable
	}
}

// NewAuthenticationError creates an error for authentication failures.
func NewAuthenticationError(details string) *CLIError {
	return &CLIError{
		Code:       ErrCodeAuthenticationFailed,
		Message:    "Authentication failed",
		Details:    details,
		Suggestion: "Verify your API key or service account token is valid and not expired. Check token expiration with clock skew tolerance (±5 minutes).",
		ExitCode:   1, // NFR-019: 1 = general error
	}
}

// NewValidationError creates an error for validation failures.
func NewValidationError(message, suggestion string) *CLIError {
	return &CLIError{
		Code:       ErrCodeValidationFailed,
		Message:    "Validation failed",
		Details:    message,
		Suggestion: suggestion,
		ExitCode:   2, // NFR-019: 2 = usage error
	}
}

// NewOperationError creates an error for operation failures.
func NewOperationError(message, suggestion string) *CLIError {
	return &CLIError{
		Code:       ErrCodeOperationFailed,
		Message:    "Operation failed",
		Details:    message,
		Suggestion: suggestion,
		ExitCode:   1, // NFR-019: 1 = general error
	}
}

// NewUsageError creates an error for incorrect usage.
func NewUsageError(message string) *CLIError {
	return &CLIError{
		Code:       ErrCodeUsage,
		Message:    "Incorrect usage",
		Details:    message,
		Suggestion: "Run with --help for usage information.",
		ExitCode:   2, // NFR-019: 2 = usage error
	}
}

