package domain

import (
	"testing"
)

func TestIsValidPlanTier(t *testing.T) {
	tests := []struct {
		name string
		tier string
		want bool
	}{
		{
			name: "valid tier - free",
			tier: "free",
			want: true,
		},
		{
			name: "valid tier - starter",
			tier: "starter",
			want: true,
		},
		{
			name: "valid tier - enterprise",
			tier: "enterprise",
			want: true,
		},
		{
			name: "invalid tier - basic",
			tier: "basic",
			want: false,
		},
		{
			name: "invalid tier - premium",
			tier: "premium",
			want: false,
		},
		{
			name: "invalid tier - pro",
			tier: "pro",
			want: false,
		},
		{
			name: "invalid tier - empty string",
			tier: "",
			want: false,
		},
		{
			name: "invalid tier - FREE uppercase",
			tier: "FREE",
			want: false,
		},
		{
			name: "invalid tier - Free mixed case",
			tier: "Free",
			want: false,
		},
		{
			name: "invalid tier - with whitespace",
			tier: " free ",
			want: false,
		},
		{
			name: "invalid tier - trial",
			tier: "trial",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPlanTier(tt.tier)
			if got != tt.want {
				t.Errorf("IsValidPlanTier(%q) = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestIsValidOrgStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{
			name:   "valid status - active",
			status: "active",
			want:   true,
		},
		{
			name:   "valid status - suspended",
			status: "suspended",
			want:   true,
		},
		{
			name:   "valid status - deleted",
			status: "deleted",
			want:   true,
		},
		{
			name:   "invalid status - inactive",
			status: "inactive",
			want:   false,
		},
		{
			name:   "invalid status - pending",
			status: "pending",
			want:   false,
		},
		{
			name:   "invalid status - archived",
			status: "archived",
			want:   false,
		},
		{
			name:   "invalid status - empty string",
			status: "",
			want:   false,
		},
		{
			name:   "invalid status - ACTIVE uppercase",
			status: "ACTIVE",
			want:   false,
		},
		{
			name:   "invalid status - Active mixed case",
			status: "Active",
			want:   false,
		},
		{
			name:   "invalid status - with whitespace",
			status: " active ",
			want:   false,
		},
		{
			name:   "invalid status - enabled",
			status: "enabled",
			want:   false,
		},
		{
			name:   "invalid status - disabled",
			status: "disabled",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidOrgStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsValidOrgStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestValidPlanTiers(t *testing.T) {
	// Ensure the list contains expected values
	expected := []string{"free", "starter", "enterprise"}

	if len(ValidPlanTiers) != len(expected) {
		t.Errorf("ValidPlanTiers length = %d, want %d", len(ValidPlanTiers), len(expected))
	}

	// Check all expected values are present
	for _, exp := range expected {
		found := false
		for _, tier := range ValidPlanTiers {
			if tier == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidPlanTiers missing expected tier: %q", exp)
		}
	}
}

func TestValidOrgStatuses(t *testing.T) {
	// Ensure the list contains expected values
	expected := []string{"active", "suspended", "deleted"}

	if len(ValidOrgStatuses) != len(expected) {
		t.Errorf("ValidOrgStatuses length = %d, want %d", len(ValidOrgStatuses), len(expected))
	}

	// Check all expected values are present
	for _, exp := range expected {
		found := false
		for _, status := range ValidOrgStatuses {
			if status == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidOrgStatuses missing expected status: %q", exp)
		}
	}
}
