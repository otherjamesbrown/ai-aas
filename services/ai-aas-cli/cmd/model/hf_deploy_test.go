// Package model provides tests for HuggingFace deploy command.
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHFDeployCommand verifies hf-deploy command structure
func TestNewHFDeployCommand(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd, "NewHFDeployCommand() returned nil")

	assert.Equal(t, "hf-deploy", cmd.Use[:9], "command Use should start with 'hf-deploy'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "RunE function should be set")

	// Verify Args validator is set (requires exactly 1 arg)
	assert.NotNil(t, cmd.Args, "Args validator should be set")
}

// TestHFDeployCommand_Flags verifies hf-deploy command flags
func TestHFDeployCommand_Flags(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	flagTests := []struct {
		name         string
		required     bool
		defaultValue string
	}{
		{"model-name", false, ""},
		{"environment", false, "development"},
		{"gpu-count", false, "1"},
		{"memory", false, "24"},
		{"min-replicas", false, "1"},
		{"max-replicas", false, "1"},
		{"accept-license", false, "false"},
		{"skip-cache", false, "false"},
		{"wait", false, "true"},
		{"timeout", false, "15m0s"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q default should be %q", tt.name, tt.defaultValue)
			}
		})
	}
}

// TestHFDeployCommand_EnvironmentFlag verifies environment flag with shorthand
func TestHFDeployCommand_EnvironmentFlag(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	envFlag := cmd.Flags().Lookup("environment")
	require.NotNil(t, envFlag, "environment flag should exist")
	assert.Equal(t, "e", envFlag.Shorthand, "environment flag shorthand should be 'e'")
	assert.Equal(t, "development", envFlag.DefValue, "environment default should be 'development'")
}

// TestHFDeployCommand_Examples verifies usage examples
func TestHFDeployCommand_Examples(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, cmd.Long, "hf-deploy", "Examples should show hf-deploy command")
	assert.Contains(t, cmd.Long, "huggingface.co", "Examples should show HuggingFace URL")
}

// TestHFDeployCommand_WorkflowDescription verifies command describes full workflow
func TestHFDeployCommand_WorkflowDescription(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	// Verify long description explains the full workflow
	assert.Contains(t, cmd.Long, "Register", "Long description should mention registration")
	assert.Contains(t, cmd.Long, "Download", "Long description should mention download/cache")
	assert.Contains(t, cmd.Long, "Deploy", "Long description should mention deployment")
	assert.Contains(t, cmd.Long, "Wait for ready", "Long description should mention waiting")
}

// TestHFDeployCommand_GPUFlags verifies GPU-related flags
func TestHFDeployCommand_GPUFlags(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	gpuCountFlag := cmd.Flags().Lookup("gpu-count")
	assert.NotNil(t, gpuCountFlag, "gpu-count flag should exist")
	assert.Equal(t, "1", gpuCountFlag.DefValue, "gpu-count default should be 1")

	memoryFlag := cmd.Flags().Lookup("memory")
	assert.NotNil(t, memoryFlag, "memory flag should exist")
	assert.Equal(t, "24", memoryFlag.DefValue, "memory default should be 24")
}

// TestHFDeployCommand_ScalingFlags verifies scaling-related flags
func TestHFDeployCommand_ScalingFlags(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	minReplicasFlag := cmd.Flags().Lookup("min-replicas")
	assert.NotNil(t, minReplicasFlag, "min-replicas flag should exist")
	assert.Equal(t, "1", minReplicasFlag.DefValue, "min-replicas default should be 1")

	maxReplicasFlag := cmd.Flags().Lookup("max-replicas")
	assert.NotNil(t, maxReplicasFlag, "max-replicas flag should exist")
	assert.Equal(t, "1", maxReplicasFlag.DefValue, "max-replicas default should be 1")
}

// TestHFDeployCommand_AcceptLicenseFlag verifies accept-license flag
func TestHFDeployCommand_AcceptLicenseFlag(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	acceptLicenseFlag := cmd.Flags().Lookup("accept-license")
	assert.NotNil(t, acceptLicenseFlag, "accept-license flag should exist")
	assert.Equal(t, "false", acceptLicenseFlag.DefValue, "accept-license default should be false")
	assert.Contains(t, acceptLicenseFlag.Usage, "gated", "accept-license usage should mention gated models")
}

// TestHFDeployCommand_SkipCacheFlag verifies skip-cache flag
func TestHFDeployCommand_SkipCacheFlag(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	skipCacheFlag := cmd.Flags().Lookup("skip-cache")
	assert.NotNil(t, skipCacheFlag, "skip-cache flag should exist")
	assert.Equal(t, "false", skipCacheFlag.DefValue, "skip-cache default should be false")
}

// TestHFDeployCommand_WaitFlags verifies wait and timeout flags
func TestHFDeployCommand_WaitFlags(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	waitFlag := cmd.Flags().Lookup("wait")
	assert.NotNil(t, waitFlag, "wait flag should exist")
	assert.Equal(t, "true", waitFlag.DefValue, "wait default should be true")

	timeoutFlag := cmd.Flags().Lookup("timeout")
	assert.NotNil(t, timeoutFlag, "timeout flag should exist")
	assert.Equal(t, "15m0s", timeoutFlag.DefValue, "timeout default should be 15m0s")
}

// TestHFDeployCommand_ModelNameFlag verifies model-name flag is optional
func TestHFDeployCommand_ModelNameFlag(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	modelNameFlag := cmd.Flags().Lookup("model-name")
	assert.NotNil(t, modelNameFlag, "model-name flag should exist")
	assert.Empty(t, modelNameFlag.DefValue, "model-name should default to empty (derived from URL)")
}

// TestHFDeployCommand_SeeAlso verifies command includes related command references
func TestHFDeployCommand_SeeAlso(t *testing.T) {
	cmd := NewHFDeployCommand()
	require.NotNil(t, cmd)

	// Verify long description includes references to related commands
	assert.Contains(t, cmd.Long, "See Also:", "Long description should contain 'See Also' section")
	assert.Contains(t, cmd.Long, "registry list", "Should reference registry list command")
	assert.Contains(t, cmd.Long, "deploy status", "Should reference deploy status command")
}
