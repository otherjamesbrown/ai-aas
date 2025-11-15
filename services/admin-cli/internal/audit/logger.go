// Package audit provides audit logging for privileged operations.
//
// Purpose:
//
//	Emit structured audit logs for all privileged operations (bootstrap, credential rotation,
//	destructive operations) with user identity, timestamp, command, parameters (masked where
//	sensitive), and outcomes. Logs are suitable for ingestion by log aggregation systems.
//
// Dependencies:
//   - encoding/json: Structured JSON log output
//   - time: Timestamp formatting
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-010 (audit logging for privileged operations)
//   - specs/009-admin-cli/spec.md#NFR-010 (audit logs capture all privileged operations)
//   - specs/009-admin-cli/spec.md#NFR-028 (audit logs include operation duration)
//
package audit

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// Logger emits audit logs for privileged operations.
type Logger struct {
	output   *json.Encoder
	maskFunc func(string) string
}

// NewLogger creates a new audit logger.
func NewLogger(w *os.File) *Logger {
	if w == nil {
		w = os.Stderr // Default to stderr for structured logs
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return &Logger{
		output: encoder,
		maskFunc: func(s string) string {
			// Mask credentials: show only last 4 characters or ***
			if len(s) <= 4 {
				return "***"
			}
			return "***" + s[len(s)-4:]
		},
	}
}

// LogEntry represents an audit log entry.
type LogEntry struct {
	Timestamp    string                 `json:"timestamp"`
	Operation    string                 `json:"operation"`
	UserIdentity string                 `json:"user_identity,omitempty"`
	Command      string                 `json:"command"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Outcome      string                 `json:"outcome"` // success, failure
	Duration     string                 `json:"duration,omitempty"`
	BreakGlass   bool                   `json:"break_glass,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

// LogOperation logs a privileged operation with all required fields.
func (l *Logger) LogOperation(op Operation) error {
	entry := LogEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Operation:    op.Type,
		UserIdentity: op.UserIdentity,
		Command:      op.Command,
		Parameters:   l.maskParameters(op.Parameters),
		Outcome:      op.Outcome,
		BreakGlass:   op.BreakGlass,
	}

	if op.Duration > 0 {
		entry.Duration = op.Duration.String()
	}

	if op.Error != nil {
		entry.Error = op.Error.Error()
	}

	return l.output.Encode(entry)
}

// Operation represents a privileged operation to be logged.
type Operation struct {
	Type        string                 // bootstrap, credential_rotation, org_delete, etc.
	UserIdentity string                // User ID or token identity
	Command     string                 // Full command executed
	Parameters  map[string]interface{} // Command parameters (will be masked)
	Outcome     string                 // success, failure
	Duration    time.Duration          // Operation duration
	BreakGlass  bool                   // True for break-glass operations
	Error       error                  // Error if operation failed
}

// maskParameters masks sensitive values in parameters.
func (l *Logger) maskParameters(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}

	masked := make(map[string]interface{})
	sensitiveKeys := []string{"api_key", "token", "password", "secret", "credential"}

	for k, v := range params {
		// Check if key is sensitive
		isSensitive := false
		lowerKey := strings.ToLower(k)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitive) {
				isSensitive = true
				break
			}
		}

		if isSensitive && v != nil {
			if str, ok := v.(string); ok {
				masked[k] = l.maskFunc(str)
			} else {
				masked[k] = "***"
			}
		} else {
			masked[k] = v
		}
	}

	return masked
}

