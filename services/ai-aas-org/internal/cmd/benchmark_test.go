package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBenchmarkCommand verifies the benchmark command structure
func TestBenchmarkCommand(t *testing.T) {
	require.NotNil(t, benchmarkCmd, "benchmarkCmd should not be nil")

	assert.Equal(t, "benchmark", benchmarkCmd.Use, "command Use should be 'benchmark'")
	assert.NotEmpty(t, benchmarkCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, benchmarkCmd.Long, "Long description should not be empty")

	// Verify subcommands exist
	expectedSubcommands := []string{"scenario", "target", "run"}
	for _, subcmd := range expectedSubcommands {
		found := false
		for _, c := range benchmarkCmd.Commands() {
			if c.Name() == subcmd {
				found = true
				break
			}
		}
		assert.True(t, found, "subcommand %q should exist", subcmd)
	}
}

// TestBenchmarkCommand_Examples verifies examples are provided
func TestBenchmarkCommand_Examples(t *testing.T) {
	assert.Contains(t, benchmarkCmd.Long, "Examples:", "Long description should contain examples")
	assert.Contains(t, benchmarkCmd.Long, "benchmark scenario", "Examples should show scenario command")
	assert.Contains(t, benchmarkCmd.Long, "benchmark target", "Examples should show target command")
	assert.Contains(t, benchmarkCmd.Long, "benchmark run", "Examples should show run command")
}

// --- Scenario Subcommand Tests ---

func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func TestBenchmarkScenarioCommand(t *testing.T) {
	scenarioCmd := findSubcommand(benchmarkCmd, "scenario")
	require.NotNil(t, scenarioCmd, "scenario subcommand should exist")

	assert.Equal(t, "scenario", scenarioCmd.Use, "command Use should be 'scenario'")
	assert.NotEmpty(t, scenarioCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, scenarioCmd.Long, "Long description should not be empty")

	// Verify subcommands
	expectedSubcommands := []string{"list", "show"}
	for _, subcmd := range expectedSubcommands {
		found := findSubcommand(scenarioCmd, subcmd)
		assert.NotNil(t, found, "subcommand %q should exist", subcmd)
	}
}

func TestBenchmarkScenarioListCommand(t *testing.T) {
	scenarioCmd := findSubcommand(benchmarkCmd, "scenario")
	require.NotNil(t, scenarioCmd)

	listCmd := findSubcommand(scenarioCmd, "list")
	require.NotNil(t, listCmd, "list subcommand should exist")

	assert.Equal(t, "list", listCmd.Use, "command Use should be 'list'")
	assert.NotEmpty(t, listCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, listCmd.Long, "Long description should not be empty")
	assert.NotNil(t, listCmd.RunE, "RunE function should be set")
}

func TestBenchmarkScenarioShowCommand(t *testing.T) {
	scenarioCmd := findSubcommand(benchmarkCmd, "scenario")
	require.NotNil(t, scenarioCmd)

	showCmd := findSubcommand(scenarioCmd, "show")
	require.NotNil(t, showCmd, "show subcommand should exist")

	assert.Equal(t, "show <scenario-name>", showCmd.Use, "command Use should be 'show <scenario-name>'")
	assert.NotEmpty(t, showCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, showCmd.Long, "Long description should not be empty")
	assert.NotNil(t, showCmd.RunE, "RunE function should be set")
	assert.NotNil(t, showCmd.Args, "Args validator should be set")
}

// --- Target Subcommand Tests ---

func TestBenchmarkTargetCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd, "target subcommand should exist")

	assert.Equal(t, "target", targetCmd.Use, "command Use should be 'target'")
	assert.NotEmpty(t, targetCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, targetCmd.Long, "Long description should not be empty")

	// Verify subcommands
	expectedSubcommands := []string{"list", "add", "show", "start", "stop", "remove"}
	for _, subcmd := range expectedSubcommands {
		found := findSubcommand(targetCmd, subcmd)
		assert.NotNil(t, found, "subcommand %q should exist", subcmd)
	}
}

func TestBenchmarkTargetListCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	listCmd := findSubcommand(targetCmd, "list")
	require.NotNil(t, listCmd, "list subcommand should exist")

	assert.Equal(t, "list", listCmd.Use, "command Use should be 'list'")
	assert.NotEmpty(t, listCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, listCmd.Long, "Long description should not be empty")
	assert.NotNil(t, listCmd.RunE, "RunE function should be set")
}

func TestBenchmarkTargetListCommand_Flags(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	listCmd := findSubcommand(targetCmd, "list")
	require.NotNil(t, listCmd)

	// Verify flags exist
	flagTests := []struct {
		name      string
		shorthand string
	}{
		{"environment", "e"},
		{"status", "s"},
		{"model", "m"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := listCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand, "flag %q should have shorthand %q", tt.name, tt.shorthand)
			}
		})
	}
}

func TestBenchmarkTargetAddCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	addCmd := findSubcommand(targetCmd, "add")
	require.NotNil(t, addCmd, "add subcommand should exist")

	assert.Equal(t, "add <name>", addCmd.Use, "command Use should be 'add <name>'")
	assert.NotEmpty(t, addCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, addCmd.Long, "Long description should not be empty")
	assert.NotNil(t, addCmd.RunE, "RunE function should be set")
	assert.NotNil(t, addCmd.Args, "Args validator should be set")
}

func TestBenchmarkTargetAddCommand_Flags(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	addCmd := findSubcommand(targetCmd, "add")
	require.NotNil(t, addCmd)

	// Verify required flags
	flagTests := []struct {
		name         string
		shorthand    string
		defaultValue string
	}{
		{"model", "m", ""},
		{"scenario", "s", ""},
		{"environment", "e", "development"},
		{"interval", "", "0"},
		{"schedule", "", "false"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := addCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand, "flag %q should have shorthand %q", tt.name, tt.shorthand)
			}
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q should default to %q", tt.name, tt.defaultValue)
			}
		})
	}
}

func TestBenchmarkTargetShowCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	showCmd := findSubcommand(targetCmd, "show")
	require.NotNil(t, showCmd, "show subcommand should exist")

	assert.Equal(t, "show <name-or-id>", showCmd.Use, "command Use should be 'show <name-or-id>'")
	assert.NotEmpty(t, showCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, showCmd.Long, "Long description should not be empty")
	assert.NotNil(t, showCmd.RunE, "RunE function should be set")
	assert.NotNil(t, showCmd.Args, "Args validator should be set")
}

func TestBenchmarkTargetStartCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	startCmd := findSubcommand(targetCmd, "start")
	require.NotNil(t, startCmd, "start subcommand should exist")

	assert.Equal(t, "start <name-or-id>", startCmd.Use, "command Use should be 'start <name-or-id>'")
	assert.NotEmpty(t, startCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, startCmd.Long, "Long description should not be empty")
	assert.NotNil(t, startCmd.RunE, "RunE function should be set")
	assert.NotNil(t, startCmd.Args, "Args validator should be set")
}

func TestBenchmarkTargetStopCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	stopCmd := findSubcommand(targetCmd, "stop")
	require.NotNil(t, stopCmd, "stop subcommand should exist")

	assert.Equal(t, "stop <name-or-id>", stopCmd.Use, "command Use should be 'stop <name-or-id>'")
	assert.NotEmpty(t, stopCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, stopCmd.Long, "Long description should not be empty")
	assert.NotNil(t, stopCmd.RunE, "RunE function should be set")
	assert.NotNil(t, stopCmd.Args, "Args validator should be set")
}

func TestBenchmarkTargetRemoveCommand(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	removeCmd := findSubcommand(targetCmd, "remove")
	require.NotNil(t, removeCmd, "remove subcommand should exist")

	assert.Equal(t, "remove <name-or-id>", removeCmd.Use, "command Use should be 'remove <name-or-id>'")
	assert.NotEmpty(t, removeCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, removeCmd.Long, "Long description should not be empty")
	assert.NotNil(t, removeCmd.RunE, "RunE function should be set")
	assert.NotNil(t, removeCmd.Args, "Args validator should be set")

	// Verify --force flag
	forceFlag := removeCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "force flag should exist")
	assert.Equal(t, "false", forceFlag.DefValue, "force flag should default to false")
}

// --- Run Subcommand Tests ---

func TestBenchmarkRunCommand(t *testing.T) {
	runCmd := findSubcommand(benchmarkCmd, "run")
	require.NotNil(t, runCmd, "run subcommand should exist")

	assert.Equal(t, "run", runCmd.Use, "command Use should be 'run'")
	assert.NotEmpty(t, runCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, runCmd.Long, "Long description should not be empty")

	// Verify subcommands
	expectedSubcommands := []string{"list", "show", "trigger"}
	for _, subcmd := range expectedSubcommands {
		found := findSubcommand(runCmd, subcmd)
		assert.NotNil(t, found, "subcommand %q should exist", subcmd)
	}
}

func TestBenchmarkRunListCommand(t *testing.T) {
	runCmd := findSubcommand(benchmarkCmd, "run")
	require.NotNil(t, runCmd)

	listCmd := findSubcommand(runCmd, "list")
	require.NotNil(t, listCmd, "list subcommand should exist")

	assert.Equal(t, "list", listCmd.Use, "command Use should be 'list'")
	assert.NotEmpty(t, listCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, listCmd.Long, "Long description should not be empty")
	assert.NotNil(t, listCmd.RunE, "RunE function should be set")
}

func TestBenchmarkRunListCommand_Flags(t *testing.T) {
	runCmd := findSubcommand(benchmarkCmd, "run")
	require.NotNil(t, runCmd)

	listCmd := findSubcommand(runCmd, "list")
	require.NotNil(t, listCmd)

	// Verify flags exist
	flagTests := []struct {
		name         string
		shorthand    string
		defaultValue string
	}{
		{"target", "t", ""},
		{"status", "s", ""},
		{"scenario", "", ""},
		{"limit", "n", "20"},
	}

	for _, tt := range flagTests {
		t.Run(tt.name, func(t *testing.T) {
			flag := listCmd.Flags().Lookup(tt.name)
			assert.NotNil(t, flag, "flag %q should exist", tt.name)
			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand, "flag %q should have shorthand %q", tt.name, tt.shorthand)
			}
			if tt.defaultValue != "" {
				assert.Equal(t, tt.defaultValue, flag.DefValue, "flag %q should default to %q", tt.name, tt.defaultValue)
			}
		})
	}
}

func TestBenchmarkRunShowCommand(t *testing.T) {
	runCmd := findSubcommand(benchmarkCmd, "run")
	require.NotNil(t, runCmd)

	showCmd := findSubcommand(runCmd, "show")
	require.NotNil(t, showCmd, "show subcommand should exist")

	assert.Equal(t, "show <run-id>", showCmd.Use, "command Use should be 'show <run-id>'")
	assert.NotEmpty(t, showCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, showCmd.Long, "Long description should not be empty")
	assert.NotNil(t, showCmd.RunE, "RunE function should be set")
	assert.NotNil(t, showCmd.Args, "Args validator should be set")
}

func TestBenchmarkRunTriggerCommand(t *testing.T) {
	runCmd := findSubcommand(benchmarkCmd, "run")
	require.NotNil(t, runCmd)

	triggerCmd := findSubcommand(runCmd, "trigger")
	require.NotNil(t, triggerCmd, "trigger subcommand should exist")

	assert.Equal(t, "trigger <target-name-or-id>", triggerCmd.Use, "command Use should be 'trigger <target-name-or-id>'")
	assert.NotEmpty(t, triggerCmd.Short, "Short description should not be empty")
	assert.NotEmpty(t, triggerCmd.Long, "Long description should not be empty")
	assert.NotNil(t, triggerCmd.RunE, "RunE function should be set")
	assert.NotNil(t, triggerCmd.Args, "Args validator should be set")
}

// --- Examples Verification Tests ---

func TestBenchmarkScenarioCommands_Examples(t *testing.T) {
	scenarioCmd := findSubcommand(benchmarkCmd, "scenario")
	require.NotNil(t, scenarioCmd)

	assert.Contains(t, scenarioCmd.Long, "Examples:", "scenario Long description should contain examples")

	listCmd := findSubcommand(scenarioCmd, "list")
	require.NotNil(t, listCmd)
	assert.Contains(t, listCmd.Long, "Examples:", "list Long description should contain examples")

	showCmd := findSubcommand(scenarioCmd, "show")
	require.NotNil(t, showCmd)
	assert.Contains(t, showCmd.Long, "Examples:", "show Long description should contain examples")
}

func TestBenchmarkTargetCommands_Examples(t *testing.T) {
	targetCmd := findSubcommand(benchmarkCmd, "target")
	require.NotNil(t, targetCmd)

	assert.Contains(t, targetCmd.Long, "Examples:", "target Long description should contain examples")

	listCmd := findSubcommand(targetCmd, "list")
	require.NotNil(t, listCmd)
	assert.Contains(t, listCmd.Long, "Examples:", "list Long description should contain examples")

	addCmd := findSubcommand(targetCmd, "add")
	require.NotNil(t, addCmd)
	assert.Contains(t, addCmd.Long, "Examples:", "add Long description should contain examples")

	showCmd := findSubcommand(targetCmd, "show")
	require.NotNil(t, showCmd)
	assert.Contains(t, showCmd.Long, "Examples:", "show Long description should contain examples")

	startCmd := findSubcommand(targetCmd, "start")
	require.NotNil(t, startCmd)
	assert.Contains(t, startCmd.Long, "Examples:", "start Long description should contain examples")

	stopCmd := findSubcommand(targetCmd, "stop")
	require.NotNil(t, stopCmd)
	assert.Contains(t, stopCmd.Long, "Examples:", "stop Long description should contain examples")

	removeCmd := findSubcommand(targetCmd, "remove")
	require.NotNil(t, removeCmd)
	assert.Contains(t, removeCmd.Long, "Examples:", "remove Long description should contain examples")
	assert.Contains(t, removeCmd.Long, "irreversible", "remove Long description should warn about irreversibility")
}

func TestBenchmarkRunCommands_Examples(t *testing.T) {
	runCmd := findSubcommand(benchmarkCmd, "run")
	require.NotNil(t, runCmd)

	assert.Contains(t, runCmd.Long, "Examples:", "run Long description should contain examples")

	listCmd := findSubcommand(runCmd, "list")
	require.NotNil(t, listCmd)
	assert.Contains(t, listCmd.Long, "Examples:", "list Long description should contain examples")

	showCmd := findSubcommand(runCmd, "show")
	require.NotNil(t, showCmd)
	assert.Contains(t, showCmd.Long, "Examples:", "show Long description should contain examples")

	triggerCmd := findSubcommand(runCmd, "trigger")
	require.NotNil(t, triggerCmd)
	assert.Contains(t, triggerCmd.Long, "Examples:", "trigger Long description should contain examples")
}
