package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModelCommand verifies the model command structure
func TestModelCommand(t *testing.T) {
	require.NotNil(t, modelCmd, "modelCmd should not be nil")

	assert.Equal(t, "model", modelCmd.Use, "command Use should be 'model'")
	assert.NotEmpty(t, modelCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, modelCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"list", "show"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range modelCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestModelCommand_Examples verifies examples are provided
func TestModelCommand_Examples(t *testing.T) {
	assert.Contains(t, modelCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, modelCmd.Long, "model list", "Examples should show list command")
	assert.Contains(t, modelCmd.Long, "model show", "Examples should show show command")
}

// TestModelListCommand verifies model list command structure
func TestModelListCommand(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range modelCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd, "list subcommand should exist")

	assert.Equal(t, "list", listCmd.Use, "command Use should be 'list'")
	assert.NotEmpty(t, listCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, listCmd.Long, "Long description should not be empty")
	assert.NotNil(t, listCmd.RunE, "RunE function should be set")
}

// TestModelListCommand_Examples verifies list command examples
func TestModelListCommand_Examples(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range modelCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	assert.Contains(t, listCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, listCmd.Long, "model list", "Examples should show basic usage")
}

// TestModelShowCommand verifies model show command structure
func TestModelShowCommand(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range modelCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd, "show subcommand should exist")

	assert.Equal(t, "show <model-name>", showCmd.Use, "command Use should be 'show <model-name>'")
	assert.NotEmpty(t, showCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, showCmd.Long, "Long description should not be empty")
	assert.NotNil(t, showCmd.RunE, "RunE function should be set")
	assert.NotNil(t, showCmd.Args, "Args validator should be set")
}

// TestModelShowCommand_Examples verifies show command examples
func TestModelShowCommand_Examples(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range modelCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd)

	assert.Contains(t, showCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, showCmd.Long, "model show", "Examples should show show command")
}
