// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/inference"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewLibraryParentCommand creates the model library parent command
func NewLibraryParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "View model library status and history",
		Long: `View model library status, deployment history, and manage aliases.

The library provides read-only views of model status across environments
and audit logs of deployment operations.

For deployment operations, use the top-level commands:
  ai-aas-cli model enable <model>       Deploy models
  ai-aas-cli model disable <model>      Undeploy models
  ai-aas-cli model swap <old> <new>     Swap models atomically

MODEL LIFECYCLE
───────────────
Models progress through these stages:

  ┌────────────┐     ┌────────────┐     ┌────────────┐     ┌────────────┐
  │ registered │────▶│   cached   │────▶│ deploying  │────▶│   ready    │
  └────────────┘     └────────────┘     └────────────┘     └────────────┘
                                               │                  │
                                               ▼                  ▼
                                        ┌────────────┐     ┌────────────┐
                                        │   failed   │     │  disabled  │
                                        └────────────┘     └────────────┘

STATUS REFERENCE
────────────────
  registered  Model is in registry, weights not cached    → model cache pull
  cached      Model weights stored in object storage      → model enable
  deploying   InferenceService is starting up             → wait or check status
  ready       Model is serving inference requests         ✓ Ready to use
  failed      Deployment failed                           → model troubleshoot
  disabled    Model manually disabled/scaled to 0         → model enable

Examples:
  # View library overview with next steps
  ai-aas-cli model library list -e development

  # View deployment history
  ai-aas-cli model library history mistral-7b

  # Manage model aliases
  ai-aas-cli model library alias set prod-llm llama-3-8b

Use Cases:
  - View model status across environments
  - Audit deployment history
  - Manage model aliases for stable references`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands - view-only operations
	cmd.AddCommand(newLibraryListCommand())
	cmd.AddCommand(newLibraryHistoryCommand())
	cmd.AddCommand(newLibraryAliasCommand())

	return cmd
}

// getCachedRevision retrieves the HF revision for a cached model
func getCachedRevision(ctx context.Context, regClient *registry.Client, modelName string) (string, error) {
	cacheEntries, err := regClient.GetCache(ctx, modelName)
	if err != nil {
		return "", fmt.Errorf("get cache: %w", err)
	}

	// Find most recent ready cache entry
	for _, entry := range cacheEntries {
		if entry.Status == "ready" && entry.HFRevision != "" {
			return entry.HFRevision, nil
		}
	}

	return "", fmt.Errorf("model not cached or no ready cache entry. Run: ai-aas-cli model cache pull %s", modelName)
}

// newLibraryListCommand creates the library list subcommand
func newLibraryListCommand() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show model library overview",
		Long: `Display an overview of all models with their status and next steps.

Shows which models are registered, cached, deployed, and available to users.

STATUS VALUES
─────────────
  registered  Model is in registry, weights not cached
  cached      Model weights stored in object storage
  deploying   InferenceService is starting up
  ready       Model is serving inference requests
  failed      Deployment encountered an error
  disabled    Model manually disabled/scaled to 0

AVAILABLE COLUMN
────────────────
  Yes   Model is routed through API Router (users can access it)
  No    Model is deployed but not routed (not in API Router config)
  -     Model is not deployed

Examples:
  # Show full library overview
  ai-aas model library list

  # Filter by environment (shows deployment status)
  ai-aas model library list -e development

See Also:
  ai-aas-cli model library --help     View full lifecycle diagram
  ai-aas-cli model registry list      View registry details
  ai-aas-cli model enable             Enable a model`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// If no environment specified, try to get from active profile
			if environment == "" {
				if profile, _, err := config.GetActiveProfile(); err == nil && profile != nil && profile.Environment != "" {
					environment = profile.Environment
				}
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			apiClient := cfg.NewAPIClient(adminEndpoint)
			regClient := registry.NewClient(apiClient)

			models, err := regClient.List(ctx, registry.ListOptions{})
			if err != nil {
				return fmt.Errorf("list models: %w", err)
			}

			if len(models) == 0 {
				fmt.Println("No models in library.")
				fmt.Println()
				fmt.Println("To add a model:")
				fmt.Println("  ai-aas model registry add <hf-model-id> --name <name>")
				return nil
			}

			// Get available models from API Router (inference endpoint)
			availableModels := make(map[string]bool)
			inferenceEndpoint := cfg.InferenceEndpoint
			if inferenceEndpoint != "" {
				inferenceClient := inference.NewClient(inferenceEndpoint, cfg.APIKey)
				availableList, err := inferenceClient.ListModels(ctx)
				if err == nil {
					for _, m := range availableList.Data {
						availableModels[m.ID] = true
					}
				}
			}

			// Color definitions
			header := color.New(color.FgWhite, color.Bold)
			muted := color.New(color.FgHiBlack)
			success := color.New(color.FgGreen, color.Bold)
			warning := color.New(color.FgYellow)
			errorColor := color.New(color.FgRed, color.Bold)
			info := color.New(color.FgCyan)

			// Print environment header if set
			if environment != "" {
				info.Printf("Environment: %s\n", environment)
				fmt.Println()
			}

			// Print header
			header.Printf("%-20s %-10s %-12s %-10s %s\n", "MODEL", "CACHED", "STATUS", "AVAILABLE", "NEXT STEP")
			muted.Println("─────────────────────────────────────────────────────────────────────────────────────────────")

			for _, m := range models {
				cached := "No"
				cachedColor := muted
				isCached := false

				// Try to get cached revision
				revision, revErr := getCachedRevision(ctx, regClient, m.Name)
				if revErr == nil && revision != "" {
					// Model is cached with a valid revision
					s3Client, err := getS3Client(ctx)
					if err == nil {
						// Note: manifest filename is .manifest.json (with leading dot)
						manifestPath := fmt.Sprintf("models/%s/%s/.manifest.json", m.Name, revision)
						exists, _ := s3Client.Exists(ctx, manifestPath)
						if exists {
							cached = "Yes"
							cachedColor = success
							isCached = true
						}
					}
				}

				status := "registered"
				statusColor := warning
				nextStep := muted.Sprintf("→ model cache pull %s", m.Name)
				isDeployed := false

				if environment != "" {
					kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
					kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

					k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
						Kubeconfig: kubeconfig,
						Context:    kubecontext,
						Namespace:  environment,
					})
					if err == nil {
						isvcName := fmt.Sprintf("%s-%s", m.Name, environment)
						isvcStatus, err := k8sClient.GetInferenceService(ctx, isvcName, environment)
						if err == nil {
							isDeployed = true
							if isvcStatus.Ready {
								status = "ready"
								statusColor = success
								nextStep = success.Sprint("✓ Serving inference")
							} else {
								// Check conditions for failure
								failed := false
								for _, cond := range isvcStatus.Conditions {
									if cond.Type == "Ready" && cond.Status == "False" && cond.Reason == "RevisionFailed" {
										failed = true
										break
									}
								}
								if failed {
									status = "failed"
									statusColor = errorColor
									nextStep = errorColor.Sprintf("→ model troubleshoot %s", m.Name)
								} else {
									status = "deploying"
									statusColor = info
									nextStep = info.Sprint("⏳ Waiting for ready...")
								}
							}
						} else if isCached {
							status = "cached"
							statusColor = info
							nextStep = muted.Sprintf("→ model enable %s -e %s", m.Name, environment)
						}
					}
				} else if isCached {
					status = "cached"
					statusColor = info
					nextStep = muted.Sprintf("→ model enable %s -e <env>", m.Name)
				}

				// Check if available via API Router
				// Models can be registered by HF model ID (org/model) or by short name
				available := "-"
				availableColor := muted
				if isDeployed {
					// Check both the HF model ID and the short name
					isAvailable := availableModels[m.HFModelID] || availableModels[m.Name]
					if isAvailable {
						available = "Yes"
						availableColor = success
					} else {
						available = "No"
						availableColor = warning
						// Update next step to indicate routing needed
						if status == "ready" {
							nextStep = warning.Sprint("⚠ Add to API Router config")
						}
					}
				}

				// Print row with colored fields
				fmt.Printf("%-20s ", m.Name)
				cachedColor.Printf("%-10s ", cached)
				statusColor.Printf("%-12s ", status)
				availableColor.Printf("%-10s ", available)
				fmt.Println(nextStep)
			}

			fmt.Printf("\nTotal: %d model(s)\n", len(models))
			if environment == "" {
				muted.Println("\nTip: Use -e <environment> or set a profile with 'ai-aas-cli profile use <profile>'")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")

	return cmd
}

// newLibraryHistoryCommand creates the library history subcommand
func newLibraryHistoryCommand() *cobra.Command {
	var (
		environment string
		limit       int
		formatFlag  string
	)

	cmd := &cobra.Command{
		Use:   "history <model-name>",
		Short: "Show enable/disable history",
		Long: `Display the audit log of enable/disable events for a model.

Shows when the model was enabled, disabled, or swapped.

Examples:
  # Show history for a model
  ai-aas model library history mistral-7b

  # Filter by environment
  ai-aas model library history mistral-7b -e production

  # Limit entries
  ai-aas model library history mistral-7b --limit 10

See Also:
  ai-aas model library swap       Swap models
  ai-aas model library enable     Enable a model`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

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

			entries, err := regClient.GetHistory(ctx, modelName, environment, limit)
			if err != nil {
				return fmt.Errorf("get history: %w", err)
			}

			if formatFlag == "json" {
				return output.PrintJSON(entries)
			}

			fmt.Printf("History for %s", modelName)
			if environment != "" {
				fmt.Printf(" (environment: %s)", environment)
			}
			fmt.Println(":")
			fmt.Println()

			if len(entries) == 0 {
				fmt.Println("No history found.")
				return nil
			}

			fmt.Printf("%-20s %-12s %-20s %-30s\n", "TIMESTAMP", "ACTION", "PERFORMED BY", "REASON")
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")

			for _, e := range entries {
				reason := e.Reason
				if len(reason) > 30 {
					reason = reason[:27] + "..."
				}
				if reason == "" {
					reason = "-"
				}
				performedBy := e.PerformedBy
				if performedBy == "" {
					performedBy = "system"
				}
				if len(performedBy) > 20 {
					performedBy = performedBy[:17] + "..."
				}

				fmt.Printf("%-20s %-12s %-20s %-30s\n",
					e.ExecutedAt.Format("2006-01-02 15:04"),
					e.Action,
					performedBy,
					reason)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().IntVar(&limit, "limit", 20, "max entries to show")
	cmd.Flags().StringVar(&formatFlag, "format", "table", "output format: table, json")

	return cmd
}

// newLibraryAliasCommand creates the library alias subcommand (wrapper)
func newLibraryAliasCommand() *cobra.Command {
	// Reuse the existing alias command structure
	return NewAliasCommand()
}
