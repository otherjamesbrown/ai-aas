package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// --- benchmark scenario ---

var benchmarkScenarioCmd = &cobra.Command{
	Use:   "scenario",
	Short: "View benchmark scenarios",
	Long: `View available benchmark scenarios.

Scenarios are pre-defined benchmark configurations that define:
  • Request types and patterns
  • Input/output token distributions
  • Concurrency and load profiles

Examples:
  ai-aas-org benchmark scenario list
  ai-aas-org benchmark scenario show short-qa`,
}

func init() {
	benchmarkCmd.AddCommand(benchmarkScenarioCmd)
}

// --- benchmark scenario list ---

var benchmarkScenarioListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available benchmark scenarios",
	Long: `List all available benchmark scenarios.

Scenarios are platform-defined and cannot be modified by organization admins.

Examples:
  ai-aas-org benchmark scenario list
  ai-aas-org benchmark scenario list --json`,
	RunE: runBenchmarkScenarioList,
}

func init() {
	benchmarkScenarioCmd.AddCommand(benchmarkScenarioListCmd)
}

func runBenchmarkScenarioList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAdminAPIClient()
	result, err := client.ListBenchmarkScenarios(ctx, 100, 0)
	if err != nil {
		return errors.NewOperationError("failed to list scenarios", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Scenarios)
	}

	if len(result.Scenarios) == 0 {
		output.InfoMsg("No benchmark scenarios available.")
		fmt.Println()
		fmt.Println("Contact your platform administrator to configure scenarios.")
		return nil
	}

	headers := []string{"NAME", "VERSION", "DESCRIPTION"}
	var rows [][]string
	for _, s := range result.Scenarios {
		desc := s.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		rows = append(rows, []string{
			s.Name,
			s.Version,
			desc,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d scenarios\n", len(result.Scenarios))
	return nil
}

// --- benchmark scenario show ---

var benchmarkScenarioShowCmd = &cobra.Command{
	Use:   "show <scenario-name>",
	Short: "Show details for a benchmark scenario",
	Long: `Show detailed information about a benchmark scenario.

Displays the scenario configuration including request patterns,
token distributions, and other benchmark parameters.

Examples:
  ai-aas-org benchmark scenario show short-qa
  ai-aas-org benchmark scenario show high-throughput --json`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkScenarioShow,
}

func init() {
	benchmarkScenarioCmd.AddCommand(benchmarkScenarioShowCmd)
}

func runBenchmarkScenarioShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	scenarioName := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAdminAPIClient()
	scenario, err := client.GetBenchmarkScenario(ctx, scenarioName)
	if err != nil {
		return errors.NewOperationError("failed to get scenario", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(scenario)
	}

	output.Header("Benchmark Scenario")
	output.KeyValue("Name", scenario.Name)
	output.KeyValue("Version", scenario.Version)
	if scenario.Description != "" {
		output.KeyValue("Description", scenario.Description)
	}
	output.KeyValue("Synced At", scenario.SyncedAt.Format(time.RFC3339))

	if len(scenario.Config) > 0 {
		fmt.Println()
		output.Header("Configuration")
		configBytes, _ := json.MarshalIndent(scenario.Config, "", "  ")
		fmt.Println(string(configBytes))
	}

	return nil
}
