// Package model provides tests for model enable and disable commands.
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewEnableCommand verifies enable command structure
func TestNewEnableCommand(t *testing.T) {
	cmd := NewEnableCommand()
	require.NotNil(t, cmd, "NewEnableCommand() returned nil")

	assert.Equal(t, "enable", cmd.Use[:6], "command Use should start with 'enable'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "RunE function should be set")

	// Verify Args validator is set (requires at least 1 arg)
	assert.NotNil(t, cmd.Args, "Args validator should be set")
}

// TestEnableCommand_Flags verifies enable command flags
func TestEnableCommand_Flags(t *testing.T) {
	cmd := NewEnableCommand()
	require.NotNil(t, cmd)

	flagTests := []struct {
		name     string
		required bool
	}{
		{"environment", true},
		{"gpu-count", false},
		{"memory", false},
		{"wait", false},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestEnableCommand_EnvironmentFlag verifies environment flag is required
func TestEnableCommand_EnvironmentFlag(t *testing.T) {
	cmd := NewEnableCommand()
	require.NotNil(t, cmd)

	envFlag := cmd.Flags().Lookup("environment")
	require.NotNil(t, envFlag, "environment flag should exist")
	assert.Equal(t, "e", envFlag.Shorthand, "environment flag shorthand should be 'e'")
	assert.NotEmpty(t, envFlag.Usage, "environment flag should have usage description")
}

// TestEnableCommand_Examples verifies usage examples
func TestEnableCommand_Examples(t *testing.T) {
	cmd := NewEnableCommand()
	require.NotNil(t, cmd)

	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, cmd.Long, "model enable", "Examples should show enable command")
	assert.Contains(t, cmd.Long, "-e development", "Examples should show environment flag")
}

// TestEnableCommand_GPUFlags verifies GPU-related flags
func TestEnableCommand_GPUFlags(t *testing.T) {
	cmd := NewEnableCommand()
	require.NotNil(t, cmd)

	gpuCountFlag := cmd.Flags().Lookup("gpu-count")
	assert.NotNil(t, gpuCountFlag, "gpu-count flag should exist")
	assert.Equal(t, "1", gpuCountFlag.DefValue, "gpu-count default should be 1")

	memoryFlag := cmd.Flags().Lookup("memory")
	assert.NotNil(t, memoryFlag, "memory flag should exist")
	assert.Equal(t, "24", memoryFlag.DefValue, "memory default should be 24")
}

// TestNewDisableCommand verifies disable command structure
func TestNewDisableCommand(t *testing.T) {
	cmd := NewDisableCommand()
	require.NotNil(t, cmd, "NewDisableCommand() returned nil")

	assert.Equal(t, "disable", cmd.Use[:7], "command Use should start with 'disable'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "RunE function should be set")

	// Verify Args validator is set (requires exactly 1 arg)
	assert.NotNil(t, cmd.Args, "Args validator should be set")
}

// TestDisableCommand_Flags verifies disable command flags
func TestDisableCommand_Flags(t *testing.T) {
	cmd := NewDisableCommand()
	require.NotNil(t, cmd)

	flagTests := []struct {
		name     string
		required bool
	}{
		{"environment", true},
		{"force", false},
		{"reason", false},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestDisableCommand_EnvironmentFlag verifies environment flag is required
func TestDisableCommand_EnvironmentFlag(t *testing.T) {
	cmd := NewDisableCommand()
	require.NotNil(t, cmd)

	envFlag := cmd.Flags().Lookup("environment")
	require.NotNil(t, envFlag, "environment flag should exist")
	assert.Equal(t, "e", envFlag.Shorthand, "environment flag shorthand should be 'e'")
	assert.NotEmpty(t, envFlag.Usage, "environment flag should have usage description")
}

// TestDisableCommand_Examples verifies usage examples
func TestDisableCommand_Examples(t *testing.T) {
	cmd := NewDisableCommand()
	require.NotNil(t, cmd)

	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, cmd.Long, "model disable", "Examples should show disable command")
	assert.Contains(t, cmd.Long, "-e development", "Examples should show environment flag")
}

// TestDisableCommand_ForceFlag verifies force flag exists and defaults to false
func TestDisableCommand_ForceFlag(t *testing.T) {
	cmd := NewDisableCommand()
	require.NotNil(t, cmd)

	forceFlag := cmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should exist")
	assert.Equal(t, "false", forceFlag.DefValue, "force default should be false")
}

// TestDisableCommand_ReasonFlag verifies reason flag exists
func TestDisableCommand_ReasonFlag(t *testing.T) {
	cmd := NewDisableCommand()
	require.NotNil(t, cmd)

	reasonFlag := cmd.Flags().Lookup("reason")
	assert.NotNil(t, reasonFlag, "reason flag should exist")
	assert.NotEmpty(t, reasonFlag.Usage, "reason flag should have usage description")
}
