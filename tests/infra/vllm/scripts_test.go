package vllm_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScriptsExist verifies that all deployment scripts exist and are executable
func TestScriptsExist(t *testing.T) {
	scripts := []string{
		"../../../scripts/register-model.sh",
		"../../../scripts/rollback-deployment.sh",
		"../../../scripts/promote-deployment.sh",
	}

	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			// Check file exists
			info, err := os.Stat(script)
			require.NoError(t, err, "Script should exist: %s", script)

			// Check it's a regular file
			assert.False(t, info.IsDir(), "Script should be a file, not a directory")

			// Check it's executable
			mode := info.Mode()
			assert.True(t, mode&0111 != 0, "Script should be executable: %s", script)
		})
	}
}

// TestRegisterModelScriptHelp tests the register-model.sh script help output
func TestRegisterModelScriptHelp(t *testing.T) {
	cmd := exec.Command("../../../scripts/register-model.sh", "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Script help should execute successfully")

	outputStr := string(output)

	// Verify help content
	assert.Contains(t, outputStr, "Register vLLM Model Deployment")
	assert.Contains(t, outputStr, "Usage:")
	assert.Contains(t, outputStr, "MODEL_NAME")
	assert.Contains(t, outputStr, "ENVIRONMENT")
	assert.Contains(t, outputStr, "NAMESPACE")
	assert.Contains(t, outputStr, "--skip-wait")
	assert.Contains(t, outputStr, "--skip-health")
	assert.Contains(t, outputStr, "--dry-run")
	assert.Contains(t, outputStr, "DATABASE_URL")
	assert.Contains(t, outputStr, "KUBECONFIG")
}

// TestRollbackDeploymentScriptHelp tests the rollback-deployment.sh script help output
func TestRollbackDeploymentScriptHelp(t *testing.T) {
	cmd := exec.Command("../../../scripts/rollback-deployment.sh", "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Script help should execute successfully")

	outputStr := string(output)

	// Verify help content
	assert.Contains(t, outputStr, "Rollback vLLM Deployment")
	assert.Contains(t, outputStr, "Usage:")
	assert.Contains(t, outputStr, "MODEL_NAME")
	assert.Contains(t, outputStr, "ENVIRONMENT")
	assert.Contains(t, outputStr, "REVISION")
	assert.Contains(t, outputStr, "--skip-disable")
	assert.Contains(t, outputStr, "--skip-enable")
	assert.Contains(t, outputStr, "--skip-wait")
	assert.Contains(t, outputStr, "--force")
	assert.Contains(t, outputStr, "DATABASE_URL")
	assert.Contains(t, outputStr, "KUBECONFIG")
}

// TestPromoteDeploymentScriptHelp tests the promote-deployment.sh script help output
func TestPromoteDeploymentScriptHelp(t *testing.T) {
	cmd := exec.Command("../../../scripts/promote-deployment.sh", "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Script help should execute successfully")

	outputStr := string(output)

	// Verify help content
	assert.Contains(t, outputStr, "Promote vLLM Deployment")
	assert.Contains(t, outputStr, "Usage:")
	assert.Contains(t, outputStr, "MODEL_NAME")
	assert.Contains(t, outputStr, "SOURCE_ENV")
	assert.Contains(t, outputStr, "TARGET_ENV")
	assert.Contains(t, outputStr, "--skip-validation")
	assert.Contains(t, outputStr, "--skip-smoke-tests")
	assert.Contains(t, outputStr, "--force")
	assert.Contains(t, outputStr, "DATABASE_URL")
	assert.Contains(t, outputStr, "KUBECONFIG")
}

// TestRegisterModelScriptPrerequisites tests prerequisite checking
func TestRegisterModelScriptPrerequisites(t *testing.T) {
	// Run without KUBECONFIG set
	cmd := exec.Command("../../../scripts/register-model.sh", "test-model", "development", "system")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")} // Minimal environment
	output, err := cmd.CombinedOutput()

	// Should fail due to missing KUBECONFIG
	assert.Error(t, err, "Script should fail without KUBECONFIG")
	assert.Contains(t, string(output), "KUBECONFIG", "Error should mention missing KUBECONFIG")
}

// TestRollbackDeploymentScriptPrerequisites tests prerequisite checking
func TestRollbackDeploymentScriptPrerequisites(t *testing.T) {
	// Run without KUBECONFIG set
	cmd := exec.Command("../../../scripts/rollback-deployment.sh", "test-model", "development")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")} // Minimal environment
	output, err := cmd.CombinedOutput()

	// Should fail due to missing prerequisites
	assert.Error(t, err, "Script should fail without prerequisites")
	outputStr := string(output)
	assert.True(t,
		strings.Contains(outputStr, "KUBECONFIG") || strings.Contains(outputStr, "DATABASE_URL"),
		"Error should mention missing prerequisites")
}

// TestPromoteDeploymentScriptPrerequisites tests prerequisite checking
func TestPromoteDeploymentScriptPrerequisites(t *testing.T) {
	// Run without KUBECONFIG set
	cmd := exec.Command("../../../scripts/promote-deployment.sh", "test-model", "development", "staging")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")} // Minimal environment
	output, err := cmd.CombinedOutput()

	// Should fail due to missing prerequisites
	assert.Error(t, err, "Script should fail without prerequisites")
	outputStr := string(output)
	assert.True(t,
		strings.Contains(outputStr, "KUBECONFIG") || strings.Contains(outputStr, "DATABASE_URL"),
		"Error should mention missing prerequisites")
}

// TestScriptEnvironmentValidation tests environment validation (development/staging/production)
func TestScriptEnvironmentValidation(t *testing.T) {
	tests := []struct {
		name        string
		script      string
		args        []string
		shouldFail  bool
	}{
		{
			name:       "register-model invalid environment",
			script:     "../../../scripts/register-model.sh",
			args:       []string{"test-model", "invalid-env", "system", "--dry-run"},
			shouldFail: true,
		},
		{
			name:       "rollback-deployment invalid environment",
			script:     "../../../scripts/rollback-deployment.sh",
			args:       []string{"test-model", "invalid-env", "--skip-disable", "--skip-enable", "--skip-wait"},
			shouldFail: true,
		},
		{
			name:       "promote-deployment invalid source environment",
			script:     "../../../scripts/promote-deployment.sh",
			args:       []string{"test-model", "invalid-env", "production", "--skip-validation", "--skip-smoke-tests"},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(tt.script, tt.args...)
			// Set minimal environment to avoid actual operations
			cmd.Env = []string{
				"PATH=" + os.Getenv("PATH"),
				"KUBECONFIG=/dev/null",
				"DATABASE_URL=postgres://invalid:invalid@localhost:5432/invalid",
			}
			output, err := cmd.CombinedOutput()

			if tt.shouldFail {
				assert.Error(t, err, "Script should fail with invalid environment")
				outputStr := string(output)
				assert.Contains(t, outputStr, "Invalid environment", "Error should mention invalid environment")
			}
		})
	}
}

// TestDocumentationExists verifies that documentation files exist
func TestDocumentationExists(t *testing.T) {
	docs := []string{
		"../../../docs/registration-workflow.md",
		"../../../docs/rollback-workflow.md",
		"../../../docs/rollout-workflow.md",
		"../../../docs/environment-separation.md",
	}

	for _, doc := range docs {
		t.Run(doc, func(t *testing.T) {
			info, err := os.Stat(doc)
			require.NoError(t, err, "Documentation should exist: %s", doc)
			assert.False(t, info.IsDir(), "Should be a file, not a directory")
			assert.Greater(t, info.Size(), int64(100), "Documentation should not be empty")
		})
	}
}

// TestEnvironmentValuesFiles verifies that environment-specific values files exist
func TestEnvironmentValuesFiles(t *testing.T) {
	valuesFiles := []string{
		"../../../infra/helm/charts/vllm-deployment/values-staging.yaml",
		"../../../infra/helm/charts/vllm-deployment/values-production.yaml",
	}

	for _, file := range valuesFiles {
		t.Run(file, func(t *testing.T) {
			info, err := os.Stat(file)
			require.NoError(t, err, "Values file should exist: %s", file)
			assert.False(t, info.IsDir(), "Should be a file, not a directory")
			assert.Greater(t, info.Size(), int64(10), "Values file should not be empty")
		})
	}
}
