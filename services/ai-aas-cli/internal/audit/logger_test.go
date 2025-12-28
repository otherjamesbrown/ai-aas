// Package audit provides tests for audit logging.
package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLogger verifies logger initialization
func TestNewLogger(t *testing.T) {
	logger := NewLogger(nil)
	require.NotNil(t, logger, "NewLogger should not return nil")
	assert.NotNil(t, logger.output, "logger should have output encoder")
	assert.NotNil(t, logger.maskFunc, "logger should have mask function")
}

// TestLogOperation_SuccessInfo tests logging successful non-destructive operations
func TestLogOperation_SuccessInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	file := &os.File{}
	logger := NewLogger(file)
	logger.output = json.NewEncoder(buf)

	err := logger.LogOperation(Operation{
		Type:         "user_list",
		UserIdentity: "user-123",
		Command:      "user list --org-id=acme",
		Parameters: map[string]interface{}{
			"orgId": "acme",
		},
		Outcome:  "success",
		Duration: 100 * time.Millisecond,
	})

	require.NoError(t, err)

	var entry LogEntry
	err = json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Equal(t, "info", entry.Level, "successful list operation should log at info level")
	assert.Equal(t, "user_list", entry.Operation)
	assert.Equal(t, "user-123", entry.UserIdentity)
	assert.Equal(t, "success", entry.Outcome)
	assert.NotEmpty(t, entry.Timestamp)
	assert.NotEmpty(t, entry.Duration)
}

// TestLogOperation_DestructiveWarn tests logging destructive operations
func TestLogOperation_DestructiveWarn(t *testing.T) {
	buf := &bytes.Buffer{}
	file := &os.File{}
	logger := NewLogger(file)
	logger.output = json.NewEncoder(buf)

	err := logger.LogOperation(Operation{
		Type:         "user_delete",
		UserIdentity: "admin-456",
		Command:      "user delete --org-id=acme --user-id=user-123 --confirm",
		Parameters: map[string]interface{}{
			"orgId":  "acme",
			"userId": "user-123",
		},
		Outcome:  "success",
		Duration: 200 * time.Millisecond,
	})

	require.NoError(t, err)

	var entry LogEntry
	err = json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Equal(t, "warn", entry.Level, "successful delete operation should log at warn level")
	assert.Equal(t, "user_delete", entry.Operation)
	assert.Equal(t, "success", entry.Outcome)
}

// TestLogOperation_FailureError tests logging failed operations
func TestLogOperation_FailureError(t *testing.T) {
	buf := &bytes.Buffer{}
	file := &os.File{}
	logger := NewLogger(file)
	logger.output = json.NewEncoder(buf)

	testErr := assert.AnError

	err := logger.LogOperation(Operation{
		Type:         "user_create",
		UserIdentity: "admin-789",
		Command:      "user create --org-id=acme --email=test@example.com",
		Parameters: map[string]interface{}{
			"orgId": "acme",
			"email": "test@example.com",
		},
		Outcome:  "failure",
		Duration: 50 * time.Millisecond,
		Error:    testErr,
	})

	require.NoError(t, err)

	var entry LogEntry
	err = json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.Equal(t, "error", entry.Level, "failed operation should log at error level")
	assert.Equal(t, "user_create", entry.Operation)
	assert.Equal(t, "failure", entry.Outcome)
	assert.Contains(t, entry.Error, "assert.AnError", "error message should be present")
}

// TestMaskParameters tests sensitive data masking
func TestMaskParameters(t *testing.T) {
	buf := &bytes.Buffer{}
	file := &os.File{}
	logger := NewLogger(file)
	logger.output = json.NewEncoder(buf)

	err := logger.LogOperation(Operation{
		Type:    "credential_rotation",
		Command: "credentials rotate",
		Parameters: map[string]interface{}{
			"api_key":  "secret-key-12345678",
			"password": "my-password",
			"username": "admin",
			"token":    "bearer-token-abcd",
		},
		Outcome: "success",
	})

	require.NoError(t, err)

	var entry LogEntry
	err = json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	// Sensitive fields should be masked
	assert.Equal(t, "***5678", entry.Parameters["api_key"], "api_key should be masked")
	assert.Equal(t, "***word", entry.Parameters["password"], "password should be masked")
	assert.Equal(t, "***abcd", entry.Parameters["token"], "token should be masked")

	// Non-sensitive fields should not be masked
	assert.Equal(t, "admin", entry.Parameters["username"], "username should not be masked")
}

// TestDetermineLogLevel tests log level determination
func TestDetermineLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		expectedLevel string
	}{
		{
			name: "successful list operation - info",
			operation: Operation{
				Type:    "user_list",
				Outcome: "success",
			},
			expectedLevel: "info",
		},
		{
			name: "successful delete operation - warn",
			operation: Operation{
				Type:    "user_delete",
				Outcome: "success",
			},
			expectedLevel: "warn",
		},
		{
			name: "failed operation - error",
			operation: Operation{
				Type:    "user_create",
				Outcome: "failure",
			},
			expectedLevel: "error",
		},
		{
			name: "operation with error - error",
			operation: Operation{
				Type:    "user_update",
				Outcome: "success",
				Error:   assert.AnError,
			},
			expectedLevel: "error",
		},
		{
			name: "bootstrap operation - warn",
			operation: Operation{
				Type:    "bootstrap",
				Outcome: "success",
			},
			expectedLevel: "warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := determineLogLevel(tt.operation)
			assert.Equal(t, tt.expectedLevel, level)
		})
	}
}

// TestBreakGlassFlag tests break-glass flag is preserved
func TestBreakGlassFlag(t *testing.T) {
	buf := &bytes.Buffer{}
	file := &os.File{}
	logger := NewLogger(file)
	logger.output = json.NewEncoder(buf)

	err := logger.LogOperation(Operation{
		Type:       "bootstrap",
		Command:    "bootstrap --break-glass",
		Outcome:    "success",
		BreakGlass: true,
	})

	require.NoError(t, err)

	var entry LogEntry
	err = json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)

	assert.True(t, entry.BreakGlass, "break-glass flag should be true")
}
