// Package output provides tests for duration formatting.
package output

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		// Negative and zero
		{"negative duration", -5 * time.Second, "0s"},
		{"zero duration", 0, "0s"},

		// Seconds only
		{"1 second", 1 * time.Second, "1s"},
		{"30 seconds", 30 * time.Second, "30s"},
		{"59 seconds", 59 * time.Second, "59s"},

		// Minutes only
		{"1 minute", 1 * time.Minute, "1m"},
		{"5 minutes", 5 * time.Minute, "5m"},
		{"30 minutes", 30 * time.Minute, "30m"},
		{"59 minutes", 59 * time.Minute, "59m"},

		// Hours (no minutes component)
		{"1 hour exact", 1 * time.Hour, "1h"},
		{"2 hours exact", 2 * time.Hour, "2h"},
		{"23 hours exact", 23 * time.Hour, "23h"},

		// Hours and minutes
		{"1 hour 30 minutes", 1*time.Hour + 30*time.Minute, "1h 30m"},
		{"2 hours 15 minutes", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"5 hours 1 minute", 5*time.Hour + 1*time.Minute, "5h 1m"},
		{"23 hours 59 minutes", 23*time.Hour + 59*time.Minute, "23h 59m"},

		// Days (no hours component)
		{"1 day exact", 24 * time.Hour, "1d"},
		{"2 days exact", 48 * time.Hour, "2d"},
		{"7 days exact", 7 * 24 * time.Hour, "7d"},

		// Days and hours
		{"1 day 1 hour", 25 * time.Hour, "1d 1h"},
		{"1 day 12 hours", 36 * time.Hour, "1d 12h"},
		{"2 days 5 hours", 53 * time.Hour, "2d 5h"},
		{"7 days 17 hours", 7*24*time.Hour + 17*time.Hour, "7d 17h"},

		// Edge cases: seconds are ignored in higher units
		{"1 minute 30 seconds", 1*time.Minute + 30*time.Second, "1m"},
		{"1 hour 30 minutes 45 seconds", 1*time.Hour + 30*time.Minute + 45*time.Second, "1h 30m"},
		{"1 day 1 hour 30 minutes", 25*time.Hour + 30*time.Minute, "1d 1h"},

		// Large values
		{"30 days", 30 * 24 * time.Hour, "30d"},
		{"365 days", 365 * 24 * time.Hour, "365d"},
		{"365 days 6 hours", 365*24*time.Hour + 6*time.Hour, "365d 6h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestFormatDurationSince(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    *time.Time
		want string
	}{
		{
			name: "nil time",
			t:    nil,
			want: "",
		},
		{
			name: "zero time",
			t:    &time.Time{},
			want: "",
		},
		{
			name: "1 minute ago",
			t:    ptr(now.Add(-1 * time.Minute)),
			want: "1m",
		},
		{
			name: "1 hour ago",
			t:    ptr(now.Add(-1 * time.Hour)),
			want: "1h",
		},
		{
			name: "1 day ago",
			t:    ptr(now.Add(-24 * time.Hour)),
			want: "1d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDurationSince(tt.t)
			if got != tt.want {
				t.Errorf("FormatDurationSince(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

func TestFormatDurationSince_FutureTime(t *testing.T) {
	// Test with a future time (should return "0s" for negative durations)
	future := time.Now().Add(1 * time.Hour)
	got := FormatDurationSince(&future)

	// Should handle negative duration (future time)
	if got != "0s" {
		t.Errorf("FormatDurationSince(future) = %q, want %q", got, "0s")
	}
}

func TestFormatDurationSince_RecentTime(t *testing.T) {
	// Test with a very recent time
	recent := time.Now().Add(-30 * time.Second)
	got := FormatDurationSince(&recent)

	// Should be in seconds format
	if got != "30s" {
		t.Errorf("FormatDurationSince(30s ago) = %q, want %q", got, "30s")
	}
}

// Helper function to create time pointers
func ptr(t time.Time) *time.Time {
	return &t
}
