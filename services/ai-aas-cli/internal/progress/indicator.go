// Package progress provides progress indicators for long-running operations.
//
// Purpose:
//
//	Display progress indicators for operations exceeding 30 seconds, showing percentage
//	complete and estimated time remaining. Emit progress events suitable for monitoring
//	systems and CI logs.
//
// Dependencies:
//   - encoding/json: Structured progress event output
//   - time: Duration tracking
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-004 (progress indicators for long-running operations)
//   - specs/009-admin-cli/spec.md#NFR-021 (progress indicators suitable for CI logs)
//   - specs/009-admin-cli/spec.md#NFR-027 (progress events for monitoring systems)
//
package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Indicator displays progress for long-running operations.
type Indicator struct {
	writer      io.Writer
	minDuration time.Duration
	format      string // table, json
	enabled     bool
}

// NewIndicator creates a new progress indicator.
func NewIndicator(w io.Writer, format string) *Indicator {
	if w == nil {
		w = os.Stderr
	}
	return &Indicator{
		writer:      w,
		minDuration: 30 * time.Second, // NFR-027: Show progress for operations >30s
		format:      format,
		enabled:     true,
	}
}

// ProgressEvent represents a progress event for monitoring systems.
type ProgressEvent struct {
	Timestamp       string  `json:"timestamp"`
	Operation       string  `json:"operation"`
	PercentComplete float64 `json:"percent_complete"`
	ItemsProcessed  int     `json:"items_processed,omitempty"`
	TotalItems      int     `json:"total_items,omitempty"`
	Elapsed         string  `json:"elapsed"`
	Remaining       string  `json:"remaining,omitempty"`
}

// Update updates progress display.
func (p *Indicator) Update(op string, processed, total int, elapsed time.Duration) error {
	if !p.enabled {
		return nil
	}

	if total == 0 {
		return nil
	}

	percent := float64(processed) / float64(total) * 100
	remaining := time.Duration(0)
	if processed > 0 {
		avgTimePerItem := elapsed / time.Duration(processed)
		remaining = avgTimePerItem * time.Duration(total-processed)
	}

	if p.format == "json" {
		event := ProgressEvent{
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
			Operation:       op,
			PercentComplete: percent,
			ItemsProcessed:  processed,
			TotalItems:      total,
			Elapsed:         elapsed.String(),
			Remaining:       remaining.String(),
		}
		encoder := json.NewEncoder(p.writer)
		return encoder.Encode(event)
	}

	// Table format: single-line update
	fmt.Fprintf(p.writer, "\r%s: %.1f%% (%d/%d) [elapsed: %s, remaining: %s]",
		op, percent, processed, total, elapsed.Round(time.Second), remaining.Round(time.Second))

	return nil
}

// Complete marks progress as complete.
func (p *Indicator) Complete(op string, total int, elapsed time.Duration) error {
	if !p.enabled {
		return nil
	}

	if p.format == "json" {
		event := ProgressEvent{
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
			Operation:       op,
			PercentComplete: 100,
			ItemsProcessed:  total,
			TotalItems:      total,
			Elapsed:         elapsed.String(),
			Remaining:       "0s",
		}
		encoder := json.NewEncoder(p.writer)
		return encoder.Encode(event)
	}

	// Table format: final update with newline
	fmt.Fprintf(p.writer, "\r%s: 100.0%% (%d/%d) [completed in %s]\n",
		op, total, total, elapsed.Round(time.Second))

	return nil
}

// ShouldShow determines if progress should be shown based on elapsed time.
func (p *Indicator) ShouldShow(elapsed time.Duration) bool {
	return p.enabled && elapsed > p.minDuration
}

