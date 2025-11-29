// Package model provides CLI commands for model management.
package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewCheckUpdatesCommand creates the model check-updates command
func NewCheckUpdatesCommand() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "check-updates [model-name]",
		Short: "Check for model updates from HuggingFace",
		Long: `Check if newer versions are available on HuggingFace Hub.

Compares cached model versions with the latest SHA from HuggingFace.
Pinned models are skipped unless explicitly specified.

Examples:
  # Check all models for updates
  ai-aas-cli model check-updates

  # Check a specific model
  ai-aas-cli model check-updates llama-3-8b`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Get configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			// Get API client
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// Get HuggingFace client
			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))

			var models []registry.Model
			if len(args) > 0 {
				// Check specific model
				model, err := regClient.Get(ctx, args[0])
				if err != nil {
					return fmt.Errorf("get model: %w", err)
				}
				models = []registry.Model{*model}
			} else {
				// Check all models
				models, err = regClient.List(ctx, registry.ListOptions{})
				if err != nil {
					return fmt.Errorf("list models: %w", err)
				}
			}

			if len(models) == 0 {
				fmt.Println("No models found in registry.")
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

				// Skip pinned models unless explicitly specified
				if model.PinnedVersion != "" && len(args) == 0 {
					result.CachedVersion = model.PinnedVersion
					result.LatestVersion = "skipped (pinned)"
					results = append(results, result)
					continue
				}

				// Get cached version
				cacheEntries, err := regClient.GetCache(ctx, model.Name)
				if err != nil || len(cacheEntries) == 0 {
					result.CachedVersion = "not cached"
					// Still check latest
					hfInfo, err := hfClient.GetModel(ctx, model.HFModelID)
					if err != nil {
						result.Error = err.Error()
					} else {
						result.LatestVersion = truncateSHA(hfInfo.SHA)
					}
					results = append(results, result)
					continue
				}

				// Use latest cached version
				latestCache := cacheEntries[0]
				for _, c := range cacheEntries {
					if c.CachedAt.After(latestCache.CachedAt) {
						latestCache = c
					}
				}
				result.CachedVersion = truncateSHA(latestCache.Version)

				// Get latest from HuggingFace
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

			// Output results
			if formatFlag == "json" {
				return output.PrintJSON(results)
			}

			// Table output
			fmt.Printf("\n%-20s %-12s %-12s %-10s\n", "MODEL", "CACHED", "LATEST", "STATUS")
			fmt.Println("────────────────────────────────────────────────────────────")

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

				fmt.Printf("%-20s %-12s %-12s %-10s\n",
					r.Name, r.CachedVersion, r.LatestVersion, status)
			}

			fmt.Println()
			if updatesAvailable > 0 {
				fmt.Printf("Updates available for %d model(s). Use 'ai-aas-cli model update <name>' to update.\n", updatesAvailable)
			} else {
				fmt.Println("All models are up to date.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")

	return cmd
}

// NewUpdateCommand creates the model update command
func NewUpdateCommand() *cobra.Command {
	var (
		environment string
		canary      string
		skipDeploy  bool
	)

	cmd := &cobra.Command{
		Use:   "update <model-name>",
		Short: "Update model to latest version",
		Long: `Update a model to the latest version from HuggingFace.

This command pulls the latest version and optionally performs a rolling
update of the deployment.

Examples:
  # Update model to latest
  ai-aas-cli model update llama-3-8b

  # Update and redeploy to environment
  ai-aas-cli model update llama-3-8b -e development

  # Update with canary rollout (future)
  ai-aas-cli model update llama-3-8b -e production --canary 10%`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Get configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			// Get API client
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// Get model info
			model, err := regClient.Get(ctx, modelName)
			if err != nil {
				return fmt.Errorf("get model: %w", err)
			}

			// Check if pinned
			if model.PinnedVersion != "" {
				return fmt.Errorf("model %s is pinned to version %s. Use 'ai-aas-cli model unpin %s' first",
					modelName, model.PinnedVersion, modelName)
			}

			// Get HuggingFace client
			hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))

			// Get latest version
			fmt.Printf("Checking latest version for %s...\n", model.HFModelID)
			hfInfo, err := hfClient.GetModel(ctx, model.HFModelID)
			if err != nil {
				return fmt.Errorf("get HuggingFace info: %w", err)
			}

			// Check cached version
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

			// Pull new version
			fmt.Printf("\nPulling latest version...\n")
			fmt.Printf("  Model: %s\n", model.HFModelID)
			fmt.Printf("  Version: %s\n", truncateSHA(hfInfo.SHA))

			// Use the pull command logic (would normally call Pull service)
			pullCmd := NewPullCommand()
			pullCmd.SetArgs([]string{modelName, "--revision", hfInfo.SHA})
			if err := pullCmd.Execute(); err != nil {
				return fmt.Errorf("pull failed: %w", err)
			}

			fmt.Printf("\n✓ Pulled version %s\n", truncateSHA(hfInfo.SHA))

			// Handle deployment update
			if environment != "" && !skipDeploy {
				fmt.Printf("\nUpdating deployment in %s...\n", environment)

				if canary != "" {
					fmt.Printf("  Canary rollout: %s (note: canary routing not yet implemented)\n", canary)
				}

				// Get k8s client
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

				// Trigger rolling restart
				if err := k8sClient.RestartInferenceService(ctx, isvcName, environment); err != nil {
					return fmt.Errorf("restart deployment: %w", err)
				}

				fmt.Printf("✓ Triggered rolling restart of %s\n", isvcName)

				// Wait for ready
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

			fmt.Printf("\n✅ Model %s updated successfully!\n", modelName)

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment for deployment update")
	cmd.Flags().StringVar(&canary, "canary", "", "canary rollout percentage (e.g., 10%)")
	cmd.Flags().BoolVar(&skipDeploy, "skip-deploy", false, "skip deployment update (pull only)")

	return cmd
}

// NewPinCommand creates the model pin command
func NewPinCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "pin <model-name>",
		Short: "Pin model to specific version",
		Long: `Pin a model to a specific version to prevent automatic updates.

Pinned models are skipped by 'check-updates' and 'update' commands.

Examples:
  # Pin to a specific version
  ai-aas-cli model pin llama-3-8b --version abc123def

  # Pin to the currently cached version
  ai-aas-cli model pin llama-3-8b --version current`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.DatabaseURL == "" {
				return fmt.Errorf("DATABASE_URL not configured")
			}

			// If version is "current", get the current cached version
			if version == "current" {
				opts := []api.ClientOption{}
				if cfg.TLSInsecure {
					opts = append(opts, api.WithInsecureSkipVerify())
				}
				apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
				regClient := registry.NewClient(apiClient)

				cacheEntries, err := regClient.GetCache(ctx, modelName)
				if err != nil || len(cacheEntries) == 0 {
					return fmt.Errorf("no cached version found for %s", modelName)
				}

				// Find latest cache entry
				latestCache := cacheEntries[0]
				for _, c := range cacheEntries {
					if c.CachedAt.After(latestCache.CachedAt) {
						latestCache = c
					}
				}
				version = latestCache.Version
			}

			db, err := sql.Open("postgres", cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer db.Close()

			// Update pinned_version
			result, err := db.ExecContext(ctx, `
				UPDATE model_registry
				SET pinned_version = $1, updated_at = NOW()
				WHERE name = $2
			`, version, modelName)
			if err != nil {
				return fmt.Errorf("update model: %w", err)
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				return fmt.Errorf("model not found: %s", modelName)
			}

			fmt.Printf("Pinned %s to version %s\n", modelName, truncateSHA(version))
			fmt.Println("Model will be skipped by 'check-updates' and 'update' commands.")

			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "version to pin (required, or 'current')")
	_ = cmd.MarkFlagRequired("version")

	return cmd
}

// NewUnpinCommand creates the model unpin command
func NewUnpinCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpin <model-name>",
		Short: "Unpin model version",
		Long: `Remove version pin from a model to allow updates.

Examples:
  ai-aas-cli model unpin llama-3-8b`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.DatabaseURL == "" {
				return fmt.Errorf("DATABASE_URL not configured")
			}

			db, err := sql.Open("postgres", cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer db.Close()

			// Check current pin status
			var currentPin sql.NullString
			err = db.QueryRowContext(ctx, `SELECT pinned_version FROM model_registry WHERE name = $1`, modelName).Scan(&currentPin)
			if err == sql.ErrNoRows {
				return fmt.Errorf("model not found: %s", modelName)
			} else if err != nil {
				return fmt.Errorf("query model: %w", err)
			}

			if !currentPin.Valid || currentPin.String == "" {
				fmt.Printf("Model %s is not pinned.\n", modelName)
				return nil
			}

			// Clear pinned_version
			_, err = db.ExecContext(ctx, `
				UPDATE model_registry
				SET pinned_version = NULL, updated_at = NOW()
				WHERE name = $1
			`, modelName)
			if err != nil {
				return fmt.Errorf("update model: %w", err)
			}

			fmt.Printf("Unpinned %s (was pinned to %s)\n", modelName, truncateSHA(currentPin.String))
			fmt.Println("Model will now be included in 'check-updates' and 'update' commands.")

			return nil
		},
	}

	return cmd
}

// truncateSHA shortens a SHA hash for display
func truncateSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
