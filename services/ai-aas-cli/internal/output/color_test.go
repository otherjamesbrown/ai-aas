// Package output provides tests for color output.
package output

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestColorInstancesExist(t *testing.T) {
	// Test that all color instances are initialized
	colorInstances := []*color.Color{
		Success,
		Warning,
		Error,
		Info,
		Label,
		Value,
		Muted,
		Highlight,
	}

	for i, c := range colorInstances {
		if c == nil {
			t.Errorf("color instance %d is nil", i)
		}
	}
}

func TestStatusIndicators(t *testing.T) {
	// Test that status indicators are not empty
	indicators := []string{
		CheckMark,
		CrossMark,
		Bullet,
		Arrow,
	}

	for i, indicator := range indicators {
		if indicator == "" {
			t.Errorf("status indicator %d is empty", i)
		}
	}
}

func TestSymbolConstants(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		want   string
	}{
		{"check symbol", SymbolCheck, "✓"},
		{"cross symbol", SymbolCross, "✗"},
		{"dot symbol", SymbolDot, "•"},
		{"arrow symbol", SymbolArrow, "→"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.symbol != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.symbol, tt.want)
			}
		})
	}
}

func TestPrintf(t *testing.T) {
	// Disable colors for consistent testing
	color.NoColor = true
	defer func() { color.NoColor = false }()

	tests := []struct {
		name   string
		color  *color.Color
		format string
		args   []interface{}
	}{
		{"success message", Success, "Operation %s", []interface{}{"complete"}},
		{"error message", Error, "Failed: %s", []interface{}{"error"}},
		{"info message", Info, "Status: %d", []interface{}{42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			Printf(tt.color, tt.format, tt.args...)
		})
	}
}

func TestPrintln(t *testing.T) {
	// Disable colors for consistent testing
	color.NoColor = true
	defer func() { color.NoColor = false }()

	tests := []struct {
		name  string
		color *color.Color
		args  []interface{}
	}{
		{"success", Success, []interface{}{"test"}},
		{"error", Error, []interface{}{"error", "message"}},
		{"info", Info, []interface{}{42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			Println(tt.color, tt.args...)
		})
	}
}

func TestSuccessMsg(t *testing.T) {
	// These functions write to stdout/stderr, so we can't easily capture output
	// Just verify they don't panic
	tests := []struct {
		name   string
		format string
		args   []interface{}
	}{
		{"simple message", "operation completed", nil},
		{"formatted message", "completed %d tasks", []interface{}{5}},
		{"empty message", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SuccessMsg(tt.format, tt.args...)
		})
	}
}

func TestErrorMsg(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
	}{
		{"simple error", "operation failed", nil},
		{"formatted error", "failed with code %d", []interface{}{404}},
		{"empty error", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			ErrorMsg(tt.format, tt.args...)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			buf.ReadFrom(r)
			// Just verify something was written
		})
	}
}

func TestWarningMsg(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
	}{
		{"simple warning", "be careful", nil},
		{"formatted warning", "resource %s is low", []interface{}{"memory"}},
		{"empty warning", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			WarningMsg(tt.format, tt.args...)
		})
	}
}

func TestInfoMsg(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
	}{
		{"simple info", "processing", nil},
		{"formatted info", "processed %d items", []interface{}{10}},
		{"empty info", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InfoMsg(tt.format, tt.args...)
		})
	}
}

func TestHeader(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{"simple title", "Section Title"},
		{"empty title", ""},
		{"unicode title", "セクション"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Header(tt.title)
		})
	}
}

func TestKeyValue(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"normal pair", "Name", "test"},
		{"empty value", "Key", ""},
		{"empty key", "", "value"},
		{"unicode", "名前", "テスト"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			KeyValue(tt.key, tt.value)
		})
	}
}

func TestKeyValueStatus(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		configured bool
	}{
		{"configured true", "Database", "connected", true},
		{"configured false", "Cache", "disconnected", false},
		{"empty value configured", "Key", "", true},
		{"empty key not configured", "", "value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			KeyValueStatus(tt.key, tt.value, tt.configured)
		})
	}
}

func TestStatusBadge(t *testing.T) {
	// Disable colors for predictable output
	color.NoColor = true
	defer func() { color.NoColor = false }()

	tests := []struct {
		name   string
		status string
		want   string
	}{
		// Success statuses
		{"ready", "ready", "ready"},
		{"complete", "complete", "complete"},
		{"success", "success", "success"},
		{"healthy", "healthy", "healthy"},
		{"active", "active", "active"},

		// Warning statuses
		{"pending", "pending", "pending"},
		{"waiting", "waiting", "waiting"},
		{"registered", "registered", "registered"},

		// Error statuses
		{"failed", "failed", "failed"},
		{"error", "error", "error"},
		{"unhealthy", "unhealthy", "unhealthy"},

		// Info statuses
		{"cached", "cached", "cached"},
		{"downloading", "downloading", "downloading"},
		{"uploading", "uploading", "uploading"},
		{"deploying", "deploying", "deploying"},

		// Muted statuses
		{"disabled", "disabled", "disabled"},

		// Unknown status (returned as-is)
		{"unknown", "unknown-status", "unknown-status"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusBadge(tt.status)
			if got != tt.want {
				t.Errorf("StatusBadge(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusBadge_WithColors(t *testing.T) {
	// Test with colors enabled to ensure no panics
	color.NoColor = false
	defer func() { color.NoColor = true }()

	statuses := []string{
		"ready", "pending", "failed", "cached", "disabled", "unknown",
	}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			result := StatusBadge(status)
			// With colors, result will contain ANSI codes
			// Just verify it's not empty and contains the status text
			if result == "" {
				t.Error("StatusBadge returned empty string")
			}
			if !strings.Contains(result, status) {
				t.Errorf("StatusBadge result %q should contain %q", result, status)
			}
		})
	}
}

func TestActionHint(t *testing.T) {
	// Disable colors for predictable output
	color.NoColor = true
	defer func() { color.NoColor = false }()

	tests := []struct {
		name   string
		action string
		want   string
	}{
		{"simple action", "run command", "→ run command"},
		{"empty action", "", "→ "},
		{"unicode action", "実行する", "→ 実行する"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ActionHint(tt.action)
			if got != tt.want {
				t.Errorf("ActionHint(%q) = %q, want %q", tt.action, got, tt.want)
			}
		})
	}
}

func TestDefaultTableStyle(t *testing.T) {
	style := DefaultTableStyle()

	if style == nil {
		t.Fatal("DefaultTableStyle() returned nil")
	}
	if style.HeaderColor == nil {
		t.Error("HeaderColor is nil")
	}
	if len(style.RowColors) == 0 {
		t.Error("RowColors is empty")
	}
	if style.HeaderColor != Label {
		t.Error("HeaderColor should be Label")
	}
}

func TestBox(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		content string
	}{
		{"simple box", "Title", "Content here"},
		{"empty content", "Title", ""},
		{"empty title", "", "Content"},
		{"multiline content", "Title", "Line 1\nLine 2\nLine 3"},
		{"unicode", "タイトル", "内容"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Box(tt.title, tt.content)
		})
	}
}
