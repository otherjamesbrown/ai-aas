// Command ai-aas-cli is the unified CLI tool for AI-AAS platform operations.
//
// Purpose:
//
//	This binary provides comprehensive model lifecycle management including:
//	registry management, HuggingFace token handling, object storage caching,
//	deployment orchestration, validation/audit capabilities, and update workflows.
//
// Key Responsibilities:
//   - Initialize CLI configuration with PATH detection
//   - Model registry operations (add, list, info, remove)
//   - Credentials management (HuggingFace tokens, S3)
//   - Object storage caching for fast deployments
//   - KServe-based model deployment
//   - Full-stack validation across all layers
//   - Enable/disable library management
//   - Model aliases for version abstraction
//
// Requirements Reference:
//   - specs/020-model-management/spec.md
//   - specs/020-model-management/plan.md
package main

import (
	"fmt"
	"os"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/cmd/ai-aas-cli"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCommand(version, buildTime, gitCommit)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
