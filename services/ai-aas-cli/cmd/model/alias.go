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

// NewAliasCommand creates the model alias command group
func NewAliasCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage model aliases",
		Long: `Manage model aliases for version abstraction.

Aliases allow you to create friendly names that point to specific models,
enabling version-agnostic routing and easy model updates.

Examples:
  # List all aliases
  ai-aas-cli model alias list

  # Create an alias
  ai-aas-cli model alias create gpt --model-name llama-3-8b --description "Default GPT model"

  # Update an alias to point to a different model
  ai-aas-cli model alias update gpt --model-name llama-3-70b

  # Delete an alias
  ai-aas-cli model alias delete gpt`,
	}

	cmd.AddCommand(aliasListCommand())
	cmd.AddCommand(aliasCreateCommand())
	cmd.AddCommand(aliasUpdateCommand())
	cmd.AddCommand(aliasDeleteCommand())
	cmd.AddCommand(aliasGetCommand())

	return cmd
}

func getAPIClient(cfg *config.Config) (*api.Client, error) {
	if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
		return nil, fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
	}

	opts := []api.ClientOption{}
	if cfg.TLSInsecure {
		opts = append(opts, api.WithInsecureSkipVerify())
	}
	return api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...), nil
}

func aliasListCommand() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all model aliases",
		Long:  "List all model aliases with their target models.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			aliases, err := regClient.ListAliases(ctx)
			if err != nil {
				return fmt.Errorf("list aliases: %w", err)
			}

			if formatFlag == "json" {
				return output.PrintJSON(aliases)
			}

			if len(aliases) == 0 {
				fmt.Println("No aliases found.")
				return nil
			}

			// Table output
			fmt.Printf("%-20s %-25s %-30s\n", "ALIAS", "TARGET MODEL", "DESCRIPTION")
			fmt.Println("────────────────────────────────────────────────────────────────────────────")
			for _, a := range aliases {
				desc := a.Description
				if len(desc) > 30 {
					desc = desc[:27] + "..."
				}
				fmt.Printf("%-20s %-25s %-30s\n", a.AliasName, a.ModelName, desc)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")

	return cmd
}

func aliasCreateCommand() *cobra.Command {
	var (
		modelName   string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create <alias-name>",
		Short: "Create a new model alias",
		Long: `Create a new alias pointing to a model.

Examples:
  ai-aas-cli model alias create gpt --model-name llama-3-8b
  ai-aas-cli model alias create gpt --model-name llama-3-8b --description "Default GPT model"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			alias, err := regClient.CreateAlias(ctx, registry.AliasCreateRequest{
				AliasName:   aliasName,
				ModelName:   modelName,
				Description: description,
			})
			if err != nil {
				return fmt.Errorf("create alias: %w", err)
			}

			fmt.Printf("Created alias '%s' -> '%s'\n", alias.AliasName, alias.ModelName)
			if description != "" {
				fmt.Printf("  Description: %s\n", description)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&modelName, "model-name", "", "Target model name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Alias description")
	_ = cmd.MarkFlagRequired("model-name")

	return cmd
}

func aliasUpdateCommand() *cobra.Command {
	var (
		modelName   string
		description string
	)

	cmd := &cobra.Command{
		Use:   "update <alias-name>",
		Short: "Update an existing model alias",
		Long: `Update an alias to point to a different model or change its description.

Examples:
  ai-aas-cli model alias update gpt --model-name llama-3-70b
  ai-aas-cli model alias update gpt --description "Updated GPT model"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

			if modelName == "" && description == "" {
				return fmt.Errorf("at least one of --model-name or --description must be provided")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			alias, err := regClient.UpdateAlias(ctx, aliasName, registry.AliasUpdateRequest{
				ModelName:   modelName,
				Description: description,
			})
			if err != nil {
				return fmt.Errorf("update alias: %w", err)
			}

			fmt.Printf("Updated alias '%s'\n", aliasName)
			if modelName != "" {
				fmt.Printf("  New target: %s\n", alias.ModelName)
			}
			if description != "" {
				fmt.Printf("  Description: %s\n", description)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&modelName, "model-name", "", "New target model name")
	cmd.Flags().StringVar(&description, "description", "", "New alias description")

	return cmd
}

func aliasDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <alias-name>",
		Short: "Delete a model alias",
		Long: `Delete an alias.

Examples:
  ai-aas-cli model alias delete gpt
  ai-aas-cli model alias delete gpt --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			// Get alias details for confirmation
			alias, err := regClient.GetAlias(ctx, aliasName)
			if err != nil {
				return fmt.Errorf("alias not found: %s", aliasName)
			}

			if !force {
				fmt.Printf("Delete alias '%s' (currently pointing to '%s')? [y/N]: ", aliasName, alias.ModelName)
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			if err := regClient.DeleteAlias(ctx, aliasName); err != nil {
				return fmt.Errorf("delete alias: %w", err)
			}

			fmt.Printf("Deleted alias '%s'\n", aliasName)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")

	return cmd
}

func aliasGetCommand() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "get <alias-name>",
		Short: "Get details of a specific alias",
		Long:  "Get details of a specific model alias including the target model.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aliasName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			alias, err := regClient.GetAlias(ctx, aliasName)
			if err != nil {
				return fmt.Errorf("alias not found: %s", aliasName)
			}

			if formatFlag == "json" {
				return output.PrintJSON(alias)
			}

			fmt.Printf("Alias: %s\n", alias.AliasName)
			fmt.Printf("  Target Model: %s\n", alias.ModelName)
			if alias.Description != "" {
				fmt.Printf("  Description: %s\n", alias.Description)
			}
			fmt.Printf("  Created: %s\n", alias.CreatedAt.Format(time.RFC3339))
			fmt.Printf("  Updated: %s\n", alias.UpdatedAt.Format(time.RFC3339))

			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")

	return cmd
}
