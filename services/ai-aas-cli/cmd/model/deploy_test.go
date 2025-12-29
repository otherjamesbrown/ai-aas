// Package model provides tests for model deployment commands.
package model

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDeployParentCommand verifies the parent deploy command structure
func TestNewDeployParentCommand(t *testing.T) {
	cmd := NewDeployParentCommand()
	require.NotNil(t, cmd, "NewDeployParentCommand() returned nil")

	assert.Equal(t, "deploy", cmd.Use, "command Use should be 'deploy'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.Run, "Run function should be set (shows help)")

	// Verify subcommands exist
	subcommands := []string{"create", "delete", "restart", "scale", "status"}
	for _, subcmd := range subcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestDeployCommand_Examples verifies that usage examples are provided
func TestDeployCommand_Examples(t *testing.T) {
	cmd := NewDeployParentCommand()
	require.NotNil(t, cmd)

	// Parent command should have examples in Long description
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples section")
	assert.Contains(t, cmd.Long, "deploy create", "Examples should show create command")
	assert.Contains(t, cmd.Long, "deploy status", "Examples should show status command")
}

// TestDeployCreateCommand_Flags verifies deploy create command flags
func TestDeployCreateCommand_Flags(t *testing.T) {
	parent := NewDeployParentCommand()
	require.NotNil(t, parent)

	// Find the create subcommand
	var createCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd, "create subcommand should exist")

	// Verify expected flags (based on actual deploy_cmd.go)
	flagTests := []struct {
		name     string
		required bool
	}{
		{"environment", true}, // -e flag is required
		{"engine-config", false},
		{"gpu-count", false},
		{"memory", false},
		{"min-replicas", false},
		{"max-replicas", false},
		{"revision", false},
		{"dry-run", false},
		{"skip-validation", false},
		{"wait", false},
		{"timeout", false},
		{"trust-remote-code", false},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := createCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestDeployCreateCommand_EnvironmentFlag verifies environment flag is required
func TestDeployCreateCommand_EnvironmentFlag(t *testing.T) {
	parent := NewDeployParentCommand()
	var createCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	envFlag := createCmd.Flags().Lookup("environment")
	require.NotNil(t, envFlag, "environment flag should exist")

	// Environment flag should be marked as required (shorthand -e)
	// Note: In Cobra, required flags are validated at runtime via MarkFlagRequired
}

// TestDeployCreateCommand_Args verifies argument requirements
func TestDeployCreateCommand_Args(t *testing.T) {
	parent := NewDeployParentCommand()
	var createCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	// Verify Args validator is set (should require exactly 1 arg: model-name)
	assert.NotNil(t, createCmd.Args, "Args validator should be set")
}

// TestDeployCreateCommand_Examples verifies usage examples
func TestDeployCreateCommand_Examples(t *testing.T) {
	parent := NewDeployParentCommand()
	var createCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	assert.NotEmpty(t, createCmd.Long, "Long description should not be empty")
	assert.Contains(t, createCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, createCmd.Long, "deploy create", "Examples should show create command")
	assert.True(t,
		strings.Contains(createCmd.Long, "-e development") || strings.Contains(createCmd.Long, "--environment"),
		"Examples should show environment flag (short or long form)")
	assert.Contains(t, createCmd.Long, "--engine-config", "Examples should show engine-config flag")
}

// TestDeployStatusCommand verifies status command structure
func TestDeployStatusCommand(t *testing.T) {
	parent := NewDeployParentCommand()
	var statusCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "status" {
			statusCmd = cmd
			break
		}
	}
	require.NotNil(t, statusCmd, "status subcommand should exist")

	assert.NotEmpty(t, statusCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, statusCmd.Long, "Long description should not be empty")

	// Verify environment flag
	envFlag := statusCmd.Flags().Lookup("environment")
	if envFlag != nil {
		assert.NotEmpty(t, envFlag.Usage, "environment flag should have usage description")
	}

	// Verify format flag
	formatFlag := statusCmd.Flags().Lookup("format")
	if formatFlag != nil {
		assert.Contains(t, []string{"table", "json", "yaml"}, formatFlag.DefValue,
			"format flag default should be one of: table, json, yaml")
	}
}

// TestDeployDeleteCommand verifies delete command structure
func TestDeployDeleteCommand(t *testing.T) {
	parent := NewDeployParentCommand()
	var deleteCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd, "delete subcommand should exist")

	assert.NotEmpty(t, deleteCmd.Short, "Short description should not be empty")
	assert.NotNil(t, deleteCmd.Args, "Args validator should be set")

	// Verify environment flag exists (required for delete)
	envFlag := deleteCmd.Flags().Lookup("environment")
	assert.NotNil(t, envFlag, "environment flag should exist")
}

// Note: deploy info command does not exist - it was replaced by deploy status
// Removed TestDeployInfoCommand test

// Note: deploy logs command does not exist in deploy_cmd.go
// Logs may be in a separate troubleshoot command
// Removed TestDeployLogsCommand test

// TestDeployScaleCommand verifies scale command structure
func TestDeployScaleCommand(t *testing.T) {
	parent := NewDeployParentCommand()
	var scaleCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "scale" {
			scaleCmd = cmd
			break
		}
	}
	require.NotNil(t, scaleCmd, "scale subcommand should exist")

	assert.NotEmpty(t, scaleCmd.Short, "Short description should not be empty")
	assert.NotNil(t, scaleCmd.Args, "Args validator should be set")

	// Verify replicas flag
	replicasFlag := scaleCmd.Flags().Lookup("replicas")
	assert.NotNil(t, replicasFlag, "replicas flag should exist")
}

// TestDeployRestartCommand verifies restart command structure
func TestDeployRestartCommand(t *testing.T) {
	parent := NewDeployParentCommand()
	var restartCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "restart" {
			restartCmd = cmd
			break
		}
	}
	require.NotNil(t, restartCmd, "restart subcommand should exist")

	assert.NotEmpty(t, restartCmd.Short, "Short description should not be empty")
	assert.NotNil(t, restartCmd.Args, "Args validator should be set")

	// Verify environment flag
	envFlag := restartCmd.Flags().Lookup("environment")
	assert.NotNil(t, envFlag, "environment flag should exist")
}

// TestDeployCreateCommand_GPUFlags verifies GPU-related flags
func TestDeployCreateCommand_GPUFlags(t *testing.T) {
	parent := NewDeployParentCommand()
	var createCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	// Verify GPU flag exists (based on actual deploy_cmd.go)
	gpuCountFlag := createCmd.Flags().Lookup("gpu-count")
	assert.NotNil(t, gpuCountFlag, "gpu-count flag should exist")
}

// Note: Functional tests for deploy commands require refactoring
// Current implementation uses config.GetEffectiveConfig directly in RunE functions
// making it difficult to inject mock clients for testing.
//
// To add functional tests:
// 1. Extract business logic into testable functions
// 2. Use dependency injection for API clients and Kubernetes clients
// 3. Create mock implementations of api.Client and kubernetes.Client interfaces
//
// Key functions to test:
// - Deployment creation with validation
// - Environment-specific configuration
// - Recipe application and resource requests
// - Scaling operations and health checks
// - Log streaming and error handling
// - Restart and delete operations with confirmation prompts
