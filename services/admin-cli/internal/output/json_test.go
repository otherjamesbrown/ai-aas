// Package output provides tests for JSON output formatting.
package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)

	data := map[string]interface{}{
		"test": "value",
	}

	if err := formatter.WriteSuccess("test-command", data, nil); err != nil {
		t.Fatalf("WriteSuccess() failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	if output["success"] != true {
		t.Error("expected success=true")
	}
}

func TestJSONFormatterError(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)

	err := formatter.WriteError("test-command", &testError{msg: "test error"}, "retry")

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	if output["success"] != false {
		t.Error("expected success=false")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

