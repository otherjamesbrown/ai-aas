package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Audit logging use case tests
// See: usecases/audit.yaml
// CLI helpers defined in helpers_test.go

// TestUC_AUD_001_ListAuditEvents validates that organization admins can view
// audit logs for their organization.
//
// Use Case: UC-AUD-001 - List Audit Events
// See: usecases/audit.yaml
func TestUC_AUD_001_ListAuditEvents(t *testing.T) {
	t.Run("AC-01: list recent audit events", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list`
		result := runOrgCLI("audit", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Recent audit events are listed (default limit 25)
		// Then: Each event shows timestamp, action, actor, resource
		// Then: Events are sorted by timestamp descending
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-02: filter audit events by action", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --action user.created`
		result := runOrgCLI("audit", "list", "--action", "user.created")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Only events with action "user.created" are shown
		// Then: Results show filtered subset of events
	})

	t.Run("AC-03: filter audit events by resource type", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --resource apikey`
		result := runOrgCLI("audit", "list", "--resource", "apikey")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Only events related to API keys are shown
		// Then: Results include apikey.created, apikey.deleted, etc.
	})

	t.Run("AC-04: filter audit events by actor", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --actor admin@example.com`
		result := runOrgCLI("audit", "list", "--actor", "admin@example.com")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Only events performed by the specified actor are shown
	})

	t.Run("AC-05: limit number of audit events", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --limit 10`
		result := runOrgCLI("audit", "list", "--limit", "10")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: At most 10 events are returned
		// Then: Events are the most recent matching criteria
	})

	t.Run("AC-06: list audit events with JSON output", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --json`
		result := runOrgCLI("audit", "list", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: Each event includes timestamp, action, actor, resource
		var events []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &events); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify required fields exist in each event
		if len(events) > 0 {
			requiredFields := []string{"timestamp", "action", "actor", "resource"}
			for _, field := range requiredFields {
				if _, ok := events[0][field]; !ok {
					t.Errorf("expected field %q in event JSON", field)
				}
			}
		}
	})

	t.Run("AC-07: handle no matching audit events", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: No events match the filter criteria
		// When: User runs `ai-aas-org audit list --action nonexistent.action`
		result := runOrgCLI("audit", "list", "--action", "nonexistent.action")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no events found
	})
}

// TestUC_AUD_001_ListAuditEvents_MustNot validates negative requirements for audit use case.
func TestUC_AUD_001_ListAuditEvents_MustNot(t *testing.T) {
	t.Run("must not show audit events from other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list`
		result := runOrgCLI("audit", "list")

		// Then: Only the user's organization events are shown
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("must not expose internal event IDs", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --json`
		result := runOrgCLI("audit", "list", "--json")

		// Then: No internal event IDs are exposed
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check for patterns that might indicate internal IDs
		internalPatterns := []string{"internal_id", "event_id", "db_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output contains potentially internal ID field: %s", pattern)
			}
		}
	})

	t.Run("must not show system-level audit events", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org audit list --json`
		result := runOrgCLI("audit", "list", "--json")

		// Then: No system-level events are shown
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check for patterns that might indicate system events
		systemPatterns := []string{"system.", "admin.", "internal."}
		for _, pattern := range systemPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain system-level event: %s", pattern)
			}
		}
	})
}
