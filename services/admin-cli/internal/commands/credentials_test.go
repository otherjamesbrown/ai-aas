// Package commands provides tests for credentials command.
package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialsCommand(t *testing.T) {
	cmd := CredentialsCommand()
	require.NotNil(t, cmd, "CredentialsCommand() returned nil")

	assert.Equal(t, "credentials", cmd.Use)
}

func TestCredentialsRotateCommand(t *testing.T) {
	cmd := CredentialsCommand()
	
	// Find rotate command by name instead of index
	rotateCmd, _, err := cmd.Find([]string{"rotate"})
	require.NoError(t, err, "rotate command should exist")
	require.NotNil(t, rotateCmd, "rotate command should not be nil")
	assert.Equal(t, "rotate", rotateCmd.Use)

	// Test flags
	orgIDFlag := rotateCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	apiKeyIDFlag := rotateCmd.Flags().Lookup("api-key-id")
	assert.NotNil(t, apiKeyIDFlag, "api-key-id flag should exist")
}

func TestCredentialsBreakGlassCommand(t *testing.T) {
	cmd := CredentialsCommand()
	
	// Find break-glass command by name instead of index
	breakGlassCmd, _, err := cmd.Find([]string{"break-glass"})
	require.NoError(t, err, "break-glass command should exist")
	require.NotNil(t, breakGlassCmd, "break-glass command should not be nil")
	assert.Equal(t, "break-glass", breakGlassCmd.Use)

	// Test flags
	recoveryTokenFlag := breakGlassCmd.Flags().Lookup("recovery-token")
	assert.NotNil(t, recoveryTokenFlag, "recovery-token flag should exist")

	emailFlag := breakGlassCmd.Flags().Lookup("email")
	assert.NotNil(t, emailFlag, "email flag should exist")
}
