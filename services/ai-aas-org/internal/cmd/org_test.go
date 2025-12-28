package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrgCommand verifies the org command structure
func TestOrgCommand(t *testing.T) {
	require.NotNil(t, orgCmd, "orgCmd should not be nil")

	assert.Equal(t, "org", orgCmd.Use, "command Use should be 'org'")
	assert.NotEmpty(t, orgCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, orgCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"show"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range orgCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestOrgCommand_Examples verifies examples are provided
func TestOrgCommand_Examples(t *testing.T) {
	assert.Contains(t, orgCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, orgCmd.Long, "org show", "Examples should show show command")
}

// TestOrgShowCommand verifies org show command structure
func TestOrgShowCommand(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range orgCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd, "show subcommand should exist")

	assert.Equal(t, "show", showCmd.Use, "command Use should be 'show'")
	assert.NotEmpty(t, showCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, showCmd.Long, "Long description should not be empty")
	assert.NotNil(t, showCmd.RunE, "RunE function should be set")
}

// TestOrgShowCommand_Examples verifies show command examples
func TestOrgShowCommand_Examples(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range orgCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd)

	assert.Contains(t, showCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, showCmd.Long, "org show", "Examples should show basic usage")
}

// TestOrgShowCommand_Description verifies comprehensive information display
func TestOrgShowCommand_Description(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range orgCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd)

	// Verify that the description mentions what will be displayed
	assert.Contains(t, showCmd.Long, "Organization name", "Long description should mention organization name")
	assert.Contains(t, showCmd.Long, "plan", "Long description should mention plan")
	assert.Contains(t, showCmd.Long, "limits", "Long description should mention limits")
}
