package admin

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserCommand verifies the user command structure
func TestUserCommand(t *testing.T) {
	cmd := UserCommand()
	require.NotNil(t, cmd, "UserCommand() should not be nil")

	assert.Equal(t, "user", cmd.Use, "command Use should be 'user'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")

	// Verify expected subcommands exist
	expectedSubcommands := []string{"list", "create", "update", "delete", "model-access"}
	for _, subcmdName := range expectedSubcommands {
		found := false
		for _, c := range cmd.Commands() {
			if c.Name() == subcmdName {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmdName)
	}
}

// TestUserCommand_Examples verifies examples are provided
func TestUserCommand_Examples(t *testing.T) {
	cmd := UserCommand()
	require.NotNil(t, cmd)

	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, cmd.Long, "user list", "Examples should show list command")
	assert.Contains(t, cmd.Long, "user create", "Examples should show create command")
	assert.Contains(t, cmd.Long, "user update", "Examples should show update command")
	assert.Contains(t, cmd.Long, "user delete", "Examples should show delete command")
}

// TestUserListCommand verifies user list command structure
func TestUserListCommand(t *testing.T) {
	cmd := UserCommand()
	var listCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "list" {
			listCmd = c
			break
		}
	}
	require.NotNil(t, listCmd, "list subcommand should exist")

	assert.Equal(t, "list", listCmd.Use, "command Use should be 'list'")
	assert.NotEmpty(t, listCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, listCmd.Long, "Long description should not be empty")
	assert.NotNil(t, listCmd.RunE, "RunE function should be set")
}

// TestUserListCommand_Flags verifies list command flags
func TestUserListCommand_Flags(t *testing.T) {
	cmd := UserCommand()
	var listCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "list" {
			listCmd = c
			break
		}
	}
	require.NotNil(t, listCmd)

	flagTests := []struct {
		name         string
		defaultValue string
	}{
		{"org-id", ""},
		{"format", "table"},
		{"verbose", "false"},
		{"quiet", "false"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := listCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q should have default %q", tt.name, tt.defaultValue)
			}
		})
	}
}

// TestUserCreateCommand verifies user create command structure
func TestUserCreateCommand(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd, "create subcommand should exist")

	assert.Equal(t, "create", createCmd.Use, "command Use should be 'create'")
	assert.NotEmpty(t, createCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, createCmd.Long, "Long description should not be empty")
	assert.NotNil(t, createCmd.RunE, "RunE function should be set")
}

// TestUserCreateCommand_Flags verifies create command flags
func TestUserCreateCommand_Flags(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd)

	requiredFlags := []struct {
		name string
	}{
		{"org-id"},
		{"email"},
		{"display-name"},
		{"roles"},
		{"direct"},
		{"force-pwd-change"},
		{"upsert"},
	}

	for _, tt := range requiredFlags {
		t.Run(tt.name, func(t *testing.T) {
			flag := createCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestUserCreateCommand_DirectFlag verifies direct flag behavior
func TestUserCreateCommand_DirectFlag(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd)

	directFlag := createCmd.Flags().Lookup("direct")
	require.NotNil(t, directFlag, "direct flag should exist")

	// Should be a boolean flag defaulting to false
	assert.Equal(t, "false", directFlag.DefValue, "direct flag should default to false")
}

// TestUserCreateCommand_ForcePwdChangeFlag verifies force-pwd-change flag
func TestUserCreateCommand_ForcePwdChangeFlag(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd)

	forcePwdFlag := createCmd.Flags().Lookup("force-pwd-change")
	require.NotNil(t, forcePwdFlag, "force-pwd-change flag should exist")

	// Should be a boolean flag defaulting to false
	assert.Equal(t, "false", forcePwdFlag.DefValue, "force-pwd-change flag should default to false")
}

// TestUserCreateCommand_Examples verifies create command examples
func TestUserCreateCommand_Examples(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd)

	assert.Contains(t, createCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, createCmd.Long, "--email", "Examples should show email flag")
	assert.Contains(t, createCmd.Long, "--direct", "Examples should show direct flag")
	assert.Contains(t, createCmd.Long, "INVITE", "Long description should explain invite mode")
	assert.Contains(t, createCmd.Long, "DIRECT", "Long description should explain direct mode")
}

// TestUserCreateCommand_ModelAccessFlag verifies model-access flag
func TestUserCreateCommand_ModelAccessFlag(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd)

	modelAccessFlag := createCmd.Flags().Lookup("model-access")
	require.NotNil(t, modelAccessFlag, "model-access flag should exist")

	// Should have shorthand 'm'
	assert.Equal(t, "m", modelAccessFlag.Shorthand, "model-access flag should have shorthand 'm'")

	// Should be optional (empty default)
	assert.Equal(t, "", modelAccessFlag.DefValue, "model-access flag should default to empty string")
}

// TestUserUpdateCommand verifies user update command structure
func TestUserUpdateCommand(t *testing.T) {
	cmd := UserCommand()
	var updateCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "update" {
			updateCmd = c
			break
		}
	}
	require.NotNil(t, updateCmd, "update subcommand should exist")

	assert.Equal(t, "update", updateCmd.Use, "command Use should be 'update'")
	assert.NotEmpty(t, updateCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, updateCmd.Long, "Long description should not be empty")
	assert.NotNil(t, updateCmd.RunE, "RunE function should be set")
}

// TestUserUpdateCommand_Flags verifies update command flags
func TestUserUpdateCommand_Flags(t *testing.T) {
	cmd := UserCommand()
	var updateCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "update" {
			updateCmd = c
			break
		}
	}
	require.NotNil(t, updateCmd)

	flagTests := []struct {
		name string
	}{
		{"org-id"},
		{"user-id"},
		{"email"},
		{"display-name"},
		{"status"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := updateCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestUserUpdateCommand_IdentifierFlags verifies either user-id or email is accepted
func TestUserUpdateCommand_IdentifierFlags(t *testing.T) {
	cmd := UserCommand()
	var updateCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "update" {
			updateCmd = c
			break
		}
	}
	require.NotNil(t, updateCmd)

	userIDFlag := updateCmd.Flags().Lookup("user-id")
	emailFlag := updateCmd.Flags().Lookup("email")

	assert.NotNil(t, userIDFlag, "user-id flag should exist")
	assert.NotNil(t, emailFlag, "email flag should exist")

	// Both should be optional at the flag level (validation happens in RunE)
	assert.Equal(t, "", userIDFlag.DefValue, "user-id should default to empty string")
	assert.Equal(t, "", emailFlag.DefValue, "email should default to empty string")
}

// TestUserUpdateCommand_Examples verifies update command examples
func TestUserUpdateCommand_Examples(t *testing.T) {
	cmd := UserCommand()
	var updateCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "update" {
			updateCmd = c
			break
		}
	}
	require.NotNil(t, updateCmd)

	assert.Contains(t, updateCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, updateCmd.Long, "--email", "Examples should show email flag")
	assert.Contains(t, updateCmd.Long, "--status", "Examples should show status flag")
}

// TestUserDeleteCommand verifies user delete command structure
func TestUserDeleteCommand(t *testing.T) {
	cmd := UserCommand()
	var deleteCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "delete" {
			deleteCmd = c
			break
		}
	}
	require.NotNil(t, deleteCmd, "delete subcommand should exist")

	assert.Equal(t, "delete", deleteCmd.Use, "command Use should be 'delete'")
	assert.NotEmpty(t, deleteCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, deleteCmd.Long, "Long description should not be empty")
	assert.NotNil(t, deleteCmd.RunE, "RunE function should be set")
}

// TestUserDeleteCommand_Flags verifies delete command flags
func TestUserDeleteCommand_Flags(t *testing.T) {
	cmd := UserCommand()
	var deleteCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "delete" {
			deleteCmd = c
			break
		}
	}
	require.NotNil(t, deleteCmd)

	flagTests := []struct {
		name         string
		defaultValue string
	}{
		{"org-id", ""},
		{"user-id", ""},
		{"email", ""},
		{"confirm", "false"},
		{"force", "false"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := deleteCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q should have default %q", tt.name, tt.defaultValue)
			}
		})
	}
}

// TestUserDeleteCommand_SafetyFlags verifies confirm and force flags
func TestUserDeleteCommand_SafetyFlags(t *testing.T) {
	cmd := UserCommand()
	var deleteCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "delete" {
			deleteCmd = c
			break
		}
	}
	require.NotNil(t, deleteCmd)

	confirmFlag := deleteCmd.Flags().Lookup("confirm")
	forceFlag := deleteCmd.Flags().Lookup("force")

	assert.NotNil(t, confirmFlag, "confirm flag should exist")
	assert.NotNil(t, forceFlag, "force flag should exist")

	// Both should default to false (safety)
	assert.Equal(t, "false", confirmFlag.DefValue, "confirm flag should default to false")
	assert.Equal(t, "false", forceFlag.DefValue, "force flag should default to false")
}

// TestUserDeleteCommand_Examples verifies delete command examples
func TestUserDeleteCommand_Examples(t *testing.T) {
	cmd := UserCommand()
	var deleteCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "delete" {
			deleteCmd = c
			break
		}
	}
	require.NotNil(t, deleteCmd)

	assert.Contains(t, deleteCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, deleteCmd.Long, "--confirm", "Examples should show confirm flag")
	assert.Contains(t, deleteCmd.Long, "cannot be undone", "Long description should warn about irreversibility")
	assert.Contains(t, deleteCmd.Long, "Warning:", "Long description should have warning section")
}

// TestDetermineAccessMode verifies model access mode determination logic
func TestDetermineAccessMode(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		roles     []string
		expected  string
	}{
		{
			name:      "explicit_all",
			flagValue: "all",
			roles:     []string{"user"},
			expected:  "auto_grant",
		},
		{
			name:      "explicit_restricted",
			flagValue: "restricted",
			roles:     []string{"admin"},
			expected:  "restricted",
		},
		{
			name:      "default_admin_role",
			flagValue: "",
			roles:     []string{"admin", "developer"},
			expected:  "auto_grant",
		},
		{
			name:      "default_non_admin_role",
			flagValue: "",
			roles:     []string{"user", "developer"},
			expected:  "restricted",
		},
		{
			name:      "empty_roles",
			flagValue: "",
			roles:     []string{},
			expected:  "restricted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineAccessMode(tt.flagValue, tt.roles)
			assert.Equal(t, tt.expected, result, "determineAccessMode(%q, %v) should return %q", tt.flagValue, tt.roles, tt.expected)
		})
	}
}

// TestHasAdminRole verifies admin role detection
func TestHasAdminRole(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		expected bool
	}{
		{
			name:     "has_admin",
			roles:    []string{"admin", "developer"},
			expected: true,
		},
		{
			name:     "no_admin",
			roles:    []string{"user", "developer"},
			expected: false,
		},
		{
			name:     "empty_roles",
			roles:    []string{},
			expected: false,
		},
		{
			name:     "only_admin",
			roles:    []string{"admin"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAdminRole(tt.roles)
			assert.Equal(t, tt.expected, result, "hasAdminRole(%v) should return %v", tt.roles, tt.expected)
		})
	}
}

// TestUserCommand_WorkflowGuidance verifies workflow guidance in help text
func TestUserCommand_WorkflowGuidance(t *testing.T) {
	cmd := UserCommand()
	require.NotNil(t, cmd)

	// Verify Long description contains workflow guidance
	assert.Contains(t, cmd.Long, "Workflow:", "Long description should contain workflow section")
	assert.Contains(t, cmd.Long, "See Also:", "Long description should contain 'See Also' section")
}

// TestUserCreateCommand_ProfileFlag verifies profile flag integration
func TestUserCreateCommand_ProfileFlag(t *testing.T) {
	cmd := UserCommand()
	var createCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "create" {
			createCmd = c
			break
		}
	}
	require.NotNil(t, createCmd)

	profileFlag := createCmd.Flags().Lookup("profile")
	require.NotNil(t, profileFlag, "profile flag should exist")

	// Should be optional (empty default)
	assert.Equal(t, "", profileFlag.DefValue, "profile flag should default to empty string")
}

// TestUserCommand_SubcommandConsistency verifies all subcommands have consistent structure
func TestUserCommand_SubcommandConsistency(t *testing.T) {
	cmd := UserCommand()
	subcommands := []string{"list", "create", "update", "delete"}

	for _, subcmdName := range subcommands {
		t.Run(subcmdName, func(t *testing.T) {
			var subcmd *cobra.Command
			for _, c := range cmd.Commands() {
				if c.Name() == subcmdName {
					subcmd = c
					break
				}
			}
			require.NotNil(t, subcmd, "subcommand %q should exist", subcmdName)

			// All subcommands should have these fields
			assert.NotEmpty(t, subcmd.Short, "subcommand %q should have Short description", subcmdName)
			assert.NotEmpty(t, subcmd.Long, "subcommand %q should have Long description", subcmdName)
			assert.NotNil(t, subcmd.RunE, "subcommand %q should have RunE function", subcmdName)

			// All subcommands should have org-id flag
			orgIDFlag := subcmd.Flags().Lookup("org-id")
			assert.NotNil(t, orgIDFlag, "subcommand %q should have org-id flag", subcmdName)
		})
	}
}
