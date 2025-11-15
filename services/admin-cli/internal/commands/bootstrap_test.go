// Package commands provides tests for bootstrap command.
package commands

import (
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

// Note: runBootstrap function requires external configuration and network access,
// so detailed validation tests are in test/integration/bootstrap_test.go

// Integration tests are in test/integration/bootstrap_test.go (Phase 3)
