// Package admin provides bootstrap key management commands.
//
// Purpose:
//
//	Bootstrap key lifecycle commands for onboarding organization administrators.
//	Bootstrap keys are one-time use tokens that allow org admins to initialize
//	their ai-aas-org CLI without requiring platform admin involvement.
//
// Requirements Reference:
//   - specs/033-org-admin-cli/spec.md (Org admin CLI onboarding)
//
package admin

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// BootstrapKeyCommand creates the bootstrap-key command group.
func BootstrapKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bootstrap-key",
		Aliases: []string{"bsk"},
		Short:   "Manage bootstrap keys for org admin onboarding",
		Long: `Manage bootstrap keys for organization administrator onboarding.

Bootstrap keys are one-time use tokens that allow organization administrators
to initialize their ai-aas-org CLI. Keys have a 7-day expiry by default.

Workflow:
  1. Create a bootstrap key for an org   ai-aas-cli bootstrap-key create --org-id <org>
  2. Share the key with the org admin    (securely, e.g., password manager)
  3. Org admin runs                       ai-aas-org init --key bsk_xxxxx

Key Lifecycle:
  active    - Key is valid and can be redeemed
  redeemed  - Key has been used to initialize ai-aas-org
  expired   - Key has passed its expiry date
  revoked   - Key has been manually revoked

Examples:
  # Create a bootstrap key for an organization
  ai-aas-cli bootstrap-key create --org-id acme

  # List all bootstrap keys
  ai-aas-cli bootstrap-key list

  # List keys for a specific org
  ai-aas-cli bootstrap-key list --org-id acme

  # Revoke a key
  ai-aas-cli bootstrap-key revoke --key-id bsk_abc123`,
	}

	cmd.AddCommand(bootstrapKeyListCommand())
	cmd.AddCommand(bootstrapKeyCreateCommand())
	cmd.AddCommand(bootstrapKeyRevokeCommand())

	return cmd
}

func bootstrapKeyListCommand() *cobra.Command {
	var flagFormat string
	var flagOrgID string
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bootstrap keys",
		Long: `List bootstrap keys, optionally filtered by organization.

Shows key ID, organization, status, creation date, and expiry.

Examples:
  # List all bootstrap keys
  ai-aas-cli bootstrap-key list

  # List keys for a specific organization
  ai-aas-cli bootstrap-key list --org-id acme

  # Output as JSON
  ai-aas-cli bootstrap-key list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrapKeyList(cmd, flagFormat, flagOrgID, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Filter by organization ID or slug")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runBootstrapKeyList(cmd *cobra.Command, flagFormat, flagOrgID, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	profileName := cmd.Root().PersistentFlags().Lookup("profile").Value.String()
	cfg, _, err := config.GetEffectiveConfig(profileName)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Apply overrides
	if flagUserOrgEndpoint != "" {
		cfg.UserOrgEndpoint = flagUserOrgEndpoint
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if flagFormat != "" {
		cfg.OutputFormat = flagFormat
	}

	// Validate
	if cfg.UserOrgEndpoint == "" {
		return errors.NewValidationError(
			"user-org-service endpoint is required",
			"Set via --user-org-endpoint flag or ADMIN_CLI_USER_ORG_ENDPOINT environment variable",
		)
	}

	// Create client
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create HTTP client: %v", err),
			"Verify the CA certificate file path is correct.",
		)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

	// List keys
	keys, err := userOrgClient.ListBootstrapKeys(cmd.Context(), flagOrgID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list bootstrap keys: %v", err),
			"Verify your API key has permission to manage bootstrap keys.",
		)
	}

	// Audit log
	auditLogger := audit.NewLogger(nil)
	if err := auditLogger.LogOperation(audit.Operation{
		Type:     "bootstrap_key_list",
		Command:  "bootstrap-key list",
		Outcome:  "success",
		Duration: time.Since(startTime),
	}); err != nil {
		output.WarningMsg("Failed to write to audit log: %v", err)
	}

	// Output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(keys)
	}

	if len(keys) == 0 {
		output.InfoMsg("No bootstrap keys found.")
		return nil
	}

	headers := []string{"KEY ID", "ORG", "STATUS", "CREATED", "EXPIRES"}
	var rows [][]string
	for _, key := range keys {
		orgDisplay := key.OrgID
		if key.OrgName != "" {
			orgDisplay = key.OrgName
		}
		// Format dates
		created := formatDate(key.CreatedAt)
		expires := formatDate(key.ExpiresAt)

		rows = append(rows, []string{
			key.KeyID,
			orgDisplay,
			statusBadge(key.Status),
			created,
			expires,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

func bootstrapKeyCreateCommand() *cobra.Command {
	var flagOrgID string
	var flagExpiresInDays int
	var flagNotes string
	var flagFormat string
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a bootstrap key for an organization",
		Long: `Create a bootstrap key for an organization administrator.

The bootstrap key is shown only once - save it securely and share it with
the organization administrator through a secure channel.

The org admin uses this key with:
  ai-aas-org init --key <bootstrap-key>

Examples:
  # Create a key with default 7-day expiry
  ai-aas-cli bootstrap-key create --org-id acme

  # Create a key with 14-day expiry
  ai-aas-cli bootstrap-key create --org-id acme --expires-in-days 14

  # Add a note for tracking
  ai-aas-cli bootstrap-key create --org-id acme --notes "For new admin John"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrapKeyCreate(cmd, flagOrgID, flagExpiresInDays, flagNotes, flagFormat, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().IntVar(&flagExpiresInDays, "expires-in-days", 7, "Key expiry in days")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "Human-readable note about this key")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	_ = cmd.MarkFlagRequired("org-id")

	return cmd
}

func runBootstrapKeyCreate(cmd *cobra.Command, flagOrgID string, flagExpiresInDays int, flagNotes, flagFormat, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	profileName := cmd.Root().PersistentFlags().Lookup("profile").Value.String()
	cfg, _, err := config.GetEffectiveConfig(profileName)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Apply overrides
	if flagUserOrgEndpoint != "" {
		cfg.UserOrgEndpoint = flagUserOrgEndpoint
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}
	if flagFormat != "" {
		cfg.OutputFormat = flagFormat
	}

	// Validate
	if cfg.UserOrgEndpoint == "" {
		return errors.NewValidationError(
			"user-org-service endpoint is required",
			"Set via --user-org-endpoint flag or ADMIN_CLI_USER_ORG_ENDPOINT environment variable",
		)
	}

	// Create client
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create HTTP client: %v", err),
			"Verify the CA certificate file path is correct.",
		)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

	// Create the key
	req := userorg.BootstrapKeyRequest{
		OrgID:         flagOrgID,
		ExpiresInDays: flagExpiresInDays,
		Notes:         flagNotes,
	}

	result, err := userOrgClient.CreateBootstrapKey(cmd.Context(), req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create bootstrap key: %v", err),
			"Verify the organization exists and you have permission to create bootstrap keys.",
		)
	}

	// Audit log
	auditLogger := audit.NewLogger(nil)
	if err := auditLogger.LogOperation(audit.Operation{
		Type:     "bootstrap_key_create",
		Command:  "bootstrap-key create",
		Outcome:  "success",
		Duration: time.Since(startTime),
	}); err != nil {
		output.WarningMsg("Failed to write to audit log: %v", err)
	}

	// Output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(result)
	}

	output.SuccessMsg("Bootstrap key created successfully!")
	fmt.Println()
	output.WarningMsg("IMPORTANT: This key is shown only once. Save it securely.")
	fmt.Println()

	output.KeyValue("Key ID", result.KeyID)
	output.KeyValue("Token", result.Token)
	output.KeyValue("Organization", result.OrgName)
	output.KeyValue("Expires", formatDate(result.ExpiresAt))
	fmt.Println()

	fmt.Println("The org admin should run:")
	fmt.Printf("  ai-aas-org init --key %s\n", result.Token)
	fmt.Println()

	return nil
}

func bootstrapKeyRevokeCommand() *cobra.Command {
	var flagKeyID string
	var flagConfirm bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a bootstrap key",
		Long: `Revoke a bootstrap key, preventing it from being used.

A revoked key cannot be used to initialize ai-aas-org. Use this if a key
was compromised or is no longer needed.

Examples:
  # Revoke a key (with confirmation prompt)
  ai-aas-cli bootstrap-key revoke --key-id bsk_abc123

  # Revoke without confirmation
  ai-aas-cli bootstrap-key revoke --key-id bsk_abc123 --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrapKeyRevoke(cmd, flagKeyID, flagConfirm, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagKeyID, "key-id", "", "Bootstrap key ID to revoke (required)")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	_ = cmd.MarkFlagRequired("key-id")

	return cmd
}

func runBootstrapKeyRevoke(cmd *cobra.Command, flagKeyID string, flagConfirm bool, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	profileName := cmd.Root().PersistentFlags().Lookup("profile").Value.String()
	cfg, _, err := config.GetEffectiveConfig(profileName)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
	}

	// Apply overrides
	if flagUserOrgEndpoint != "" {
		cfg.UserOrgEndpoint = flagUserOrgEndpoint
	}
	if flagAPIKey != "" {
		cfg.APIKey = flagAPIKey
	}

	// Validate
	if cfg.UserOrgEndpoint == "" {
		return errors.NewValidationError(
			"user-org-service endpoint is required",
			"Set via --user-org-endpoint flag or ADMIN_CLI_USER_ORG_ENDPOINT environment variable",
		)
	}

	// Confirm if not using --confirm flag
	if !flagConfirm {
		output.WarningMsg("You are about to revoke bootstrap key: %s", flagKeyID)
		fmt.Print("Type 'REVOKE' to confirm: ")
		var input string
		fmt.Scanln(&input)
		if input != "REVOKE" {
			output.InfoMsg("Revocation cancelled.")
			return nil
		}
	}

	// Create client
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create HTTP client: %v", err),
			"Verify the CA certificate file path is correct.",
		)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

	// Revoke the key
	err = userOrgClient.RevokeBootstrapKey(cmd.Context(), flagKeyID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to revoke bootstrap key: %v", err),
			"Verify the key exists and you have permission to revoke it.",
		)
	}

	// Audit log
	auditLogger := audit.NewLogger(nil)
	if err := auditLogger.LogOperation(audit.Operation{
		Type:     "bootstrap_key_revoke",
		Command:  "bootstrap-key revoke",
		Outcome:  "success",
		Duration: time.Since(startTime),
	}); err != nil {
		output.WarningMsg("Failed to write to audit log: %v", err)
	}

	output.SuccessMsg("Bootstrap key %s has been revoked.", flagKeyID)
	return nil
}

// Helper functions

func formatDate(dateStr string) string {
	if dateStr == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("2006-01-02 15:04")
}

func statusBadge(status string) string {
	return output.StatusBadge(status)
}
