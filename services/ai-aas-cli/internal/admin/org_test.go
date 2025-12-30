// Package admin provides tests for org commands.
package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgCommand(t *testing.T) {
	cmd := OrgCommand()
	require.NotNil(t, cmd, "OrgCommand() returned nil")

	assert.Equal(t, "org", cmd.Use)
	assert.Equal(t, "Manage organizations", cmd.Short)
}

func TestOrgListCommand(t *testing.T) {
	cmd := OrgCommand()

	listCmd, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err, "list command should exist")
	require.NotNil(t, listCmd, "list command should not be nil")
	assert.Equal(t, "list", listCmd.Use)

	// Test flags
	formatFlag := listCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")

	verboseFlag := listCmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")

	quietFlag := listCmd.Flags().Lookup("quiet")
	assert.NotNil(t, quietFlag, "quiet flag should exist")
}

func TestOrgCreateCommand(t *testing.T) {
	cmd := OrgCommand()

	createCmd, _, err := cmd.Find([]string{"create"})
	require.NoError(t, err, "create command should exist")
	require.NotNil(t, createCmd, "create command should not be nil")
	assert.Equal(t, "create", createCmd.Use)

	// Test flags
	nameFlag := createCmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag, "name flag should exist")

	slugFlag := createCmd.Flags().Lookup("slug")
	assert.NotNil(t, slugFlag, "slug flag should exist")

	formatFlag := createCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")

	dryRunFlag := createCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "dry-run flag should exist")

	useFlag := createCmd.Flags().Lookup("use")
	assert.NotNil(t, useFlag, "use flag should exist")
}

func TestOrgUpdateCommand(t *testing.T) {
	cmd := OrgCommand()

	updateCmd, _, err := cmd.Find([]string{"update"})
	require.NoError(t, err, "update command should exist")
	require.NotNil(t, updateCmd, "update command should not be nil")
	assert.Equal(t, "update", updateCmd.Use)

	// Test flags
	orgIDFlag := updateCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	statusFlag := updateCmd.Flags().Lookup("status")
	assert.NotNil(t, statusFlag, "status flag should exist")

	displayNameFlag := updateCmd.Flags().Lookup("display-name")
	assert.NotNil(t, displayNameFlag, "display-name flag should exist")
}

func TestOrgDeleteCommand(t *testing.T) {
	cmd := OrgCommand()

	deleteCmd, _, err := cmd.Find([]string{"delete"})
	require.NoError(t, err, "delete command should exist")
	require.NotNil(t, deleteCmd, "delete command should not be nil")
	assert.Equal(t, "delete", deleteCmd.Use)

	// Test flags
	orgIDFlag := deleteCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	confirmFlag := deleteCmd.Flags().Lookup("confirm")
	assert.NotNil(t, confirmFlag, "confirm flag should exist")

	forceFlag := deleteCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should exist")
}

func TestOrgUseCommand(t *testing.T) {
	cmd := OrgCommand()

	useCmd, _, err := cmd.Find([]string{"use"})
	require.NoError(t, err, "use command should exist")
	require.NotNil(t, useCmd, "use command should not be nil")
	assert.Equal(t, "use [org-id]", useCmd.Use)

	// Test flags
	clearFlag := useCmd.Flags().Lookup("clear")
	assert.NotNil(t, clearFlag, "clear flag should exist")
}

func TestOrgBootstrapCommand(t *testing.T) {
	cmd := OrgCommand()

	bootstrapCmd, _, err := cmd.Find([]string{"bootstrap"})
	require.NoError(t, err, "bootstrap command should exist")
	require.NotNil(t, bootstrapCmd, "bootstrap command should not be nil")
	assert.Equal(t, "bootstrap", bootstrapCmd.Use)
	assert.Equal(t, "Create organization with admin user and API key", bootstrapCmd.Short)

	// Test required flags
	nameFlag := bootstrapCmd.Flags().Lookup("org-name")
	assert.NotNil(t, nameFlag, "org-name flag should exist")

	slugFlag := bootstrapCmd.Flags().Lookup("org-slug")
	assert.NotNil(t, slugFlag, "org-slug flag should exist")

	adminEmailFlag := bootstrapCmd.Flags().Lookup("admin-email")
	assert.NotNil(t, adminEmailFlag, "admin-email flag should exist")

	// Test optional flags
	adminNameFlag := bootstrapCmd.Flags().Lookup("admin-name")
	assert.NotNil(t, adminNameFlag, "admin-name flag should exist")

	profileFlag := bootstrapCmd.Flags().Lookup("profile")
	assert.NotNil(t, profileFlag, "profile flag should exist")

	formatFlag := bootstrapCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
	assert.Equal(t, "table", formatFlag.DefValue, "format default should be table")

	verboseFlag := bootstrapCmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")

	quietFlag := bootstrapCmd.Flags().Lookup("quiet")
	assert.NotNil(t, quietFlag, "quiet flag should exist")
}

func TestOrgBootstrapCommand_HelpText(t *testing.T) {
	cmd := OrgCommand()

	bootstrapCmd, _, err := cmd.Find([]string{"bootstrap"})
	require.NoError(t, err, "bootstrap command should exist")

	// Verify help text mentions key features
	long := bootstrapCmd.Long
	assert.Contains(t, long, "Bootstrap a new organization", "help should describe purpose")
	assert.Contains(t, long, "Create the organization", "help should mention org creation")
	assert.Contains(t, long, "Create an admin user", "help should mention user creation")
	assert.Contains(t, long, "Create an API key", "help should mention API key creation")
	assert.Contains(t, long, "ai-aas-org", "help should mention ai-aas-org for next steps")
}

func TestOrgBootstrapCommand_Validation(t *testing.T) {
	// Test that the command structure is correct for validation
	// The actual validation happens at runtime, but we can verify the flags exist
	cmd := OrgCommand()

	bootstrapCmd, _, err := cmd.Find([]string{"bootstrap"})
	require.NoError(t, err, "bootstrap command should exist")

	// Verify the command has a RunE function (validates at execution time)
	assert.NotNil(t, bootstrapCmd.RunE, "bootstrap should have RunE function for execution")
}
