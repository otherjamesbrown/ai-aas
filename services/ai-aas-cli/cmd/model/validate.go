// Package model provides CLI commands for model management.
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/validation"
)

// NewValidateCommand creates the model validate command
func NewValidateCommand() *cobra.Command {
	var (
		environment string
		jsonOutput  bool
		skipChecks  []string
	)

	cmd := &cobra.Command{
		Use:   "validate [model-name]",
		Short: "Validate model configuration and deployment",
		Long: `Validate a model across all layers: registry, cache, deployment, and endpoint.

The validation performs these checks:
- Registry: Model is registered and configuration is valid
- Cache: Model files are cached in object storage
- Deployment: InferenceService exists and is healthy
- Endpoint: Model responds to inference requests

Examples:
  # Validate all models
  ai-aas-cli model validate -e development

  # Validate a specific model
  ai-aas-cli model validate llama-3-8b -e development

  # Output validation results as JSON
  ai-aas-cli model validate llama-3-8b -e development --json

  # Skip certain checks
  ai-aas-cli model validate llama-3-8b -e development --skip endpoint`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

			// Get models to validate
			var modelNames []string
			if len(args) > 0 {
				modelNames = []string{args[0]}
			} else {
				// List all models
				models, err := regClient.List(ctx, registry.ListOptions{})
				if err != nil {
					return fmt.Errorf("list models: %w", err)
				}
				for _, m := range models {
					modelNames = append(modelNames, m.Name)
				}
			}

			if len(modelNames) == 0 {
				fmt.Println("No models found to validate.")
				return nil
			}

			// Build skip set
			skipSet := make(map[string]bool)
			for _, s := range skipChecks {
				skipSet[s] = true
			}

			// Validate each model
			results := make(map[string]*ModelValidationResult)
			allPassed := true

			for _, modelName := range modelNames {
				result := validateModel(ctx, modelName, environment, regClient, skipSet)
				results[modelName] = result
				if !result.Passed {
					allPassed = false
				}
			}

			// Output results
			if jsonOutput {
				output := map[string]interface{}{
					"timestamp":   time.Now().UTC(),
					"environment": environment,
					"passed":      allPassed,
					"results":     results,
				}
				jsonBytes, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal json: %w", err)
				}
				fmt.Println(string(jsonBytes))
			} else {
				printValidationResults(results, allPassed)
			}

			if !allPassed {
				return fmt.Errorf("validation failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().StringSliceVar(&skipChecks, "skip", nil, "skip specific checks (registry, cache, deployment, endpoint)")

	return cmd
}

// ModelValidationResult contains validation results for a single model
type ModelValidationResult struct {
	Model      string            `json:"model"`
	Passed     bool              `json:"passed"`
	Duration   time.Duration     `json:"duration"`
	Registry   *CheckResult      `json:"registry,omitempty"`
	Cache      *CheckResult      `json:"cache,omitempty"`
	Deployment *CheckResult      `json:"deployment,omitempty"`
	Endpoint   *CheckResult      `json:"endpoint,omitempty"`
}

// CheckResult contains the result of a single validation check
type CheckResult struct {
	Passed   bool              `json:"passed"`
	Message  string            `json:"message"`
	Details  map[string]string `json:"details,omitempty"`
	Duration time.Duration     `json:"duration"`
	Skipped  bool              `json:"skipped,omitempty"`
}

func validateModel(ctx context.Context, modelName, environment string, regClient *registry.Client, skipSet map[string]bool) *ModelValidationResult {
	start := time.Now()
	result := &ModelValidationResult{
		Model:  modelName,
		Passed: true,
	}

	// Registry check
	if !skipSet["registry"] {
		result.Registry = checkRegistry(ctx, modelName, regClient)
		if !result.Registry.Passed && !result.Registry.Skipped {
			result.Passed = false
		}
	}

	// Cache check
	if !skipSet["cache"] {
		result.Cache = checkCache(ctx, modelName)
		if !result.Cache.Passed && !result.Cache.Skipped {
			result.Passed = false
		}
	}

	// Deployment check
	if !skipSet["deployment"] && environment != "" {
		result.Deployment = checkDeployment(ctx, modelName, environment)
		if !result.Deployment.Passed && !result.Deployment.Skipped {
			result.Passed = false
		}
	}

	// Endpoint check
	if !skipSet["endpoint"] && environment != "" && result.Deployment != nil && result.Deployment.Passed {
		result.Endpoint = checkEndpoint(ctx, modelName, environment)
		if !result.Endpoint.Passed && !result.Endpoint.Skipped {
			result.Passed = false
		}
	}

	result.Duration = time.Since(start)
	return result
}

func checkRegistry(ctx context.Context, modelName string, regClient *registry.Client) *CheckResult {
	start := time.Now()
	result := &CheckResult{}

	model, err := regClient.Get(ctx, modelName)
	if err != nil {
		result.Message = fmt.Sprintf("Not found in registry: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Passed = true
	result.Message = "Model registered"
	result.Details = map[string]string{
		"hf_model_id": model.HFModelID,
	}
	result.Duration = time.Since(start)
	return result
}

func checkCache(ctx context.Context, modelName string) *CheckResult {
	start := time.Now()
	result := &CheckResult{}

	s3Client, err := getS3Client(ctx)
	if err != nil {
		result.Message = fmt.Sprintf("Cannot connect to S3: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Check for manifest
	manifestPath := fmt.Sprintf("models/%s/main/manifest.json", modelName)
	exists, err := s3Client.Exists(ctx, manifestPath)
	if err != nil {
		result.Message = fmt.Sprintf("Error checking cache: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	if !exists {
		result.Message = "Not cached (no manifest found)"
		result.Duration = time.Since(start)
		return result
	}

	result.Passed = true
	result.Message = "Model cached"
	result.Duration = time.Since(start)
	return result
}

func checkDeployment(ctx context.Context, modelName, environment string) *CheckResult {
	start := time.Now()
	result := &CheckResult{}

	// Get kubeconfig for environment
	kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
	kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

	k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
		Kubeconfig: kubeconfig,
		Context:    kubecontext,
		Namespace:  environment,
	})
	if err != nil {
		result.Message = fmt.Sprintf("Cannot connect to cluster: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	isvcName := fmt.Sprintf("%s-%s", modelName, environment)
	status, err := k8sClient.GetInferenceService(ctx, isvcName, environment)
	if err != nil {
		result.Message = fmt.Sprintf("Not deployed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Details = map[string]string{
		"ready": fmt.Sprintf("%v", status.Ready),
	}
	if status.URL != "" {
		result.Details["url"] = status.URL
	}

	if status.Ready {
		result.Passed = true
		result.Message = "Deployed and ready"
	} else {
		result.Message = "Deployed but not ready"
		// Get more details from conditions
		for _, cond := range status.Conditions {
			if cond.Type == "Ready" && cond.Status == "False" {
				result.Details["reason"] = cond.Reason
				result.Details["detail"] = cond.Message
			}
		}
	}

	result.Duration = time.Since(start)
	return result
}

func checkEndpoint(ctx context.Context, modelName, environment string) *CheckResult {
	start := time.Now()
	result := &CheckResult{}

	// Get kubeconfig for environment
	kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
	kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))

	k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
		Kubeconfig: kubeconfig,
		Context:    kubecontext,
		Namespace:  environment,
	})
	if err != nil {
		result.Message = fmt.Sprintf("Cannot connect to cluster: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	isvcName := fmt.Sprintf("%s-%s", modelName, environment)
	status, err := k8sClient.GetInferenceService(ctx, isvcName, environment)
	if err != nil {
		result.Message = "Cannot get endpoint URL"
		result.Duration = time.Since(start)
		return result
	}

	if status.URL == "" {
		result.Message = "No endpoint URL available"
		result.Duration = time.Since(start)
		return result
	}

	// Test the endpoint
	validator := validation.NewInferenceValidator()
	inferenceResult := validator.ValidateEndpoint(ctx, status.URL, "")

	result.Details = map[string]string{
		"url":           status.URL,
		"response_time": inferenceResult.ResponseTime.String(),
	}

	if inferenceResult.Passed {
		result.Passed = true
		result.Message = fmt.Sprintf("Inference OK (%s)", inferenceResult.ResponseTime.Round(time.Millisecond))
	} else {
		result.Message = inferenceResult.Message
		if inferenceResult.Error != "" {
			result.Details["error"] = inferenceResult.Error
		}
	}

	result.Duration = time.Since(start)
	return result
}

func printValidationResults(results map[string]*ModelValidationResult, allPassed bool) {
	for modelName, result := range results {
		status := "PASS"
		statusIcon := "\u2705" // green checkmark
		if !result.Passed {
			status = "FAIL"
			statusIcon = "\u274c" // red X
		}

		fmt.Printf("\n%s %s (%s)\n", statusIcon, modelName, status)
		fmt.Println(strings.Repeat("-", 40))

		if result.Registry != nil {
			printCheck("Registry", result.Registry)
		}
		if result.Cache != nil {
			printCheck("Cache", result.Cache)
		}
		if result.Deployment != nil {
			printCheck("Deployment", result.Deployment)
		}
		if result.Endpoint != nil {
			printCheck("Endpoint", result.Endpoint)
		}

		fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Millisecond))
	}

	fmt.Println()
	if allPassed {
		fmt.Println("\u2705 All validations passed")
	} else {
		fmt.Println("\u274c Some validations failed")
	}
}

func printCheck(name string, check *CheckResult) {
	icon := "\u2705"
	if !check.Passed {
		icon = "\u274c"
	}
	if check.Skipped {
		icon = "\u23ed" // skip icon
	}

	fmt.Printf("  %s %-12s %s\n", icon, name+":", check.Message)

	// Print details if present
	if len(check.Details) > 0 {
		for k, v := range check.Details {
			if k != "error" && v != "" { // Don't double-print errors
				fmt.Printf("     %s: %s\n", k, v)
			}
		}
	}
}
