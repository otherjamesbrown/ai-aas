// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/spf13/cobra"
)

// NewStatusCommand creates the model status command
func NewStatusCommand() *cobra.Command {
	var (
		environment string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "status [model-name]",
		Short: "Show model status",
		Long: `Show the current status of a model or all models.

Examples:
  # Show status of all models
  ai-aas-cli model status

  # Show status of specific model
  ai-aas-cli model status llama-3-8b

  # Show status in specific environment
  ai-aas-cli model status llama-3-8b --environment production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Use Admin API endpoint for registry operations
			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			regClient := registry.NewClient(apiClient)
			deployClient := client.NewDeploymentClient(apiClient)

			// Single model status
			if len(args) > 0 {
				name := args[0]
				return showModelStatus(ctx, regClient, deployClient, name, environment, format)
			}

			// All models status
			return showAllModelsStatus(ctx, regClient, deployClient, environment, format)
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func showModelStatus(ctx context.Context, regClient *registry.Client, deployClient *client.DeploymentClient, name, environment, format string) error {
	model, err := regClient.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	// Try to get deployment details if environment is specified
	var deployment *client.Deployment
	if environment != "" {
		deployment, _ = deployClient.Get(ctx, name, environment)
	}

	if format == "json" {
		result := map[string]interface{}{
			"model":      model,
			"deployment": deployment,
		}
		return output.PrintJSON(result, true)
	}

	// Status indicators
	registryStatus := "✅"
	cacheStatus := "❌"
	deployStatus := "❌"

	if model.CacheStatus == "ready" {
		cacheStatus = "✅"
	} else if model.CacheStatus == "downloading" {
		cacheStatus = "⏳"
	}

	deployStatusStr := model.DeploymentStatus
	if deployment != nil {
		deployStatusStr = deployment.Status
	}

	if deployStatusStr == "ready" {
		deployStatus = "✅"
	} else if deployStatusStr == "deploying" {
		deployStatus = "⏳"
	} else if deployStatusStr == "disabled" {
		deployStatus = "⏸️"
	}

	fmt.Printf("\n%s %s\n", name, model.HFModelID)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Registry:    %s registered\n", registryStatus)
	fmt.Printf("  Cache:       %s %s\n", cacheStatus, model.CacheStatus)

	// Show deployment status with duration if available
	if deployment != nil && deployment.StatusChangedAt != nil {
		duration := output.FormatDurationSince(deployment.StatusChangedAt)
		fmt.Printf("  Deployment:  %s %s (%s)\n", deployStatus, deployStatusStr, duration)
	} else {
		fmt.Printf("  Deployment:  %s %s\n", deployStatus, deployStatusStr)
	}

	return nil
}

func showAllModelsStatus(ctx context.Context, regClient *registry.Client, deployClient *client.DeploymentClient, environment, format string) error {
	models, err := regClient.List(ctx, registry.ListOptions{
		Environment: environment,
	})
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	// Fetch deployments for duration information if environment specified
	deploymentsMap := make(map[string]*client.Deployment)
	if environment != "" {
		deployments, _ := deployClient.List(ctx, client.ListOptions{
			Environment: environment,
		})
		for i := range deployments {
			deploymentsMap[deployments[i].ModelName] = &deployments[i]
		}
	}

	if format == "json" {
		result := map[string]interface{}{
			"models":      models,
			"deployments": deploymentsMap,
		}
		return output.PrintJSON(result, true)
	}

	table := output.NewTableWriter()
	table.SetHeader([]string{"MODEL", "REGISTRY", "CACHED", "DEPLOYED", "STATUS", "DURATION"})

	for _, m := range models {
		regIcon := "✅"

		cacheIcon := "❌"
		if m.CacheStatus == "ready" {
			cacheIcon = "✅"
		} else if m.CacheStatus == "downloading" {
			cacheIcon = "⏳"
		}

		deployIcon := "❌"
		deployStatusStr := m.DeploymentStatus

		// Get deployment details if available
		deployment := deploymentsMap[m.Name]
		if deployment != nil {
			deployStatusStr = deployment.Status
		}

		if deployStatusStr == "ready" {
			deployIcon = "✅"
		} else if deployStatusStr == "deploying" {
			deployIcon = "⏳"
		} else if deployStatusStr == "disabled" {
			deployIcon = "⏸️"
		}

		status := "registered"
		if deployStatusStr == "ready" {
			status = "ready"
		} else if m.CacheStatus == "ready" {
			status = "cached"
		}

		// Calculate duration if available
		durationStr := "-"
		if deployment != nil && deployment.StatusChangedAt != nil {
			durationStr = output.FormatDurationSince(deployment.StatusChangedAt)
		}

		table.Append([]string{
			m.Name,
			regIcon,
			cacheIcon,
			deployIcon,
			status,
			durationStr,
		})
	}

	table.Render()
	return nil
}
