// Package model provides tests for model cache commands.
package model

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCacheParentCommand verifies the parent cache command structure
func TestNewCacheParentCommand(t *testing.T) {
	cmd := NewCacheParentCommand()
	require.NotNil(t, cmd, "NewCacheParentCommand() returned nil")

	assert.Equal(t, "cache", cmd.Use, "command Use should be 'cache'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.Run, "Run function should be set (shows help)")

	// Verify subcommands exist
	subcommands := []string{"pull", "list", "info", "delete", "gc"}
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

// TestCacheCommand_Examples verifies that usage examples are provided
func TestCacheCommand_Examples(t *testing.T) {
	cmd := NewCacheParentCommand()
	require.NotNil(t, cmd)

	// Parent command should have examples in Long description
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples section")
	assert.Contains(t, cmd.Long, "cache pull", "Examples should show pull command")
	assert.Contains(t, cmd.Long, "cache list", "Examples should show list command")
	assert.Contains(t, cmd.Long, "Workflow:", "Long description should contain workflow section")
}

// TestCachePullCommand_Flags verifies cache pull command flags
func TestCachePullCommand_Flags(t *testing.T) {
	parent := NewCacheParentCommand()
	require.NotNil(t, parent)

	// Find the pull subcommand
	var pullCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "pull" {
			pullCmd = cmd
			break
		}
	}
	require.NotNil(t, pullCmd, "pull subcommand should exist")

	// Verify expected flags
	flagTests := []struct {
		name         string
		expectedType string
	}{
		{"revision", "string"},
		{"dry-run", "bool"},
		{"skip-verify", "bool"},
		{"local", "bool"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := pullCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestCachePullCommand_Args verifies argument requirements
func TestCachePullCommand_Args(t *testing.T) {
	parent := NewCacheParentCommand()
	var pullCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "pull" {
			pullCmd = cmd
			break
		}
	}
	require.NotNil(t, pullCmd)

	// Verify Args validator is set (should require exactly 1 arg: model-name)
	assert.NotNil(t, pullCmd.Args, "Args validator should be set")
}

// TestCachePullCommand_Examples verifies usage examples
func TestCachePullCommand_Examples(t *testing.T) {
	parent := NewCacheParentCommand()
	var pullCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "pull" {
			pullCmd = cmd
			break
		}
	}
	require.NotNil(t, pullCmd)

	assert.NotEmpty(t, pullCmd.Long, "Long description should not be empty")
	assert.Contains(t, pullCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, pullCmd.Long, "cache pull", "Examples should show pull command")
	assert.Contains(t, pullCmd.Long, "--dry-run", "Examples should show dry-run flag")
	assert.Contains(t, pullCmd.Long, "--local", "Examples should show local flag")
}

// TestCacheListCommand verifies list command structure
func TestCacheListCommand(t *testing.T) {
	parent := NewCacheParentCommand()
	var listCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd, "list subcommand should exist")

	assert.NotEmpty(t, listCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, listCmd.Long, "Long description should not be empty")

	// Verify format flag if it exists
	formatFlag := listCmd.Flags().Lookup("format")
	if formatFlag != nil {
		assert.Contains(t, []string{"table", "json", "yaml"}, formatFlag.DefValue,
			"format flag default should be one of: table, json, yaml")
	}
}

// TestCacheInfoCommand verifies info command structure
func TestCacheInfoCommand(t *testing.T) {
	parent := NewCacheParentCommand()
	var infoCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "info" {
			infoCmd = cmd
			break
		}
	}
	require.NotNil(t, infoCmd, "info subcommand should exist")

	assert.NotEmpty(t, infoCmd.Short, "Short description should not be empty")
	assert.NotNil(t, infoCmd.Args, "Args validator should be set for model-name argument")
}

// TestCacheDeleteCommand verifies delete command structure
func TestCacheDeleteCommand(t *testing.T) {
	parent := NewCacheParentCommand()
	var deleteCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd, "delete subcommand should exist")

	assert.NotEmpty(t, deleteCmd.Short, "Short description should not be empty")
	assert.NotNil(t, deleteCmd.Args, "Args validator should be set for model-name argument")

	// Verify force flag if it exists (common for delete operations)
	forceFlag := deleteCmd.Flags().Lookup("force")
	if forceFlag != nil {
		assert.Equal(t, "bool", forceFlag.Value.Type(), "force flag should be boolean")
	}
}

// TestCacheGCCommand verifies garbage collection command structure
func TestCacheGCCommand(t *testing.T) {
	parent := NewCacheParentCommand()
	var gcCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "gc" {
			gcCmd = cmd
			break
		}
	}
	require.NotNil(t, gcCmd, "gc subcommand should exist")

	assert.NotEmpty(t, gcCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, gcCmd.Long, "Long description should not be empty")
}

// TestCachePullCommand_RevisionDefaultValue verifies revision default
func TestCachePullCommand_RevisionDefaultValue(t *testing.T) {
	parent := NewCacheParentCommand()
	var pullCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "pull" {
			pullCmd = cmd
			break
		}
	}
	require.NotNil(t, pullCmd)

	revisionFlag := pullCmd.Flags().Lookup("revision")
	require.NotNil(t, revisionFlag, "revision flag should exist")

	// Revision defaults to "main"
	assert.Equal(t, "main", revisionFlag.DefValue, "revision flag should default to 'main'")
}

// Note: Functional tests for cache commands require refactoring
// Current implementation uses config.GetEffectiveConfig directly in RunE functions
// making it difficult to inject mock clients for testing.
//
// To add functional tests:
// 1. Extract business logic into testable functions
// 2. Use dependency injection for API clients and storage clients
// 3. Create mock implementations of storage.Client and api.Client interfaces
//
// Key functions to test:
// - runServerSidePull: Server-side model caching via Admin API
// - Local pull logic: Direct download and upload to S3
// - Cache validation and verification
// - Progress reporting and error handling
