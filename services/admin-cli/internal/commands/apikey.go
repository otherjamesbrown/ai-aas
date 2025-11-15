// Package commands provides API key management commands.
//
// Purpose:
//
//	API key lifecycle commands: list, create, delete for organizations and users.
//	Supports structured output and audit logging.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-002 (Day-2 Management)
//
package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

// APIKeyCommand creates the apikey command group.
func APIKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "Manage API keys",
		Long:  "Manage API keys: list, create, delete",
	}

	cmd.AddCommand(apiKeyListCommand())
	cmd.AddCommand(apiKeyCreateCommand())
	cmd.AddCommand(apiKeyDeleteCommand())

	return cmd
}

func apiKeyListCommand() *cobra.Command {
	var flagOrgID string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Long:  "List API keys for an organization with structured output (table, json, csv)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPIKeyList(cmd, args, flagOrgID, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runAPIKeyList(cmd *cobra.Command, args []string, flagOrgID, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
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
		return errors.NewValidationError(
			"user-org-service endpoint is required",
			"Set via --user-org-endpoint flag or ADMIN_CLI_USER_ORG_ENDPOINT environment variable",
		)
	}

	// Validate required fields
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}

	// Health check
	checker := health.NewChecker(5 * time.Second)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	// Create client and list API keys
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	apiKeys, err := userOrgClient.ListAPIKeys(cmd.Context(), flagOrgID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list API keys: %v", err),
			"Verify your API key is valid and you have permission to list API keys in this organization.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "apikey_list",
		Command:     fmt.Sprintf("apikey list --org-id=%s", flagOrgID),
		Outcome:     "success",
		Duration:    time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(apiKeys)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"apiKeyId", "userId", "fingerprint", "status", "expiresAt"}
		var rows [][]string
		for _, key := range apiKeys {
			rows = append(rows, []string{
				key.APIKeyID,
				key.UserID,
				key.Fingerprint,
				key.Status,
				key.ExpiresAt,
			})
		}
		return output.PrintTable(headers, rows)
	} else {
		headers := []string{"API Key ID", "User ID", "Fingerprint", "Status", "Expires At"}
		var rows [][]string
		for _, key := range apiKeys {
			rows = append(rows, []string{
				key.APIKeyID,
				key.UserID,
				key.Fingerprint,
				key.Status,
				key.ExpiresAt,
			})
		}
		if len(rows) == 0 && !cfg.Quiet {
			fmt.Println("No API keys found.")
			return nil
		}
		return output.PrintTable(headers, rows)
	}
}

func apiKeyCreateCommand() *cobra.Command {
	var flagOrgID string
	var flagUserID string
	var flagScopes []string
	var flagExpiresInDays int
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create API key",
		Long:  "Create an API key for a user in an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPIKeyCreate(cmd, args, flagOrgID, flagUserID, flagScopes, flagExpiresInDays,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID (required)")
	cmd.Flags().StringSliceVar(&flagScopes, "scopes", []string{}, "API key scopes")
	cmd.Flags().IntVar(&flagExpiresInDays, "expires-in-days", 0, "Expiration in days (0 = no expiration)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runAPIKeyCreate(cmd *cobra.Command, args []string, flagOrgID, flagUserID string, flagScopes []string, flagExpiresInDays int,
	flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
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
		return errors.NewValidationError(
			"user-org-service endpoint is required",
			"Set via --user-org-endpoint flag or ADMIN_CLI_USER_ORG_ENDPOINT environment variable",
		)
	}

	// Validate required fields
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}
	if flagUserID == "" {
		return errors.NewValidationError(
			"--user-id is required",
			"Provide user ID with --user-id flag",
		)
	}

	// Health check
	checker := health.NewChecker(5 * time.Second)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	// Build request
	req := userorg.IssueAPIKeyRequest{
		Scopes: flagScopes,
	}
	if flagExpiresInDays > 0 {
		req.ExpiresInDays = &flagExpiresInDays
	}

	// Execute create
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	apiKey, err := userOrgClient.IssueUserAPIKey(cmd.Context(), flagOrgID, flagUserID, req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create API key: %v", err),
			"Verify your API key is valid and you have permission to create API keys.",
		)
	}

	// Audit logging (mask secret)
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "apikey_create",
		Command:     fmt.Sprintf("apikey create --org-id=%s --user-id=%s", flagOrgID, flagUserID),
		Parameters: map[string]interface{}{
			"orgId":     flagOrgID,
			"userId":    flagUserID,
			"apiKeyId":  apiKey.APIKeyID,
			"secret":    apiKey.Secret, // Will be masked by audit logger
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output - show secret only once
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(apiKey)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"apiKeyId", "secret", "fingerprint", "status", "expiresAt"}
		rows := [][]string{{
			apiKey.APIKeyID,
			apiKey.Secret,
			apiKey.Fingerprint,
			apiKey.Status,
			apiKey.ExpiresAt,
		}}
		return output.PrintTable(headers, rows)
	} else {
		if !cfg.Quiet {
			fmt.Println("⚠️  IMPORTANT: Save this API key now. It will not be shown again.")
			fmt.Printf("API key created successfully:\n")
			fmt.Printf("  API Key ID: %s\n", apiKey.APIKeyID)
			fmt.Printf("  Secret: %s\n", apiKey.Secret)
			fmt.Printf("  Fingerprint: %s\n", apiKey.Fingerprint)
			fmt.Printf("  Status: %s\n", apiKey.Status)
			if apiKey.ExpiresAt != "" {
				fmt.Printf("  Expires At: %s\n", apiKey.ExpiresAt)
			}
		}
		if cfg.OutputFormat == "table" {
			headers := []string{"API Key ID", "Secret", "Fingerprint", "Status", "Expires At"}
			rows := [][]string{{
				apiKey.APIKeyID,
				apiKey.Secret,
				apiKey.Fingerprint,
				apiKey.Status,
				apiKey.ExpiresAt,
			}}
			return output.PrintTable(headers, rows)
		}
	}
	return nil
}

func apiKeyDeleteCommand() *cobra.Command {
	var flagOrgID string
	var flagAPIKeyID string
	var flagConfirm bool
	var flagForce bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete API key",
		Long:  "Delete (revoke) an API key with confirmation and force flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPIKeyDelete(cmd, args, flagOrgID, flagAPIKeyID, flagConfirm, flagForce,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagAPIKeyID, "api-key-id", "", "API key ID (required)")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Confirm deletion (required unless --force)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Force deletion without confirmation prompt")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runAPIKeyDelete(cmd *cobra.Command, args []string, flagOrgID, flagAPIKeyID string, flagConfirm, flagForce bool,
	flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
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
		return errors.NewValidationError(
			"user-org-service endpoint is required",
			"Set via --user-org-endpoint flag or ADMIN_CLI_USER_ORG_ENDPOINT environment variable",
		)
	}

	// Validate required fields
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}
	if flagAPIKeyID == "" {
		return errors.NewValidationError(
			"--api-key-id is required",
			"Provide API key ID with --api-key-id flag",
		)
	}

	// Confirmation check (non-interactive mode: require --confirm or --force)
	if !flagForce && !flagConfirm {
		return errors.NewValidationError(
			"confirmation required for destructive operation",
			"Use --confirm to confirm deletion or --force to skip confirmation (non-interactive mode).",
		)
	}

	// Health check
	checker := health.NewChecker(5 * time.Second)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	// Show confirmation warning (unless forced or quiet)
	if !flagForce && !cfg.Quiet {
		fmt.Printf("⚠️  WARNING: This will revoke API key: %s\n", flagAPIKeyID)
		fmt.Println("   This action cannot be undone.")
	}

	// Execute delete
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	if err := userOrgClient.DeleteAPIKey(cmd.Context(), flagOrgID, flagAPIKeyID); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to delete API key: %v", err),
			"Verify your API key is valid and the API key exists.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "apikey_delete",
		Command:     fmt.Sprintf("apikey delete --org-id=%s --api-key-id=%s --confirm", flagOrgID, flagAPIKeyID),
		Parameters: map[string]interface{}{
			"orgId":    flagOrgID,
			"apiKeyId": flagAPIKeyID,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"success": true,
			"apiKeyId": flagAPIKeyID,
			"message": "API key deleted successfully",
		})
	} else {
		if !cfg.Quiet {
			fmt.Printf("API key deleted successfully: %s\n", flagAPIKeyID)
		}
	}
	return nil
}


