// Package commands provides deployment status inspection commands.
//
// Purpose:
//
//	Deployment status inspection commands: status aggregation from multiple sources
//	(Helm, Kubernetes, database registry) with health check monitoring.
//
// Requirements Reference:
//   - specs/010-vllm-deployment/spec.md#US-003 (Safe operations)
//   - specs/010-vllm-deployment/tasks.md#T-S010-P05-052 (Status inspection)
//
package commands

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

// DeploymentCommand creates the deployment command group.
func DeploymentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment",
		Short: "Manage and inspect vLLM model deployments",
		Long:  "Manage and inspect vLLM model deployments: status inspection, health monitoring",
	}

	cmd.AddCommand(deploymentStatusCommand())

	return cmd
}

func deploymentStatusCommand() *cobra.Command {
	var flagModelName string
	var flagEnvironment string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Inspect deployment status across multiple sources",
		Long: `Inspect deployment status by aggregating information from:
- Model registry database (deployment metadata, health checks)
- Kubernetes (pod status, readiness) - if kubectl configured
- Helm (release status, revisions) - if helm configured

This provides a comprehensive view of deployment health and status.`,
		Example: `  # Check status of a specific model
  admin-cli deployment status --model-name llama-2-7b --environment development

  # Check all deployments in production
  admin-cli deployment status --environment production

  # JSON output for automation
  admin-cli deployment status --model-name llama-2-7b --environment development --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploymentStatus(cmd, args, flagModelName, flagEnvironment, flagFormat, flagVerbose, flagQuiet)
		},
	}

	cmd.Flags().StringVar(&flagModelName, "model-name", "", "Model name to inspect (optional - shows all if not specified)")
	cmd.Flags().StringVar(&flagEnvironment, "environment", "development", "Deployment environment")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")

	return cmd
}

func runDeploymentStatus(cmd *cobra.Command, args []string, modelName, environment, flagFormat string, verbose, quiet bool) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Inspecting deployment status...\n")
		if modelName != "" {
			fmt.Fprintf(os.Stderr, "  Model: %s\n", modelName)
		} else {
			fmt.Fprintf(os.Stderr, "  Model: all models\n")
		}
		fmt.Fprintf(os.Stderr, "  Environment: %s\n", environment)
		fmt.Fprintf(os.Stderr, "\n")
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

	// Query deployment status from registry
	statuses, err := queryDeploymentStatuses(ctx, db, modelName, environment)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to query deployment status: %v", err),
			"Check database connectivity and schema.",
		)
	}

	if len(statuses) == 0 {
		if !quiet {
			if modelName != "" {
				fmt.Fprintf(os.Stderr, "No deployment found for model %s in %s environment\n", modelName, environment)
			} else {
				fmt.Fprintf(os.Stderr, "No deployments found in %s environment\n", environment)
			}
		}
		return nil
	}

	// Output based on format
	if flagFormat == "json" {
		return output.PrintJSON(statuses)
	}

	// Table format
	headers := []string{"Model Name", "Status", "Endpoint", "Namespace", "Last Health", "Updated"}
	var tableRows [][]string
	for _, status := range statuses {
		lastHealth := "never"
		if status["last_health"].(string) != "" {
			lastHealth = status["last_health"].(string)
		}

		tableRows = append(tableRows, []string{
			status["model_name"].(string),
			status["status"].(string),
			status["endpoint"].(string),
			status["namespace"].(string),
			lastHealth,
			status["updated_at"].(time.Time).Format("2006-01-02 15:04"),
		})
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "Found %d deployment(s)\n\n", len(statuses))
	}

	return output.PrintTable(headers, tableRows)
}

// queryDeploymentStatuses queries the database for deployment statuses.
func queryDeploymentStatuses(ctx context.Context, db *sql.DB, modelName, environment string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			model_name,
			deployment_endpoint,
			deployment_status,
			deployment_environment,
			deployment_namespace,
			last_health_check_at,
			updated_at
		FROM model_registry_entries
		WHERE deployment_environment = $1
		  AND deployment_endpoint IS NOT NULL
	`

	args := []interface{}{environment}

	if modelName != "" {
		query += " AND model_name = $2"
		args = append(args, modelName)
	}

	query += " ORDER BY model_name"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query deployment status: %w", err)
	}
	defer rows.Close()

	var statuses []map[string]interface{}
	for rows.Next() {
		var (
			modelName   string
			endpoint    string
			status      string
			env         string
			namespace   string
			lastHealth  sql.NullTime
			updatedAt   time.Time
		)

		err := rows.Scan(
			&modelName,
			&endpoint,
			&status,
			&env,
			&namespace,
			&lastHealth,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		healthStr := ""
		if lastHealth.Valid {
			healthStr = lastHealth.Time.Format(time.RFC3339)
		}

		statuses = append(statuses, map[string]interface{}{
			"model_name":  modelName,
			"endpoint":    endpoint,
			"status":      status,
			"environment": env,
			"namespace":   namespace,
			"last_health": healthStr,
			"updated_at":  updatedAt,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return statuses, nil
}
