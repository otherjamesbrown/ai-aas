// Package output provides tests for table output formatting.
package output

import (
	"bytes"
	"testing"
)

func TestTableFormatter(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTableFormatter(&buf)

	headers := []string{"ID", "Name", "Status"}
	if err := formatter.WriteHeader(headers...); err != nil {
		t.Fatalf("WriteHeader() failed: %v", err)
	}

	row1 := []string{"1", "test", "active"}
	if err := formatter.WriteRow(row1...); err != nil {
		t.Fatalf("WriteRow() failed: %v", err)
	}

	if err := formatter.Flush(); err != nil {
		t.Fatalf("Flush() failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output")
	}
}

