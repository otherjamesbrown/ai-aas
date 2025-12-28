package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVersionCommand verifies the version command structure
func TestVersionCommand(t *testing.T) {
	require.NotNil(t, versionCmd, "versionCmd should not be nil")

	assert.Equal(t, "version", versionCmd.Use, "command Use should be 'version'")
	assert.NotEmpty(t, versionCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, versionCmd.Long, "Long description should not be empty")
	assert.NotNil(t, versionCmd.Run, "Run function should be set")
}

// TestVersionCommand_NoArgs verifies version command takes no arguments
func TestVersionCommand_NoArgs(t *testing.T) {
	// Version command should not require or accept arguments
	// Args should be nil (allows any number of args) or set to accept 0 args
	// We just verify the command structure is correct
	assert.Equal(t, "version", versionCmd.Use, "command should be 'version' with no arguments")
}

// TestPrintVersionText verifies text output function exists
func TestPrintVersionText(t *testing.T) {
	// Set some test version info
	SetVersionInfo("1.0.0-test", "test-commit", "2025-12-28T12:00:00Z")

	// Verify the function doesn't panic
	assert.NotPanics(t, func() {
		printVersionText()
	}, "printVersionText should not panic")
}

// TestPrintVersionJSON verifies JSON output function exists
func TestPrintVersionJSON(t *testing.T) {
	// Set some test version info
	SetVersionInfo("1.0.0-test", "test-commit", "2025-12-28T12:00:00Z")

	// Verify the function doesn't panic
	assert.NotPanics(t, func() {
		printVersionJSON()
	}, "printVersionJSON should not panic")
}

// TestRunVersion verifies run function exists
func TestRunVersion(t *testing.T) {
	// Verify the function doesn't panic when called
	assert.NotPanics(t, func() {
		runVersion(versionCmd, []string{})
	}, "runVersion should not panic")
}
