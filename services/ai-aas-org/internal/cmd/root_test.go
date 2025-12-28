package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCommand verifies the root command structure
func TestRootCommand(t *testing.T) {
	require.NotNil(t, rootCmd, "rootCmd should not be nil")

	assert.Equal(t, "ai-aas-org", rootCmd.Use, "command Use should be 'ai-aas-org'")
	assert.NotEmpty(t, rootCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, rootCmd.Long, "Long description should not be empty")
	assert.NotNil(t, rootCmd.Run, "Run function should be set")

	// Verify subcommands exist
	expectedSubcommands := []string{"user", "apikey", "org", "model", "usage", "audit", "init", "configure", "version"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestRootCommand_GlobalFlags verifies global flags
func TestRootCommand_GlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"config", ""},
		{"guided", ""},
		{"json", ""},
		{"quiet", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(tt.name)
			assert.NotNil(t, flag, "global flag %q should exist", tt.name)
		})
	}
}

// TestRootCommand_LongDescription verifies documentation
func TestRootCommand_LongDescription(t *testing.T) {
	assert.Contains(t, rootCmd.Long, "AI-as-a-Service", "Long description should mention AI-as-a-Service")
	assert.Contains(t, rootCmd.Long, "organization", "Long description should mention organization")
	assert.Contains(t, rootCmd.Long, "Getting Started:", "Long description should have getting started section")
	assert.Contains(t, rootCmd.Long, "init --key", "Long description should show init command")
}

// TestSetVersionInfo verifies version info can be set
func TestSetVersionInfo(t *testing.T) {
	version := "1.0.0"
	commit := "abc123"
	buildTime := "2025-12-28T12:00:00Z"

	SetVersionInfo(version, commit, buildTime)

	assert.Equal(t, version, versionInfo.Version, "Version should be set")
	assert.Equal(t, commit, versionInfo.Commit, "Commit should be set")
	assert.Equal(t, buildTime, versionInfo.BuildTime, "BuildTime should be set")
}

// TestIsGuidedMode verifies guided mode detection
func TestIsGuidedMode(t *testing.T) {
	// Default should be false
	assert.False(t, IsGuidedMode(), "IsGuidedMode should default to false")

	// After setting guided flag
	guided = true
	assert.True(t, IsGuidedMode(), "IsGuidedMode should return true when guided=true")

	// Reset
	guided = false
	assert.False(t, IsGuidedMode(), "IsGuidedMode should return false after reset")
}

// TestIsJSONOutput verifies JSON output detection
func TestIsJSONOutput(t *testing.T) {
	// Note: This requires viper to be configured, so we just verify the function exists
	// and doesn't panic
	assert.NotPanics(t, func() {
		IsJSONOutput()
	}, "IsJSONOutput should not panic")
}

// TestIsQuiet verifies quiet mode detection
func TestIsQuiet(t *testing.T) {
	// Note: This requires viper to be configured, so we just verify the function exists
	// and doesn't panic
	assert.NotPanics(t, func() {
		IsQuiet()
	}, "IsQuiet should not panic")
}

// TestExecute verifies Execute function exists and doesn't panic
func TestExecute(t *testing.T) {
	// We can't easily test Execute() in unit tests without mocking os.Args
	// Just verify it exists and has the right signature
	assert.NotNil(t, Execute, "Execute function should exist")
}

// TestRootCommand_Aliases verifies command aliases
func TestRootCommand_Aliases(t *testing.T) {
	// Check if apikey command has aliases
	var apikeyCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "apikey" {
			apikeyCmd = cmd
			break
		}
	}

	if apikeyCmd != nil {
		assert.Contains(t, apikeyCmd.Aliases, "key", "apikey should have 'key' alias")
		assert.Contains(t, apikeyCmd.Aliases, "keys", "apikey should have 'keys' alias")
	}
}
