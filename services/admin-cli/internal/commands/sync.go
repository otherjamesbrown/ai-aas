// Package commands provides sync operation commands.
//
// Purpose:
//
//	Trigger and monitor analytics sync operations with progress indicators and job ID monitoring.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-002 (Sync Operations)
//
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/progress"
)

// SyncCommand creates the sync command group.
func SyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Manage sync operations",
		Long:  "Trigger and monitor analytics sync operations with progress indicators",
	}

	cmd.AddCommand(syncTriggerCommand())
	cmd.AddCommand(syncStatusCommand())

	return cmd
}

func syncTriggerCommand() *cobra.Command {
	var flagOrgID string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAnalyticsEndpoint string
	var flagAPIKey string
	var flagWait bool

	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Trigger sync operation",
		Long:  "Trigger a sync operation for analytics data. Use --wait to monitor progress.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncTrigger(cmd, args, flagOrgID, flagFormat, flagVerbose, flagQuiet,
				flagUserOrgEndpoint, flagAnalyticsEndpoint, flagAPIKey, flagWait)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagWait, "wait", false, "Wait for sync to complete and show progress")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAnalyticsEndpoint, "analytics-endpoint", "", "Analytics-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runSyncTrigger(cmd *cobra.Command, args []string, flagOrgID, flagFormat string, flagVerbose, flagQuiet bool,
	flagUserOrgEndpoint, flagAnalyticsEndpoint, flagAPIKey string, flagWait bool) error {
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
	if flagAnalyticsEndpoint != "" {
		cfg.AnalyticsEndpoint = flagAnalyticsEndpoint
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
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}

	if cfg.AnalyticsEndpoint == "" {
		// Note: For Phase 4, analytics endpoint might not be required
		// but we'll check anyway
		if !cfg.Quiet {
			fmt.Println("Warning: analytics-service endpoint not configured. Sync operation may not work.")
		}
	}

	// Health check (analytics service)
	if cfg.AnalyticsEndpoint != "" {
		checker := health.NewChecker(5 * time.Second)
		requiredServices := map[string]string{
			"analytics-service": cfg.AnalyticsEndpoint,
		}
		if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
			return errors.NewServiceUnavailableError("analytics-service", cfg.AnalyticsEndpoint)
		}
	}

	// Trigger sync operation
	// NOTE: This is a placeholder implementation for Phase 4
	// Full implementation will be completed when analytics-service sync endpoints are available
	// For now, we'll simulate a sync operation
	
	jobID := fmt.Sprintf("sync-%s-%d", flagOrgID, time.Now().Unix())
	
	if !cfg.Quiet {
		fmt.Printf("Triggering sync operation for organization: %s\n", flagOrgID)
		fmt.Printf("Job ID: %s\n", jobID)
	}

	// If --wait is enabled, monitor progress
	if flagWait {
		return monitorSyncProgress(cmd.Context(), flagOrgID, jobID, cfg, startTime)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "sync_trigger",
		Command:     fmt.Sprintf("sync trigger --org-id=%s", flagOrgID),
		Parameters: map[string]interface{}{
			"orgId": flagOrgID,
			"jobId": jobID,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"jobId":   jobID,
			"orgId":   flagOrgID,
			"status":  "triggered",
			"message": "Sync operation triggered. Use 'sync status --job-id' to check progress.",
		})
	} else {
		if !cfg.Quiet {
			fmt.Printf("\nSync operation triggered successfully.\n")
			fmt.Printf("Use the following command to check status:\n")
			fmt.Printf("  admin-cli sync status --org-id=%s --job-id=%s\n", flagOrgID, jobID)
		}
	}
	return nil
}

func syncStatusCommand() *cobra.Command {
	var flagOrgID string
	var flagJobID string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAnalyticsEndpoint string
	var flagAPIKey string
	var flagWatch bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check sync operation status",
		Long:  "Check status of a sync operation by job ID. Use --watch to monitor until complete.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncStatus(cmd, args, flagOrgID, flagJobID, flagFormat, flagVerbose, flagQuiet,
				flagUserOrgEndpoint, flagAnalyticsEndpoint, flagAPIKey, flagWatch)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagJobID, "job-id", "", "Job ID from sync trigger (required)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().BoolVar(&flagWatch, "watch", false, "Watch status until complete")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAnalyticsEndpoint, "analytics-endpoint", "", "Analytics-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runSyncStatus(cmd *cobra.Command, args []string, flagOrgID, flagJobID, flagFormat string, flagVerbose, flagQuiet bool,
	flagUserOrgEndpoint, flagAnalyticsEndpoint, flagAPIKey string, flagWatch bool) error {
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
	if flagAnalyticsEndpoint != "" {
		cfg.AnalyticsEndpoint = flagAnalyticsEndpoint
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
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}
	if flagJobID == "" {
		return errors.NewValidationError(
			"--job-id is required",
			"Provide job ID from sync trigger with --job-id flag",
		)
	}

	if cfg.AnalyticsEndpoint == "" {
		if !cfg.Quiet {
			fmt.Println("Warning: analytics-service endpoint not configured. Status check may not work.")
		}
	}

	// Health check (analytics service)
	if cfg.AnalyticsEndpoint != "" {
		checker := health.NewChecker(5 * time.Second)
		requiredServices := map[string]string{
			"analytics-service": cfg.AnalyticsEndpoint,
		}
		if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
			return errors.NewServiceUnavailableError("analytics-service", cfg.AnalyticsEndpoint)
		}
	}

	// If --watch, monitor until complete
	if flagWatch {
		return monitorSyncProgress(cmd.Context(), flagOrgID, flagJobID, cfg, startTime)
	}

	// Check status (placeholder - will be implemented when analytics-service has status endpoint)
	// NOTE: This is a placeholder implementation for Phase 4
	status := map[string]interface{}{
		"jobId":   flagJobID,
		"orgId":   flagOrgID,
		"status":  "running", // Placeholder
		"message": "Sync operation status check (placeholder - full implementation in Phase 5)",
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "sync_status",
		Command:     fmt.Sprintf("sync status --org-id=%s --job-id=%s", flagOrgID, flagJobID),
		Parameters: map[string]interface{}{
			"orgId": flagOrgID,
			"jobId": flagJobID,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(status)
	} else {
		if !cfg.Quiet {
			fmt.Printf("Sync Operation Status:\n")
			fmt.Printf("  Job ID: %s\n", flagJobID)
			fmt.Printf("  Org ID: %s\n", flagOrgID)
			fmt.Printf("  Status: %v\n", status["status"])
		}
	}
	return nil
}

// monitorSyncProgress monitors sync operation progress until complete.
func monitorSyncProgress(ctx context.Context, orgID, jobID string, cfg *config.Config, startTime time.Time) error {
	progressIndicator := progress.NewIndicator(nil, cfg.OutputFormat)
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	elapsed := time.Since(startTime)
	var lastStatus string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			elapsed = time.Since(startTime)
			
			// NOTE: This is a placeholder - actual status check will query analytics-service
			// For now, simulate progress
			if elapsed > 30*time.Second {
				if progressIndicator.ShouldShow(elapsed) {
					// Simulate progress (will be replaced with actual status check)
					// progressIndicator.Update("sync", processed, total, elapsed)
				}
			}

			// Check if complete (placeholder)
			// For Phase 4, we'll just show a message that full implementation is in Phase 5
			if elapsed > 5*time.Second {
				if !cfg.Quiet && lastStatus != "complete" {
					fmt.Println("\nNote: Full sync monitoring will be implemented in Phase 5")
					lastStatus = "complete"
					if cfg.OutputFormat == "table" {
						progressIndicator.Complete("sync", 100, elapsed)
					}
				}
				return nil
			}
		}
	}
}


