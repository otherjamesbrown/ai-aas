// Package output provides output formatting utilities for the CLI.
package output

import (
	"fmt"
	"io"
	"os"

	"github.com/olekukonko/tablewriter"
)

// TableWriter wraps tablewriter for consistent CLI output
type TableWriter struct {
	table  *tablewriter.Table
	writer io.Writer
}

// NewTableWriter creates a new table writer that outputs to stdout
func NewTableWriter() *TableWriter {
	return NewTableWriterTo(os.Stdout)
}

// NewTableWriterTo creates a new table writer with a custom output
func NewTableWriterTo(w io.Writer) *TableWriter {
	table := tablewriter.NewWriter(w)
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.SetAutoWrapText(false) // Disable wrapping to support ANSI colors

	return &TableWriter{
		table:  table,
		writer: w,
	}
}

// SetHeader sets the table headers
func (t *TableWriter) SetHeader(headers []string) {
	t.table.SetHeader(headers)
}

// Append adds a row to the table
func (t *TableWriter) Append(row []string) {
	t.table.Append(row)
}

// AppendBulk adds multiple rows to the table
func (t *TableWriter) AppendBulk(rows [][]string) {
	t.table.AppendBulk(rows)
}

// Render outputs the table
func (t *TableWriter) Render() {
	t.table.Render()
}

// PrintTable is a convenience function to print a table to stdout.
// This provides compatibility with admin commands.
func PrintTable(headers []string, rows [][]string) error {
	tw := NewTableWriter()
	tw.SetHeader(headers)
	tw.AppendBulk(rows)
	tw.Render()
	return nil
}

// SetAutoWrap enables or disables auto-wrapping
func (t *TableWriter) SetAutoWrap(wrap bool) {
	t.table.SetAutoWrapText(wrap)
}

// SetColWidth sets the max width for a specific column
func (t *TableWriter) SetColWidth(width int) {
	t.table.SetColWidth(width)
}

// FormatModelList formats a list of models for table output
func FormatModelList(models []ModelSummary) [][]string {
	rows := make([][]string, len(models))
	for i, m := range models {
		rows[i] = []string{
			m.Name,
			m.HFModelID,
			formatBool(m.Cached),
			formatBool(m.Deployed),
			m.Status,
		}
	}
	return rows
}

// ModelSummary contains model data for table display
type ModelSummary struct {
	Name      string
	HFModelID string
	Cached    bool
	Deployed  bool
	Status    string
}

// FormatCacheList formats cache entries for table output
func FormatCacheList(entries []CacheSummary) [][]string {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			e.ModelName,
			e.Version,
			FormatBytes(e.SizeBytes),
			e.CachedAt,
		}
	}
	return rows
}

// CacheSummary contains cache entry data for table display
type CacheSummary struct {
	ModelName string
	Version   string
	SizeBytes int64
	CachedAt  string
}

// FormatBytes formats bytes to human-readable size
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return formatInt64(bytes) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatFloat64(float64(bytes)/float64(div)) + " " + string("KMGTPE"[exp]) + "B"
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func formatInt64(n int64) string {
	return formatFloat64(float64(n))
}

func formatFloat64(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%.1f", f)
}

