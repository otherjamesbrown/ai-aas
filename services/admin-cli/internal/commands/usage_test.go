// Package commands provides tests for usage command.
package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageCommand(t *testing.T) {
	cmd := UsageCommand()
	require.NotNil(t, cmd, "UsageCommand() returned nil")

	assert.Equal(t, "usage", cmd.Use)
	assert.Equal(t, "Query usage data", cmd.Short)
}

func TestUsageQueryCommand(t *testing.T) {
	cmd := UsageCommand()

	// Find query command by name
	queryCmd, _, err := cmd.Find([]string{"query"})
	require.NoError(t, err, "query command should exist")
	require.NotNil(t, queryCmd, "query command should not be nil")
	assert.Equal(t, "query", queryCmd.Use)

	// Test flags
	orgIDFlag := queryCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	fromFlag := queryCmd.Flags().Lookup("from")
	assert.NotNil(t, fromFlag, "from flag should exist")

	toFlag := queryCmd.Flags().Lookup("to")
	assert.NotNil(t, toFlag, "to flag should exist")

	granularityFlag := queryCmd.Flags().Lookup("granularity")
	assert.NotNil(t, granularityFlag, "granularity flag should exist")

	modelFlag := queryCmd.Flags().Lookup("model")
	assert.NotNil(t, modelFlag, "model flag should exist")

	formatFlag := queryCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
}

func TestUsageSummaryCommand(t *testing.T) {
	cmd := UsageCommand()

	// Find summary command by name
	summaryCmd, _, err := cmd.Find([]string{"summary"})
	require.NoError(t, err, "summary command should exist")
	require.NotNil(t, summaryCmd, "summary command should not be nil")
	assert.Equal(t, "summary", summaryCmd.Use)

	// Test flags
	orgIDFlag := summaryCmd.Flags().Lookup("org-id")
	assert.NotNil(t, orgIDFlag, "org-id flag should exist")

	periodFlag := summaryCmd.Flags().Lookup("period")
	assert.NotNil(t, periodFlag, "period flag should exist")

	formatFlag := summaryCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
}
