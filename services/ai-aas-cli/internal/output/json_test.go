// Package output provides tests for JSON formatting.
package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewJSONWriter(t *testing.T) {
	tests := []struct {
		name   string
		pretty bool
	}{
		{"pretty format", true},
		{"compact format", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := NewJSONWriter(tt.pretty)
			if writer == nil {
				t.Fatal("NewJSONWriter() returned nil")
			}
			if writer.pretty != tt.pretty {
				t.Errorf("expected pretty=%v, got %v", tt.pretty, writer.pretty)
			}
		})
	}
}

func TestNewJSONWriterTo(t *testing.T) {
	var buf bytes.Buffer
	writer := NewJSONWriterTo(&buf, true)

	if writer == nil {
		t.Fatal("NewJSONWriterTo() returned nil")
	}
	if writer.writer != &buf {
		t.Error("writer not set correctly")
	}
	if !writer.pretty {
		t.Error("expected pretty=true")
	}
}

func TestJSONWriter_Write(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		pretty   bool
		wantJSON string
	}{
		{
			name:     "simple string",
			value:    "test",
			pretty:   false,
			wantJSON: `"test"`,
		},
		{
			name:     "number",
			value:    42,
			pretty:   false,
			wantJSON: `42`,
		},
		{
			name:     "boolean",
			value:    true,
			pretty:   false,
			wantJSON: `true`,
		},
		{
			name:     "nil value",
			value:    nil,
			pretty:   false,
			wantJSON: `null`,
		},
		{
			name:     "empty string",
			value:    "",
			pretty:   false,
			wantJSON: `""`,
		},
		{
			name:     "unicode string",
			value:    "Hello 世界",
			pretty:   false,
			wantJSON: `"Hello 世界"`,
		},
		{
			name:     "string with newlines",
			value:    "line1\nline2",
			pretty:   false,
			wantJSON: `"line1\nline2"`,
		},
		{
			name:     "map",
			value:    map[string]string{"key": "value"},
			pretty:   false,
			wantJSON: `{"key":"value"}`,
		},
		{
			name:     "empty map",
			value:    map[string]string{},
			pretty:   false,
			wantJSON: `{}`,
		},
		{
			name:     "slice",
			value:    []int{1, 2, 3},
			pretty:   false,
			wantJSON: `[1,2,3]`,
		},
		{
			name:     "empty slice",
			value:    []int{},
			pretty:   false,
			wantJSON: `[]`,
		},
		{
			name:     "nested structure",
			value:    map[string]interface{}{"outer": map[string]int{"inner": 123}},
			pretty:   false,
			wantJSON: `{"outer":{"inner":123}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONWriterTo(&buf, tt.pretty)

			err := writer.Write(tt.value)
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			got := strings.TrimSpace(buf.String())
			want := tt.wantJSON

			if got != want {
				t.Errorf("Write() = %q, want %q", got, want)
			}
		})
	}
}

func TestJSONWriter_WriteString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON string
	}{
		{"normal string", "hello", `"hello"`},
		{"empty string", "", `""`},
		{"special characters", "a\"b\\c", `"a\"b\\c"`},
		{"unicode", "こんにちは", `"こんにちは"`},
		{"emoji", "👍🎉", `"👍🎉"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONWriterTo(&buf, false)

			err := writer.WriteString(tt.input)
			if err != nil {
				t.Fatalf("WriteString() error = %v", err)
			}

			got := strings.TrimSpace(buf.String())
			if got != tt.wantJSON {
				t.Errorf("WriteString() = %q, want %q", got, tt.wantJSON)
			}
		})
	}
}

func TestJSONWriter_WriteError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantJSON string
	}{
		{
			name:     "simple error",
			err:      errors.New("test error"),
			wantJSON: `{"error":"test error"}`,
		},
		{
			name:     "empty error message",
			err:      errors.New(""),
			wantJSON: `{"error":""}`,
		},
		{
			name:     "error with special characters",
			err:      errors.New("error: \"value\" is invalid"),
			wantJSON: `{"error":"error: \"value\" is invalid"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONWriterTo(&buf, false)

			err := writer.WriteError(tt.err)
			if err != nil {
				t.Fatalf("WriteError() error = %v", err)
			}

			got := strings.TrimSpace(buf.String())
			if got != tt.wantJSON {
				t.Errorf("WriteError() = %q, want %q", got, tt.wantJSON)
			}
		})
	}
}

func TestJSONWriter_WriteResult(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		message  string
		data     interface{}
		wantJSON string
	}{
		{
			name:     "success with data",
			success:  true,
			message:  "operation completed",
			data:     map[string]int{"count": 5},
			wantJSON: `{"data":{"count":5},"message":"operation completed","success":true}`,
		},
		{
			name:     "success without data",
			success:  true,
			message:  "done",
			data:     nil,
			wantJSON: `{"message":"done","success":true}`,
		},
		{
			name:     "failure with message",
			success:  false,
			message:  "operation failed",
			data:     nil,
			wantJSON: `{"message":"operation failed","success":false}`,
		},
		{
			name:     "empty message",
			success:  true,
			message:  "",
			data:     nil,
			wantJSON: `{"message":"","success":true}`,
		},
		{
			name:     "data is empty map",
			success:  true,
			message:  "empty",
			data:     map[string]string{},
			wantJSON: `{"data":{},"message":"empty","success":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONWriterTo(&buf, false)

			err := writer.WriteResult(tt.success, tt.message, tt.data)
			if err != nil {
				t.Fatalf("WriteResult() error = %v", err)
			}

			got := strings.TrimSpace(buf.String())

			// Parse both to compare structurally (field order may vary)
			var gotMap, wantMap map[string]interface{}
			if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
				t.Fatalf("failed to parse output JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantMap); err != nil {
				t.Fatalf("failed to parse expected JSON: %v", err)
			}

			if !jsonEqual(gotMap, wantMap) {
				t.Errorf("WriteResult() = %q, want %q", got, tt.wantJSON)
			}
		})
	}
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		pretty  bool
		wantErr bool
	}{
		{"simple value", "test", false, false},
		{"map", map[string]int{"a": 1}, false, false},
		{"pretty map", map[string]int{"a": 1}, true, false},
		{"nil", nil, false, false},
		{"channel (invalid)", make(chan int), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MarshalJSON(tt.value, tt.pretty)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("MarshalJSON() returned nil bytes")
			}

			// Verify pretty formatting adds indentation
			if tt.pretty && !tt.wantErr && len(got) > 0 {
				str := string(got)
				if !strings.Contains(str, "\n") && !strings.Contains(str, "  ") {
					// Single values may not have indentation, only complex structures
					if _, ok := tt.value.(map[string]int); ok {
						t.Error("Pretty format should contain newlines or indentation for maps")
					}
				}
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	// Note: PrintJSON writes to stdout, so we can't easily capture it
	// We'll test it doesn't panic and returns no error for valid input
	tests := []struct {
		name      string
		value     interface{}
		prettyOpt []bool
		wantErr   bool
	}{
		{"simple value default pretty", "test", nil, false},
		{"explicit pretty", "test", []bool{true}, false},
		{"explicit compact", "test", []bool{false}, false},
		{"map pretty", map[string]int{"a": 1}, []bool{true}, false},
		{"invalid value", make(chan int), []bool{true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PrintJSON(tt.value, tt.prettyOpt...)
			if (err != nil) != tt.wantErr {
				t.Errorf("PrintJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewSuccessResponse(t *testing.T) {
	tests := []struct {
		name    string
		message string
		data    interface{}
	}{
		{"with data", "success", map[string]int{"count": 5}},
		{"without data", "done", nil},
		{"empty message", "", "some data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewSuccessResponse(tt.message, tt.data)

			if !resp.Success {
				t.Error("expected Success=true")
			}
			if resp.Message != tt.message {
				t.Errorf("expected Message=%q, got %q", tt.message, resp.Message)
			}
			if tt.data != nil && resp.Data == nil {
				t.Error("expected Data to be set")
			}
			if resp.Error != "" {
				t.Errorf("expected empty Error, got %q", resp.Error)
			}
		})
	}
}

func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"simple error", errors.New("test error")},
		{"empty error", errors.New("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewErrorResponse(tt.err)

			if resp.Success {
				t.Error("expected Success=false")
			}
			if resp.Error != tt.err.Error() {
				t.Errorf("expected Error=%q, got %q", tt.err.Error(), resp.Error)
			}
			if resp.Message != "" {
				t.Errorf("expected empty Message, got %q", resp.Message)
			}
			if resp.Data != nil {
				t.Error("expected nil Data")
			}
		})
	}
}

// Helper to compare JSON maps structurally
func jsonEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !deepEqual(v, bv) {
			return false
		}
	}
	return true
}

func deepEqual(a, b interface{}) bool {
	// Simple deep equality for testing
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
