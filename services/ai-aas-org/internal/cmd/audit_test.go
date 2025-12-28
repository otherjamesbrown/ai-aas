package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditCommand verifies the audit command structure
func TestAuditCommand(t *testing.T) {
	require.NotNil(t, auditCmd, "auditCmd should not be nil")

	assert.Equal(t, "audit", auditCmd.Use, "command Use should be 'audit'")
	assert.NotEmpty(t, auditCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, auditCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"list"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range auditCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestAuditCommand_Examples verifies examples are provided
func TestAuditCommand_Examples(t *testing.T) {
	assert.Contains(t, auditCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, auditCmd.Long, "audit list", "Examples should show list command")
	assert.Contains(t, auditCmd.Long, "security", "Long description should mention security")
	assert.Contains(t, auditCmd.Long, "compliance", "Long description should mention compliance")
}

// TestAuditListCommand verifies audit list command structure
func TestAuditListCommand(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range auditCmd.Commands() {
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

// TestAuditListCommand_Flags verifies list command flags
func TestAuditListCommand_Flags(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range auditCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	flagTests := []struct {
		name         string
		defaultValue string
	}{
		{"action", ""},
		{"resource", ""},
		{"actor", ""},
		{"limit", "25"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := listCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q should have default value %q", tt.name, tt.defaultValue)
			}
		})
	}
}

// TestAuditListCommand_Examples verifies list command examples
func TestAuditListCommand_Examples(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range auditCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	assert.Contains(t, listCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, listCmd.Long, "audit list", "Examples should show basic usage")
	assert.Contains(t, listCmd.Long, "--action", "Examples should show action filter")
	assert.Contains(t, listCmd.Long, "--resource", "Examples should show resource filter")
	assert.Contains(t, listCmd.Long, "--limit", "Examples should show limit flag")
}

// TestAuditListCommand_Filters verifies filter documentation
func TestAuditListCommand_Filters(t *testing.T) {
	var listCmd *cobra.Command
	for _, cmd := range auditCmd.Commands() {
		if cmd.Name() == "list" {
			listCmd = cmd
			break
		}
	}
	require.NotNil(t, listCmd)

	assert.Contains(t, listCmd.Long, "Filters:", "Long description should document filters")
	assert.Contains(t, listCmd.Long, "action", "Long description should explain action filter")
	assert.Contains(t, listCmd.Long, "resource", "Long description should explain resource filter")
	assert.Contains(t, listCmd.Long, "actor", "Long description should explain actor filter")
}

// TestTruncate verifies string truncation helper
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10c", 10, "exactly10c"},
		{"this is a very long string", 10, "this is..."},
		{"abc", 5, "abc"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result, "truncate(%q, %d) should return %q", tt.input, tt.maxLen, tt.expected)
		})
	}
}
