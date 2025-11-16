// Package output provides CSV output formatting for the Admin CLI.
//
// Purpose:
//
//	Format export output as CSV with column headers, schema comments, and proper
//	formatting. Used for usage and membership exports with reconciliation support.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-005 (exports with column definitions and schema headers)
//   - specs/009-admin-cli/spec.md#FR-006 (CSV format for exports)
//
package output

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

// CSVFormatter formats output as CSV with schema headers.
type CSVFormatter struct {
	writer *csv.Writer
	file   *os.File
}

// NewCSVFormatter creates a new CSV formatter writing to the specified file.
func NewCSVFormatter(filePath string) (*CSVFormatter, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600) // NFR-013: 0600 permissions
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV file: %w", err)
	}

	writer := csv.NewWriter(file)

	return &CSVFormatter{
		writer: writer,
		file:   file,
	}, nil
}

// WriteSchemaComment writes a schema comment as a CSV comment line.
func (c *CSVFormatter) WriteSchemaComment(comment string) error {
	// CSV comments are typically lines starting with #
	_, err := fmt.Fprintf(c.file, "# %s\n", comment)
	return err
}

// WriteHeader writes CSV column headers.
func (c *CSVFormatter) WriteHeader(headers []string) error {
	return c.writer.Write(headers)
}

// WriteRow writes a CSV data row.
func (c *CSVFormatter) WriteRow(row []string) error {
	return c.writer.Write(row)
}

// WriteMetadata writes export metadata as CSV comments.
func (c *CSVFormatter) WriteMetadata(metadata map[string]interface{}) error {
	c.WriteSchemaComment("Export Metadata")
	for key, value := range metadata {
		c.WriteSchemaComment(fmt.Sprintf("%s: %v", key, value))
	}
	c.WriteSchemaComment(fmt.Sprintf("Export Date: %s", time.Now().UTC().Format(time.RFC3339)))
	return nil
}

// Flush flushes the CSV writer and closes the file.
func (c *CSVFormatter) Flush() error {
	c.writer.Flush()
	if err := c.writer.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV: %w", err)
	}
	return nil
}

// Close closes the CSV file.
func (c *CSVFormatter) Close() error {
	if err := c.Flush(); err != nil {
		return err
	}
	return c.file.Close()
}

