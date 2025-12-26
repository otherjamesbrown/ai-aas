// Package pod provides pod management commands.
package pod

import (
	"github.com/spf13/cobra"
)

// NewPodCommand creates the pod command group
func NewPodCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod",
		Short: "Pod health and diagnostics",
		Long: `Retrieve health information for model pods in the platform.

The pod command provides diagnostics for model inference pods:
  - Health status and readiness
  - Restart counts and failure reasons
  - Resource usage and node placement
  - Container states and conditions

Examples:
  ai-aas-cli pod health                     # Show all pod health
  ai-aas-cli pod health --model llama-7b    # Filter by model
  ai-aas-cli pod health --unhealthy-only    # Show only unhealthy pods`,
	}

	// Add subcommands
	cmd.AddCommand(NewHealthCommand())

	return cmd
}
