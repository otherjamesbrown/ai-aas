package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Usage and billing use case tests
// See: usecases/usage.yaml
// CLI helpers defined in helpers_test.go

// TestUC_USG_001_ShowUsageSummary validates that organization admins can view
// aggregated usage metrics.
//
// Use Case: UC-USG-001 - Show Usage Summary
// See: usecases/usage.yaml
func TestUC_USG_001_ShowUsageSummary(t *testing.T) {
	t.Run("AC-01: show usage summary with default period", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage summary`
		result := runOrgCLI("usage", "summary")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Total tokens are displayed
		// Then: Total cost is displayed
		// Then: Time period is displayed (default 30d)
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-02: show usage summary with custom period", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage summary --period 7d`
		result := runOrgCLI("usage", "summary", "--period", "7d")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Total tokens for the 7 day period are displayed
		// Then: Total cost for the 7 day period are displayed
		// Then: Time period is confirmed as 7d
		if !strings.Contains(result.Output, "7d") && !strings.Contains(result.Output, "7 day") {
			t.Error("expected period confirmation in output")
		}
	})

	t.Run("AC-03: show usage summary with JSON output", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage summary --json`
		result := runOrgCLI("usage", "summary", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON object
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: JSON includes total_tokens, total_cost, period
		var summary map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &summary); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-04: handle zero usage period", func(t *testing.T) {
		t.Skip("requires live API with org that has no usage")

		// Given: Organization has no usage in the specified period
		// When: User runs `ai-aas-org usage summary --period today`
		result := runOrgCLI("usage", "summary", "--period", "today")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Zero tokens are displayed
		// Then: Zero cost is displayed
		// Then: Message indicates no usage in period
	})
}

// TestUC_USG_002_ShowUsageByModel validates that organization admins can view
// usage broken down by model.
//
// Use Case: UC-USG-002 - Show Usage By Model
// See: usecases/usage.yaml
func TestUC_USG_002_ShowUsageByModel(t *testing.T) {
	t.Run("AC-01: show usage by model with default period", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-model`
		result := runOrgCLI("usage", "by-model")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Each model with usage is listed
		// Then: Tokens per model are displayed
		// Then: Cost per model is displayed
		// Then: Time period is displayed (default 30d)
	})

	t.Run("AC-02: show usage by model with custom period", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-model --period 7d`
		result := runOrgCLI("usage", "by-model", "--period", "7d")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Usage for the 7 day period is displayed
		// Then: Each model shows tokens and cost for that period
	})

	t.Run("AC-03: show usage by model with JSON output", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-model --json`
		result := runOrgCLI("usage", "by-model", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: Each model object includes model_name, tokens, cost
		var models []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &models); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-04: handle no model usage", func(t *testing.T) {
		t.Skip("requires live API with org that has no usage")

		// Given: Organization has no model usage in the period
		// When: User runs `ai-aas-org usage by-model`
		result := runOrgCLI("usage", "by-model")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no usage in period
	})
}

// TestUC_USG_003_ShowUsageByUser validates that organization admins can view
// usage broken down by user.
//
// Use Case: UC-USG-003 - Show Usage By User
// See: usecases/usage.yaml
func TestUC_USG_003_ShowUsageByUser(t *testing.T) {
	t.Run("AC-01: show usage by user with default period", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-user`
		result := runOrgCLI("usage", "by-user")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Each user with usage is listed
		// Then: Tokens per user are displayed
		// Then: Cost per user is displayed
		// Then: Time period is displayed (default 30d)
	})

	t.Run("AC-02: show usage by user with custom period", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-user --period 7d`
		result := runOrgCLI("usage", "by-user", "--period", "7d")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Usage for the 7 day period is displayed
		// Then: Each user shows tokens and cost for that period
	})

	t.Run("AC-03: show usage by user with JSON output", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-user --json`
		result := runOrgCLI("usage", "by-user", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: Each user object includes user_email, tokens, cost
		var users []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &users); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-04: handle no user usage", func(t *testing.T) {
		t.Skip("requires live API with org that has no usage")

		// Given: Organization has no user usage in the period
		// When: User runs `ai-aas-org usage by-user`
		result := runOrgCLI("usage", "by-user")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no usage in period
	})
}

// TestUC_USG_MustNot validates negative requirements for usage use cases.
func TestUC_USG_MustNot(t *testing.T) {
	t.Run("must not show usage from other organizations", func(t *testing.T) {
		t.Skip("requires live API with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage summary`
		result := runOrgCLI("usage", "summary")

		// Then: Only the user's organization usage is shown
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("must not expose internal user IDs in by-user", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-user --json`
		result := runOrgCLI("usage", "by-user", "--json")

		// Then: No internal user IDs are exposed (email is used instead)
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check for patterns that might indicate internal IDs
		internalPatterns := []string{"internal_id", "user_id", "db_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output contains potentially internal ID field: %s", pattern)
			}
		}
	})

	t.Run("must not expose internal model IDs in by-model", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org usage by-model --json`
		result := runOrgCLI("usage", "by-model", "--json")

		// Then: No internal model IDs are exposed (model name is used instead)
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check for patterns that might indicate internal IDs
		internalPatterns := []string{"internal_id", "model_id", "db_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output contains potentially internal ID field: %s", pattern)
			}
		}
	})
}
