package usecases_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitOps Model Lifecycle Tests (UC-MLC-010, UC-MLC-011)
//
// These tests validate the GitOps-only model deployment workflow:
// 1. Models are deployed by adding YAML to ai-aas-config repo
// 2. ArgoCD syncs the config and creates AIModel CRs
// 3. Operator reconciles AIModel to InferenceService
// 4. Removing config from repo triggers prune and cleanup
//
// Prerequisites:
// - RUN_GITOPS_TESTS=1 environment variable
// - AI_AAS_CONFIG_PATH set to local ai-aas-config repo path
// - KUBECONFIG set to development cluster kubeconfig
// - Git credentials configured for push access to ai-aas-config

const (
	// Test model configuration
	testModelName      = "tinyllama-gitops-test"
	testModelNamespace = "development"
	testModelID        = "TinyLlama/TinyLlama-1.1B-Chat-v1.0"

	// Timeouts
	argoCDSyncTimeout   = 3 * time.Minute
	modelDeployTimeout  = 10 * time.Minute
	modelCleanupTimeout = 5 * time.Minute
	pollInterval        = 10 * time.Second
)

// tinyllamaModelYAML generates the AIModel YAML for the test model
func tinyllamaModelYAML(uniqueSuffix string) string {
	modelName := fmt.Sprintf("%s-%s", testModelName, uniqueSuffix)
	return fmt.Sprintf(`# AIModel CR for TinyLlama GitOps Test - Development
#
# This model is created by UC-MLC-010/UC-MLC-011 tests.
# It will be automatically cleaned up after the test.
#
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: %s
  namespace: %s
  labels:
    app: vllm-inference
    model: tinyllama-test
    runtime: vllm
    environment: development
    test: gitops-lifecycle
spec:
  modelName: %s
  modelID: %s
  externalName: tinyllama-gitops-test
  enabled: true
  runtime: vllm
  modelType: text
  trustRemoteCode: true
  minReplicas: 1
  maxReplicas: 1
  runtimeArgs:
    - "--max-model-len=2048"
    - "--dtype=float16"
  resources:
    requests:
      cpu: "2"
      memory: "8Gi"
      nvidia.com/gpu: "1"
    limits:
      cpu: "4"
      memory: "16Gi"
      nvidia.com/gpu: "1"
  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
`, modelName, testModelNamespace, modelName, testModelID)
}

// GitOpsTestFixture manages git operations for ai-aas-config
type GitOpsTestFixture struct {
	t              *testing.T
	configRepoPath string
	kubeconfig     string
	modelFileName  string
	uniqueSuffix   string
}

// NewGitOpsTestFixture creates a new fixture for GitOps tests
func NewGitOpsTestFixture(t *testing.T) *GitOpsTestFixture {
	t.Helper()

	configPath := os.Getenv("AI_AAS_CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join(os.Getenv("HOME"), "ai-aas-config")
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), "kubeconfigs", "kubeconfig-development.yaml")
	}

	// Generate unique suffix to avoid conflicts between test runs
	uniqueSuffix := fmt.Sprintf("%d", time.Now().Unix())

	fixture := &GitOpsTestFixture{
		t:              t,
		configRepoPath: configPath,
		kubeconfig:     kubeconfig,
		modelFileName:  fmt.Sprintf("%s-%s.yaml", testModelName, uniqueSuffix),
		uniqueSuffix:   uniqueSuffix,
	}

	// Register cleanup
	t.Cleanup(func() {
		fixture.Cleanup()
	})

	return fixture
}

// ModelName returns the unique model name for this test run
func (f *GitOpsTestFixture) ModelName() string {
	return fmt.Sprintf("%s-%s", testModelName, f.uniqueSuffix)
}

// ModelFilePath returns the full path to the model file in ai-aas-config
func (f *GitOpsTestFixture) ModelFilePath() string {
	return filepath.Join(f.configRepoPath, "environments", "development", "models", f.modelFileName)
}

// Cleanup removes the test model file if it exists
func (f *GitOpsTestFixture) Cleanup() {
	modelFile := f.ModelFilePath()
	if _, err := os.Stat(modelFile); err == nil {
		f.t.Logf("Cleanup: Removing test model file %s", modelFile)
		if err := os.Remove(modelFile); err != nil {
			f.t.Logf("Warning: Failed to remove model file: %v", err)
		}
		// Commit and push cleanup
		if err := f.gitCommitAndPush("test cleanup: remove " + f.modelFileName); err != nil {
			f.t.Logf("Warning: Failed to push cleanup: %v", err)
		}
	}
}

// gitExec runs a git command in the config repo
func (f *GitOpsTestFixture) gitExec(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = f.configRepoPath
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// gitCommitAndPush commits and pushes changes to the config repo
func (f *GitOpsTestFixture) gitCommitAndPush(message string) error {
	// Pull latest first to avoid conflicts
	if _, err := f.gitExec("pull", "--rebase", "origin", "develop"); err != nil {
		f.t.Logf("Warning: git pull failed (may be ok): %v", err)
	}

	// Add all changes
	if _, err := f.gitExec("add", "-A"); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Check if there are changes to commit
	status, _ := f.gitExec("status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		f.t.Log("No changes to commit")
		return nil
	}

	// Commit
	if _, err := f.gitExec("commit", "-m", message); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	// Push
	if output, err := f.gitExec("push", "origin", "develop"); err != nil {
		return fmt.Errorf("git push failed: %s: %w", output, err)
	}

	f.t.Logf("Committed and pushed: %s", message)
	return nil
}

// kubectlExec runs a kubectl command
func (f *GitOpsTestFixture) kubectlExec(args ...string) (string, error) {
	fullArgs := append([]string{"--kubeconfig=" + f.kubeconfig}, args...)
	cmd := exec.Command("kubectl", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// AddModelConfig adds the test model YAML to ai-aas-config
func (f *GitOpsTestFixture) AddModelConfig() error {
	modelFile := f.ModelFilePath()
	content := tinyllamaModelYAML(f.uniqueSuffix)

	if err := os.WriteFile(modelFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	return f.gitCommitAndPush(fmt.Sprintf("test: add %s for UC-MLC-010", f.modelFileName))
}

// RemoveModelConfig removes the test model YAML from ai-aas-config
func (f *GitOpsTestFixture) RemoveModelConfig() error {
	modelFile := f.ModelFilePath()

	if err := os.Remove(modelFile); err != nil {
		return fmt.Errorf("failed to remove model file: %w", err)
	}

	return f.gitCommitAndPush(fmt.Sprintf("test: remove %s for UC-MLC-011", f.modelFileName))
}

// WaitForAIModelCreated waits for the AIModel CR to be created
func (f *GitOpsTestFixture) WaitForAIModelCreated(ctx context.Context) error {
	modelName := f.ModelName()
	f.t.Logf("Waiting for AIModel %s to be created...", modelName)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for AIModel %s to be created", modelName)
		case <-ticker.C:
			output, err := f.kubectlExec("get", "aimodel", modelName, "-n", testModelNamespace, "-o", "name")
			if err == nil && strings.Contains(output, modelName) {
				f.t.Logf("AIModel %s created", modelName)
				return nil
			}
			f.t.Logf("AIModel not yet created, waiting...")
		}
	}
}

// WaitForAIModelReady waits for the AIModel to reach Ready phase
func (f *GitOpsTestFixture) WaitForAIModelReady(ctx context.Context) error {
	modelName := f.ModelName()
	f.t.Logf("Waiting for AIModel %s to reach Ready phase...", modelName)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Get current status for debugging
			status, _ := f.kubectlExec("get", "aimodel", modelName, "-n", testModelNamespace,
				"-o", "jsonpath={.status.phase} - {.status.message}")
			return fmt.Errorf("timeout waiting for AIModel %s to be Ready, current status: %s", modelName, status)
		case <-ticker.C:
			output, err := f.kubectlExec("get", "aimodel", modelName, "-n", testModelNamespace,
				"-o", "jsonpath={.status.phase}")
			if err == nil {
				phase := strings.TrimSpace(output)
				f.t.Logf("AIModel phase: %s", phase)
				if phase == "Ready" {
					return nil
				}
				if phase == "Failed" {
					msg, _ := f.kubectlExec("get", "aimodel", modelName, "-n", testModelNamespace,
						"-o", "jsonpath={.status.message}")
					return fmt.Errorf("AIModel %s failed: %s", modelName, msg)
				}
			}
		}
	}
}

// WaitForAIModelDeleted waits for the AIModel CR to be deleted
func (f *GitOpsTestFixture) WaitForAIModelDeleted(ctx context.Context) error {
	modelName := f.ModelName()
	f.t.Logf("Waiting for AIModel %s to be deleted...", modelName)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for AIModel %s to be deleted", modelName)
		case <-ticker.C:
			_, err := f.kubectlExec("get", "aimodel", modelName, "-n", testModelNamespace)
			if err != nil && strings.Contains(err.Error(), "NotFound") {
				f.t.Logf("AIModel %s deleted", modelName)
				return nil
			}
			// Also check output for "not found"
			output, _ := f.kubectlExec("get", "aimodel", modelName, "-n", testModelNamespace, "-o", "name")
			if strings.Contains(strings.ToLower(output), "not found") || output == "" {
				f.t.Logf("AIModel %s deleted", modelName)
				return nil
			}
			f.t.Logf("AIModel still exists, waiting for deletion...")
		}
	}
}

// VerifyNoOrphanedResources checks that no orphaned resources remain
func (f *GitOpsTestFixture) VerifyNoOrphanedResources() error {
	modelName := f.ModelName()

	// Check InferenceService
	output, _ := f.kubectlExec("get", "inferenceservice", modelName, "-n", testModelNamespace, "-o", "name")
	if strings.TrimSpace(output) != "" && !strings.Contains(strings.ToLower(output), "not found") {
		return fmt.Errorf("orphaned InferenceService found: %s", modelName)
	}

	// Check for pods with the model label
	output, _ = f.kubectlExec("get", "pods", "-n", testModelNamespace,
		"-l", fmt.Sprintf("serving.kserve.io/inferenceservice=%s", modelName), "-o", "name")
	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("orphaned pods found for model %s: %s", modelName, output)
	}

	f.t.Log("No orphaned resources found")
	return nil
}

// VerifyModelNotAccessible checks that the model is no longer accessible via inference
func (f *GitOpsTestFixture) VerifyModelNotAccessible() error {
	// Get API Router URL from environment
	apiRouterURL := os.Getenv("AI_AAS_API_ROUTER_URL")
	if apiRouterURL == "" {
		f.t.Skip("AI_AAS_API_ROUTER_URL not set - cannot test inference endpoint")
		return nil
	}

	modelName := f.ModelName()

	// Create test client
	client := NewTestClient(apiRouterURL, "")

	// Attempt inference request
	// The model name should map to an inference endpoint like /v1/chat/completions
	// We expect a 404 or similar error indicating the model is not found
	inferenceRequest := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
	}

	resp, err := client.POST("/v1/chat/completions", inferenceRequest)

	// We expect either:
	// 1. HTTP error (connection refused, DNS failure) - model service is gone
	// 2. 404 Not Found - routing knows model is gone
	// 3. 503 Service Unavailable - service exists but model not ready

	if err != nil {
		// Network error is acceptable - service may be completely gone
		f.t.Logf("Inference request failed as expected (network error): %v", err)
		return nil
	}

	// Check status code
	if resp.StatusCode == http.StatusNotFound ||
	   resp.StatusCode == http.StatusServiceUnavailable ||
	   resp.StatusCode == http.StatusBadGateway {
		f.t.Logf("Inference request returned expected error status: %d", resp.StatusCode)
		return nil
	}

	// If we get a 200 OK, that's a problem - model should not be accessible
	if resp.StatusCode == http.StatusOK {
		return fmt.Errorf("model %s is still accessible (HTTP 200), expected 404/503/502 or network error", modelName)
	}

	// Any other error status is also acceptable
	f.t.Logf("Inference request returned error status: %d (acceptable)", resp.StatusCode)
	return nil
}

// TestUC_MLC_010_DeployModelViaGitOps validates UC-MLC-010
// Spec: usecases/model-lifecycle.yaml
//
// Models are deployed by adding YAML to ai-aas-config repo and letting
// ArgoCD sync create the AIModel CR.
func TestUC_MLC_010_DeployModelViaGitOps(t *testing.T) {
	if os.Getenv("RUN_GITOPS_TESTS") != "1" {
		t.Skip("Skipping GitOps tests. Set RUN_GITOPS_TESTS=1 to run.")
	}

	fixture := NewGitOpsTestFixture(t)

	t.Run("AC-01: AIModel config added to ai-aas-config creates deployment", func(t *testing.T) {
		// Given: ai-aas-config/environments/development/models/ is synced by ArgoCD
		// When: Operator commits tinyllama-test.yaml to ai-aas-config develop branch
		err := fixture.AddModelConfig()
		require.NoError(t, err, "Failed to add model config to ai-aas-config")

		// Then: ArgoCD detects the change and creates AIModel CR
		ctx, cancel := context.WithTimeout(context.Background(), argoCDSyncTimeout)
		defer cancel()

		err = fixture.WaitForAIModelCreated(ctx)
		require.NoError(t, err, "AIModel was not created by ArgoCD sync")

		// Verify AIModel matches committed spec
		output, err := fixture.kubectlExec("get", "aimodel", fixture.ModelName(), "-n", testModelNamespace,
			"-o", "jsonpath={.spec.modelID}")
		require.NoError(t, err)
		assert.Equal(t, testModelID, strings.TrimSpace(output), "AIModel modelID should match")
	})

	t.Run("AC-02: Deployed model reaches Ready phase", func(t *testing.T) {
		// Given: AIModel CR exists from GitOps sync
		// When: Operator waits for model to deploy
		ctx, cancel := context.WithTimeout(context.Background(), modelDeployTimeout)
		defer cancel()

		err := fixture.WaitForAIModelReady(ctx)
		require.NoError(t, err, "AIModel did not reach Ready phase")

		// Then: InferenceService is created
		output, err := fixture.kubectlExec("get", "inferenceservice", fixture.ModelName(), "-n", testModelNamespace, "-o", "name")
		require.NoError(t, err, "InferenceService should exist")
		assert.Contains(t, output, fixture.ModelName(), "InferenceService name should match")

		// Verify pod is running
		output, err = fixture.kubectlExec("get", "pods", "-n", testModelNamespace,
			"-l", fmt.Sprintf("serving.kserve.io/inferenceservice=%s", fixture.ModelName()),
			"-o", "jsonpath={.items[0].status.phase}")
		require.NoError(t, err)
		assert.Equal(t, "Running", strings.TrimSpace(output), "Pod should be running")
	})
}

// TestUC_MLC_011_RemoveModelViaGitOps validates UC-MLC-011
// Spec: usecases/model-lifecycle.yaml
//
// Removing model config from ai-aas-config triggers ArgoCD prune and cleanup.
func TestUC_MLC_011_RemoveModelViaGitOps(t *testing.T) {
	if os.Getenv("RUN_GITOPS_TESTS") != "1" {
		t.Skip("Skipping GitOps tests. Set RUN_GITOPS_TESTS=1 to run.")
	}

	fixture := NewGitOpsTestFixture(t)

	// Setup: Deploy a model first (prerequisite)
	t.Log("Setup: Deploying test model via GitOps")
	err := fixture.AddModelConfig()
	require.NoError(t, err, "Failed to add model config for setup")

	ctx, cancel := context.WithTimeout(context.Background(), modelDeployTimeout)
	err = fixture.WaitForAIModelReady(ctx)
	cancel()
	require.NoError(t, err, "Model did not reach Ready phase during setup")

	t.Run("AC-01: Removing config from ai-aas-config triggers deletion", func(t *testing.T) {
		// Given: tinyllama-test.yaml exists in ai-aas-config and model is Ready
		// When: Operator deletes tinyllama-test.yaml and pushes to develop branch
		err := fixture.RemoveModelConfig()
		require.NoError(t, err, "Failed to remove model config from ai-aas-config")

		// Then: ArgoCD prunes the AIModel CR
		ctx, cancel := context.WithTimeout(context.Background(), argoCDSyncTimeout+modelCleanupTimeout)
		defer cancel()

		err = fixture.WaitForAIModelDeleted(ctx)
		require.NoError(t, err, "AIModel was not deleted by ArgoCD prune")
	})

	t.Run("AC-02: Operator cleans up all resources", func(t *testing.T) {
		// Give operator time to clean up
		time.Sleep(30 * time.Second)

		// Then: No orphaned resources remain
		err := fixture.VerifyNoOrphanedResources()
		require.NoError(t, err, "Orphaned resources found after cleanup")
	})

	t.Run("AC-03: Model no longer accessible via inference", func(t *testing.T) {
		// Given: All resources are cleaned up
		// When: Client attempts inference request to the deleted model
		err := fixture.VerifyModelNotAccessible()

		// Then: Request returns 404 or model not found error
		require.NoError(t, err, "Model should not be accessible after cleanup")
	})
}

// TestUC_MLC_010_AC03_NonGitOpsCreationBlocked validates that direct AIModel creation is blocked
// This test requires the Kyverno policy to be deployed.
func TestUC_MLC_010_AC03_NonGitOpsCreationBlocked(t *testing.T) {
	if os.Getenv("RUN_GITOPS_TESTS") != "1" {
		t.Skip("Skipping GitOps tests. Set RUN_GITOPS_TESTS=1 to run.")
	}

	if os.Getenv("KYVERNO_POLICY_DEPLOYED") != "1" {
		t.Skip("Skipping Kyverno policy test. Set KYVERNO_POLICY_DEPLOYED=1 when policy is active.")
	}

	fixture := NewGitOpsTestFixture(t)

	t.Run("AC-03: Non-GitOps AIModel creation is blocked", func(t *testing.T) {
		// Given: Kyverno policy is active
		// When: Operator attempts to create AIModel via kubectl apply
		modelYAML := tinyllamaModelYAML(fixture.uniqueSuffix)
		tmpFile := filepath.Join(os.TempDir(), "test-aimodel.yaml")
		err := os.WriteFile(tmpFile, []byte(modelYAML), 0644)
		require.NoError(t, err)
		defer os.Remove(tmpFile)

		output, err := fixture.kubectlExec("apply", "-f", tmpFile)

		// Then: Kyverno rejects the request
		require.Error(t, err, "kubectl apply should have been rejected")
		assert.Contains(t, strings.ToLower(output), "gitops", "Error should mention GitOps policy")
	})
}
