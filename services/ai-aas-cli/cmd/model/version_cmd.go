// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cli"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewVersionParentCommand creates the model version parent command
func NewVersionParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Manage model versions",
		Long: `Manage model versions - check for updates, update to latest, pin to specific versions.

Models can be pinned to prevent automatic updates. Pinned models are skipped
by check and update commands unless explicitly specified.

Examples:
  # Check for available updates
  ai-aas model version check

  # Update to latest version
  ai-aas model version update mistral-7b

  # Pin model to current version
  ai-aas model version pin mistral-7b --version current

  # Unpin to allow updates
  ai-aas model version unpin mistral-7b

Workflow:
  1. Check for updates    ai-aas model version check
  2. Review changes       Check HuggingFace release notes
  3. Update model         ai-aas model version update <name> -e <env>
  4. Validate             ai-aas model troubleshoot test <name> -e <env>`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newVersionCheckCommand())
	cmd.AddCommand(newVersionUpdateCommand())
	cmd.AddCommand(newVersionPinCommand())
	cmd.AddCommand(newVersionUnpinCommand())

	return cmd
}

// newVersionCheckCommand creates the version check subcommand
func newVersionCheckCommand() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "check [model-name]",
		Short: "Check for model updates from HuggingFace",
		Long: `Check if newer versions are available on HuggingFace Hub.

Compares cached model versions with the latest SHA from HuggingFace.
Pinned models are skipped unless explicitly specified.

Examples:
  # Check all models for updates
  ai-aas model version check

  # Check a specific model
  ai-aas model version check mistral-7b

  # Output as JSON
  ai-aas model version check --format json

See Also:
  ai-aas model version update    Update to latest
  ai-aas model version pin       Pin version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))

			var models []registry.Model
			if len(args) > 0 {
				model, err := regClient.Get(ctx, args[0])
				if err != nil {
					return fmt.Errorf("model not found: %s", args[0])
				}
				models = []registry.Model{*model}
			} else {
				models, err = regClient.List(ctx, registry.ListOptions{})
				if err != nil {
					return fmt.Errorf("list models: %w", err)
				}
			}

			if len(models) == 0 {
				fmt.Println("No models found in registry.")
				fmt.Println()
				fmt.Println("To register a model:")
				fmt.Println("  ai-aas model registry add <hf-model-id> --name <name>")
				return nil
			}

			type updateResult struct {
				Name          string `json:"name"`
				HFModelID     string `json:"hf_model_id"`
				CachedVersion string `json:"cached_version"`
				LatestVersion string `json:"latest_version"`
				Pinned        bool   `json:"pinned"`
				UpdateAvail   bool   `json:"update_available"`
				Error         string `json:"error,omitempty"`
			}

			var results []updateResult
			fmt.Println("Checking for updates...")

			for _, model := range models {
				result := updateResult{
					Name:      model.Name,
					HFModelID: model.HFModelID,
					Pinned:    model.PinnedVersion != "",
				}

				if model.PinnedVersion != "" && len(args) == 0 {
					result.CachedVersion = model.PinnedVersion
					result.LatestVersion = "skipped (pinned)"
					results = append(results, result)
					continue
				}

				cacheEntries, err := regClient.GetCache(ctx, model.Name)
				if err != nil || len(cacheEntries) == 0 {
					result.CachedVersion = "not cached"
					hfInfo, err := hfClient.GetModel(ctx, model.HFModelID)
					if err != nil {
						result.Error = err.Error()
					} else {
						result.LatestVersion = truncateSHA(hfInfo.SHA)
					}
					results = append(results, result)
					continue
				}

				latestCache := cacheEntries[0]
				for _, c := range cacheEntries {
					if c.CachedAt.After(latestCache.CachedAt) {
						latestCache = c
					}
				}
				result.CachedVersion = truncateSHA(latestCache.Version)

				hfInfo, err := hfClient.GetModel(ctx, model.HFModelID)
				if err != nil {
					result.Error = err.Error()
					results = append(results, result)
					continue
				}

				result.LatestVersion = truncateSHA(hfInfo.SHA)
				result.UpdateAvail = latestCache.Version != hfInfo.SHA

				results = append(results, result)
			}

			if formatFlag == "json" {
				return output.PrintJSON(results)
			}

			fmt.Printf("\n%-20s %-12s %-12s %-15s\n", "MODEL", "CACHED", "LATEST", "STATUS")
			fmt.Println("────────────────────────────────────────────────────────────────")

			updatesAvailable := 0
			for _, r := range results {
				status := "up-to-date"
				if r.Error != "" {
					status = "error"
				} else if r.Pinned && len(args) == 0 {
					status = "pinned"
				} else if r.UpdateAvail {
					status = "UPDATE AVAIL"
					updatesAvailable++
				} else if r.CachedVersion == "not cached" {
					status = "not cached"
				}

				fmt.Printf("%-20s %-12s %-12s %-15s\n",
					r.Name, r.CachedVersion, r.LatestVersion, status)
			}

			fmt.Println()
			if updatesAvailable > 0 {
				fmt.Printf("Updates available for %d model(s).\n", updatesAvailable)
				cli.PrintNextSteps([]cli.NextStep{
					{
						Command:     "ai-aas model version update <model-name>",
						Description: "Update a model",
					},
				})
			} else {
				fmt.Println("All models are up to date.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "table", "output format: table, json")

	return cmd
}

// newVersionUpdateCommand creates the version update subcommand
func newVersionUpdateCommand() *cobra.Command {
	var (
		environment string
		canary      string
		skipDeploy  bool
	)

	cmd := &cobra.Command{
		Use:   "update <model-name>",
		Short: "Update model to latest version",
		Long: `Update a model to the latest version from HuggingFace.

Downloads the latest version and optionally performs a rolling update
of the deployment.

Examples:
  # Update model to latest
  ai-aas model version update mistral-7b

  # Update and redeploy to environment
  ai-aas model version update mistral-7b -e development

  # Update cache only (skip deployment)
  ai-aas model version update mistral-7b --skip-deploy

See Also:
  ai-aas model version check    Check for updates first
  ai-aas model version pin      Pin to prevent updates`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			model, err := regClient.Get(ctx, modelName)
			if err != nil {
				return fmt.Errorf("model not found: %s", modelName)
			}

			if model.PinnedVersion != "" {
				return fmt.Errorf("model %s is pinned to version %s\n\nTo unpin:\n  ai-aas model version unpin %s",
					modelName, truncateSHA(model.PinnedVersion), modelName)
			}

			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))

			fmt.Printf("Checking latest version for %s...\n", model.HFModelID)
			hfInfo, err := hfClient.GetModel(ctx, model.HFModelID)
			if err != nil {
				return fmt.Errorf("get HuggingFace info: %w", err)
			}

			cacheEntries, err := regClient.GetCache(ctx, modelName)
			if err == nil && len(cacheEntries) > 0 {
				latestCache := cacheEntries[0]
				for _, c := range cacheEntries {
					if c.CachedAt.After(latestCache.CachedAt) {
						latestCache = c
					}
				}
				if latestCache.Version == hfInfo.SHA {
					fmt.Printf("Model %s is already at latest version (%s)\n", modelName, truncateSHA(hfInfo.SHA))
					return nil
				}
				fmt.Printf("Update available: %s -> %s\n", truncateSHA(latestCache.Version), truncateSHA(hfInfo.SHA))
			}

			fmt.Printf("\nPulling latest version...\n")
			fmt.Printf("  Model: %s\n", model.HFModelID)
			fmt.Printf("  Version: %s\n", truncateSHA(hfInfo.SHA))

			pullCmd := NewPullCommand()
			pullCmd.SetArgs([]string{modelName, "--revision", hfInfo.SHA})
			if err := pullCmd.Execute(); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}

			cli.PrintSuccess(fmt.Sprintf("Pulled version %s", truncateSHA(hfInfo.SHA)))

			if environment != "" && !skipDeploy {
				fmt.Printf("\nUpdating deployment in %s...\n", environment)

				if canary != "" {
					fmt.Printf("  Canary rollout: %s (not yet implemented)\n", canary)
				}

				kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
				kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

				k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
					Kubeconfig: kubeconfig,
					Context:    kubecontext,
					Namespace:  environment,
				})
				if err != nil {
					return fmt.Errorf("create k8s client: %w", err)
				}

				isvcName := fmt.Sprintf("%s-%s", modelName, environment)

				if err := k8sClient.RestartInferenceService(ctx, isvcName, environment); err != nil {
					return fmt.Errorf("restart deployment: %w", err)
				}

				cli.PrintSuccess(fmt.Sprintf("Triggered rolling restart of %s", isvcName))

				fmt.Print("  Waiting for ready...")
				waitOpts := kubernetes.WaitOptions{
					Timeout:      10 * time.Minute,
					PollInterval: 5 * time.Second,
				}
				if err := k8sClient.WaitForReady(ctx, isvcName, environment, waitOpts); err != nil {
					fmt.Printf(" TIMEOUT\n")
					return fmt.Errorf("deployment did not become ready: %w", err)
				}
				fmt.Printf(" READY\n")
			}

			fmt.Println()
			cli.PrintSuccess(fmt.Sprintf("Model %s updated successfully!", modelName))

			if environment == "" {
				cli.PrintNextSteps([]cli.NextStep{
					{
						Command:     fmt.Sprintf("ai-aas model deploy restart %s -e development", modelName),
						Description: "Restart deployment to use new version",
					},
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment for deployment update")
	cmd.Flags().StringVar(&canary, "canary", "", "canary rollout percentage (future)")
	cmd.Flags().BoolVar(&skipDeploy, "skip-deploy", false, "skip deployment update (pull only)")

	return cmd
}

// newVersionPinCommand creates the version pin subcommand
func newVersionPinCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "pin <model-name>",
		Short: "Pin model to specific version",
		Long: `Pin a model to a specific version to prevent automatic updates.

Pinned models are skipped by 'check' and 'update' commands.

Examples:
  # Pin to a specific version
  ai-aas model version pin mistral-7b --version abc123def

  # Pin to the currently cached version
  ai-aas model version pin mistral-7b --version current

See Also:
  ai-aas model version unpin     Remove pin
  ai-aas model version check     Check for updates`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			if version == "current" {
				cacheEntries, err := regClient.GetCache(ctx, modelName)
				if err != nil || len(cacheEntries) == 0 {
					return fmt.Errorf("no cached version found for %s\n\nTo cache the model:\n  ai-aas model cache pull %s", modelName, modelName)
				}

				latestCache := cacheEntries[0]
				for _, c := range cacheEntries {
					if c.CachedAt.After(latestCache.CachedAt) {
						latestCache = c
					}
				}
				version = latestCache.Version
			}

			if err := regClient.Pin(ctx, modelName, version); err != nil {
				return fmt.Errorf("pin model: %w", err)
			}

			cli.PrintSuccess(fmt.Sprintf("Pinned %s to version %s", modelName, truncateSHA(version)))
			fmt.Println("Model will be skipped by 'check' and 'update' commands.")

			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "version to pin (required, or 'current')")
	_ = cmd.MarkFlagRequired("version")

	return cmd
}

// newVersionUnpinCommand creates the version unpin subcommand
func newVersionUnpinCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpin <model-name>",
		Short: "Unpin model version",
		Long: `Remove version pin from a model to allow updates.

Examples:
  # Unpin a model
  ai-aas model version unpin mistral-7b

See Also:
  ai-aas model version pin       Pin a version
  ai-aas model version check     Check for updates`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			apiClient, err := getAPIClient(cfg)
			if err != nil {
				return err
			}
			regClient := registry.NewClient(apiClient)

			model, err := regClient.Get(ctx, modelName)
			if err != nil {
				return fmt.Errorf("model not found: %s", modelName)
			}

			if model.PinnedVersion == "" {
				fmt.Printf("Model %s is not pinned.\n", modelName)
				return nil
			}

			previousVersion := model.PinnedVersion

			if err := regClient.Unpin(ctx, modelName); err != nil {
				return fmt.Errorf("unpin model: %w", err)
			}

			cli.PrintSuccess(fmt.Sprintf("Unpinned %s (was pinned to %s)", modelName, truncateSHA(previousVersion)))
			fmt.Println("Model will now be included in 'check' and 'update' commands.")

			return nil
		},
	}

	return cmd
}

// Note: getAPIClient is defined in alias.go
