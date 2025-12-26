// Package recipe provides commands for managing model recipes.
package recipe

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// NewListCommand creates the list command for recipes
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all model recipes",
		Long: `List all model recipes available in the AI-AAS platform.

Recipes define pre-configured settings for deploying models with specific
runtimes, resource requirements, and runtime arguments.`,
		Example: `  # List all recipes
  ai-aas-cli model recipe list

  # List only vLLM recipes
  ai-aas-cli model recipe list --runtime vllm

  # List recipes in JSON format
  ai-aas-cli model recipe list --format json

  # List recipes in YAML format
  ai-aas-cli model recipe list --format yaml`,
		RunE: runList,
	}

	cmd.Flags().String("runtime", "", "Filter by runtime (vllm, triton, tgi)")
	cmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")

	return cmd
}

// runList executes the list command
func runList(cmd *cobra.Command, args []string) error {
	// Get flags
	runtime, _ := cmd.Flags().GetString("runtime")
	format, _ := cmd.Flags().GetString("format")

	// Get profile flag and load config with profile support
	profileName, _ := cmd.Flags().GetString("profile")
	cfg, _, err := config.GetEffectiveConfig(profileName)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use Admin API endpoint
	adminEndpoint := cfg.AdminAPIEndpoint
	if adminEndpoint == "" {
		adminEndpoint = cfg.APIEndpoint // fallback for backward compatibility
	}

	if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
		return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := []api.ClientOption{}
	if cfg.TLSInsecure {
		opts = append(opts, api.WithInsecureSkipVerify())
	}
	apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)

	// Build API path with optional runtime filter
	path := "/v1/recipes"
	if runtime != "" {
		path = fmt.Sprintf("/v1/recipes?runtime=%s", url.QueryEscape(runtime))
	}

	// Call API
	var response RecipesResponse
	if err := apiClient.Get(ctx, path, &response); err != nil {
		return fmt.Errorf("failed to list recipes: %w", err)
	}

	// Handle empty list
	if len(response.Recipes) == 0 {
		if runtime != "" {
			fmt.Printf("No recipes found for runtime: %s\n", runtime)
			fmt.Println("Try without --runtime to see all recipes")
		} else {
			fmt.Println("No recipes found.")
			fmt.Println("Create a recipe with: ai-aas-cli model recipe create")
		}
		return nil
	}

	// Output based on format
	switch format {
	case "json":
		return output.PrintJSON(response.Recipes, true)
	case "yaml":
		data, err := yaml.Marshal(response.Recipes)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		fmt.Println(string(data))
		return nil
	case "table":
		// Table output
		table := output.NewTableWriter()
		table.SetHeader([]string{"NAME", "DISPLAY NAME", "RUNTIME", "MODEL ID"})

		for _, r := range response.Recipes {
			table.Append([]string{
				r.Name,
				r.DisplayName,
				r.Runtime,
				r.ModelID,
			})
		}

		table.Render()
		fmt.Printf("\nTotal: %d recipe(s)\n", len(response.Recipes))
		return nil
	default:
		return fmt.Errorf("invalid format: %s (valid formats: table, json, yaml)", format)
	}
}
