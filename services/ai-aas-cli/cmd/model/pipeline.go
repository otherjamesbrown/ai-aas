// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/admin"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewPipelineCommand creates the model pipeline diagnostic command
func NewPipelineCommand() *cobra.Command {
	var (
		environment string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "pipeline <model-name>",
		Short: "Show full deployment pipeline status for a model",
		Long: `Show the complete pipeline status for a model across all stages:
1. GitOps (ai-aas-config AIModel CR)
2. Kubernetes (ai-model-operator InferenceService)
3. Admin API (model registry, deployments, backends)
4. Routing Policy (global and org-specific policies)
5. Inference Endpoint (health check)

This command helps diagnose where in the pipeline a problem exists when
inference fails or a model is not accessible.

Examples:
  # Show pipeline for a model in development
  ai-aas-cli model pipeline gpt-oss-20b -e development

  # Show pipeline in JSON format (for scripting)
  ai-aas-cli model pipeline llama-7b -e production --format json

  # Check pipeline status in staging
  ai-aas-cli model pipeline my-model -e staging`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Get profile flag and load config
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Validate configuration
			adminEndpoint := cfg.GetAdminEndpoint()
			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			if environment == "" {
				return fmt.Errorf("environment is required. Use -e/--environment flag")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Create pipeline client
			apiClient := cfg.NewAPIClient(adminEndpoint)
			pipelineClient := admin.NewPipelineClient(apiClient)

			// Fetch pipeline status
			status, err := pipelineClient.GetPipelineStatus(ctx, modelName, environment)
			if err != nil {
				return fmt.Errorf("failed to get pipeline status: %w", err)
			}

			// Output based on format
			if format == "json" {
				return output.PrintJSON(status, true)
			}

			// Display formatted output
			displayPipelineStatus(status)
			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "environment (development, staging, production)")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.MarkFlagRequired("environment")

	return cmd
}

// displayPipelineStatus displays the pipeline status in a formatted table view
func displayPipelineStatus(status *admin.PipelineStatus) {
	// Header
	fmt.Println()
	output.Header(fmt.Sprintf("Model Pipeline: %s (%s)", status.ModelName, status.Environment))
	fmt.Println()

	// Overall summary at the top
	overallIcon := "✓"
	overallColor := output.Success
	if !status.IsFullyOperational {
		overallIcon = "❌"
		overallColor = output.Error
	}
	fmt.Printf("Overall Status: %s %s\n", overallColor.Sprint(overallIcon), overallColor.Sprint(status.OverallStatus))
	fmt.Println()

	// Stage 1: GitOps
	fmt.Println(output.Bold.Sprint("1. GitOps (ai-aas-config)"))
	displayStageStatus(status.GitOps)
	fmt.Println()

	// Stage 2: Kubernetes
	fmt.Println(output.Bold.Sprint("2. Kubernetes (ai-model-operator)"))
	displayStageStatus(status.Kubernetes)
	fmt.Println()

	// Stage 3: Admin API
	fmt.Println(output.Bold.Sprint("3. Admin API (model registry)"))
	displayStageStatus(status.AdminAPI)
	fmt.Println()

	// Stage 4: Routing Policy
	fmt.Println(output.Bold.Sprint("4. Routing Policy"))
	displayStageStatus(status.RoutingPolicy)
	fmt.Println()

	// Stage 5: Inference Endpoint
	fmt.Println(output.Bold.Sprint("5. Inference Endpoint"))
	displayStageStatus(status.InferenceEndpoint)
	fmt.Println()

	// Summary and suggested actions
	fmt.Println(output.Muted.Sprint("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()
	if status.IsFullyOperational {
		fmt.Println(output.Success.Sprint("Summary: ✓ Model is fully operational"))
	} else {
		fmt.Println(output.Error.Sprint("Summary: ❌ Model has issues"))
		if len(status.SuggestedActions) > 0 {
			fmt.Println()
			fmt.Println(output.Bold.Sprint("Suggested Actions:"))
			for _, action := range status.SuggestedActions {
				fmt.Printf("  → %s\n", action)
			}
		}
	}
	fmt.Println()
}

// displayStageStatus displays a single pipeline stage
func displayStageStatus(stage admin.PipelineStage) {
	for _, check := range stage.Checks {
		icon := getStatusIcon(check.Status)
		statusColor := getStatusColor(check.Status)
		fmt.Printf("   ├─ %-20s %s %s\n", check.Name+":", statusColor.Sprint(icon), statusColor.Sprint(check.Value))
		if check.Details != "" {
			fmt.Printf("   │  %s\n", output.Muted.Sprint(check.Details))
		}
		if check.Error != "" {
			fmt.Printf("   │  %s\n", output.Error.Sprintf("ERROR: %s", check.Error))
		}
	}
	if stage.Message != "" {
		fmt.Printf("   └─ %s\n", output.Info.Sprint(stage.Message))
	}
}

// getStatusIcon returns the appropriate icon for a status
func getStatusIcon(status string) string {
	switch status {
	case "ok", "success", "ready":
		return "✓"
	case "warning", "pending":
		return "⚠"
	case "error", "missing", "failed":
		return "✗"
	default:
		return "-"
	}
}

// getStatusColor returns the appropriate color for a status
func getStatusColor(status string) *color.Color {
	switch status {
	case "ok", "success", "ready":
		return output.Success
	case "warning", "pending":
		return output.Warning
	case "error", "missing", "failed":
		return output.Error
	default:
		return output.Muted
	}
}
