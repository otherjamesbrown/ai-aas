package pod

import (
	"strings"
	"testing"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/pod"
)

func TestTruncatePodName(t *testing.T) {
	tests := []struct {
		name     string
		podName  string
		maxLen   int
		expected string
	}{
		{
			name:     "short name unchanged",
			podName:  "short-pod",
			maxLen:   43,
			expected: "short-pod",
		},
		{
			name:     "exact length unchanged",
			podName:  strings.Repeat("a", 43),
			maxLen:   43,
			expected: strings.Repeat("a", 43),
		},
		{
			name:     "long name truncated",
			podName:  "gpt-oss-20b-predictor-00001-deployment-abc123-def456-ghi789",
			maxLen:   43,
			expected: "gpt-oss-20b-predictor-00001-deployment-a...",
		},
		{
			name:     "very long name truncated",
			podName:  strings.Repeat("x", 100),
			maxLen:   43,
			expected: strings.Repeat("x", 40) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePodName(tt.podName, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncatePodName(%q, %d) = %q, expected %q", tt.podName, tt.maxLen, result, tt.expected)
			}

			if len(result) > tt.maxLen {
				t.Errorf("truncatePodName result length %d exceeds maxLen %d", len(result), tt.maxLen)
			}
		})
	}
}

func TestTruncateNodeName(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		expected string
	}{
		{
			name:     "short node name unchanged",
			nodeName: "node-1",
			expected: "node-1",
		},
		{
			name:     "medium node name unchanged",
			nodeName: "lke-pool-abc123",
			expected: "lke-pool-abc123",
		},
		{
			name:     "long LKE node name truncated",
			nodeName: "lke123456-78901-abc123def456",
			expected: "lke123456-78...",
		},
		{
			name:     "very long node name truncated",
			nodeName: strings.Repeat("x", 50),
			expected: strings.Repeat("x", 12) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateNodeName(tt.nodeName)
			if result != tt.expected {
				t.Errorf("truncateNodeName(%q) = %q, expected %q", tt.nodeName, result, tt.expected)
			}

			if len(result) > 16 {
				t.Errorf("truncateNodeName result length %d exceeds max 16", len(result))
			}
		})
	}
}

func TestFormatRestartCount(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		count       int
		lastRestart *time.Time
		expected    string
	}{
		{
			name:        "zero restarts",
			count:       0,
			lastRestart: nil,
			expected:    "0",
		},
		{
			name:        "restarts without time",
			count:       3,
			lastRestart: nil,
			expected:    "3",
		},
		{
			name:        "restarts with time - seconds",
			count:       1,
			lastRestart: timePtr(now.Add(-30 * time.Second)),
			expected:    "1 (30s ago)",
		},
		{
			name:        "restarts with time - minutes",
			count:       2,
			lastRestart: timePtr(now.Add(-5 * time.Minute)),
			expected:    "2 (5m ago)",
		},
		{
			name:        "restarts with time - hours",
			count:       5,
			lastRestart: timePtr(now.Add(-3 * time.Hour)),
			expected:    "5 (3h ago)",
		},
		{
			name:        "restarts with time - days",
			count:       10,
			lastRestart: timePtr(now.Add(-2 * 24 * time.Hour)),
			expected:    "10 (2d ago)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRestartCount(tt.count, tt.lastRestart)
			if result != tt.expected {
				t.Errorf("formatRestartCount(%d, %v) = %q, expected %q", tt.count, tt.lastRestart, result, tt.expected)
			}
		})
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int64
		expected string
	}{
		{
			name:     "seconds",
			seconds:  30,
			expected: "30s",
		},
		{
			name:     "exactly 1 minute",
			seconds:  60,
			expected: "1m",
		},
		{
			name:     "minutes",
			seconds:  300, // 5 minutes
			expected: "5m",
		},
		{
			name:     "exactly 1 hour",
			seconds:  3600,
			expected: "1h",
		},
		{
			name:     "hours",
			seconds:  7200, // 2 hours
			expected: "2h",
		},
		{
			name:     "exactly 1 day",
			seconds:  86400,
			expected: "1d",
		},
		{
			name:     "days",
			seconds:  172800, // 2 days
			expected: "2d",
		},
		{
			name:     "weeks in days",
			seconds:  604800, // 7 days
			expected: "7d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAge(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatAge(%d) = %q, expected %q", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0s",
		},
		{
			name:     "less than minute shows seconds",
			duration: 45 * time.Second,
			expected: "45s",
		},
		{
			name:     "less than hour shows minutes",
			duration: 45 * time.Minute,
			expected: "45m",
		},
		{
			name:     "less than day shows hours",
			duration: 12 * time.Hour,
			expected: "12h",
		},
		{
			name:     "more than day shows days",
			duration: 36 * time.Hour,
			expected: "1d",
		},
		{
			name:     "multiple days",
			duration: 5 * 24 * time.Hour,
			expected: "5d",
		},
		{
			name:     "fractional units round down",
			duration: 90 * time.Second, // 1.5 minutes
			expected: "1m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, expected %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatTermination(t *testing.T) {
	tests := []struct {
		name     string
		term     *pod.TerminationInfo
		expected string
	}{
		{
			name:     "nil termination",
			term:     nil,
			expected: "-",
		},
		{
			name: "OOMKilled",
			term: &pod.TerminationInfo{
				Reason:   "OOMKilled",
				ExitCode: 137,
			},
			expected: "OOMKilled (exit 137)",
		},
		{
			name: "Error",
			term: &pod.TerminationInfo{
				Reason:   "Error",
				ExitCode: 1,
			},
			expected: "Error (exit 1)",
		},
		{
			name: "CrashLoopBackOff",
			term: &pod.TerminationInfo{
				Reason:   "CrashLoopBackOff",
				ExitCode: 143,
			},
			expected: "CrashLoopBackOff (exit 143)",
		},
		{
			name: "very long reason truncated",
			term: &pod.TerminationInfo{
				Reason:   "VeryLongReasonThatExceedsTheMaximumLength",
				ExitCode: 1,
			},
			expected: "VeryLongReasonThatExceedsTh...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTermination(tt.term)
			if result != tt.expected {
				t.Errorf("formatTermination(%v) = %q, expected %q", tt.term, result, tt.expected)
			}

			// Verify length constraint
			if len(result) > 30 && result != "-" {
				t.Errorf("formatTermination result length %d exceeds max 30", len(result))
			}
		})
	}
}

func TestPrintHealthTable_EmptyPods(t *testing.T) {
	resp := &pod.HealthResponse{
		Pods: []pod.PodHealth{},
		Summary: pod.HealthSummary{
			TotalPods:     0,
			HealthyPods:   0,
			UnhealthyPods: 0,
			TotalRestarts: 0,
		},
	}

	// Should not panic or error on empty pods
	err := printHealthTable(resp, false)
	if err != nil {
		t.Errorf("printHealthTable failed on empty pods: %v", err)
	}
}

func TestPrintHealthTable_WithPods(t *testing.T) {
	now := time.Now()
	resp := &pod.HealthResponse{
		Pods: []pod.PodHealth{
			{
				Name:         "gpt-oss-20b-predictor-00001-deployment-abc123",
				Namespace:    "system",
				ModelName:    "gpt-oss-20b",
				Phase:        "Running",
				Ready:        true,
				Node:         "lke-pool-abc123",
				RestartCount: 0,
				AgeSeconds:   86400, // 1 day
				CreatedAt:    now.Add(-24 * time.Hour),
			},
			{
				Name:            "mistral-7b-predictor-00001-deployment-def456",
				Namespace:       "system",
				ModelName:       "mistral-7b",
				Phase:           "Running",
				Ready:           true,
				Node:            "lke-pool-def456",
				RestartCount:    2,
				LastRestartTime: timePtr(now.Add(-3 * time.Hour)),
				AgeSeconds:      432000, // 5 days
				CreatedAt:       now.Add(-5 * 24 * time.Hour),
			},
		},
		Summary: pod.HealthSummary{
			TotalPods:        2,
			HealthyPods:      2,
			UnhealthyPods:    0,
			TotalRestarts:    2,
			PodsWithRestarts: 1,
		},
	}

	// Test without details
	err := printHealthTable(resp, false)
	if err != nil {
		t.Errorf("printHealthTable failed: %v", err)
	}

	// Test with details
	err = printHealthTable(resp, true)
	if err != nil {
		t.Errorf("printHealthTable with details failed: %v", err)
	}
}

func TestPrintHealthTable_WithUnhealthyPods(t *testing.T) {
	now := time.Now()
	resp := &pod.HealthResponse{
		Pods: []pod.PodHealth{
			{
				Name:            "llama-3-8b-predictor-00001-deployment-ghi789",
				Namespace:       "system",
				ModelName:       "llama-3-8b",
				Phase:           "CrashLoopBackOff",
				Ready:           false,
				Node:            "lke-pool-ghi789",
				RestartCount:    5,
				LastRestartTime: timePtr(now.Add(-10 * time.Minute)),
				LastTermination: &pod.TerminationInfo{
					Reason:   "OOMKilled",
					ExitCode: 137,
					Message:  "Container exceeded memory limit",
				},
				AgeSeconds: 3600, // 1 hour
				CreatedAt:  now.Add(-1 * time.Hour),
			},
		},
		Summary: pod.HealthSummary{
			TotalPods:        1,
			HealthyPods:      0,
			UnhealthyPods:    1,
			TotalRestarts:    5,
			PodsWithRestarts: 1,
		},
	}

	// Should print next steps when there are unhealthy pods
	err := printHealthTable(resp, true)
	if err != nil {
		t.Errorf("printHealthTable failed: %v", err)
	}
}

func TestPrintHealthTable_DetailedOutput(t *testing.T) {
	now := time.Now()
	resp := &pod.HealthResponse{
		Pods: []pod.PodHealth{
			{
				Name:      "test-pod",
				ModelName: "test-model",
				Phase:     "Running",
				Ready:     true,
				Node:      "node-1",
				LastTermination: &pod.TerminationInfo{
					Reason:   "Error",
					ExitCode: 1,
				},
				AgeSeconds: 3600,
				CreatedAt:  now,
			},
		},
		Summary: pod.HealthSummary{
			TotalPods:   1,
			HealthyPods: 1,
		},
	}

	// Test with details flag - should include LAST TERM column
	err := printHealthTable(resp, true)
	if err != nil {
		t.Errorf("printHealthTable with details failed: %v", err)
	}
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
