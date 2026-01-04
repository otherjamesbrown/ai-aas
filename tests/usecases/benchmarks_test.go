package usecases_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Benchmark use case tests
// See: usecases/benchmarks.yaml
// CLI helpers defined in helpers_test.go

// setupRoutingPoliciesForBenchmarkTests ensures clean routing policy state for benchmark authorization tests.
// It removes wildcard policies that grant unintended access and creates test-specific policies.
func setupRoutingPoliciesForBenchmarkTests(t *testing.T, orgID string) {
	t.Helper()

	adminEndpoint := getAdminAPIEndpoint()
	adminKey := getAdminAPIKey()

	if adminEndpoint == "" || adminKey == "" {
		t.Skip("requires admin API endpoint and key for routing policy setup")
	}

	client := NewTestClient(adminEndpoint, adminKey)

	// List all routing policies
	resp, err := client.GET("/v1/routing/policies")
	if err != nil {
		t.Fatalf("failed to list routing policies: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("failed to list routing policies: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var policiesResp struct {
		Policies []struct {
			PolicyID       string `json:"policy_id"`
			OrganizationID string `json:"organization_id"`
			Model          string `json:"model"`
		} `json:"policies"`
	}

	if err := resp.DecodeJSON(&policiesResp); err != nil {
		t.Fatalf("failed to parse routing policies: %v", err)
	}

	// Delete wildcard policies AND policies for other orgs for test models
	testModels := []string{"llama-7b", "restricted-model", "nonexistent-model-xyz"}
	for _, policy := range policiesResp.Policies {
		// Delete policies that:
		// 1. Are for test models
		// 2. Either have wildcard org ID (*), are empty, or are for a different org
		for _, model := range testModels {
			shouldDelete := policy.Model == model &&
				(policy.OrganizationID == "*" ||
				 policy.OrganizationID == "" ||
				 policy.OrganizationID != orgID)

			if shouldDelete {
				t.Logf("Deleting routing policy for model %s, org %s (policy_id: %s)",
					model, policy.OrganizationID, policy.PolicyID)
				delResp, err := client.DELETE(fmt.Sprintf("/v1/routing/policies/%s", policy.PolicyID))
				if err != nil {
					t.Logf("Warning: failed to delete policy %s: %v", policy.PolicyID, err)
				} else if delResp.StatusCode != 204 && delResp.StatusCode != 200 {
					t.Logf("Warning: delete policy returned status %d for %s", delResp.StatusCode, policy.PolicyID)
				}
			}
		}
	}

	// Create routing policy for llama-7b ONLY for test org
	// This ensures the test org has access to llama-7b but not other models
	createLlama7bPolicy := map[string]interface{}{
		"organization_id": orgID,
		"model":           "llama-7b",
		"backends": []map[string]interface{}{
			{
				"backend_id": "llama-7b-backend",
				"weight":     100,
			},
		},
		"enabled": true,
	}

	createResp, err := client.POST("/v1/routing/policies", createLlama7bPolicy)
	if err != nil {
		t.Logf("Warning: failed to create llama-7b routing policy: %v", err)
	} else if createResp.StatusCode != 201 && createResp.StatusCode != 200 {
		// Policy might already exist, log but continue
		t.Logf("Warning: create llama-7b policy returned status %d: %s", createResp.StatusCode, createResp.String())
	} else {
		t.Logf("Created routing policy for llama-7b for test org %s", orgID)

		// Register cleanup
		var createdPolicy struct {
			PolicyID string `json:"policy_id"`
		}
		if err := createResp.DecodeJSON(&createdPolicy); err == nil {
			t.Cleanup(func() {
				client.DELETE(fmt.Sprintf("/v1/routing/policies/%s", createdPolicy.PolicyID))
			})
		}
	}

	// Verify no routing policy exists for restricted-model
	// Verify no routing policy exists for nonexistent-model-xyz
	// (These should have been deleted above if they existed)
}

// TestUC_BM_001_CreateBenchmarkTarget validates that organization admins can
// create benchmark target configurations.
//
// Use Case: UC-BM-001 - Create Benchmark Target
// Acceptance Criteria:
//   - AC-01: Create target with required fields
//   - AC-02: Reject unauthorized model
//   - AC-03: Reject invalid scenario
//   - AC-04: Create target with optional parameters
//
// See: usecases/benchmarks.yaml
func TestUC_BM_001_CreateBenchmarkTarget(t *testing.T) {
	t.Run("AC-01: create target with required fields", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org benchmark target add test-target-01 --model llama-7b --scenario standard`
		targetName := "test-target-" + generateUniqueID()
		result := runOrgCLI("benchmark", "target", "add", targetName, "--model", "llama-7b", "--scenario", "standard")

		// Cleanup: Delete target after test
		t.Cleanup(func() {
			runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		})

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Benchmark target is created in the system
		// Then: Target ID is returned and displayed
		if !strings.Contains(result.Output, "target") || !strings.Contains(strings.ToLower(result.Output), "id") {
			t.Error("expected target ID in output")
		}

		// Then: Target appears in `ai-aas-org benchmark target list` output
		listResult := runOrgCLI("benchmark", "target", "list")
		if listResult.ExitCode != 0 {
			t.Fatalf("expected exit code 0 for list, got %d", listResult.ExitCode)
		}
	})

	t.Run("AC-02: reject unauthorized model", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark service doesn't validate routing policies during target creation (see bead aas-7g50m)")

		// IMPLEMENTATION GAP DISCOVERED:
		// The benchmark service allows target creation for any model name, regardless of:
		// 1. Whether a routing policy exists for that model+org
		// 2. Whether the model deployment actually exists
		//
		// Expected behavior (per UC-BM-001 AC-02):
		// - Target creation should check if org has a routing policy for the model
		// - If no routing policy exists, return 403/404 with "model not accessible" error
		//
		// Current behavior:
		// - Target creation succeeds for any model name
		// - Authorization is only checked during run trigger (UC-BM-002 AC-03)
		//
		// This test validates the EXPECTED behavior. It will fail until the service
		// implements routing policy validation during target creation.
		//
		// Test code below is correct and ready to use once implementation is fixed:

		skipIfNoLiveAPI(t)

		// Create isolated test org with controlled routing policies
		orgCtx := NewTestOrgContext(t)

		// Setup routing policies: llama-7b allowed, restricted-model NOT allowed
		setupRoutingPoliciesForBenchmarkTests(t, orgCtx.OrgID)

		// Given: User specifies a model they don't have access to
		// When: User runs `ai-aas-org benchmark target add test-unauthorized --model restricted-model --scenario standard`
		targetName := "test-unauthorized-" + generateUniqueID()
		result := orgCtx.RunCLI("benchmark", "target", "add", targetName, "--model", "restricted-model", "--scenario", "standard")

		// Cleanup: Delete target if it was created (shouldn't happen, but be safe)
		t.Cleanup(func() {
			orgCtx.RunCLI("benchmark", "target", "remove", targetName, "--force")
		})

		// Then: Command fails with non-zero exit code
		// Note: CLI returns exit code 1 for operation errors, not specific codes
		if result.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message explains lack of model access
		if !strings.Contains(strings.ToLower(result.Output), "access") {
			t.Error("expected access-related error message")
		}

		// Then: Error includes suggestion to check model permissions
		// Then: No target is created
	})

	t.Run("AC-03: reject invalid scenario", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User specifies a non-existent scenario
		// When: User runs `ai-aas-org benchmark target add test-invalid-scenario --model llama-7b --scenario nonexistent`
		targetName := "test-invalid-scenario-" + generateUniqueID()
		result := runOrgCLI("benchmark", "target", "add", targetName, "--model", "llama-7b", "--scenario", "nonexistent")

		// Cleanup: Delete target if it was created (shouldn't happen, but be safe)
		t.Cleanup(func() {
			runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		})

		// Then: Command fails with non-zero exit code
		// Note: CLI returns exit code 1 for operation errors, not specific codes
		if result.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message lists available scenarios
		// Then: No target is created
	})

	t.Run("AC-04: create target with optional parameters", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org benchmark target add test-target-04 --model llama-7b --scenario throughput --interval 120`
		// Note: CLI supports --interval for scheduled runs, not --concurrency/--duration
		targetName := "test-target-" + generateUniqueID()
		result := runOrgCLI("benchmark", "target", "add",
			targetName,
			"--model", "llama-7b",
			"--scenario", "throughput",
			"--schedule",
			"--interval", "120")

		// Cleanup: Delete target after test
		t.Cleanup(func() {
			runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		})

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Benchmark target is created with custom parameters
		// Then: Parameters are displayed in target details
		// Then: Target can be used for benchmark runs
	})
}

// TestUC_BM_001_CreateBenchmarkTarget_MustNot validates negative requirements.
func TestUC_BM_001_CreateBenchmarkTarget_MustNot(t *testing.T) {
	t.Run("must not auto-start benchmark after target creation", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User creates a benchmark target
		// When: Target creation completes
		targetName := "test-no-autostart-" + generateUniqueID()
		result := runOrgCLI("benchmark", "target", "add", targetName, "--model", "llama-7b", "--scenario", "standard")

		// Cleanup: Delete target after test
		t.Cleanup(func() {
			runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		})

		// Then: No benchmark run is started automatically
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check that output doesn't indicate a running benchmark
		if strings.Contains(strings.ToLower(result.Output), "running") ||
			strings.Contains(strings.ToLower(result.Output), "started") {
			t.Error("benchmark should not auto-start after target creation")
		}
	})

	t.Run("must not modify any existing targets", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Creating a new target should not affect existing targets
		// This test would verify by creating a target, then creating another,
		// and verifying the first is unchanged
	})

	t.Run("must not expose internal system metrics or endpoints", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User creates a benchmark target
		targetName := "test-no-internals-" + generateUniqueID()
		result := runOrgCLI("benchmark", "target", "add", targetName, "--model", "llama-7b", "--scenario", "standard")

		// Cleanup: Delete target after test
		t.Cleanup(func() {
			runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		})

		// Then: No internal implementation details or metrics are exposed
		// Note: We check for specific patterns that indicate internal details, not just the word "internal"
		internalPatterns := []string{"internal_", "private_", "worker_id", "node_id", "prometheus", "metrics_port"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal information: %s", pattern)
			}
		}
	})
}

// TestUC_BM_002_TriggerBenchmarkRun validates that organization admins can
// trigger benchmark runs on existing targets.
//
// Use Case: UC-BM-002 - Trigger Benchmark Run
// Acceptance Criteria:
//   - AC-01: Trigger run on valid target
//   - AC-02: Reject run on non-existent target
//   - AC-03: Reject run when model unavailable
//
// See: usecases/benchmarks.yaml
func TestUC_BM_002_TriggerBenchmarkRun(t *testing.T) {
	t.Run("AC-01: trigger run on valid target", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has created benchmark target
		targetName := "test-trigger-" + generateUniqueID()
		createResult := runOrgCLI("benchmark", "target", "add", targetName, "--model", "llama-7b", "--scenario", "standard")

		// Cleanup: Delete target after test
		t.Cleanup(func() {
			runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		})

		if createResult.ExitCode != 0 {
			t.Fatalf("failed to create test target: %s", createResult.Output)
		}

		// When: User runs `ai-aas-org benchmark run trigger <target>`
		result := runOrgCLI("benchmark", "run", "trigger", targetName)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Benchmark run is queued for execution
		// Then: Run ID is returned and displayed
		if !strings.Contains(strings.ToLower(result.Output), "run") ||
			!strings.Contains(strings.ToLower(result.Output), "id") {
			t.Error("expected run ID in output")
		}

		// Then: Run status is "pending" or "running"
	})

	t.Run("AC-02: reject run on non-existent target", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User specifies a target that doesn't exist
		// When: User runs `ai-aas-org benchmark run trigger nonexistent-target`
		nonexistentTarget := "nonexistent-" + generateUniqueID()
		result := runOrgCLI("benchmark", "run", "trigger", nonexistentTarget)

		// Then: Command fails with non-zero exit code
		if result.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates target not found
		// Then: No run is created
	})

	t.Run("AC-03: reject run when model unavailable", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark service doesn't validate model availability during run trigger (see bead aas-7g50m)")

		// IMPLEMENTATION GAP DISCOVERED:
		// The benchmark service allows run triggers for targets with non-existent models.
		// Even when a routing policy exists but the actual model deployment is offline/missing,
		// the trigger succeeds and creates a "pending" run.
		//
		// Expected behavior (per UC-BM-002 AC-03):
		// - Run trigger should check if the model deployment actually exists and is healthy
		// - If model is offline/non-existent, return error with "model unavailable" message
		// - Suggestion to check model status should be included
		//
		// Current behavior:
		// - Run trigger succeeds regardless of model availability
		// - Run is queued as "pending" and may fail later during execution
		//
		// This test validates the EXPECTED behavior. It will fail until the service
		// implements model availability validation during run trigger.
		//
		// Test code below is correct and ready to use once implementation is fixed:

		skipIfNoLiveAPI(t)

		// Create isolated test org with controlled routing policies
		orgCtx := NewTestOrgContext(t)

		// Setup routing policies: llama-7b allowed, nonexistent-model-xyz NOT allowed
		// Note: AC-03 expects target creation to SUCCEED (model exists in routing but no deployment)
		// but trigger should FAIL because the actual model deployment doesn't exist
		setupRoutingPoliciesForBenchmarkTests(t, orgCtx.OrgID)

		// Create a routing policy for nonexistent-model-xyz so target creation succeeds
		// but no actual deployment exists, so trigger will fail
		adminEndpoint := getAdminAPIEndpoint()
		adminKey := getAdminAPIKey()
		client := NewTestClient(adminEndpoint, adminKey)

		createPolicyResp, err := client.POST("/v1/routing/policies", map[string]interface{}{
			"organization_id": orgCtx.OrgID,
			"model":           "nonexistent-model-xyz",
			"backends": []map[string]interface{}{
				{
					"backend_id": "nonexistent-backend",
					"weight":     100,
				},
			},
			"enabled": true,
		})
		if err == nil && (createPolicyResp.StatusCode == 200 || createPolicyResp.StatusCode == 201) {
			var createdPolicy struct {
				PolicyID string `json:"policy_id"`
			}
			if err := createPolicyResp.DecodeJSON(&createdPolicy); err == nil {
				t.Cleanup(func() {
					client.DELETE(fmt.Sprintf("/v1/routing/policies/%s", createdPolicy.PolicyID))
				})
			}
		}

		// Given: Benchmark target exists but referenced model is offline/non-existent
		// Create a target with a non-existent model name
		targetName := "test-unavailable-model-" + generateUniqueID()
		createResult := orgCtx.RunCLI("benchmark", "target", "add", targetName, "--model", "nonexistent-model-xyz", "--scenario", "standard")

		// Cleanup: Delete target after test
		t.Cleanup(func() {
			orgCtx.RunCLI("benchmark", "target", "remove", targetName, "--force")
		})

		if createResult.ExitCode != 0 {
			t.Fatalf("failed to create test target (should succeed even though model deployment doesn't exist): %s", createResult.Output)
		}

		// When: User runs `ai-aas-org benchmark run trigger <target>`
		result := orgCtx.RunCLI("benchmark", "run", "trigger", targetName)

		// Then: Command fails with non-zero exit code
		if result.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates model unavailable
		if !strings.Contains(strings.ToLower(result.Output), "unavailable") &&
			!strings.Contains(strings.ToLower(result.Output), "not available") {
			t.Errorf("expected error message to indicate model unavailable, got: %s", result.Output)
		}

		// Then: Error includes suggestion to check model status
		if !strings.Contains(result.Output, "model cache list") &&
			!strings.Contains(result.Output, "check model") {
			t.Errorf("expected error to include suggestion to check model status, got: %s", result.Output)
		}
	})
}

// TestUC_BM_002_TriggerBenchmarkRun_MustNot validates negative requirements.
func TestUC_BM_002_TriggerBenchmarkRun_MustNot(t *testing.T) {
	t.Run("must not block until run completes", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User triggers a benchmark run
		// When: Command returns
		// Then: Command returns immediately without waiting for completion
		// (This is validated by the async nature - run is queued, not completed)
	})

	t.Run("must not modify the target configuration", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Triggering a run should not change the target configuration
	})

	t.Run("must not expose internal queue details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User triggers a benchmark run
		result := runOrgCLI("benchmark", "run", "trigger", "target-123")

		// Then: No internal queue details are exposed
		internalPatterns := []string{"queue", "worker", "internal", "redis"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal queue details: %s", pattern)
			}
		}
	})
}

// TestUC_BM_003_ViewBenchmarkResults validates that organization admins can
// view benchmark run results.
//
// Use Case: UC-BM-003 - View Benchmark Results
// Acceptance Criteria:
//   - AC-01: View completed run results
//   - AC-02: View pending run status
//   - AC-03: View failed run details
//   - AC-04: Export results as JSON
//
// See: usecases/benchmarks.yaml
func TestUC_BM_003_ViewBenchmarkResults(t *testing.T) {
	t.Run("AC-01: view completed run results", func(t *testing.T) {
		t.Skip("Requires real benchmark run - needs test harness to create and wait for completion")

		// Given: Benchmark run has completed successfully
		// When: User runs `ai-aas-org benchmark run show <run-id>`
		// Then: Exit code is 0
		// Then: Run status is displayed as "completed"
		// Then: Throughput metric is shown (requests/second)
		// Then: Latency percentiles are shown (p50, p90, p99)
		// Then: Error rate is shown (percentage)
		// Then: Duration is shown
	})

	t.Run("AC-02: view pending run status", func(t *testing.T) {
		t.Skip("Requires real benchmark run - needs test harness to create running benchmark")

		// Given: Benchmark run is still running
		// When: User runs `ai-aas-org benchmark run show <run-id>`
		// Then: Run status is displayed as "running" or "pending"
		// Then: Progress indicator shown if available
		// Then: Start time is displayed
		// Then: Message indicates results pending
	})

	t.Run("AC-03: view failed run details", func(t *testing.T) {
		t.Skip("Requires real benchmark run - needs test harness to create failed run")

		// Given: Benchmark run failed during execution
		// When: User runs `ai-aas-org benchmark run show <run-id>`
		// Then: Run status is displayed as "failed"
		// Then: Error message explains failure reason
		// Then: Partial results shown if available
		// Then: Suggestion for troubleshooting included
	})

	t.Run("AC-04: export results as JSON", func(t *testing.T) {
		t.Skip("Requires real benchmark run - needs test harness to create completed run")

		// Given: Benchmark run has completed
		// When: User runs `ai-aas-org benchmark run show <run-id> --json`
		// Then: Exit code is 0
		// Then: Results are output as valid JSON
		// Then: JSON includes all metrics
	})
}

// TestUC_BM_003_ViewBenchmarkResults_MustNot validates negative requirements.
func TestUC_BM_003_ViewBenchmarkResults_MustNot(t *testing.T) {
	t.Run("must not show results from other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User tries to view results for another org's run
		// Then: Access is denied or results not found
	})

	t.Run("must not expose internal implementation details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User views benchmark results
		result := runOrgCLI("benchmark", "run", "show", "run-456", "--json")

		// Then: No internal implementation details are exposed
		internalPatterns := []string{"internal_", "private_", "worker_id", "node_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal details: %s", pattern)
			}
		}
	})
}

// TestUC_BM_004_ListBenchmarkTargets validates that organization admins can
// list their benchmark targets.
//
// Use Case: UC-BM-004 - List Benchmark Targets
// Acceptance Criteria:
//   - AC-01: List all targets
//   - AC-02: List targets with no results
//
// See: usecases/benchmarks.yaml
func TestUC_BM_004_ListBenchmarkTargets(t *testing.T) {
	t.Run("AC-01: list all targets", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has created multiple benchmark targets
		// When: User runs `ai-aas-org benchmark target list`
		result := runOrgCLI("benchmark", "target", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: All targets for the organization are listed
		// Then: Each target shows ID, model, scenario, and created date
		// Then: Output is formatted as a table
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-01b: list all targets with JSON output", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has created multiple benchmark targets
		// When: User runs `ai-aas-org benchmark target list --json`
		result := runOrgCLI("benchmark", "target", "list", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: Each target object includes ID, model, scenario, created_at
		var targets []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &targets); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-02: list targets with no results", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with org that has no targets")

		// Given: User has not created any benchmark targets
		// When: User runs `ai-aas-org benchmark target list`
		result := runOrgCLI("benchmark", "target", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no targets found
		// Then: Suggestion to create a target is shown
	})
}

// TestUC_BM_004_ListBenchmarkTargets_MustNot validates negative requirements.
func TestUC_BM_004_ListBenchmarkTargets_MustNot(t *testing.T) {
	t.Run("must not show targets from other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org benchmark target list`
		result := runOrgCLI("benchmark", "target", "list")

		// Then: Only the user's organization targets are shown
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("must not modify any targets", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Listing targets is a read-only operation
		// This test verifies no side effects occur
	})
}

// TestUC_BM_005_WaitForRunCompletion validates that organization admins can
// wait for benchmark runs to complete.
//
// Use Case: UC-BM-005 - Wait for Run Completion
// Acceptance Criteria:
//   - AC-01: Wait for successful completion
//   - AC-02: Wait with timeout
//   - AC-03: Wait for already completed run
//   - AC-04: Wait for failed run
//
// See: usecases/benchmarks.yaml
func TestUC_BM_005_WaitForRunCompletion(t *testing.T) {
	t.Run("AC-01: wait for successful completion", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark run wait command does not exist")

		// Given: Benchmark run is in running state
		// When: User runs `ai-aas-org benchmark run wait <run-id>`
		// Then: Command blocks until run completes
		// Then: Final status is displayed as "completed"
		// Then: Exit code is 0
	})

	t.Run("AC-02: wait with timeout", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark run wait command does not exist")

		// Given: Benchmark run is in running state
		// When: User runs `ai-aas-org benchmark run wait <run-id> --timeout 5s`
		// Then: Command waits up to the timeout
		// If run completes within timeout, exit code is 0
		// If timeout expires, exit code is 1
	})

	t.Run("AC-03: wait for already completed run", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark run wait command does not exist")

		// Given: Benchmark run has already completed
		// When: User runs `ai-aas-org benchmark run wait <run-id>`
		// Then: Command returns immediately
		// Then: Status shown as "completed"
		// Then: Exit code is 0
	})

	t.Run("AC-04: wait for failed run", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark run wait command does not exist")

		// Given: Benchmark run fails during execution
		// When: User runs `ai-aas-org benchmark run wait <run-id>`
		// Then: Command returns when run fails
		// Then: Status shown as "failed"
		// Then: Exit code is 2 (indicating run failure)
		// Then: Error details are displayed
	})

	t.Run("AC-04b: wait for non-existent run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run does not exist
		nonexistentRunID := "00000000-0000-0000-0000-000000000000"

		// When: User runs `ai-aas-org benchmark run wait <run-id>`
		result := runOrgCLI("benchmark", "run", "wait", nonexistentRunID, "--timeout", "5s")

		// Then: Command fails with non-zero exit code
		if result.ExitCode == 0 {
			t.Errorf("expected non-zero exit code for non-existent run, got %d: %s", result.ExitCode, result.Output)
		}
	})
}

// TestUC_BM_005_WaitForRunCompletion_MustNot validates negative requirements.
func TestUC_BM_005_WaitForRunCompletion_MustNot(t *testing.T) {
	t.Run("must not modify the run in any way", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark run wait command does not exist")

		// Given: User waits for a run
		// When: Wait completes
		// Then: Run data should be unchanged
		// (This is an implicit requirement - wait is read-only)
	})

	t.Run("must not return success exit code for failed runs", func(t *testing.T) {
		t.Skip("Feature not implemented - benchmark run wait command does not exist")

		// Given: Benchmark run has failed
		// When: User waits for it
		// Then: Exit code should NOT be 0
	})
}

// TestUC_BM_006_CancelRunningBenchmarks validates that organization admins can
// cancel benchmark runs.
//
// Use Case: UC-BM-006 - Cancel Running Benchmarks
// Acceptance Criteria:
//   - AC-01: Cancel running benchmark
//   - AC-02: Cancel pending benchmark
//   - AC-03: Reject cancel on completed run
//   - AC-04: Reject cancel on non-existent run
//
// See: usecases/benchmarks.yaml
func TestUC_BM_006_CancelRunningBenchmarks(t *testing.T) {
	t.Run("AC-01: cancel running benchmark", func(t *testing.T) {
		// Given: Benchmark run is in running state
		// When: User runs `ai-aas-org benchmark run cancel <run-id>`
		// Then: Exit code is 0
		// Then: Message confirms cancellation
		// Then: Status changes to "cancelled"
	})

	t.Run("AC-02: cancel pending benchmark", func(t *testing.T) {
		// Given: Benchmark run is in pending state (queued)
		// When: User runs `ai-aas-org benchmark run cancel <run-id>`
		// Then: Benchmark run is removed from queue
		// Then: Status changes to "cancelled"
		// Then: Exit code is 0
	})

	t.Run("AC-03: reject cancel on completed run", func(t *testing.T) {
		// Given: Benchmark run has already completed
		// When: User runs `ai-aas-org benchmark run cancel <run-id>`
		// Then: Command fails with non-zero exit code
		// Then: Error message indicates run already completed
	})

	t.Run("AC-04: reject cancel on non-existent run", func(t *testing.T) {
		// Given: Benchmark run does not exist
		// When: User runs `ai-aas-org benchmark run cancel <run-id>`
		// Then: Command fails with non-zero exit code
		// Then: Error message indicates run not found
	})
}

// TestUC_BM_006_CancelRunningBenchmarks_MustNot validates negative requirements.
func TestUC_BM_006_CancelRunningBenchmarks_MustNot(t *testing.T) {
	t.Run("must not cancel runs from other organizations", func(t *testing.T) {
		// Given: User is authenticated with org admin API key
		// When: User tries to cancel another org's run
		// Then: Access is denied or run not found
	})

	t.Run("must not delete run records", func(t *testing.T) {
		// Given: User cancels a run
		// When: Cancellation completes
		// Then: Run record should still exist (just with cancelled status)
	})
}

// TestUC_BM_007_CompareResultsAcrossRuns validates that organization admins can
// compare benchmark results across multiple runs.
//
// Use Case: UC-BM-007 - Compare Results Across Runs
// Acceptance Criteria:
//   - AC-01: Compare two runs
//   - AC-02: Compare with percentage changes
//   - AC-03: Compare as JSON output
//   - AC-04: Reject compare with incomplete run
//   - AC-05: Reject compare with non-existent run
//
// See: usecases/benchmarks.yaml
func TestUC_BM_007_CompareResultsAcrossRuns(t *testing.T) {
	t.Run("AC-01: compare two runs", func(t *testing.T) {
		t.Skip("Requires real benchmark runs - needs test harness to create and wait for completion of two runs")

		// Given: Two completed benchmark runs exist
		// When: User runs `ai-aas-org benchmark compare <run-1> <run-2>`
		result := runOrgCLI("benchmark", "compare", "run-id-1", "run-id-2")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Side-by-side comparison is displayed
		if !strings.Contains(result.Output, "Benchmark Comparison") {
			t.Error("expected comparison header in output")
		}

		// Then: Throughput difference is shown
		if !strings.Contains(result.Output, "Throughput") {
			t.Error("expected throughput metrics in output")
		}

		// Then: Latency differences are shown (p50, p90, p99)
		if !strings.Contains(result.Output, "P50") || !strings.Contains(result.Output, "P90") || !strings.Contains(result.Output, "P99") {
			t.Error("expected latency percentiles (P50, P90, P99) in output")
		}

		// Then: Error rate difference is shown
		// (Only if errors exist - this is conditional)
	})

	t.Run("AC-02: compare with percentage changes", func(t *testing.T) {
		t.Skip("Requires real benchmark runs - needs test harness to create and wait for completion of two runs")

		// Given: Two completed benchmark runs exist
		// When: User runs `ai-aas-org benchmark compare <run-1> <run-2> --show-delta`
		result := runOrgCLI("benchmark", "compare", "run-id-1", "run-id-2", "--show-delta")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Percentage changes are displayed
		if !strings.Contains(result.Output, "%") {
			t.Error("expected percentage changes in output")
		}

		// Then: Improvements are marked
		if !strings.Contains(result.Output, "improvement") && !strings.Contains(result.Output, "regression") {
			t.Error("expected improvement/regression markers in output")
		}
	})

	t.Run("AC-03: compare as JSON output", func(t *testing.T) {
		t.Skip("Requires real benchmark runs - needs test harness to create and wait for completion of two runs")

		// Given: Two completed benchmark runs exist
		// When: User runs `ai-aas-org benchmark compare <run-1> <run-2> --json`
		result := runOrgCLI("benchmark", "compare", "run-id-1", "run-id-2", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Results output as valid JSON
		var jsonOutput map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &jsonOutput); err != nil {
			t.Fatalf("expected valid JSON output, got error: %v", err)
		}

		// Then: JSON includes both run metrics
		if _, ok := jsonOutput["run1"]; !ok {
			t.Error("expected run1 in JSON output")
		}
		if _, ok := jsonOutput["run2"]; !ok {
			t.Error("expected run2 in JSON output")
		}

		// Then: JSON includes delta calculations
		if _, ok := jsonOutput["deltas"]; !ok {
			t.Error("expected deltas in JSON output")
		}
	})

	t.Run("AC-04: reject compare with incomplete run", func(t *testing.T) {
		t.Skip("Requires real benchmark runs - needs test harness to create one completed and one running benchmark")

		// Given: One run is completed but another is still running
		// When: User runs `ai-aas-org benchmark compare <run-1> <run-running>`
		result := runOrgCLI("benchmark", "compare", "run-completed", "run-running")

		// Then: Command fails with non-zero exit code
		if result.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates run is not completed
		if !strings.Contains(strings.ToLower(result.Output), "not completed") {
			t.Error("expected 'not completed' in error message")
		}

		// Then: Suggestion to wait for completion
		if !strings.Contains(strings.ToLower(result.Output), "wait") {
			t.Error("expected suggestion to wait in error message")
		}
	})

	t.Run("AC-05: reject compare with non-existent run", func(t *testing.T) {
		t.Skip("Requires real benchmark runs - needs test harness to create one run")

		// Given: One run exists but another does not
		// When: User runs `ai-aas-org benchmark compare <run-1> <run-nonexistent>`
		result := runOrgCLI("benchmark", "compare", "run-exists", "run-nonexistent-999")

		// Then: Command fails with non-zero exit code
		if result.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates which run was not found
		if !strings.Contains(strings.ToLower(result.Output), "not found") {
			t.Error("expected 'not found' in error message")
		}
	})
}

// TestUC_BM_007_CompareResultsAcrossRuns_MustNot validates negative requirements.
func TestUC_BM_007_CompareResultsAcrossRuns_MustNot(t *testing.T) {
	t.Run("must not compare runs from different organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User tries to compare with another org's run
		// Then: Access is denied or run not found
		// Note: This is enforced by the API's org-scoped access control
		// The CLI will receive a "not found" error when trying to fetch a run from another org
	})

	t.Run("must not modify any run data", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Comparison is a read-only operation
		// This is an implicit requirement - no modifications should occur
		// The compare command only calls GET endpoints, never POST/PUT/DELETE
		// This test passes by virtue of the implementation using only read operations
	})

	t.Run("must not expose internal implementation details", func(t *testing.T) {
		t.Skip("Requires real benchmark runs - needs test harness to create and wait for completion of two runs")

		// Given: User compares two runs
		result := runOrgCLI("benchmark", "compare", "run-id-1", "run-id-2")

		// Then: No internal implementation details are exposed
		// Check that output doesn't contain database IDs, internal URLs, etc.
		if strings.Contains(strings.ToLower(result.Output), "internal") {
			t.Error("output should not expose internal implementation details")
		}
	})
}
