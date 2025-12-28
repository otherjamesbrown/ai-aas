package domain

import (
	"testing"
)

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{
			name:   "valid status - ready",
			status: "ready",
			want:   true,
		},
		{
			name:   "valid status - degraded",
			status: "degraded",
			want:   true,
		},
		{
			name:   "valid status - offline",
			status: "offline",
			want:   true,
		},
		{
			name:   "valid status - pending",
			status: "pending",
			want:   true,
		},
		{
			name:   "invalid status - unknown",
			status: "unknown",
			want:   false,
		},
		{
			name:   "invalid status - empty string",
			status: "",
			want:   false,
		},
		{
			name:   "invalid status - READY uppercase",
			status: "READY",
			want:   false,
		},
		{
			name:   "invalid status - Ready mixed case",
			status: "Ready",
			want:   false,
		},
		{
			name:   "invalid status - running",
			status: "running",
			want:   false,
		},
		{
			name:   "invalid status - failed",
			status: "failed",
			want:   false,
		},
		{
			name:   "invalid status - with whitespace",
			status: " ready ",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsValidEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{
			name: "valid environment - development",
			env:  "development",
			want: true,
		},
		{
			name: "valid environment - staging",
			env:  "staging",
			want: true,
		},
		{
			name: "valid environment - production",
			env:  "production",
			want: true,
		},
		{
			name: "invalid environment - dev",
			env:  "dev",
			want: false,
		},
		{
			name: "invalid environment - prod",
			env:  "prod",
			want: false,
		},
		{
			name: "invalid environment - test",
			env:  "test",
			want: false,
		},
		{
			name: "invalid environment - empty string",
			env:  "",
			want: false,
		},
		{
			name: "invalid environment - DEVELOPMENT uppercase",
			env:  "DEVELOPMENT",
			want: false,
		},
		{
			name: "invalid environment - Development mixed case",
			env:  "Development",
			want: false,
		},
		{
			name: "invalid environment - with whitespace",
			env:  " development ",
			want: false,
		},
		{
			name: "invalid environment - qa",
			env:  "qa",
			want: false,
		},
		{
			name: "invalid environment - local",
			env:  "local",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidEnvironment(tt.env)
			if got != tt.want {
				t.Errorf("IsValidEnvironment(%q) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestValidDeploymentStatuses(t *testing.T) {
	// Ensure the list contains expected values
	expected := []string{"ready", "degraded", "offline", "pending"}

	if len(ValidDeploymentStatuses) != len(expected) {
		t.Errorf("ValidDeploymentStatuses length = %d, want %d", len(ValidDeploymentStatuses), len(expected))
	}

	// Check all expected values are present
	for _, exp := range expected {
		found := false
		for _, status := range ValidDeploymentStatuses {
			if status == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidDeploymentStatuses missing expected status: %q", exp)
		}
	}
}

func TestValidDeploymentEnvironments(t *testing.T) {
	// Ensure the list contains expected values
	expected := []string{"development", "staging", "production"}

	if len(ValidDeploymentEnvironments) != len(expected) {
		t.Errorf("ValidDeploymentEnvironments length = %d, want %d", len(ValidDeploymentEnvironments), len(expected))
	}

	// Check all expected values are present
	for _, exp := range expected {
		found := false
		for _, env := range ValidDeploymentEnvironments {
			if env == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidDeploymentEnvironments missing expected environment: %q", exp)
		}
	}
}
