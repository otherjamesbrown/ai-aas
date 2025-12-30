// Package model provides CLI commands for model management.
package model

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/kubernetes"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// NewStatusCommand creates the model status command
func NewStatusCommand() *cobra.Command {
	var (
		environment string
		format      string
		watch       bool
		namespace   string
	)

	cmd := &cobra.Command{
		Use:   "status [model-name]",
		Short: "Show model status",
		Long: `Show the current status of a model or all models.

Examples:
  # Show status of all models
  ai-aas-cli model status

  # Show status of specific model
  ai-aas-cli model status llama-3-8b

  # Show status in specific environment
  ai-aas-cli model status llama-3-8b --environment production

  # Watch live deployment progress
  ai-aas-cli model status llama-3-8b --watch
  ai-aas-cli model status llama-3-8b -w --namespace ai-system`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Use Admin API endpoint for registry operations
			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx := context.Background()

			// Watch mode requires a specific model
			if watch {
				if len(args) == 0 {
					return fmt.Errorf("watch mode requires a model name")
				}
				name := args[0]
				return watchModelStatus(ctx, name, namespace)
			}

			// Non-watch mode with timeout
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			apiClient := cfg.NewAPIClient(adminEndpoint)
			regClient := registry.NewClient(apiClient)
			deployClient := client.NewDeploymentClient(apiClient)

			// Single model status
			if len(args) > 0 {
				name := args[0]
				return showModelStatus(ctx, regClient, deployClient, name, environment, format)
			}

			// All models status
			return showAllModelsStatus(ctx, regClient, deployClient, environment, format)
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "", "filter by environment")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch live deployment progress")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "ai-system", "kubernetes namespace")

	return cmd
}

func showModelStatus(ctx context.Context, regClient *registry.Client, deployClient *client.DeploymentClient, name, environment, format string) error {
	model, err := regClient.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	// Try to get deployment details if environment is specified
	var deployment *client.Deployment
	if environment != "" {
		deployment, _ = deployClient.Get(ctx, name, environment)
	}

	if format == "json" {
		result := map[string]interface{}{
			"model":      model,
			"deployment": deployment,
		}
		return output.PrintJSON(result, true)
	}

	// Status indicators
	registryStatus := "✅"
	cacheStatus := "❌"
	deployStatus := "❌"

	if model.CacheStatus == "ready" {
		cacheStatus = "✅"
	} else if model.CacheStatus == "downloading" {
		cacheStatus = "⏳"
	}

	deployStatusStr := model.DeploymentStatus
	if deployment != nil {
		deployStatusStr = deployment.Status
	}

	if deployStatusStr == "ready" {
		deployStatus = "✅"
	} else if deployStatusStr == "deploying" {
		deployStatus = "⏳"
	} else if deployStatusStr == "disabled" {
		deployStatus = "⏸️"
	}

	fmt.Printf("\n%s %s\n", name, model.HFModelID)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Registry:    %s registered\n", registryStatus)
	fmt.Printf("  Cache:       %s %s\n", cacheStatus, model.CacheStatus)

	// Show deployment status with duration if available
	if deployment != nil && deployment.StatusChangedAt != nil {
		duration := output.FormatDurationSince(deployment.StatusChangedAt)
		fmt.Printf("  Deployment:  %s %s (%s)\n", deployStatus, deployStatusStr, duration)
	} else {
		fmt.Printf("  Deployment:  %s %s\n", deployStatus, deployStatusStr)
	}

	return nil
}

func showAllModelsStatus(ctx context.Context, regClient *registry.Client, deployClient *client.DeploymentClient, environment, format string) error {
	models, err := regClient.List(ctx, registry.ListOptions{
		Environment: environment,
	})
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	// Fetch deployments for duration information if environment specified
	deploymentsMap := make(map[string]*client.Deployment)
	if environment != "" {
		deployments, _ := deployClient.List(ctx, client.ListOptions{
			Environment: environment,
		})
		for i := range deployments {
			deploymentsMap[deployments[i].ModelName] = &deployments[i]
		}
	}

	if format == "json" {
		result := map[string]interface{}{
			"models":      models,
			"deployments": deploymentsMap,
		}
		return output.PrintJSON(result, true)
	}

	table := output.NewTableWriter()
	table.SetHeader([]string{"MODEL", "REGISTRY", "CACHED", "DEPLOYED", "STATUS", "DURATION"})

	for _, m := range models {
		regIcon := "✅"

		cacheIcon := "❌"
		if m.CacheStatus == "ready" {
			cacheIcon = "✅"
		} else if m.CacheStatus == "downloading" {
			cacheIcon = "⏳"
		}

		deployIcon := "❌"
		deployStatusStr := m.DeploymentStatus

		// Get deployment details if available
		deployment := deploymentsMap[m.Name]
		if deployment != nil {
			deployStatusStr = deployment.Status
		}

		if deployStatusStr == "ready" {
			deployIcon = "✅"
		} else if deployStatusStr == "deploying" {
			deployIcon = "⏳"
		} else if deployStatusStr == "disabled" {
			deployIcon = "⏸️"
		}

		status := "registered"
		if deployStatusStr == "ready" {
			status = "ready"
		} else if m.CacheStatus == "ready" {
			status = "cached"
		}

		// Calculate duration if available
		durationStr := "-"
		if deployment != nil && deployment.StatusChangedAt != nil {
			durationStr = output.FormatDurationSince(deployment.StatusChangedAt)
		}

		table.Append([]string{
			m.Name,
			regIcon,
			cacheIcon,
			deployIcon,
			status,
			durationStr,
		})
	}

	table.Render()
	return nil
}

// AIModelWatchStatus represents the status fields we extract for watch mode
type AIModelWatchStatus struct {
	Name                 string
	Namespace            string
	Phase                string
	PhaseStartTime       *time.Time
	PhaseDuration        string
	Progress             *PhaseProgress
	SchedulingStatus     *SchedulingStatus
	InferenceServiceName string
	InferenceEndpoint    string
	ReadyReplicas        int32
	Message              string
}

// PhaseProgress represents download/deployment progress
type PhaseProgress struct {
	Percentage int
	Downloaded string
	ETA        string
}

// SchedulingStatus represents pod scheduling information
type SchedulingStatus struct {
	Scheduled      bool
	BlockedReason  string
	BlockedMessage string
	NodeName       string
}

func watchModelStatus(ctx context.Context, name, namespace string) error {
	// Import kubernetes client
	k8sClient, err := getKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	if namespace == "" {
		namespace = k8sClient.Namespace()
	}

	// Set up signal handler for graceful shutdown (Ctrl+C)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Poll every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Initial fetch to check if model exists
	status, err := fetchAIModelStatus(ctx, k8sClient, name, namespace)
	if err != nil {
		return err
	}

	// Display initial status
	clearScreen()
	displayWatchStatus(status)

	// Watch loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			status, err := fetchAIModelStatus(ctx, k8sClient, name, namespace)
			if err != nil {
				// Display error but continue watching
				clearScreen()
				output.ErrorMsg("Failed to fetch status: %v", err)
				fmt.Printf("\nLast updated: %s [Ctrl+C to exit]\n", time.Now().Format("2006-01-02 15:04:05"))
				continue
			}

			clearScreen()
			displayWatchStatus(status)

			// Exit watch if model reaches terminal state
			if status.Phase == "Ready" || status.Phase == "Failed" || status.Phase == "Disabled" {
				fmt.Println()
				output.InfoMsg("Model reached terminal state: %s", status.Phase)
				return nil
			}
		}
	}
}

func displayWatchStatus(status *AIModelWatchStatus) {
	output.Header(fmt.Sprintf("Model: %s", status.Name))
	fmt.Printf("Namespace: %s\n", status.Namespace)
	fmt.Println()

	// Phase with duration
	phaseDisplay := status.Phase
	if status.PhaseDuration != "" {
		phaseDisplay = fmt.Sprintf("%s (%s)", status.Phase, status.PhaseDuration)
	}
	output.KeyValue("Phase", phaseDisplay)

	// Progress information (if available)
	if status.Progress != nil {
		if status.Progress.Percentage > 0 {
			progressBar := buildProgressBar(status.Progress.Percentage)
			fmt.Printf("  Progress: %s %d%%\n", progressBar, status.Progress.Percentage)
		}
		if status.Progress.Downloaded != "" {
			output.KeyValue("Downloaded", status.Progress.Downloaded)
		}
		if status.Progress.ETA != "" {
			output.KeyValue("ETA", status.Progress.ETA)
		}
	}

	// Scheduling status (if in Scheduling phase)
	if status.SchedulingStatus != nil {
		fmt.Println()
		if status.SchedulingStatus.Scheduled {
			output.KeyValue("Scheduling", output.Success.Sprintf("Scheduled on %s", status.SchedulingStatus.NodeName))
		} else {
			schedulingMsg := fmt.Sprintf("Blocked (%s)", status.SchedulingStatus.BlockedReason)
			if status.SchedulingStatus.BlockedMessage != "" {
				schedulingMsg = fmt.Sprintf("%s - %s", schedulingMsg, status.SchedulingStatus.BlockedMessage)
			}
			output.KeyValue("Scheduling", output.Warning.Sprint(schedulingMsg))
		}
	}

	// Message (if present)
	if status.Message != "" {
		fmt.Println()
		output.KeyValue("Message", status.Message)
	}

	// InferenceService details
	fmt.Println()
	output.KeyValue("InferenceService", status.InferenceServiceName)
	if status.InferenceEndpoint != "" {
		output.KeyValue("Endpoint", status.InferenceEndpoint)
	} else {
		output.KeyValue("Endpoint", output.Muted.Sprint("(pending)"))
	}
	output.KeyValue("Ready Replicas", fmt.Sprintf("%d", status.ReadyReplicas))

	fmt.Println()
	fmt.Printf("%s %s\n",
		output.Muted.Sprint("Last updated:"),
		time.Now().Format("2006-01-02 15:04:05"))
	output.InfoMsg("Press Ctrl+C to exit")
}

func buildProgressBar(percentage int) string {
	const barWidth = 20
	filled := percentage * barWidth / 100
	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return output.Info.Sprint(bar)
}

func clearScreen() {
	// ANSI escape code to clear screen and move cursor to top
	fmt.Print("\033[2J\033[H")
}

func getKubernetesClient() (*kubernetes.Client, error) {
	// Import the kubernetes package
	k8sClient, err := kubernetes.NewClient(kubernetes.ClientConfig{
		Namespace: "ai-system",
	})
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return k8sClient, nil
}

func fetchAIModelStatus(ctx context.Context, k8sClient *kubernetes.Client, name, namespace string) (*AIModelWatchStatus, error) {
	// Use dynamic client to fetch AIModel
	dynamicClient, err := dynamic.NewForConfig(k8sClient.RESTConfig())
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}

	result, err := dynamicClient.Resource(kubernetes.AIModelGVR).Namespace(namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("model %q not found in namespace %q. Run 'ai-aas-cli model list' to see available models", name, namespace)
		}
		return nil, fmt.Errorf("get aimodel: %w", err)
	}

	return parseWatchStatus(result)
}

func parseWatchStatus(obj *unstructured.Unstructured) (*AIModelWatchStatus, error) {
	status := &AIModelWatchStatus{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Get phase
	if phase, ok, _ := unstructured.NestedString(obj.Object, "status", "phase"); ok {
		status.Phase = phase
	}

	// Get phase start time and duration
	if phaseStartTimeStr, ok, _ := unstructured.NestedString(obj.Object, "status", "phaseStartTime"); ok {
		t, err := time.Parse(time.RFC3339, phaseStartTimeStr)
		if err == nil {
			status.PhaseStartTime = &t
		}
	}
	if phaseDuration, ok, _ := unstructured.NestedString(obj.Object, "status", "phaseDuration"); ok {
		status.PhaseDuration = phaseDuration
	}

	// Get progress
	if progressMap, ok, _ := unstructured.NestedMap(obj.Object, "status", "progress"); ok {
		progress := &PhaseProgress{}
		if percentage, ok := progressMap["percentage"].(int64); ok {
			progress.Percentage = int(percentage)
		}
		if downloaded, ok := progressMap["downloaded"].(string); ok {
			progress.Downloaded = downloaded
		}
		if eta, ok := progressMap["eta"].(string); ok {
			progress.ETA = eta
		}
		status.Progress = progress
	}

	// Get scheduling status
	if schedulingMap, ok, _ := unstructured.NestedMap(obj.Object, "status", "schedulingStatus"); ok {
		scheduling := &SchedulingStatus{}
		if scheduled, ok := schedulingMap["scheduled"].(bool); ok {
			scheduling.Scheduled = scheduled
		}
		if blockedReason, ok := schedulingMap["blockedReason"].(string); ok {
			scheduling.BlockedReason = blockedReason
		}
		if blockedMessage, ok := schedulingMap["blockedMessage"].(string); ok {
			scheduling.BlockedMessage = blockedMessage
		}
		if nodeName, ok := schedulingMap["nodeName"].(string); ok {
			scheduling.NodeName = nodeName
		}
		status.SchedulingStatus = scheduling
	}

	// Get InferenceService name
	if isvcName, ok, _ := unstructured.NestedString(obj.Object, "status", "inferenceServiceName"); ok {
		status.InferenceServiceName = isvcName
	}

	// Get endpoint
	if endpoint, ok, _ := unstructured.NestedString(obj.Object, "status", "inferenceEndpoint"); ok {
		status.InferenceEndpoint = endpoint
	}

	// Get ready replicas
	if readyReplicas, ok, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas"); ok {
		status.ReadyReplicas = int32(readyReplicas)
	}

	// Get message
	if message, ok, _ := unstructured.NestedString(obj.Object, "status", "message"); ok {
		status.Message = message
	}

	return status, nil
}
