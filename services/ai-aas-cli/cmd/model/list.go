// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewListCommand creates the model list command
func NewListCommand() *cobra.Command {
	var (
		cached      bool
		deployed    bool
		orphaned    bool
		environment string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered models",
		Long: `List all models registered in the platform.

Examples:
  # List all models
  ai-aas-cli model list

  # List only cached models
  ai-aas-cli model list --cached

  # List deployed models in production
  ai-aas-cli model list --deployed --environment production

  # Output as JSON
  ai-aas-cli model list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiEndpoint := viper.GetString("api.endpoint")
			apiKey := viper.GetString("api.key")

			if apiEndpoint == "" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			apiClient := api.NewClient(apiEndpoint, apiKey)
			regClient := registry.NewClient(apiClient)

			models, err := regClient.List(ctx, registry.ListOptions{
				Cached:      cached,
				Deployed:    deployed,
				Orphaned:    orphaned,
				Environment: environment,
			})
			if err != nil {
				return fmt.Errorf("failed to list models: %w", err)
			}

			if len(models) == 0 {
				fmt.Println("No models found.")
				return nil
			}

			// Output based on format
			if format == "json" {
				return output.PrintJSON(models, true)
			}

			// Table output
			table := output.NewTableWriter()
			table.SetHeader([]string{"NAME", "HF MODEL ID", "CACHED", "DEPLOYED", "STATUS"})

			for _, m := range models {
				cached := "No"
				if m.CacheStatus == "ready" {
					cached = "Yes"
				}
				
				deployed := "No"
				if m.DeploymentStatus != "" && m.DeploymentStatus != "none" {
					deployed = "Yes"
				}

				status := "registered"
				if m.DeploymentStatus == "ready" {
					status = "ready"
				} else if m.CacheStatus == "ready" {
					status = "cached"
				}

				table.Append([]string{
					m.Name,
					truncate(m.HFModelID, 35),
					cached,
					deployed,
					status,
				})
			}

			table.Render()
			fmt.Printf("\nTotal: %d model(s)\n", len(models))

			return nil
		},
	}

	cmd.Flags().BoolVar(&cached, "cached", false, "show only cached models")
	cmd.Flags().BoolVar(&deployed, "deployed", false, "show only deployed models")
	cmd.Flags().BoolVar(&orphaned, "orphaned", false, "show orphaned cache entries")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

