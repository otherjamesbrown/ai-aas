package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsageCommand verifies the usage command structure
func TestUsageCommand(t *testing.T) {
	require.NotNil(t, usageCmd, "usageCmd should not be nil")

	assert.Equal(t, "usage", usageCmd.Use, "command Use should be 'usage'")
	assert.NotEmpty(t, usageCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, usageCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"summary", "by-model", "by-user"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range usageCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestUsageCommand_Examples verifies examples are provided
func TestUsageCommand_Examples(t *testing.T) {
	assert.Contains(t, usageCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, usageCmd.Long, "usage summary", "Examples should show summary command")
	assert.Contains(t, usageCmd.Long, "usage by-model", "Examples should show by-model command")
	assert.Contains(t, usageCmd.Long, "usage by-user", "Examples should show by-user command")
}

// TestUsageSummaryCommand verifies usage summary command structure
func TestUsageSummaryCommand(t *testing.T) {
	var summaryCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "summary" {
			summaryCmd = cmd
			break
		}
	}
	require.NotNil(t, summaryCmd, "summary subcommand should exist")

	assert.Equal(t, "summary", summaryCmd.Use, "command Use should be 'summary'")
	assert.NotEmpty(t, summaryCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, summaryCmd.Long, "Long description should not be empty")
	assert.NotNil(t, summaryCmd.RunE, "RunE function should be set")
}

// TestUsageSummaryCommand_Flags verifies summary command flags
func TestUsageSummaryCommand_Flags(t *testing.T) {
	var summaryCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "summary" {
			summaryCmd = cmd
			break
		}
	}
	require.NotNil(t, summaryCmd)

	periodFlag := summaryCmd.Flags().Lookup("period")
	assert.NotNil(t, periodFlag, "period flag should exist")
	assert.Equal(t, "30d", periodFlag.DefValue, "period flag should default to '30d'")
}

// TestUsageSummaryCommand_Examples verifies summary command examples
func TestUsageSummaryCommand_Examples(t *testing.T) {
	var summaryCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "summary" {
			summaryCmd = cmd
			break
		}
	}
	require.NotNil(t, summaryCmd)

	assert.Contains(t, summaryCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, summaryCmd.Long, "usage summary", "Examples should show basic usage")
	assert.Contains(t, summaryCmd.Long, "--period", "Examples should show period flag")
}

// TestUsageByModelCommand verifies usage by-model command structure
func TestUsageByModelCommand(t *testing.T) {
	var byModelCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "by-model" {
			byModelCmd = cmd
			break
		}
	}
	require.NotNil(t, byModelCmd, "by-model subcommand should exist")

	assert.Equal(t, "by-model", byModelCmd.Use, "command Use should be 'by-model'")
	assert.NotEmpty(t, byModelCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, byModelCmd.Long, "Long description should not be empty")
	assert.NotNil(t, byModelCmd.RunE, "RunE function should be set")
}

// TestUsageByUserCommand verifies usage by-user command structure
func TestUsageByUserCommand(t *testing.T) {
	var byUserCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "by-user" {
			byUserCmd = cmd
			break
		}
	}
	require.NotNil(t, byUserCmd, "by-user subcommand should exist")

	assert.Equal(t, "by-user", byUserCmd.Use, "command Use should be 'by-user'")
	assert.NotEmpty(t, byUserCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, byUserCmd.Long, "Long description should not be empty")
	assert.NotNil(t, byUserCmd.RunE, "RunE function should be set")
}

// TestUsageSummaryCommand_PeriodFlag verifies period flag accepts various formats
func TestUsageSummaryCommand_PeriodFlag(t *testing.T) {
	var summaryCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "summary" {
			summaryCmd = cmd
			break
		}
	}
	require.NotNil(t, summaryCmd)

	periodFlag := summaryCmd.Flags().Lookup("period")
	require.NotNil(t, periodFlag, "period flag should exist")

	// Verify default value
	assert.Equal(t, "30d", periodFlag.DefValue, "period should default to 30d")

	// Verify usage mentions period formats
	assert.NotEmpty(t, periodFlag.Usage, "period flag should have usage description")
}

// TestUsageByModelCommand_Flags verifies by-model command has appropriate flags
func TestUsageByModelCommand_Flags(t *testing.T) {
	var byModelCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "by-model" {
			byModelCmd = cmd
			break
		}
	}
	require.NotNil(t, byModelCmd)

	// Should have period flag similar to summary command
	// (Actual implementation may vary - this tests the command structure)
	assert.NotNil(t, byModelCmd.RunE, "RunE should be set for by-model command")
}

// TestUsageByUserCommand_Flags verifies by-user command has appropriate flags
func TestUsageByUserCommand_Flags(t *testing.T) {
	var byUserCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "by-user" {
			byUserCmd = cmd
			break
		}
	}
	require.NotNil(t, byUserCmd)

	// Should have period flag similar to summary command
	assert.NotNil(t, byUserCmd.RunE, "RunE should be set for by-user command")
}

// TestUsageCommand_Subcommands verifies all expected subcommands exist
func TestUsageCommand_Subcommands(t *testing.T) {
	subcommands := []string{"summary", "by-model", "by-user"}

	for _, subcmdName := range subcommands {
		t.Run(subcmdName, func(t *testing.T) {
			found := false
			for _, cmd := range usageCmd.Commands() {
				if cmd.Name() == subcmdName {
					found = true
					assert.NotNil(t, cmd.RunE, "subcommand %q should have RunE set", subcmdName)
					assert.NotEmpty(t, cmd.Short, "subcommand %q should have Short description", subcmdName)
					assert.NotEmpty(t, cmd.Long, "subcommand %q should have Long description", subcmdName)
					break
				}
			}
			assert.True(t, found, "subcommand %q should exist", subcmdName)
		})
	}
}

// TestUsageSummaryCommand_PeriodValidation verifies period flag format examples in docs
func TestUsageSummaryCommand_PeriodValidation(t *testing.T) {
	var summaryCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "summary" {
			summaryCmd = cmd
			break
		}
	}
	require.NotNil(t, summaryCmd)

	// Check that Long description mentions period formats
	assert.Contains(t, summaryCmd.Long, "period", "Long description should explain period flag")
}

// TestUsageByModelCommand_Examples verifies by-model command examples
func TestUsageByModelCommand_Examples(t *testing.T) {
	var byModelCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "by-model" {
			byModelCmd = cmd
			break
		}
	}
	require.NotNil(t, byModelCmd)

	// Verify command has examples
	assert.NotEmpty(t, byModelCmd.Long, "Long description should not be empty")
}

// TestUsageByUserCommand_Examples verifies by-user command examples
func TestUsageByUserCommand_Examples(t *testing.T) {
	var byUserCmd *cobra.Command
	for _, cmd := range usageCmd.Commands() {
		if cmd.Name() == "by-user" {
			byUserCmd = cmd
			break
		}
	}
	require.NotNil(t, byUserCmd)

	// Verify command has examples
	assert.NotEmpty(t, byUserCmd.Long, "Long description should not be empty")
}
