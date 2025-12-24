// Package model provides CLI commands for model management.
package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/cli"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
)

// NewTroubleshootParentCommand creates the model troubleshoot parent command
func NewTroubleshootParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "troubleshoot",
		Short: "Debug and validate model deployments",
		Long: `Debug and validate model deployments with comprehensive tools.

Use these commands to investigate issues with model deployments, view logs,
check events, and run validation tests.

Examples:
  # View pod logs
  ai-aas model troubleshoot logs mistral-7b -e development

  # Check Kubernetes events
  ai-aas model troubleshoot events mistral-7b -e development

  # Get detailed deployment info
  ai-aas model troubleshoot describe mistral-7b -e development

  # Test inference
  ai-aas model troubleshoot test mistral-7b -e development

  # Run full validation
  ai-aas model troubleshoot validate mistral-7b -e development

Common Issues:
  - Pod stuck Pending    Check events for resource issues
  - Model not ready      Check logs for loading errors
  - Inference failing    Run test command with --prompt`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newTroubleshootLogsCommand())
	cmd.AddCommand(newTroubleshootEventsCommand())
	cmd.AddCommand(newTroubleshootDescribeCommand())
	cmd.AddCommand(newTroubleshootTestCommand())
	cmd.AddCommand(newTroubleshootValidateCommand())

	return cmd
}

// newTroubleshootLogsCommand creates the troubleshoot logs subcommand
func newTroubleshootLogsCommand() *cobra.Command {
	var (
		environment string
		follow      bool
		tailLines   int
		container   string
		previous    bool
	)

	cmd := &cobra.Command{
		Use:   "logs <model-name>",
		Short: "Stream model pod logs",
		Long: `Stream logs from the pods running a deployed model.

View startup logs to debug loading issues, or follow logs in real-time
to monitor inference requests.

Examples:
  # View recent logs
  ai-aas model troubleshoot logs mistral-7b -e development

  # Follow logs in real-time
  ai-aas model troubleshoot logs mistral-7b -e development --follow

  # View last 200 lines
  ai-aas model troubleshoot logs mistral-7b -e development --tail 200

  # View logs from a specific container
  ai-aas model troubleshoot logs mistral-7b -e development --container kserve-container

  # View logs from previous (crashed) pod
  ai-aas model troubleshoot logs mistral-7b -e development --previous

See Also:
  ai-aas model troubleshoot events    Check K8s events
  ai-aas model troubleshoot describe  Get detailed info
  ai-aas model deploy status          Check deployment status`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))
			namespace := environment

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			pods, err := k8sClient.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("serving.kserve.io/inferenceservice=%s", isvcName),
			})
			if err != nil {
				return fmt.Errorf("list pods: %w", err)
			}

			if len(pods.Items) == 0 {
				return fmt.Errorf("no pods found for model %s in %s\n\nTo check deployment status:\n  ai-aas model deploy status %s -e %s", modelName, environment, modelName, environment)
			}

			fmt.Printf("Found %d pod(s) for %s\n\n", len(pods.Items), modelName)

			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning && !previous {
					continue
				}

				containerName := container
				if containerName == "" {
					for _, c := range pod.Spec.Containers {
						if c.Name == "kserve-container" {
							containerName = c.Name
							break
						}
					}
					if containerName == "" && len(pod.Spec.Containers) > 0 {
						containerName = pod.Spec.Containers[0].Name
					}
				}

				fmt.Printf("=== Pod: %s (container: %s) ===\n", pod.Name, containerName)

				opts := &corev1.PodLogOptions{
					Container: containerName,
					Follow:    follow,
					TailLines: int64Ptr(int64(tailLines)),
					Previous:  previous,
				}

				req := k8sClient.Clientset().CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
				stream, err := req.Stream(ctx)
				if err != nil {
					fmt.Printf("Error getting logs: %v\n", err)
					continue
				}
				defer stream.Close()

				reader := bufio.NewReader(stream)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							break
						}
						return err
					}
					fmt.Print(line)
				}

				if !follow {
					break
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().BoolVar(&follow, "follow", false, "follow log output")
	cmd.Flags().IntVar(&tailLines, "tail", 100, "lines to show from end of logs")
	cmd.Flags().StringVar(&container, "container", "", "container name (default: kserve-container)")
	cmd.Flags().BoolVar(&previous, "previous", false, "show logs from previous container instance")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newTroubleshootEventsCommand creates the troubleshoot events subcommand
func newTroubleshootEventsCommand() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "events <model-name>",
		Short: "Show Kubernetes events for model",
		Long: `Show Kubernetes events related to a model's deployment.

Events reveal scheduling issues, resource problems, image pull errors,
and other cluster-level issues.

Examples:
  # View events for a model
  ai-aas model troubleshoot events mistral-7b -e development

See Also:
  ai-aas model troubleshoot logs      View pod logs
  ai-aas model troubleshoot describe  Get detailed info`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))
			namespace := environment

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)

			events, err := k8sClient.Clientset().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("list events: %w", err)
			}

			fmt.Printf("Events for %s in %s:\n", modelName, environment)
			fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────")
			fmt.Printf("%-8s %-12s %-40s %s\n", "TYPE", "REASON", "OBJECT", "MESSAGE")
			fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────")

			found := false
			for _, event := range events.Items {
				objName := event.InvolvedObject.Name
				if strings.HasPrefix(objName, isvcName) || strings.Contains(objName, modelName) {
					found = true
					typeStr := event.Type
					objRef := fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name)
					if len(objRef) > 40 {
						objRef = objRef[:37] + "..."
					}
					msg := event.Message
					if len(msg) > 50 {
						msg = msg[:47] + "..."
					}
					fmt.Printf("%-8s %-12s %-40s %s\n", typeStr, event.Reason, objRef, msg)
				}
			}

			if !found {
				fmt.Println("No recent events found.")
			}

			cli.PrintNextSteps([]cli.NextStep{
				{
					Command:     fmt.Sprintf("ai-aas model troubleshoot logs %s -e %s", modelName, environment),
					Description: "View pod logs",
				},
				{
					Command:     fmt.Sprintf("ai-aas model troubleshoot describe %s -e %s", modelName, environment),
					Description: "Get detailed info",
				},
			})

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newTroubleshootDescribeCommand creates the troubleshoot describe subcommand
func newTroubleshootDescribeCommand() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "describe <model-name>",
		Short: "Show full deployment details",
		Long: `Show detailed information about a model deployment.

Includes InferenceService status, conditions, pod details, resource usage,
and container statuses.

Examples:
  # Get detailed info
  ai-aas model troubleshoot describe mistral-7b -e development

See Also:
  ai-aas model deploy status          Quick status check
  ai-aas model troubleshoot logs      View logs
  ai-aas model troubleshoot events    View events`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))
			namespace := environment

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)

			isvc, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("deployment not found: %s in %s\n\nTo deploy:\n  ai-aas model deploy create %s -e %s", isvcName, namespace, modelName, environment)
			}

			fmt.Printf("=== InferenceService: %s ===\n\n", isvcName)
			fmt.Printf("Name:      %s\n", isvc.Name)
			fmt.Printf("Namespace: %s\n", isvc.Namespace)
			fmt.Printf("Ready:     %v\n", isvc.Ready)
			if isvc.URL != "" {
				fmt.Printf("URL:       %s\n", isvc.URL)
			}

			fmt.Printf("\nConditions:\n")
			for _, cond := range isvc.Conditions {
				fmt.Printf("  - Type: %s, Status: %s", cond.Type, cond.Status)
				if cond.Reason != "" {
					fmt.Printf(", Reason: %s", cond.Reason)
				}
				if cond.Message != "" {
					fmt.Printf("\n    Message: %s", cond.Message)
				}
				fmt.Println()
			}

			pods, err := k8sClient.GetPodStatus(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("get pods: %w", err)
			}

			fmt.Printf("\n=== Pods (%d) ===\n\n", len(pods))
			for _, pod := range pods {
				fmt.Printf("Name:     %s\n", pod.Name)
				fmt.Printf("Phase:    %s\n", pod.Phase)
				fmt.Printf("Ready:    %v\n", pod.Ready)
				fmt.Printf("Restarts: %d\n", pod.Restarts)
				fmt.Printf("Age:      %s\n", formatDurationShort(pod.Age))
				fmt.Println()
			}

			podList, err := k8sClient.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("serving.kserve.io/inferenceservice=%s", isvcName),
			})
			if err == nil && len(podList.Items) > 0 {
				pod := podList.Items[0]
				fmt.Printf("=== Resource Details (from %s) ===\n\n", pod.Name)

				for _, container := range pod.Spec.Containers {
					fmt.Printf("Container: %s\n", container.Name)
					if container.Resources.Requests != nil {
						fmt.Printf("  Requests:\n")
						if cpu := container.Resources.Requests.Cpu(); cpu != nil {
							fmt.Printf("    CPU: %s\n", cpu.String())
						}
						if mem := container.Resources.Requests.Memory(); mem != nil {
							fmt.Printf("    Memory: %s\n", mem.String())
						}
						if gpu, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
							fmt.Printf("    GPU: %s\n", gpu.String())
						}
					}
					if container.Resources.Limits != nil {
						fmt.Printf("  Limits:\n")
						if cpu := container.Resources.Limits.Cpu(); cpu != nil {
							fmt.Printf("    CPU: %s\n", cpu.String())
						}
						if mem := container.Resources.Limits.Memory(); mem != nil {
							fmt.Printf("    Memory: %s\n", mem.String())
						}
						if gpu, ok := container.Resources.Limits["nvidia.com/gpu"]; ok {
							fmt.Printf("    GPU: %s\n", gpu.String())
						}
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newTroubleshootTestCommand creates the troubleshoot test subcommand
func newTroubleshootTestCommand() *cobra.Command {
	var (
		environment string
		prompt      string
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "test <model-name>",
		Short: "Run inference test",
		Long: `Run an inference test against a deployed model to verify it's working.

Sends a simple chat completion request and displays the response,
latency, and status.

Examples:
  # Basic test
  ai-aas model troubleshoot test mistral-7b -e development

  # Custom prompt
  ai-aas model troubleshoot test mistral-7b -e development --prompt "Explain quantum computing"

  # With timeout
  ai-aas model troubleshoot test mistral-7b -e development --timeout 60s

See Also:
  ai-aas model troubleshoot validate  Full validation
  ai-aas model deploy status          Check status`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			kubeconfig := viper.GetString(fmt.Sprintf("environments.%s.kubeconfig", environment))
			kubecontext := viper.GetString(fmt.Sprintf("environments.%s.context", environment))
			namespace := environment

			k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
				Kubeconfig: kubeconfig,
				Context:    kubecontext,
				Namespace:  namespace,
			})
			if err != nil {
				return fmt.Errorf("create k8s client: %w", err)
			}

			isvcName := fmt.Sprintf("%s-%s", modelName, environment)

			isvc, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("deployment not found: %s\n\nTo deploy:\n  ai-aas model deploy create %s -e %s", modelName, modelName, environment)
			}

			if !isvc.Ready {
				pods, podErr := k8sClient.GetPodStatus(ctx, isvcName, namespace)
				if podErr == nil && len(pods) > 0 {
					pod := pods[0]
					switch pod.Phase {
					case "Pending":
						return fmt.Errorf("model is pending (waiting for resources or scheduling)\n\nTo check events:\n  ai-aas model troubleshoot events %s -e %s", modelName, environment)
					case "Running":
						if !pod.Ready {
							return fmt.Errorf("model is starting up (loading model weights, this may take several minutes)\n\nTo watch progress:\n  ai-aas model troubleshoot logs %s -e %s --follow", modelName, environment)
						}
					case "ContainerCreating":
						return fmt.Errorf("model container is being created (pulling image or initializing)")
					}
				}

				for _, cond := range isvc.Conditions {
					if cond.Type == "Ready" && cond.Status != "True" {
						if cond.Reason != "" {
							return fmt.Errorf("model not ready: %s - %s", cond.Reason, cond.Message)
						}
					}
				}

				return fmt.Errorf("model not ready\n\nTo check details:\n  ai-aas model troubleshoot describe %s -e %s", modelName, environment)
			}

			fmt.Printf("Testing %s in %s\n", modelName, environment)
			fmt.Printf("Endpoint: %s\n", cfg.APIEndpoint)
			fmt.Printf("Prompt: %s\n\n", prompt)

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}

			startTime := time.Now()

			reqBody := map[string]interface{}{
				"model": modelName,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
				"max_tokens": 100,
			}

			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequestWithContext(ctx, "POST",
				fmt.Sprintf("%s/v1/chat/completions", cfg.APIEndpoint),
				bytes.NewBuffer(jsonBody))
			if err != nil {
				return fmt.Errorf("create request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")
			if cfg.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			}

			client := api.NewHTTPClient(cfg.TLSInsecure)
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("inference request failed: %w", err)
			}
			defer resp.Body.Close()

			latency := time.Since(startTime)

			body, _ := io.ReadAll(resp.Body)

			fmt.Printf("Status: %s\n", resp.Status)
			fmt.Printf("Latency: %s\n", latency.Round(time.Millisecond))

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err == nil {
					if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if message, ok := choice["message"].(map[string]interface{}); ok {
								if content, ok := message["content"].(string); ok {
									fmt.Printf("\nResponse:\n%s\n", content)
								}
							}
						}
					}
				}
				fmt.Println()
				cli.PrintSuccess("Test PASSED")
			} else {
				fmt.Printf("\nError: %s\n", string(body))
				fmt.Println()
				cli.PrintWarning("Test FAILED")
				return fmt.Errorf("inference test failed with status %d", resp.StatusCode)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "What is the capital of the UK?", "test prompt")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "request timeout")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// newTroubleshootValidateCommand creates the troubleshoot validate subcommand
func newTroubleshootValidateCommand() *cobra.Command {
	var (
		environment string
		jsonOutput  bool
		skipChecks  []string
	)

	cmd := &cobra.Command{
		Use:   "validate [model-name]",
		Short: "Run full-stack validation",
		Long: `Validate a model across all layers: registry, cache, deployment, and endpoint.

Performs comprehensive checks:
- Registry: Model is registered and configuration is valid
- Cache: Model files are cached in object storage
- Deployment: InferenceService exists and is healthy
- Endpoint: Model responds to inference requests

Examples:
  # Validate all models
  ai-aas model troubleshoot validate -e development

  # Validate a specific model
  ai-aas model troubleshoot validate mistral-7b -e development

  # Output as JSON
  ai-aas model troubleshoot validate mistral-7b -e development --json

  # Skip certain checks
  ai-aas model troubleshoot validate mistral-7b -e development --skip endpoint

See Also:
  ai-aas model troubleshoot test      Quick inference test
  ai-aas model registry status        Registry status only`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.AdminAPIEndpoint
			if adminEndpoint == "" {
				adminEndpoint = cfg.APIEndpoint
			}

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			regClient := registry.NewClient(apiClient)

			var modelNames []string
			if len(args) > 0 {
				modelNames = []string{args[0]}
			} else {
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

			skipSet := make(map[string]bool)
			for _, s := range skipChecks {
				skipSet[s] = true
			}

			results := make(map[string]*ModelValidationResult)
			allPassed := true

			// Get inference timeout from config (in seconds)
			inferenceTimeout := time.Duration(cfg.InferenceTimeout) * time.Second

			for _, modelName := range modelNames {
				result := validateModel(ctx, modelName, environment, regClient, skipSet, inferenceTimeout)
				results[modelName] = result
				if !result.Passed {
					allPassed = false
				}
			}

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

// Helper functions
// Note: int64Ptr is defined in troubleshoot.go

func formatDurationShort(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// validateModel and related functions are in validate.go
// CheckResult and ModelValidationResult types are in validate.go
