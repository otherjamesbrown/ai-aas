package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// --- benchmark run ---

var benchmarkRunCmd = &cobra.Command{
	Use:   "run",
	Short: "View and trigger benchmark runs",
	Long: `View and trigger benchmark runs for your organization.

A run is a single execution of a benchmark against a target.

Examples:
  ai-aas-org benchmark run list
  ai-aas-org benchmark run list --target my-test
  ai-aas-org benchmark run show <run-id>
  ai-aas-org benchmark run trigger my-test`,
}

func init() {
	benchmarkCmd.AddCommand(benchmarkRunCmd)
}

// --- benchmark run list ---

var benchmarkRunListCmd = &cobra.Command{
	Use:   "list",
	Short: "List benchmark runs",
	Long: `List benchmark runs for your organization.

Examples:
  ai-aas-org benchmark run list
  ai-aas-org benchmark run list --target my-test
  ai-aas-org benchmark run list --status completed
  ai-aas-org benchmark run list --json`,
	RunE: runBenchmarkRunList,
}

var (
	runListTarget   string
	runListStatus   string
	runListScenario string
	runListLimit    int
)

func init() {
	benchmarkRunCmd.AddCommand(benchmarkRunListCmd)
	benchmarkRunListCmd.Flags().StringVarP(&runListTarget, "target", "t", "", "Filter by target name or ID")
	benchmarkRunListCmd.Flags().StringVarP(&runListStatus, "status", "s", "", "Filter by status (pending, running, completed, failed, cancelled)")
	benchmarkRunListCmd.Flags().StringVar(&runListScenario, "scenario", "", "Filter by scenario name")
	benchmarkRunListCmd.Flags().IntVarP(&runListLimit, "limit", "n", 20, "Maximum number of runs to display")
}

func runBenchmarkRunList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAdminAPIClient()

	opts := api.ListBenchmarkRunsOptions{
		Status:       runListStatus,
		ScenarioName: runListScenario,
		Limit:        runListLimit,
	}

	// Resolve target name to ID if provided
	if runListTarget != "" {
		targetID, err := client.ResolveBenchmarkTargetID(ctx, runListTarget)
		if err != nil {
			return errors.NewOperationError("failed to find target", err.Error())
		}
		opts.TargetID = targetID
	}

	result, err := client.ListBenchmarkRuns(ctx, opts)
	if err != nil {
		return errors.NewOperationError("failed to list runs", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Runs)
	}

	if len(result.Runs) == 0 {
		output.InfoMsg("No benchmark runs found.")
		fmt.Println()
		fmt.Println("Trigger a run with:")
		fmt.Println("  ai-aas-org benchmark run trigger <target-name>")
		return nil
	}

	headers := []string{"ID", "Scenario", "Status", "Started", "Duration", "Throughput"}
	var rows [][]string
	for _, r := range result.Runs {
		started := "-"
		if r.StartedAt != nil {
			started = r.StartedAt.Format("2006-01-02 15:04")
		}
		duration := "-"
		if r.DurationSeconds != nil {
			duration = fmt.Sprintf("%ds", *r.DurationSeconds)
		}
		throughput := "-"
		if r.Results != nil && r.Results.RequestsPerSecond > 0 {
			throughput = fmt.Sprintf("%.1f req/s", r.Results.RequestsPerSecond)
		}

		// Truncate ID for display
		idDisplay := r.ID
		if len(idDisplay) > 8 {
			idDisplay = idDisplay[:8] + "..."
		}

		rows = append(rows, []string{
			idDisplay,
			r.ScenarioName,
			output.StatusBadge(r.Status),
			started,
			duration,
			throughput,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nShowing %d of %d runs\n", len(result.Runs), result.Pagination.Total)
	return nil
}

// --- benchmark run show ---

var benchmarkRunShowCmd = &cobra.Command{
	Use:   "show <run-id>",
	Short: "Show details for a benchmark run",
	Long: `Show detailed information and results for a benchmark run.

Displays run status, timing information, and performance metrics
including throughput, latency percentiles, and error rates.

Examples:
  ai-aas-org benchmark run show <run-id>
  ai-aas-org benchmark run show <run-id> --json`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkRunShow,
}

func init() {
	benchmarkRunCmd.AddCommand(benchmarkRunShowCmd)
}

func runBenchmarkRunShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	runID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAdminAPIClient()
	run, err := client.GetBenchmarkRun(ctx, runID)
	if err != nil {
		return errors.NewOperationError("failed to get run", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(run)
	}

	output.Header("Benchmark Run")
	output.KeyValue("ID", run.ID)
	output.KeyValue("Target ID", run.TargetID)
	output.KeyValue("Scenario", run.ScenarioName)
	output.KeyValue("Status", output.StatusBadge(run.Status))
	output.KeyValue("Triggered By", run.TriggeredBy)
	if run.TriggeredByUser != nil {
		output.KeyValue("Triggered By User", *run.TriggeredByUser)
	}

	fmt.Println()
	output.Header("Timing")
	if run.StartedAt != nil {
		output.KeyValue("Started", run.StartedAt.Format(time.RFC3339))
	}
	if run.CompletedAt != nil {
		output.KeyValue("Completed", run.CompletedAt.Format(time.RFC3339))
	}
	if run.DurationSeconds != nil {
		output.KeyValue("Duration", fmt.Sprintf("%d seconds", *run.DurationSeconds))
	}

	if run.Results != nil {
		fmt.Println()
		output.Header("Results")

		// Throughput
		output.KeyValue("Total Requests", fmt.Sprintf("%d", run.Results.RequestsTotal))
		output.KeyValue("Requests/sec", fmt.Sprintf("%.2f", run.Results.RequestsPerSecond))
		if run.Results.TokensPerSecond > 0 {
			output.KeyValue("Tokens/sec", fmt.Sprintf("%.2f", run.Results.TokensPerSecond))
		}

		// Latency
		fmt.Println()
		output.Header("Latency (ms)")
		output.KeyValue("P50", fmt.Sprintf("%.2f", run.Results.LatencyP50))
		output.KeyValue("P90", fmt.Sprintf("%.2f", run.Results.LatencyP90))
		output.KeyValue("P95", fmt.Sprintf("%.2f", run.Results.LatencyP95))
		output.KeyValue("P99", fmt.Sprintf("%.2f", run.Results.LatencyP99))
		output.KeyValue("Avg", fmt.Sprintf("%.2f", run.Results.LatencyAvg))

		// TTFT
		if run.Results.TTFTAvg > 0 {
			fmt.Println()
			output.Header("Time to First Token (ms)")
			output.KeyValue("P50", fmt.Sprintf("%.2f", run.Results.TTFTP50))
			output.KeyValue("P90", fmt.Sprintf("%.2f", run.Results.TTFTP90))
			output.KeyValue("P99", fmt.Sprintf("%.2f", run.Results.TTFTP99))
			output.KeyValue("Avg", fmt.Sprintf("%.2f", run.Results.TTFTAvg))
		}

		// Errors
		if run.Results.ErrorCount > 0 || run.Results.ErrorRate > 0 {
			fmt.Println()
			output.Header("Errors")
			output.KeyValue("Error Count", fmt.Sprintf("%d", run.Results.ErrorCount))
			output.KeyValue("Error Rate", fmt.Sprintf("%.2f%%", run.Results.ErrorRate*100))
		}

		// Raw metrics if available
		if len(run.Results.RawMetrics) > 0 {
			fmt.Println()
			output.Header("Raw Metrics")
			rawBytes, _ := json.MarshalIndent(run.Results.RawMetrics, "", "  ")
			fmt.Println(string(rawBytes))
		}
	}

	if run.ErrorMessage != nil && *run.ErrorMessage != "" {
		fmt.Println()
		output.Header("Error")
		fmt.Println(*run.ErrorMessage)
	}

	return nil
}

// --- benchmark run trigger ---

var benchmarkRunTriggerCmd = &cobra.Command{
	Use:   "trigger <target-name-or-id>",
	Short: "Trigger a benchmark run",
	Long: `Trigger a manual benchmark run for the specified target.

Examples:
  ai-aas-org benchmark run trigger my-test
  ai-aas-org benchmark run trigger my-test --json`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkRunTrigger,
}

func init() {
	benchmarkRunCmd.AddCommand(benchmarkRunTriggerCmd)
}

func runBenchmarkRunTrigger(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	nameOrID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := newAdminAPIClient()

	// Resolve target name to ID
	targetID, err := client.ResolveBenchmarkTargetID(ctx, nameOrID)
	if err != nil {
		return errors.NewOperationError("failed to find target", err.Error())
	}

	run, err := client.TriggerBenchmarkRun(ctx, targetID, nil)
	if err != nil {
		return errors.NewOperationError("failed to trigger run", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(run)
	}

	output.SuccessMsg("Benchmark run triggered")
	fmt.Println()
	output.KeyValue("Run ID", run.ID)
	output.KeyValue("Target ID", run.TargetID)
	output.KeyValue("Scenario", run.ScenarioName)
	output.KeyValue("Status", run.Status)
	fmt.Println()
	fmt.Println("Check the run status with:")
	fmt.Printf("  ai-aas-org benchmark run show %s\n", run.ID)

	return nil
}
