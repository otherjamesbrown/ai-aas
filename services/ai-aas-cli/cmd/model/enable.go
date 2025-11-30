// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewEnableCommand creates the model enable command
func NewEnableCommand() *cobra.Command {
	var (
		environment string
		gpuCount    int
		memoryGB    int
		wait        bool
	)

	cmd := &cobra.Command{
		Use:   "enable <model-name> [model-name...]",
		Short: "Enable (deploy) models from library",
		Long: `Enable one or more models by deploying them to the target environment.

This is a convenience wrapper around 'model deploy' for quickly enabling
models that are already registered and cached.

Examples:
  # Enable a single model
  ai-aas-cli model enable llama-3-8b -e development

  # Enable multiple models
  ai-aas-cli model enable llama-3-8b mistral-7b -e development

  # Enable with custom resources
  ai-aas-cli model enable llama-3-8b -e development --gpu-count 2 --memory 48`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Get configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			s3Bucket := viper.GetString("s3.bucket")

			// Use Admin API endpoint for registry operations
			adminEndpoint := cfg.AdminAPIEndpoint
			if adminEndpoint == "" {
				adminEndpoint = cfg.APIEndpoint // fallback for backward compatibility
			}

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			// Get API client
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// Get kubeconfig for environment
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

				// Get model from registry
				model, err := regClient.Get(ctx, modelName)
				if err != nil {
					fmt.Printf("  ERROR: %v\n", err)
					failed = append(failed, modelName)
					continue
				}

				// Build storage URI
				storageURI := fmt.Sprintf("s3://%s/models/%s/main/", s3Bucket, modelName)
				isvcName := fmt.Sprintf("%s-%s", modelName, environment)

				// Check if already deployed
				existing, err := k8sClient.GetInferenceService(ctx, isvcName, environment)
				if err == nil && existing != nil {
					fmt.Printf("  Already enabled (ready: %v)\n", existing.Ready)
					continue
				}

				// Create InferenceService
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
				fmt.Printf("Successfully enabled %d model(s)\n", len(args))
			} else {
				fmt.Printf("Enabled %d/%d model(s), %d failed\n", len(args)-len(failed), len(args), len(failed))
				return fmt.Errorf("some models failed to enable: %v", failed)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().IntVar(&gpuCount, "gpu-count", 1, "number of GPUs")
	cmd.Flags().IntVar(&memoryGB, "memory", 24, "memory allocation in GB")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for models to be ready")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// NewDisableCommand creates the model disable command
func NewDisableCommand() *cobra.Command {
	var (
		environment string
		force       bool
		reason      string
	)

	cmd := &cobra.Command{
		Use:   "disable <model-name>",
		Short: "Disable (undeploy) model, keeping cache",
		Long: `Disable a model by removing its deployment while preserving the cache.

This allows quick re-enable without re-downloading the model files.
The model remains in the registry and cache, only the deployment is removed.

Examples:
  # Disable a model
  ai-aas-cli model disable llama-3-8b -e development

  # Disable with reason
  ai-aas-cli model disable llama-3-8b -e development --reason "maintenance"

  # Force disable without confirmation
  ai-aas-cli model disable llama-3-8b -e development --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			// Get kubeconfig for environment
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

			// Check if deployed
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

			// Delete InferenceService
			if err := k8sClient.DeleteInferenceService(ctx, isvcName, environment); err != nil {
				return fmt.Errorf("delete inferenceservice: %w", err)
			}

			fmt.Printf("\nDisabled %s\n", modelName)
			fmt.Println("Note: Model cache is preserved. Use 'model enable' to re-deploy.")

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for disabling")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// NewLibraryCommand creates the model library command
func NewLibraryCommand() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "library",
		Short: "Show model library overview",
		Long: `Display an overview of all models with their status across environments.

Shows which models are registered, cached, and deployed in each environment.

Examples:
  # Show full library overview
  ai-aas-cli model library

  # Filter by environment
  ai-aas-cli model library -e development`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Use Admin API endpoint for registry operations
			adminEndpoint := cfg.AdminAPIEndpoint
			if adminEndpoint == "" {
				adminEndpoint = cfg.APIEndpoint // fallback for backward compatibility
			}

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			// Get API client
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			// List models
			models, err := regClient.List(ctx, registry.ListOptions{})
			if err != nil {
				return fmt.Errorf("list models: %w", err)
			}

			if len(models) == 0 {
				fmt.Println("No models in library.")
				return nil
			}

			// Print header
			fmt.Printf("%-20s %-10s %-12s\n", "MODEL", "CACHED", "STATUS")
			fmt.Println(fmt.Sprintf("%s", "────────────────────────────────────────────"))

			// Print each model
			for _, m := range models {
				cached := "No"
				// Check cache
				s3Client, err := getS3Client(ctx)
				if err == nil {
					manifestPath := fmt.Sprintf("models/%s/main/manifest.json", m.Name)
					exists, _ := s3Client.Exists(ctx, manifestPath)
					if exists {
						cached = "Yes"
					}
				}

				// Check deployment status
				status := "registered"
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
							if isvcStatus.Ready {
								status = "enabled (ready)"
							} else {
								status = "enabled (pending)"
							}
						} else if cached == "Yes" {
							status = "cached"
						}
					}
				} else if cached == "Yes" {
					status = "cached"
				}

				fmt.Printf("%-20s %-10s %-12s\n", m.Name, cached, status)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")

	return cmd
}
