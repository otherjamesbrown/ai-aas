// Package cli provides utilities for CLI formatting and user interaction.
package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// isTTY checks if stdout is a terminal
func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// bold returns bold text if in a TTY, otherwise plain text
func bold(s string) string {
	if isTTY() {
		return colorBold + s + colorReset
	}
	return s
}

// dim returns dimmed text if in a TTY, otherwise plain text
func dim(s string) string {
	if isTTY() {
		return colorDim + s + colorReset
	}
	return s
}

// green returns green text if in a TTY, otherwise plain text
func green(s string) string {
	if isTTY() {
		return colorGreen + s + colorReset
	}
	return s
}

// yellow returns yellow text if in a TTY, otherwise plain text
func yellow(s string) string {
	if isTTY() {
		return colorYellow + s + colorReset
	}
	return s
}

// cyan returns cyan text if in a TTY, otherwise plain text
func cyan(s string) string {
	if isTTY() {
		return colorCyan + s + colorReset
	}
	return s
}

// NextStep represents a suggested next action
type NextStep struct {
	Command     string // The command to run
	Description string // Brief description of what it does
}

// PrintSuccess prints a success message with a checkmark
func PrintSuccess(message string) {
	fmt.Printf("%s %s\n", green("✓"), message)
}

// PrintWarning prints a warning message
func PrintWarning(message string) {
	fmt.Printf("%s %s\n", yellow("!"), message)
}

// PrintNextSteps prints a formatted list of suggested next steps
func PrintNextSteps(steps []NextStep) {
	if len(steps) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(bold("Next steps:"))

	// Find the longest command for alignment
	maxLen := 0
	for _, step := range steps {
		if len(step.Command) > maxLen {
			maxLen = len(step.Command)
		}
	}

	// Print each step with aligned comments
	for _, step := range steps {
		padding := strings.Repeat(" ", maxLen-len(step.Command)+2)
		fmt.Printf("  %s%s%s\n", cyan(step.Command), padding, dim("# "+step.Description))
	}
}

// PrintSeeAlso prints a "See Also" section with related commands
func PrintSeeAlso(commands []NextStep) {
	if len(commands) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(bold("See Also:"))

	// Find the longest command for alignment
	maxLen := 0
	for _, cmd := range commands {
		if len(cmd.Command) > maxLen {
			maxLen = len(cmd.Command)
		}
	}

	for _, cmd := range commands {
		padding := strings.Repeat(" ", maxLen-len(cmd.Command)+2)
		fmt.Printf("  %s%s%s\n", cmd.Command, padding, dim(cmd.Description))
	}
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Number      int
	Description string
	Command     string
}

// PrintWorkflow prints a workflow guide section
func PrintWorkflow(steps []WorkflowStep) {
	if len(steps) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(bold("Workflow:"))

	for _, step := range steps {
		fmt.Printf("  %d. %-18s %s  %s\n",
			step.Number,
			step.Description,
			dim("→"),
			cyan(step.Command))
	}
}

// PrintNote prints a note/info message
func PrintNote(message string) {
	fmt.Printf("\n%s %s\n", dim("Note:"), message)
}

// PrintDeploymentStarted prints the deployment started message with guidance
func PrintDeploymentStarted(modelName, environment string) {
	PrintSuccess(fmt.Sprintf("Deployed: %s-%s", modelName, environment))
	fmt.Println()
	fmt.Println(dim("The model is starting up. Large models may take 5-10 minutes to load."))

	PrintNextSteps([]NextStep{
		{
			Command:     fmt.Sprintf("ai-aas model deploy status %s -e %s", modelName, environment),
			Description: "Check deployment status",
		},
		{
			Command:     fmt.Sprintf("ai-aas model troubleshoot logs %s -e %s", modelName, environment),
			Description: "Watch startup logs",
		},
		{
			Command:     fmt.Sprintf("ai-aas model troubleshoot test %s -e %s", modelName, environment),
			Description: "Test inference (when ready)",
		},
	})
}

// PrintDeploymentDeleted prints the deployment deleted message with guidance
func PrintDeploymentDeleted(modelName, environment string) {
	PrintSuccess(fmt.Sprintf("Removed deployment: %s-%s", modelName, environment))

	PrintNote("Model cache is preserved. To fully remove:")
	fmt.Printf("  %s         %s\n", cyan(fmt.Sprintf("ai-aas model cache delete %s", modelName)), dim("# Remove cached files"))
	fmt.Printf("  %s      %s\n", cyan(fmt.Sprintf("ai-aas model registry remove %s", modelName)), dim("# Remove from registry"))

	fmt.Println()
	fmt.Println("To re-deploy:")
	fmt.Printf("  %s\n", cyan(fmt.Sprintf("ai-aas model deploy create %s -e %s", modelName, environment)))
}

// PrintModelRegistered prints the model registered message with guidance
func PrintModelRegistered(name, hfModelID string) {
	PrintSuccess(fmt.Sprintf("Model registered: %s", name))
	fmt.Printf("   HuggingFace: %s\n", hfModelID)

	PrintNextSteps([]NextStep{
		{
			Command:     fmt.Sprintf("ai-aas model cache pull %s", name),
			Description: "Download to object storage",
		},
		{
			Command:     fmt.Sprintf("ai-aas model deploy create %s -e dev", name),
			Description: "Deploy directly from HuggingFace",
		},
		{
			Command:     fmt.Sprintf("ai-aas model registry info %s", name),
			Description: "View model details",
		},
	})
}

// PrintModelCached prints the model cached message with guidance
func PrintModelCached(name string, sizeBytes int64) {
	sizeStr := FormatBytes(sizeBytes)
	PrintSuccess(fmt.Sprintf("Model cached: %s (%s)", name, sizeStr))

	PrintNextSteps([]NextStep{
		{
			Command:     fmt.Sprintf("ai-aas model deploy create %s -e development", name),
			Description: "Deploy to development",
		},
		{
			Command:     "ai-aas model cache list",
			Description: "View all cached models",
		},
	})
}

// PrintModelDeleted prints the model deleted message
func PrintModelDeleted(name string) {
	PrintSuccess(fmt.Sprintf("Model removed: %s", name))

	PrintNote("The model has been removed from the registry.")
	fmt.Println("If cached files exist, remove them with:")
	fmt.Printf("  %s\n", cyan(fmt.Sprintf("ai-aas model cache delete %s", name)))
}

// PrintCacheDeleted prints the cache deleted message
func PrintCacheDeleted(name string) {
	PrintSuccess(fmt.Sprintf("Cache deleted: %s", name))

	PrintNote("Cached model files have been removed from object storage.")
	fmt.Println("The model remains in the registry. To fully remove:")
	fmt.Printf("  %s\n", cyan(fmt.Sprintf("ai-aas model registry remove %s", name)))
}

// FormatBytes formats bytes to human-readable string.
// Exported for use in other packages to avoid duplication.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ModelWorkflow returns the standard model deployment workflow steps
func ModelWorkflow() []WorkflowStep {
	return []WorkflowStep{
		{1, "Register model", "ai-aas model registry add <hf-id> --name <name>"},
		{2, "Cache model", "ai-aas model cache pull <name>"},
		{3, "Deploy model", "ai-aas model deploy create <name> -e <env>"},
		{4, "Test inference", "ai-aas model troubleshoot test <name> -e <env>"},
	}
}
