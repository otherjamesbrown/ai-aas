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
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
)

// NewLogsCommand creates the model logs command
func NewLogsCommand() *cobra.Command {
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

Examples:
  # View recent logs
  ai-aas-cli model logs llama-3-8b -e development

  # Follow logs in real-time
  ai-aas-cli model logs llama-3-8b -e development --follow

  # View last 200 lines
  ai-aas-cli model logs llama-3-8b -e development --tail 200

  # View logs from a specific container
  ai-aas-cli model logs llama-3-8b -e development --container kserve-container

  # View logs from previous (crashed) pod
  ai-aas-cli model logs llama-3-8b -e development --previous`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			// Get kubeconfig for environment
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

			// Find pods for this model
			isvcName := fmt.Sprintf("%s-%s", modelName, environment)
			pods, err := k8sClient.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("serving.kserve.io/inferenceservice=%s", isvcName),
			})
			if err != nil {
				return fmt.Errorf("list pods: %w", err)
			}

			if len(pods.Items) == 0 {
				return fmt.Errorf("no pods found for model %s in %s", modelName, environment)
			}

			fmt.Printf("Found %d pod(s) for %s\n\n", len(pods.Items), modelName)

			// Stream logs from the first running pod
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning && !previous {
					continue
				}

				containerName := container
				if containerName == "" {
					// Default to first container or kserve-container if exists
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

				// Only show first pod's logs unless following all
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

// NewEventsCommand creates the model events command
func NewEventsCommand() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "events <model-name>",
		Short: "Show Kubernetes events for model",
		Long: `Show Kubernetes events related to a model's deployment.

This includes InferenceService events, pod events, and related resource events.

Examples:
  ai-aas-cli model events llama-3-8b -e development`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get kubeconfig for environment
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

			// Get all events in namespace and filter by related objects
			events, err := k8sClient.Clientset().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("list events: %w", err)
			}

			fmt.Printf("Events for %s in %s:\n\n", modelName, environment)
			fmt.Printf("%-8s %-12s %-40s %s\n", "TYPE", "REASON", "OBJECT", "MESSAGE")
			fmt.Println(strings.Repeat("-", 100))

			found := false
			for _, event := range events.Items {
				// Filter events related to this model
				objName := event.InvolvedObject.Name
				if strings.HasPrefix(objName, isvcName) || strings.Contains(objName, modelName) {
					found = true
					typeStr := event.Type
					if typeStr == "Warning" {
						typeStr = "Warning"
					}
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

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "target environment (required)")
	_ = cmd.MarkFlagRequired("environment")

	return cmd
}

// NewDescribeCommand creates the model describe command
func NewDescribeCommand() *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "describe <model-name>",
		Short: "Show full deployment details",
		Long: `Show detailed information about a model deployment including:
- InferenceService status and configuration
- Pod details and resource usage
- Container statuses
- Conditions and events

Examples:
  ai-aas-cli model describe llama-3-8b -e development`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get kubeconfig for environment
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

			// Get InferenceService
			isvc, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("get inferenceservice: %w", err)
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

			// Get pods
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
				fmt.Printf("Age:      %s\n", formatDuration(pod.Age))
				fmt.Println()
			}

			// Get detailed pod info
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

// NewTestCommand creates the model test command
func NewTestCommand() *cobra.Command {
	var (
		environment string
		prompt      string
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "test <model-name>",
		Short: "Run inference test",
		Long: `Run an inference test against a deployed model to verify it's working.

Examples:
  # Basic test
  ai-aas-cli model test llama-3-8b -e development

  # Custom prompt
  ai-aas-cli model test llama-3-8b -e development --prompt "Explain quantum computing"

  # With timeout
  ai-aas-cli model test llama-3-8b -e development --timeout 60s`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Get configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.APIEndpoint == "" || cfg.APIEndpoint == "http://localhost:8080" {
				return fmt.Errorf("API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			// Get kubeconfig for environment
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

			// Check if deployed
			isvc, err := k8sClient.GetInferenceService(ctx, isvcName, namespace)
			if err != nil {
				return fmt.Errorf("model not deployed: %w", err)
			}

			if !isvc.Ready {
				// Check pod status to provide more helpful error message
				pods, podErr := k8sClient.GetPodStatus(ctx, isvcName, namespace)
				if podErr == nil && len(pods) > 0 {
					pod := pods[0]
					switch pod.Phase {
					case "Pending":
						return fmt.Errorf("model is pending (waiting for resources or scheduling)")
					case "Running":
						if !pod.Ready {
							return fmt.Errorf("model is starting up (loading model weights, this may take several minutes)")
						}
					case "ContainerCreating":
						return fmt.Errorf("model container is being created (pulling image or initializing)")
					}
				}

				// Check conditions for more specific info
				for _, cond := range isvc.Conditions {
					if cond.Type == "Ready" && cond.Status != "True" {
						if cond.Reason != "" {
							return fmt.Errorf("model not ready: %s - %s", cond.Reason, cond.Message)
						}
					}
				}

				return fmt.Errorf("model not ready (use 'ai-aas model describe %s -e %s' for details)", modelName, environment)
			}

			fmt.Printf("Testing %s in %s\n", modelName, environment)
			fmt.Printf("Endpoint: %s\n", cfg.APIEndpoint)
			fmt.Printf("Prompt: %s\n\n", prompt)

			// Build request using the API router
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}

			// Send inference request via API router
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
				fmt.Println("\nTest PASSED")
			} else {
				fmt.Printf("\nError: %s\n", string(body))
				fmt.Println("\nTest FAILED")
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

// Helper functions

func int64Ptr(i int64) *int64 {
	return &i
}

func formatDuration(d time.Duration) string {
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
