// Package commands provides model registry management commands.
//
// Purpose:
//
//	Model registry lifecycle commands: register, deregister, enable, disable, list
//	with structured output and validation.
//
// Requirements Reference:
//   - specs/010-vllm-deployment/spec.md#US-002 (Register models for routing)
//   - specs/010-vllm-deployment/tasks.md#T-S010-P04-032 (Model registration command)
//
package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
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

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to database: %v", err),
			"Check your database configuration and connectivity.",
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Upsert model registry entry
	query := `
		INSERT INTO model_registry_entries (
			model_name,
			deployment_endpoint,
			deployment_status,
			deployment_environment,
			deployment_namespace,
			created_at,
			updated_at
		) VALUES ($1, $2, 'ready', $3, $4, NOW(), NOW())
		ON CONFLICT (model_name, deployment_environment)
		DO UPDATE SET
			deployment_endpoint = EXCLUDED.deployment_endpoint,
			deployment_status = 'ready',
			deployment_namespace = EXCLUDED.deployment_namespace,
			updated_at = NOW()
		RETURNING id, model_name, deployment_endpoint, deployment_status, deployment_environment, deployment_namespace, created_at, updated_at
	`

	var entry struct {
		ID                    int64
		ModelName             string
		DeploymentEndpoint    string
		DeploymentStatus      string
		DeploymentEnvironment string
		DeploymentNamespace   string
		CreatedAt             time.Time
		UpdatedAt             time.Time
	}

	err = db.QueryRowContext(ctx, query, modelName, endpoint, environment, namespace).Scan(
		&entry.ID,
		&entry.ModelName,
		&entry.DeploymentEndpoint,
		&entry.DeploymentStatus,
		&entry.DeploymentEnvironment,
		&entry.DeploymentNamespace,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to register model: %v", err),
			"Check database permissions and schema migrations.",
		)
	}

	duration := time.Since(startTime)

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Model registered successfully in %.2fs\n", duration.Seconds())
		fmt.Fprintf(os.Stderr, "  ID: %d\n", entry.ID)
		fmt.Fprintf(os.Stderr, "  Status: %s\n", entry.DeploymentStatus)
		fmt.Fprintf(os.Stderr, "  Updated: %s\n", entry.UpdatedAt.Format(time.RFC3339))
	}

	// Output structured data
	if flagFormat == "json" {
		data := map[string]interface{}{
			"id":          entry.ID,
			"model_name":  entry.ModelName,
			"endpoint":    entry.DeploymentEndpoint,
			"status":      entry.DeploymentStatus,
			"environment": entry.DeploymentEnvironment,
			"namespace":   entry.DeploymentNamespace,
			"created_at":  entry.CreatedAt,
			"updated_at":  entry.UpdatedAt,
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

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to database: %v", err),
			"Check your database configuration and connectivity.",
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update deployment status to disabled
	query := `
		UPDATE model_registry_entries
		SET deployment_status = 'disabled', updated_at = NOW()
		WHERE model_name = $1 AND deployment_environment = $2
		RETURNING id, model_name, deployment_status, deployment_environment
	`

	var entry struct {
		ID                    int64
		ModelName             string
		DeploymentStatus      string
		DeploymentEnvironment string
	}

	err = db.QueryRowContext(ctx, query, modelName, environment).Scan(
		&entry.ID,
		&entry.ModelName,
		&entry.DeploymentStatus,
		&entry.DeploymentEnvironment,
	)
	if err == sql.ErrNoRows {
		return errors.NewOperationError(
			fmt.Sprintf("model not found: %s in %s environment", modelName, environment),
			"Check that the model name and environment are correct.",
		)
	} else if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to deregister model: %v", err),
			"Check database permissions and connectivity.",
		)
	}

	duration := time.Since(startTime)

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Model deregistered successfully in %.2fs\n", duration.Seconds())
		fmt.Fprintf(os.Stderr, "  ID: %d\n", entry.ID)
		fmt.Fprintf(os.Stderr, "  Status: %s\n", entry.DeploymentStatus)
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
	return updateModelStatus(modelName, environment, "ready", quiet, dryRun, "enabled")
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
	return updateModelStatus(modelName, environment, "disabled", quiet, dryRun, "disabled")
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

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to database: %v", err),
			"Check your database configuration and connectivity.",
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update status
	query := `
		UPDATE model_registry_entries
		SET deployment_status = $3, updated_at = NOW()
		WHERE model_name = $1 AND deployment_environment = $2
		RETURNING id, model_name, deployment_status
	`

	var entry struct {
		ID               int64
		ModelName        string
		DeploymentStatus string
	}

	err = db.QueryRowContext(ctx, query, modelName, environment, status).Scan(
		&entry.ID,
		&entry.ModelName,
		&entry.DeploymentStatus,
	)
	if err == sql.ErrNoRows {
		return errors.NewOperationError(
			fmt.Sprintf("model not found: %s in %s environment", modelName, environment),
			"Check that the model name and environment are correct.",
		)
	} else if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to update model status: %v", err),
			"Check database permissions and connectivity.",
		)
	}

	duration := time.Since(startTime)

	if !quiet {
		fmt.Fprintf(os.Stderr, "✓ Model %s successfully in %.2fs\n", action, duration.Seconds())
		fmt.Fprintf(os.Stderr, "  ID: %d\n", entry.ID)
		fmt.Fprintf(os.Stderr, "  Status: %s\n", entry.DeploymentStatus)
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

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to connect to database: %v", err),
			"Check your database configuration and connectivity.",
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query with filters
	query := `
		SELECT
			id,
			model_name,
			deployment_endpoint,
			deployment_status,
			deployment_environment,
			deployment_namespace,
			last_health_check_at,
			created_at,
			updated_at
		FROM model_registry_entries
		WHERE deployment_endpoint IS NOT NULL
	`
	queryArgs := []interface{}{}
	argCount := 1

	if environment != "" {
		query += fmt.Sprintf(" AND deployment_environment = $%d", argCount)
		queryArgs = append(queryArgs, environment)
		argCount++
	}

	if status != "" {
		query += fmt.Sprintf(" AND deployment_status = $%d", argCount)
		queryArgs = append(queryArgs, status)
		argCount++
	}

	query += " ORDER BY deployment_environment, model_name"

	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to query registry: %v", err),
			"Check database connectivity and permissions.",
		)
	}
	defer rows.Close()

	entries := []map[string]interface{}{}
	for rows.Next() {
		var entry struct {
			ID                    int64
			ModelName             string
			DeploymentEndpoint    sql.NullString
			DeploymentStatus      sql.NullString
			DeploymentEnvironment sql.NullString
			DeploymentNamespace   sql.NullString
			LastHealthCheckAt     sql.NullTime
			CreatedAt             time.Time
			UpdatedAt             time.Time
		}

		err := rows.Scan(
			&entry.ID,
			&entry.ModelName,
			&entry.DeploymentEndpoint,
			&entry.DeploymentStatus,
			&entry.DeploymentEnvironment,
			&entry.DeploymentNamespace,
			&entry.LastHealthCheckAt,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to scan row: %v", err),
				"Check database schema and migrations.",
			)
		}

		healthCheck := ""
		if entry.LastHealthCheckAt.Valid {
			healthCheck = entry.LastHealthCheckAt.Time.Format(time.RFC3339)
		}

		entries = append(entries, map[string]interface{}{
			"id":          entry.ID,
			"model_name":  entry.ModelName,
			"endpoint":    entry.DeploymentEndpoint.String,
			"status":      entry.DeploymentStatus.String,
			"environment": entry.DeploymentEnvironment.String,
			"namespace":   entry.DeploymentNamespace.String,
			"last_health": healthCheck,
			"created_at":  entry.CreatedAt,
			"updated_at":  entry.UpdatedAt,
		})
	}

	if err = rows.Err(); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("error iterating rows: %v", err),
			"Check database connectivity.",
		)
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
			fmt.Sprintf("%d", entry["id"]),
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
