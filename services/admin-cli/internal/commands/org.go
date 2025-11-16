// Package commands provides org management commands.
//
// Purpose:
//
//	Organization lifecycle commands: list, create, update, delete with batch operations,
//	dry-run, file input, and structured output.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-002 (Day-2 Management)
//   - specs/009-admin-cli/spec.md#FR-002 (batch operations)
//   - specs/009-admin-cli/spec.md#FR-006 (structured output)
//
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

// OrgCommand creates the org command group.
func OrgCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage organizations",
		Long:  "Manage organizations: list, create, update, delete",
	}

	cmd.AddCommand(orgListCommand())
	cmd.AddCommand(orgCreateCommand())
	cmd.AddCommand(orgUpdateCommand())
	cmd.AddCommand(orgDeleteCommand())

	return cmd
}

func orgListCommand() *cobra.Command {
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List organizations",
		Long:  "List all organizations with structured output (table, json, csv)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgList(cmd, args, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runOrgList(cmd *cobra.Command, args []string, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		cliErr := errors.NewOperationError(
			fmt.Sprintf("failed to load configuration: %v", err),
			"Check your configuration file or environment variables.",
		)
		return cliErr
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

	// Health check
	checker := health.NewChecker(5 * time.Second)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	// Create client and list orgs
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	orgs, err := userOrgClient.ListOrgs(cmd.Context())
	if err != nil {
		cliErr := errors.NewOperationError(
			fmt.Sprintf("failed to list organizations: %v", err),
			"Verify your API key is valid and you have permission to list organizations.",
		)
		return cliErr
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "org_list",
		Command:     "org list",
		Outcome:     "success",
		Duration:    time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(orgs)
	} else if cfg.OutputFormat == "csv" {
		// Convert to CSV format
		headers := []string{"orgId", "name", "slug", "status", "createdAt"}
		var rows [][]string
		for _, org := range orgs {
			rows = append(rows, []string{
				org.OrgID,
				org.Name,
				org.Slug,
				org.Status,
				org.CreatedAt,
			})
		}
		return output.PrintTable(headers, rows)
	} else {
		// Table format (default)
		headers := []string{"Org ID", "Name", "Slug", "Status", "Created At"}
		var rows [][]string
		for _, org := range orgs {
			rows = append(rows, []string{
				org.OrgID,
				org.Name,
				org.Slug,
				org.Status,
				org.CreatedAt,
			})
		}
		if len(rows) == 0 && !cfg.Quiet {
			fmt.Println("No organizations found.")
			return nil
		}
		return output.PrintTable(headers, rows)
	}
}

func orgCreateCommand() *cobra.Command {
	var flagName string
	var flagSlug string
	var flagBillingOwnerEmail string
	var flagDeclarativeEnabled bool
	var flagDeclarativeRepoURL string
	var flagDeclarativeBranch string
	var flagDryRun bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create organization",
		Long:  "Create a new organization with dry-run support",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgCreate(cmd, args, flagName, flagSlug, flagBillingOwnerEmail,
				flagDeclarativeEnabled, flagDeclarativeRepoURL, flagDeclarativeBranch,
				flagDryRun, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagName, "name", "", "Organization name (required)")
	cmd.Flags().StringVar(&flagSlug, "slug", "", "Organization slug (required)")
	cmd.Flags().StringVar(&flagBillingOwnerEmail, "billing-owner-email", "", "Billing owner email")
	cmd.Flags().BoolVar(&flagDeclarativeEnabled, "declarative-enabled", false, "Enable declarative GitOps")
	cmd.Flags().StringVar(&flagDeclarativeRepoURL, "declarative-repo-url", "", "Declarative repo URL")
	cmd.Flags().StringVar(&flagDeclarativeBranch, "declarative-branch", "", "Declarative branch")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runOrgCreate(cmd *cobra.Command, args []string, flagName, flagSlug, flagBillingOwnerEmail string,
	flagDeclarativeEnabled bool, flagDeclarativeRepoURL, flagDeclarativeBranch string,
	flagDryRun bool, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
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
	if flagName == "" {
		return errors.NewValidationError(
			"--name is required",
			"Provide organization name with --name flag",
		)
	}
	if flagSlug == "" {
		return errors.NewValidationError(
			"--slug is required",
			"Provide organization slug with --slug flag",
		)
	}

	// Health check (only if not dry-run)
	if !flagDryRun {
		checker := health.NewChecker(5 * time.Second)
		requiredServices := map[string]string{
			"user-org-service": cfg.UserOrgEndpoint,
		}
		if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
			return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
		}
	}

	// Build request
	req := userorg.CreateOrgRequest{
		Name:              flagName,
		Slug:              flagSlug,
		BillingOwnerEmail: flagBillingOwnerEmail,
	}
	if flagDeclarativeEnabled {
		req.Declarative = &userorg.DeclarativeConfig{
			Enabled: true,
			RepoURL: flagDeclarativeRepoURL,
			Branch:  flagDeclarativeBranch,
		}
	}

	// Dry-run mode
	if flagDryRun {
		if !cfg.Quiet {
			fmt.Println("DRY-RUN MODE: Preview of changes")
			fmt.Println("============================================================")
			fmt.Println("Operation: Create organization")
			fmt.Println("Name:", flagName)
			fmt.Println("Slug:", flagSlug)
			if flagBillingOwnerEmail != "" {
				fmt.Println("Billing Owner Email:", flagBillingOwnerEmail)
			}
			if flagDeclarativeEnabled {
				fmt.Println("Declarative: Enabled")
				if flagDeclarativeRepoURL != "" {
					fmt.Println("  Repo URL:", flagDeclarativeRepoURL)
				}
				if flagDeclarativeBranch != "" {
					fmt.Println("  Branch:", flagDeclarativeBranch)
				}
			}
			fmt.Println("\nUse without --dry-run to execute")
		}

		if cfg.OutputFormat == "json" {
			return output.PrintJSON(map[string]interface{}{
				"mode":      "dry-run",
				"operation": "org_create",
				"request":   req,
			})
		}
		return nil
	}

	// Execute create
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	org, err := userOrgClient.CreateOrg(cmd.Context(), req)
	if err != nil {
		cliErr := errors.NewOperationError(
			fmt.Sprintf("failed to create organization: %v", err),
			"Verify your API key is valid and you have permission to create organizations.",
		)
		return cliErr
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "org_create",
		Command:     fmt.Sprintf("org create --name=%s --slug=%s", flagName, flagSlug),
		Parameters: map[string]interface{}{
			"name":  flagName,
			"slug":  flagSlug,
			"orgId": org.OrgID,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(org)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"orgId", "name", "slug", "status", "createdAt"}
		rows := [][]string{{
			org.OrgID,
			org.Name,
			org.Slug,
			org.Status,
			org.CreatedAt,
		}}
		return output.PrintTable(headers, rows)
	} else {
		if !cfg.Quiet {
			fmt.Printf("Organization created successfully:\n")
			fmt.Printf("  Org ID: %s\n", org.OrgID)
			fmt.Printf("  Name: %s\n", org.Name)
			fmt.Printf("  Slug: %s\n", org.Slug)
			fmt.Printf("  Status: %s\n", org.Status)
		}
		if cfg.OutputFormat == "table" {
			headers := []string{"Org ID", "Name", "Slug", "Status", "Created At"}
			rows := [][]string{{
				org.OrgID,
				org.Name,
				org.Slug,
				org.Status,
				org.CreatedAt,
			}}
			return output.PrintTable(headers, rows)
		}
	}
	return nil
}

func orgUpdateCommand() *cobra.Command {
	var flagOrgID string
	var flagFile string
	var flagDisplayName string
	var flagStatus string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update organization",
		Long:  "Update an organization with file input (JSON/YAML) support",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgUpdate(cmd, args, flagOrgID, flagFile, flagDisplayName, flagStatus,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagFile, "file", "", "File path (JSON/YAML) containing update data")
	cmd.Flags().StringVar(&flagDisplayName, "display-name", "", "Organization display name")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Organization status (active, suspended)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runOrgUpdate(cmd *cobra.Command, args []string, flagOrgID, flagFile, flagDisplayName, flagStatus string,
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

	// Build update request
	req := userorg.UpdateOrgRequest{}

	// Load from file if provided
	if flagFile != "" {
		data, err := os.ReadFile(flagFile)
		if err != nil {
			return errors.NewValidationError(
				fmt.Sprintf("failed to read file: %v", err),
				"Verify the file path is correct and readable.",
			)
		}

		// Try JSON first, then YAML
		if err := json.Unmarshal(data, &req); err != nil {
			if err := yaml.Unmarshal(data, &req); err != nil {
				return errors.NewValidationError(
					fmt.Sprintf("failed to parse file: %v", err),
					"File must be valid JSON or YAML format.",
				)
			}
		}
	} else {
		// Use flags
		if flagDisplayName != "" {
			req.DisplayName = &flagDisplayName
		}
		if flagStatus != "" {
			req.Status = &flagStatus
		}
	}

	// Validate at least one field to update
	if req.DisplayName == nil && req.Status == nil && req.BudgetPolicyID == nil && req.Declarative == nil && req.Metadata == nil {
		return errors.NewValidationError(
			"no fields to update",
			"Provide at least one field to update via --file or flags (--display-name, --status).",
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

	// Execute update
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	org, err := userOrgClient.UpdateOrg(cmd.Context(), flagOrgID, req)
	if err != nil {
		cliErr := errors.NewOperationError(
			fmt.Sprintf("failed to update organization: %v", err),
			"Verify your API key is valid and the organization exists.",
		)
		return cliErr
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "org_update",
		Command:     fmt.Sprintf("org update --org-id=%s", flagOrgID),
		Parameters: map[string]interface{}{
			"orgId": flagOrgID,
			"request": req,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(org)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"orgId", "name", "slug", "status", "updatedAt"}
		rows := [][]string{{
			org.OrgID,
			org.Name,
			org.Slug,
			org.Status,
			org.UpdatedAt,
		}}
		return output.PrintTable(headers, rows)
	} else {
		if !cfg.Quiet {
			fmt.Printf("Organization updated successfully:\n")
			fmt.Printf("  Org ID: %s\n", org.OrgID)
			fmt.Printf("  Name: %s\n", org.Name)
			fmt.Printf("  Slug: %s\n", org.Slug)
			fmt.Printf("  Status: %s\n", org.Status)
		}
		if cfg.OutputFormat == "table" {
			headers := []string{"Org ID", "Name", "Slug", "Status", "Updated At"}
			rows := [][]string{{
				org.OrgID,
				org.Name,
				org.Slug,
				org.Status,
				org.UpdatedAt,
			}}
			return output.PrintTable(headers, rows)
		}
	}
	return nil
}

func orgDeleteCommand() *cobra.Command {
	var flagOrgID string
	var flagConfirm bool
	var flagForce bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete organization",
		Long:  "Delete an organization with confirmation and force flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgDelete(cmd, args, flagOrgID, flagConfirm, flagForce,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Confirm deletion (required unless --force)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Force deletion without confirmation prompt")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runOrgDelete(cmd *cobra.Command, args []string, flagOrgID string, flagConfirm, flagForce bool,
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

	// Get org details for confirmation display
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	var orgName string
	org, err := userOrgClient.GetOrg(cmd.Context(), flagOrgID)
	if err == nil {
		orgName = org.Name
	}

	// Show confirmation warning (unless forced or quiet)
	if !flagForce && !cfg.Quiet {
		fmt.Printf("⚠️  WARNING: This will delete organization: %s\n", flagOrgID)
		if orgName != "" {
			fmt.Printf("   Name: %s\n", orgName)
		}
		fmt.Println("   This action cannot be undone.")
	}

	// Execute delete
	if err := userOrgClient.DeleteOrg(cmd.Context(), flagOrgID); err != nil {
		cliErr := errors.NewOperationError(
			fmt.Sprintf("failed to delete organization: %v", err),
			"Verify your API key is valid and the organization exists.",
		)
		return cliErr
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "org_delete",
		Command:     fmt.Sprintf("org delete --org-id=%s --confirm", flagOrgID),
		Parameters: map[string]interface{}{
			"orgId": flagOrgID,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"success": true,
			"orgId":   flagOrgID,
			"message": "Organization deleted successfully",
		})
	} else {
		if !cfg.Quiet {
			fmt.Printf("Organization deleted successfully: %s\n", flagOrgID)
		}
	}
	return nil
}

