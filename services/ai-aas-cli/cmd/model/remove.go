// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewRemoveCommand creates the model remove command
func NewRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <model-name>",
		Short: "Remove a model from registry",
		Long: `Remove a model from the platform registry.

This will not delete cached files or affect running deployments unless --force is used.

Examples:
  # Remove a model (fails if deployed)
  ai-aas-cli model remove old-model

  # Force remove including deployed instances
  ai-aas-cli model remove old-model --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

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

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// Check if model exists first
			model, err := regClient.Get(ctx, name)
			if err != nil {
				return fmt.Errorf("model not found: %s", name)
			}

			// Confirm if model has deployments and not forcing
			if model.DeploymentStatus != "" && model.DeploymentStatus != "none" && !force {
				return fmt.Errorf("model %s has active deployments. Use --force to remove anyway", name)
			}

			fmt.Printf("Removing model: %s\n", name)

			if err := regClient.Remove(ctx, name, force); err != nil {
				return fmt.Errorf("failed to remove model: %w", err)
			}

			fmt.Printf("✅ Model %s removed from registry\n", name)
			
			if model.CacheStatus == "ready" {
				fmt.Printf("\n💡 Note: Cached files still exist. To delete cache:\n")
				fmt.Printf("   ai-aas-cli model cache delete %s\n", name)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force removal even if deployed")

	return cmd
}

