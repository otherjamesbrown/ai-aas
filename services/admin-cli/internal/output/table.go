// Package output provides output formatting for the Admin CLI.
//
// Purpose:
//
//	Format command output in different formats: table (human-readable), JSON (machine-readable),
//	and CSV (for exports). Provides consistent output formatting across all commands.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-006 (structured output for scripting)
//   - specs/009-admin-cli/spec.md#NFR-018 (structured JSON output)
//
package output

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// TableFormatter formats output as a human-readable table.
type TableFormatter struct {
	writer *tabwriter.Writer
}

// NewTableFormatter creates a new table formatter.
func NewTableFormatter(w io.Writer) *TableFormatter {
	return &TableFormatter{
		writer: tabwriter.NewWriter(w, 0, 0, 2, ' ', 0),
	}
}

// WriteHeader writes table headers.
func (t *TableFormatter) WriteHeader(headers ...string) error {
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprint(t.writer, h)
	}
	fmt.Fprintln(t.writer)
	fmt.Fprintln(t.writer, "---\t---")
	return nil
}

// WriteRow writes a table row.
func (t *TableFormatter) WriteRow(values ...string) error {
	for i, v := range values {
		if i > 0 {
			fmt.Fprint(t.writer, "\t")
		}
		fmt.Fprint(t.writer, v)
	}
	fmt.Fprintln(t.writer)
	return nil
}

// Flush flushes the table output.
func (t *TableFormatter) Flush() error {
	return t.writer.Flush()
}

// PrintTable is a convenience function to print a table to stdout.
func PrintTable(headers []string, rows [][]string) error {
	formatter := NewTableFormatter(os.Stdout)
	if err := formatter.WriteHeader(headers...); err != nil {
		return err
	}
	for _, row := range rows {
		if err := formatter.WriteRow(row...); err != nil {
			return err
		}
	}
	return formatter.Flush()
}

