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

	headers := []string{"Name", "Profile", "Rate", "Duration", "Tokens", "Description"}
	var rows [][]string
	for _, s := range result.Scenarios {
		// Extract useful info from config
		profile := "-"
		rate := "-"
		duration := "-"
		tokens := "-"

		if defaults, ok := s.Config["defaults"].(map[string]interface{}); ok {
			if p, ok := defaults["profile"].(string); ok {
				profile = p
			}
			if r, ok := defaults["rate"].(float64); ok {
				rate = fmt.Sprintf("%.0f/s", r)
			}
			if maxSec, ok := defaults["max_seconds"].(float64); ok {
				if maxSec >= 60 {
					duration = fmt.Sprintf("%.0fm", maxSec/60)
				} else {
					duration = fmt.Sprintf("%.0fs", maxSec)
				}
			}
		}

		if data, ok := s.Config["data"].(map[string]interface{}); ok {
			promptTok := 0.0
			outputTok := 0.0
			if p, ok := data["prompt_tokens"].(float64); ok {
				promptTok = p
			}
			if o, ok := data["output_tokens"].(float64); ok {
				outputTok = o
			}
			if promptTok > 0 || outputTok > 0 {
				tokens = fmt.Sprintf("%.0f→%.0f", promptTok, outputTok)
			}
		}

		// Truncate description - take first line only
		desc := s.Description
		if idx := indexAny(desc, "\n\r"); idx > 0 {
			desc = desc[:idx]
		}
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}

		rows = append(rows, []string{
			s.Name,
			profile,
			rate,
			duration,
			tokens,
			desc,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d scenarios\n", len(result.Scenarios))
	fmt.Println("\nUse 'ai-aas-org benchmark scenario show <name>' for full details.")
	return nil
}

// indexAny returns the index of the first occurrence of any char in chars, or -1
func indexAny(s string, chars string) int {
	for i, c := range s {
		for _, ch := range chars {
			if c == ch {
				return i
			}
		}
	}
	return -1
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
