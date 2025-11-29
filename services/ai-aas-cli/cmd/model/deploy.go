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
		environment    string
		gpuCount       int
		memoryGB       int
		minReplicas    int
		maxReplicas    int
		revision       string
		dryRun         bool
		skipValidation bool
		wait           bool
		timeout        time.Duration
	)

	cmd := &cobra.Command{
		Use:   "deploy <model-name>",
		Short: "Deploy model to Kubernetes",
		Long: `Deploy a cached model to Kubernetes using KServe InferenceService.

The model must already be cached in object storage before deployment.

Examples:
  # Deploy a model to development
  ai-aas-cli model deploy llama-3-8b -e development

  # Deploy with specific resource requirements
  ai-aas-cli model deploy llama-3-8b -e development --gpu-count 2 --memory 48

  # Deploy with auto-scaling
  ai-aas-cli model deploy llama-3-8b -e production --min-replicas 2 --max-replicas 5

  # Preview deployment YAML
  ai-aas-cli model deploy llama-3-8b -e development --dry-run

  # Deploy and wait for ready
  ai-aas-cli model deploy llama-3-8b -e development --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			// Get configuration
			cfg, err := config.Load()
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
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
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

			// Build storage URI
			storageURI := fmt.Sprintf("s3://%s/models/%s/%s/", s3Bucket, modelName, revision)

			// Create InferenceService config
			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			namespace := fmt.Sprintf("%s", environment) // Using environment as namespace

			isvcCfg := kubernetes.InferenceServiceConfig{
				Name:        isvcName,
				Namespace:   namespace,
				ModelName:   modelName,
				StorageURI:  storageURI,
				GPUCount:    gpuCount,
				MemoryGB:    memoryGB,
				MinReplicas: minReplicas,
				MaxReplicas: maxReplicas,
				Environment: environment,
				Labels: map[string]string{
					"ai-aas.io/model":       modelName,
					"ai-aas.io/revision":    revision,
					"ai-aas.io/environment": environment,
				},
				Annotations: map[string]string{
					"ai-aas.io/hf-model-id": model.HFModelID,
				},
			}

			// Show configuration
			fmt.Printf("\nDeployment Configuration:\n")
			fmt.Printf("  Model: %s (%s)\n", modelName, model.HFModelID)
			fmt.Printf("  Environment: %s\n", environment)
			fmt.Printf("  InferenceService: %s/%s\n", namespace, isvcName)
			fmt.Printf("  Storage: %s\n", storageURI)
			fmt.Printf("  Resources: %d GPU(s), %dGB memory\n", gpuCount, memoryGB)
			fmt.Printf("  Replicas: %d-%d\n", minReplicas, maxReplicas)

			if dryRun {
				// Generate YAML and print
				yamlBytes, err := generateInferenceServiceYAML(isvcCfg)
				if err != nil {
					return fmt.Errorf("generate YAML: %w", err)
				}
				fmt.Println("\n---")
				fmt.Println(string(yamlBytes))
				fmt.Println("---")
				fmt.Println("\n[DRY RUN] No resources created.")
				return nil
			}

			// Validate cache exists if not skipping validation
			if !skipValidation {
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
			existing, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err == nil && existing != nil {
				fmt.Printf("\nWARNING: InferenceService %s already exists.\n", isvcName)
				fmt.Printf("  Status: Ready=%v, URL=%s\n", existing.Ready, existing.URL)
				fmt.Println("Use 'model undeploy' first, or 'model restart' to update.")
				return nil
			}

			// Create InferenceService
			fmt.Println("\nCreating InferenceService...")
			if err := k8sClient.CreateInferenceService(ctx, isvcCfg); err != nil {
				return fmt.Errorf("create inferenceservice: %w", err)
			}

			fmt.Printf("Created InferenceService: %s/%s\n", namespace, isvcName)

			// Wait for ready if requested
			if wait {
				fmt.Println("\nWaiting for deployment to be ready...")
				spinner := output.NewSpinner("Deploying")
				spinner.Start()

				waitOpts := kubernetes.WaitOptions{
					Timeout:      timeout,
					PollInterval: 5 * time.Second,
				}

				err := k8sClient.WaitForReady(ctx, isvcName, namespace, waitOpts)
				spinner.Stop()

				if err != nil {
					return fmt.Errorf("deployment failed: %w", err)
				}

				status, _ := k8sClient.GetInferenceService(ctx, isvcName, namespace)
				if status != nil {
					fmt.Printf("\nDeployment ready!\n")
					fmt.Printf("  URL: %s\n", status.URL)
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

This removes the InferenceService but preserves the cached model files.
To fully remove a model including cache, use 'model cache delete'.

Examples:
  # Undeploy a model
  ai-aas-cli model undeploy llama-3-8b -e development

  # Force undeploy without graceful drain
  ai-aas-cli model undeploy llama-3-8b -e development --force

  # Undeploy and wait for deletion
  ai-aas-cli model undeploy llama-3-8b -e development --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
			status, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				fmt.Printf("InferenceService %s not found in %s\n", isvcName, namespace)
				return nil
			}

			fmt.Printf("Removing InferenceService: %s/%s\n", namespace, isvcName)
			fmt.Printf("  Ready: %v\n", status.Ready)
			if status.URL != "" {
				fmt.Printf("  URL: %s\n", status.URL)
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

			// Delete InferenceService
			fmt.Println("\nDeleting InferenceService...")
			if err := k8sClient.DeleteInferenceService(ctx, isvcName, namespace); err != nil {
				return fmt.Errorf("delete inferenceservice: %w", err)
			}

			if wait {
				fmt.Println("Waiting for deletion...")
				if err := k8sClient.WaitForDelete(ctx, isvcName, namespace, 5*time.Minute); err != nil {
					return fmt.Errorf("wait for delete: %w", err)
				}
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
				"model": map[string]interface{}{
					"modelFormat": map[string]interface{}{
						"name": "vllm",
					},
					"storageUri": cfg.StorageURI,
					"resources":  resources,
				},
				"minReplicas": cfg.MinReplicas,
				"maxReplicas": cfg.MaxReplicas,
			},
		},
	}

	return yaml.Marshal(isvc)
}
