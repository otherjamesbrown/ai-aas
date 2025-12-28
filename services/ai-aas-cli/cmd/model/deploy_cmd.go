// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cli"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/engines"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/storage"
)

// NewDeployParentCommand creates the model deploy parent command
func NewDeployParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Manage model deployments",
		Long: `Manage model deployments on Kubernetes using KServe InferenceService.

Models can be deployed directly from HuggingFace or from object storage cache
for faster startup. Deployments support auto-scaling, rolling restarts, and
resource configuration.

Examples:
  # Deploy a model
  ai-aas model deploy create mistral-7b -e development

  # Check deployment status
  ai-aas model deploy status mistral-7b -e development

  # Scale deployment
  ai-aas model deploy scale mistral-7b -e development --replicas 2

  # Remove deployment
  ai-aas model deploy delete mistral-7b -e development

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
	cmd.AddCommand(newDeployCreateCommand())
	cmd.AddCommand(newDeployDeleteCommand())
	cmd.AddCommand(newDeployRestartCommand())
	cmd.AddCommand(newDeployScaleCommand())
	cmd.AddCommand(newDeployStatusCommand())

	return cmd
}

// newDeployCreateCommand creates the deploy create subcommand
func newDeployCreateCommand() *cobra.Command {
	var (
		environment     string
		engineConfig    string
		gpuCount        int
		memoryGB        int
		minReplicas     int
		maxReplicas     int
		revision        string
		dryRun          bool
		skipValidation  bool
		wait            bool
		timeout         time.Duration
		trustRemoteCode bool
	)

	cmd := &cobra.Command{
		Use:   "create <model-name>",
		Short: "Deploy a model to Kubernetes",
		Long: `Deploy a model to Kubernetes using AIModel custom resource.

The AIModel CR is managed by the ai-model-operator, which creates the underlying
KServe InferenceService and syncs deployment status to the Admin API.

The model can be loaded from the object storage cache (faster) or directly
from HuggingFace. GPU and memory resources are allocated based on flags
or an engine configuration profile.

Examples:
  # Deploy to development
  ai-aas model deploy create mistral-7b -e development

  # Deploy with engine config profile
  ai-aas model deploy create mistral-7b -e development --engine-config vllm/default

  # Deploy with custom resources
  ai-aas model deploy create mistral-7b -e development --gpu-count 2 --memory 48

  # Deploy with auto-scaling
  ai-aas model deploy create mistral-7b -e production --min-replicas 2 --max-replicas 5

  # Deploy with trust remote code (for custom model architectures)
  ai-aas model deploy create mistral-7b -e development --trust-remote-code

  # Preview deployment YAML
  ai-aas model deploy create mistral-7b -e development --dry-run

  # Deploy and wait for ready
  ai-aas model deploy create mistral-7b -e development --wait

See Also:
  ai-aas model deploy status      Check deployment status
  ai-aas model deploy delete      Remove deployment
  ai-aas engine config list       List available engine configs
  ai-aas model troubleshoot logs  View startup logs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			s3Bucket := viper.GetString("s3.bucket")

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Get model from registry
			adminEndpoint := cfg.GetAdminEndpoint()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			regClient := registry.NewClient(apiClient)

			fmt.Printf("Looking up model: %s\n", modelName)
			model, err := regClient.Get(ctx, modelName)
			if err != nil {
				return fmt.Errorf("model not found: %s\n\nIs the model registered? Try:\n  ai-aas model registry add <hf-model-id> --name %s", modelName, modelName)
			}

			if revision == "" {
				revision = "main"
			}

			// Build S3 key for model storage
			s3Key := fmt.Sprintf("models/%s/%s/", modelName, revision)

			// Determine runtime and resource settings
			runtime := "vllm"
			effectiveGPU := gpuCount
			effectiveMemory := memoryGB
			effectiveMinReplicas := minReplicas
			effectiveMaxReplicas := maxReplicas
			var configName string

			// If engine config specified, fetch it and apply settings
			if engineConfig != "" {
				fmt.Printf("Loading engine config: %s\n", engineConfig)
				engClient := engines.NewClient(apiClient)
				ecfg, err := engClient.GetConfig(ctx, engineConfig)
				if err != nil {
					return fmt.Errorf("engine config not found: %s\n\nTo list available configs:\n  ai-aas engine config list", engineConfig)
				}

				// Use engine name directly (e.g., vllm)
				runtime = ecfg.EngineName
				configName = ecfg.Name

				// Apply config values unless explicitly overridden by flags
				if !cmd.Flags().Changed("gpu-count") {
					effectiveGPU = ecfg.GPUCount
				}
				if !cmd.Flags().Changed("memory") {
					effectiveMemory = ecfg.MemoryGB
				}
				if !cmd.Flags().Changed("min-replicas") {
					effectiveMinReplicas = ecfg.MinReplicas
				}
				if !cmd.Flags().Changed("max-replicas") {
					effectiveMaxReplicas = ecfg.MaxReplicas
				}

				fmt.Printf("Using config: %s (engine: %s, %d GPU, %dGB memory)\n",
					ecfg.Name, ecfg.EngineName, effectiveGPU, effectiveMemory)
			}

			// Create AIModel config
			aimodelName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			aimodelCfg := kubernetes.AIModelConfig{
				Name:            aimodelName,
				Namespace:       namespace,
				ModelName:       modelName,
				ModelID:         model.HFModelID,
				S3Bucket:        s3Bucket,
				S3Key:           s3Key,
				Runtime:         runtime,
				Enabled:         true,
				MinReplicas:     effectiveMinReplicas,
				MaxReplicas:     effectiveMaxReplicas,
				GPUCount:        effectiveGPU,
				MemoryGB:        effectiveMemory,
				TrustRemoteCode: trustRemoteCode,
				Environment:     environment,
				Labels: map[string]string{
					"ai-aas.io/model":       modelName,
					"ai-aas.io/revision":    revision,
					"ai-aas.io/environment": environment,
				},
				Annotations: map[string]string{
					"ai-aas.io/hf-model-id": model.HFModelID,
				},
			}

			// Add engine config label if specified
			if configName != "" {
				aimodelCfg.Labels["ai-aas.io/engine-config"] = configName
			}

			// Show configuration
			fmt.Printf("\nDeployment Configuration:\n")
			fmt.Printf("  Model: %s (%s)\n", modelName, model.HFModelID)
			fmt.Printf("  Environment: %s\n", environment)
			fmt.Printf("  AIModel CR: %s/%s\n", namespace, aimodelName)
			fmt.Printf("  S3 Location: s3://%s/%s\n", s3Bucket, s3Key)
			fmt.Printf("  Runtime: %s\n", runtime)
			if configName != "" {
				fmt.Printf("  Engine Config: %s\n", configName)
			}
			fmt.Printf("  Resources: %d GPU(s), %dGB memory\n", effectiveGPU, effectiveMemory)
			fmt.Printf("  Replicas: %d-%d\n", effectiveMinReplicas, effectiveMaxReplicas)
			if trustRemoteCode {
				fmt.Printf("  Trust Remote Code: enabled\n")
			}

			if dryRun {
				yamlBytes, err := generateAIModelYAML(aimodelCfg)
				if err != nil {
					return fmt.Errorf("generate YAML: %w", err)
				}
				fmt.Println("\n---")
				fmt.Println(string(yamlBytes))
				fmt.Println("---")
				fmt.Println("\n[DRY RUN] No resources created.")
				return nil
			}

			// Validate cache exists if using S3
			if !skipValidation && s3Bucket != "" {
				fmt.Println("\nValidating model cache...")
				s3Client, err := getS3Client(ctx)
				if err != nil {
					return err
				}
				manifestPath := fmt.Sprintf("models/%s/%s/%s", modelName, revision, storage.ManifestFileName)
				exists, err := s3Client.Exists(ctx, manifestPath)
				if err != nil {
					return fmt.Errorf("check cache: %w", err)
				}
				if !exists {
					return fmt.Errorf("model not cached\n\nTo cache the model first:\n  ai-aas model cache pull %s\n\nOr deploy directly from HuggingFace:\n  ai-aas model deploy create %s -e %s --skip-validation", modelName, modelName, environment)
				}
				fmt.Println("Cache validated.")
			} else if s3Bucket == "" {
				fmt.Println("\nUsing HuggingFace direct loading (no S3 cache configured)")
			}

			// Get kubeconfig
			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			if err := k8sClient.Ping(ctx); err != nil {
				return fmt.Errorf("cannot connect to cluster: %w", err)
			}

			// Check if already deployed (check for AIModel first, then InferenceService)
			existing, err := k8sClient.GetAIModel(ctx, aimodelName, namespace)
			if err == nil && existing != nil {
				fmt.Printf("\nAIModel %s already exists.\n", aimodelName)
				fmt.Printf("  Phase: %s\n", existing.Phase)
				if existing.InferenceEndpoint != "" {
					fmt.Printf("  Endpoint: %s\n", existing.InferenceEndpoint)
				}
				fmt.Println("\nTo update, delete first:")
				fmt.Printf("  ai-aas model deploy delete %s -e %s\n", modelName, environment)
				fmt.Println("\nOr restart:")
				fmt.Printf("  ai-aas model deploy restart %s -e %s\n", modelName, environment)
				return nil
			}

			// Create AIModel CR
			fmt.Println("\nCreating AIModel custom resource...")
			fmt.Println("The ai-model-operator will create the InferenceService and sync status to Admin API.")
			if err := k8sClient.CreateAIModel(ctx, aimodelCfg); err != nil {
				return fmt.Errorf("create aimodel: %w", err)
			}

			fmt.Printf("Created AIModel: %s/%s\n", namespace, aimodelName)

			// Wait for ready if requested
			if wait {
				fmt.Println("\nWaiting for deployment to be ready...")
				fmt.Println("(The operator will download the model, create InferenceService, and wait for pods)")
				spinner := output.NewSpinner("Deploying")
				spinner.Start()

				waitOpts := kubernetes.WaitOptions{
					Timeout:      timeout,
					PollInterval: 5 * time.Second,
				}

				err := k8sClient.WaitForAIModelReady(ctx, aimodelName, namespace, waitOpts)
				spinner.Stop()

				if err != nil {
					return fmt.Errorf("deployment failed: %w\n\nTo check logs:\n  ai-aas model troubleshoot logs %s -e %s", err, modelName, environment)
				}

				status, _ := k8sClient.GetAIModel(ctx, aimodelName, namespace)
				if status != nil {
					fmt.Printf("\nDeployment ready!\n")
					fmt.Printf("  Phase: %s\n", status.Phase)
					if status.InferenceEndpoint != "" {
						fmt.Printf("  Endpoint: %s\n", status.InferenceEndpoint)
					}
					fmt.Printf("  Ready Replicas: %d\n", status.ReadyReplicas)
				}
			}

			fmt.Println()
			cli.PrintDeploymentStarted(modelName, environment)

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().StringVar(&engineConfig, "engine-config", "", "engine configuration profile (e.g., vllm/default)")
	cmd.Flags().IntVar(&gpuCount, "gpu-count", 1, "number of GPUs")
	cmd.Flags().IntVar(&memoryGB, "memory", 24, "memory allocation in GB")
	cmd.Flags().IntVar(&minReplicas, "min-replicas", 1, "minimum replica count")
	cmd.Flags().IntVar(&maxReplicas, "max-replicas", 1, "maximum replica count")
	cmd.Flags().StringVarP(&revision, "revision", "r", "main", "model revision/version")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "output YAML without applying")
	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "skip pre-deploy validation")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for deployment to be ready")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "timeout when waiting")
	cmd.Flags().BoolVar(&trustRemoteCode, "trust-remote-code", false, "allow execution of custom model code (required for some models)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newDeployDeleteCommand creates the deploy delete subcommand
func newDeployDeleteCommand() *cobra.Command {
	var (
		environment string
		force       bool
		wait        bool
	)

	cmd := &cobra.Command{
		Use:   "delete <model-name>",
		Short: "Remove a model deployment",
		Long: `Remove a model deployment from Kubernetes.

This removes the AIModel custom resource, which triggers the operator to
clean up the InferenceService. Model cache files are preserved.

Examples:
  # Remove deployment
  ai-aas model deploy delete mistral-7b -e development

  # Force delete without confirmation
  ai-aas model deploy delete mistral-7b -e development --force

  # Delete and wait for cleanup
  ai-aas model deploy delete mistral-7b -e development --wait

See Also:
  ai-aas model cache delete       Delete cached files
  ai-aas model registry remove    Remove from registry`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			aimodelName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			// Try to get AIModel first
			status, err := k8sClient.GetAIModel(ctx, aimodelName, namespace)
			if err != nil {
				// Fall back to checking for InferenceService (legacy deployments)
				isvcStatus, isvcErr := k8sClient.GetInferenceService(ctx, aimodelName, namespace)
				if isvcErr != nil {
					fmt.Printf("Deployment %s not found in %s\n", aimodelName, namespace)
					return nil
				}
				// Legacy InferenceService exists, delete it directly
				fmt.Printf("Removing legacy InferenceService: %s/%s\n", namespace, aimodelName)
				fmt.Printf("  Ready: %v\n", isvcStatus.Ready)
				if isvcStatus.URL != "" {
					fmt.Printf("  URL: %s\n", isvcStatus.URL)
				}

				if !force {
					fmt.Print("\nAre you sure? [y/N]: ")
					var response string
					fmt.Scanln(&response)
					if strings.ToLower(response) != "y" {
						fmt.Println("Cancelled.")
						return nil
					}
				}

				fmt.Println("\nDeleting InferenceService...")
				if err := k8sClient.DeleteInferenceService(ctx, aimodelName, namespace); err != nil {
					return fmt.Errorf("delete inferenceservice: %w", err)
				}

				if wait {
					fmt.Println("Waiting for deletion...")
					if err := k8sClient.WaitForDelete(ctx, aimodelName, namespace, 5*time.Minute); err != nil {
						return fmt.Errorf("wait for delete: %w", err)
					}
				}

				fmt.Println()
				cli.PrintDeploymentDeleted(modelName, environment)
				return nil
			}

			// AIModel found, delete it (operator will clean up InferenceService)
			fmt.Printf("Removing AIModel: %s/%s\n", namespace, aimodelName)
			fmt.Printf("  Phase: %s\n", status.Phase)
			if status.InferenceEndpoint != "" {
				fmt.Printf("  Endpoint: %s\n", status.InferenceEndpoint)
			}
			fmt.Printf("  Ready Replicas: %d\n", status.ReadyReplicas)

			if !force {
				fmt.Print("\nAre you sure? [y/N]: ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			fmt.Println("\nDeleting AIModel...")
			fmt.Println("The operator will automatically clean up the InferenceService.")
			if err := k8sClient.DeleteAIModel(ctx, aimodelName, namespace); err != nil {
				return fmt.Errorf("delete aimodel: %w", err)
			}

			if wait {
				fmt.Println("Waiting for deletion...")
				// Give the operator time to clean up
				time.Sleep(5 * time.Second)
				fmt.Println("AIModel deleted. The operator will continue cleaning up resources.")
			}

			fmt.Println()
			cli.PrintDeploymentDeleted(modelName, environment)
			fmt.Println("Note: Model cache is preserved. Use 'model cache delete' to remove cache.")

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for deletion to complete")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newDeployRestartCommand creates the deploy restart subcommand
func newDeployRestartCommand() *cobra.Command {
	var (
		environment string
		wait        bool
	)

	cmd := &cobra.Command{
		Use:   "restart <model-name>",
		Short: "Rolling restart of model deployment",
		Long: `Perform a rolling restart of a model deployment.

This triggers a new rollout of the InferenceService pods without
changing the configuration. Useful for picking up new model weights
or recovering from issues.

Examples:
  # Restart deployment
  ai-aas model deploy restart mistral-7b -e development

  # Restart and wait for ready
  ai-aas model deploy restart mistral-7b -e development --wait

See Also:
  ai-aas model deploy status      Check status
  ai-aas model troubleshoot logs  View logs during restart`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			_, err = k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("deployment not found: %s in %s\n\nTo deploy:\n  ai-aas model deploy create %s -e %s", isvcName, namespace, modelName, environment)
			}

			fmt.Printf("Restarting: %s/%s\n", namespace, isvcName)
			fmt.Println("Triggering rolling restart...")

			if err := k8sClient.RestartInferenceService(ctx, isvcName, namespace); err != nil {
				return fmt.Errorf("restart inferenceservice: %w", err)
			}

			fmt.Println("Rolling restart triggered.")

			if wait {
				fmt.Println("\nWaiting for pods to be ready...")
				waitOpts := kubernetes.WaitOptions{
					Timeout:      10 * time.Minute,
					PollInterval: 5 * time.Second,
				}
				if err := k8sClient.WaitForPodReady(ctx, isvcName, namespace, waitOpts); err != nil {
					return fmt.Errorf("wait for ready: %w", err)
				}
				fmt.Println("All pods ready.")
			}

			cli.PrintSuccess(fmt.Sprintf("Restart initiated for %s in %s", modelName, environment))

			cli.PrintNextSteps([]cli.NextStep{
				{
					Command:     fmt.Sprintf("ai-aas model deploy status %s -e %s", modelName, environment),
					Description: "Check status",
				},
				{
					Command:     fmt.Sprintf("ai-aas model troubleshoot logs %s -e %s", modelName, environment),
					Description: "Watch logs",
				},
			})

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for restart to complete")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newDeployScaleCommand creates the deploy scale subcommand
func newDeployScaleCommand() *cobra.Command {
	var (
		environment string
		replicas    string
	)

	cmd := &cobra.Command{
		Use:   "scale <model-name>",
		Short: "Scale model replicas",
		Long: `Scale the replica count of a model deployment.

Specify a fixed replica count for the deployment.

Examples:
  # Scale to 3 replicas
  ai-aas model deploy scale mistral-7b -e development --replicas 3

  # Scale down to 1 replica
  ai-aas model deploy scale mistral-7b -e development --replicas 1

Note: Auto-scaling ranges (e.g., 2-5) are not yet implemented.

See Also:
  ai-aas model deploy status      Check current status
  ai-aas model deploy restart     Restart deployment`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			replicaCount, _, err := parseReplicaString(replicas)
			if err != nil {
				return fmt.Errorf("invalid replicas: %w", err)
			}

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			status, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("deployment not found: %s in %s", isvcName, namespace)
			}

			fmt.Printf("Scaling: %s/%s\n", namespace, isvcName)
			fmt.Printf("  Current replicas: %d (ready: %d)\n", status.Replicas, status.ReadyReplicas)
			fmt.Printf("  Target: %d replicas\n", replicaCount)

			if err := k8sClient.ScaleInferenceService(ctx, isvcName, namespace, replicaCount); err != nil {
				return fmt.Errorf("scale inferenceservice: %w", err)
			}

			cli.PrintSuccess(fmt.Sprintf("Scaled %s to %d replicas", modelName, replicaCount))

			cli.PrintNextSteps([]cli.NextStep{
				{
					Command:     fmt.Sprintf("ai-aas model deploy status %s -e %s", modelName, environment),
					Description: "Check scaling progress",
				},
			})

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().StringVar(&replicas, "replicas", "1", "replica count or range (e.g., 1-3)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newDeployStatusCommand creates the deploy status subcommand
func newDeployStatusCommand() *cobra.Command {
	var (
		environment string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "status <model-name>",
		Short: "Show deployment status",
		Long: `Show the current status of a model deployment.

Displays InferenceService status, replicas, URL, and conditions.

Examples:
  # Check deployment status
  ai-aas model deploy status mistral-7b -e development

  # Output as JSON
  ai-aas model deploy status mistral-7b -e development --format json

See Also:
  ai-aas model troubleshoot describe  Detailed deployment info
  ai-aas model troubleshoot logs      View pod logs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			status, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("deployment not found: %s in %s\n\nTo deploy:\n  ai-aas model deploy create %s -e %s", isvcName, namespace, modelName, environment)
			}

			if format == "json" {
				return output.PrintJSON(status, true)
			}

			fmt.Printf("Deployment: %s/%s\n", namespace, isvcName)
			fmt.Println("────────────────────────────────────────────────────")

			// Status overview
			readyIcon := "[-]"
			if status.Ready {
				readyIcon = "[+]"
			}

			fmt.Printf("\nStatus\n")
			fmt.Printf("  Ready:         %s %v\n", readyIcon, status.Ready)
			fmt.Printf("  Replicas:      %d/%d ready\n", status.ReadyReplicas, status.Replicas)
			if status.URL != "" {
				fmt.Printf("  URL:           %s\n", status.URL)
			}

			// Conditions
			if len(status.Conditions) > 0 {
				fmt.Printf("\nConditions\n")
				for _, cond := range status.Conditions {
					icon := "[-]"
					if cond.Status == "True" {
						icon = "[+]"
					} else if cond.Status == "Unknown" {
						icon = "[~]"
					}
					fmt.Printf("  %s %-20s %s\n", icon, cond.Type, cond.Reason)
					if cond.Message != "" {
						fmt.Printf("     %s\n", cond.Message)
					}
				}
			}

			// Suggest next action based on status
			fmt.Println()
			if status.Ready {
				cli.PrintNextSteps([]cli.NextStep{
					{
						Command:     fmt.Sprintf("ai-aas model troubleshoot test %s -e %s", modelName, environment),
						Description: "Test inference",
					},
				})
			} else {
				cli.PrintNextSteps([]cli.NextStep{
					{
						Command:     fmt.Sprintf("ai-aas model troubleshoot logs %s -e %s", modelName, environment),
						Description: "Check startup logs",
					},
					{
						Command:     fmt.Sprintf("ai-aas model troubleshoot events %s -e %s", modelName, environment),
						Description: "Check events for errors",
					},
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// parseReplicaString parses a replica string like "3" or "2-5"
func parseReplicaString(s string) (min, max int, err error) {
	parts := strings.Split(s, "-")
	if len(parts) == 1 {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid number: %s", parts[0])
		}
		return n, n, nil
	}
	if len(parts) == 2 {
		min, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid min: %s", parts[0])
		}
		max, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid max: %s", parts[1])
		}
		if min > max {
			return 0, 0, fmt.Errorf("min (%d) cannot be greater than max (%d)", min, max)
		}
		return min, max, nil
	}
	return 0, 0, fmt.Errorf("invalid format: %s (expected N or N-M)", s)
}

// Note: generateInferenceServiceYAML is defined in deploy.go
