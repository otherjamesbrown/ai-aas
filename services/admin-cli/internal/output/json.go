// Package output provides JSON output formatting for the Admin CLI.
//
// Purpose:
//
//	Format command output as JSON for machine-readable output suitable for parsing
//	by CI/CD systems or monitoring tools. Provides consistent JSON schema across
//	all commands.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-006 (structured output for scripting)
//   - specs/009-admin-cli/spec.md#NFR-018 (structured JSON output with stable schema)
//
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// JSONFormatter formats output as JSON.
type JSONFormatter struct {
	writer io.Writer
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{writer: w}
}

// Output represents structured CLI output with consistent schema.
type Output struct {
	Success   bool                   `json:"success"`
	Timestamp string                 `json:"timestamp"`
	Command   string                 `json:"command,omitempty"`
	Data      interface{}            `json:"data,omitempty"`
	Error     *ErrorOutput           `json:"error,omitempty"`
	Summary   map[string]interface{} `json:"summary,omitempty"`
}

// ErrorOutput represents error information in JSON output.
type ErrorOutput struct {
	Message   string `json:"message"`
	Code      string `json:"code,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Write outputs data as JSON.
func (j *JSONFormatter) Write(output Output) error {
	if output.Timestamp == "" {
		output.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	encoder := json.NewEncoder(j.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// WriteError outputs an error as JSON.
func (j *JSONFormatter) WriteError(cmd string, err error, suggestion string) error {
	return j.Write(Output{
		Success:   false,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   cmd,
		Error: &ErrorOutput{
			Message:    err.Error(),
			Suggestion: suggestion,
		},
	})
}

// WriteSuccess outputs successful operation result as JSON.
func (j *JSONFormatter) WriteSuccess(cmd string, data interface{}, summary map[string]interface{}) error {
	return j.Write(Output{
		Success:   true,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   cmd,
		Data:      data,
		Summary:   summary,
	})
}

// PrintJSON is a convenience function to print JSON output to stdout.
func PrintJSON(data interface{}) error {
	formatter := NewJSONFormatter(os.Stdout)
	return formatter.Write(Output{
		Success:   true,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	})
}

