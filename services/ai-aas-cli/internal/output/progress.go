// Package output provides output formatting utilities for the CLI.
package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ProgressBar wraps progressbar for consistent CLI progress indication
type ProgressBar struct {
	bar    *progressbar.ProgressBar
	writer io.Writer
}

// ProgressBarOptions configures progress bar behavior
type ProgressBarOptions struct {
	Description    string
	Total          int64
	ShowBytes      bool
	ShowSpeed      bool
	ShowPercentage bool
	ShowTime       bool
	Width          int
	Writer         io.Writer
}

// DefaultProgressOptions returns sensible defaults for progress bar
func DefaultProgressOptions(description string, total int64) ProgressBarOptions {
	return ProgressBarOptions{
		Description:    description,
		Total:          total,
		ShowBytes:      true,
		ShowSpeed:      true,
		ShowPercentage: true,
		ShowTime:       true,
		Width:          40,
		Writer:         os.Stderr,
	}
}

// NewProgressBar creates a new progress bar with the given options
func NewProgressBar(opts ProgressBarOptions) *ProgressBar {
	if opts.Writer == nil {
		opts.Writer = os.Stderr
	}
	if opts.Width == 0 {
		opts.Width = 40
	}

	pbOpts := []progressbar.Option{
		progressbar.OptionSetDescription(opts.Description),
		progressbar.OptionSetWriter(opts.Writer),
		progressbar.OptionSetWidth(opts.Width),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	}

	if opts.ShowBytes {
		pbOpts = append(pbOpts, progressbar.OptionShowBytes(true))
	}
	if opts.ShowPercentage {
		pbOpts = append(pbOpts, progressbar.OptionSetPredictTime(true))
	}
	if opts.ShowTime {
		pbOpts = append(pbOpts, progressbar.OptionShowElapsedTimeOnFinish())
	}

	bar := progressbar.NewOptions64(opts.Total, pbOpts...)

	return &ProgressBar{
		bar:    bar,
		writer: opts.Writer,
	}
}

// NewDownloadProgressBar creates a progress bar optimized for downloads
func NewDownloadProgressBar(description string, totalBytes int64) *ProgressBar {
	opts := DefaultProgressOptions(description, totalBytes)
	return NewProgressBar(opts)
}

// Add adds to the progress
func (p *ProgressBar) Add(n int) error {
	return p.bar.Add(n)
}

// Add64 adds to the progress (int64 version)
func (p *ProgressBar) Add64(n int64) error {
	return p.bar.Add64(n)
}

// Set sets the progress to a specific value
func (p *ProgressBar) Set(n int) error {
	return p.bar.Set(n)
}

// Set64 sets the progress to a specific value (int64 version)
func (p *ProgressBar) Set64(n int64) error {
	return p.bar.Set64(n)
}

// Finish completes the progress bar
func (p *ProgressBar) Finish() error {
	return p.bar.Finish()
}

// Clear clears the progress bar
func (p *ProgressBar) Clear() error {
	return p.bar.Clear()
}

// ChangeMax changes the total value
func (p *ProgressBar) ChangeMax(max int) {
	p.bar.ChangeMax(max)
}

// ChangeMax64 changes the total value (int64 version)
func (p *ProgressBar) ChangeMax64(max int64) {
	p.bar.ChangeMax64(max)
}

// Describe updates the description
func (p *ProgressBar) Describe(description string) {
	p.bar.Describe(description)
}

// Spinner provides a simple spinner for indeterminate progress
type Spinner struct {
	done    chan bool
	message string
	writer  io.Writer
	mu      sync.Mutex
}

// NewSpinner creates a new spinner
func NewSpinner(message string) *Spinner {
	return &Spinner{
		done:    make(chan bool),
		message: message,
		writer:  os.Stderr,
	}
}

// Start starts the spinner animation
func (s *Spinner) Start() {
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.done:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()
				fmt.Fprintf(s.writer, "\r%s... done\n", msg)
				return
			default:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()
				fmt.Fprintf(s.writer, "\r%s %s", frames[i], msg)
				i = (i + 1) % len(frames)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.done <- true
}

// StopWithMessage stops the spinner with a custom message
func (s *Spinner) StopWithMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
	s.done <- true
}

