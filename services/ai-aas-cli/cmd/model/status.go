// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
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
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// Single model status
			if len(args) > 0 {
				name := args[0]
				return showModelStatus(ctx, regClient, name, environment, format)
			}

			// All models status
			return showAllModelsStatus(ctx, regClient, environment, format)
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func showModelStatus(ctx context.Context, client *registry.Client, name, environment, format string) error {
	model, err := client.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	if format == "json" {
		return output.PrintJSON(model, true)
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

	if model.DeploymentStatus == "ready" {
		deployStatus = "✅"
	} else if model.DeploymentStatus == "deploying" {
		deployStatus = "⏳"
	} else if model.DeploymentStatus == "disabled" {
		deployStatus = "⏸️"
	}

	fmt.Printf("\n%s %s\n", name, model.HFModelID)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Registry:    %s registered\n", registryStatus)
	fmt.Printf("  Cache:       %s %s\n", cacheStatus, model.CacheStatus)
	fmt.Printf("  Deployment:  %s %s\n", deployStatus, model.DeploymentStatus)

	return nil
}

func showAllModelsStatus(ctx context.Context, client *registry.Client, environment, format string) error {
	models, err := client.List(ctx, registry.ListOptions{
		Environment: environment,
	})
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	if format == "json" {
		return output.PrintJSON(models, true)
	}

	table := output.NewTableWriter()
	table.SetHeader([]string{"MODEL", "REGISTRY", "CACHE", "DEPLOY", "STATUS"})

	for _, m := range models {
		regIcon := "✅"
		
		cacheIcon := "❌"
		if m.CacheStatus == "ready" {
			cacheIcon = "✅"
		} else if m.CacheStatus == "downloading" {
			cacheIcon = "⏳"
		}

		deployIcon := "❌"
		if m.DeploymentStatus == "ready" {
			deployIcon = "✅"
		} else if m.DeploymentStatus == "deploying" {
			deployIcon = "⏳"
		} else if m.DeploymentStatus == "disabled" {
			deployIcon = "⏸️"
		}

		status := "registered"
		if m.DeploymentStatus == "ready" {
			status = "ready"
		} else if m.CacheStatus == "ready" {
			status = "cached"
		}

		table.Append([]string{
			m.Name,
			regIcon,
			cacheIcon,
			deployIcon,
			status,
		})
	}

	table.Render()
	return nil
}

