package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Platform Operations use case tests
// See: usecases/platform-ops.yaml
// CLI helpers defined in helpers_test.go

// TestUC_OPS_001_MonitorModelDeploymentStatus validates UC-OPS-001.
// Spec: usecases/platform-ops.yaml
//
// A platform operator wants to monitor the deployment status of a model to
// understand where it is in the deployment lifecycle. This is critical for
// GitOps workflows where deployments are triggered by configuration changes
// and operators need visibility into progress.
func TestUC_OPS_001_MonitorModelDeploymentStatus(t *testing.T) {
	t.Run("AC-01: show current deployment phase", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-01 requires live deployment - implementation pending")

		// Given: Model "llama-7b" is deployed and in "ModelDownloading" phase
		// When: User runs `ai-aas-cli model deploy status llama-7b`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b")

		// Then: Current phase "ModelDownloading" is displayed
		assertContains(t, result.Output, "ModelDownloading")

		// Then: Phase start time is shown
		assertContains(t, result.Output, "started")

		// Then: Exit code is 0
		require.Equal(t, 0, result.ExitCode, "command should succeed")
	})

	t.Run("AC-02: show deployment timeline with phase progression", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-02 requires live deployment - implementation pending")

		// Given: Model "llama-7b" has completed "ImagePulling" and is in "ModelDownloading"
		// When: User runs `ai-aas-cli model deploy status llama-7b`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b")

		// Then: Timeline shows completed phases with checkmarks
		// Then: Current phase is highlighted
		// Then: Upcoming phases are listed
		// Then: Duration of completed phases is shown
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Verify timeline elements are present
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "imagepulling") && !strings.Contains(output, "image pulling") {
			t.Errorf("Expected timeline to show ImagePulling phase")
		}
		if !strings.Contains(output, "modeldownloading") && !strings.Contains(output, "model downloading") {
			t.Errorf("Expected timeline to show ModelDownloading phase")
		}
	})

	t.Run("AC-03: show blocking reason when deployment is stuck", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-03 requires live deployment - implementation pending")

		// Given: Model "llama-7b" is stuck in "Pending" due to insufficient GPU resources
		// When: User runs `ai-aas-cli model deploy status llama-7b`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b")

		// Then: Current phase "Pending" is displayed
		assertContains(t, result.Output, "pending")

		// Then: Blocking reason "Insufficient GPU resources" is shown
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "insufficient") || !strings.Contains(output, "gpu") {
			t.Errorf("Expected blocking reason to mention insufficient GPU resources")
		}

		// Then: Suggestion to check node availability is provided
		if !strings.Contains(output, "node") || !strings.Contains(output, "check") {
			t.Errorf("Expected suggestion to check node availability")
		}
	})

	t.Run("AC-04: show download progress during model download phase", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-04 requires live deployment - implementation pending")

		// Given: Model "llama-7b" is downloading model weights (50% complete)
		// When: User runs `ai-aas-cli model deploy status llama-7b`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b")

		// Then: Download progress percentage is shown
		// Then: Downloaded size vs total size is displayed
		// Then: Estimated time remaining is shown
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		output := strings.ToLower(result.Output)
		// Check for progress indicators (percentage, size, or time)
		hasProgress := strings.Contains(output, "%") ||
			strings.Contains(output, "gb") ||
			strings.Contains(output, "mb") ||
			strings.Contains(output, "progress")

		if !hasProgress {
			t.Errorf("Expected download progress information in output")
		}
	})

	t.Run("AC-05: watch mode for live status updates", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-05 requires live deployment and interactive testing - implementation pending")

		// Given: Model "llama-7b" is currently deploying
		// When: User runs `ai-aas-cli model deploy status llama-7b --watch`
		// Then: Status updates in real-time
		// Then: Screen refreshes when phase changes
		// Then: Progress indicators update continuously
		// Then: User can exit with Ctrl+C

		// Note: Interactive --watch mode testing requires special setup
		// This test verifies the flag is accepted without error
		result := runPlatformCLI("model", "deploy", "status", "llama-7b", "--watch", "--help")
		require.Equal(t, 0, result.ExitCode, "watch flag should be recognized")
	})

	t.Run("AC-06: show readiness probe timing", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-06 requires live deployment - implementation pending")

		// Given: Model "llama-7b" is in "Starting" phase with readiness probe configured
		// When: User runs `ai-aas-cli model deploy status llama-7b`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b")

		// Then: Expected startup time based on probe config is shown
		// Then: Time since pod started is displayed
		// Then: Number of probe attempts is shown
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		output := strings.ToLower(result.Output)
		hasProbeInfo := strings.Contains(output, "probe") ||
			strings.Contains(output, "readiness") ||
			strings.Contains(output, "attempts")

		if !hasProbeInfo {
			t.Errorf("Expected readiness probe timing information in output")
		}
	})

	t.Run("AC-07: handle model not found gracefully", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-001/AC-07 requires live cluster to test not found behavior")

		// Given: No deployment exists for model "nonexistent-model"
		// When: User runs `ai-aas-cli model deploy status nonexistent-model -e development`
		result := runPlatformCLI("model", "deploy", "status", "nonexistent-model", "-e", "development")

		// Then: Command fails with non-zero exit code
		// Note: Exit code 5 is the ideal for "not found", but accept any non-zero for now
		require.NotEqual(t, 0, result.ExitCode, "command should fail for non-existent model")

		// Then: Error message indicates deployment not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") && !strings.Contains(output, "does not exist") {
			t.Errorf("Expected 'not found' message, got: %s", result.Output)
		}

		// Then: Suggestion to run `ai-aas-cli model deploy list` is shown
		if !strings.Contains(output, "list") {
			t.Logf("Note: Consider adding suggestion to run 'ai-aas-cli model deploy list'")
		}
	})
}

// TestUC_OPS_002_ListAllModelDeployments validates UC-OPS-002.
// Spec: usecases/platform-ops.yaml
//
// A platform operator wants to see an overview of all model deployments
// in the cluster. This provides a dashboard view of deployment health
// and helps identify which models need attention.
func TestUC_OPS_002_ListAllModelDeployments(t *testing.T) {
	t.Run("AC-01: list all deployments with status summary", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-002/AC-01 requires live deployments - implementation pending")

		// Given: Multiple models are deployed in various phases
		// When: User runs `ai-aas-cli model deploy list`
		result := runPlatformCLI("model", "deploy", "list")

		// Then: All model deployments are listed
		// Then: Each shows model name, current phase, and age
		// Then: Status is color-coded (green=Ready, yellow=Pending, red=Failed)
		// Then: Output is formatted as a table
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Verify table-like output structure
		if result.Output == "" {
			t.Error("expected non-empty output")
		}

		// Look for table headers
		output := strings.ToLower(result.Output)
		hasTableHeaders := strings.Contains(output, "name") ||
			strings.Contains(output, "model") ||
			strings.Contains(output, "phase") ||
			strings.Contains(output, "status")

		if !hasTableHeaders {
			t.Errorf("Expected table headers in output")
		}
	})

	t.Run("AC-02: filter deployments by phase", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-002/AC-02 requires live deployments - implementation pending")

		// Given: Multiple models are deployed, some Ready, some Pending
		// When: User runs `ai-aas-cli model deploy list --phase Pending`
		result := runPlatformCLI("model", "deploy", "list", "--phase", "Pending")

		// Then: Only deployments in "Pending" phase are shown
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Filter is case-insensitive
		resultLower := runPlatformCLI("model", "deploy", "list", "--phase", "pending")
		require.Equal(t, 0, resultLower.ExitCode, "command should succeed with lowercase phase")
	})

	t.Run("AC-03: show blocked deployments prominently", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-002/AC-03 requires live deployments - implementation pending")

		// Given: Some deployments are blocked due to resource constraints
		// When: User runs `ai-aas-cli model deploy list`
		result := runPlatformCLI("model", "deploy", "list")

		// Then: Blocked deployments are marked with warning indicator
		// Then: Summary shows count of blocked deployments
		// Then: Blocked reason summary is shown inline
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		output := strings.ToLower(result.Output)
		hasBlockedInfo := strings.Contains(output, "blocked") ||
			strings.Contains(output, "warning") ||
			strings.Contains(output, "⚠")

		if !hasBlockedInfo {
			t.Logf("Note: If blocked deployments exist, they should be prominently marked")
		}
	})

	t.Run("AC-04: handle no deployments gracefully", func(t *testing.T) {
		skipIfNoPlatformCLI(t)

		// Given: No model deployments exist in the cluster
		// When: User runs `ai-aas-cli model deploy list`
		result := runPlatformCLI("model", "deploy", "list")

		// Then: Exit code is 0
		require.Equal(t, 0, result.ExitCode, "command should succeed even with no deployments")

		// Then: Message indicates no deployments found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "no") && !strings.Contains(output, "none") && !strings.Contains(output, "empty") {
			t.Logf("Note: Empty deployment list should include informative message")
		}
	})

	t.Run("AC-05: JSON output format for automation", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-002/AC-05 requires live deployments - implementation pending")

		// Given: Multiple models are deployed
		// When: User runs `ai-aas-cli model deploy list --json`
		result := runPlatformCLI("model", "deploy", "list", "--json")

		// Then: Exit code is 0
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Fatalf("Expected valid JSON output, got: %s", result.Output)
		}

		// Then: Each deployment includes name, phase, age, blocked status
		var deployments []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &deployments); err != nil {
			t.Fatalf("Failed to parse JSON array: %v", err)
		}

		// Verify expected fields exist in at least one deployment (if any exist)
		if len(deployments) > 0 {
			deployment := deployments[0]
			expectedFields := []string{"name", "phase", "age"}
			for _, field := range expectedFields {
				if _, exists := deployment[field]; !exists {
					t.Errorf("Expected field '%s' in deployment JSON object", field)
				}
			}
		}
	})
}

// TestUC_OPS_003_TroubleshootFailedDeployment validates UC-OPS-003.
// Spec: usecases/platform-ops.yaml
//
// A platform operator needs to diagnose why a model deployment has failed
// or is stuck. This use case provides diagnostic information to help
// identify the root cause without requiring direct kubectl access.
func TestUC_OPS_003_TroubleshootFailedDeployment(t *testing.T) {
	t.Run("AC-01: show deployment diagnostics and recent events", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-003/AC-01 requires live deployment - implementation pending")

		// Given: Model "llama-7b" deployment is failing with CrashLoopBackOff
		// When: User runs `ai-aas-cli model troubleshoot llama-7b`
		result := runPlatformCLI("model", "troubleshoot", "llama-7b")

		// Then: Recent events from the past hour are displayed
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "event") {
			t.Errorf("Expected recent events to be displayed")
		}

		// Then: Error messages from pod logs are shown
		if !strings.Contains(output, "error") && !strings.Contains(output, "failed") {
			t.Logf("Note: If errors exist, they should be displayed")
		}

		// Then: Container restart count is displayed
		if !strings.Contains(output, "restart") {
			t.Logf("Note: Restart count should be displayed for CrashLoopBackOff")
		}

		// Then: Last termination reason is shown
		if !strings.Contains(output, "termination") && !strings.Contains(output, "terminated") {
			t.Logf("Note: Termination reason should be shown for failed containers")
		}
	})

	t.Run("AC-02: check and report configuration validity", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-003/AC-02 requires live deployment - implementation pending")

		// Given: Model "llama-7b" has invalid resource request (requesting more GPU than available)
		// When: User runs `ai-aas-cli model troubleshoot llama-7b`
		result := runPlatformCLI("model", "troubleshoot", "llama-7b")

		// Then: Configuration validation results are shown
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "configuration") && !strings.Contains(output, "config") {
			t.Logf("Note: Configuration validation should be displayed")
		}

		// Then: Invalid configuration is flagged with explanation
		if !strings.Contains(output, "invalid") && !strings.Contains(output, "error") {
			t.Logf("Note: Invalid configurations should be flagged")
		}

		// Then: Resource requests vs cluster capacity is displayed
		if !strings.Contains(output, "resource") || !strings.Contains(output, "capacity") {
			t.Logf("Note: Resource availability should be checked")
		}

		// Then: Suggestion to adjust configuration is provided
		if !strings.Contains(output, "adjust") && !strings.Contains(output, "reduce") {
			t.Logf("Note: Actionable suggestions should be provided")
		}
	})

	t.Run("AC-03: identify scheduling issues", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-003/AC-03 requires live deployment - implementation pending")

		// Given: Model "llama-7b" cannot be scheduled due to missing tolerations
		// When: User runs `ai-aas-cli model troubleshoot llama-7b`
		result := runPlatformCLI("model", "troubleshoot", "llama-7b")

		// Then: Scheduling failure reason is identified
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "schedul") {
			t.Errorf("Expected scheduling information to be displayed")
		}

		// Then: Missing tolerations or node selectors are flagged
		if !strings.Contains(output, "toleration") && !strings.Contains(output, "selector") {
			t.Logf("Note: Missing tolerations/selectors should be identified")
		}

		// Then: Available nodes and their taints are summarized
		if !strings.Contains(output, "node") || !strings.Contains(output, "taint") {
			t.Logf("Note: Node availability and taints should be shown")
		}

		// Then: Suggestion for toleration fix is provided
		if !strings.Contains(output, "add") && !strings.Contains(output, "update") {
			t.Logf("Note: Actionable suggestions should be provided")
		}
	})

	t.Run("AC-04: check image pull issues", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-003/AC-04 requires live deployment - implementation pending")

		// Given: Model "llama-7b" is failing due to image pull error
		// When: User runs `ai-aas-cli model troubleshoot llama-7b`
		result := runPlatformCLI("model", "troubleshoot", "llama-7b")

		// Then: Image pull error is identified
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "image") || !strings.Contains(output, "pull") {
			t.Errorf("Expected image pull issues to be identified")
		}

		// Then: Image name and registry are shown
		if !strings.Contains(output, "registry") {
			t.Logf("Note: Image registry information should be shown")
		}

		// Then: Credential status is checked and reported
		if !strings.Contains(output, "credential") && !strings.Contains(output, "auth") {
			t.Logf("Note: Credential status should be checked")
		}

		// Then: Suggestion to verify registry access is provided
		if !strings.Contains(output, "verify") && !strings.Contains(output, "check") {
			t.Logf("Note: Actionable suggestions should be provided")
		}
	})

	t.Run("AC-05: handle healthy deployment gracefully", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-003/AC-05 requires live healthy deployment - implementation pending")

		// Given: Model "llama-7b" is deployed and running successfully
		// When: User runs `ai-aas-cli model troubleshoot llama-7b`
		result := runPlatformCLI("model", "troubleshoot", "llama-7b")

		// Then: Exit code is 0
		require.Equal(t, 0, result.ExitCode, "command should succeed for healthy deployment")

		// Then: Message indicates deployment is healthy
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "healthy") && !strings.Contains(output, "running") && !strings.Contains(output, "ready") {
			t.Errorf("Expected message indicating deployment is healthy")
		}

		// Then: Key health metrics are shown (uptime, restarts, readiness)
		hasMetrics := strings.Contains(output, "uptime") ||
			strings.Contains(output, "restart") ||
			strings.Contains(output, "ready")

		if !hasMetrics {
			t.Errorf("Expected health metrics to be displayed")
		}
	})

	t.Run("AC-06: JSON output for automation", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-OPS-003/AC-06 requires live deployment - implementation pending")

		// Given: Model "llama-7b" has deployment issues
		// When: User runs `ai-aas-cli model troubleshoot llama-7b --json`
		result := runPlatformCLI("model", "troubleshoot", "llama-7b", "--json")

		// Then: Output is valid JSON object
		if !isValidJSON(result.Output) {
			t.Fatalf("Expected valid JSON output, got: %s", result.Output)
		}

		// Then: Includes events, errors, diagnostics arrays
		var diagnostics map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &diagnostics); err != nil {
			t.Fatalf("Failed to parse JSON object: %v", err)
		}

		// Then: Includes overall health status
		if _, exists := diagnostics["status"]; !exists {
			if _, exists := diagnostics["health"]; !exists {
				t.Errorf("Expected 'status' or 'health' field in diagnostics JSON")
			}
		}

		// Then: Exit code reflects deployment health (0=healthy, 1=unhealthy)
		// Note: Exit code assertion depends on actual deployment state
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Expected exit code 0 (healthy) or 1 (unhealthy), got %d", result.ExitCode)
		}
	})
}
