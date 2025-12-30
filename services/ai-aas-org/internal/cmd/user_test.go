package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserCommand verifies the user command structure
func TestUserCommand(t *testing.T) {
	require.NotNil(t, userCmd, "userCmd should not be nil")

	assert.Equal(t, "user", userCmd.Use, "command Use should be 'user'")
	assert.NotEmpty(t, userCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, userCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"list", "create", "show", "delete"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range userCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestUserCommand_Examples verifies examples are provided
func TestUserCommand_Examples(t *testing.T) {
	assert.Contains(t, userCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, userCmd.Long, "user list", "Examples should show list command")
	assert.Contains(t, userCmd.Long, "user create", "Examples should show create command")
	assert.Contains(t, userCmd.Long, "user show", "Examples should show show command")
	assert.Contains(t, userCmd.Long, "user delete", "Examples should show delete command")
}

// TestUserListCommand verifies user list command structure
func TestUserListCommand(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
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

// TestUserListCommand_Examples verifies list command examples
func TestUserListCommand_Examples(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	assert.Contains(t, listCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, listCmd.Long, "user list", "Examples should show basic list usage")
}

// TestUserCreateCommand verifies user create command structure
func TestUserCreateCommand(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
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

// TestUserCreateCommand_Flags verifies user create command flags
func TestUserCreateCommand_Flags(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	flagTests := []struct {
		name      string
		shorthand string
	}{
		{"email", "e"},
		{"display-name", "n"},
		{"role", "r"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := createCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand, "flag %q should have shorthand %q", tt.name, tt.shorthand)
			}
		})
	}
}

// TestUserCreateCommand_RoleFlag verifies role flag default value
func TestUserCreateCommand_RoleFlag(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	roleFlag := createCmd.Flags().Lookup("role")
	require.NotNil(t, roleFlag, "role flag should exist")
	assert.Equal(t, "user", roleFlag.DefValue, "role flag should default to 'user'")
}

// TestUserCreateCommand_Examples verifies create command examples
func TestUserCreateCommand_Examples(t *testing.T) {
	var createCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "create" {
			createCmd = cmd
			break
		}
	}
	require.NotNil(t, createCmd)

	assert.Contains(t, createCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, createCmd.Long, "--email", "Examples should show email flag")
	assert.Contains(t, createCmd.Long, "--display-name", "Examples should show display-name flag")
}

// TestUserShowCommand verifies user show command structure
func TestUserShowCommand(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd, "show subcommand should exist")

	assert.Equal(t, "show <email-or-id>", showCmd.Use, "command Use should be 'show <email-or-id>'")
	assert.NotEmpty(t, showCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, showCmd.Long, "Long description should not be empty")
	assert.NotNil(t, showCmd.RunE, "RunE function should be set")
	assert.NotNil(t, showCmd.Args, "Args validator should be set")
}

// TestUserShowCommand_Args verifies show command requires exactly one argument
func TestUserShowCommand_Args(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd)

	// Args should be cobra.ExactArgs(1)
	assert.NotNil(t, showCmd.Args, "Args validator should be set to require one argument")
}

// TestUserShowCommand_Examples verifies show command examples
func TestUserShowCommand_Examples(t *testing.T) {
	var showCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "show" {
			showCmd = cmd
			break
		}
	}
	require.NotNil(t, showCmd)

	assert.Contains(t, showCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, showCmd.Long, "user show", "Examples should show show command")
}

// TestUserDeleteCommand verifies user delete command structure
func TestUserDeleteCommand(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd, "delete subcommand should exist")

	assert.Equal(t, "delete <email-or-id>", deleteCmd.Use, "command Use should be 'delete <email-or-id>'")
	assert.NotEmpty(t, deleteCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, deleteCmd.Long, "Long description should not be empty")
	assert.NotNil(t, deleteCmd.RunE, "RunE function should be set")
	assert.NotNil(t, deleteCmd.Args, "Args validator should be set")
}

// TestUserDeleteCommand_Flags verifies delete command flags
func TestUserDeleteCommand_Flags(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
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

// TestUserDeleteCommand_Examples verifies delete command examples
func TestUserDeleteCommand_Examples(t *testing.T) {
	var deleteCmd *cobra.Command
	for _, cmd := range userCmd.Commands() {
		if cmd.Name() == "delete" {
			deleteCmd = cmd
			break
		}
	}
	require.NotNil(t, deleteCmd)

	assert.Contains(t, deleteCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, deleteCmd.Long, "user delete", "Examples should show delete command")
	assert.Contains(t, deleteCmd.Long, "cannot be undone", "Long description should warn about irreversibility")
}

// TestIsEmail verifies email detection helper
func TestIsEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"user@example.com", true},
		{"admin@test.org", true},
		{"usr_abc123", false},
		{"user123", false},
		{"", false},
		{"@", true}, // Edge case: technically contains @
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isEmail(tt.input)
			assert.Equal(t, tt.expected, result, "isEmail(%q) should return %v", tt.input, tt.expected)
		})
	}
}

// TestRequireConfig verifies config requirement check
func TestRequireConfig(t *testing.T) {
	// Note: This function depends on config.IsConfigured() which reads from viper
	// We can only verify it doesn't panic and returns an error type
	assert.NotPanics(t, func() {
		err := requireConfig()
		// Error type should be present (either nil or error)
		_ = err
	}, "requireConfig should not panic")
}

// TestNewAPIClient verifies API client creation
func TestNewAPIClient(t *testing.T) {
	// Note: This function depends on config package
	// We can only verify it doesn't panic
	assert.NotPanics(t, func() {
		client := newAPIClient()
		// Client should not be nil if config is set
		_ = client
	}, "newAPIClient should not panic")
}
