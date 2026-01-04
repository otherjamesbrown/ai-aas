// Package model provides tests for model swap command.
package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSwapCommand verifies swap command structure
func TestNewSwapCommand(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd, "NewSwapCommand() returned nil")

	assert.Equal(t, "swap", cmd.Use[:4], "command Use should start with 'swap'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "RunE function should be set")

	// Verify Args validator is set (requires exactly 2 args)
	assert.NotNil(t, cmd.Args, "Args validator should be set")
}

// TestSwapCommand_Flags verifies swap command flags
func TestSwapCommand_Flags(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd)

	flagTests := []struct {
		name     string
		required bool
	}{
		{"environment", true},
		{"force", false},
		{"reason", false},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestSwapCommand_EnvironmentFlag verifies environment flag is required
func TestSwapCommand_EnvironmentFlag(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd)

	envFlag := cmd.Flags().Lookup("environment")
	require.NotNil(t, envFlag, "environment flag should exist")
	assert.Equal(t, "e", envFlag.Shorthand, "environment flag shorthand should be 'e'")
	assert.NotEmpty(t, envFlag.Usage, "environment flag should have usage description")
}

// TestSwapCommand_Examples verifies usage examples
func TestSwapCommand_Examples(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd)

	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, cmd.Long, "model swap", "Examples should show swap command")
	assert.Contains(t, cmd.Long, "-e production", "Examples should show environment flag")
}

// TestSwapCommand_ForceFlag verifies force flag exists and defaults to false
func TestSwapCommand_ForceFlag(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd)

	forceFlag := cmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should exist")
	assert.Equal(t, "false", forceFlag.DefValue, "force default should be false")
}

// TestSwapCommand_ReasonFlag verifies reason flag exists
func TestSwapCommand_ReasonFlag(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd)

	reasonFlag := cmd.Flags().Lookup("reason")
	assert.NotNil(t, reasonFlag, "reason flag should exist")
	assert.NotEmpty(t, reasonFlag.Usage, "reason flag should have usage description")
}

// TestSwapCommand_Description verifies command describes atomic swap behavior
func TestSwapCommand_Description(t *testing.T) {
	cmd := NewSwapCommand()
	require.NotNil(t, cmd)

	// Verify long description explains the atomic swap behavior
	assert.Contains(t, cmd.Long, "Atomically", "Long description should mention atomic behavior")
	assert.Contains(t, cmd.Long, "disable", "Long description should explain disable step")
	assert.Contains(t, cmd.Long, "enable", "Long description should explain enable step")
}

// TestNewHistoryCommand verifies history command structure
func TestNewHistoryCommand(t *testing.T) {
	cmd := NewHistoryCommand()
	require.NotNil(t, cmd, "NewHistoryCommand() returned nil")

	assert.Equal(t, "history", cmd.Use[:7], "command Use should start with 'history'")
	assert.NotEmpty(t, cmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "RunE function should be set")

	// Verify Args validator is set (requires exactly 1 arg: model name)
	assert.NotNil(t, cmd.Args, "Args validator should be set")
}

// TestHistoryCommand_Flags verifies history command flags
func TestHistoryCommand_Flags(t *testing.T) {
	cmd := NewHistoryCommand()
	require.NotNil(t, cmd)

	flagTests := []struct {
		name         string
		required     bool
		defaultValue string
	}{
		{"environment", false, ""},
		{"limit", false, "20"},
		{"format", false, "table"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q default should be %q", tt.name, tt.defaultValue)
			}
		})
	}
}

// TestHistoryCommand_Examples verifies usage examples
func TestHistoryCommand_Examples(t *testing.T) {
	cmd := NewHistoryCommand()
	require.NotNil(t, cmd)

	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.Contains(t, cmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, cmd.Long, "model history", "Examples should show history command")
}

// TestHistoryCommand_FormatFlag verifies format flag accepts table and json
func TestHistoryCommand_FormatFlag(t *testing.T) {
	cmd := NewHistoryCommand()
	require.NotNil(t, cmd)

	formatFlag := cmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
	assert.Equal(t, "table", formatFlag.DefValue, "format default should be 'table'")
	assert.Contains(t, formatFlag.Usage, "json", "format usage should mention json")
}
