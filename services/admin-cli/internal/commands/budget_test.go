// Package commands provides tests for budget command.
package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBudgetCommand(t *testing.T) {
	cmd := BudgetCommand()
	require.NotNil(t, cmd, "BudgetCommand() returned nil")

	assert.Equal(t, "budget", cmd.Use)
	assert.Equal(t, "Manage organization budgets", cmd.Short)
}

func TestBudgetListCommand(t *testing.T) {
	cmd := BudgetCommand()

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

func TestBudgetSetCommand(t *testing.T) {
	cmd := BudgetCommand()

	// Find set command by name
	setCmd, _, err := cmd.Find([]string{"set"})
	require.NoError(t, err, "set command should exist")
	require.NotNil(t, setCmd, "set command should not be nil")
	assert.Equal(t, "set", setCmd.Use)

	// Test flags
	orgIDFlag := setCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	monthlyLimitFlag := setCmd.Flags().Lookup("monthly-limit")
	assert.NotNil(t, monthlyLimitFlag, "monthly-limit flag should exist")

	dryRunFlag := setCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "dry-run flag should exist")

	confirmFlag := setCmd.Flags().Lookup("confirm")
	assert.NotNil(t, confirmFlag, "confirm flag should exist")
}
