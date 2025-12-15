// Package recipe provides commands for managing model recipes.
package recipe

import (
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command for recipes
// TODO: Implementation pending - see list_test.go for expected behavior (T025)
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
	// TODO: Implement in T025
	return nil
}
