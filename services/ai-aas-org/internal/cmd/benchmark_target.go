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

// --- benchmark target ---

var benchmarkTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "Manage benchmark targets",
	Long: `Manage benchmark targets for your organization.

A target combines a model with a scenario to define what to benchmark.
Each target can be started/stopped to control when benchmarks run.

Examples:
  ai-aas-org benchmark target list
  ai-aas-org benchmark target add my-test --model llama-7b --scenario short-qa -e development
  ai-aas-org benchmark target show my-test
  ai-aas-org benchmark target start my-test
  ai-aas-org benchmark target stop my-test
  ai-aas-org benchmark target remove my-test`,
}

func init() {
	benchmarkCmd.AddCommand(benchmarkTargetCmd)
}

// --- benchmark target list ---

var benchmarkTargetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List benchmark targets",
	Long: `List all benchmark targets for your organization.

Examples:
  ai-aas-org benchmark target list
  ai-aas-org benchmark target list --status active
  ai-aas-org benchmark target list --environment development
  ai-aas-org benchmark target list --json`,
	RunE: runBenchmarkTargetList,
}

var (
	targetListEnvironment string
	targetListStatus      string
	targetListModel       string
)

func init() {
	benchmarkTargetCmd.AddCommand(benchmarkTargetListCmd)
	benchmarkTargetListCmd.Flags().StringVarP(&targetListEnvironment, "environment", "e", "", "Filter by environment (development, staging, production)")
	benchmarkTargetListCmd.Flags().StringVarP(&targetListStatus, "status", "s", "", "Filter by status (active, paused, disabled)")
	benchmarkTargetListCmd.Flags().StringVarP(&targetListModel, "model", "m", "", "Filter by model name")
}

func runBenchmarkTargetList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	result, err := client.ListBenchmarkTargets(ctx, api.ListBenchmarkTargetsOptions{
		Environment: targetListEnvironment,
		Status:      targetListStatus,
		ModelName:   targetListModel,
		Limit:       100,
	})
	if err != nil {
		return errors.NewOperationError("failed to list targets", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Targets)
	}

	if len(result.Targets) == 0 {
		output.InfoMsg("No benchmark targets found.")
		fmt.Println()
		fmt.Println("Create a target with:")
		fmt.Println("  ai-aas-org benchmark target add <name> --model <model> --scenario <scenario>")
		return nil
	}

	headers := []string{"NAME", "MODEL", "SCENARIO", "ENV", "STATUS", "LAST RUN"}
	var rows [][]string
	for _, t := range result.Targets {
		lastRun := "-"
		if t.LastRunAt != nil {
			lastRun = t.LastRunAt.Format("2006-01-02 15:04")
			if t.LastRunStatus != nil {
				lastRun += " (" + *t.LastRunStatus + ")"
			}
		}
		rows = append(rows, []string{
			t.Name,
			t.ModelName,
			t.ScenarioName,
			t.Environment,
			output.StatusBadge(t.Status),
			lastRun,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d targets\n", len(result.Targets))
	return nil
}

// --- benchmark target add ---

var benchmarkTargetAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new benchmark target",
	Long: `Create a new benchmark target for your organization.

A target defines which model to benchmark and with which scenario.

Examples:
  ai-aas-org benchmark target add qa-bench --model llama-7b --scenario short-qa -e development
  ai-aas-org benchmark target add perf-test --model gpt-4 --scenario high-throughput -e staging`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkTargetAdd,
}

var (
	targetAddModel       string
	targetAddScenario    string
	targetAddEnvironment string
	targetAddInterval    int
	targetAddSchedule    bool
)

func init() {
	benchmarkTargetCmd.AddCommand(benchmarkTargetAddCmd)
	benchmarkTargetAddCmd.Flags().StringVarP(&targetAddModel, "model", "m", "", "Model name to benchmark (required)")
	benchmarkTargetAddCmd.Flags().StringVarP(&targetAddScenario, "scenario", "s", "", "Scenario name to use (required)")
	benchmarkTargetAddCmd.Flags().StringVarP(&targetAddEnvironment, "environment", "e", "development", "Environment (development, staging, production)")
	benchmarkTargetAddCmd.Flags().IntVar(&targetAddInterval, "interval", 0, "Scheduled run interval in seconds (minimum 60)")
	benchmarkTargetAddCmd.Flags().BoolVar(&targetAddSchedule, "schedule", false, "Enable scheduled runs")
	benchmarkTargetAddCmd.MarkFlagRequired("model")
	benchmarkTargetAddCmd.MarkFlagRequired("scenario")
}

func runBenchmarkTargetAdd(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	targetName := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := api.CreateBenchmarkTargetRequest{
		Name:         targetName,
		ModelName:    targetAddModel,
		ScenarioName: targetAddScenario,
		Environment:  targetAddEnvironment,
	}

	if targetAddInterval > 0 {
		req.IntervalSeconds = &targetAddInterval
	}
	if targetAddSchedule {
		req.ScheduleEnabled = &targetAddSchedule
	}

	client := newAPIClient()
	target, err := client.CreateBenchmarkTarget(ctx, req)
	if err != nil {
		return errors.NewOperationError("failed to create target", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(target)
	}

	output.SuccessMsg("Created benchmark target '%s'", target.Name)
	fmt.Println()
	output.KeyValue("ID", target.ID)
	output.KeyValue("Model", target.ModelName)
	output.KeyValue("Scenario", target.ScenarioName)
	output.KeyValue("Environment", target.Environment)
	output.KeyValue("Status", target.Status)
	fmt.Println()
	fmt.Println("To start the target:")
	fmt.Printf("  ai-aas-org benchmark target start %s\n", target.Name)

	return nil
}

// --- benchmark target show ---

var benchmarkTargetShowCmd = &cobra.Command{
	Use:   "show <name-or-id>",
	Short: "Show details for a benchmark target",
	Long: `Show detailed information about a benchmark target.

Examples:
  ai-aas-org benchmark target show my-test
  ai-aas-org benchmark target show <uuid> --json`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkTargetShow,
}

func init() {
	benchmarkTargetCmd.AddCommand(benchmarkTargetShowCmd)
}

func runBenchmarkTargetShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	nameOrID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve name to ID
	targetID, err := client.ResolveBenchmarkTargetID(ctx, nameOrID)
	if err != nil {
		return errors.NewOperationError("failed to find target", err.Error())
	}

	target, err := client.GetBenchmarkTarget(ctx, targetID)
	if err != nil {
		return errors.NewOperationError("failed to get target", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(target)
	}

	output.Header("Benchmark Target")
	output.KeyValue("Name", target.Name)
	output.KeyValue("ID", target.ID)
	output.KeyValue("Model", target.ModelName)
	output.KeyValue("Scenario", target.ScenarioName)
	output.KeyValue("Environment", target.Environment)
	output.KeyValue("Status", output.StatusBadge(target.Status))

	if target.IntervalSeconds != nil {
		output.KeyValue("Interval", fmt.Sprintf("%d seconds", *target.IntervalSeconds))
	}
	output.KeyValue("Scheduled", fmt.Sprintf("%v", target.ScheduleEnabled))

	fmt.Println()
	output.Header("Run History")
	if target.LastRunAt != nil {
		output.KeyValue("Last Run", target.LastRunAt.Format(time.RFC3339))
		if target.LastRunStatus != nil {
			output.KeyValue("Last Status", *target.LastRunStatus)
		}
	} else {
		output.InfoMsg("No runs yet")
	}

	output.KeyValue("Created", target.CreatedAt.Format(time.RFC3339))
	output.KeyValue("Updated", target.UpdatedAt.Format(time.RFC3339))

	return nil
}

// --- benchmark target start ---

var benchmarkTargetStartCmd = &cobra.Command{
	Use:   "start <name-or-id>",
	Short: "Start a benchmark target",
	Long: `Start a benchmark target to enable scheduled runs.

Examples:
  ai-aas-org benchmark target start my-test`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkTargetStart,
}

func init() {
	benchmarkTargetCmd.AddCommand(benchmarkTargetStartCmd)
}

func runBenchmarkTargetStart(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	nameOrID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve name to ID
	targetID, err := client.ResolveBenchmarkTargetID(ctx, nameOrID)
	if err != nil {
		return errors.NewOperationError("failed to find target", err.Error())
	}

	target, err := client.StartBenchmarkTarget(ctx, targetID)
	if err != nil {
		return errors.NewOperationError("failed to start target", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(target)
	}

	output.SuccessMsg("Started benchmark target '%s'", target.Name)
	output.KeyValue("Status", output.StatusBadge(target.Status))

	return nil
}

// --- benchmark target stop ---

var benchmarkTargetStopCmd = &cobra.Command{
	Use:   "stop <name-or-id>",
	Short: "Stop a benchmark target",
	Long: `Stop a benchmark target to disable scheduled runs.

Examples:
  ai-aas-org benchmark target stop my-test`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkTargetStop,
}

func init() {
	benchmarkTargetCmd.AddCommand(benchmarkTargetStopCmd)
}

func runBenchmarkTargetStop(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	nameOrID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve name to ID
	targetID, err := client.ResolveBenchmarkTargetID(ctx, nameOrID)
	if err != nil {
		return errors.NewOperationError("failed to find target", err.Error())
	}

	target, err := client.StopBenchmarkTarget(ctx, targetID)
	if err != nil {
		return errors.NewOperationError("failed to stop target", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(target)
	}

	output.SuccessMsg("Stopped benchmark target '%s'", target.Name)
	output.KeyValue("Status", output.StatusBadge(target.Status))

	return nil
}

// --- benchmark target remove ---

var benchmarkTargetRemoveCmd = &cobra.Command{
	Use:   "remove <name-or-id>",
	Short: "Remove a benchmark target",
	Long: `Remove a benchmark target and all its run history.

This action is irreversible.

Examples:
  ai-aas-org benchmark target remove my-test
  ai-aas-org benchmark target remove my-test --force`,
	Args: cobra.ExactArgs(1),
	RunE: runBenchmarkTargetRemove,
}

var targetRemoveForce bool

func init() {
	benchmarkTargetCmd.AddCommand(benchmarkTargetRemoveCmd)
	benchmarkTargetRemoveCmd.Flags().BoolVarP(&targetRemoveForce, "force", "f", false, "Skip confirmation prompt")
}

func runBenchmarkTargetRemove(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	nameOrID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve name to ID
	targetID, err := client.ResolveBenchmarkTargetID(ctx, nameOrID)
	if err != nil {
		return errors.NewOperationError("failed to find target", err.Error())
	}

	// Get target for display name
	target, err := client.GetBenchmarkTarget(ctx, targetID)
	if err != nil {
		return errors.NewOperationError("failed to get target", err.Error())
	}

	if !targetRemoveForce {
		output.WarningMsg("You are about to delete benchmark target '%s'", target.Name)
		fmt.Println()
		fmt.Println("This will delete all run history for this target.")
		fmt.Println()
		fmt.Println("Use --force to skip this confirmation.")
		return nil
	}

	if err := client.DeleteBenchmarkTarget(ctx, targetID); err != nil {
		return errors.NewOperationError("failed to delete target", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(map[string]string{
			"status":  "deleted",
			"target":  target.Name,
			"id":      target.ID,
		})
	}

	output.SuccessMsg("Deleted benchmark target '%s'", target.Name)

	return nil
}
