// Package benchmark provides CLI commands for benchmark management.
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/benchmark"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// NewBenchmarkCommand creates the benchmark parent command
func NewBenchmarkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Manage benchmark scenarios, targets, and runs",
		Long: `Manage performance benchmarks for deployed models.

The benchmark system allows you to:
  - Define benchmark scenarios (test configurations)
  - Create targets linking models to scenarios
  - Run benchmarks and view results

Workflow:
  1. List available scenarios:     ai-aas-cli benchmark scenario list
  2. Create a benchmark target:    ai-aas-cli benchmark target add <name> --model <model> --scenario <scenario>
  3. Start benchmarking:           ai-aas-cli benchmark target start <name>
  4. View run results:             ai-aas-cli benchmark run list --target <name>

Examples:
  # List all benchmark scenarios
  ai-aas-cli benchmark scenario list

  # Create a benchmark target for a model
  ai-aas-cli benchmark target add llama-7b-qa --model llama-7b --scenario short-qa -e development

  # Start benchmarking
  ai-aas-cli benchmark target start llama-7b-qa

  # View benchmark status
  ai-aas-cli benchmark status`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newScenarioCommand())
	cmd.AddCommand(newTargetCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newStatusCommand())

	return cmd
}

// ============================================================================
// Scenario Commands
// ============================================================================

func newScenarioCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Manage benchmark scenarios",
		Long: `Manage benchmark scenarios (test configurations).

Scenarios define how benchmarks are run, including:
  - Request patterns and load profiles
  - Token counts and response sizes
  - Timing configurations

Examples:
  ai-aas-cli benchmark scenario list
  ai-aas-cli benchmark scenario show short-qa
  ai-aas-cli benchmark scenario sync`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(newScenarioListCommand())
	cmd.AddCommand(newScenarioShowCommand())
	cmd.AddCommand(newScenarioSyncCommand())

	return cmd
}

func newScenarioListCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available benchmark scenarios",
		Long: `List all available benchmark scenarios.

Examples:
  ai-aas-cli benchmark scenario list
  ai-aas-cli benchmark scenario list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			scenarios, err := bmClient.ListScenarios(ctx)
			if err != nil {
				return fmt.Errorf("failed to list scenarios: %w", err)
			}

			if len(scenarios) == 0 {
				fmt.Println("No scenarios found.")
				fmt.Println("\nRun 'ai-aas-cli benchmark scenario sync' to sync scenarios from config.")
				return nil
			}

			if format == "json" {
				return output.PrintJSON(scenarios, true)
			}

			// Table output
			table := output.NewTableWriter()
			table.SetHeader([]string{"NAME", "VERSION", "DESCRIPTION", "SYNCED"})

			for _, s := range scenarios {
				desc := s.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				syncedAt := s.SyncedAt.Format("2006-01-02 15:04")

				table.Append([]string{
					s.Name,
					s.Version,
					desc,
					syncedAt,
				})
			}

			table.Render()
			fmt.Printf("\nTotal: %d scenario(s)\n", len(scenarios))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func newScenarioShowCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <scenario-name>",
		Short: "Show scenario details",
		Long: `Display detailed information about a benchmark scenario.

Examples:
  ai-aas-cli benchmark scenario show short-qa
  ai-aas-cli benchmark scenario show long-generation --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scenarioName := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			scenario, err := bmClient.GetScenario(ctx, scenarioName)
			if err != nil {
				return fmt.Errorf("scenario not found: %s", scenarioName)
			}

			if format == "json" {
				return output.PrintJSON(scenario, true)
			}

			// Display scenario info
			fmt.Printf("Scenario: %s\n", scenario.Name)
			fmt.Println(strings.Repeat("-", 50))

			if scenario.Description != "" {
				fmt.Printf("\nDescription\n")
				fmt.Printf("  %s\n", scenario.Description)
			}

			fmt.Printf("\nDetails\n")
			fmt.Printf("  Version:   %s\n", scenario.Version)
			fmt.Printf("  Synced At: %s\n", scenario.SyncedAt.Format("2006-01-02 15:04:05"))

			if len(scenario.Config) > 0 {
				fmt.Printf("\nConfiguration\n")
				configJSON, _ := json.MarshalIndent(scenario.Config, "  ", "  ")
				fmt.Printf("  %s\n", string(configJSON))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func newScenarioSyncCommand() *cobra.Command {
	var (
		scenariosDir  string
		deleteOrphans bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync scenarios from a local directory",
		Long: `Sync benchmark scenarios from a local directory to the Admin API.

Reads all YAML files from the specified directory and syncs them to the
benchmark scenarios database. Each YAML file should define a single scenario.

Examples:
  # Sync scenarios from ai-aas-config
  ai-aas-cli benchmark scenario sync --dir ~/ai-aas-config/benchmark-scenarios

  # Sync and remove scenarios not in the directory
  ai-aas-cli benchmark scenario sync --dir ./scenarios --delete-orphans`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if scenariosDir == "" {
				return fmt.Errorf("--dir flag is required: specify the directory containing scenario YAML files")
			}

			// Expand ~ in path
			if strings.HasPrefix(scenariosDir, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				scenariosDir = filepath.Join(home, scenariosDir[2:])
			}

			// Check directory exists
			info, err := os.Stat(scenariosDir)
			if err != nil {
				return fmt.Errorf("scenarios directory not found: %s", scenariosDir)
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", scenariosDir)
			}

			// Load all YAML files
			files, err := filepath.Glob(filepath.Join(scenariosDir, "*.yaml"))
			if err != nil {
				return fmt.Errorf("failed to list YAML files: %w", err)
			}

			// Also check for .yml extension
			ymlFiles, _ := filepath.Glob(filepath.Join(scenariosDir, "*.yml"))
			files = append(files, ymlFiles...)

			if len(files) == 0 {
				return fmt.Errorf("no YAML files found in %s", scenariosDir)
			}

			fmt.Printf("Loading %d scenario file(s) from %s\n", len(files), scenariosDir)

			// Parse each file
			var scenarios []benchmark.ScenarioUpsert
			for _, file := range files {
				scenario, err := loadScenarioFile(file)
				if err != nil {
					return fmt.Errorf("failed to load %s: %w", filepath.Base(file), err)
				}
				scenarios = append(scenarios, *scenario)
				fmt.Printf("  Loaded: %s\n", scenario.Name)
			}

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			fmt.Println("\nSyncing scenarios to Admin API...")

			resp, err := bmClient.SyncScenarios(ctx, scenarios, deleteOrphans)
			if err != nil {
				return fmt.Errorf("failed to sync scenarios: %w", err)
			}

			fmt.Println("\nSync complete:")
			if len(resp.Created) > 0 {
				fmt.Printf("  Created: %s\n", strings.Join(resp.Created, ", "))
			}
			if len(resp.Updated) > 0 {
				fmt.Printf("  Updated: %s\n", strings.Join(resp.Updated, ", "))
			}
			if len(resp.Deleted) > 0 {
				fmt.Printf("  Deleted: %s\n", strings.Join(resp.Deleted, ", "))
			}
			if len(resp.Created) == 0 && len(resp.Updated) == 0 && len(resp.Deleted) == 0 {
				fmt.Println("  No changes")
			}

			fmt.Println("\nRun 'ai-aas-cli benchmark scenario list' to see available scenarios.")

			return nil
		},
	}

	cmd.Flags().StringVarP(&scenariosDir, "dir", "d", "", "directory containing scenario YAML files (required)")
	cmd.Flags().BoolVar(&deleteOrphans, "delete-orphans", false, "delete scenarios not present in the directory")
	cmd.MarkFlagRequired("dir")

	return cmd
}

// scenarioYAML represents the YAML structure of a scenario file
type scenarioYAML struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Version     string                 `yaml:"version"`
	Data        map[string]interface{} `yaml:"data"`
	Defaults    map[string]interface{} `yaml:"defaults"`
	Metrics     map[string]interface{} `yaml:"metrics"`
}

// loadScenarioFile loads a scenario from a YAML file
func loadScenarioFile(path string) (*benchmark.ScenarioUpsert, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var raw scenarioYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	if raw.Name == "" {
		return nil, fmt.Errorf("scenario must have a name")
	}

	// Build config from the YAML sections
	config := make(map[string]interface{})
	if raw.Data != nil {
		config["data"] = raw.Data
	}
	if raw.Defaults != nil {
		config["defaults"] = raw.Defaults
	}
	if raw.Metrics != nil {
		config["metrics"] = raw.Metrics
	}

	return &benchmark.ScenarioUpsert{
		Name:        raw.Name,
		Description: raw.Description,
		Version:     raw.Version,
		Config:      config,
	}, nil
}

// ============================================================================
// Target Commands
// ============================================================================

func newTargetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "target",
		Short: "Manage benchmark targets",
		Long: `Manage benchmark targets.

Targets link a deployed model to a benchmark scenario, defining what to test.

Examples:
  ai-aas-cli benchmark target list
  ai-aas-cli benchmark target add llama-7b-qa --model llama-7b --scenario short-qa
  ai-aas-cli benchmark target show llama-7b-qa
  ai-aas-cli benchmark target start llama-7b-qa
  ai-aas-cli benchmark target stop llama-7b-qa
  ai-aas-cli benchmark target remove llama-7b-qa`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(newTargetListCommand())
	cmd.AddCommand(newTargetAddCommand())
	cmd.AddCommand(newTargetShowCommand())
	cmd.AddCommand(newTargetStartCommand())
	cmd.AddCommand(newTargetStopCommand())
	cmd.AddCommand(newTargetRemoveCommand())

	return cmd
}

func newTargetListCommand() *cobra.Command {
	var (
		format      string
		environment string
		modelName   string
		scenario    string
		status      string
		wide        bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List benchmark targets",
		Long: `List benchmark targets with optional filtering.

Examples:
  ai-aas-cli benchmark target list
  ai-aas-cli benchmark target list -e development
  ai-aas-cli benchmark target list --model llama-7b
  ai-aas-cli benchmark target list --status active
  ai-aas-cli benchmark target list -o wide`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			opts := benchmark.ListTargetsOptions{
				Environment:  environment,
				ModelName:    modelName,
				ScenarioName: scenario,
				Status:       status,
			}

			targets, err := bmClient.ListTargets(ctx, opts)
			if err != nil {
				return fmt.Errorf("failed to list targets: %w", err)
			}

			if len(targets) == 0 {
				fmt.Println("No targets found.")
				return nil
			}

			if format == "json" {
				return output.PrintJSON(targets, true)
			}

			// Table output
			table := output.NewTableWriter()
			if wide {
				table.SetHeader([]string{"NAME", "MODEL", "SCENARIO", "ENV", "STATUS", "SCHEDULE", "LAST RUN", "LAST STATUS"})
			} else {
				table.SetHeader([]string{"NAME", "MODEL", "SCENARIO", "ENV", "STATUS", "LAST RUN"})
			}

			for _, t := range targets {
				lastRun := "-"
				if t.LastRunAt != nil {
					lastRun = t.LastRunAt.Format("2006-01-02 15:04")
				}

				lastStatus := "-"
				if t.LastRunStatus != nil {
					lastStatus = *t.LastRunStatus
				}

				scheduleEnabled := "No"
				if t.ScheduleEnabled {
					scheduleEnabled = "Yes"
				}

				if wide {
					table.Append([]string{
						t.Name,
						t.ModelName,
						t.ScenarioName,
						t.Environment,
						t.Status,
						scheduleEnabled,
						lastRun,
						lastStatus,
					})
				} else {
					table.Append([]string{
						t.Name,
						t.ModelName,
						t.ScenarioName,
						t.Environment,
						t.Status,
						lastRun,
					})
				}
			}

			table.Render()
			fmt.Printf("\nTotal: %d target(s)\n", len(targets))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().StringVar(&modelName, "model", "", "filter by model name")
	cmd.Flags().StringVar(&scenario, "scenario", "", "filter by scenario name")
	cmd.Flags().StringVar(&status, "status", "", "filter by status (active, paused, disabled)")
	cmd.Flags().BoolVarP(&wide, "wide", "o", false, "show additional columns")

	return cmd
}

func newTargetAddCommand() *cobra.Command {
	var (
		modelName       string
		scenario        string
		environment     string
		displayName     string
		endpointURL     string
		orgID           string
		intervalSeconds int
		noSchedule      bool
	)

	cmd := &cobra.Command{
		Use:   "add <target-name>",
		Short: "Add a new benchmark target",
		Long: `Add a new benchmark target linking a model to a scenario.

The target name should be unique within the environment.

Examples:
  # Create a basic target
  ai-aas-cli benchmark target add llama-7b-qa --model llama-7b --scenario short-qa -e development

  # Create with custom endpoint
  ai-aas-cli benchmark target add llama-external --model llama-7b --scenario short-qa \
    --endpoint https://external.example.com/v1

  # Create without scheduled runs
  ai-aas-cli benchmark target add llama-manual --model llama-7b --scenario short-qa --no-schedule`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := args[0]

			if modelName == "" {
				return fmt.Errorf("--model flag is required")
			}
			if scenario == "" {
				return fmt.Errorf("--scenario flag is required")
			}

			// Default environment
			if environment == "" {
				environment = "development"
			}

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			req := benchmark.CreateTargetRequest{
				Name:         targetName,
				ModelName:    modelName,
				ScenarioName: scenario,
				Environment:  environment,
			}

			if displayName != "" {
				req.DisplayName = &displayName
			}
			if endpointURL != "" {
				req.EndpointURL = &endpointURL
			}
			if orgID != "" {
				req.OrgID = &orgID
			}
			if intervalSeconds > 0 {
				req.IntervalSeconds = &intervalSeconds
			}
			if noSchedule {
				enabled := false
				req.ScheduleEnabled = &enabled
			}

			target, err := bmClient.CreateTarget(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to create target: %w", err)
			}

			fmt.Printf("Created benchmark target: %s\n", target.Name)
			fmt.Printf("  Model:       %s\n", target.ModelName)
			fmt.Printf("  Scenario:    %s\n", target.ScenarioName)
			fmt.Printf("  Environment: %s\n", target.Environment)
			fmt.Printf("  Status:      %s\n", target.Status)
			if target.ScheduleEnabled {
				fmt.Println("  Scheduled:   Yes")
			} else {
				fmt.Println("  Scheduled:   No (use 'benchmark target start' to enable)")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&modelName, "model", "", "model name (required)")
	cmd.Flags().StringVar(&scenario, "scenario", "", "scenario name (required)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "environment (default: development)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "display name for the target")
	cmd.Flags().StringVar(&endpointURL, "endpoint", "", "custom endpoint URL (overrides model's default)")
	cmd.Flags().StringVar(&orgID, "org", "", "organization ID")
	cmd.Flags().IntVar(&intervalSeconds, "interval", 0, "benchmark interval in seconds")
	cmd.Flags().BoolVar(&noSchedule, "no-schedule", false, "disable scheduled runs")

	cmd.MarkFlagRequired("model")
	cmd.MarkFlagRequired("scenario")

	return cmd
}

func newTargetShowCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <target-name-or-id>",
		Short: "Show target details",
		Long: `Display detailed information about a benchmark target.

You can specify either the target name or ID.

Examples:
  ai-aas-cli benchmark target show llama-7b-qa
  ai-aas-cli benchmark target show 550e8400-e29b-41d4-a716-446655440000
  ai-aas-cli benchmark target show llama-7b-qa --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRef := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			// Resolve target name to ID if needed
			targetID, err := bmClient.ResolveTargetID(ctx, targetRef)
			if err != nil {
				return fmt.Errorf("target not found: %s", targetRef)
			}

			target, err := bmClient.GetTarget(ctx, targetID)
			if err != nil {
				return fmt.Errorf("failed to get target: %w", err)
			}

			if format == "json" {
				return output.PrintJSON(target, true)
			}

			// Display target info
			fmt.Printf("Target: %s\n", target.Name)
			fmt.Println(strings.Repeat("-", 50))

			fmt.Printf("\nConfiguration\n")
			fmt.Printf("  ID:          %s\n", target.ID)
			fmt.Printf("  Model:       %s\n", target.ModelName)
			fmt.Printf("  Scenario:    %s\n", target.ScenarioName)
			fmt.Printf("  Environment: %s\n", target.Environment)
			if target.DisplayName != nil {
				fmt.Printf("  Display:     %s\n", *target.DisplayName)
			}
			if target.EndpointURL != nil {
				fmt.Printf("  Endpoint:    %s\n", *target.EndpointURL)
			}
			if target.OrgID != nil {
				fmt.Printf("  Org ID:      %s\n", *target.OrgID)
			}

			fmt.Printf("\nStatus\n")
			fmt.Printf("  Status:      %s\n", target.Status)
			if target.ScheduleEnabled {
				fmt.Printf("  Scheduled:   Yes\n")
				if target.IntervalSeconds != nil {
					fmt.Printf("  Interval:    %d seconds\n", *target.IntervalSeconds)
				}
			} else {
				fmt.Printf("  Scheduled:   No\n")
			}

			fmt.Printf("\nLast Run\n")
			if target.LastRunAt != nil {
				fmt.Printf("  Time:        %s\n", target.LastRunAt.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("  Time:        Never\n")
			}
			if target.LastRunStatus != nil {
				fmt.Printf("  Status:      %s\n", *target.LastRunStatus)
			}

			fmt.Printf("\nTimestamps\n")
			fmt.Printf("  Created:     %s\n", target.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Updated:     %s\n", target.UpdatedAt.Format("2006-01-02 15:04:05"))

			if len(target.ConfigOverrides) > 0 {
				fmt.Printf("\nConfig Overrides\n")
				configJSON, _ := json.MarshalIndent(target.ConfigOverrides, "  ", "  ")
				fmt.Printf("  %s\n", string(configJSON))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func newTargetStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <target-name-or-id>",
		Short: "Start benchmarking for a target",
		Long: `Enable scheduled benchmarking for a target.

This sets the target status to 'active' and enables scheduled runs.

Examples:
  ai-aas-cli benchmark target start llama-7b-qa`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRef := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			// Resolve target name to ID if needed
			targetID, err := bmClient.ResolveTargetID(ctx, targetRef)
			if err != nil {
				return fmt.Errorf("failed to resolve target: %w", err)
			}

			target, err := bmClient.StartTarget(ctx, targetID)
			if err != nil {
				return fmt.Errorf("failed to start target: %w", err)
			}

			fmt.Printf("Started benchmarking for target: %s\n", target.Name)
			fmt.Printf("  Status:    %s\n", target.Status)
			fmt.Printf("  Scheduled: Yes\n")

			return nil
		},
	}

	return cmd
}

func newTargetStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <target-name-or-id>",
		Short: "Stop benchmarking for a target",
		Long: `Disable scheduled benchmarking for a target.

This sets the target status to 'paused' and disables scheduled runs.

Examples:
  ai-aas-cli benchmark target stop llama-7b-qa`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRef := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			// Resolve target name to ID if needed
			targetID, err := bmClient.ResolveTargetID(ctx, targetRef)
			if err != nil {
				return fmt.Errorf("failed to resolve target: %w", err)
			}

			target, err := bmClient.StopTarget(ctx, targetID)
			if err != nil {
				return fmt.Errorf("failed to stop target: %w", err)
			}

			fmt.Printf("Stopped benchmarking for target: %s\n", target.Name)
			fmt.Printf("  Status:    %s\n", target.Status)
			fmt.Printf("  Scheduled: No\n")

			return nil
		},
	}

	return cmd
}

func newTargetRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <target-name-or-id>",
		Short: "Remove a benchmark target",
		Long: `Remove a benchmark target.

This will delete the target and all associated run history.

Examples:
  ai-aas-cli benchmark target remove llama-7b-qa
  ai-aas-cli benchmark target remove llama-7b-qa --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRef := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			// Resolve target name to ID if needed
			targetID, err := bmClient.ResolveTargetID(ctx, targetRef)
			if err != nil {
				return fmt.Errorf("target not found: %s", targetRef)
			}

			// Get target to show info
			target, err := bmClient.GetTarget(ctx, targetID)
			if err != nil {
				return fmt.Errorf("failed to get target: %w", err)
			}

			if !force {
				fmt.Printf("This will delete target '%s' and all associated run history.\n", target.Name)
				fmt.Printf("Use --force to confirm deletion.\n")
				return nil
			}

			if err := bmClient.DeleteTarget(ctx, target.ID); err != nil {
				return fmt.Errorf("failed to remove target: %w", err)
			}

			fmt.Printf("Removed benchmark target: %s\n", target.Name)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")

	return cmd
}

// ============================================================================
// Run Commands
// ============================================================================

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage benchmark runs",
		Long: `Manage benchmark runs and view results.

Examples:
  ai-aas-cli benchmark run list
  ai-aas-cli benchmark run list --target llama-7b-qa
  ai-aas-cli benchmark run show <run-id>
  ai-aas-cli benchmark run trigger llama-7b-qa`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(newRunListCommand())
	cmd.AddCommand(newRunShowCommand())
	cmd.AddCommand(newRunTriggerCommand())

	return cmd
}

func newRunListCommand() *cobra.Command {
	var (
		format      string
		targetID    string
		scenario    string
		status      string
		triggeredBy string
		limit       int
		wide        bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List benchmark runs",
		Long: `List benchmark runs with optional filtering.

Examples:
  ai-aas-cli benchmark run list
  ai-aas-cli benchmark run list --target llama-7b-qa
  ai-aas-cli benchmark run list --status completed
  ai-aas-cli benchmark run list --triggered-by manual
  ai-aas-cli benchmark run list -o wide`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			opts := benchmark.ListRunsOptions{
				TargetID:     targetID,
				ScenarioName: scenario,
				Status:       status,
				TriggeredBy:  triggeredBy,
				Limit:        limit,
			}

			runs, err := bmClient.ListRuns(ctx, opts)
			if err != nil {
				return fmt.Errorf("failed to list runs: %w", err)
			}

			if len(runs) == 0 {
				fmt.Println("No runs found.")
				return nil
			}

			if format == "json" {
				return output.PrintJSON(runs, true)
			}

			// Table output
			table := output.NewTableWriter()
			if wide {
				table.SetHeader([]string{"ID", "SCENARIO", "STATUS", "STARTED", "DURATION", "TRIGGERED", "RPS", "P99"})
			} else {
				table.SetHeader([]string{"ID", "SCENARIO", "STATUS", "STARTED", "DURATION", "RPS"})
			}

			for _, r := range runs {
				// Truncate ID for display
				shortID := r.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}

				startedAt := "-"
				if r.StartedAt != nil {
					startedAt = r.StartedAt.Format("2006-01-02 15:04")
				}

				duration := "-"
				if r.DurationSeconds != nil {
					duration = fmt.Sprintf("%ds", *r.DurationSeconds)
				}

				rps := "-"
				p99 := "-"
				if r.Results != nil {
					if r.Results.RequestsPerSecond > 0 {
						rps = fmt.Sprintf("%.1f", r.Results.RequestsPerSecond)
					}
					if r.Results.LatencyP99 > 0 {
						p99 = fmt.Sprintf("%.0fms", r.Results.LatencyP99)
					}
				}

				if wide {
					table.Append([]string{
						shortID,
						r.ScenarioName,
						r.Status,
						startedAt,
						duration,
						r.TriggeredBy,
						rps,
						p99,
					})
				} else {
					table.Append([]string{
						shortID,
						r.ScenarioName,
						r.Status,
						startedAt,
						duration,
						rps,
					})
				}
			}

			table.Render()
			fmt.Printf("\nTotal: %d run(s)\n", len(runs))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().StringVar(&targetID, "target", "", "filter by target name or ID")
	cmd.Flags().StringVar(&scenario, "scenario", "", "filter by scenario name")
	cmd.Flags().StringVar(&status, "status", "", "filter by status (pending, running, completed, failed)")
	cmd.Flags().StringVar(&triggeredBy, "triggered-by", "", "filter by trigger type (schedule, manual, api)")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of runs to return")
	cmd.Flags().BoolVarP(&wide, "wide", "o", false, "show additional columns")

	return cmd
}

func newRunShowCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show run details and results",
		Long: `Display detailed information about a benchmark run including results.

Examples:
  ai-aas-cli benchmark run show 550e8400-e29b-41d4-a716-446655440000
  ai-aas-cli benchmark run show 550e8400 --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			run, err := bmClient.GetRun(ctx, runID)
			if err != nil {
				return fmt.Errorf("run not found: %s", runID)
			}

			if format == "json" {
				return output.PrintJSON(run, true)
			}

			// Display run info
			fmt.Printf("Run: %s\n", run.ID)
			fmt.Println(strings.Repeat("-", 60))

			fmt.Printf("\nDetails\n")
			fmt.Printf("  Target ID:   %s\n", run.TargetID)
			fmt.Printf("  Scenario:    %s\n", run.ScenarioName)
			fmt.Printf("  Status:      %s\n", run.Status)
			fmt.Printf("  Triggered:   %s\n", run.TriggeredBy)
			if run.TriggeredByUser != nil {
				fmt.Printf("  User:        %s\n", *run.TriggeredByUser)
			}

			fmt.Printf("\nTiming\n")
			fmt.Printf("  Created:     %s\n", run.CreatedAt.Format("2006-01-02 15:04:05"))
			if run.StartedAt != nil {
				fmt.Printf("  Started:     %s\n", run.StartedAt.Format("2006-01-02 15:04:05"))
			}
			if run.CompletedAt != nil {
				fmt.Printf("  Completed:   %s\n", run.CompletedAt.Format("2006-01-02 15:04:05"))
			}
			if run.DurationSeconds != nil {
				fmt.Printf("  Duration:    %d seconds\n", *run.DurationSeconds)
			}

			if run.ErrorMessage != nil {
				fmt.Printf("\nError\n")
				fmt.Printf("  %s\n", *run.ErrorMessage)
			}

			if run.Results != nil {
				r := run.Results
				fmt.Printf("\nResults\n")

				fmt.Printf("\n  Throughput\n")
				fmt.Printf("    Requests Total:    %d\n", r.RequestsTotal)
				fmt.Printf("    Requests/sec:      %.2f\n", r.RequestsPerSecond)
				fmt.Printf("    Tokens/sec:        %.2f\n", r.TokensPerSecond)

				fmt.Printf("\n  Latency (ms)\n")
				fmt.Printf("    P50:   %.2f    P90:   %.2f    P95:   %.2f    P99:   %.2f\n",
					r.LatencyP50, r.LatencyP90, r.LatencyP95, r.LatencyP99)
				fmt.Printf("    Avg:   %.2f    Min:   %.2f    Max:   %.2f\n",
					r.LatencyAvg, r.LatencyMin, r.LatencyMax)

				if r.TTFTP50 > 0 || r.TTFTAvg > 0 {
					fmt.Printf("\n  Time to First Token (ms)\n")
					fmt.Printf("    P50:   %.2f    P90:   %.2f    P99:   %.2f    Avg:   %.2f\n",
						r.TTFTP50, r.TTFTP90, r.TTFTP99, r.TTFTAvg)
				}

				if r.ITLP50 > 0 || r.ITLAvg > 0 {
					fmt.Printf("\n  Inter-Token Latency (ms)\n")
					fmt.Printf("    P50:   %.2f    P90:   %.2f    P99:   %.2f    Avg:   %.2f\n",
						r.ITLP50, r.ITLP90, r.ITLP99, r.ITLAvg)
				}

				fmt.Printf("\n  Errors\n")
				fmt.Printf("    Count:  %d    Rate:  %.2f%%\n", r.ErrorCount, r.ErrorRate*100)

				if r.TotalInputTokens > 0 || r.TotalOutputTokens > 0 {
					fmt.Printf("\n  Tokens\n")
					fmt.Printf("    Input:  %d    Output:  %d\n", r.TotalInputTokens, r.TotalOutputTokens)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func newRunTriggerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <target-name-or-id>",
		Short: "Trigger a manual benchmark run",
		Long: `Trigger a manual benchmark run for a target.

This starts an immediate benchmark run, regardless of the schedule.

Examples:
  ai-aas-cli benchmark run trigger llama-7b-qa`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRef := args[0]

			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			fmt.Printf("Triggering benchmark run for target: %s\n", targetRef)

			run, err := bmClient.TriggerRun(ctx, targetRef)
			if err != nil {
				return fmt.Errorf("failed to trigger run: %w", err)
			}

			fmt.Printf("Run created: %s\n", run.ID)
			fmt.Printf("  Status:   %s\n", run.Status)
			fmt.Printf("  Scenario: %s\n", run.ScenarioName)
			fmt.Println()
			fmt.Printf("Use 'ai-aas-cli benchmark run show %s' to view results.\n", run.ID)

			return nil
		},
	}

	return cmd
}

// ============================================================================
// Status Command
// ============================================================================

func newStatusCommand() *cobra.Command {
	var (
		format      string
		environment string
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show overall benchmark status",
		Long: `Display an overview of benchmark status across the platform.

Shows:
  - Total and active targets
  - Recent runs and their status
  - Per-environment breakdown

Examples:
  ai-aas-cli benchmark status
  ai-aas-cli benchmark status -e development`,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			bmClient := benchmark.NewClient(apiClient)

			// Get targets
			opts := benchmark.ListTargetsOptions{
				Environment: environment,
			}
			targets, err := bmClient.ListTargets(ctx, opts)
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}

			// Get recent runs
			runOpts := benchmark.ListRunsOptions{
				Limit: 5,
			}
			runs, err := bmClient.ListRuns(ctx, runOpts)
			if err != nil {
				// Non-fatal - continue with target info
				runs = nil
			}

			if format == "json" {
				data := map[string]interface{}{
					"targets":     targets,
					"recent_runs": runs,
				}
				return output.PrintJSON(data, true)
			}

			// Calculate stats
			activeCount := 0
			pausedCount := 0
			envCount := make(map[string]int)
			for _, t := range targets {
				if t.Status == "active" {
					activeCount++
				} else if t.Status == "paused" {
					pausedCount++
				}
				envCount[t.Environment]++
			}

			// Display status
			fmt.Println("Benchmark Status")
			fmt.Println(strings.Repeat("=", 60))

			fmt.Printf("\nTargets\n")
			fmt.Printf("  Total:   %d\n", len(targets))
			fmt.Printf("  Active:  %d\n", activeCount)
			fmt.Printf("  Paused:  %d\n", pausedCount)

			if len(envCount) > 0 {
				fmt.Printf("\nBy Environment\n")
				for env, count := range envCount {
					fmt.Printf("  %s: %d\n", env, count)
				}
			}

			if len(runs) > 0 {
				fmt.Printf("\nRecent Runs\n")
				table := output.NewTableWriter()
				table.SetHeader([]string{"ID", "SCENARIO", "STATUS", "STARTED"})

				for _, r := range runs {
					shortID := r.ID
					if len(shortID) > 8 {
						shortID = shortID[:8]
					}

					startedAt := "-"
					if r.StartedAt != nil {
						startedAt = r.StartedAt.Format("2006-01-02 15:04")
					}

					table.Append([]string{
						shortID,
						r.ScenarioName,
						r.Status,
						startedAt,
					})
				}

				table.Render()
			}

			fmt.Println()
			fmt.Println("Use 'ai-aas-cli benchmark target list' for target details.")
			fmt.Println("Use 'ai-aas-cli benchmark run list' for run details.")

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")

	return cmd
}
