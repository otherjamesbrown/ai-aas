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

func TestRunBootstrapValidation(t *testing.T) {
	t.Run("missing email in execution mode", func(t *testing.T) {
		err := runBootstrap(&cobra.Command{}, []string{},
			"", "", "", false, true, "table", false, false, "http://localhost:8080", "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--email is required")
	})

	t.Run("missing user-org-endpoint", func(t *testing.T) {
		err := runBootstrap(&cobra.Command{}, []string{},
			"admin@example.com", "", "", true, false, "table", false, false, "", "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user-org-service endpoint is required")
	})
}

// Integration tests are in test/integration/bootstrap_test.go (Phase 3)
