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
	
	rotateCmd := cmd.Commands()[0] // First subcommand should be rotate
	require.NotNil(t, rotateCmd, "rotate command should exist")
	assert.Equal(t, "rotate", rotateCmd.Use)

	// Test flags
	orgIDFlag := rotateCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	apiKeyIDFlag := rotateCmd.Flags().Lookup("api-key-id")
	assert.NotNil(t, apiKeyIDFlag, "api-key-id flag should exist")
}

func TestCredentialsBreakGlassCommand(t *testing.T) {
	cmd := CredentialsCommand()
	
	breakGlassCmd := cmd.Commands()[1] // Second subcommand should be break-glass
	require.NotNil(t, breakGlassCmd, "break-glass command should exist")
	assert.Equal(t, "break-glass", breakGlassCmd.Use)

	// Test flags
	recoveryTokenFlag := breakGlassCmd.Flags().Lookup("recovery-token")
	assert.NotNil(t, recoveryTokenFlag, "recovery-token flag should exist")

	emailFlag := breakGlassCmd.Flags().Lookup("email")
	assert.NotNil(t, emailFlag, "email flag should exist")
}
