// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cli"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewRegistryCommand creates the model registry parent command
func NewRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage the model registry",
		Long: `Manage the model registry - add, list, and remove models from HuggingFace Hub.

The registry stores metadata about models available to the platform. Models must
be registered before they can be cached or deployed.

Examples:
  # Register a new model
  ai-aas model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b

  # List all models with their status
  ai-aas model registry list

  # Get detailed info about a model
  ai-aas model registry info mistral-7b

Workflow:
  1. Register model    ai-aas model registry add <hf-id> --name <name>
  2. Cache model       ai-aas model cache pull <name>
  3. Deploy model      ai-aas model deploy create <name> -e <env>
  4. Test inference    ai-aas model troubleshoot test <name> -e <env>`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newRegistryAddCommand())
	cmd.AddCommand(newRegistryListCommand())
	cmd.AddCommand(newRegistryInfoCommand())
	cmd.AddCommand(newRegistryRemoveCommand())
	cmd.AddCommand(newRegistryRenameCommand())
	cmd.AddCommand(newRegistryStatusCommand())

	return cmd
}

// newRegistryAddCommand creates the registry add subcommand
func newRegistryAddCommand() *cobra.Command {
	var (
		name          string
		requiresAuth  bool
		licenseType   string
		acceptLicense bool
		gpuMemory     int
		cpuMemory     int
		modelType     string
	)

	cmd := &cobra.Command{
		Use:   "add <hf-model-id>",
		Short: "Register a model from HuggingFace Hub",
		Long: `Register a model from HuggingFace Hub in the platform registry.

This command fetches model metadata from HuggingFace and creates a registry entry.
For gated models (like Llama), you must first accept the license on HuggingFace.

Model types: text (default), vision-language, embedding, audio

Examples:
  # Add a text model (default type)
  ai-aas model registry add mistralai/Mistral-7B-v0.1 --name mistral-7b

  # Add a vision-language model
  ai-aas model registry add Qwen/Qwen2-VL-7B-Instruct \
    --name qwen2-vl-7b-instruct --model-type vision-language --gpu-memory 16

  # Add an embedding model
  ai-aas model registry add sentence-transformers/all-MiniLM-L6-v2 \
    --name all-minilm-l6-v2 --model-type embedding

  # Add a gated model (must accept license on HuggingFace first)
  ai-aas model registry add meta-llama/Llama-3-8B-Instruct \
    --name llama-3-8b --accept-license

  # Add with resource recommendations
  ai-aas model registry add mistralai/Mixtral-8x7B-v0.1 \
    --name mixtral-8x7b --gpu-memory 80 --cpu-memory 128

See Also:
  ai-aas model registry list    List registered models
  ai-aas model cache pull       Download model to cache
  ai-aas model deploy create    Deploy model to cluster`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hfModelID := args[0]

			// Validate model type
			validModelTypes := []string{"text", "vision-language", "embedding", "audio"}
			if modelType != "" {
				valid := false
				for _, vt := range validModelTypes {
					if modelType == vt {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("invalid model type: %s\nValid types: text, vision-language, embedding, audio", modelType)
				}
			}

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Check model on HuggingFace
			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))

			fmt.Printf("Checking model on HuggingFace Hub: %s\n", hfModelID)

			modelInfo, err := hfClient.GetModel(ctx, hfModelID)
			if err != nil {
				if err == huggingface.ErrModelNotFound {
					return fmt.Errorf("model not found on HuggingFace Hub: %s", hfModelID)
				}
				if err == huggingface.ErrForbidden {
					return fmt.Errorf("access denied to model %s. This may be a gated model requiring license acceptance.\nVisit: https://huggingface.co/%s", hfModelID, hfModelID)
				}
				return fmt.Errorf("failed to check model: %w", err)
			}

			// Check if model is gated
			if modelInfo.Gated {
				if !acceptLicense {
					return fmt.Errorf("gated model requires license acceptance\n\n   License: %s\n   Accept license at: https://huggingface.co/%s\n\nAfter accepting the license on HuggingFace, run again with --accept-license", huggingface.GetLicenseFullName(licenseType), hfModelID)
				}
				requiresAuth = true
			}

			// Determine if auth is required
			if modelInfo.Private {
				requiresAuth = true
			}

			// Extract license info
			if licenseType == "" {
				if info, err := hfClient.GetLicenseInfo(ctx, hfModelID); err == nil {
					licenseType = info.LicenseType
				}
			}

			// Register in platform via Admin API
			fmt.Printf("Registering model in platform as: %s\n", name)

			adminEndpoint := cfg.GetAdminEndpoint()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			model, err := regClient.Add(ctx, registry.AddRequest{
				HFModelID:     hfModelID,
				Name:          name,
				RequiresAuth:  requiresAuth,
				LicenseType:   licenseType,
				AcceptLicense: acceptLicense,
				GPUMemoryGB:   gpuMemory,
				CPUMemoryGB:   cpuMemory,
				ModelType:     modelType,
			})
			if err != nil {
				return fmt.Errorf("failed to register model: %w", err)
			}

			fmt.Println()
			cli.PrintModelRegistered(model.Name, model.HFModelID)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "internal name for the model (required)")
	cmd.Flags().BoolVar(&requiresAuth, "requires-auth", false, "model requires HuggingFace authentication")
	cmd.Flags().StringVar(&licenseType, "license", "", "license type (auto-detected if not specified)")
	cmd.Flags().BoolVar(&acceptLicense, "accept-license", false, "accept gated model license terms")
	cmd.Flags().IntVar(&gpuMemory, "gpu-memory", 0, "recommended GPU memory in GB")
	cmd.Flags().IntVar(&cpuMemory, "cpu-memory", 0, "recommended CPU memory in GB")
	cmd.Flags().StringVar(&modelType, "model-type", "", "model type (text, vision-language, embedding, audio)")

	cmd.MarkFlagRequired("name")

	return cmd
}

// newRegistryListCommand creates the registry list subcommand
func newRegistryListCommand() *cobra.Command {
	var (
		cached      bool
		deployed    bool
		orphaned    bool
		environment string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered models",
		Long: `List all models registered in the platform registry.

Filter by cache status, deployment status, or environment.

Examples:
  # List all models
  ai-aas model registry list

  # List only cached models
  ai-aas model registry list --cached

  # List deployed models in production
  ai-aas model registry list --deployed -e production

  # Output as JSON
  ai-aas model registry list --format json

See Also:
  ai-aas model registry info      View model details
  ai-aas model registry status    View model status overview`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

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
				fmt.Println()
				fmt.Println("To register a model:")
				fmt.Println("  ai-aas model registry add <hf-model-id> --name <name>")
				return nil
			}

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

// newRegistryInfoCommand creates the registry info subcommand
func newRegistryInfoCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "info <model-name>",
		Short: "Show detailed model information",
		Long: `Display detailed information about a registered model.

Shows registry metadata, license info, resource recommendations, cache status,
and deployment status.

Examples:
  # View model details
  ai-aas model registry info mistral-7b

  # Output as JSON
  ai-aas model registry info mistral-7b --format json

See Also:
  ai-aas model registry list      List all models
  ai-aas model registry status    View status overview
  ai-aas model cache info         View cache details`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

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

			model, err := regClient.Get(ctx, name)
			if err != nil {
				return fmt.Errorf("model not found: %s\n\nTo see available models:\n  ai-aas model registry list", name)
			}

			if format == "json" {
				return output.PrintJSON(model, true)
			}

			// Display model information
			fmt.Printf("Model: %s\n", model.Name)
			fmt.Println("────────────────────────────────────────────────────")

			// Registry info
			fmt.Printf("\nRegistry\n")
			fmt.Printf("  ID:            %s\n", model.ID)
			fmt.Printf("  HF Model ID:   %s\n", model.HFModelID)
			fmt.Printf("  HF Revision:   %s\n", model.HFRevision)
			if model.ModelType != "" {
				fmt.Printf("  Model Type:    %s\n", model.ModelType)
			}
			fmt.Printf("  Created:       %s\n", model.CreatedAt.Format(time.RFC3339))
			fmt.Printf("  Updated:       %s\n", model.UpdatedAt.Format(time.RFC3339))

			// License info
			if model.IsGated || model.LicenseType != "" {
				fmt.Printf("\nLicense\n")
				if model.LicenseType != "" {
					fmt.Printf("  Type:          %s\n", huggingface.GetLicenseFullName(model.LicenseType))
				}
				if model.IsGated {
					fmt.Printf("  Gated:         Yes\n")
				}
				if model.LicenseAcceptedAt != nil {
					fmt.Printf("  Accepted:      %s\n", model.LicenseAcceptedAt.Format(time.RFC3339))
					fmt.Printf("  Accepted By:   %s\n", model.LicenseAcceptedBy)
				}
			}

			// Authentication
			if model.RequiresAuth {
				fmt.Printf("\nAuthentication\n")
				fmt.Printf("  Requires HF Token: Yes\n")
			}

			// Resource recommendations
			if model.RecommendedGPUMemoryGB > 0 || model.RecommendedCPUMemoryGB > 0 {
				fmt.Printf("\nResource Recommendations\n")
				if model.RecommendedGPUMemoryGB > 0 {
					fmt.Printf("  GPU Memory:    %d GB\n", model.RecommendedGPUMemoryGB)
				}
				if model.RecommendedCPUMemoryGB > 0 {
					fmt.Printf("  CPU Memory:    %d GB\n", model.RecommendedCPUMemoryGB)
				}
			}

			// Cache status
			fmt.Printf("\nCache Status\n")
			if model.CacheStatus == "" || model.CacheStatus == "none" {
				fmt.Printf("  Status:        Not cached\n")
				fmt.Printf("\n  To cache this model:\n")
				fmt.Printf("    ai-aas model cache pull %s\n", name)
			} else {
				fmt.Printf("  Status:        %s\n", model.CacheStatus)

				cacheEntries, err := regClient.GetCache(ctx, name)
				if err == nil && len(cacheEntries) > 0 {
					latest := cacheEntries[0]
					fmt.Printf("  Version:       %s\n", latest.Version)
					fmt.Printf("  Size:          %s\n", output.FormatBytes(latest.SizeBytes))
					fmt.Printf("  Files:         %d\n", latest.FileCount)
					fmt.Printf("  Cached At:     %s\n", latest.CachedAt.Format(time.RFC3339))
				}
			}

			// Deployment status
			fmt.Printf("\nDeployment Status\n")
			if model.DeploymentStatus == "" || model.DeploymentStatus == "none" {
				fmt.Printf("  Status:        Not deployed\n")
				fmt.Printf("\n  To deploy this model:\n")
				fmt.Printf("    ai-aas model deploy create %s -e development\n", name)
			} else {
				fmt.Printf("  Status:        %s\n", model.DeploymentStatus)
			}

			// Version pinning
			if model.PinnedVersion != "" {
				fmt.Printf("\nVersion Pinning\n")
				fmt.Printf("  Pinned To:     %s\n", model.PinnedVersion)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// newRegistryRemoveCommand creates the registry remove subcommand
func newRegistryRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <model-name>",
		Short: "Remove a model from the registry",
		Long: `Remove a model from the platform registry.

This removes the registry entry but does not delete cached files or affect
running deployments unless --force is used.

Examples:
  # Remove a model (fails if deployed)
  ai-aas model registry remove old-model

  # Force remove including deployed instances
  ai-aas model registry remove old-model --force

See Also:
  ai-aas model cache delete       Delete cached files
  ai-aas model deploy delete      Remove deployment first`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

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
				return fmt.Errorf("model %s has active deployments\n\nTo remove the deployment first:\n  ai-aas model deploy delete %s -e <environment>\n\nOr use --force to remove anyway", name, name)
			}

			fmt.Printf("Removing model: %s\n", name)

			if err := regClient.Remove(ctx, name, force); err != nil {
				return fmt.Errorf("failed to remove model: %w", err)
			}

			cli.PrintModelDeleted(name)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force removal even if deployed")

	return cmd
}

// newRegistryStatusCommand creates the registry status subcommand
func newRegistryStatusCommand() *cobra.Command {
	var (
		environment string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "status [model-name]",
		Short: "Show model status overview",
		Long: `Show the current status of a model or all models.

Displays registry, cache, and deployment status at a glance.

Examples:
  # Show status of all models
  ai-aas model registry status

  # Show status of a specific model
  ai-aas model registry status mistral-7b

  # Filter by environment
  ai-aas model registry status -e production

See Also:
  ai-aas model registry info      Detailed model information
  ai-aas model deploy status      Deployment-specific status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

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

			// Single model status
			if len(args) > 0 {
				name := args[0]
				return showSingleModelStatus(ctx, regClient, name, format)
			}

			// All models status
			return showAllModelStatus(ctx, regClient, environment, format)
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// newRegistryRenameCommand creates the registry rename subcommand
func newRegistryRenameCommand() *cobra.Command {
	var skipCacheMigration bool

	cmd := &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a model in the registry",
		Long: `Rename a model in the platform registry.

This updates the model name in the registry and optionally migrates cached files
to the new name. Model names must follow naming conventions (alphanumeric, hyphens,
underscores only - no dots or special characters).

Common use case: Fixing model names with problematic characters (e.g., dots in version numbers).

Examples:
  # Rename a model with invalid characters
  ai-aas model registry rename mistral-7b-instruct-v0.3 mistral-7b-instruct-v03

  # Rename without migrating cache (faster, use if not cached)
  ai-aas model registry rename old-name new-name --skip-cache-migration

Warning:
  - Model cannot be renamed if it has active deployments
  - Cache migration may take time for large models
  - Deployments using the old name must be updated manually

See Also:
  ai-aas model registry list      List all models
  ai-aas model deploy update      Update deployment after rename`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName := args[0]
			newName := args[1]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// Display operation
			fmt.Printf("Renaming model: %s → %s\n\n", oldName, newName)

			// Call API to rename
			resp, err := regClient.Rename(ctx, oldName, registry.RenameRequest{
				NewName:      newName,
				MigrateCache: !skipCacheMigration,
			})
			if err != nil {
				return fmt.Errorf("failed to rename model: %w", err)
			}

			// Show results
			cli.PrintSuccess("Registry entry updated")

			if resp.CacheMigrated {
				sizeStr := cli.FormatBytes(resp.CacheSizeBytes)
				cli.PrintSuccess(fmt.Sprintf("Cache migrated (%s)", sizeStr))
			} else if !skipCacheMigration {
				cli.PrintWarning("No cache to migrate")
			}

			fmt.Println()
			fmt.Println("Model renamed successfully.")

			// Next steps
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Printf("  ai-aas model registry info %s     # Verify the rename\n", newName)
			fmt.Printf("  ai-aas model deploy create %s -e <env>  # Deploy with new name\n", newName)

			return nil
		},
	}

	cmd.Flags().BoolVar(&skipCacheMigration, "skip-cache-migration", false, "don't migrate cached files (use if cache is empty)")

	return cmd
}

func showSingleModelStatus(ctx context.Context, client *registry.Client, name, format string) error {
	model, err := client.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("model not found: %s", name)
	}

	if format == "json" {
		return output.PrintJSON(model, true)
	}

	// Status indicators
	regIcon := "+"
	cacheIcon := "-"
	deployIcon := "-"

	if model.CacheStatus == "ready" {
		cacheIcon = "+"
	} else if model.CacheStatus == "downloading" {
		cacheIcon = "~"
	}

	if model.DeploymentStatus == "ready" {
		deployIcon = "+"
	} else if model.DeploymentStatus == "deploying" {
		deployIcon = "~"
	} else if model.DeploymentStatus == "disabled" {
		deployIcon = "x"
	}

	fmt.Printf("\n%s (%s)\n", name, model.HFModelID)
	fmt.Println("────────────────────────────────────────────────────")
	fmt.Printf("  Registry:    [%s] registered\n", regIcon)
	fmt.Printf("  Cache:       [%s] %s\n", cacheIcon, statusText(model.CacheStatus, "not cached"))
	fmt.Printf("  Deployment:  [%s] %s\n", deployIcon, statusText(model.DeploymentStatus, "not deployed"))

	// Suggest next action
	fmt.Println()
	if model.CacheStatus != "ready" && model.DeploymentStatus == "" {
		fmt.Println("Next step:")
		fmt.Printf("  ai-aas model cache pull %s\n", name)
	} else if model.CacheStatus == "ready" && (model.DeploymentStatus == "" || model.DeploymentStatus == "none") {
		fmt.Println("Next step:")
		fmt.Printf("  ai-aas model deploy create %s -e development\n", name)
	} else if model.DeploymentStatus == "ready" {
		fmt.Println("Model is ready for inference:")
		fmt.Printf("  ai-aas model troubleshoot test %s -e <environment>\n", name)
	}

	return nil
}

func showAllModelStatus(ctx context.Context, client *registry.Client, environment, format string) error {
	models, err := client.List(ctx, registry.ListOptions{
		Environment: environment,
	})
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models found.")
		fmt.Println()
		fmt.Println("To register a model:")
		fmt.Println("  ai-aas model registry add <hf-model-id> --name <name>")
		return nil
	}

	if format == "json" {
		return output.PrintJSON(models, true)
	}

	table := output.NewTableWriter()
	table.SetHeader([]string{"MODEL", "REGISTRY", "CACHE", "DEPLOY", "STATUS"})

	for _, m := range models {
		regIcon := "[+]"

		cacheIcon := "[-]"
		if m.CacheStatus == "ready" {
			cacheIcon = "[+]"
		} else if m.CacheStatus == "downloading" {
			cacheIcon = "[~]"
		}

		deployIcon := "[-]"
		if m.DeploymentStatus == "ready" {
			deployIcon = "[+]"
		} else if m.DeploymentStatus == "deploying" {
			deployIcon = "[~]"
		} else if m.DeploymentStatus == "disabled" {
			deployIcon = "[x]"
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
	fmt.Println()
	fmt.Println("Legend: [+] ready  [-] not available  [~] in progress  [x] disabled")

	return nil
}

func statusText(status, defaultText string) string {
	if status == "" || status == "none" {
		return defaultText
	}
	return status
}
