// Package output provides output formatting utilities for CLI applications.
package output

import (
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
