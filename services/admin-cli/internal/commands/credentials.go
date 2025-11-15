// Package commands provides credential rotation commands.
//
// Purpose:
//
//	Credential rotation and break-glass recovery operations with audit logging
//	and confirmation prompts for safety.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-001 (Credential Rotation)
//   - specs/009-admin-cli/spec.md#FR-001 (bootstrap with dry-run)
//   - specs/009-admin-cli/spec.md#FR-003 (confirmation prompts)
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
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

// CredentialsCommand creates the credentials command group.
func CredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Manage credentials",
		Long:  "Rotate credentials and perform break-glass recovery",
	}

	cmd.AddCommand(credentialsRotateCommand())
	cmd.AddCommand(credentialsBreakGlassCommand())

	return cmd
}

func credentialsRotateCommand() *cobra.Command {
	var flagOrgID string
	var flagAPIKeyID string
	var flagDryRun bool
	var flagConfirm bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate API key",
		Long: `Rotate an API key, invalidating the old key and generating a new one.
The new key secret will be displayed once - save it securely.

Requires --org-id and --api-key-id flags, or organization context from config.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCredentialsRotate(cmd, args, flagOrgID, flagAPIKeyID, flagDryRun, flagConfirm, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&flagAPIKeyID, "api-key-id", "", "API key ID to rotate (required)")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", true, "Preview changes without executing")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Execute rotation (requires --dry-run=false)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runCredentialsRotate(cmd *cobra.Command, args []string, flagOrgID, flagAPIKeyID string, flagDryRun, flagConfirm bool, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
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

	// Validate required fields
	if flagOrgID == "" {
		return fmt.Errorf("--org-id is required")
	}
	if flagAPIKeyID == "" {
		return fmt.Errorf("--api-key-id is required")
	}
	if cfg.UserOrgEndpoint == "" {
		return fmt.Errorf("user-org-service endpoint is required (set via --user-org-endpoint or config)")
	}

	// Determine operation mode
	executing := !flagDryRun && flagConfirm

	// Dry-run mode: show planned changes
	if flagDryRun || !executing {
		if !cfg.Quiet {
			fmt.Println("DRY-RUN MODE: Preview of changes")
			fmt.Println("============================================================")
			fmt.Println("Operation: Rotate API key")
			fmt.Println("Organization ID:", flagOrgID)
			fmt.Println("API Key ID:", flagAPIKeyID)
			fmt.Println("Endpoint:", cfg.UserOrgEndpoint)
			fmt.Println("\nUse --confirm and --dry-run=false to execute")
		}

		if cfg.OutputFormat == "json" {
			return output.PrintJSON(map[string]interface{}{
				"mode":        "dry-run",
				"operation":   "rotate-api-key",
				"org_id":      flagOrgID,
				"api_key_id":  flagAPIKeyID,
				"endpoint":    cfg.UserOrgEndpoint,
			})
		}

		return nil
	}

	// Execute rotation
	if !cfg.Quiet {
		fmt.Println("Rotating API key...")
	}

	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	resp, err := userOrgClient.RotateAPIKey(cmd.Context(), flagOrgID, flagAPIKeyID)
	if err != nil {
		return fmt.Errorf("rotate API key failed: %w", err)
	}

	duration := time.Since(startTime)

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	if err := auditLogger.LogOperation(audit.Operation{
		Type:        "credential_rotation",
		UserIdentity: "cli-user", // TODO: Get from config/auth
		Command:     cmd.CommandPath(),
		Parameters: map[string]interface{}{
			"org_id":     flagOrgID,
			"api_key_id": flagAPIKeyID,
			"endpoint":   cfg.UserOrgEndpoint,
		},
		Outcome:   "success",
		Duration:  duration,
		BreakGlass: false,
	}); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to write audit log: %v\n", err)
	}

	// Output results
	if !cfg.Quiet {
		fmt.Printf("✓ API key rotated successfully\n")
		fmt.Printf("  API Key ID: %s\n", resp.APIKeyID)
		fmt.Printf("  New Secret: %s (save this securely)\n", resp.Secret)
		fmt.Printf("  Fingerprint: %s\n", resp.Fingerprint)
		fmt.Printf("  Duration: %s\n", duration.Round(time.Millisecond))
	}

	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"success":     true,
			"api_key_id":  resp.APIKeyID,
			"secret":      resp.Secret,
			"fingerprint": resp.Fingerprint,
			"duration":    duration.String(),
		})
	}

	return nil
}

func credentialsBreakGlassCommand() *cobra.Command {
	var flagRecoveryToken string
	var flagEmail string
	var flagDryRun bool
	var flagConfirm bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string

	cmd := &cobra.Command{
		Use:   "break-glass",
		Short: "Break-glass recovery operation",
		Long: `Perform break-glass recovery to regain access to the platform.
This requires a recovery token and will create a new admin account.

⚠️  WARNING: This is a privileged operation that should only be used in
emergency situations when normal access has been lost.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBreakGlass(cmd, args, flagRecoveryToken, flagEmail, flagDryRun, flagConfirm, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint)
		},
	}

	cmd.Flags().StringVar(&flagRecoveryToken, "recovery-token", "", "Recovery token (required)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "Email for new admin user (required)")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", true, "Preview changes without executing")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Execute break-glass operation (requires --dry-run=false)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")

	return cmd
}

func runBreakGlass(cmd *cobra.Command, args []string, flagRecoveryToken, flagEmail string, flagDryRun, flagConfirm bool, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint string) error {
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
	if flagFormat != "" {
		cfg.OutputFormat = flagFormat
	}
	if flagVerbose {
		cfg.Verbose = true
	}
	if flagQuiet {
		cfg.Quiet = true
	}

	// Validate required fields
	if flagRecoveryToken == "" {
		return fmt.Errorf("--recovery-token is required for break-glass operation")
	}
	if flagEmail == "" {
		return fmt.Errorf("--email is required for break-glass operation")
	}
	if cfg.UserOrgEndpoint == "" {
		return fmt.Errorf("user-org-service endpoint is required (set via --user-org-endpoint or config)")
	}

	// Determine operation mode
	executing := !flagDryRun && flagConfirm

	// Dry-run mode: show planned changes with strong warnings
	if flagDryRun || !executing {
		if !cfg.Quiet {
			fmt.Println("⚠️  BREAK-GLASS OPERATION: Preview of changes")
			fmt.Println("============================================================")
			fmt.Println("⚠️  WARNING: This is a privileged recovery operation!")
			fmt.Println("Operation: Break-glass recovery")
			fmt.Println("Action: Create new admin account with recovery token")
			fmt.Println("Email:", flagEmail)
			fmt.Println("Endpoint:", cfg.UserOrgEndpoint)
			fmt.Println("\nUse --confirm and --dry-run=false to execute")
		}

		if cfg.OutputFormat == "json" {
			return output.PrintJSON(map[string]interface{}{
				"mode":         "dry-run",
				"operation":    "break-glass",
				"email":        flagEmail,
				"endpoint":     cfg.UserOrgEndpoint,
				"warning":      "This is a privileged recovery operation",
			})
		}

		return nil
	}

	// Execute break-glass (same as bootstrap but with recovery token)
	// For now, use bootstrap with recovery token in metadata
	if !cfg.Quiet {
		fmt.Println("Executing break-glass recovery...")
	}

	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, flagRecoveryToken) // Use recovery token as auth
	bootstrapReq := userorg.BootstrapRequest{
		Email:       flagEmail,
		DisplayName: "Recovery Admin",
		OrgName:     "Recovery Organization",
		OrgSlug:     "recovery-org",
	}

	resp, err := userOrgClient.Bootstrap(cmd.Context(), bootstrapReq)
	if err != nil {
		return fmt.Errorf("break-glass recovery failed: %w", err)
	}

	duration := time.Since(startTime)

	// Audit logging (mark as break-glass)
	auditLogger := audit.NewLogger(nil)
	if err := auditLogger.LogOperation(audit.Operation{
		Type:        "break_glass_recovery",
		UserIdentity: "recovery-token",
		Command:     cmd.CommandPath(),
		Parameters: map[string]interface{}{
			"email":    flagEmail,
			"endpoint": cfg.UserOrgEndpoint,
		},
		Outcome:    "success",
		Duration:   duration,
		BreakGlass: true,
	}); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to write audit log: %v\n", err)
	}

	// Output results
	if !cfg.Quiet {
		fmt.Printf("✓ Break-glass recovery completed successfully\n")
		fmt.Printf("  Admin ID: %s\n", resp.AdminID)
		fmt.Printf("  API Key: %s (save this securely)\n", resp.APIKey)
		fmt.Printf("  Duration: %s\n", duration.Round(time.Millisecond))
	}

	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"success":   true,
			"admin_id":  resp.AdminID,
			"api_key":   resp.APIKey,
			"duration":  duration.String(),
			"break_glass": true,
		})
	}

	return nil
}
