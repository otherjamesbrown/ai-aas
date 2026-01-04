package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// --- benchmark compare ---

var benchmarkCompareCmd = &cobra.Command{
	Use:   "compare <run-1> <run-2>",
	Short: "Compare results between two benchmark runs",
	Long: `Compare results between two benchmark runs.

Displays side-by-side metrics and percentage differences for throughput,
latency, and token rates.

Examples:
  ai-aas-org benchmark compare <run-1> <run-2>
  ai-aas-org benchmark compare <run-1> <run-2> --show-delta
  ai-aas-org benchmark compare <run-1> <run-2> --json`,
	Args: cobra.ExactArgs(2),
	RunE: runBenchmarkCompare,
}

var showDelta bool

func init() {
	benchmarkCmd.AddCommand(benchmarkCompareCmd)
	benchmarkCompareCmd.Flags().BoolVar(&showDelta, "show-delta", false, "Show percentage changes")
}

// CompareResult represents the comparison between two benchmark runs
type CompareResult struct {
	Run1   *api.BenchmarkRun `json:"run1"`
	Run2   *api.BenchmarkRun `json:"run2"`
	Deltas *DeltaMetrics     `json:"deltas,omitempty"`
}

// DeltaMetrics represents percentage changes between runs
type DeltaMetrics struct {
	ThroughputDelta float64 `json:"throughput_delta"`
	LatencyP50Delta float64 `json:"latency_p50_delta"`
	LatencyP90Delta float64 `json:"latency_p90_delta"`
	LatencyP99Delta float64 `json:"latency_p99_delta"`
	TokenRateDelta  float64 `json:"token_rate_delta"`
	ErrorRateDelta  float64 `json:"error_rate_delta"`
}

func runBenchmarkCompare(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	run1ID := args[0]
	run2ID := args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAdminAPIClient()

	// Fetch both runs
	run1, err := client.GetBenchmarkRun(ctx, run1ID)
	if err != nil {
		return errors.NewNotFoundError("benchmark run", run1ID)
	}

	run2, err := client.GetBenchmarkRun(ctx, run2ID)
	if err != nil {
		return errors.NewNotFoundError("benchmark run", run2ID)
	}

	// Validate both runs are completed
	if run1.Status != "completed" {
		return errors.NewOperationError(
			fmt.Sprintf("run %s is not completed (status: %s)", run1ID, run1.Status),
			"Wait for the run to complete before comparing",
		)
	}
	if run2.Status != "completed" {
		return errors.NewOperationError(
			fmt.Sprintf("run %s is not completed (status: %s)", run2ID, run2.Status),
			"Wait for the run to complete before comparing",
		)
	}

	// Validate results exist
	if run1.Results == nil {
		return errors.NewOperationError(fmt.Sprintf("run %s has no results", run1ID), "")
	}
	if run2.Results == nil {
		return errors.NewOperationError(fmt.Sprintf("run %s has no results", run2ID), "")
	}

	// Calculate deltas
	deltas := calculateDeltas(run1.Results, run2.Results)

	result := CompareResult{
		Run1:   run1,
		Run2:   run2,
		Deltas: deltas,
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	// Display comparison
	displayComparison(run1, run2, deltas)

	return nil
}

func calculateDeltas(r1, r2 *api.BenchmarkResults) *DeltaMetrics {
	deltas := &DeltaMetrics{}

	// Throughput delta
	if r1.RequestsPerSecond > 0 {
		deltas.ThroughputDelta = ((r2.RequestsPerSecond - r1.RequestsPerSecond) / r1.RequestsPerSecond) * 100
	}

	// Latency deltas (lower is better, so negative delta is improvement)
	if r1.LatencyP50 > 0 {
		deltas.LatencyP50Delta = ((r2.LatencyP50 - r1.LatencyP50) / r1.LatencyP50) * 100
	}
	if r1.LatencyP90 > 0 {
		deltas.LatencyP90Delta = ((r2.LatencyP90 - r1.LatencyP90) / r1.LatencyP90) * 100
	}
	if r1.LatencyP99 > 0 {
		deltas.LatencyP99Delta = ((r2.LatencyP99 - r1.LatencyP99) / r1.LatencyP99) * 100
	}

	// Token rate delta
	if r1.TokensPerSecond > 0 {
		deltas.TokenRateDelta = ((r2.TokensPerSecond - r1.TokensPerSecond) / r1.TokensPerSecond) * 100
	}

	// Error rate delta
	if r1.ErrorRate > 0 {
		deltas.ErrorRateDelta = ((r2.ErrorRate - r1.ErrorRate) / r1.ErrorRate) * 100
	} else if r2.ErrorRate > 0 {
		// Special case: r1 had no errors, r2 has errors
		deltas.ErrorRateDelta = 100
	}

	return deltas
}

func displayComparison(run1, run2 *api.BenchmarkRun, deltas *DeltaMetrics) {
	output.Header("Benchmark Comparison")
	fmt.Println()

	// Run details
	fmt.Printf("Run 1: %s (Scenario: %s)\n", run1.ID, run1.ScenarioName)
	fmt.Printf("Run 2: %s (Scenario: %s)\n", run2.ID, run2.ScenarioName)
	fmt.Println()

	// Throughput comparison
	output.Header("Throughput")
	displayMetricRow("Requests/sec", run1.Results.RequestsPerSecond, run2.Results.RequestsPerSecond, deltas.ThroughputDelta, true)
	displayMetricRow("Tokens/sec", run1.Results.TokensPerSecond, run2.Results.TokensPerSecond, deltas.TokenRateDelta, true)
	fmt.Println()

	// Latency comparison (lower is better)
	output.Header("Latency (ms)")
	displayMetricRow("P50", run1.Results.LatencyP50, run2.Results.LatencyP50, deltas.LatencyP50Delta, false)
	displayMetricRow("P90", run1.Results.LatencyP90, run2.Results.LatencyP90, deltas.LatencyP90Delta, false)
	displayMetricRow("P99", run1.Results.LatencyP99, run2.Results.LatencyP99, deltas.LatencyP99Delta, false)
	displayMetricRow("Avg", run1.Results.LatencyAvg, run2.Results.LatencyAvg, 0, false)
	fmt.Println()

	// TTFT comparison if available
	if run1.Results.TTFTAvg > 0 || run2.Results.TTFTAvg > 0 {
		output.Header("Time to First Token (ms)")
		displayMetricRow("P50", run1.Results.TTFTP50, run2.Results.TTFTP50, 0, false)
		displayMetricRow("P90", run1.Results.TTFTP90, run2.Results.TTFTP90, 0, false)
		displayMetricRow("P99", run1.Results.TTFTP99, run2.Results.TTFTP99, 0, false)
		displayMetricRow("Avg", run1.Results.TTFTAvg, run2.Results.TTFTAvg, 0, false)
		fmt.Println()
	}

	// Error comparison
	if run1.Results.ErrorCount > 0 || run2.Results.ErrorCount > 0 {
		output.Header("Errors")
		fmt.Printf("%-20s  %12.2f%%  %12.2f%%", "Error Rate", run1.Results.ErrorRate*100, run2.Results.ErrorRate*100)
		if showDelta && deltas.ErrorRateDelta != 0 {
			fmt.Printf("  %s", formatDelta(deltas.ErrorRateDelta, false))
		}
		fmt.Println()
		fmt.Println()
	}
}

func displayMetricRow(name string, val1, val2, delta float64, higherIsBetter bool) {
	fmt.Printf("%-20s  %12.2f  %12.2f", name, val1, val2)
	if showDelta && delta != 0 {
		fmt.Printf("  %s", formatDelta(delta, higherIsBetter))
	}
	fmt.Println()
}

func formatDelta(delta float64, higherIsBetter bool) string {
	// Determine if change is improvement or regression
	isImprovement := (delta > 0 && higherIsBetter) || (delta < 0 && !higherIsBetter)

	prefix := "+"
	if delta < 0 {
		prefix = ""
	}

	marker := ""
	if isImprovement {
		marker = " ✓ improvement"
	} else if delta != 0 {
		marker = " ✗ regression"
	}

	return fmt.Sprintf("%s%.1f%%%s", prefix, delta, marker)
}
