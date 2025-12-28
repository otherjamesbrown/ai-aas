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
package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// APIKeyCommand creates the apikey command group.
func APIKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "Manage API keys",
		Long: `Manage API keys for organizations and users.

API keys authenticate requests to the AI-AAS inference API. Keys can be scoped
to specific models and have optional expiration dates.

Examples:
  # List all API keys for an organization
  ai-aas-cli apikey list --org-id acme

  # Create an API key for a user
  ai-aas-cli apikey create --org-id acme --user-id u_123

  # Create with expiration
  ai-aas-cli apikey create --org-id acme --user-id u_123 --expires-in-days 90

  # Delete an API key
  ai-aas-cli apikey delete --org-id acme --api-key-id ak_123 --confirm

Workflow:
  1. Create org        ai-aas-cli org create --name <name> --slug <slug>
  2. Add users         ai-aas-cli user create --org-id <org> --email <email>
  3. Create API key    ai-aas-cli apikey create --org-id <org> --user-id <user>
  4. Use API key       curl -H "Authorization: Bearer <key>" ...

Security:
  - API key secrets are shown only once at creation time
  - Keys can be revoked immediately with 'apikey delete'
  - Consider setting expiration for production keys

See Also:
  ai-aas-cli user list     List users who can own keys
  ai-aas-cli org list      List organizations`,
	}

	cmd.AddCommand(apiKeyListCommand())
	cmd.AddCommand(apiKeyCreateCommand())
	cmd.AddCommand(apiKeyDeleteCommand())
	cmd.AddCommand(apiKeyInspectCommand())

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
		Long: `List all API keys in an organization.

Output includes key ID, associated user, fingerprint, status, and expiration.

Examples:
  # List in table format (default)
  ai-aas-cli apikey list --org-id acme

  # List in JSON format
  ai-aas-cli apikey list --org-id acme --format json

  # List in CSV format for audit
  ai-aas-cli apikey list --org-id acme --format csv > keys.csv

Note:
  API key secrets are never shown in list output for security reasons.
  Only the fingerprint (partial hash) is displayed.

See Also:
  ai-aas-cli apikey create    Create a new API key
  ai-aas-cli user list        List users who own keys`,
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

	// Use default org if not specified
	orgID := flagOrgID
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, or set a default with 'ai-aas-cli org use <org-id>'",
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
	apiKeys, err := userOrgClient.ListAPIKeys(cmd.Context(), orgID)
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
		Command:     fmt.Sprintf("apikey list --org-id=%s", orgID),
		Outcome:     "success",
		Duration:    time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(apiKeys)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"keyId", "notes", "fingerprint", "status", "expiresAt"}
		var rows [][]string
		for _, key := range apiKeys {
			rows = append(rows, []string{
				key.KeyID,
				key.Notes,
				key.Fingerprint,
				key.Status,
				key.ExpiresAt,
			})
		}
		return output.PrintTable(headers, rows)
	} else {
		headers := []string{"Key ID", "Notes", "Fingerprint", "Status", "Expires At"}
		var rows [][]string
		for _, key := range apiKeys {
			rows = append(rows, []string{
				key.KeyID,
				key.Notes,
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
	var flagEmail string
	var flagScopes []string
	var flagPlatformAdmin bool
	var flagExpiresInDays int
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string
	var flagProfile string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create API key",
		Long: `Create an API key for a user.

The key secret is only displayed once. Store it securely immediately after
creation - it cannot be retrieved later.

Examples:
  # Create a key using email (never expires)
  ai-aas-cli apikey create --org-id acme --email user@example.com

  # Create a key using user ID
  ai-aas-cli apikey create --org-id acme --user-id u_123

  # Create with 90-day expiration
  ai-aas-cli apikey create --org-id acme --email user@example.com --expires-in-days 90

  # Create with specific scopes
  ai-aas-cli apikey create --org-id acme --email user@example.com \
    --scopes inference:read,inference:write

  # Create platform admin key (cross-tenant access)
  ai-aas-cli apikey create --org-id platform-admin --email admin@company.com --platform-admin

  # Output in JSON for automation
  ai-aas-cli apikey create --org-id acme --email user@example.com --format json

  # Create key and save to profile (uses org_id/user_id from profile)
  ai-aas-cli apikey create --profile acme-admin

Platform Admin Keys:
  - Use --platform-admin to create keys with 'admin' scope
  - Admin scope grants cross-tenant access (can manage users/keys in any org)
  - Required for system administrators who need to manage multiple orgs

Security Best Practices:
  - Set expiration for production keys
  - Rotate keys regularly
  - Use minimal scopes
  - Only use --platform-admin for trusted system administrators

See Also:
  ai-aas-cli apikey list      List existing keys
  ai-aas-cli apikey delete    Revoke a key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPIKeyCreate(cmd, args, flagOrgID, flagUserID, flagEmail, flagScopes, flagPlatformAdmin, flagExpiresInDays,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey, flagProfile)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID (required if --email not provided)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email (required if --user-id not provided)")
	cmd.Flags().StringSliceVar(&flagScopes, "scopes", []string{}, "API key scopes")
	cmd.Flags().BoolVar(&flagPlatformAdmin, "platform-admin", false, "Create platform admin key with cross-tenant access (adds 'admin' scope)")
	cmd.Flags().IntVar(&flagExpiresInDays, "expires-in-days", 0, "Expiration in days (0 = no expiration)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Use org_id/user_id from profile and save api_key to it")

	return cmd
}

func runAPIKeyCreate(cmd *cobra.Command, args []string, flagOrgID, flagUserID, flagEmail string, flagScopes []string, flagPlatformAdmin bool, flagExpiresInDays int,
	flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string, flagProfile string) error {
	startTime := time.Now()

	// Load configuration with profile support
	cfg, _, err := config.GetEffectiveConfig(flagProfile)
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

	// Use profile org_id/user_id if --profile is set
	orgID := flagOrgID
	resolvedUserID := flagUserID
	emailToResolve := flagEmail
	if flagProfile != "" {
		if profile, err := config.GetProfile(flagProfile); err == nil {
			if orgID == "" && profile.OrgID != "" {
				orgID = profile.OrgID
			}
			if resolvedUserID == "" && emailToResolve == "" && profile.UserID != "" {
				resolvedUserID = profile.UserID
			}
		}
	}

	// Fall back to default org
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, set a default with 'ai-aas-cli org use <org-id>', or use --profile",
		)
	}
	if resolvedUserID == "" && emailToResolve == "" {
		return errors.NewValidationError(
			"either --user-id or --email is required",
			"Provide user ID with --user-id or email with --email flag. API keys must be associated with a user.",
		)
	}

	// Create HTTP client with TLS config (reused for all operations)
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
	}

	// Health check
	checker := createHealthCheckerWithHTTPClient(httpClient)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

	// Resolve user ID from email if needed
	// Also handle case where user provides email in --user-id flag
	// If --user-id looks like an email (contains @), treat it as email
	if resolvedUserID != "" && strings.Contains(resolvedUserID, "@") {
		emailToResolve = resolvedUserID
		resolvedUserID = ""
	}

	if resolvedUserID == "" && emailToResolve != "" {
		user, err := userOrgClient.GetUserByEmail(cmd.Context(), orgID, emailToResolve)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to find user by email: %v", err),
				"Verify the email exists in this organization.",
			)
		}
		resolvedUserID = user.UserID
	}

	// Build request
	scopes := flagScopes
	if flagPlatformAdmin {
		// Add 'admin' scope for platform admin keys
		// This grants cross-tenant access (bypasses requireOrgAccess checks)
		scopes = append(scopes, "admin")
	}

	req := userorg.IssueAPIKeyRequest{
		Scopes: scopes,
	}
	if flagExpiresInDays > 0 {
		req.ExpiresInDays = &flagExpiresInDays
	}

	// Execute create
	apiKey, err := userOrgClient.IssueUserAPIKey(cmd.Context(), orgID, resolvedUserID, req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create API key: %v", err),
			"Verify your API key is valid and you have permission to create API keys.",
		)
	}

	// Audit logging (mask token)
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "apikey_create",
		Command:     fmt.Sprintf("apikey create --org-id=%s --user-id=%s", orgID, resolvedUserID),
		Parameters: map[string]interface{}{
			"orgId":   orgID,
			"userId":  resolvedUserID,
			"email":   emailToResolve,
			"keyId":   apiKey.KeyID,
			"token":   apiKey.Token, // Will be masked by audit logger
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Save to profile if --profile flag is set
	if flagProfile != "" {
		if err := config.UpdateProfile(flagProfile, func(p *config.Profile) {
			p.APIKey = apiKey.Token
		}); err != nil {
			if !cfg.Quiet {
				fmt.Printf("Warning: failed to update profile '%s': %v\n", flagProfile, err)
			}
		} else if !cfg.Quiet {
			fmt.Printf("\nProfile '%s' is now complete!\n", flagProfile)
			if profile, err := config.GetProfile(flagProfile); err == nil {
				fmt.Printf("  environment: %s\n", profile.Environment)
				fmt.Printf("  org_id:      %s\n", profile.OrgID)
				fmt.Printf("  user_id:     %s\n", profile.UserID)
				fmt.Printf("  email:       %s\n", profile.Email)
				fmt.Printf("  api_key:     %s\n", config.MaskSecret(profile.APIKey))
				fmt.Println()
				fmt.Printf("Switch to this profile: ai-aas-cli profile use %s\n", flagProfile)
			}
		}
	}

	// Format output - show token only once
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(apiKey)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"keyId", "token", "fingerprint", "status", "expiresAt"}
		rows := [][]string{{
			apiKey.KeyID,
			apiKey.Token,
			apiKey.Fingerprint,
			apiKey.Status,
			apiKey.ExpiresAt,
		}}
		return output.PrintTable(headers, rows)
	} else {
		if !cfg.Quiet {
			fmt.Println("⚠️  IMPORTANT: Save this API key now. It will not be shown again.")
			fmt.Printf("API key created successfully:\n")
			fmt.Printf("  Key ID: %s\n", apiKey.KeyID)
			fmt.Printf("  Token:  %s\n", apiKey.Token)
			fmt.Printf("  Fingerprint: %s\n", apiKey.Fingerprint)
			fmt.Printf("  Status: %s\n", apiKey.Status)
			if apiKey.ExpiresAt != "" {
				fmt.Printf("  Expires At: %s\n", apiKey.ExpiresAt)
			}
		}
		if cfg.OutputFormat == "table" {
			headers := []string{"Key ID", "Token", "Fingerprint", "Status", "Expires At"}
			rows := [][]string{{
				apiKey.KeyID,
				apiKey.Token,
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
		Long: `Revoke an API key.

Once deleted, the key immediately stops working. This cannot be undone.
Any applications using this key will lose access.

Examples:
  # Delete with confirmation
  ai-aas-cli apikey delete --org-id acme --api-key-id ak_123 --confirm

  # Force delete (for scripts)
  ai-aas-cli apikey delete --org-id acme --api-key-id ak_123 --force

  # Delete in quiet mode
  ai-aas-cli apikey delete --org-id acme --api-key-id ak_123 --force --quiet

Warning:
  - Deletion takes effect immediately
  - Applications using this key will get authentication errors
  - Consider creating a replacement key before deleting

See Also:
  ai-aas-cli apikey list      Find the key ID to delete
  ai-aas-cli apikey create    Create a replacement key`,
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

	// Use default org if not specified
	orgID := flagOrgID
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, or set a default with 'ai-aas-cli org use <org-id>'",
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
	if err := userOrgClient.DeleteAPIKey(cmd.Context(), orgID, flagAPIKeyID); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to delete API key: %v", err),
			"Verify your API key is valid and the API key exists.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "apikey_delete",
		Command:     fmt.Sprintf("apikey delete --org-id=%s --api-key-id=%s --confirm", orgID, flagAPIKeyID),
		Parameters: map[string]interface{}{
			"orgId":    orgID,
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

func apiKeyInspectCommand() *cobra.Command {
	var flagKey string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "inspect [key]",
		Short: "Inspect API key details",
		Long: `Inspect an API key and show its details.

This command accepts an API key secret and returns detailed information about
the key, including which organization it belongs to, the principal (user or
service account), scopes, status, and expiration.

Useful for:
  - Debugging authentication issues
  - Verifying key ownership and permissions
  - Checking key status and expiration
  - Support and troubleshooting scenarios

Examples:
  # Inspect a key (positional argument)
  ai-aas-cli apikey inspect ai-aas_xxxxxxxxxxxxx

  # Inspect using --key flag
  ai-aas-cli apikey inspect --key ai-aas_xxxxxxxxxxxxx

  # Output in JSON format
  ai-aas-cli apikey inspect ai-aas_xxxxxxxxxxxxx --format json

  # Output in table format (default)
  ai-aas-cli apikey inspect ai-aas_xxxxxxxxxxxxx --format table

Security:
  - This operation is logged for audit purposes
  - Requires admin or apikey:manage scope
  - The key secret itself is never returned in the response

Output Fields:
  - Key ID: Unique identifier for the key
  - Organization: Which org the key belongs to (ID and slug)
  - Principal: User or service account that owns the key
  - Status: active, revoked, or expired
  - Scopes: Permissions granted to the key
  - Created: When the key was created
  - Expires: When the key expires (if set)
  - Last Used: Last time the key was used (if tracked)

See Also:
  ai-aas-cli apikey list      List all keys in an organization
  ai-aas-cli apikey create    Create a new API key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := flagKey
			if key == "" && len(args) > 0 {
				key = args[0]
			}
			return runAPIKeyInspect(cmd, args, key, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagKey, "key", "", "API key to inspect")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runAPIKeyInspect(cmd *cobra.Command, args []string, key, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
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

	// Validate key argument
	if key == "" {
		return errors.NewValidationError(
			"API key is required",
			"Provide the API key to inspect as a positional argument or with --key flag",
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

	// Create client and inspect API key
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	keyDetails, err := userOrgClient.InspectAPIKey(cmd.Context(), key)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to inspect API key: %v", err),
			"Verify the key is valid and you have permission to inspect API keys.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:     "apikey_inspect",
		Command:  "apikey inspect",
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(keyDetails)
	} else {
		// Table format
		if !cfg.Quiet {
			fmt.Println("API Key Details")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Printf("Key ID:         %s\n", keyDetails.KeyID)
			fmt.Printf("Organization:   %s (%s)\n", keyDetails.OrgSlug, keyDetails.OrgID)
			fmt.Printf("Principal:      %s (%s)\n", keyDetails.PrincipalType, keyDetails.PrincipalID)
			fmt.Printf("Status:         %s\n", keyDetails.Status)
			fmt.Printf("Scopes:         %s\n", strings.Join(keyDetails.Scopes, ", "))
			fmt.Printf("Created:        %s\n", keyDetails.CreatedAt)
			if keyDetails.ExpiresAt != "" {
				fmt.Printf("Expires:        %s\n", keyDetails.ExpiresAt)
			} else {
				fmt.Printf("Expires:        Never\n")
			}
			if keyDetails.LastUsedAt != "" {
				fmt.Printf("Last Used:      %s\n", keyDetails.LastUsedAt)
			} else {
				fmt.Printf("Last Used:      Never\n")
			}
		}
	}
	return nil
}


