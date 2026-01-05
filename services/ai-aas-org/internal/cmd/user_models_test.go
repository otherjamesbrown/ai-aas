package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserModelsCommand verifies the user models command structure
func TestUserModelsCommand(t *testing.T) {
	require.NotNil(t, userModelsCmd, "userModelsCmd should not be nil")

	assert.Equal(t, "models", userModelsCmd.Use, "command Use should be 'models'")
	assert.NotEmpty(t, userModelsCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, userModelsCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"list", "add", "remove"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range userModelsCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestUserModelsCommand_Examples verifies examples are provided
func TestUserModelsCommand_Examples(t *testing.T) {
	assert.Contains(t, userModelsCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, userModelsCmd.Long, "user models list", "Examples should show list command")
	assert.Contains(t, userModelsCmd.Long, "user models add", "Examples should show add command")
	assert.Contains(t, userModelsCmd.Long, "user models remove", "Examples should show remove command")
}

// TestUserModelsListCommand verifies user models list command structure
func TestUserModelsListCommand(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range userModelsCmd.Commands() {
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

// TestUserModelsListCommand_Flags verifies list command flags
func TestUserModelsListCommand_Flags(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range userModelsCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	userFlag := listCmd.Flags().Lookup("user")
	assert.NotNil(t, userFlag, "user flag should exist")
}

// TestUserModelsAddCommand verifies user models add command structure
func TestUserModelsAddCommand(t *testing.T) {
	var addCmd *cobra.Command
	for _, cmd := range userModelsCmd.Commands() {
		if cmd.Name() == "add" {
			addCmd = cmd
			break
		}
	}
	require.NotNil(t, addCmd, "add subcommand should exist")

	assert.Equal(t, "add", addCmd.Use, "command Use should be 'add'")
	assert.NotEmpty(t, addCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, addCmd.Long, "Long description should not be empty")
	assert.NotNil(t, addCmd.RunE, "RunE function should be set")
}

// TestUserModelsAddCommand_Flags verifies add command flags
func TestUserModelsAddCommand_Flags(t *testing.T) {
	var addCmd *cobra.Command
	for _, cmd := range userModelsCmd.Commands() {
		if cmd.Name() == "add" {
			addCmd = cmd
			break
		}
	}
	require.NotNil(t, addCmd)

	flagTests := []string{"user", "model"}
	for _, flagName := range flagTests {
		flag := addCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "flag %q should exist", flagName)
	}
}

// TestUserModelsRemoveCommand verifies user models remove command structure
func TestUserModelsRemoveCommand(t *testing.T) {
	var removeCmd *cobra.Command
	for _, cmd := range userModelsCmd.Commands() {
		if cmd.Name() == "remove" {
			removeCmd = cmd
			break
		}
	}
	require.NotNil(t, removeCmd, "remove subcommand should exist")

	assert.Equal(t, "remove", removeCmd.Use, "command Use should be 'remove'")
	assert.NotEmpty(t, removeCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, removeCmd.Long, "Long description should not be empty")
	assert.NotNil(t, removeCmd.RunE, "RunE function should be set")
}

// TestUserModelsRemoveCommand_Flags verifies remove command flags
func TestUserModelsRemoveCommand_Flags(t *testing.T) {
	var removeCmd *cobra.Command
	for _, cmd := range userModelsCmd.Commands() {
		if cmd.Name() == "remove" {
			removeCmd = cmd
			break
		}
	}
	require.NotNil(t, removeCmd)

	flagTests := []string{"user", "model"}
	for _, flagName := range flagTests {
		flag := removeCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "flag %q should exist", flagName)
	}
}
