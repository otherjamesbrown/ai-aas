// Package output provides tests for progress indicators.
package output

import (
	"bytes"
	"testing"
	"time"
)

func TestDefaultProgressOptions(t *testing.T) {
	tests := []struct {
		name        string
		description string
		total       int64
	}{
		{"normal options", "Downloading", 1024},
		{"zero total", "Processing", 0},
		{"large total", "Uploading", 1024 * 1024 * 1024},
		{"empty description", "", 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultProgressOptions(tt.description, tt.total)

			if opts.Description != tt.description {
				t.Errorf("Description = %q, want %q", opts.Description, tt.description)
			}
			if opts.Total != tt.total {
				t.Errorf("Total = %d, want %d", opts.Total, tt.total)
			}
			if !opts.ShowBytes {
				t.Error("ShowBytes should be true by default")
			}
			if !opts.ShowSpeed {
				t.Error("ShowSpeed should be true by default")
			}
			if !opts.ShowPercentage {
				t.Error("ShowPercentage should be true by default")
			}
			if !opts.ShowTime {
				t.Error("ShowTime should be true by default")
			}
			if opts.Width != 40 {
				t.Errorf("Width = %d, want 40", opts.Width)
			}
		})
	}
}

func TestNewProgressBar(t *testing.T) {
	tests := []struct {
		name string
		opts ProgressBarOptions
	}{
		{
			name: "default options",
			opts: DefaultProgressOptions("Test", 100),
		},
		{
			name: "custom width",
			opts: ProgressBarOptions{
				Description: "Custom",
				Total:       50,
				Width:       80,
			},
		},
		{
			name: "nil writer",
			opts: ProgressBarOptions{
				Description: "Test",
				Total:       100,
				Writer:      nil, // Should default to os.Stderr
			},
		},
		{
			name: "zero width",
			opts: ProgressBarOptions{
				Description: "Test",
				Total:       100,
				Width:       0, // Should default to 40
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar(tt.opts)

			if pb == nil {
				t.Fatal("NewProgressBar() returned nil")
			}
			if pb.bar == nil {
				t.Error("progress bar not initialized")
			}
		})
	}
}

func TestNewProgressBar_CustomWriter(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
		Width:       40,
	}

	pb := NewProgressBar(opts)
	if pb == nil {
		t.Fatal("NewProgressBar() returned nil")
	}
	if pb.writer != &buf {
		t.Error("custom writer not set correctly")
	}
}

func TestNewDownloadProgressBar(t *testing.T) {
	tests := []struct {
		name        string
		description string
		totalBytes  int64
	}{
		{"small download", "test.txt", 1024},
		{"large download", "model.bin", 1024 * 1024 * 1024},
		{"zero bytes", "empty.dat", 0},
		{"empty description", "", 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewDownloadProgressBar(tt.description, tt.totalBytes)

			if pb == nil {
				t.Fatal("NewDownloadProgressBar() returned nil")
			}
			if pb.bar == nil {
				t.Error("progress bar not initialized")
			}
		})
	}
}

func TestProgressBar_Add(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	err := pb.Add(10)
	if err != nil {
		t.Errorf("Add() error = %v", err)
	}

	err = pb.Add(20)
	if err != nil {
		t.Errorf("Add() error = %v", err)
	}
}

func TestProgressBar_Add64(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       1000000,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	err := pb.Add64(500000)
	if err != nil {
		t.Errorf("Add64() error = %v", err)
	}
}

func TestProgressBar_Set(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	err := pb.Set(50)
	if err != nil {
		t.Errorf("Set() error = %v", err)
	}

	err = pb.Set(100)
	if err != nil {
		t.Errorf("Set() error = %v", err)
	}
}

func TestProgressBar_Set64(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       1000000,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	err := pb.Set64(750000)
	if err != nil {
		t.Errorf("Set64() error = %v", err)
	}
}

func TestProgressBar_Finish(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	err := pb.Finish()
	if err != nil {
		t.Errorf("Finish() error = %v", err)
	}
}

func TestProgressBar_Clear(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	// Add some progress first
	pb.Add(50)

	err := pb.Clear()
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}
}

func TestProgressBar_ChangeMax(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	// Should not panic
	pb.ChangeMax(200)
	pb.ChangeMax(50)
}

func TestProgressBar_ChangeMax64(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Test",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	// Should not panic
	pb.ChangeMax64(1000000)
	pb.ChangeMax64(50)
}

func TestProgressBar_Describe(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Initial",
		Total:       100,
		Writer:      &buf,
	}
	pb := NewProgressBar(opts)

	// Should not panic
	pb.Describe("Updated description")
	pb.Describe("")
	pb.Describe("Final description")
}

func TestProgressBar_FullWorkflow(t *testing.T) {
	var buf bytes.Buffer
	opts := ProgressBarOptions{
		Description: "Download",
		Total:       100,
		Writer:      &buf,
		ShowBytes:   true,
		ShowSpeed:   true,
	}
	pb := NewProgressBar(opts)

	// Simulate a download
	for i := 0; i < 10; i++ {
		if err := pb.Add(10); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	if err := pb.Finish(); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	// Verify something was written to buffer
	if buf.Len() == 0 {
		t.Error("expected output to buffer, got nothing")
	}
}

func TestNewSpinner(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"normal message", "Loading"},
		{"empty message", ""},
		{"unicode message", "読み込み中"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := NewSpinner(tt.message)

			if spinner == nil {
				t.Fatal("NewSpinner() returned nil")
			}
			if spinner.message != tt.message {
				t.Errorf("message = %q, want %q", spinner.message, tt.message)
			}
			if spinner.done == nil {
				t.Error("done channel not initialized")
			}
		})
	}
}

func TestSpinner_StartStop(t *testing.T) {
	spinner := NewSpinner("Testing")

	spinner.Start()

	// Let it spin for a short time
	time.Sleep(250 * time.Millisecond)

	spinner.Stop()

	// Give it time to finish
	time.Sleep(50 * time.Millisecond)
}

func TestSpinner_StopWithMessage(t *testing.T) {
	spinner := NewSpinner("Initial")

	spinner.Start()

	// Let it spin for a short time
	time.Sleep(250 * time.Millisecond)

	spinner.StopWithMessage("Completed")

	// Give it time to finish
	time.Sleep(50 * time.Millisecond)
}

func TestSpinner_MultipleSpinners(t *testing.T) {
	// Test that multiple spinners can be created without issues
	spinner1 := NewSpinner("Task 1")
	spinner2 := NewSpinner("Task 2")

	if spinner1 == nil || spinner2 == nil {
		t.Fatal("NewSpinner() returned nil")
	}

	// Start and stop in sequence
	spinner1.Start()
	time.Sleep(100 * time.Millisecond)
	spinner1.Stop()

	time.Sleep(50 * time.Millisecond)

	spinner2.Start()
	time.Sleep(100 * time.Millisecond)
	spinner2.Stop()

	time.Sleep(50 * time.Millisecond)
}

func TestProgressBarOptions_AllFlags(t *testing.T) {
	// Test all flag combinations
	tests := []struct {
		name string
		opts ProgressBarOptions
	}{
		{
			name: "all enabled",
			opts: ProgressBarOptions{
				Description:    "All",
				Total:          100,
				ShowBytes:      true,
				ShowSpeed:      true,
				ShowPercentage: true,
				ShowTime:       true,
				Width:          40,
			},
		},
		{
			name: "all disabled",
			opts: ProgressBarOptions{
				Description:    "None",
				Total:          100,
				ShowBytes:      false,
				ShowSpeed:      false,
				ShowPercentage: false,
				ShowTime:       false,
				Width:          40,
			},
		},
		{
			name: "bytes only",
			opts: ProgressBarOptions{
				Description:    "Bytes",
				Total:          100,
				ShowBytes:      true,
				ShowSpeed:      false,
				ShowPercentage: false,
				ShowTime:       false,
				Width:          40,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.opts.Writer = &buf

			pb := NewProgressBar(tt.opts)
			if pb == nil {
				t.Fatal("NewProgressBar() returned nil")
			}

			// Should work regardless of flag combination
			err := pb.Add(50)
			if err != nil {
				t.Errorf("Add() error = %v", err)
			}
			err = pb.Finish()
			if err != nil {
				t.Errorf("Finish() error = %v", err)
			}
		})
	}
}
