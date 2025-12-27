// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewHFDeployCommand creates the hf-deploy command
func NewHFDeployCommand() *cobra.Command {
	var (
		name           string
		environment    string
		gpuCount       int
		memoryGB       int
		minReplicas    int
		maxReplicas    int
		acceptLicense  bool
		skipCache      bool
		wait           bool
		timeout        time.Duration
	)

	cmd := &cobra.Command{
		Use:   "hf-deploy <huggingface-url-or-model-id>",
		Short: "Deploy a HuggingFace model in one command",
		Long: `Deploy a model directly from HuggingFace in a single command.

This command combines the full deployment workflow:
  1. Register model from HuggingFace
  2. Download model to cache (server-side)
  3. Deploy to Kubernetes
  4. Wait for ready status

Examples:
  # Deploy from HuggingFace URL
  ai-aas-cli model hf-deploy https://huggingface.co/unsloth/gpt-oss-20b

  # Deploy using model ID directly
  ai-aas-cli model hf-deploy mistralai/Mistral-7B-Instruct-v0.3

  # Deploy with custom name and resources
  ai-aas-cli model hf-deploy https://huggingface.co/meta-llama/Llama-3-8B \
    --name llama-3-8b --gpu-count 1 --memory 24 --accept-license

  # Deploy without caching (slower startup, no S3 needed)
  ai-aas-cli model hf-deploy unsloth/gpt-oss-20b --skip-cache

See Also:
  ai-aas model registry list    View registered models
  ai-aas model deploy status    Check deployment status
  ai-aas model cache list       View cached models`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			// Parse HuggingFace URL or model ID
			hfModelID, err := parseHuggingFaceInput(input)
			if err != nil {
				return err
			}

			// Derive model name if not provided
			if name == "" {
				// Use the repo name (last part of model ID)
				parts := strings.Split(hfModelID, "/")
				if len(parts) >= 2 {
					name = parts[len(parts)-1]
				} else {
					name = hfModelID
				}
				// Sanitize name for k8s (lowercase, no special chars)
				name = strings.ToLower(name)
				name = strings.ReplaceAll(name, "_", "-")
			}

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()

			// Create API clients
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			fmt.Println("╔══════════════════════════════════════════════════════════════╗")
			fmt.Println("║  HuggingFace Model Deployment                                ║")
			fmt.Println("╚══════════════════════════════════════════════════════════════╝")
			fmt.Println()
			fmt.Printf("  Model ID:    %s\n", hfModelID)
			fmt.Printf("  Name:        %s\n", name)
			fmt.Printf("  Environment: %s\n", environment)
			fmt.Println()

			// Step 1: Check if already registered, if not register
			fmt.Printf("Step 1/4: Checking registry...\n")
			model, err := regClient.Get(ctx, name)
			if err != nil {
				// Model not found, need to register
				fmt.Printf("  Model not registered, checking HuggingFace Hub...\n")

				hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))
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

				// Check if gated
				requiresAuth := modelInfo.Private
				if modelInfo.Gated {
					if !acceptLicense {
						return fmt.Errorf("gated model requires license acceptance\n\n   Accept license at: https://huggingface.co/%s\n\nAfter accepting, run again with --accept-license", hfModelID)
					}
					requiresAuth = true
				}

				// Get license info
				licenseType := ""
				if info, err := hfClient.GetLicenseInfo(ctx, hfModelID); err == nil {
					licenseType = info.LicenseType
				}

				fmt.Printf("  Registering model as '%s'...\n", name)
				model, err = regClient.Add(ctx, registry.AddRequest{
					HFModelID:     hfModelID,
					Name:          name,
					RequiresAuth:  requiresAuth,
					LicenseType:   licenseType,
					AcceptLicense: acceptLicense,
					GPUMemoryGB:   gpuCount * 16, // Estimate
					CPUMemoryGB:   memoryGB,
				})
				if err != nil {
					return fmt.Errorf("failed to register model: %w", err)
				}
				fmt.Printf("  ✓ Model registered\n")
			} else {
				fmt.Printf("  ✓ Model already registered\n")
			}

			// Step 2: Cache the model (unless skip-cache)
			s3Bucket := viper.GetString("s3.bucket")
			revision := "main"

			if !skipCache && s3Bucket != "" {
				fmt.Printf("\nStep 2/4: Caching model (server-side download)...\n")
				fmt.Printf("  This may take several minutes for large models.\n")

				err = runServerSidePull(ctx, apiClient, name, revision, false)
				if err != nil {
					// Check if already cached
					if strings.Contains(err.Error(), "already cached") || strings.Contains(err.Error(), "already exists") {
						fmt.Printf("  ✓ Model already cached\n")
					} else {
						return fmt.Errorf("failed to cache model: %w", err)
					}
				} else {
					fmt.Printf("  ✓ Model cached to S3\n")
				}
			} else if skipCache {
				fmt.Printf("\nStep 2/4: Skipping cache (will download from HF on deploy)\n")
			} else {
				fmt.Printf("\nStep 2/4: Skipping cache (S3 not configured)\n")
			}

			// Step 3: Deploy to Kubernetes
			fmt.Printf("\nStep 3/4: Deploying to Kubernetes...\n")

			// Build storage URI
			var storageURI string
			if !skipCache && s3Bucket != "" {
				storageURI = fmt.Sprintf("s3://%s/models/%s/%s/", s3Bucket, name, revision)
			} else {
				storageURI = fmt.Sprintf("hf://%s", model.HFModelID)
			}

			isvcName := fmt.Sprintf("%s-%s", name, environment)
			namespace := environment

			isvcCfg := kubernetes.InferenceServiceConfig{
				Name:        isvcName,
				Namespace:   namespace,
				ModelName:   name,
				StorageURI:  storageURI,
				HFModelID:   model.HFModelID,
				Runtime:     "vllm-runtime",
				GPUCount:    gpuCount,
				MemoryGB:    memoryGB,
				MinReplicas: minReplicas,
				MaxReplicas: maxReplicas,
				Environment: environment,
				Labels: map[string]string{
					"ai-aas.io/model":       name,
					"ai-aas.io/revision":    revision,
					"ai-aas.io/environment": environment,
				},
				Annotations: map[string]string{
					"ai-aas.io/hf-model-id": model.HFModelID,
				},
			}

			fmt.Printf("  InferenceService: %s/%s\n", namespace, isvcName)
			fmt.Printf("  Storage: %s\n", storageURI)
			fmt.Printf("  Resources: %d GPU(s), %dGB memory\n", gpuCount, memoryGB)

			// Get kubeconfig for environment
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

			// Check if already exists
			existing, _ := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if existing != nil {
				fmt.Printf("  InferenceService already exists, checking status...\n")
			} else {
				// Create the InferenceService
				if err := k8sClient.CreateInferenceService(ctx, isvcCfg); err != nil {
					return fmt.Errorf("create inferenceservice: %w", err)
				}
				fmt.Printf("  ✓ InferenceService created\n")
			}

			// Step 4: Wait for ready
			fmt.Printf("\nStep 4/4: Waiting for deployment to be ready...\n")

			waitTimeout := timeout
			if waitTimeout == 0 {
				waitTimeout = 15 * time.Minute
			}

			waitCtx, waitCancel := context.WithTimeout(ctx, waitTimeout)
			defer waitCancel()

			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			startTime := time.Now()
			lastStatus := ""

			for {
				select {
				case <-waitCtx.Done():
					return fmt.Errorf("timeout waiting for deployment to be ready after %s\n\nCheck logs:\n  ai-aas model troubleshoot logs %s -e %s", waitTimeout, name, environment)
				case <-ticker.C:
					status, err := k8sClient.GetInferenceService(waitCtx, isvcName, namespace)
					if err != nil {
						continue
					}

					currentStatus := fmt.Sprintf("Ready=%v Replicas=%d/%d", status.Ready, status.ReadyReplicas, status.Replicas)
					if currentStatus != lastStatus {
						elapsed := time.Since(startTime).Round(time.Second)
						fmt.Printf("  [%s] %s\n", elapsed, currentStatus)
						lastStatus = currentStatus
					}

					if status.Ready && status.ReadyReplicas > 0 {
						fmt.Println()
						fmt.Println("╔══════════════════════════════════════════════════════════════╗")
						fmt.Println("║  Deployment Complete!                                        ║")
						fmt.Println("╚══════════════════════════════════════════════════════════════╝")
						fmt.Println()
						fmt.Printf("  Model:       %s\n", name)
						fmt.Printf("  Environment: %s\n", environment)
						fmt.Printf("  URL:         %s\n", status.URL)
						fmt.Printf("  Replicas:    %d/%d ready\n", status.ReadyReplicas, status.Replicas)
						fmt.Println()
						fmt.Println("Next steps:")
						fmt.Printf("  ai-aas model troubleshoot test %s -e %s   # Test inference\n", name, environment)
						fmt.Printf("  ai-aas model deploy status %s -e %s       # Check status\n", name, environment)
						return nil
					}
				}
			}
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Model name (derived from URL if not specified)")
	cmd.Flags().StringVarP(&environment, "environment", "e", "development", "Target environment")
	cmd.Flags().IntVar(&gpuCount, "gpu-count", 1, "Number of GPUs")
	cmd.Flags().IntVar(&memoryGB, "memory", 24, "Memory in GB")
	cmd.Flags().IntVar(&minReplicas, "min-replicas", 1, "Minimum replicas")
	cmd.Flags().IntVar(&maxReplicas, "max-replicas", 1, "Maximum replicas")
	cmd.Flags().BoolVar(&acceptLicense, "accept-license", false, "Accept model license (required for gated models)")
	cmd.Flags().BoolVar(&skipCache, "skip-cache", false, "Skip caching, deploy directly from HuggingFace")
	cmd.Flags().BoolVar(&wait, "wait", true, "Wait for deployment to be ready")
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "Timeout waiting for ready")

	return cmd
}

// parseHuggingFaceInput parses a HuggingFace URL or model ID
func parseHuggingFaceInput(input string) (string, error) {
	// If it looks like a URL, parse it
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}

		// Must be huggingface.co
		if u.Host != "huggingface.co" && u.Host != "www.huggingface.co" {
			return "", fmt.Errorf("URL must be from huggingface.co, got: %s", u.Host)
		}

		// Extract model ID from path (e.g., /unsloth/gpt-oss-20b)
		modelPath := strings.TrimPrefix(u.Path, "/")
		// Remove any trailing parts like /tree/main
		parts := strings.Split(modelPath, "/")
		if len(parts) >= 2 {
			// Return org/model format
			return path.Join(parts[0], parts[1]), nil
		}

		return "", fmt.Errorf("could not extract model ID from URL path: %s", u.Path)
	}

	// Otherwise treat as model ID directly
	// Validate it looks like org/model format
	if !strings.Contains(input, "/") {
		return "", fmt.Errorf("model ID must be in format 'org/model', got: %s", input)
	}

	return input, nil
}
