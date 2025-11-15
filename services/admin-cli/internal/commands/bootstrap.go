// Package commands provides Cobra command implementations for the Admin CLI.
//
// Purpose:
//
//	Bootstrap command for initializing the platform with the first admin account.
//	Supports dry-run mode, confirmation prompts, service health checks, and audit logging.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-001 (Bootstrap Operations)
//   - specs/009-admin-cli/spec.md#FR-001 (bootstrap with dry-run)
//   - specs/009-admin-cli/spec.md#FR-003 (confirmation prompts)
//   - specs/009-admin-cli/spec.md#FR-008 (service health checks)
//   - specs/009-admin-cli/spec.md#FR-010 (audit logging)
//
package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

var (
	// Global flags
	flagDryRun          bool
	flagConfirm         bool
	flagFormat          string
	flagVerbose         bool
	flagQuiet           bool
	flagUserOrgEndpoint string
	flagAPIKey          string
)

// BootstrapCommand creates the bootstrap command.
func BootstrapCommand() *cobra.Command {
	var flagEmail string
	var flagOrgName string
	var flagOrgSlug string
	var flagDryRunLocal bool
	var flagConfirmLocal bool
	var flagFormatLocal string
	var flagVerboseLocal bool
	var flagQuietLocal bool
	var flagUserOrgEndpointLocal string
	var flagAPIKeyLocal string

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the platform with first admin account",
		Long: `Bootstrap initializes the platform by creating the first admin account.
This command requires the user-org-service to be running and accessible.

By default, this command runs in dry-run mode to preview changes.
Use --confirm to execute the bootstrap operation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrap(cmd, args, flagEmail, flagOrgName, flagOrgSlug, flagDryRunLocal, flagConfirmLocal, flagFormatLocal, flagVerboseLocal, flagQuietLocal, flagUserOrgEndpointLocal, flagAPIKeyLocal)
		},
	}

	cmd.Flags().BoolVar(&flagDryRunLocal, "dry-run", true, "Preview changes without executing (default)")
	cmd.Flags().BoolVar(&flagConfirmLocal, "confirm", false, "Execute bootstrap operation (requires --dry-run=false)")
	cmd.Flags().StringVar(&flagFormatLocal, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerboseLocal, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuietLocal, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpointLocal, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKeyLocal, "api-key", "", "API key for authentication (overrides config)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "Email for admin user (required)")
	cmd.Flags().StringVar(&flagOrgName, "org-name", "", "Organization name (default: 'Platform Admin Organization')")
	cmd.Flags().StringVar(&flagOrgSlug, "org-slug", "", "Organization slug (default: 'platform-admin')")

	return cmd
}

func runBootstrap(cmd *cobra.Command, args []string, flagEmail, flagOrgName, flagOrgSlug string, flagDryRun, flagConfirm bool, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply flag overrides
	if flagUserOrgEndpoint != "" {
		cfg.UserOrgEndpoint = flagUserOrgEndpoint
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if flagFormat != "" {
		cfg.OutputFormat = flagFormat
	}
	if flagVerbose {
		cfg.Verbose = true
	}
	if flagQuiet {
		cfg.Quiet = true
	}

	// Validate configuration
	if cfg.UserOrgEndpoint == "" {
		return fmt.Errorf("user-org-service endpoint is required (set via --user-org-endpoint or config)")
	}

	// Validate email if executing (or in dry-run for preview)
	if flagEmail == "" {
		// In dry-run, we can show preview without email, but warn
		if !flagDryRun {
			return fmt.Errorf("--email is required for bootstrap operation")
		}
		if !cfg.Quiet {
			fmt.Println("Note: --email is required when executing. Dry-run continuing without email.")
		}
	}

	// Health check
	if !flagDryRun {
		checker := health.NewChecker(5 * time.Second)
		requiredServices := map[string]string{
			"user-org-service": cfg.UserOrgEndpoint,
		}
		if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
			return fmt.Errorf("service health check failed: %w", err)
		}
	}

	// Determine operation mode
	executing := !flagDryRun && flagConfirm

	// Check for existing admin
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	existingAdmin, err := userOrgClient.CheckExistingAdmin(cmd.Context())
	if err != nil {
		// Log warning but continue - service might not support this check
		if !cfg.Quiet {
			fmt.Printf("Warning: could not check for existing admin: %v\n", err)
		}
		existingAdmin = false
	}

	if existingAdmin && !executing {
		if !cfg.Quiet {
			fmt.Println("⚠️  WARNING: Admin account already exists")
			fmt.Println("This operation will create an additional admin account.")
			fmt.Println("Use --confirm and --dry-run=false to proceed.")
		}
		if cfg.OutputFormat == "json" {
			return output.PrintJSON(map[string]interface{}{
				"mode":      "dry-run",
				"operation": "bootstrap",
				"warning":   "Admin account already exists - will create additional account",
			})
		}
		return nil // Don't error, just show warning in dry-run
	}

	// Dry-run mode: show planned changes
	if flagDryRun || !executing {
		if !cfg.Quiet {
			fmt.Println("DRY-RUN MODE: Preview of changes")
			fmt.Println("============================================================")
			fmt.Println("Operation: Bootstrap platform")
			fmt.Println("Action: Create first admin account")
			fmt.Println("Endpoint:", cfg.UserOrgEndpoint)
			if existingAdmin {
				fmt.Println("⚠️  WARNING: Admin account exists - this will overwrite it")
			}
			fmt.Println("\nUse --confirm and --dry-run=false to execute")
		}

		if cfg.OutputFormat == "json" {
			return output.PrintJSON(map[string]interface{}{
				"mode":      "dry-run",
				"operation": "bootstrap",
				"endpoint":  cfg.UserOrgEndpoint,
				"warning":   "Admin account exists - will overwrite",
			})
		}

		return nil
	}

	// Execute bootstrap
	if !cfg.Quiet {
		fmt.Println("Executing bootstrap...")
	}

	bootstrapReq := userorg.BootstrapRequest{
		Email:       flagEmail,
		DisplayName: "Platform Admin",
		OrgName:     flagOrgName,
		OrgSlug:     flagOrgSlug,
	}

	resp, err := userOrgClient.Bootstrap(cmd.Context(), bootstrapReq)
	if err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	adminID := resp.AdminID
	apiKey := resp.APIKey

	duration := time.Since(startTime)

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	if err := auditLogger.LogOperation(audit.Operation{
		Type:        "bootstrap",
		UserIdentity: "cli-user", // TODO: Get from config/auth
		Command:     cmd.CommandPath(),
		Parameters: map[string]interface{}{
			"endpoint": cfg.UserOrgEndpoint,
			"existing_admin": existingAdmin,
		},
		Outcome:   "success",
		Duration:  duration,
		BreakGlass: existingAdmin,
	}); err != nil {
		// Log error but don't fail operation
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to write audit log: %v\n", err)
	}

	// Output results
	if !cfg.Quiet {
		fmt.Printf("✓ Bootstrap completed successfully\n")
		fmt.Printf("  Admin ID: %s\n", adminID)
		fmt.Printf("  API Key: %s (save this securely)\n", apiKey)
		fmt.Printf("  Duration: %s\n", duration.Round(time.Millisecond))
	}

	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"success":   true,
			"admin_id":  adminID,
			"api_key":   apiKey,
			"duration":  duration.String(),
			"endpoint":  cfg.UserOrgEndpoint,
		})
	}

	return nil
}

