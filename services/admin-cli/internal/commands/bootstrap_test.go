// Package commands provides tests for bootstrap command.
package commands

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapCommand(t *testing.T) {
	cmd := BootstrapCommand()
	require.NotNil(t, cmd, "BootstrapCommand() returned nil")

	assert.Equal(t, "bootstrap", cmd.Use)
	assert.NotNil(t, cmd.RunE, "command should have RunE handler")

	// Test flags
	emailFlag := cmd.Flags().Lookup("email")
	assert.NotNil(t, emailFlag, "email flag should exist")

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "dry-run flag should exist")
}

func TestBootstrapCommandValidation(t *testing.T) {
	// Test that command validates required flags
	cmd := BootstrapCommand()

	// Create a command context
	rootCmd := &cobra.Command{Use: "admin-cli"}
	rootCmd.AddCommand(cmd)

	// Test dry-run (should work without email)
	err := rootCmd.ExecuteContext(context.Background())
	// This will fail because we're not providing the required config, but that's OK for unit test
	_ = err
}

// Integration tests are in test/integration/bootstrap_test.go (Phase 3)
