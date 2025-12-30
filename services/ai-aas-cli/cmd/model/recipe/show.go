// Package recipe provides commands for managing model recipes.
package recipe

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewShowCommand creates the show command for recipes
func NewShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <recipe-name>",
		Short: "Show details of a model recipe",
		Long: `Show detailed information about a specific model recipe.

Examples:
  # Show recipe details in table format
  ai-aas-cli model recipe show mistral-7b-instruct-v03

  # Show recipe details in JSON format
  ai-aas-cli model recipe show mistral-7b-instruct-v03 --format json

  # Show recipe details in YAML format
  ai-aas-cli model recipe show mistral-7b-instruct-v03 --format yaml`,
		Args: cobra.ExactArgs(1),
		RunE: runShow,
	}

	cmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")

	return cmd
}

// runShow executes the show command
func runShow(cmd *cobra.Command, args []string) error {
	recipeName := args[0]
	if recipeName == "" {
		return fmt.Errorf("recipe name is required")
	}

	format, _ := cmd.Flags().GetString("format")
	if format != "table" && format != "json" && format != "yaml" {
		return fmt.Errorf("format must be one of: table, json, yaml")
	}

	// Load configuration and create API client
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)

	// Get the recipe from the API
	ctx := context.Background()
	var recipe Recipe
	err = apiClient.Request(ctx, "GET", fmt.Sprintf("/v1/recipes/%s", recipeName), nil, &recipe)
	if err != nil {
		return err
	}

	// Format and output the recipe
	switch format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(recipe)
	case "yaml":
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		return encoder.Encode(recipe)
	default:
		// Table format - simple output
		fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", recipe.Name)
		if recipe.DisplayName != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Display Name: %s\n", recipe.DisplayName)
		}
		if recipe.Runtime != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Runtime: %s\n", recipe.Runtime)
		}
		return nil
	}
}
