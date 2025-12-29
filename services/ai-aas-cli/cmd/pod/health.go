// Package pod provides pod management commands.
package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/pod"
	"github.com/spf13/cobra"
)

// NewHealthCommand creates the pod health command
func NewHealthCommand() *cobra.Command {
	var (
		namespace     string
		modelName     string
		environment   string
		unhealthyOnly bool
		details       bool
		outputFormat  string
	)

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show pod health status",
		Long: `Retrieve health information for model pods in the platform.

This command queries the Admin API to retrieve pod health information including:
  - Pod status and readiness
  - Container restart counts and reasons
  - Resource usage and node placement
  - Last termination details

Examples:
  # Show all model pod health (default namespace: system)
  ai-aas-cli pod health

  # Show health for a specific model
  ai-aas-cli pod health --model gpt-oss-20b

  # Show only unhealthy pods
  ai-aas-cli pod health --unhealthy-only

  # Show detailed container information
  ai-aas-cli pod health --details

  # Filter by environment
  ai-aas-cli pod health --environment development

  # Output as JSON
  ai-aas-cli pod health -f json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHealth(cmd.Context(), namespace, modelName, environment, unhealthyOnly, details, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "system", "Kubernetes namespace to query")
	cmd.Flags().StringVarP(&modelName, "model", "m", "", "Filter by model name")
	cmd.Flags().StringVarP(&environment, "environment", "e", "", "Filter by environment")
	cmd.Flags().BoolVar(&unhealthyOnly, "unhealthy-only", false, "Show only unhealthy pods")
	cmd.Flags().BoolVar(&details, "details", false, "Show detailed container info")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "Output format (table, json)")

	return cmd
}

func runHealth(ctx context.Context, namespace, modelName, environment string, unhealthyOnly, details bool, outputFormat string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w. Run 'ai-aas-cli --init' to configure", err)
	}

	// Determine API endpoint
	apiEndpoint := cfg.AdminAPIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = cfg.APIEndpoint
	}

	// Create API client
	apiClient := cfg.NewAPIClient(apiEndpoint)

	// Create pod client
	podClient := pod.NewClient(apiClient)

	// Query pod health
	healthOpts := pod.HealthOptions{
		Namespace:      namespace,
		ModelName:      modelName,
		Environment:    environment,
		IncludeHealthy: !unhealthyOnly,
	}

	resp, err := podClient.GetHealth(ctx, healthOpts)
	if err != nil {
		return fmt.Errorf("get pod health: %w", err)
	}

	// Output results
	if outputFormat == "json" {
		return output.PrintJSON(resp)
	}

	return printHealthTable(resp, details)
}

func printHealthTable(resp *pod.HealthResponse, showDetails bool) error {
	// Print summary header
	fmt.Println("Pod Health Summary")
	fmt.Println("==================")
	fmt.Printf("Total: %d | Healthy: %d | Unhealthy: %d | Total Restarts: %d\n",
		resp.Summary.TotalPods,
		resp.Summary.HealthyPods,
		resp.Summary.UnhealthyPods,
		resp.Summary.TotalRestarts,
	)
	fmt.Println()

	// Build table
	headers := []string{"POD", "MODEL", "STATUS", "READY", "RESTARTS", "AGE", "NODE"}
	if showDetails {
		headers = append(headers, "LAST TERM")
	}

	var rows [][]string
	for _, p := range resp.Pods {
		// Truncate long pod names
		podName := truncatePodName(p.Name, 43)

		// Format ready status
		readyStatus := "Yes"
		if !p.Ready {
			readyStatus = "No"
		}

		// Format restart count with last restart time
		restartStr := formatRestartCount(int(p.RestartCount), p.LastRestartTime)

		// Format age
		age := formatAge(p.AgeSeconds)

		// Build row
		row := []string{
			podName,
			p.ModelName,
			p.Phase,
			readyStatus,
			restartStr,
			age,
			truncateNodeName(p.Node),
		}

		// Add termination info if requested
		if showDetails {
			termInfo := formatTermination(p.LastTermination)
			row = append(row, termInfo)
		}

		rows = append(rows, row)
	}

	// Print table
	if err := output.PrintTable(headers, rows); err != nil {
		return err
	}

	// Print next steps
	if resp.Summary.UnhealthyPods > 0 || resp.Summary.TotalRestarts > 0 {
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  ai-aas-cli model troubleshoot logs <model>    # View pod logs")
		fmt.Println("  ai-aas-cli model troubleshoot events <model>  # Check K8s events")
	}

	return nil
}

// truncatePodName truncates a pod name to maxLen, adding "..." suffix
func truncatePodName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen-3] + "..."
}

// truncateNodeName truncates node name to show only the suffix
func truncateNodeName(node string) string {
	// For LKE nodes like "lke123456-78901-abc123def456", show abbreviated form
	if len(node) > 16 {
		return node[:12] + "..."
	}
	return node
}

// formatRestartCount formats restart count with optional last restart time
func formatRestartCount(count int, lastRestart *time.Time) string {
	if count == 0 {
		return "0"
	}

	if lastRestart == nil {
		return fmt.Sprintf("%d", count)
	}

	// Calculate time since last restart
	elapsed := time.Since(*lastRestart)
	return fmt.Sprintf("%d (%s ago)", count, formatDuration(elapsed))
}

// formatAge formats age in seconds to human-readable duration
func formatAge(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	return formatDuration(duration)
}

// formatDuration formats a duration to human-readable string
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

// formatTermination formats last termination info
func formatTermination(term *pod.TerminationInfo) string {
	if term == nil {
		return "-"
	}

	// Format: "Reason (exit code)"
	msg := fmt.Sprintf("%s (exit %d)", term.Reason, term.ExitCode)

	// Truncate if too long
	if len(msg) > 30 {
		return msg[:27] + "..."
	}

	return msg
}
