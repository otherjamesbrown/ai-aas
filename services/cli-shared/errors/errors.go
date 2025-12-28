// Package errors provides structured error types and recovery suggestions
// for CLI applications.
//
// This package is designed to be shared across multiple CLI tools including
// ai-aas-cli and ai-aas-org to ensure consistent error handling and user
// guidance across the platform.
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
	// ErrCodeAuthorizationFailed indicates the user lacks permission.
	ErrCodeAuthorizationFailed ErrorCode = "AUTHORIZATION_FAILED"
	// ErrCodeValidationFailed indicates input validation failure.
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	// ErrCodeOperationFailed indicates a general operation failure.
	ErrCodeOperationFailed ErrorCode = "OPERATION_FAILED"
	// ErrCodeUsage indicates incorrect command usage.
	ErrCodeUsage ErrorCode = "USAGE_ERROR"
	// ErrCodeNotFound indicates a resource was not found.
	ErrCodeNotFound ErrorCode = "NOT_FOUND"
	// ErrCodeConflict indicates a resource conflict (e.g., already exists).
	ErrCodeConflict ErrorCode = "CONFLICT"
	// ErrCodeRateLimited indicates rate limiting is in effect.
	ErrCodeRateLimited ErrorCode = "RATE_LIMITED"
)

// Exit codes for consistent scripting behavior.
const (
	ExitOK               = 0 // Success
	ExitError            = 1 // General error
	ExitUsageError       = 2 // Usage/validation error
	ExitServiceError     = 3 // Service unavailable
	ExitAuthError        = 4 // Authentication/authorization error
	ExitNotFound         = 5 // Resource not found
	ExitConflict         = 6 // Resource conflict
)

// CLIError represents a structured CLI error with recovery suggestions.
type CLIError struct {
	Code       ErrorCode
	Message    string
	Suggestion string
	Details    string
	ExitCode   int
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

// Unwrap returns nil as CLIError doesn't wrap other errors.
func (e *CLIError) Unwrap() error {
	return nil
}

// WithDetails returns a copy of the error with additional details.
func (e *CLIError) WithDetails(details string) *CLIError {
	return &CLIError{
		Code:       e.Code,
		Message:    e.Message,
		Suggestion: e.Suggestion,
		Details:    details,
		ExitCode:   e.ExitCode,
	}
}

// WithSuggestion returns a copy of the error with a custom suggestion.
func (e *CLIError) WithSuggestion(suggestion string) *CLIError {
	return &CLIError{
		Code:       e.Code,
		Message:    e.Message,
		Suggestion: suggestion,
		Details:    e.Details,
		ExitCode:   e.ExitCode,
	}
}

// NewServiceUnavailableError creates an error for service unavailability.
func NewServiceUnavailableError(service, endpoint string) *CLIError {
	return &CLIError{
		Code:       ErrCodeServiceUnavailable,
		Message:    fmt.Sprintf("Service '%s' is unavailable", service),
		Details:    fmt.Sprintf("Endpoint: %s", endpoint),
		Suggestion: fmt.Sprintf("Verify '%s' is running and accessible at %s. Check network connectivity and service health.", service, endpoint),
		ExitCode:   ExitServiceError,
	}
}

// NewAuthenticationError creates an error for authentication failures.
func NewAuthenticationError(details string) *CLIError {
	return &CLIError{
		Code:       ErrCodeAuthenticationFailed,
		Message:    "Authentication failed",
		Details:    details,
		Suggestion: "Verify your API key is valid and not expired. Run 'configure' to update credentials.",
		ExitCode:   ExitAuthError,
	}
}

// NewAuthorizationError creates an error for permission failures.
func NewAuthorizationError(action, resource string) *CLIError {
	return &CLIError{
		Code:       ErrCodeAuthorizationFailed,
		Message:    "Permission denied",
		Details:    fmt.Sprintf("You don't have permission to %s %s", action, resource),
		Suggestion: "Contact your organization administrator to request access.",
		ExitCode:   ExitAuthError,
	}
}

// NewValidationError creates an error for validation failures.
func NewValidationError(message, suggestion string) *CLIError {
	return &CLIError{
		Code:       ErrCodeValidationFailed,
		Message:    "Validation failed",
		Details:    message,
		Suggestion: suggestion,
		ExitCode:   ExitUsageError,
	}
}

// NewOperationError creates an error for operation failures.
func NewOperationError(message, suggestion string) *CLIError {
	return &CLIError{
		Code:       ErrCodeOperationFailed,
		Message:    "Operation failed",
		Details:    message,
		Suggestion: suggestion,
		ExitCode:   ExitError,
	}
}

// NewUsageError creates an error for incorrect usage.
func NewUsageError(message string) *CLIError {
	return &CLIError{
		Code:       ErrCodeUsage,
		Message:    "Incorrect usage",
		Details:    message,
		Suggestion: "Run with --help for usage information.",
		ExitCode:   ExitUsageError,
	}
}

// NewNotFoundError creates an error for missing resources.
func NewNotFoundError(resourceType, identifier string) *CLIError {
	return &CLIError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found", resourceType),
		Details:    fmt.Sprintf("Could not find %s '%s'", resourceType, identifier),
		Suggestion: fmt.Sprintf("Verify the %s exists and you have access to it.", resourceType),
		ExitCode:   ExitNotFound,
	}
}

// NewConflictError creates an error for resource conflicts.
func NewConflictError(resourceType, identifier string) *CLIError {
	return &CLIError{
		Code:       ErrCodeConflict,
		Message:    fmt.Sprintf("%s already exists", resourceType),
		Details:    fmt.Sprintf("A %s with identifier '%s' already exists", resourceType, identifier),
		Suggestion: "Use a different identifier or update the existing resource.",
		ExitCode:   ExitConflict,
	}
}

// NewRateLimitError creates an error for rate limiting.
func NewRateLimitError(retryAfter string) *CLIError {
	suggestion := "Please wait before retrying."
	if retryAfter != "" {
		suggestion = fmt.Sprintf("Please wait %s before retrying.", retryAfter)
	}
	return &CLIError{
		Code:       ErrCodeRateLimited,
		Message:    "Rate limit exceeded",
		Suggestion: suggestion,
		ExitCode:   ExitError,
	}
}

// IsCLIError checks if an error is a CLIError.
func IsCLIError(err error) bool {
	_, ok := err.(*CLIError)
	return ok
}

// GetExitCode returns the appropriate exit code for an error.
func GetExitCode(err error) int {
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.ExitCode
	}
	return ExitError
}
