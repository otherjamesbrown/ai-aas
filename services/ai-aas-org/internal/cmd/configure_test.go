package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigureCommand verifies the configure command structure
func TestConfigureCommand(t *testing.T) {
	require.NotNil(t, configureCmd, "configureCmd should not be nil")

	assert.Equal(t, "configure", configureCmd.Use, "command Use should be 'configure'")
	assert.NotEmpty(t, configureCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, configureCmd.Long, "Long description should not be empty")
	assert.NotNil(t, configureCmd.RunE, "RunE function should be set")
}

// TestConfigureCommand_Flags verifies configure command flags
func TestConfigureCommand_Flags(t *testing.T) {
	flagTests := []struct {
		name string
	}{
		{"api-endpoint"},
		{"api-key"},
		{"org-id"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := configureCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
		})
	}
}

// TestConfigureCommand_Examples verifies examples are provided
func TestConfigureCommand_Examples(t *testing.T) {
	assert.Contains(t, configureCmd.Long, "Example:", "Long description should contain examples")
	assert.Contains(t, configureCmd.Long, "configure", "Examples should show configure command")
	assert.Contains(t, configureCmd.Long, "--api-endpoint", "Examples should show api-endpoint flag")
	assert.Contains(t, configureCmd.Long, "--api-key", "Examples should show api-key flag")
}

// TestConfigureCommand_AdvancedWarning verifies warning about advanced usage
func TestConfigureCommand_AdvancedWarning(t *testing.T) {
	assert.Contains(t, configureCmd.Long, "advanced users", "Long description should mention advanced users")
	assert.Contains(t, configureCmd.Long, "init --key", "Long description should recommend init for most users")
}
