// Package admin provides deployment status inspection commands.
//
// Purpose:
//
//	Deployment status inspection commands: status aggregation from multiple sources
//	(Admin API, Kubernetes, Helm) with health check monitoring.
//
// Requirements Reference:
//   - specs/010-vllm-deployment/spec.md#US-003 (Safe operations)
//   - specs/010-vllm-deployment/tasks.md#T-S010-P05-052 (Status inspection)
package admin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/deploymentregistry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
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
- Admin API model registry (deployment metadata, health checks)
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

	// Create API client and query via Admin API
	client := newRegistryClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query deployment status from registry API
	statuses, err := queryDeploymentStatusesFromAPI(ctx, client, modelName, environment)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to query deployment status: %v", err),
			"Check API connectivity and permissions.",
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

// queryDeploymentStatusesFromAPI queries the Admin API for deployment statuses.
func queryDeploymentStatusesFromAPI(ctx context.Context, client *deploymentregistry.Client, modelName, environment string) ([]map[string]interface{}, error) {
	// If a specific model is requested, use Get
	if modelName != "" {
		model, err := client.Get(ctx, modelName, environment)
		if err != nil {
			// Check if it's a not found error - return empty list
			if isNotFoundError(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("get model: %w", err)
		}

		return []map[string]interface{}{modelToStatusMap(model)}, nil
	}

	// List all models in the environment
	resp, err := client.List(ctx, deploymentregistry.ListParams{
		Environment: environment,
		Limit:       1000, // High limit to get all results
	})
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	statuses := make([]map[string]interface{}, 0, len(resp.Models))
	for _, model := range resp.Models {
		statuses = append(statuses, modelToStatusMap(&model))
	}

	return statuses, nil
}

// modelToStatusMap converts a Model to a status map for output.
func modelToStatusMap(model *deploymentregistry.Model) map[string]interface{} {
	healthStr := ""
	if model.LastHealthCheckAt != nil {
		healthStr = model.LastHealthCheckAt.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"model_name":  model.ModelName,
		"endpoint":    model.DeploymentEndpoint,
		"status":      model.DeploymentStatus,
		"environment": model.DeploymentEnvironment,
		"namespace":   model.DeploymentNamespace,
		"last_health": healthStr,
		"updated_at":  model.UpdatedAt,
	}
}

// isNotFoundError checks if an error indicates a not found condition.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "404")
}
