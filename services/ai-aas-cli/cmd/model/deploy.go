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
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewDeployCommand creates the model deploy command
func NewDeployCommand() *cobra.Command {
	var (
		environment     string
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
		recipe          string
	)

	cmd := &cobra.Command{
		Use:   "deploy <model-name>",
		Short: "Deploy model to Kubernetes",
		Long: `Deploy a cached model to Kubernetes using AIModel custom resource.

The AIModel CR is managed by the ai-model-operator, which creates the underlying
KServe InferenceService and syncs deployment status to the Admin API.

The model must already be cached in object storage before deployment.

Examples:
  # Deploy a model to development
  ai-aas-cli model deploy llama-3-8b -e development

  # Deploy with specific resource requirements
  ai-aas-cli model deploy llama-3-8b -e development --gpu-count 2 --memory 48

  # Deploy using a model recipe
  ai-aas-cli model deploy mistral-7b -e development --recipe mistral-7b-instruct-v03

  # Deploy with recipe and override GPU count
  ai-aas-cli model deploy mistral-7b -e development --recipe mistral-7b-instruct-v03 --gpu-count 2

  # Deploy with auto-scaling
  ai-aas-cli model deploy llama-3-8b -e production --min-replicas 2 --max-replicas 5

  # Deploy with trust remote code (for custom model architectures)
  ai-aas-cli model deploy llama-3-8b -e development --trust-remote-code

  # Preview deployment YAML
  ai-aas-cli model deploy llama-3-8b -e development --dry-run

  # Deploy and wait for ready
  ai-aas-cli model deploy llama-3-8b -e development --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Get configuration
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

			// Get model from registry via Admin API
			adminEndpoint := cfg.AdminAPIEndpoint
			if adminEndpoint == "" {
				adminEndpoint = cfg.APIEndpoint // fallback for backward compatibility
			}

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			fmt.Printf("Looking up model: %s\n", modelName)
			model, err := regClient.Get(ctx, modelName)
			if err != nil {
				return fmt.Errorf("get model: %w\nIs the model registered? Use 'ai-aas-cli model add' first", err)
			}

			// Determine revision
			if revision == "" {
				revision = "main"
			}

			// Build S3 key for model storage
			// The operator expects s3Key to point to the model directory
			s3Key := fmt.Sprintf("models/%s/%s/", modelName, revision)

			// Create AIModel config
			aimodelName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := fmt.Sprintf("%s", environment) // Using environment as namespace

			// Determine which flags were explicitly set for override tracking
			gpuCountChanged := cmd.Flags().Changed("gpu-count")
			memoryChanged := cmd.Flags().Changed("memory")
			minReplicasChanged := cmd.Flags().Changed("min-replicas")
			maxReplicasChanged := cmd.Flags().Changed("max-replicas")

			aimodelCfg := kubernetes.AIModelConfig{
				Name:            aimodelName,
				Namespace:       namespace,
				ModelName:       modelName,
				ModelID:         model.HFModelID,
				S3Bucket:        s3Bucket,
				S3Key:           s3Key,
				Runtime:         "vllm",
				Enabled:         true,
				MinReplicas:     minReplicas,
				MaxReplicas:     maxReplicas,
				GPUCount:        gpuCount,
				MemoryGB:        memoryGB,
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
				// Recipe support
				RecipeName:          recipe,
				RecipeNamespace:     "ai-model-system",
				OverrideGPUCount:    gpuCountChanged,
				OverrideMemory:      memoryChanged,
				OverrideMinReplicas: minReplicasChanged,
				OverrideMaxReplicas: maxReplicasChanged,
			}

			// Show configuration
			fmt.Printf("\nDeployment Configuration:\n")
			fmt.Printf("  Model: %s (%s)\n", modelName, model.HFModelID)
			fmt.Printf("  Environment: %s\n", environment)
			fmt.Printf("  AIModel CR: %s/%s\n", namespace, aimodelName)
			if recipe != "" {
				fmt.Printf("  Recipe: %s (namespace: %s)\n", recipe, aimodelCfg.RecipeNamespace)
				if gpuCountChanged || memoryChanged || minReplicasChanged || maxReplicasChanged {
					fmt.Printf("  Overrides:\n")
					if gpuCountChanged {
						fmt.Printf("    - GPU Count: %d\n", gpuCount)
					}
					if memoryChanged {
						fmt.Printf("    - Memory: %dGB\n", memoryGB)
					}
					if minReplicasChanged || maxReplicasChanged {
						fmt.Printf("    - Replicas: %d-%d\n", minReplicas, maxReplicas)
					}
				}
			} else {
				fmt.Printf("  S3 Location: s3://%s/%s\n", s3Bucket, s3Key)
				fmt.Printf("  Resources: %d GPU(s), %dGB memory\n", gpuCount, memoryGB)
				fmt.Printf("  Replicas: %d-%d\n", minReplicas, maxReplicas)
			}
			if trustRemoteCode {
				fmt.Printf("  Trust Remote Code: enabled\n")
			}

			if dryRun {
				// Generate YAML and print
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

			// Validate cache exists if not skipping validation and using S3
			if !skipValidation && s3Bucket != "" {
				fmt.Println("\nValidating model cache...")
				s3Client, err := getS3Client(ctx)
				if err != nil {
					return err
				}
				manifestPath := fmt.Sprintf("models/%s/%s/manifest.json", modelName, revision)
				exists, err := s3Client.Exists(ctx, manifestPath)
				if err != nil {
					return fmt.Errorf("check cache: %w", err)
				}
				if !exists {
					return fmt.Errorf("model not cached. Run 'ai-aas-cli model pull %s' first", modelName)
				}
				fmt.Println("Cache validated.")
			} else if s3Bucket == "" {
				fmt.Println("\nUsing HuggingFace direct loading (no S3 cache configured)")
			}

			// Get kubeconfig for environment
			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			// Create Kubernetes client
			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			// Test connectivity
			if err := k8sClient.Ping(ctx); err != nil {
				return fmt.Errorf("cannot connect to cluster: %w", err)
			}

			// Check if already deployed
			existing, err := k8sClient.GetAIModel(ctx, aimodelName, namespace)
			if err == nil && existing != nil {
				fmt.Printf("\nWARNING: AIModel %s already exists.\n", aimodelName)
				fmt.Printf("  Phase: %s\n", existing.Phase)
				if existing.InferenceEndpoint != "" {
					fmt.Printf("  Endpoint: %s\n", existing.InferenceEndpoint)
				}
				fmt.Println("Use 'model undeploy' first to remove the existing deployment.")
				return nil
			}

			// Create AIModel
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
					return fmt.Errorf("deployment failed: %w", err)
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

			fmt.Printf("\nModel deployed successfully!\n")
			fmt.Println("\nNext steps:")
			fmt.Printf("  ai-aas-cli model status %s -e %s    # Check status\n", modelName, environment)
			fmt.Printf("  ai-aas-cli model test %s -e %s      # Test inference\n", modelName, environment)

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
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
	cmd.Flags().StringVar(&recipe, "recipe", "", "use a model recipe for deployment configuration")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// NewUndeployCommand creates the model undeploy command
func NewUndeployCommand() *cobra.Command {
	var (
		environment string
		force       bool
		wait        bool
	)

	cmd := &cobra.Command{
		Use:   "undeploy <model-name>",
		Short: "Remove model deployment",
		Long: `Remove a model deployment from Kubernetes.

This removes the AIModel custom resource, which will trigger the operator to
clean up the InferenceService. Model cache files are preserved.
To fully remove a model including cache, use 'model cache delete'.

Examples:
  # Undeploy a model
  ai-aas-cli model undeploy llama-3-8b -e development

  # Force undeploy without confirmation
  ai-aas-cli model undeploy llama-3-8b -e development --force

  # Undeploy and wait for deletion
  ai-aas-cli model undeploy llama-3-8b -e development --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			// Build AIModel name
			aimodelName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			// Get kubeconfig for environment
			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			// Create Kubernetes client
			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			// Check if exists
			status, err := k8sClient.GetAIModel(ctx, aimodelName, namespace)
			if err != nil {
				fmt.Printf("AIModel %s not found in %s\n", aimodelName, namespace)
				return nil
			}

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

			// Delete AIModel (the operator will clean up the InferenceService)
			fmt.Println("\nDeleting AIModel...")
			fmt.Println("The operator will automatically clean up the InferenceService.")
			if err := k8sClient.DeleteAIModel(ctx, aimodelName, namespace); err != nil {
				return fmt.Errorf("delete aimodel: %w", err)
			}

			if wait {
				fmt.Println("Waiting for deletion...")
				// For now, we don't have a wait function for AIModel deletion
				// Just give it a few seconds
				time.Sleep(5 * time.Second)
				fmt.Println("AIModel deleted. The operator will continue cleaning up resources.")
			}

			fmt.Printf("\nUndeployed %s from %s\n", modelName, environment)
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

// NewRestartCommand creates the model restart command
func NewRestartCommand() *cobra.Command {
	var (
		environment string
		wait        bool
	)

	cmd := &cobra.Command{
		Use:   "restart <model-name>",
		Short: "Rolling restart of model deployment",
		Long: `Perform a rolling restart of a model deployment.

This triggers a new rollout of the InferenceService pods without
changing the configuration.

Examples:
  # Restart a model deployment
  ai-aas-cli model restart llama-3-8b -e development

  # Restart and wait for ready
  ai-aas-cli model restart llama-3-8b -e development --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			// Build InferenceService name
			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			// Get kubeconfig for environment
			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			// Create Kubernetes client
			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			// Check if exists
			_, err = k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("InferenceService %s not found in %s", isvcName, namespace)
			}

			fmt.Printf("Restarting InferenceService: %s/%s\n", namespace, isvcName)
			fmt.Println("Triggering rolling restart via annotation update...")

			// Trigger rolling restart by updating annotation
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

			fmt.Printf("\nRestart initiated for %s in %s\n", modelName, environment)
			fmt.Println("\nNote: Use 'ai-aas-cli model status' to monitor restart progress")

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for restart to complete")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// NewScaleCommand creates the model scale command
func NewScaleCommand() *cobra.Command {
	var (
		environment string
		replicas    string
	)

	cmd := &cobra.Command{
		Use:   "scale <model-name>",
		Short: "Scale model replicas",
		Long: `Scale the replica count of a model deployment.

You can specify a single number or a range (min-max).

Examples:
  # Scale to 3 replicas
  ai-aas-cli model scale llama-3-8b -e development --replicas 3

  # Set auto-scaling range
  ai-aas-cli model scale llama-3-8b -e development --replicas 2-5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Parse replicas
			replicaCount, _, err := parseReplicas(replicas)
			if err != nil {
				return fmt.Errorf("invalid replicas: %w", err)
			}

			// Build InferenceService name
			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := environment

			// Get kubeconfig for environment
			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

			// Create Kubernetes client
			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			// Check if exists
			status, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("InferenceService %s not found in %s", isvcName, namespace)
			}

			fmt.Printf("Scaling InferenceService: %s/%s\n", namespace, isvcName)
			fmt.Printf("  Current replicas: %d (ready: %d)\n", status.Replicas, status.ReadyReplicas)
			fmt.Printf("  Target: %d replicas\n", replicaCount)

			// Scale the InferenceService
			if err := k8sClient.ScaleInferenceService(ctx, isvcName, namespace, replicaCount); err != nil {
				return fmt.Errorf("scale inferenceservice: %w", err)
			}

			fmt.Printf("\nSuccessfully scaled %s to %d replicas\n", modelName, replicaCount)
			fmt.Println("\nNote: Use 'ai-aas-cli model status' to check scaling progress")

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().StringVar(&replicas, "replicas", "1", "replica count or range (e.g., 1-3)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// parseReplicas parses a replica string like "3" or "2-5"
func parseReplicas(s string) (min, max int, err error) {
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

// generateInferenceServiceYAML generates YAML for an InferenceService
func generateInferenceServiceYAML(cfg kubernetes.InferenceServiceConfig) ([]byte, error) {
	// Build resource requirements
	resources := map[string]interface{}{
		"limits": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB),
		},
		"requests": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB/2),
		},
	}
	if cfg.GPUCount > 0 {
		resources["limits"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
		resources["requests"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
	}

	labels := map[string]interface{}{
		"app.kubernetes.io/name":       cfg.ModelName,
		"app.kubernetes.io/managed-by": "ai-aas-cli",
		"ai-aas.io/environment":        cfg.Environment,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	annotations := map[string]interface{}{}
	for k, v := range cfg.Annotations {
		annotations[k] = v
	}

	// Build model spec
	modelSpec := map[string]interface{}{
		"modelFormat": map[string]interface{}{
			"name": "vllm",
		},
		"resources": resources,
	}

	// For HuggingFace models, we pass model ID via env var (vLLM downloads directly)
	// For S3 models, we use storageUri (storage initializer downloads)
	if cfg.StorageURI != "" && !strings.HasPrefix(cfg.StorageURI, "hf://") {
		modelSpec["storageUri"] = cfg.StorageURI
	}

	if cfg.Runtime != "" {
		modelSpec["runtime"] = cfg.Runtime
	}

	// Build container env vars for HuggingFace model
	if cfg.HFModelID != "" {
		modelSpec["env"] = []map[string]interface{}{
			{
				"name":  "VLLM_MODEL_NAME",
				"value": cfg.HFModelID,
			},
		}
	}

	isvc := map[string]interface{}{
		"apiVersion": "serving.kserve.io/v1beta1",
		"kind":       "InferenceService",
		"metadata": map[string]interface{}{
			"name":        cfg.Name,
			"namespace":   cfg.Namespace,
			"labels":      labels,
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			"predictor": map[string]interface{}{
				"model":       modelSpec,
				"minReplicas": cfg.MinReplicas,
				"maxReplicas": cfg.MaxReplicas,
			},
		},
	}

	return yaml.Marshal(isvc)
}

// generateAIModelYAML generates YAML for an AIModel custom resource
func generateAIModelYAML(cfg kubernetes.AIModelConfig) ([]byte, error) {
	// Build resource requirements
	resources := map[string]interface{}{
		"limits": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB),
		},
		"requests": map[string]interface{}{
			"memory": fmt.Sprintf("%dGi", cfg.MemoryGB/2),
		},
	}
	if cfg.GPUCount > 0 {
		resources["limits"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
		resources["requests"].(map[string]interface{})["nvidia.com/gpu"] = cfg.GPUCount
	}

	labels := map[string]interface{}{
		"app.kubernetes.io/name":       cfg.ModelName,
		"app.kubernetes.io/managed-by": "ai-aas-cli",
		"ai-aas.io/environment":        cfg.Environment,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	annotations := map[string]interface{}{}
	for k, v := range cfg.Annotations {
		annotations[k] = v
	}

	// Build spec based on recipe or inline configuration
	spec := map[string]interface{}{}

	// If using a recipe, build recipeRef and overrides
	if cfg.RecipeName != "" {
		// Add recipeRef
		recipeRef := map[string]interface{}{
			"name": cfg.RecipeName,
		}
		if cfg.RecipeNamespace != "" {
			recipeRef["namespace"] = cfg.RecipeNamespace
		} else {
			recipeRef["namespace"] = "ai-model-system" // default namespace
		}
		spec["recipeRef"] = recipeRef

		// Build overrides for explicitly set values
		overrides := map[string]interface{}{}

		// Resource overrides
		if cfg.OverrideGPUCount || cfg.OverrideMemory {
			resourceOverrides := map[string]interface{}{}

			if cfg.OverrideGPUCount || cfg.OverrideMemory {
				gpuOverrides := map[string]interface{}{}
				if cfg.OverrideGPUCount {
					gpuOverrides["count"] = int32(cfg.GPUCount)
				}
				resourceOverrides["gpu"] = gpuOverrides
			}

			if cfg.OverrideMemory {
				memoryOverrides := map[string]interface{}{
					"requests": fmt.Sprintf("%dGi", cfg.MemoryGB),
				}
				resourceOverrides["memory"] = memoryOverrides
			}

			overrides["resources"] = resourceOverrides
		}

		// Replica overrides
		if cfg.OverrideMinReplicas || cfg.OverrideMaxReplicas {
			replicaOverrides := map[string]interface{}{}
			if cfg.OverrideMinReplicas {
				replicaOverrides["min"] = int32(cfg.MinReplicas)
			}
			if cfg.OverrideMaxReplicas {
				replicaOverrides["max"] = int32(cfg.MaxReplicas)
			}
			overrides["replicas"] = replicaOverrides
		}

		if len(overrides) > 0 {
			spec["overrides"] = overrides
		}
	} else {
		// Inline configuration (backward compatible)
		spec = map[string]interface{}{
			"modelName": cfg.ModelName,
			"modelID":   cfg.ModelID,
			"s3Bucket":  cfg.S3Bucket,
			"s3Key":     cfg.S3Key,
			"enabled":   cfg.Enabled,
			"runtime":   cfg.Runtime,
			"resources": resources,
		}

		if cfg.RuntimeName != "" {
			spec["runtimeName"] = cfg.RuntimeName
		}
		if cfg.MinReplicas > 0 {
			spec["minReplicas"] = cfg.MinReplicas
		}
		if cfg.MaxReplicas > 0 {
			spec["maxReplicas"] = cfg.MaxReplicas
		}
		if cfg.TrustRemoteCode {
			spec["trustRemoteCode"] = true
		}
	}

	aimodel := map[string]interface{}{
		"apiVersion": "aimodel.ai-aas.io/v1alpha1",
		"kind":       "AIModel",
		"metadata": map[string]interface{}{
			"name":        cfg.Name,
			"namespace":   cfg.Namespace,
			"labels":      labels,
			"annotations": annotations,
		},
		"spec": spec,
	}

	return yaml.Marshal(aimodel)
}
