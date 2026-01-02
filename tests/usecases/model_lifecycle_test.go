package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUC_MLC_001_DeployModelFromRegistry validates UC-MLC-001.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator wants to deploy a registered model to a target environment.
// The deployment creates an AIModel custom resource that the ai-model-operator
// reconciles into a KServe InferenceService with appropriate resource allocations.
//
// NOTE: These tests are currently skipped because they require the CLI commands to be implemented first.
// They should be enabled once the CLI `ai-aas-cli model deploy` commands are fully functional.
func TestUC_MLC_001_DeployModelFromRegistry(t *testing.T) {
	t.Run("AC-01: deploy model with default settings", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-01 requires live cluster - see gitops_model_lifecycle_test.go for GitOps tests")

		// Given: Model "llama-7b" is registered and cached
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e development`
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development")

		// Then: AIModel CR is created in development namespace
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Deployment configuration is displayed (runtime, resources, replicas)
		assertContains(t, result.Output, "runtime")
		assertContains(t, result.Output, "resources")
		assertContains(t, result.Output, "replicas")

		// Then: Success message with next steps is shown
		assertContains(t, result.Output, "success")

		// Cleanup: Delete the deployment
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-02: deploy model with custom resources", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-02 requires live cluster - see gitops_model_lifecycle_test.go for GitOps tests")

		// Given: Model requires specific GPU and memory allocation
		// When: Operator runs `ai-aas-cli model deploy create llama-70b -e development --gpu-count 4 --memory 96`
		result := runPlatformCLI("model", "deploy", "create", "llama-70b", "-e", "development", "--gpu-count", "4", "--memory", "96")

		// Then: AIModel CR is created with 4 GPUs and 96GB memory
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Resource configuration is displayed before creation
		assertContains(t, result.Output, "4")      // GPU count
		assertContains(t, result.Output, "96")     // Memory

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-70b", "-e", "development", "--force")
		})
	})

	t.Run("AC-03: deploy with auto-scaling configuration", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-03 requires live cluster - see gitops_model_lifecycle_test.go for GitOps tests")

		// Given: Production environment requires auto-scaling
		// When: Operator runs `ai-aas-cli model deploy create mistral-7b -e production --min-replicas 2 --max-replicas 5`
		result := runPlatformCLI("model", "deploy", "create", "mistral-7b", "-e", "production", "--min-replicas", "2", "--max-replicas", "5")

		// Then: AIModel CR is created with min=2, max=5 replicas
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Scaling configuration is displayed
		assertContains(t, result.Output, "min")
		assertContains(t, result.Output, "max")
		assertContains(t, result.Output, "2")
		assertContains(t, result.Output, "5")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "mistral-7b", "-e", "production", "--force")
		})
	})

	t.Run("AC-04: deploy without routing policy", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-04 requires live cluster - see gitops_model_lifecycle_test.go for GitOps tests")

		// Given: Operator wants to test deployment before exposing to traffic
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e development --no-routing-policy`
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--no-routing-policy")

		// Then: AIModel CR is created successfully
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: No routing policy is created
		// Then: Message explains manual policy creation if needed
		assertContains(t, result.Output, "routing")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-05: wait for deployment to be ready", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-05 requires live cluster - see gitops_model_lifecycle_test.go for GitOps tests")

		// Given: Operator wants confirmation deployment is operational
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e development --wait`
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--wait")

		// Then: Command blocks until AIModel phase is "Ready"
		// Then: Final status shows ready replicas and inference endpoint
		require.Equal(t, 0, result.ExitCode, "command should succeed only if deployment reaches Ready")

		// Then: Progress indicator shows deployment status
		assertContains(t, result.Output, "ready")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-06: reject deployment to unconfigured environment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-06 requires CLI configuration testing")

		// Given: Environment "staging" is not configured in CLI
		// When: Operator runs `ai-aas-cli model deploy create llama-7b -e staging`
		result := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "nonexistent-env")

		// Then: Command fails with exit code 3 (config error)
		require.NotEqual(t, 0, result.ExitCode, "command should fail for unconfigured environment")

		// Then: Error message indicates environment not configured
		assertContains(t, result.Output, "environment")
		assertContains(t, result.Output, "not configured")
	})

	t.Run("AC-07: reject deployment of unregistered model", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-001/AC-07 requires model registry checks")

		// Given: Model "unknown-model" is not in the registry
		// When: Operator runs `ai-aas-cli model deploy create unknown-model -e development`
		result := runPlatformCLI("model", "deploy", "create", "unknown-model-xyz-999", "-e", "development")

		// Then: Command fails with exit code 5 (not found)
		require.NotEqual(t, 0, result.ExitCode, "command should fail for unregistered model")

		// Then: Error message indicates model not found in registry
		assertContains(t, result.Output, "not found")
	})
}

// TestUC_MLC_002_ScaleModelDeployment validates UC-MLC-002.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator needs to adjust the number of replicas for a deployed
// model in response to load changes or capacity planning.
func TestUC_MLC_002_ScaleModelDeployment(t *testing.T) {
	t.Run("AC-01: scale deployment to specific replica count", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-002/AC-01 requires live cluster with deployed model")

		// Setup: Deploy a model first
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Model "llama-7b" is deployed with 1 replica in development
		// When: Operator runs `ai-aas-cli model deploy scale llama-7b -e development --replicas 3`
		result := runPlatformCLI("model", "deploy", "scale", "llama-7b", "-e", "development", "--replicas", "3")

		// Then: Current and target replica counts are displayed
		// Then: InferenceService is scaled to 3 replicas
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Success message confirms scaling operation
		assertContains(t, result.Output, "3")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-02: scale down to reduce costs", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-002/AC-02 requires live cluster with deployed model")

		// Setup: Deploy a model with multiple replicas
		setupResult := runPlatformCLI("model", "deploy", "create", "mistral-7b", "-e", "development", "--min-replicas", "5")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Model "mistral-7b" is running 5 replicas
		// When: Operator runs `ai-aas-cli model deploy scale mistral-7b -e development --replicas 1`
		result := runPlatformCLI("model", "deploy", "scale", "mistral-7b", "-e", "development", "--replicas", "1")

		// Then: Scaling operation reduces replicas to 1
		require.Equal(t, 0, result.ExitCode, "command should succeed")

		// Then: Current status shows the scale-down
		assertContains(t, result.Output, "1")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "mistral-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-03: reject scaling non-existent deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-002/AC-03 requires live cluster")

		// Given: Model "unknown-model" is not deployed
		// When: Operator runs `ai-aas-cli model deploy scale unknown-model -e development --replicas 2`
		result := runPlatformCLI("model", "deploy", "scale", "unknown-model-xyz-999", "-e", "development", "--replicas", "2")

		// Then: Command fails with exit code 5 (not found)
		require.NotEqual(t, 0, result.ExitCode, "command should fail for non-existent deployment")

		// Then: Error message indicates deployment not found
		assertContains(t, result.Output, "not found")
	})
}

// TestUC_MLC_003_ViewDeploymentStatus validates UC-MLC-003.
// Spec: usecases/model-lifecycle.yaml
//
// A platform operator wants to check the current status of a deployed model
// to verify it's operational, troubleshoot issues, or monitor replica counts.
func TestUC_MLC_003_ViewDeploymentStatus(t *testing.T) {
	t.Run("AC-01: show deployment status in table format", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-01 requires live cluster with deployed model")

		// Setup: Deploy a model
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--wait")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Model "llama-7b" is deployed and ready
		// When: Operator runs `ai-aas-cli model deploy status llama-7b -e development`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b", "-e", "development")

		// Then: Deployment name and namespace are displayed
		// Then: Ready status shows true with checkmark icon
		// Then: Replica counts show ready/total (e.g., "2/2 ready")
		// Then: Inference URL is displayed
		require.Equal(t, 0, result.ExitCode, "command should succeed")
		assertContains(t, result.Output, "llama-7b")
		assertContains(t, result.Output, "development")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-02: show status as JSON", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-02 requires live cluster with deployed model")

		// Setup: Deploy a model
		setupResult := runPlatformCLI("model", "deploy", "create", "llama-7b", "-e", "development", "--wait")
		require.Equal(t, 0, setupResult.ExitCode, "setup: failed to deploy model")

		// Given: Operator wants machine-readable output
		// When: Operator runs `ai-aas-cli model deploy status llama-7b -e development --format json`
		result := runPlatformCLI("model", "deploy", "status", "llama-7b", "-e", "development", "--format", "json")

		// Then: Output is valid JSON object
		require.Equal(t, 0, result.ExitCode, "command should succeed")
		require.True(t, isValidJSON(result.Output), "output should be valid JSON")

		// Then: JSON includes ready, replicas, readyReplicas, url, conditions
		assertContains(t, result.Output, "ready")
		assertContains(t, result.Output, "replicas")

		// Cleanup
		t.Cleanup(func() {
			runPlatformCLI("model", "deploy", "delete", "llama-7b", "-e", "development", "--force")
		})
	})

	t.Run("AC-03: show status of failing deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-03 requires live cluster with failing deployment (hard to test reliably)")

		// This test requires creating a deployment that will fail (e.g., invalid image)
		// which is difficult to test reliably without affecting the cluster state.
		// Manual testing or specialized test fixtures recommended.
	})

	t.Run("AC-04: handle non-existent deployment", func(t *testing.T) {
		skipIfNoPlatformCLI(t)
		t.Skip("UC-MLC-003/AC-04 requires live cluster")

		// Given: Model "unknown-model" is not deployed
		// When: Operator runs `ai-aas-cli model deploy status unknown-model -e development`
		result := runPlatformCLI("model", "deploy", "status", "unknown-model-xyz-999", "-e", "development")

		// Then: Command fails with exit code 5 (not found)
		require.NotEqual(t, 0, result.ExitCode, "command should fail for non-existent deployment")

		// Then: Error message indicates deployment not found
		assertContains(t, result.Output, "not found")
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
