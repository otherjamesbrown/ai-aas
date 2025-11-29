// Package commands provides tests for apikey command.
package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCommand(t *testing.T) {
	cmd := APIKeyCommand()
	require.NotNil(t, cmd, "APIKeyCommand() returned nil")

	assert.Equal(t, "apikey", cmd.Use)
	assert.Equal(t, "Manage API keys", cmd.Short)
}

func TestAPIKeyListCommand(t *testing.T) {
	cmd := APIKeyCommand()

	// Find list command by name
	listCmd, _, err := cmd.Find([]string{"list"})
	require.NoError(t, err, "list command should exist")
	require.NotNil(t, listCmd, "list command should not be nil")
	assert.Equal(t, "list", listCmd.Use)

	// Test flags
	orgIDFlag := listCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	formatFlag := listCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")

	verboseFlag := listCmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")

	quietFlag := listCmd.Flags().Lookup("quiet")
	assert.NotNil(t, quietFlag, "quiet flag should exist")
}

func TestAPIKeyCreateCommand(t *testing.T) {
	cmd := APIKeyCommand()

	// Find create command by name
	createCmd, _, err := cmd.Find([]string{"create"})
	require.NoError(t, err, "create command should exist")
	require.NotNil(t, createCmd, "create command should not be nil")
	assert.Equal(t, "create", createCmd.Use)

	// Test flags
	orgIDFlag := createCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	userIDFlag := createCmd.Flags().Lookup("user-id")
	assert.NotNil(t, userIDFlag, "user-id flag should exist")

	scopesFlag := createCmd.Flags().Lookup("scopes")
	assert.NotNil(t, scopesFlag, "scopes flag should exist")

	expiresInDaysFlag := createCmd.Flags().Lookup("expires-in-days")
	assert.NotNil(t, expiresInDaysFlag, "expires-in-days flag should exist")

	formatFlag := createCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
}

func TestAPIKeyDeleteCommand(t *testing.T) {
	cmd := APIKeyCommand()

	// Find delete command by name
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	require.NoError(t, err, "delete command should exist")
	require.NotNil(t, deleteCmd, "delete command should not be nil")
	assert.Equal(t, "delete", deleteCmd.Use)

	// Test flags
	orgIDFlag := deleteCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	apiKeyIDFlag := deleteCmd.Flags().Lookup("api-key-id")
	assert.NotNil(t, apiKeyIDFlag, "api-key-id flag should exist")

	confirmFlag := deleteCmd.Flags().Lookup("confirm")
	assert.NotNil(t, confirmFlag, "confirm flag should exist")

	forceFlag := deleteCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should exist")

	formatFlag := deleteCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
}

func TestAPIKeyRotateCommand(t *testing.T) {
	cmd := APIKeyCommand()

	// Find rotate command by name
	rotateCmd, _, err := cmd.Find([]string{"rotate"})
	require.NoError(t, err, "rotate command should exist")
	require.NotNil(t, rotateCmd, "rotate command should not be nil")
	assert.Equal(t, "rotate", rotateCmd.Use)

	// Test flags
	orgIDFlag := rotateCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	apiKeyIDFlag := rotateCmd.Flags().Lookup("api-key-id")
	assert.NotNil(t, apiKeyIDFlag, "api-key-id flag should exist")

	confirmFlag := rotateCmd.Flags().Lookup("confirm")
	assert.NotNil(t, confirmFlag, "confirm flag should exist")

	formatFlag := rotateCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")

	verboseFlag := rotateCmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")

	quietFlag := rotateCmd.Flags().Lookup("quiet")
	assert.NotNil(t, quietFlag, "quiet flag should exist")
}

func TestAPIKeyUpdateCommand(t *testing.T) {
	cmd := APIKeyCommand()

	// Find update command by name
	updateCmd, _, err := cmd.Find([]string{"update"})
	require.NoError(t, err, "update command should exist")
	require.NotNil(t, updateCmd, "update command should not be nil")
	assert.Equal(t, "update", updateCmd.Use)

	// Test flags
	orgIDFlag := updateCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	apiKeyIDFlag := updateCmd.Flags().Lookup("api-key-id")
	assert.NotNil(t, apiKeyIDFlag, "api-key-id flag should exist")

	scopesFlag := updateCmd.Flags().Lookup("scopes")
	assert.NotNil(t, scopesFlag, "scopes flag should exist")

	expiresAtFlag := updateCmd.Flags().Lookup("expires-at")
	assert.NotNil(t, expiresAtFlag, "expires-at flag should exist")

	formatFlag := updateCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")

	verboseFlag := updateCmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should exist")

	quietFlag := updateCmd.Flags().Lookup("quiet")
	assert.NotNil(t, quietFlag, "quiet flag should exist")
}
