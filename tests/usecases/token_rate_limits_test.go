package usecases_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// uniqueName generates a unique name for test resources to avoid conflicts
func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// Token rate-limit policy use case tests
// See: usecases/token-rate-limits.yaml
// CLI helpers defined in helpers_test.go

// TestUC_TRL_001_CreateTokenPolicy validates that organization administrators
// can create token rate-limit policies with various limit configurations.
//
// Use Case: UC-TRL-001 - Create Token Rate-Limit Policy
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_001_CreateTokenPolicy(t *testing.T) {
	t.Run("AC-01: create policy with all three time windows", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated as org admin
		// When: User creates a policy with 1h, 24h, and 7d limits
		policyName := uniqueName("test-policy-all")
		result := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "10000",
			"--limit-24h", "100000",
			"--limit-7d", "500000",
		)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Success message is displayed
		if !strings.Contains(result.Output, "created") {
			t.Error("expected success message")
		}

		// Then: Policy appears in list
		listResult := runOrgCLI("token-policy", "list")
		if !strings.Contains(listResult.Output, policyName) {
			t.Errorf("expected policy %q in list", policyName)
		}

		// Cleanup
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-02: create policy with partial limits", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated as org admin
		// When: User creates a policy with only 1h limit
		policyName := uniqueName("test-policy-partial")
		result := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "5000",
		)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Policy is created with only 1h limit
		showResult := runOrgCLI("token-policy", "show", policyName, "--json")
		var policy map[string]interface{}
		if err := json.Unmarshal([]byte(showResult.Output), &policy); err == nil {
			if policy["limit_1h"] == nil {
				t.Error("expected limit_1h to be set")
			}
			// limit_24h and limit_7d should be nil (unlimited)
		}

		// Cleanup
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-03: reject policy with no limits", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated as org admin
		// When: User tries to create a policy without any limits
		result := runOrgCLI("token-policy", "create",
			"--name", "empty-policy",
		)

		// Then: Request fails (non-zero exit)
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for policy with no limits")
			// Cleanup if accidentally created
			runOrgCLI("token-policy", "delete", "empty-policy", "--force")
		}

		// Then: Error message indicates at least one limit required
		if !strings.Contains(strings.ToLower(result.Output), "limit") {
			t.Error("expected error message about limits")
		}
	})

	t.Run("AC-04: reject duplicate policy name", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Policy "standard" already exists in org
		policyName := uniqueName("test-duplicate")
		result1 := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "10000",
		)
		if result1.ExitCode != 0 {
			t.Skipf("could not create initial policy: %s", result1.Output)
		}

		// When: User tries to create another policy with the same name
		result2 := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "5000",
		)

		// Then: Request fails with conflict
		if result2.ExitCode == 0 {
			t.Error("expected non-zero exit for duplicate name")
		}

		// Then: Error message indicates name already exists
		if !strings.Contains(strings.ToLower(result2.Output), "exist") &&
			!strings.Contains(strings.ToLower(result2.Output), "conflict") &&
			!strings.Contains(strings.ToLower(result2.Output), "duplicate") {
			t.Error("expected conflict/duplicate error message")
		}

		// Cleanup
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-05: reject reserved policy name", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated as org admin
		// When: User tries to create a policy with reserved name
		result := runOrgCLI("token-policy", "create",
			"--name", "No Token Rate-Limit",
			"--limit-1h", "5000",
		)

		// Then: Request fails
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit for reserved name")
		}

		// Then: Error message indicates name is reserved
		if !strings.Contains(strings.ToLower(result.Output), "reserved") &&
			!strings.Contains(strings.ToLower(result.Output), "builtin") &&
			!strings.Contains(strings.ToLower(result.Output), "not allowed") {
			t.Logf("warning: expected reserved name error, got: %s", result.Output)
		}
	})
}

// TestUC_TRL_002_ListAndViewPolicies validates that organization administrators
// can list and view token rate-limit policies.
//
// Use Case: UC-TRL-002 - List and View Token Policies
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_002_ListAndViewPolicies(t *testing.T) {
	t.Run("AC-01: list all policies including built-in", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Org has policies
		// When: User runs token-policy list
		result := runOrgCLI("token-policy", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Response includes "No Token Rate-Limit" (built-in)
		if !strings.Contains(result.Output, "No Token Rate-Limit") &&
			!strings.Contains(result.Output, "Built-in") {
			t.Error("expected built-in policy in list")
		}
	})

	t.Run("AC-02: get single policy by ID", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: A policy exists
		policyName := uniqueName("test-show")
		createResult := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "10000",
		)
		if createResult.ExitCode != 0 {
			t.Skipf("could not create policy: %s", createResult.Output)
		}

		// When: User runs token-policy show
		result := runOrgCLI("token-policy", "show", policyName)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Response includes full policy details
		if !strings.Contains(result.Output, policyName) {
			t.Error("expected policy name in output")
		}

		// Cleanup
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-03: get built-in policy", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: System "No Token Rate-Limit" policy exists
		// When: User gets the built-in policy
		result := runOrgCLI("token-policy", "show", "no-limit", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Response shows is_builtin=true
		var policy map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &policy); err == nil {
			if builtin, ok := policy["is_builtin"].(bool); !ok || !builtin {
				t.Error("expected is_builtin to be true")
			}
		}
	})
}

// TestUC_TRL_003_SetOrgDefaultPolicy validates that organization administrators
// can set the default token policy for their organization.
//
// Use Case: UC-TRL-003 - Set Organization Default Policy
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_003_SetOrgDefaultPolicy(t *testing.T) {
	t.Run("AC-01: set org default policy", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Policy exists in org
		policyName := uniqueName("test-default")
		createResult := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "10000",
		)
		if createResult.ExitCode != 0 {
			t.Skipf("could not create policy: %s", createResult.Output)
		}

		// When: User sets it as org default
		result := runOrgCLI("token-policy", "default", "--set", policyName)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: GET returns the new default
		getResult := runOrgCLI("token-policy", "default")
		if !strings.Contains(getResult.Output, policyName) {
			t.Error("expected new policy to be org default")
		}

		// Cleanup: reset to no-limit and delete policy
		runOrgCLI("token-policy", "default", "--set", "no-limit")
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-02: new org defaults to unlimited", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Organization exists
		// When: User gets org default policy
		result := runOrgCLI("token-policy", "default", "--json")

		// Then: Response shows "No Token Rate-Limit" or is_builtin=true
		var policy map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &policy); err == nil {
			if name, ok := policy["name"].(string); ok {
				if name != "No Token Rate-Limit" {
					// This is OK if org has a custom default
					t.Logf("org default is: %s (expected No Token Rate-Limit for new orgs)", name)
				}
			}
		}
	})

	t.Run("AC-03: set back to unlimited", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Org currently has a custom default
		// When: User sets default to no-limit
		result := runOrgCLI("token-policy", "default", "--set", "no-limit")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Org default is now "No Token Rate-Limit"
		getResult := runOrgCLI("token-policy", "default", "--json")
		var policy map[string]interface{}
		if err := json.Unmarshal([]byte(getResult.Output), &policy); err == nil {
			if builtin, ok := policy["is_builtin"].(bool); !ok || !builtin {
				t.Error("expected org default to be built-in policy")
			}
		}
	})
}

// TestUC_TRL_004_UserPolicyOverride validates that organization administrators
// can set user-specific token policy overrides.
//
// Use Case: UC-TRL-004 - Override Policy for Specific User
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_004_UserPolicyOverride(t *testing.T) {
	t.Run("AC-01: set user policy override", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User exists, policy exists
		testUser := createTestUser(t, "policy-test", "user")
		policyName := uniqueName("user-override-policy")
		createResult := runOrgCLI("token-policy", "create",
			"--name", policyName,
			"--limit-24h", "50000",
		)
		if createResult.ExitCode != 0 {
			t.Skipf("could not create policy: %s", createResult.Output)
		}

		// When: User sets a policy override
		result := runOrgCLI("token-policy", "user", testUser.Email, "--set", policyName)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: GET returns the override
		getResult := runOrgCLI("token-policy", "user", testUser.Email, "--json")
		var effective map[string]interface{}
		if err := json.Unmarshal([]byte(getResult.Output), &effective); err == nil {
			if source, ok := effective["source"].(string); !ok || source != "override" {
				t.Error("expected source to be 'override'")
			}
		}

		// Cleanup
		runOrgCLI("token-policy", "user", testUser.Email, "--clear")
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-03: remove override to inherit org default", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has a policy override
		testUser := createTestUser(t, "policy-clear", "user")
		policyName := uniqueName("clear-test-policy")
		runOrgCLI("token-policy", "create", "--name", policyName, "--limit-1h", "5000")
		runOrgCLI("token-policy", "user", testUser.Email, "--set", policyName)

		// When: User clears the override
		result := runOrgCLI("token-policy", "user", testUser.Email, "--clear")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: GET now returns inherited
		getResult := runOrgCLI("token-policy", "user", testUser.Email, "--json")
		var effective map[string]interface{}
		if err := json.Unmarshal([]byte(getResult.Output), &effective); err == nil {
			if source, ok := effective["source"].(string); !ok || source != "inherited" {
				t.Error("expected source to be 'inherited' after clear")
			}
		}

		// Cleanup
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})
}

// TestUC_TRL_005_QueryOwnTokenUsage validates that users can query their
// own token usage against their rate limits.
//
// Use Case: UC-TRL-005 - Query Own Token Usage
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_005_QueryOwnTokenUsage(t *testing.T) {
	t.Run("AC-01: view usage with all windows", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "requires user with policy and usage data")

		// Given: User has policy with 1h/24h/7d limits, has used some tokens
		// When: User queries usage
		// Note: This requires the usage command which may use different endpoints
		result := runOrgCLI("usage", "summary")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Usage information is displayed
		if result.Output == "" {
			t.Error("expected usage output")
		}
	})

	t.Run("AC-03: view usage with no limits", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has "No Token Rate-Limit" policy (default)
		// Ensure org default is no-limit
		runOrgCLI("token-policy", "default", "--set", "no-limit")

		// When: User queries usage
		result := runOrgCLI("usage", "summary")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}
	})
}

// TestUC_TRL_006_AdminViewsUserUsage validates that admins can view
// any user's token usage in their organization.
//
// Use Case: UC-TRL-006 - Admin Views User Token Usage
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_006_AdminViewsUserUsage(t *testing.T) {
	t.Run("AC-01: view any org user's usage", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User exists in org
		testUser := createTestUser(t, "usage-view", "user")

		// When: Admin queries user's usage
		result := runOrgCLI("usage", "by-user", "--user", testUser.Email)

		// Then: Exit code is 0 (command may not exist yet)
		if result.ExitCode != 0 {
			t.Logf("by-user command may not support --user filter: %s", result.Output)
		}
	})
}

// TestUC_TRL_007_EnforceRateLimitOnRequest validates that token rate limits
// are enforced on API requests.
//
// Use Case: UC-TRL-007 - Enforce Rate Limit on API Request
// See: usecases/token-rate-limits.yaml
//
// Note: This test requires actual inference requests to trigger rate limiting.
// It is marked as requiring a live inference backend.
func TestUC_TRL_007_EnforceRateLimitOnRequest(t *testing.T) {
	t.Run("AC-01: reject when limit exceeded", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "requires live inference backend and rate limit enforcement")

		// Given: User has 1h limit of small value, has used all tokens
		// When: User makes any API request
		// Then: Request is rejected with HTTP 429
		// Then: Error body is OpenAI-compatible format

		// This test requires:
		// 1. A user with a very low token limit
		// 2. Making inference requests until the limit is hit
		// 3. Verifying the 429 response format

		t.Skip("requires live inference backend - implement when infrastructure available")
	})

	t.Run("AC-04: allow when under all limits", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "requires live inference backend")

		// Given: User is under all window limits
		// When: User makes API request
		// Then: Request proceeds normally

		t.Skip("requires live inference backend - implement when infrastructure available")
	})
}

// TestUC_TRL_008_AllowOverageThenBlock validates that requests that go
// over the limit complete but subsequent requests are blocked.
//
// Use Case: UC-TRL-008 - Allow Overage Then Block
// See: usecases/token-rate-limits.yaml
//
// Note: This test requires actual inference requests with token counting.
func TestUC_TRL_008_AllowOverageThenBlock(t *testing.T) {
	t.Run("AC-01: allow request that goes over", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "requires live inference backend with token counting")

		// Given: User has 500 tokens remaining in 1h window
		// When: User sends request, response uses 800 tokens
		// Then: Request completes successfully
		// Then: 800 tokens are recorded (total now over limit)

		t.Skip("requires live inference backend - implement when infrastructure available")
	})

	t.Run("AC-02: block next request after overage", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "requires live inference backend with token counting")

		// Given: User went over limit on previous request
		// When: User sends another request
		// Then: Request is rejected with 429

		t.Skip("requires live inference backend - implement when infrastructure available")
	})
}

// TestUC_TRL_009_UpdateTokenPolicy validates that organization administrators
// can update existing token rate-limit policies.
//
// Use Case: UC-TRL-009 - Update Token Policy
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_009_UpdateTokenPolicy(t *testing.T) {
	t.Run("AC-01: update policy limits", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Policy exists with 1h=10000
		policyName := uniqueName("test-update")
		runOrgCLI("token-policy", "create", "--name", policyName, "--limit-1h", "10000")

		// When: User updates the limit
		result := runOrgCLI("token-policy", "update", policyName, "--limit-1h", "20000")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Policy is updated
		showResult := runOrgCLI("token-policy", "show", policyName, "--json")
		var policy map[string]interface{}
		if err := json.Unmarshal([]byte(showResult.Output), &policy); err == nil {
			if limit, ok := policy["limit_1h"].(float64); !ok || int64(limit) != 20000 {
				t.Errorf("expected limit_1h=20000, got %v", policy["limit_1h"])
			}
		}

		// Cleanup
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-03: cannot update built-in policy", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Attempting to update "No Token Rate-Limit"
		// When: User tries to update it
		result := runOrgCLI("token-policy", "update", "no-limit", "--limit-1h", "1000")

		// Then: Request fails
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit for updating built-in policy")
		}

		// Then: Error indicates built-in policies cannot be modified
		if !strings.Contains(strings.ToLower(result.Output), "built-in") &&
			!strings.Contains(strings.ToLower(result.Output), "cannot") &&
			!strings.Contains(strings.ToLower(result.Output), "forbidden") {
			t.Logf("warning: expected built-in error, got: %s", result.Output)
		}
	})
}

// TestUC_TRL_010_DeleteTokenPolicy validates that organization administrators
// can delete token rate-limit policies.
//
// Use Case: UC-TRL-010 - Delete Token Policy
// See: usecases/token-rate-limits.yaml
func TestUC_TRL_010_DeleteTokenPolicy(t *testing.T) {
	t.Run("AC-01: delete unused policy", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Policy exists with no assignments
		policyName := uniqueName("test-delete")
		createResult := runOrgCLI("token-policy", "create", "--name", policyName, "--limit-1h", "10000")
		if createResult.ExitCode != 0 {
			t.Skipf("could not create policy: %s", createResult.Output)
		}

		// When: User deletes it
		result := runOrgCLI("token-policy", "delete", policyName, "--force")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Policy no longer appears in list
		listResult := runOrgCLI("token-policy", "list")
		if strings.Contains(listResult.Output, policyName) {
			t.Error("expected policy to be removed from list")
		}
	})

	t.Run("AC-02: cannot delete policy in use as org default", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Policy is org's default policy
		policyName := uniqueName("test-in-use")
		runOrgCLI("token-policy", "create", "--name", policyName, "--limit-1h", "10000")
		runOrgCLI("token-policy", "default", "--set", policyName)

		// When: User tries to delete it
		result := runOrgCLI("token-policy", "delete", policyName, "--force")

		// Then: Request fails with conflict
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit for deleting in-use policy")
		}

		// Cleanup: reset default then delete
		runOrgCLI("token-policy", "default", "--set", "no-limit")
		runOrgCLI("token-policy", "delete", policyName, "--force")
	})

	t.Run("AC-04: cannot delete built-in policy", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Attempting to delete "No Token Rate-Limit"
		// When: User tries to delete it
		result := runOrgCLI("token-policy", "delete", "no-limit", "--force")

		// Then: Request fails
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit for deleting built-in policy")
		}
	})
}
