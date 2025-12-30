// Package model provides tests for model registry commands.
package model

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRegistryCommand verifies the parent registry command structure
func TestNewRegistryCommand(t *testing.T) {
	cmd := NewRegistryCommand()
	require.NotNil(t, cmd, "NewRegistryCommand() returned nil")

	assert.Equal(t, "registry", cmd.Use, "command Use should be 'registry'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.Run, "Run function should be set (shows help)")

	// Verify subcommands exist
	subcommands := []string{"add", "list", "info", "remove", "rename", "status"}
	for _, subcmd := range subcommands {
		found := false
		for _, c := range cmd.Commands() {
			// Commands may have different Use patterns like "add <hf-model-id>", "status [model-name]", etc.
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestRegistryCommand_Examples verifies that usage examples are provided
func TestRegistryCommand_Examples(t *testing.T) {
	cmd := NewRegistryCommand()
	require.NotNil(t, cmd)

	// Parent command should have examples in Long description
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples section")
	assert.Contains(t, cmd.Long, "registry add", "Examples should show add command")
	assert.Contains(t, cmd.Long, "registry list", "Examples should show list command")
}

// TestRegistryAddCommand_Flags verifies registry add command flags
func TestRegistryAddCommand_Flags(t *testing.T) {
	parent := NewRegistryCommand()
	require.NotNil(t, parent)

	// Find the add subcommand
	var addCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "add" {
			addCmd = cmd
			break
		}
	}
	require.NotNil(t, addCmd, "add subcommand should exist")

	// Verify required flags
	flagTests := []struct {
		name         string
		expectedType string
		required     bool
	}{
		{"model-name", "string", false},
		{"requires-auth", "bool", false},
		{"license", "string", false},
		{"accept-license", "bool", false},
		{"gpu-memory", "int", false},
		{"cpu-memory", "int", false},
		{"model-type", "string", false},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := addCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestRegistryAddCommand_Args verifies argument requirements
func TestRegistryAddCommand_Args(t *testing.T) {
	parent := NewRegistryCommand()
	var addCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "add" {
			addCmd = cmd
			break
		}
	}
	require.NotNil(t, addCmd)

	// Verify Args validator is set (should require exactly 1 arg: hf-model-id)
	assert.NotNil(t, addCmd.Args, "Args validator should be set")
}

// TestRegistryListCommand verifies list command structure
func TestRegistryListCommand(t *testing.T) {
	parent := NewRegistryCommand()
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

	// Verify format flag exists
	formatFlag := listCmd.Flags().Lookup("format")
	if formatFlag != nil {
		// If format flag exists, verify default
		assert.Contains(t, []string{"table", "json", "yaml"}, formatFlag.DefValue,
			"format flag default should be one of: table, json, yaml")
	}
}

// TestRegistryInfoCommand verifies info command structure
func TestRegistryInfoCommand(t *testing.T) {
	parent := NewRegistryCommand()
	var infoCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "info" {
			infoCmd = cmd
			break
		}
	}
	require.NotNil(t, infoCmd, "info subcommand should exist")

	assert.NotEmpty(t, infoCmd.Short, "Short description should not be empty")
	assert.NotNil(t, infoCmd.Args, "Args validator should be set")
}

// TestRegistryRemoveCommand verifies remove command structure
func TestRegistryRemoveCommand(t *testing.T) {
	parent := NewRegistryCommand()
	var removeCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "remove" {
			removeCmd = cmd
			break
		}
	}
	require.NotNil(t, removeCmd, "remove subcommand should exist")

	assert.NotEmpty(t, removeCmd.Short, "Short description should not be empty")
	assert.NotNil(t, removeCmd.Args, "Args validator should be set")
}

// TestRegistryRenameCommand verifies rename command structure
func TestRegistryRenameCommand(t *testing.T) {
	parent := NewRegistryCommand()
	var renameCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "rename" {
			renameCmd = cmd
			break
		}
	}
	require.NotNil(t, renameCmd, "rename subcommand should exist")

	assert.NotEmpty(t, renameCmd.Short, "Short description should not be empty")
	assert.NotNil(t, renameCmd.Args, "Args validator should be set")
}

// TestRegistryStatusCommand verifies status command structure
func TestRegistryStatusCommand(t *testing.T) {
	parent := NewRegistryCommand()
	var statusCmd *cobra.Command
	for _, cmd := range parent.Commands() {
		if cmd.Name() == "status" {
			statusCmd = cmd
			break
		}
	}
	require.NotNil(t, statusCmd, "status subcommand should exist")

	assert.NotEmpty(t, statusCmd.Short, "Short description should not be empty")
	// Status command takes optional argument [model-name], so Args may be nil or custom validator
}

// Note: Functional tests for registry commands require refactoring
// Current implementation uses config.GetEffectiveConfig directly in RunE functions
// making it difficult to inject mock clients for testing.
//
// To add functional tests:
// 1. Extract business logic into testable functions
// 2. Use dependency injection for API clients
// 3. Create mock implementations of registry.Client interface
//
// See cmd/model/recipe/show_test.go for an example of proper dependency injection
