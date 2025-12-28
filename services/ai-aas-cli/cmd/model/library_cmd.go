// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cli"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/inference"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewLibraryParentCommand creates the model library parent command
func NewLibraryParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Quick model enable/disable operations",
		Long: `Quick operations for enabling and disabling models in the library.

The library provides simplified commands for quickly enabling (deploying) and
disabling (undeploying) models that are already registered and cached.

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
  cached      Model weights stored in object storage      → model library enable
  deploying   InferenceService is starting up             → wait or check status
  ready       Model is serving inference requests         ✓ Ready to use
  failed      Deployment failed                           → model troubleshoot
  disabled    Model manually disabled/scaled to 0         → model library enable

Examples:
  # View library overview with next steps
  ai-aas model library list -e development

  # Quick enable a model
  ai-aas model library enable mistral-7b -e development

  # Quick disable a model
  ai-aas model library disable mistral-7b -e development

  # Swap models atomically
  ai-aas model library swap old-model new-model -e development

Use Cases:
  - Quickly enable/disable models without specifying resources
  - Swap models atomically for capacity management
  - View library status at a glance`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newLibraryListCommand())
	cmd.AddCommand(newLibraryEnableCommand())
	cmd.AddCommand(newLibraryDisableCommand())
	cmd.AddCommand(newLibrarySwapCommand())
	cmd.AddCommand(newLibraryHistoryCommand())
	cmd.AddCommand(newLibraryAliasCommand())

	return cmd
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
  ai-aas model library --help     View full lifecycle diagram
  ai-aas model registry list      View registry details
  ai-aas model library enable     Enable a model`,
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
				s3Client, err := getS3Client(ctx)
				if err == nil {
					// Note: manifest filename is .manifest.json (with leading dot)
					manifestPath := fmt.Sprintf("models/%s/main/.manifest.json", m.Name)
					exists, _ := s3Client.Exists(ctx, manifestPath)
					if exists {
						cached = "Yes"
						cachedColor = success
						isCached = true
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
							nextStep = muted.Sprintf("→ model library enable %s -e %s", m.Name, environment)
						}
					}
				} else if isCached {
					status = "cached"
					statusColor = info
					nextStep = muted.Sprintf("→ model library enable %s -e <env>", m.Name)
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

// newLibraryEnableCommand creates the library enable subcommand
func newLibraryEnableCommand() *cobra.Command {
	var (
		environment string
		gpuCount    int
		memoryGB    int
		wait        bool
	)

	cmd := &cobra.Command{
		Use:   "enable <model-name> [model-name...]",
		Short: "Quick enable (deploy) models",
		Long: `Enable one or more models by deploying them to the target environment.

This is a convenience command for quickly enabling models that are already
registered and cached. Uses default resource settings.

Examples:
  # Enable a single model
  ai-aas model library enable mistral-7b -e development

  # Enable multiple models
  ai-aas model library enable mistral-7b llama-3-8b -e development

  # Enable with custom resources
  ai-aas model library enable mistral-7b -e development --gpu-count 2 --memory 48

  # Enable and wait for ready
  ai-aas model library enable mistral-7b -e development --wait

See Also:
  ai-aas model library disable    Disable a model
  ai-aas model deploy create      Full deploy with all options`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
			if environment == "" {
				return fmt.Errorf("environment is required. Use -e <environment> or set a profile")
			}

			s3Bucket := viper.GetString("s3.bucket")

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			apiClient := cfg.NewAPIClient(adminEndpoint)
			regClient := registry.NewClient(apiClient)

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

			fmt.Printf("Enabling %d model(s) in %s\n\n", len(args), environment)

			var failed []string
			for _, modelName := range args {
				fmt.Printf("Enabling %s...\n", modelName)

				model, err := regClient.Get(ctx, modelName)
				if err != nil {
					fmt.Printf("  ERROR: %v\n", err)
					failed = append(failed, modelName)
					continue
				}

				storageURI := fmt.Sprintf("s3://%s/models/%s/main/", s3Bucket, modelName)
				isvcName := fmt.Sprintf("%s-%s", modelName, environment)

				existing, err := k8sClient.GetInferenceService(ctx, isvcName, environment)
				if err == nil && existing != nil {
					fmt.Printf("  Already enabled (ready: %v)\n", existing.Ready)
					continue
				}

				isvcCfg := kubernetes.InferenceServiceConfig{
					Name:        isvcName,
					Namespace:   environment,
					ModelName:   modelName,
					StorageURI:  storageURI,
					GPUCount:    gpuCount,
					MemoryGB:    memoryGB,
					MinReplicas: 1,
					MaxReplicas: 1,
					Environment: environment,
					Labels: map[string]string{
						"ai-aas.io/model":       modelName,
						"ai-aas.io/environment": environment,
					},
					Annotations: map[string]string{
						"ai-aas.io/hf-model-id": model.HFModelID,
					},
				}

				if err := k8sClient.CreateInferenceService(ctx, isvcCfg); err != nil {
					fmt.Printf("  ERROR: %v\n", err)
					failed = append(failed, modelName)
					continue
				}

				fmt.Printf("  Created InferenceService\n")

				if wait {
					fmt.Printf("  Waiting for ready...")
					waitOpts := kubernetes.WaitOptions{
						Timeout:      10 * time.Minute,
						PollInterval: 5 * time.Second,
					}
					if err := k8sClient.WaitForReady(ctx, isvcName, environment, waitOpts); err != nil {
						fmt.Printf(" TIMEOUT\n")
						failed = append(failed, modelName)
						continue
					}
					fmt.Printf(" READY\n")
				}
			}

			fmt.Println()
			if len(failed) == 0 {
				cli.PrintSuccess(fmt.Sprintf("Enabled %d model(s)", len(args)))
			} else {
				fmt.Printf("Enabled %d/%d model(s), %d failed\n", len(args)-len(failed), len(args), len(failed))
				return fmt.Errorf("some models failed to enable: %v", failed)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (uses profile if not specified)")
	cmd.Flags().IntVar(&gpuCount, "gpu-count", 1, "number of GPUs")
	cmd.Flags().IntVar(&memoryGB, "memory", 24, "memory allocation in GB")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for models to be ready")

	return cmd
}

// newLibraryDisableCommand creates the library disable subcommand
func newLibraryDisableCommand() *cobra.Command {
	var (
		environment string
		force       bool
		reason      string
	)

	cmd := &cobra.Command{
		Use:   "disable <model-name>",
		Short: "Quick disable (undeploy) model",
		Long: `Disable a model by removing its deployment while preserving the cache.

This allows quick re-enable without re-downloading the model files.

Examples:
  # Disable a model
  ai-aas model library disable mistral-7b -e development

  # Disable with reason for audit
  ai-aas model library disable mistral-7b -e development --reason "maintenance"

  # Force disable without confirmation
  ai-aas model library disable mistral-7b -e development --force

See Also:
  ai-aas model library enable     Re-enable a model
  ai-aas model deploy delete      Full delete with options`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// If no environment specified, try to get from active profile
			if environment == "" {
				if profile, _, err := config.GetActiveProfile(); err == nil && profile != nil && profile.Environment != "" {
					environment = profile.Environment
				}
			}
			if environment == "" {
				return fmt.Errorf("environment is required. Use -e <environment> or set a profile")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

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

			status, err := k8sClient.GetInferenceService(ctx, isvcName, environment)
			if err != nil {
				fmt.Printf("Model %s is not enabled in %s\n", modelName, environment)
				return nil
			}

			fmt.Printf("Disabling %s in %s\n", modelName, environment)
			fmt.Printf("  Ready: %v\n", status.Ready)
			if status.URL != "" {
				fmt.Printf("  URL: %s\n", status.URL)
			}
			if reason != "" {
				fmt.Printf("  Reason: %s\n", reason)
			}

			if !force {
				fmt.Print("\nConfirm disable? [y/N]: ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			if err := k8sClient.DeleteInferenceService(ctx, isvcName, environment); err != nil {
				return fmt.Errorf("delete inferenceservice: %w", err)
			}

			fmt.Println()
			cli.PrintSuccess(fmt.Sprintf("Disabled %s", modelName))
			cli.PrintNote("Model cache is preserved. Use 'ai-aas model library enable' to re-deploy.")

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (uses profile if not specified)")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for disabling")

	return cmd
}

// newLibrarySwapCommand creates the library swap subcommand
func newLibrarySwapCommand() *cobra.Command {
	var (
		environment string
		force       bool
		reason      string
	)

	cmd := &cobra.Command{
		Use:   "swap <disable-model> <enable-model>",
		Short: "Atomically swap models",
		Long: `Atomically disable one model and enable another.

Performs a graceful swap: the first model is disabled, then the second
is enabled. Useful for capacity management when you can't run both
models simultaneously.

Examples:
  # Swap models
  ai-aas model library swap old-model new-model -e production

  # Swap with reason for audit
  ai-aas model library swap old-model new-model -e production --reason "Upgrading"

  # Force swap without confirmation
  ai-aas model library swap old-model new-model -e production --force

See Also:
  ai-aas model library enable     Enable a model
  ai-aas model library disable    Disable a model
  ai-aas model library history    View swap history`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			disableModel := args[0]
			enableModel := args[1]

			// If no environment specified, try to get from active profile
			if environment == "" {
				if profile, _, err := config.GetActiveProfile(); err == nil && profile != nil && profile.Environment != "" {
					environment = profile.Environment
				}
			}
			if environment == "" {
				return fmt.Errorf("environment is required. Use -e <environment> or set a profile")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))
			s3Bucket := viper.GetString("s3.bucket")

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  environment,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			disableIsvc := fmt.Sprintf("%s-%s", disableModel, environment)
			enableIsvc := fmt.Sprintf("%s-%s", enableModel, environment)

			disableStatus, err := k8sClient.GetInferenceService(ctx, disableIsvc, environment)
			if err != nil {
				return fmt.Errorf("model %s is not deployed in %s", disableModel, environment)
			}

			fmt.Printf("Swapping models in %s:\n", environment)
			fmt.Printf("  Disable: %s (ready: %v)\n", disableModel, disableStatus.Ready)
			fmt.Printf("  Enable:  %s\n", enableModel)
			if reason != "" {
				fmt.Printf("  Reason:  %s\n", reason)
			}

			if !force {
				fmt.Print("\nConfirm swap? [y/N]: ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			fmt.Println()

			// Step 1: Disable
			fmt.Printf("Step 1: Disabling %s...\n", disableModel)
			if err := k8sClient.DeleteInferenceService(ctx, disableIsvc, environment); err != nil {
				return fmt.Errorf("disable %s: %w", disableModel, err)
			}

			apiClient, apiErr := getAPIClient(cfg)
			var regClient *registry.Client
			if apiErr != nil {
				fmt.Printf("  Warning: could not get API client to record history: %v\n", apiErr)
			} else if apiClient != nil {
				regClient = registry.NewClient(apiClient)
				if err := regClient.RecordState(ctx, disableModel, registry.RecordStateRequest{
					Environment: environment,
					Action:      "swapped_out",
					Reason:      reason,
				}); err != nil {
					fmt.Printf("  Warning: could not record history: %v\n", err)
				}
			}

			cli.PrintSuccess(fmt.Sprintf("Disabled %s", disableModel))

			fmt.Print("  Waiting for resources to be released...")
			if err := k8sClient.WaitForDelete(ctx, disableIsvc, environment, 30*time.Second); err != nil {
				fmt.Println(" FAILED")
				return fmt.Errorf("failed waiting for old model '%s' to be deleted: %w", disableModel, err)
			}
			fmt.Println(" done")

			// Step 2: Enable
			fmt.Printf("Step 2: Enabling %s...\n", enableModel)

			storageURI := fmt.Sprintf("s3://%s/models/%s/main/", s3Bucket, enableModel)

			isvcCfg := kubernetes.InferenceServiceConfig{
				Name:        enableIsvc,
				Namespace:   environment,
				ModelName:   enableModel,
				StorageURI:  storageURI,
				GPUCount:    1,
				MemoryGB:    24,
				MinReplicas: 1,
				MaxReplicas: 1,
				Environment: environment,
				Labels: map[string]string{
					"ai-aas.io/model":       enableModel,
					"ai-aas.io/environment": environment,
				},
			}

			if err := k8sClient.CreateInferenceService(ctx, isvcCfg); err != nil {
				fmt.Printf("  ERROR: %v\n", err)
				fmt.Printf("  Attempting rollback: re-enabling %s...\n", disableModel)

				rollbackCfg := isvcCfg
				rollbackCfg.Name = disableIsvc
				rollbackCfg.ModelName = disableModel
				rollbackCfg.StorageURI = fmt.Sprintf("s3://%s/models/%s/main/", s3Bucket, disableModel)
				rollbackCfg.Labels["ai-aas.io/model"] = disableModel

				if rollbackErr := k8sClient.CreateInferenceService(ctx, rollbackCfg); rollbackErr != nil {
					return fmt.Errorf("enable %s failed and rollback also failed: %v (rollback: %v)",
						enableModel, err, rollbackErr)
				}
				cli.PrintSuccess(fmt.Sprintf("Rollback successful, %s re-enabled", disableModel))
				return fmt.Errorf("enable %s: %w", enableModel, err)
			}

			if regClient != nil {
				if err := regClient.RecordState(ctx, enableModel, registry.RecordStateRequest{
					Environment: environment,
					Action:      "swapped_in",
					Reason:      reason,
				}); err != nil {
					fmt.Printf("  Warning: could not record history: %v\n", err)
				}
			}

			cli.PrintSuccess(fmt.Sprintf("Created InferenceService for %s", enableModel))

			fmt.Print("  Waiting for ready...")
			waitOpts := kubernetes.WaitOptions{
				Timeout:      10 * time.Minute,
				PollInterval: 5 * time.Second,
			}
			if err := k8sClient.WaitForReady(ctx, enableIsvc, environment, waitOpts); err != nil {
				fmt.Printf(" TIMEOUT\n")
				fmt.Printf("  Warning: %s did not become ready in time\n", enableModel)
			} else {
				fmt.Printf(" READY\n")
			}

			fmt.Println()
			cli.PrintSuccess(fmt.Sprintf("Swap complete: %s -> %s", disableModel, enableModel))

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (uses profile if not specified)")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for swap (for audit)")

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
