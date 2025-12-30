// Package model provides CLI commands for model management.
package model

import (
	"fmt"
	"regexp"
	"strings"
)

// multiHyphenRegex matches two or more consecutive hyphens
var multiHyphenRegex = regexp.MustCompile(`-{2,}`)

// formatBytes formats a byte count as a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// sanitizeModelName converts a model name to KServe-compatible format.
// KServe requires lowercase alphanumeric with hyphens, no periods or underscores.
// This function:
//   - Converts to lowercase
//   - Replaces underscores with hyphens
//   - Replaces periods with hyphens (for version numbers like v0.3)
//   - Removes consecutive hyphens
//   - Trims leading/trailing hyphens
func sanitizeModelName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")
	// Remove consecutive hyphens using regex (more efficient for many consecutive chars)
	name = multiHyphenRegex.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}
