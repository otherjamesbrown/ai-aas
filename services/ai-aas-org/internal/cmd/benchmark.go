package cmd

import (
	"github.com/spf13/cobra"
)

// benchmarkCmd represents the benchmark command
var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Manage benchmark tests for your organization's models",
	Long: `Manage benchmark tests for your organization's AI models.

Benchmarks help you measure model performance including throughput,
latency, and token processing speeds.

Subcommands:
  scenario  View available benchmark scenarios
  target    Manage benchmark targets (model + scenario combinations)
  run       View and trigger benchmark runs
  compare   Compare results between benchmark runs

Examples:
  ai-aas-org benchmark scenario list
  ai-aas-org benchmark target list
  ai-aas-org benchmark target add my-benchmark --model llama-7b --scenario short-qa
  ai-aas-org benchmark target start my-benchmark
  ai-aas-org benchmark run list --target my-benchmark
  ai-aas-org benchmark compare <run-1> <run-2> --show-delta`,
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)
}
