// Package admin provides model registry management commands.
//
// Purpose:
//
//	Model registry lifecycle commands: register, deregister, enable, disable, list
//	with structured output and validation.
//
// Requirements Reference:
//   - specs/010-vllm-deployment/spec.md#US-002 (Register models for routing)
//   - specs/010-vllm-deployment/tasks.md#T-S010-P04-032 (Model registration command)
package admin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/deploymentregistry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// RegistryCommand creates the registry command group.
func RegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage model registry entries for deployment routing",
		Long:  "Manage model registry entries: register, deregister, enable, disable, list deployed models",
	}

	cmd.AddCommand(registryRegisterCommand())
	cmd.AddCommand(registryDeregisterCommand())
	cmd.AddCommand(registryEnableCommand())
	cmd.AddCommand(registryDisableCommand())
	cmd.AddCommand(registryListCommand())

	return cmd
}

// newRegistryClient creates a deployment registry client from config.
func newRegistryClient(cfg *config.Config) *deploymentregistry.Client {
	endpoint := cfg.AdminAPIEndpoint
	if endpoint == "" {
		endpoint = cfg.APIEndpoint
	}

	opts := []api.ClientOption{}
	if cfg.TLSInsecure {
		opts = append(opts, api.WithInsecureSkipVerify())
	}
	if cfg.Timeout > 0 {
		opts = append(opts, api.WithTimeout(time.Duration(cfg.Timeout)*time.Second))
	}

	apiClient := api.NewClient(endpoint, cfg.APIKey, opts...)
	return deploymentregistry.NewClient(apiClient)
}

func registryRegisterCommand() *cobra.Command {
	var flagModelName string
	var flagEndpoint string
	var flagEnvironment string
	var flagNamespace string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagDryRun bool

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a model deployment for API routing",
		Long: `Register a model deployment in the registry with its endpoint, environment, and namespace.
This makes the model available for API routing through the API Router Service.`,
		Example: `  # Register a model in development
  admin-cli registry register \
    --model-name llama-2-7b \
    --endpoint llama-2-7b-development.system.svc.cluster.local:8000 \
    --environment development \
    --namespace system

  # Register a model in production with dry-run
  admin-cli registry register \
    --model-name llama-2-7b \
    --endpoint llama-2-7b-production.system.svc.cluster.local:8000 \
    --environment production \
    --namespace system \
    --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegistryRegister(cmd, args, flagModelName, flagEndpoint, flagEnvironment, flagNamespace, flagFormat, flagVerbose, flagQuiet, flagDryRun)
		},
	}

	cmd.Flags().StringVar(&flagModelName, "model-name", "", "Model name (required)")
	cmd.Flags().StringVar(&flagEndpoint, "endpoint", "", "Deployment endpoint URL (required)")
	cmd.Flags().StringVar(&flagEnvironment, "environment", "development", "Deployment environment: development, staging, production")
	cmd.Flags().StringVar(&flagNamespace, "namespace", "system", "Kubernetes namespace")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Simulate registration without applying changes")

	cmd.MarkFlagRequired("model-name")
	cmd.MarkFlagRequired("endpoint")

	return cmd
}

func runRegistryRegister(cmd *cobra.Command, args []string, modelName, endpoint, environment, namespace, flagFormat string, verbose, quiet, dryRun bool) error {
	startTime := time.Now()

	// Validate environment
	validEnvironments := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvironments[environment] {
		return errors.NewValidationError(
			fmt.Sprintf("invalid environment: %s", environment),
			"Environment must be one of: development, staging, production",
		)
	}

	// Validate endpoint format (should contain : for port)
	if !strings.Contains(endpoint, ":") {
		return errors.NewValidationError(
			fmt.Sprintf("invalid endpoint format: %s", endpoint),
			"Endpoint must include port (e.g., service.namespace.svc.cluster.local:8000)",
		)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Registering model deployment...\n")
		fmt.Fprintf(os.Stderr, "  Model Name: %s\n", modelName)
		fmt.Fprintf(os.Stderr, "  Endpoint: %s\n", endpoint)
		fmt.Fprintf(os.Stderr, "  Environment: %s\n", environment)
		fmt.Fprintf(os.Stderr, "  Namespace: %s\n", namespace)
		if dryRun {
			fmt.Fprintf(os.Stderr, "  Mode: DRY RUN (no changes will be made)\n")
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if dryRun {
		if !quiet {
			fmt.Fprintf(os.Stderr, "✓ Dry run successful - no changes made\n")
		}
		return nil
	}

	// Create API client and register via Admin API
	client := newRegistryClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	model, err := client.Register(ctx, deploymentregistry.RegisterRequest{
		ModelName:             modelName,
		DeploymentEndpoint:    endpoint,
		DeploymentEnvironment: environment,
		DeploymentNamespace:   namespace,
		DeploymentStatus:      deploymentregistry.StatusReady,
	})
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to register model: %v", err),
			"Check API connectivity and permissions.",
		)
	}

	duration := time.Since(startTime)

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Model registered successfully in %.2fs\n", duration.Seconds())
		fmt.Fprintf(os.Stderr, "  ID: %s\n", model.ModelID)
		fmt.Fprintf(os.Stderr, "  Status: %s\n", model.DeploymentStatus)
		fmt.Fprintf(os.Stderr, "  Updated: %s\n", model.UpdatedAt.Format(time.RFC3339))
	}

	// Output structured data
	if flagFormat == "json" {
		data := map[string]interface{}{
			"id":          model.ModelID,
			"model_name":  model.ModelName,
			"endpoint":    model.DeploymentEndpoint,
			"status":      model.DeploymentStatus,
			"environment": model.DeploymentEnvironment,
			"namespace":   model.DeploymentNamespace,
			"created_at":  model.CreatedAt,
			"updated_at":  model.UpdatedAt,
		}
		return output.PrintJSON(data)
	}

	return nil
}

func registryDeregisterCommand() *cobra.Command {
	var flagModelName string
	var flagEnvironment string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagDryRun bool

	cmd := &cobra.Command{
		Use:   "deregister",
		Short: "Deregister a model deployment from API routing",
		Long:  `Deregister a model deployment by setting its status to 'disabled'. The model will no longer be available for API routing.`,
		Example: `  # Deregister a model in development
  admin-cli registry deregister --model-name llama-2-7b --environment development

  # Deregister with dry-run
  admin-cli registry deregister --model-name llama-2-7b --environment production --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegistryDeregister(cmd, args, flagModelName, flagEnvironment, flagFormat, flagVerbose, flagQuiet, flagDryRun)
		},
	}

	cmd.Flags().StringVar(&flagModelName, "model-name", "", "Model name (required)")
	cmd.Flags().StringVar(&flagEnvironment, "environment", "development", "Deployment environment: development, staging, production")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Simulate deregistration without applying changes")

	cmd.MarkFlagRequired("model-name")

	return cmd
}

func runRegistryDeregister(cmd *cobra.Command, args []string, modelName, environment, flagFormat string, verbose, quiet, dryRun bool) error {
	startTime := time.Now()

	// Validate environment
	validEnvironments := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvironments[environment] {
		return errors.NewValidationError(
			fmt.Sprintf("invalid environment: %s", environment),
			"Environment must be one of: development, staging, production",
		)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Deregistering model deployment...\n")
		fmt.Fprintf(os.Stderr, "  Model Name: %s\n", modelName)
		fmt.Fprintf(os.Stderr, "  Environment: %s\n", environment)
		if dryRun {
			fmt.Fprintf(os.Stderr, "  Mode: DRY RUN (no changes will be made)\n")
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if dryRun {
		if !quiet {
			fmt.Fprintf(os.Stderr, "✓ Dry run successful - no changes made\n")
		}
		return nil
	}

	// Create API client and update status via Admin API
	client := newRegistryClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	model, err := client.UpdateStatus(ctx, modelName, environment, deploymentregistry.StatusDisabled)
	if err != nil {
		// Check if it's a not found error
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			return errors.NewOperationError(
				fmt.Sprintf("model not found: %s in %s environment", modelName, environment),
				"Check that the model name and environment are correct.",
			)
		}
		return errors.NewOperationError(
			fmt.Sprintf("failed to deregister model: %v", err),
			"Check API connectivity and permissions.",
		)
	}

	duration := time.Since(startTime)

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Model deregistered successfully in %.2fs\n", duration.Seconds())
		fmt.Fprintf(os.Stderr, "  ID: %s\n", model.ModelID)
		fmt.Fprintf(os.Stderr, "  Status: %s\n", model.DeploymentStatus)
	}

	return nil
}

func registryEnableCommand() *cobra.Command {
	var flagModelName string
	var flagEnvironment string
	var flagQuiet bool
	var flagDryRun bool

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable a model deployment for API routing",
		Long:  `Enable a previously disabled model deployment by setting its status to 'ready'.`,
		Example: `  # Enable a model
  admin-cli registry enable --model-name llama-2-7b --environment development`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegistryEnable(cmd, args, flagModelName, flagEnvironment, flagQuiet, flagDryRun)
		},
	}

	cmd.Flags().StringVar(&flagModelName, "model-name", "", "Model name (required)")
	cmd.Flags().StringVar(&flagEnvironment, "environment", "development", "Deployment environment")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Simulate enable without applying changes")

	cmd.MarkFlagRequired("model-name")

	return cmd
}

func runRegistryEnable(cmd *cobra.Command, args []string, modelName, environment string, quiet, dryRun bool) error {
	return updateModelStatus(modelName, environment, deploymentregistry.StatusReady, quiet, dryRun, "enabled")
}

func registryDisableCommand() *cobra.Command {
	var flagModelName string
	var flagEnvironment string
	var flagQuiet bool
	var flagDryRun bool

	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a model deployment from API routing",
		Long:  `Disable a model deployment by setting its status to 'disabled'. The model will remain registered but unavailable for routing.`,
		Example: `  # Disable a model
  admin-cli registry disable --model-name llama-2-7b --environment development`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegistryDisable(cmd, args, flagModelName, flagEnvironment, flagQuiet, flagDryRun)
		},
	}

	cmd.Flags().StringVar(&flagModelName, "model-name", "", "Model name (required)")
	cmd.Flags().StringVar(&flagEnvironment, "environment", "development", "Deployment environment")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Simulate disable without applying changes")

	cmd.MarkFlagRequired("model-name")

	return cmd
}

func runRegistryDisable(cmd *cobra.Command, args []string, modelName, environment string, quiet, dryRun bool) error {
	return updateModelStatus(modelName, environment, deploymentregistry.StatusDisabled, quiet, dryRun, "disabled")
}

func updateModelStatus(modelName, environment, status string, quiet, dryRun bool, action string) error {
	startTime := time.Now()

	// Validate environment
	validEnvironments := map[string]bool{"development": true, "staging": true, "production": true}
	if !validEnvironments[environment] {
		return errors.NewValidationError(
			fmt.Sprintf("invalid environment: %s", environment),
			"Environment must be one of: development, staging, production",
		)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Updating model status to %s...\n", status)
		fmt.Fprintf(os.Stderr, "  Model Name: %s\n", modelName)
		fmt.Fprintf(os.Stderr, "  Environment: %s\n", environment)
		if dryRun {
			fmt.Fprintf(os.Stderr, "  Mode: DRY RUN (no changes will be made)\n")
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if dryRun {
		if !quiet {
			fmt.Fprintf(os.Stderr, "✓ Dry run successful - no changes made\n")
		}
		return nil
	}

	// Create API client and update status via Admin API
	client := newRegistryClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	model, err := client.UpdateStatus(ctx, modelName, environment, status)
	if err != nil {
		// Check if it's a not found error
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			return errors.NewOperationError(
				fmt.Sprintf("model not found: %s in %s environment", modelName, environment),
				"Check that the model name and environment are correct.",
			)
		}
		return errors.NewOperationError(
			fmt.Sprintf("failed to update model status: %v", err),
			"Check API connectivity and permissions.",
		)
	}

	duration := time.Since(startTime)

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Model %s successfully in %.2fs\n", action, duration.Seconds())
		fmt.Fprintf(os.Stderr, "  ID: %s\n", model.ModelID)
		fmt.Fprintf(os.Stderr, "  Status: %s\n", model.DeploymentStatus)
	}

	return nil
}

func registryListCommand() *cobra.Command {
	var flagEnvironment string
	var flagStatus string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered model deployments",
		Long:  `List all registered model deployments with their status, endpoint, and environment.`,
		Example: `  # List all models
  admin-cli registry list

  # List production models
  admin-cli registry list --environment production

  # List only ready models
  admin-cli registry list --status ready

  # List in JSON format
  admin-cli registry list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegistryList(cmd, args, flagEnvironment, flagStatus, flagFormat, flagVerbose, flagQuiet)
		},
	}

	cmd.Flags().StringVar(&flagEnvironment, "environment", "", "Filter by environment: development, staging, production")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Filter by status: ready, disabled, deploying, failed")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")

	return cmd
}

func runRegistryList(cmd *cobra.Command, args []string, environment, status, flagFormat string, verbose, quiet bool) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Create API client and list via Admin API
	client := newRegistryClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.List(ctx, deploymentregistry.ListParams{
		Environment: environment,
		Status:      status,
		Limit:       1000, // High limit to get all results
	})
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list registry: %v", err),
			"Check API connectivity and permissions.",
		)
	}

	entries := make([]map[string]interface{}, 0, len(resp.Models))
	for _, model := range resp.Models {
		healthCheck := ""
		if model.LastHealthCheckAt != nil {
			healthCheck = model.LastHealthCheckAt.Format(time.RFC3339)
		}

		entries = append(entries, map[string]interface{}{
			"id":          model.ModelID,
			"model_name":  model.ModelName,
			"endpoint":    model.DeploymentEndpoint,
			"status":      model.DeploymentStatus,
			"environment": model.DeploymentEnvironment,
			"namespace":   model.DeploymentNamespace,
			"last_health": healthCheck,
			"created_at":  model.CreatedAt,
			"updated_at":  model.UpdatedAt,
		})
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Found %d model deployment(s)\n\n", len(entries))
	}

	// Output based on format
	if flagFormat == "json" {
		return output.PrintJSON(entries)
	}

	// Table format (default)
	if len(entries) == 0 {
		if !quiet {
			fmt.Println("No model deployments found.")
		}
		return nil
	}

	headers := []string{"ID", "Model Name", "Endpoint", "Status", "Environment", "Namespace", "Last Health", "Updated"}
	var tableRows [][]string
	for _, entry := range entries {
		tableRows = append(tableRows, []string{
			fmt.Sprintf("%s", entry["id"]),
			fmt.Sprintf("%s", entry["model_name"]),
			fmt.Sprintf("%s", entry["endpoint"]),
			fmt.Sprintf("%s", entry["status"]),
			fmt.Sprintf("%s", entry["environment"]),
			fmt.Sprintf("%s", entry["namespace"]),
			fmt.Sprintf("%s", entry["last_health"]),
			fmt.Sprintf("%s", entry["updated_at"].(time.Time).Format("2006-01-02 15:04")),
		})
	}

	return output.PrintTable(headers, tableRows)
}
