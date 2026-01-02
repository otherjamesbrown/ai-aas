package usecases_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUC_MLC_001_DeployModelFromRegistry validates UC-MLC-001.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator wants to deploy a registered model to a target environment.
// The CLI now uses GitOps-first workflow:
//   1. Generates AIModel YAML manifest
//   2. Writes to ai-aas-config/environments/<env>/models/<model>.yaml
//   3. Commits and pushes to appropriate branch (develop/staging/main)
//   4. ArgoCD syncs and creates the AIModel CR
//
// These tests validate manifest generation and GitOps workflow, not cluster deployment.
// For end-to-end GitOps deployment tests, see gitops_model_lifecycle_test.go (UC-MLC-010/011).
func TestUC_MLC_001_DeployModelFromRegistry(t *testing.T) {
	t.Run("AC-01: deploy model with default settings", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("AC-01 requires GitOps config - not implemented yet")

		// Given: Model "llama-7b" is registered and cached
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e development`
		// Note: Using --dry-run or testing in isolated ai-aas-config clone to avoid polluting repo
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--dry-run")

		// Then: Command succeeds and shows what would be created
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Deployment configuration is displayed (runtime, resources, replicas)
		assertContains(t, result.Output, "llama-7b")
		assertContains(t, result.Output, "development")

		// Then: Shows manifest path and commit message
		assertContains(t, result.Output, "environments/development/models/")
		assertContains(t, result.Output, "deploy:")
	})

	t.Run("AC-02: deploy model with custom resources", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("AC-02 requires GitOps config - not implemented yet")

		// Given: Model requires specific GPU and memory allocation
		// When: Operator runs `ai-aas-cli model deploy create llama-70b -e development --gpu-count 4 --memory 96`
		result := runPlatformCLI("model", "deploy", "create", "llama-70b", "-e", "development",
			"--gpu-count", "4", "--memory", "96", "--dry-run")

		// Then: Command succeeds
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Resource configuration is displayed
		assertContains(t, result.Output, "4")  // GPU count
		assertContains(t, result.Output, "96") // Memory in GB
	})

	t.Run("AC-03: deploy with auto-scaling configuration", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("AC-03 requires GitOps config - not implemented yet")

		// Given: Production environment requires auto-scaling
		// When: Operator runs `ai-aas-cli model deploy create mistral-7b -e production --min-replicas 2 --max-replicas 5`
		result := runPlatformCLI("model", "deploy", "create", "mistral-7b", "-e", "production",
			"--min-replicas", "2", "--max-replicas", "5", "--dry-run")

		// Then: Command succeeds
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Scaling configuration is displayed
		assertContains(t, result.Output, "2") // Min replicas
		assertContains(t, result.Output, "5") // Max replicas
	})

	t.Run("AC-04: deploy without routing policy", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("AC-04 routing policy behavior not yet implemented in CLI")

		// Given: Operator wants to test deployment before exposing to traffic
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e development --no-routing-policy`
		// Note: This flag may not exist yet in the GitOps workflow
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development",
			"--no-routing-policy", "--dry-run")

		// Then: AIModel manifest is created without routing annotations
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Message explains manual policy creation if needed
		assertContains(t, result.Output, "routing")
	})

	t.Run("AC-05: wait for deployment to be ready", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("AC-05 --wait flag requires live cluster - tested in gitops_model_lifecycle_test.go")

		// Given: Operator wants confirmation deployment is operational
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e development --wait`
		// Note: --wait requires live cluster access to poll AIModel status
		// This is covered by UC-MLC-010/AC-02 in gitops_model_lifecycle_test.go
	})

	t.Run("AC-06: reject deployment to unconfigured environment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)

		// Given: Environment "nonexistent-env" is not configured in CLI
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e nonexistent-env`
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "nonexistent-env-xyz-999")

		// Then: Command fails with non-zero exit code
		require.NotEqual(t, 0, result.ExitCode, "command should fail for unconfigured environment")

		// Then: Error message indicates environment not configured or invalid
		lowerOutput := strings.ToLower(result.Output)
		// Accept either "environment not configured" or "invalid environment" or similar
		hasEnvError := strings.Contains(lowerOutput, "environment") &&
			(strings.Contains(lowerOutput, "not") || strings.Contains(lowerOutput, "invalid") ||
				strings.Contains(lowerOutput, "unknown"))
		require.True(t, hasEnvError, "error should mention invalid/unconfigured environment, got: %s", result.Output)
	})

	t.Run("AC-07: reject deployment of unregistered model", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		skipIfNoLiveAPI(t) // Model registry check requires API access

		// Given: Model "unknown-model-xyz-999" is not in the registry
		// When: Operator runs `ai-aas-cli model deploy create unknown-model-xyz-999 -e development`
		result := runPlatformCLI("model", "deploy", "create", "unknown-model-xyz-999", "-e", "development")

		// Then: Command fails with non-zero exit code
		require.NotEqual(t, 0, result.ExitCode, "command should fail for unregistered model")

		// Then: Error message indicates model not found in registry
		lowerOutput := strings.ToLower(result.Output)
		hasNotFoundError := strings.Contains(lowerOutput, "not found") ||
			strings.Contains(lowerOutput, "does not exist") ||
			strings.Contains(lowerOutput, "unknown model")
		require.True(t, hasNotFoundError, "error should indicate model not found, got: %s", result.Output)
	})
}

// TestUC_MLC_002_ScaleModelDeployment validates UC-MLC-002.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator needs to adjust the number of replicas for a deployed
// model in response to load changes or capacity planning.
//
// NOTE: Scaling operations in GitOps workflow require updating the AIModel manifest
// in ai-aas-config and waiting for ArgoCD sync. These tests are skipped pending
// implementation of `model deploy scale` command that edits existing manifests.
func TestUC_MLC_002_ScaleModelDeployment(t *testing.T) {
	t.Run("AC-01: scale deployment to specific replica count", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-002/AC-01 not implemented - scaling requires editing existing manifest in GitOps workflow")

		// Given: Model "llama-7b" is deployed with 1 replica in development
		// When: Operator runs `ai-aas-cli model deploy scale llama-7b -e development --replicas 3`
		// Expected behavior: CLI would read existing manifest, update replicas, commit and push
		// Implementation TODO: Add model deploy scale command that:
		//   1. Reads environments/<env>/models/<model>.yaml
		//   2. Updates spec.minReplicas and spec.maxReplicas
		//   3. Commits with message "scale: <model> to <replicas> replicas in <env>"
		//   4. Pushes to appropriate branch
	})

	t.Run("AC-02: scale down to reduce costs", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-002/AC-02 not implemented - scaling requires editing existing manifest in GitOps workflow")

		// Given: Model "mistral-7b" is running 5 replicas
		// When: Operator runs `ai-aas-cli model deploy scale mistral-7b -e development --replicas 1`
		// Expected: Same GitOps workflow as AC-01
	})

	t.Run("AC-03: reject scaling non-existent deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-002/AC-03 not implemented - scaling requires editing existing manifest in GitOps workflow")

		// Given: Model "unknown-model" is not deployed (no manifest in ai-aas-config)
		// When: Operator runs `ai-aas-cli model deploy scale unknown-model -e development --replicas 2`
		// Expected: CLI checks if environments/<env>/models/<model>.yaml exists, returns error if not
	})
}

// TestUC_MLC_003_ViewDeploymentStatus validates UC-MLC-003.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator wants to check the current status of a deployed model
// to verify it's operational, troubleshoot issues, or monitor replica counts.
//
// NOTE: Status checks require live cluster access to query AIModel CR status.
// These tests are skipped unless running against a live cluster.
func TestUC_MLC_003_ViewDeploymentStatus(t *testing.T) {
	t.Run("AC-01: show deployment status in table format", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-01 requires live cluster with deployed AIModel CR")

		// Given: Model "llama-7b" is deployed and ready
		// When: Operator runs `ai-aas-cli model deploy status llama-7b -e development`
		// Expected: CLI uses kubectl to query AIModel CR status and formats output
		// Implementation: CLI would:
		//   1. Determine namespace from environment (development -> development)
		//   2. kubectl get aimodel llama-7b -n development -o json
		//   3. Parse .status.phase, .status.readyReplicas, .status.conditions
		//   4. Format as table with columns: NAME, NAMESPACE, READY, REPLICAS, URL
	})

	t.Run("AC-02: show status as JSON", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-02 requires live cluster with deployed AIModel CR")

		// Given: Operator wants machine-readable output
		// When: Operator runs `ai-aas-cli model deploy status llama-7b -e development --format json`
		// Expected: CLI returns AIModel status as JSON
		// Output should include: ready, phase, replicas, readyReplicas, inferenceEndpoint, conditions
	})

	t.Run("AC-03: show status of failing deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-03 requires live cluster with failing deployment (hard to test reliably)")

		// This test requires creating a deployment that will fail (e.g., invalid modelID)
		// which is difficult to test reliably without affecting the cluster state.
		// Manual testing or specialized test fixtures recommended.
	})

	t.Run("AC-04: handle non-existent deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-04 requires live cluster access to verify non-existence")

		// Given: Model "unknown-model-xyz-999" is not deployed (no AIModel CR exists)
		// When: Operator runs `ai-aas-cli model deploy status unknown-model-xyz-999 -e development`
		// Expected: CLI detects kubectl get returns NotFound error
		// Then: Command fails with non-zero exit code
		// Then: Error message indicates deployment not found
	})
}

// TestUC_MLC_004_RetireModelDeployment validates UC-MLC-004.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator wants to remove a model deployment from an environment.
func TestUC_MLC_004_RetireModelDeployment(t *testing.T) {
	t.Run("AC-01: delete deployment with confirmation", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-004/AC-01 requires interactive input - difficult to test in automated tests")

		// This test requires interactive confirmation which is difficult to test
		// in automated tests. Manual testing recommended.
	})

	t.Run("AC-02: force delete without confirmation", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-004/AC-02 requires live cluster with deployed model")

		// Setup: Deploy a model
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Operator wants to skip interactive prompt
		// When: Operator runs `ai-aas-cli model deploy delete llama-7b -e development --force`
		result := runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")

		// Then: No confirmation prompt is shown
		// Then: AIModel CR is deleted immediately
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Success message is displayed
		assertContains(t, result.Output, "delet")
	})

	t.Run("AC-03: wait for deletion to complete", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-004/AC-03 requires live cluster with deployed model")

		// Setup: Deploy a model
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Operator wants confirmation cleanup finished
		// When: Operator runs `ai-aas-cli model deploy delete llama-7b -e development --force --wait`
		result := runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force", "--wait")

		// Then: Deletion is initiated
		// Then: Command waits for operator to clean up resources
		// Then: Message confirms deletion complete
		require.Equal(t, 0, result.ExitCode, "command should succeed")
		assertContains(t, result.Output, "delet")
	})

	t.Run("AC-04: cancel deletion on negative confirmation", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-004/AC-04 requires interactive input - difficult to test in automated tests")

		// This test requires interactive confirmation which is difficult to test
		// in automated tests. Manual testing recommended.
	})

	t.Run("AC-05: handle non-existent deployment gracefully", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-004/AC-05 requires live cluster")

		// Given: Model "unknown-model" is not deployed
		// When: Operator runs `ai-aas-cli model deploy delete unknown-model -e development`
		result := runPlatformCLI("model", "deploy", "delete", "unknown-model-xyz-999", "-e", "development", "--force")

		// Then: Message shows "Deployment unknown-model-development not found in development"
		// Then: No error is raised (graceful handling)
		require.Equal(t, 0, result.ExitCode, "command should handle non-existent deployment gracefully")
	})
}

// TestUC_MLC_005_RollbackDeployment validates UC-MLC-005.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator needs to perform a rolling restart of a model deployment
// to pick up new model weights, recover from issues, or refresh pods after
// configuration changes.
func TestUC_MLC_005_RollbackDeployment(t *testing.T) {
	t.Run("AC-01: trigger rolling restart", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-005/AC-01 requires live cluster with deployed model")

		// Setup: Deploy a model
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--wait")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Model "llama-7b" is deployed in development
		// When: Operator runs `ai-aas-cli model deploy restart llama-7b -e development`
		result := runPlatformCLI("model", "deploy", "restart", "llama-7b", "-e", "development")

		// Then: Deployment name and namespace are displayed
		// Then: Message confirms "Rolling restart triggered"
		require.Equal(t, 0, result.ExitCode, "command should succeed")
		assertContains(t, result.Output, "restart")

		// Then: Suggested next steps include checking status and logs
		// (Output validation for suggestions is optional)

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-02: restart and wait for completion", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-005/AC-02 requires live cluster with deployed model")

		// Setup: Deploy a model
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--wait")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Operator wants confirmation pods are ready after restart
		// When: Operator runs `ai-aas-cli model deploy restart llama-7b -e development --wait`
		result := runPlatformCLI("model", "deploy", "restart", "llama-7b", "-e", "development", "--wait")

		// Then: Restart is initiated
		// Then: Command blocks until all pods are ready
		// Then: Success message confirms all pods ready
		require.Equal(t, 0, result.ExitCode, "command should succeed only after pods are ready")
		assertContains(t, result.Output, "ready")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-03: reject restart of non-existent deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-005/AC-03 requires live cluster")

		// Given: Model "unknown-model" is not deployed
		// When: Operator runs `ai-aas-cli model deploy restart unknown-model -e development`
		result := runPlatformCLI("model", "deploy", "restart", "unknown-model-xyz-999", "-e", "development")

		// Then: Command fails with exit code 5 (not found)
		require.NotEqual(t, 0, result.ExitCode, "command should fail for non-existent deployment")

		// Then: Error message indicates deployment not found
		assertContains(t, result.Output, "not found")
	})
}
