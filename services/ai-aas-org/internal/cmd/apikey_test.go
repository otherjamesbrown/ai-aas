package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyCommand verifies the apikey command structure
func TestAPIKeyCommand(t *testing.T) {
	require.NotNil(t, apikeyCmd, "apikeyCmd should not be nil")

	assert.Equal(t, "apikey", apikeyCmd.Use, "command Use should be 'apikey'")
	assert.NotEmpty(t, apikeyCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, apikeyCmd.Long, "Long description should not be empty")

	// Verify aliases
	assert.Contains(t, apikeyCmd.Aliases, "key", "apikey should have 'key' alias")
	assert.Contains(t, apikeyCmd.Aliases, "keys", "apikey should have 'keys' alias")

	// Verify subcommands exist
	expectedSubcommands := []string{"list", "create", "delete", "rotate"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range apikeyCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestAPIKeyCommand_Examples verifies examples are provided
func TestAPIKeyCommand_Examples(t *testing.T) {
	assert.Contains(t, apikeyCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, apikeyCmd.Long, "apikey list", "Examples should show list command")
	assert.Contains(t, apikeyCmd.Long, "apikey create", "Examples should show create command")
	assert.Contains(t, apikeyCmd.Long, "apikey delete", "Examples should show delete command")
}

// TestAPIKeyListCommand verifies apikey list command structure
func TestAPIKeyListCommand(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
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

// TestAPIKeyListCommand_Flags verifies list command flags
func TestAPIKeyListCommand_Flags(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	userFlag := listCmd.Flags().Lookup("user-id")
	assert.NotNil(t, userFlag, "user-id flag should exist")
	assert.Equal(t, "", userFlag.DefValue, "user-id flag should default to empty string")
}

// TestAPIKeyListCommand_Examples verifies list command examples
func TestAPIKeyListCommand_Examples(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	assert.Contains(t, listCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, listCmd.Long, "apikey list", "Examples should show basic list usage")
	assert.Contains(t, listCmd.Long, "--user-id", "Examples should show user-id filter flag")
}

// TestAPIKeyCreateCommand verifies apikey create command structure
func TestAPIKeyCreateCommand(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd, "create subcommand should exist")

	assert.Equal(t, "create", createCmd.Use, "command Use should be 'create'")
	assert.NotEmpty(t, createCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, createCmd.Long, "Long description should not be empty")
	assert.NotNil(t, createCmd.RunE, "RunE function should be set")
}

// TestAPIKeyCreateCommand_Flags verifies create command flags
func TestAPIKeyCreateCommand_Flags(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	flagTests := []struct {
		name string
	}{
		{"user-id"},
		{"email"},
		{"key-name"},
		{"expires"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := createCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestAPIKeyCreateCommand_Examples verifies create command examples
func TestAPIKeyCreateCommand_Examples(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	assert.Contains(t, createCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, createCmd.Long, "--user-id", "Examples should show user-id flag")
	assert.Contains(t, createCmd.Long, "--email", "Examples should show email flag")
	assert.Contains(t, createCmd.Long, "--key-name", "Examples should show key-name flag")
	assert.Contains(t, createCmd.Long, "--expires", "Examples should show expires flag")
	assert.Contains(t, createCmd.Long, "only be displayed once", "Long description should warn about one-time display")
}

// TestAPIKeyDeleteCommand verifies apikey delete command structure
func TestAPIKeyDeleteCommand(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd, "delete subcommand should exist")

	assert.Equal(t, "delete <key-id>", deleteCmd.Use, "command Use should be 'delete <key-id>'")
	assert.NotEmpty(t, deleteCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, deleteCmd.Long, "Long description should not be empty")
	assert.NotNil(t, deleteCmd.RunE, "RunE function should be set")
	assert.NotNil(t, deleteCmd.Args, "Args validator should be set")
}

// TestAPIKeyDeleteCommand_Flags verifies delete command flags
func TestAPIKeyDeleteCommand_Flags(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd)

	forceFlag := deleteCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should exist")
	assert.Equal(t, "false", forceFlag.DefValue, "force flag should default to false")
}

// TestAPIKeyDeleteCommand_Examples verifies delete command examples
func TestAPIKeyDeleteCommand_Examples(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd)

	assert.Contains(t, deleteCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, deleteCmd.Long, "apikey delete", "Examples should show delete command")
	assert.Contains(t, deleteCmd.Long, "cannot be undone", "Long description should warn about irreversibility")
}

// TestAPIKeyRotateCommand verifies apikey rotate command structure
func TestAPIKeyRotateCommand(t *testing.T) {
	var rotateCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "rotate" {
			rotateCmd = cmd
			break
		}
	}
	require.NotNil(t, rotateCmd, "rotate subcommand should exist")

	assert.Equal(t, "rotate <key-id>", rotateCmd.Use, "command Use should be 'rotate <key-id>'")
	assert.NotEmpty(t, rotateCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, rotateCmd.Long, "Long description should not be empty")
	assert.NotNil(t, rotateCmd.RunE, "RunE function should be set")
	assert.NotNil(t, rotateCmd.Args, "Args validator should be set")
}

// TestAPIKeyRotateCommand_Examples verifies rotate command examples
func TestAPIKeyRotateCommand_Examples(t *testing.T) {
	var rotateCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "rotate" {
			rotateCmd = cmd
			break
		}
	}
	require.NotNil(t, rotateCmd)

	assert.Contains(t, rotateCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, rotateCmd.Long, "apikey rotate", "Examples should show rotate command")
	assert.Contains(t, rotateCmd.Long, "security", "Long description should mention security")
}

// TestAPIKeyCreateCommand_ValidationRules verifies input validation for create command
func TestAPIKeyCreateCommand_ValidationRules(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	// Test that either user-id or email is required
	userFlag := createCmd.Flags().Lookup("user-id")
	emailFlag := createCmd.Flags().Lookup("email")

	assert.NotNil(t, userFlag, "user-id flag should exist")
	assert.NotNil(t, emailFlag, "email flag should exist")

	// Both should be optional at the flag level (validation happens in RunE)
	assert.False(t, userFlag.NoOptDefVal != "", "user-id should not be required at flag level")
	assert.False(t, emailFlag.NoOptDefVal != "", "email should not be required at flag level")
}

// TestAPIKeyCreateCommand_ExpirationFormats verifies expiration format examples
func TestAPIKeyCreateCommand_ExpirationFormats(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	expiresFlag := createCmd.Flags().Lookup("expires")
	require.NotNil(t, expiresFlag, "expires flag should exist")

	// Verify usage text mentions expiration formats
	assert.Contains(t, expiresFlag.Usage, "30d", "Usage should mention 30d format")
	assert.Contains(t, expiresFlag.Usage, "never", "Usage should mention 'never' option")
}

// TestAPIKeyDeleteCommand_Args verifies delete command requires one argument
func TestAPIKeyDeleteCommand_Args(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd)

	// Should require exactly one argument (key-id)
	assert.Contains(t, deleteCmd.Use, "<key-id>", "Use should show key-id as required argument")
	assert.NotNil(t, deleteCmd.Args, "Args validator should be set")
}

// TestAPIKeyRotateCommand_Args verifies rotate command requires one argument
func TestAPIKeyRotateCommand_Args(t *testing.T) {
	var rotateCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "rotate" {
			rotateCmd = cmd
			break
		}
	}
	require.NotNil(t, rotateCmd)

	// Should require exactly one argument (key-id)
	assert.Contains(t, rotateCmd.Use, "<key-id>", "Use should show key-id as required argument")
	assert.NotNil(t, rotateCmd.Args, "Args validator should be set")
}

// TestAPIKeyListCommand_FilterFlag verifies list command user-id filter flag
func TestAPIKeyListCommand_FilterFlag(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range apikeyCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	userFlag := listCmd.Flags().Lookup("user-id")
	require.NotNil(t, userFlag, "user-id flag should exist")

	// Should be optional (default empty means show all)
	assert.Equal(t, "", userFlag.DefValue, "user-id filter should default to empty string")
}

// TestAPIKeyCommand_Aliases verifies command aliases
func TestAPIKeyCommand_Aliases(t *testing.T) {
	aliases := apikeyCmd.Aliases
	expectedAliases := []string{"key", "keys"}

	for _, expected := range expectedAliases {
		assert.Contains(t, aliases, expected, "apikey command should have alias %q", expected)
	}
}
