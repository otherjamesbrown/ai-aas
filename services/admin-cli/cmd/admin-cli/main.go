// Command admin-cli is the main entrypoint for the Admin CLI tool.
//
// Purpose:
//
//	This binary provides a command-line interface for platform administrators
//	to perform privileged operations: bootstrap, org/user/key management,
//	credential rotation, sync triggers, and exports. All operations support
//	dry-run, confirmations, and structured output (JSON/CSV/table).
//
// Dependencies:
//   - internal/config: Configuration loading from environment/config files
//   - internal/commands: Cobra command implementations
//   - internal/client: API clients for user-org-service and analytics-service
//
// Key Responsibilities:
//   - Initialize CLI root command with Cobra
//   - Register all command subcommands (bootstrap, org, user, credentials, sync, export)
//   - Handle global flags (--verbose, --quiet, --format, --config)
//   - Set up structured output and audit logging
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md
//   - specs/009-admin-cli/plan.md
//
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/commands"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "admin-cli",
		Short: "Admin CLI for platform operations",
		Long: `Admin CLI provides a command-line interface for platform administrators
to perform privileged operations: bootstrap, org/user/key management,
credential rotation, sync triggers, and exports.`,
		Version: version,
	}

	// Register subcommands
	rootCmd.AddCommand(commands.BootstrapCommand())
	rootCmd.AddCommand(commands.OrgCommand())
	rootCmd.AddCommand(commands.UserCommand())
	rootCmd.AddCommand(commands.APIKeyCommand())
	rootCmd.AddCommand(commands.CredentialsCommand())
	rootCmd.AddCommand(commands.SyncCommand())
	rootCmd.AddCommand(commands.ExportCommand())

	if err := rootCmd.Execute(); err != nil {
		// Handle structured CLI errors with exit codes
		if cliErr, ok := err.(*errors.CLIError); ok {
			fmt.Fprintf(os.Stderr, "%v\n", cliErr)
			os.Exit(cliErr.ExitCode)
		}
		
		// Default to exit code 1 for unknown errors
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

