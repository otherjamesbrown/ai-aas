// Package main provides the entry point for the ai-aas-org CLI.
//
// ai-aas-org is a user-friendly CLI for organization administrators
// to manage users, API keys, and view usage metrics within their organization.
package main

import (
	"os"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/cmd"
	clierrors "github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
)

// Build information - set via ldflags at build time
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	// Set version info for the root command
	cmd.SetVersionInfo(version, commit, buildTime)

	if err := cmd.Execute(); err != nil {
		os.Exit(clierrors.GetExitCode(err))
	}
}
