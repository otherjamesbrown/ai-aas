package output

import (
	"fmt"
	"time"
)

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

// FormatBool formats a boolean for display
func FormatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// FormatDuration formats a duration in a human-readable compact form.
// Examples: "5m", "2h 30m", "3d 4h", "7d 17h"
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "0s"
	}

	seconds := int(d.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	// Less than 1 minute
	if minutes == 0 {
		return fmt.Sprintf("%ds", seconds)
	}

	// Less than 1 hour
	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	// Less than 1 day
	if days == 0 {
		remainingMinutes := minutes % 60
		if remainingMinutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
	}

	// 1 day or more
	remainingHours := hours % 24
	if remainingHours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, remainingHours)
}

// FormatDurationSince formats the duration since a given time.
// Returns empty string if the time is nil or zero.
func FormatDurationSince(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return FormatDuration(time.Since(*t))
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
