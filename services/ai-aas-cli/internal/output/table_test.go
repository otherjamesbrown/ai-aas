// Package output provides tests for table formatting.
package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewTableWriter(t *testing.T) {
	tw := NewTableWriter()
	if tw == nil {
		t.Fatal("NewTableWriter() returned nil")
	}
	if tw.table == nil {
		t.Error("table not initialized")
	}
}

func TestNewTableWriterTo(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTableWriterTo(&buf)

	if tw == nil {
		t.Fatal("NewTableWriterTo() returned nil")
	}
	if tw.writer != &buf {
		t.Error("writer not set correctly")
	}
}

func TestTableWriter_SetHeader(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTableWriterTo(&buf)

	headers := []string{"Name", "Status", "Age"}
	tw.SetHeader(headers)
	tw.Render()

	output := buf.String()
	// Headers are uppercased by tablewriter
	for _, h := range headers {
		upperH := strings.ToUpper(h)
		if !strings.Contains(output, upperH) {
			t.Errorf("header %q (uppercase %q) not found in output", h, upperH)
		}
	}
}

func TestTableWriter_Append(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTableWriterTo(&buf)

	tw.SetHeader([]string{"Col1", "Col2"})
	tw.Append([]string{"val1", "val2"})
	tw.Render()

	output := buf.String()
	if !strings.Contains(output, "val1") {
		t.Error("expected val1 in output")
	}
	if !strings.Contains(output, "val2") {
		t.Error("expected val2 in output")
	}
}

func TestTableWriter_AppendBulk(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
	}{
		{
			name:    "single row",
			headers: []string{"A", "B"},
			rows:    [][]string{{"a1", "b1"}},
		},
		{
			name:    "multiple rows",
			headers: []string{"Name", "Value"},
			rows: [][]string{
				{"row1", "val1"},
				{"row2", "val2"},
				{"row3", "val3"},
			},
		},
		{
			name:    "empty rows",
			headers: []string{"Col"},
			rows:    [][]string{},
		},
		{
			name:    "unicode content",
			headers: []string{"Name", "Value"},
			rows: [][]string{
				{"日本語", "テスト"},
				{"中文", "测试"},
			},
		},
		{
			name:    "special characters",
			headers: []string{"Name", "Value"},
			rows: [][]string{
				{"test\"quote", "value"},
				{"test\ttab", "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tw := NewTableWriterTo(&buf)

			tw.SetHeader(tt.headers)
			tw.AppendBulk(tt.rows)
			tw.Render()

			output := buf.String()

			// Check headers present (headers are uppercased by tablewriter)
			for _, h := range tt.headers {
				upperH := strings.ToUpper(h)
				if !strings.Contains(output, upperH) {
					t.Errorf("header %q (uppercase %q) not found in output", h, upperH)
				}
			}

			// Check row values present
			for _, row := range tt.rows {
				for _, cell := range row {
					if !strings.Contains(output, cell) {
						t.Errorf("cell value %q not found in output", cell)
					}
				}
			}
		})
	}
}

func TestPrintTable(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		wantErr bool
	}{
		{
			name:    "valid table",
			headers: []string{"Col1", "Col2"},
			rows:    [][]string{{"a", "b"}, {"c", "d"}},
			wantErr: false,
		},
		{
			name:    "empty table",
			headers: []string{},
			rows:    [][]string{},
			wantErr: false,
		},
		{
			name:    "nil rows",
			headers: []string{"Col"},
			rows:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PrintTable(tt.headers, tt.rows)
			if (err != nil) != tt.wantErr {
				t.Errorf("PrintTable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTableWriter_SetAutoWrap(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTableWriterTo(&buf)

	// Should not panic
	tw.SetAutoWrap(true)
	tw.SetAutoWrap(false)
}

func TestTableWriter_SetColWidth(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTableWriterTo(&buf)

	// Should not panic
	tw.SetColWidth(50)
	tw.SetColWidth(100)
}

func TestFormatModelList(t *testing.T) {
	tests := []struct {
		name   string
		models []ModelSummary
		want   [][]string
	}{
		{
			name:   "empty list",
			models: []ModelSummary{},
			want:   [][]string{},
		},
		{
			name: "single model",
			models: []ModelSummary{
				{
					Name:      "llama-7b",
					HFModelID: "meta-llama/Llama-2-7b-hf",
					Cached:    true,
					Deployed:  false,
					Status:    "ready",
				},
			},
			want: [][]string{
				{"llama-7b", "meta-llama/Llama-2-7b-hf", "Yes", "No", "ready"},
			},
		},
		{
			name: "multiple models",
			models: []ModelSummary{
				{Name: "model1", HFModelID: "org/model1", Cached: true, Deployed: true, Status: "active"},
				{Name: "model2", HFModelID: "org/model2", Cached: false, Deployed: false, Status: "pending"},
			},
			want: [][]string{
				{"model1", "org/model1", "Yes", "Yes", "active"},
				{"model2", "org/model2", "No", "No", "pending"},
			},
		},
		{
			name: "empty fields",
			models: []ModelSummary{
				{Name: "", HFModelID: "", Cached: false, Deployed: false, Status: ""},
			},
			want: [][]string{
				{"", "", "No", "No", ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatModelList(tt.models)

			if len(got) != len(tt.want) {
				t.Fatalf("FormatModelList() length = %d, want %d", len(got), len(tt.want))
			}

			for i, row := range got {
				if len(row) != len(tt.want[i]) {
					t.Errorf("row %d length = %d, want %d", i, len(row), len(tt.want[i]))
					continue
				}
				for j, cell := range row {
					if cell != tt.want[i][j] {
						t.Errorf("row %d, col %d = %q, want %q", i, j, cell, tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestFormatCacheList(t *testing.T) {
	tests := []struct {
		name    string
		entries []CacheSummary
		want    [][]string
	}{
		{
			name:    "empty list",
			entries: []CacheSummary{},
			want:    [][]string{},
		},
		{
			name: "single entry",
			entries: []CacheSummary{
				{
					ModelName: "model1",
					Version:   "v1",
					SizeBytes: 1024,
					CachedAt:  "2023-01-01",
				},
			},
			want: [][]string{
				{"model1", "v1", "1 KB", "2023-01-01"},
			},
		},
		{
			name: "multiple entries with different sizes",
			entries: []CacheSummary{
				{ModelName: "small", Version: "v1", SizeBytes: 512, CachedAt: "2023-01-01"},
				{ModelName: "medium", Version: "v2", SizeBytes: 1024 * 1024, CachedAt: "2023-01-02"},
				{ModelName: "large", Version: "v3", SizeBytes: 1024 * 1024 * 1024, CachedAt: "2023-01-03"},
			},
			want: [][]string{
				{"small", "v1", "512 B", "2023-01-01"},
				{"medium", "v2", "1 MB", "2023-01-02"},
				{"large", "v3", "1 GB", "2023-01-03"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCacheList(tt.entries)

			if len(got) != len(tt.want) {
				t.Fatalf("FormatCacheList() length = %d, want %d", len(got), len(tt.want))
			}

			for i, row := range got {
				if len(row) != len(tt.want[i]) {
					t.Errorf("row %d length = %d, want %d", i, len(row), len(tt.want[i]))
					continue
				}
				for j, cell := range row {
					if cell != tt.want[i][j] {
						t.Errorf("row %d, col %d = %q, want %q", i, j, cell, tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"single byte", 1, "1 B"},
		{"bytes only", 512, "512 B"},
		{"exact KB", 1024, "1 KB"},
		{"KB with decimal", 1536, "1.5 KB"},
		{"exact MB", 1024 * 1024, "1 MB"},
		{"MB with decimal", 1024 * 1024 * 1.5, "1.5 MB"},
		{"exact GB", 1024 * 1024 * 1024, "1 GB"},
		{"GB with decimal", int64(1024 * 1024 * 1024 * 2.5), "2.5 GB"},
		{"exact TB", 1024 * 1024 * 1024 * 1024, "1 TB"},
		{"exact PB", 1024 * 1024 * 1024 * 1024 * 1024, "1 PB"},
		{"exact EB", 1024 * 1024 * 1024 * 1024 * 1024 * 1024, "1 EB"},
		{"large value", 9223372036854775807, "8 EB"}, // max int64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatBool(t *testing.T) {
	tests := []struct {
		name  string
		value bool
		want  string
	}{
		{"true", true, "Yes"},
		{"false", false, "No"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBool(tt.value)
			if got != tt.want {
				t.Errorf("formatBool(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatInt64(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  string
	}{
		{"zero", 0, "0"},
		{"positive", 42, "42"},
		{"negative", -42, "-42"},
		{"large", 123456789, "123456789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatInt64(tt.value)
			if got != tt.want {
				t.Errorf("formatInt64(%d) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatFloat64(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{"zero", 0.0, "0"},
		{"integer value", 42.0, "42"},
		{"decimal value", 42.5, "42.5"},
		{"small decimal", 0.1, "0.1"},
		{"negative", -3.7, "-3.7"},
		{"large", 123456789.5, "123456789.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat64(tt.value)
			if got != tt.want {
				t.Errorf("formatFloat64(%f) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
