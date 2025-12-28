package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitCommand verifies the init command structure
func TestInitCommand(t *testing.T) {
	require.NotNil(t, initCmd, "initCmd should not be nil")

	assert.Equal(t, "init", initCmd.Use, "command Use should be 'init'")
	assert.NotEmpty(t, initCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, initCmd.Long, "Long description should not be empty")
	assert.NotNil(t, initCmd.RunE, "RunE function should be set")
}

// TestInitCommand_Flags verifies init command flags
func TestInitCommand_Flags(t *testing.T) {
	flagTests := []struct {
		name      string
		shorthand string
	}{
		{"key", "k"},
		{"api-endpoint", ""},
		{"force", ""},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := initCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand, "flag %q should have shorthand %q", tt.name, tt.shorthand)
			}
		})
	}
}

// TestInitCommand_Examples verifies examples are provided
func TestInitCommand_Examples(t *testing.T) {
	assert.Contains(t, initCmd.Long, "Example:", "Long description should contain examples")
	assert.Contains(t, initCmd.Long, "init --key", "Examples should show init with key flag")
	assert.Contains(t, initCmd.Long, "bsk_", "Examples should show bootstrap key format")
}

// TestInitCommand_Documentation verifies comprehensive documentation
func TestInitCommand_Documentation(t *testing.T) {
	assert.Contains(t, initCmd.Long, "bootstrap key", "Long description should explain bootstrap key")
	assert.Contains(t, initCmd.Long, "one-time", "Long description should mention one-time usage")
	assert.Contains(t, initCmd.Long, "expires", "Long description should mention expiration")
	assert.Contains(t, initCmd.Long, "platform administrator", "Long description should mention admin")
	assert.Contains(t, initCmd.Long, "credentials", "Long description should mention credentials")
}

// TestInitCommand_ConfigPath verifies config file path is mentioned
func TestInitCommand_ConfigPath(t *testing.T) {
	assert.Contains(t, initCmd.Long, ".ai-aas-org.yaml", "Long description should mention config file")
}

// TestExtractEndpointFromKey verifies endpoint extraction function
func TestExtractEndpointFromKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"bsk_abc123", ""},
		{"bsk_xyz789", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := extractEndpointFromKey(tt.key)
			assert.Equal(t, tt.expected, result, "extractEndpointFromKey should return %q for key %q", tt.expected, tt.key)
		})
	}
}

// TestIsInteractive verifies interactive detection function
func TestIsInteractive(t *testing.T) {
	// Just verify the function doesn't panic
	// Actual behavior depends on stdin which we can't easily mock
	assert.NotPanics(t, func() {
		_ = isInteractive()
	}, "isInteractive should not panic")
}
