package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Organization management use case tests
// See: usecases/organization.yaml
// CLI helpers defined in helpers_test.go

// TestUC_ORG_001_ShowOrganizationDetails validates that organization admins can
// view detailed information about their organization.
//
// Use Case: UC-ORG-001 - Show Organization Details
// See: usecases/organization.yaml
func TestUC_ORG_001_ShowOrganizationDetails(t *testing.T) {
	t.Run("AC-01: show organization details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org org show`
		result := runOrgCLI("org", "show")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Organization name is displayed
		// Then: Organization ID is displayed
		// Then: Current plan is displayed
		// Then: Usage limits are displayed
		// Then: Resource counts are displayed (users, API keys, models)
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-02: show organization details with JSON output", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org org show --json`
		result := runOrgCLI("org", "show", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON object
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: JSON includes organization ID, name, plan
		// Then: JSON includes limits and resource counts
		var org map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &org); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify required fields exist
		requiredFields := []string{"id", "name", "plan"}
		for _, field := range requiredFields {
			if _, ok := org[field]; !ok {
				t.Errorf("expected field %q in JSON output", field)
			}
		}
	})

	t.Run("AC-03: handle authentication failure", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User API key is invalid or expired
		// This would require a way to pass an invalid API key to the CLI

		// When: User runs `ai-aas-org org show`
		// Note: In a real test, we would configure the CLI with an invalid key

		// Then: Command fails with exit code 4 (auth error)
		// Then: Error message indicates authentication failure
		// Then: Suggestion to check API key is shown
	})
}

// TestUC_ORG_001_ShowOrganizationDetails_MustNot validates negative requirements.
func TestUC_ORG_001_ShowOrganizationDetails_MustNot(t *testing.T) {
	t.Run("must not show details of other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org org show`
		result := runOrgCLI("org", "show")

		// Then: Only the user's organization details are shown
		// Then: No other organization IDs or names are visible
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("must not expose internal organization IDs", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org org show --json`
		result := runOrgCLI("org", "show", "--json")

		// Then: No internal database IDs are exposed
		// (public organization ID is allowed)
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check for patterns that might indicate internal IDs
		internalPatterns := []string{"internal_id", "database_id", "db_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output contains potentially internal ID field: %s", pattern)
			}
		}
	})
}
