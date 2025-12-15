// Package recipe provides commands for managing model recipes.
package recipe

import "github.com/spf13/cobra"

// NewRecipeCommand creates the recipe command group
func NewRecipeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Manage model recipes",
		Long: `Manage model deployment recipes - known-good configurations for AI models.

Recipes define pre-configured settings for deploying models with specific
runtimes, resource requirements, and runtime arguments. They serve as
templates that capture best practices for deploying specific models.

Available commands:
  list      List all available recipes
  show      Show details of a specific recipe
  validate  Validate a recipe YAML file

Examples:
  # List all available recipes
  ai-aas-cli model recipe list

  # Show details for a specific recipe
  ai-aas-cli model recipe show mistral-7b-instruct-v03

  # Validate a custom recipe file
  ai-aas-cli model recipe validate my-recipe.yaml

For more information about recipes, see:
  https://docs.ai-aas.io/recipes`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewShowCommand())
	cmd.AddCommand(NewValidateCommand())

	return cmd
}
