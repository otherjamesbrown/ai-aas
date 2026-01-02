// Package commands provides budget management commands.
//
// Purpose:
//
//	Budget lifecycle commands: list and set for organizations.
//	Supports structured output and audit logging.
//
// Requirements Reference:
//   - specs/019-admin-cli-enhancements/spec.md#US-001 (Budget Management)
package admin

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// BudgetCommand creates the budget command group.
//
// DEPRECATED: The monetary budget system has been replaced by token rate-limit policies.
// Use the ai-aas-org CLI for token policy management:
//
//	ai-aas-org token-policy list
//	ai-aas-org token-policy create --name "Standard" --1h 10000 --24h 100000
//	ai-aas-org token-policy set-default --policy "Standard"
func BudgetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "budget",
		Short:      "[DEPRECATED] Manage organization budgets",
		Long:       "View and configure organization budget limits\n\nDEPRECATED: Use 'ai-aas-org token-policy' for token rate-limit management instead.",
		Deprecated: "The monetary budget system has been replaced by token rate-limit policies.\nUse 'ai-aas-org token-policy list|create|set-default' for rate limiting.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.ErrOrStderr(), "")
			fmt.Fprintln(cmd.ErrOrStderr(), "╭──────────────────────────────────────────────────────────────────────────────╮")
			fmt.Fprintln(cmd.ErrOrStderr(), "│ DEPRECATED: The 'budget' command is deprecated.                             │")
			fmt.Fprintln(cmd.ErrOrStderr(), "│                                                                              │")
			fmt.Fprintln(cmd.ErrOrStderr(), "│ Token rate limiting is now managed with 'ai-aas-org token-policy':          │")
			fmt.Fprintln(cmd.ErrOrStderr(), "│   ai-aas-org token-policy list                                              │")
			fmt.Fprintln(cmd.ErrOrStderr(), "│   ai-aas-org token-policy create --name \"Standard\" --1h 10000              │")
			fmt.Fprintln(cmd.ErrOrStderr(), "│   ai-aas-org token-policy set-default --policy \"Standard\"                  │")
			fmt.Fprintln(cmd.ErrOrStderr(), "╰──────────────────────────────────────────────────────────────────────────────╯")
			fmt.Fprintln(cmd.ErrOrStderr(), "")
		},
	}

	cmd.AddCommand(budgetListCommand())
	cmd.AddCommand(budgetSetCommand())
	// budgetAlertsCommand() - Phase 2, requires new backend endpoint

	return cmd
}

func budgetListCommand() *cobra.Command {
	var flagOrgID string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List budget configuration",
		Long:  "Display current budget configuration and usage for an organization",
		Example: `  # List budget for an organization
  admin-cli budget list --org-id acme-corp

  # Output as JSON
  admin-cli budget list --org-id acme-corp --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBudgetList(cmd, args, flagOrgID, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
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

func runBudgetList(cmd *cobra.Command, args []string, flagOrgID, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
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

	// Create client and get budget info
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)

	// Get organization details (includes budget policy ID)
	org, err := userOrgClient.GetOrg(cmd.Context(), flagOrgID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to get organization: %v", err),
			"Verify the organization ID is correct and you have permission to view it.",
		)
	}

	// Get budget status
	budgetStatus, err := userOrgClient.GetBudgetStatus(cmd.Context(), org.OrgID)
	if err != nil {
		// Budget status endpoint may not exist yet - show org info with warning
		if cfg.Verbose && !cfg.Quiet {
			fmt.Printf("Warning: Could not retrieve budget status: %v\n", err)
		}
		budgetStatus = &userorg.BudgetStatusResponse{
			OrgID:             org.OrgID,
			BudgetLimitCents:  0,
			CurrentUsageCents: 0,
			RemainingCents:    0,
			Status:            "unknown",
		}
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:     "budget_list",
		Command:  fmt.Sprintf("budget list --org-id=%s", flagOrgID),
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Build combined response
	result := map[string]interface{}{
		"orgId":             org.OrgID,
		"orgName":           org.Name,
		"budgetLimitCents":  budgetStatus.BudgetLimitCents,
		"currentUsageCents": budgetStatus.CurrentUsageCents,
		"remainingCents":    budgetStatus.RemainingCents,
		"status":            budgetStatus.Status,
		"periodStart":       budgetStatus.PeriodStart,
		"periodEnd":         budgetStatus.PeriodEnd,
	}

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(result)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"orgId", "orgName", "budgetLimitCents", "currentUsageCents", "remainingCents", "status"}
		rows := [][]string{{
			org.OrgID,
			org.Name,
			fmt.Sprintf("%d", budgetStatus.BudgetLimitCents),
			fmt.Sprintf("%d", budgetStatus.CurrentUsageCents),
			fmt.Sprintf("%d", budgetStatus.RemainingCents),
			budgetStatus.Status,
		}}
		return output.PrintTable(headers, rows)
	} else {
		// Table format with human-readable output
		if !cfg.Quiet {
			fmt.Printf("Organization: %s (%s)\n", org.Name, org.OrgID)
			fmt.Println("─────────────────────────────────")
			if budgetStatus.BudgetLimitCents > 0 {
				fmt.Printf("Budget Limit:   $%.2f\n", float64(budgetStatus.BudgetLimitCents)/100)
				fmt.Printf("Current Usage:  $%.2f\n", float64(budgetStatus.CurrentUsageCents)/100)
				fmt.Printf("Remaining:      $%.2f\n", float64(budgetStatus.RemainingCents)/100)
				fmt.Printf("Status:         %s\n", budgetStatus.Status)
				if budgetStatus.PeriodStart != "" {
					fmt.Printf("Period:         %s to %s\n", budgetStatus.PeriodStart, budgetStatus.PeriodEnd)
				}
			} else {
				fmt.Println("No budget limit configured")
			}
		}
		return nil
	}
}

func budgetSetCommand() *cobra.Command {
	var flagOrgID string
	var flagMonthlyLimit int64
	var flagDryRun bool
	var flagConfirm bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set budget limit",
		Long:  "Configure the monthly budget limit for an organization",
		Example: `  # Preview budget change (dry-run)
  admin-cli budget set --org-id acme-corp --monthly-limit 100000 --dry-run

  # Apply budget change
  admin-cli budget set --org-id acme-corp --monthly-limit 100000 --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBudgetSet(cmd, args, flagOrgID, flagMonthlyLimit, flagDryRun, flagConfirm,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().Int64Var(&flagMonthlyLimit, "monthly-limit", 0, "Monthly budget limit in cents (required)")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without applying them")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Confirm the budget change")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runBudgetSet(cmd *cobra.Command, args []string, flagOrgID string, flagMonthlyLimit int64, flagDryRun, flagConfirm bool,
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

	if flagMonthlyLimit < 0 {
		return errors.NewValidationError(
			"--monthly-limit must be non-negative",
			"Provide a positive budget limit in cents (e.g., 100000 for $1000)",
		)
	}

	// Confirmation check (non-dry-run mode)
	if !flagDryRun && !flagConfirm {
		return errors.NewValidationError(
			"confirmation required for budget change",
			"Use --dry-run to preview changes or --confirm to apply them.",
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

	// Create client
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)

	// Get current organization details
	org, err := userOrgClient.GetOrg(cmd.Context(), flagOrgID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to get organization: %v", err),
			"Verify the organization ID is correct and you have permission to view it.",
		)
	}

	// Get current budget status for comparison
	currentBudget, _ := userOrgClient.GetBudgetStatus(cmd.Context(), org.OrgID)

	// Dry-run mode: show what would change
	if flagDryRun {
		if !cfg.Quiet {
			fmt.Println("DRY RUN - No changes will be made")
			fmt.Println("─────────────────────────────────")
			fmt.Printf("Organization: %s (%s)\n", org.Name, org.OrgID)
			fmt.Printf("Current Limit: $%.2f\n", float64(currentBudget.BudgetLimitCents)/100)
			fmt.Printf("New Limit:     $%.2f\n", float64(flagMonthlyLimit)/100)
		}
		if cfg.OutputFormat == "json" {
			return output.PrintJSON(map[string]interface{}{
				"dryRun":       true,
				"orgId":        org.OrgID,
				"currentLimit": currentBudget.BudgetLimitCents,
				"newLimit":     flagMonthlyLimit,
			})
		}
		return nil
	}

	// Apply budget change
	// Note: Currently updates via PATCH /v1/orgs/{id} with budgetPolicyId
	// The budget limit may need to be set via a separate budget policy endpoint
	budgetPolicyID := fmt.Sprintf("budget-%d", flagMonthlyLimit) // Simplified - real impl would use budget policy service
	updateReq := userorg.UpdateOrgRequest{
		BudgetPolicyID: &budgetPolicyID,
	}

	_, err = userOrgClient.UpdateOrg(cmd.Context(), org.OrgID, updateReq)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to update budget: %v", err),
			"Verify you have permission to update organization settings.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:    "budget_set",
		Command: fmt.Sprintf("budget set --org-id=%s --monthly-limit=%d --confirm", flagOrgID, flagMonthlyLimit),
		Parameters: map[string]interface{}{
			"orgId":         org.OrgID,
			"previousLimit": currentBudget.BudgetLimitCents,
			"newLimit":      flagMonthlyLimit,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	result := map[string]interface{}{
		"success":       true,
		"orgId":         org.OrgID,
		"previousLimit": currentBudget.BudgetLimitCents,
		"newLimit":      flagMonthlyLimit,
	}

	if cfg.OutputFormat == "json" {
		return output.PrintJSON(result)
	} else {
		if !cfg.Quiet {
			fmt.Printf("Budget updated successfully for organization: %s\n", org.Name)
			fmt.Printf("  Previous Limit: $%.2f\n", float64(currentBudget.BudgetLimitCents)/100)
			fmt.Printf("  New Limit:      $%.2f\n", float64(flagMonthlyLimit)/100)
		}
	}
	return nil
}
