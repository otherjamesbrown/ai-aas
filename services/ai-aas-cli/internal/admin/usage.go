// Package commands provides usage query commands.
//
// Purpose:
//
//	Usage query and summary commands for organizations.
//	Supports structured output and filtering.
//
// Requirements Reference:
//   - specs/019-admin-cli-enhancements/spec.md#US-003 (Usage Queries)
//
package admin

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/analytics"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// UsageCommand creates the usage command group.
func UsageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Query usage data",
		Long:  "Query and summarize usage data for organizations",
	}

	cmd.AddCommand(usageQueryCommand())
	cmd.AddCommand(usageSummaryCommand())

	return cmd
}

func usageQueryCommand() *cobra.Command {
	var flagOrgID string
	var flagFrom string
	var flagTo string
	var flagGranularity string
	var flagModel string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagAnalyticsEndpoint string
	var flagAPIKey string
	var flagLastHour bool
	var flagLast24h bool
	var flagLast7d bool
	var flagAllTime bool

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query usage data",
		Long:  "Retrieve filtered usage data for an organization",
		Example: `  # Query usage for January 2025
  admin-cli usage query --org-id acme-corp --from 2025-01-01 --to 2025-01-31

  # Query usage for last 24 hours (convenience flag)
  admin-cli usage query --org-id acme-corp --last-24h

  # Query usage for last 7 days
  admin-cli usage query --org-id acme-corp --last-7d

  # Query usage for last hour with hourly granularity
  admin-cli usage query --org-id acme-corp --last-hour --granularity hour

  # Query with hourly granularity
  admin-cli usage query --org-id acme-corp --from 2025-01-01 --to 2025-01-07 --granularity hour

  # Filter by model
  admin-cli usage query --org-id acme-corp --from 2025-01-01 --to 2025-01-31 --model gpt-4

  # Output as JSON
  admin-cli usage query --org-id acme-corp --from 2025-01-01 --to 2025-01-31 --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsageQuery(cmd, args, flagOrgID, flagFrom, flagTo, flagGranularity, flagModel,
				flagFormat, flagVerbose, flagQuiet, flagAnalyticsEndpoint, flagAPIKey,
				flagLastHour, flagLast24h, flagLast7d, flagAllTime)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagGranularity, "granularity", "day", "Data granularity: hour, day")
	cmd.Flags().StringVar(&flagModel, "model", "", "Filter by model ID (optional)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagAnalyticsEndpoint, "analytics-endpoint", "", "Analytics-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	// Convenience time range flags
	cmd.Flags().BoolVar(&flagLastHour, "last-hour", false, "Usage in the last 60 minutes")
	cmd.Flags().BoolVar(&flagLast24h, "last-24h", false, "Usage in the last 24 hours")
	cmd.Flags().BoolVar(&flagLast7d, "last-7d", false, "Usage in the last 7 days")
	cmd.Flags().BoolVar(&flagAllTime, "all-time", false, "All usage data (no time filter)")

	return cmd
}

func runUsageQuery(cmd *cobra.Command, args []string, flagOrgID, flagFrom, flagTo, flagGranularity, flagModel string,
	flagFormat string, flagVerbose, flagQuiet bool, flagAnalyticsEndpoint, flagAPIKey string,
	flagLastHour, flagLast24h, flagLast7d, flagAllTime bool) error {
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
	if cfg.AnalyticsEndpoint == "" {
		return errors.NewValidationError(
			"analytics-service endpoint is required",
			"Set via --analytics-endpoint flag or ADMIN_CLI_ANALYTICS_ENDPOINT environment variable",
		)
	}

	// Validate required fields
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}

	// Check for mutual exclusivity between convenience flags and explicit dates
	convenientFlagsCount := 0
	if flagLastHour {
		convenientFlagsCount++
	}
	if flagLast24h {
		convenientFlagsCount++
	}
	if flagLast7d {
		convenientFlagsCount++
	}
	if flagAllTime {
		convenientFlagsCount++
	}

	if convenientFlagsCount > 1 {
		return errors.NewValidationError(
			"only one convenience time range flag can be used at a time",
			"Use either --last-hour, --last-24h, --last-7d, or --all-time",
		)
	}

	if convenientFlagsCount > 0 && (flagFrom != "" || flagTo != "") {
		return errors.NewValidationError(
			"cannot use convenience time range flags with --from/--to",
			"Use either convenience flags (--last-hour, --last-24h, --last-7d, --all-time) OR explicit dates (--from/--to)",
		)
	}

	// Calculate date range from convenience flags
	now := time.Now()
	if flagLastHour {
		flagFrom = now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05")
		flagTo = now.Format("2006-01-02T15:04:05")
	} else if flagLast24h {
		flagFrom = now.Add(-24 * time.Hour).Format("2006-01-02")
		flagTo = now.Format("2006-01-02")
	} else if flagLast7d {
		flagFrom = now.AddDate(0, 0, -7).Format("2006-01-02")
		flagTo = now.Format("2006-01-02")
	} else if flagAllTime {
		// Use a very early date for all-time queries
		flagFrom = "2000-01-01"
		flagTo = now.Format("2006-01-02")
	}

	// Now validate explicit dates if no convenience flag was used
	if flagFrom == "" {
		return errors.NewValidationError(
			"--from is required",
			"Provide start date with --from flag (YYYY-MM-DD) or use a convenience flag (--last-hour, --last-24h, --last-7d, --all-time)",
		)
	}
	if flagTo == "" {
		return errors.NewValidationError(
			"--to is required",
			"Provide end date with --to flag (YYYY-MM-DD) or use a convenience flag (--last-hour, --last-24h, --last-7d, --all-time)",
		)
	}

	// Parse and validate dates (support both date-only and datetime formats)
	var fromDate, toDate time.Time

	// Try parsing with datetime format first (for --last-hour), then fall back to date-only
	fromDate, err = time.Parse("2006-01-02T15:04:05", flagFrom)
	if err != nil {
		fromDate, err = time.Parse("2006-01-02", flagFrom)
		if err != nil {
			return errors.NewValidationError(
				"invalid 'from' date format",
				"Use YYYY-MM-DD format (e.g., 2025-01-01)",
			)
		}
	}

	toDate, err = time.Parse("2006-01-02T15:04:05", flagTo)
	if err != nil {
		toDate, err = time.Parse("2006-01-02", flagTo)
		if err != nil {
			return errors.NewValidationError(
				"invalid 'to' date format",
				"Use YYYY-MM-DD format (e.g., 2025-01-31)",
			)
		}
	}

	if fromDate.After(toDate) {
		return errors.NewValidationError(
			"'from' date must be before 'to' date",
			"Check your date range",
		)
	}

	// Validate granularity
	if flagGranularity != "hour" && flagGranularity != "day" {
		return errors.NewValidationError(
			"granularity must be 'hour' or 'day'",
			"Use --granularity hour or --granularity day",
		)
	}

	// Health check
	checker := health.NewChecker(5 * time.Second)
	requiredServices := map[string]string{
		"analytics-service": cfg.AnalyticsEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("analytics-service", cfg.AnalyticsEndpoint)
	}

	// Create client and query usage
	analyticsClient := analytics.NewClient(cfg.AnalyticsEndpoint, cfg.APIKey)

	params := analytics.UsageQueryParams{
		OrgID:       flagOrgID,
		Start:       fromDate,
		End:         toDate.AddDate(0, 0, 1), // End date is inclusive, add 1 day
		Granularity: flagGranularity,
		ModelID:     flagModel,
	}

	usageData, err := analyticsClient.QueryUsage(cmd.Context(), params)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to query usage: %v", err),
			"Verify your API key is valid and you have permission to view usage data.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:     "usage_query",
		Command:  fmt.Sprintf("usage query --org-id=%s --from=%s --to=%s", flagOrgID, flagFrom, flagTo),
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(usageData)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"bucketStart", "invocations", "inputTokens", "outputTokens", "costCents"}
		var rows [][]string
		for _, point := range usageData.Series {
			rows = append(rows, []string{
				point.BucketStart,
				fmt.Sprintf("%d", point.Invocations),
				fmt.Sprintf("%d", point.InputTokens),
				fmt.Sprintf("%d", point.OutputTokens),
				fmt.Sprintf("%d", point.CostEstimateCents),
			})
		}
		return output.PrintTable(headers, rows)
	} else {
		// Table format
		if !cfg.Quiet {
			fmt.Printf("Usage Query: %s to %s\n", flagFrom, flagTo)
			fmt.Printf("Organization: %s | Granularity: %s\n", usageData.OrgID, usageData.Granularity)
			fmt.Println("─────────────────────────────────────────────────────────────")
		}

		headers := []string{"Period", "Invocations", "Input Tokens", "Output Tokens", "Cost"}
		var rows [][]string
		for _, point := range usageData.Series {
			rows = append(rows, []string{
				point.BucketStart,
				fmt.Sprintf("%d", point.Invocations),
				fmt.Sprintf("%d", point.InputTokens),
				fmt.Sprintf("%d", point.OutputTokens),
				fmt.Sprintf("$%.2f", float64(point.CostEstimateCents)/100),
			})
		}

		if len(rows) == 0 && !cfg.Quiet {
			fmt.Println("No usage data found for the specified period.")
			return nil
		}

		if err := output.PrintTable(headers, rows); err != nil {
			return err
		}

		// Print totals
		if !cfg.Quiet {
			fmt.Println("─────────────────────────────────────────────────────────────")
			fmt.Printf("Totals: %d invocations, %d input tokens, %d output tokens, $%.2f\n",
				usageData.Totals.Invocations,
				usageData.Totals.InputTokens,
				usageData.Totals.OutputTokens,
				float64(usageData.Totals.CostEstimateCents)/100)
		}
		return nil
	}
}

func usageSummaryCommand() *cobra.Command {
	var flagOrgID string
	var flagPeriod string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagAnalyticsEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Show usage summary",
		Long:  "Display aggregated usage totals for an organization",
		Example: `  # Show summary for current month
  admin-cli usage summary --org-id acme-corp

  # Show summary for a specific period
  admin-cli usage summary --org-id acme-corp --period week

  # Output as JSON
  admin-cli usage summary --org-id acme-corp --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsageSummary(cmd, args, flagOrgID, flagPeriod,
				flagFormat, flagVerbose, flagQuiet, flagAnalyticsEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagPeriod, "period", "month", "Summary period: day, week, month")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagAnalyticsEndpoint, "analytics-endpoint", "", "Analytics-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runUsageSummary(cmd *cobra.Command, args []string, flagOrgID, flagPeriod string,
	flagFormat string, flagVerbose, flagQuiet bool, flagAnalyticsEndpoint, flagAPIKey string) error {
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
	if cfg.AnalyticsEndpoint == "" {
		return errors.NewValidationError(
			"analytics-service endpoint is required",
			"Set via --analytics-endpoint flag or ADMIN_CLI_ANALYTICS_ENDPOINT environment variable",
		)
	}

	// Validate required fields
	if flagOrgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag",
		)
	}

	// Calculate date range based on period
	now := time.Now()
	var fromDate, toDate time.Time

	switch flagPeriod {
	case "day":
		fromDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		toDate = fromDate.AddDate(0, 0, 1)
	case "week":
		// Start of current week (Sunday)
		weekday := int(now.Weekday())
		fromDate = time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, now.Location())
		toDate = now.AddDate(0, 0, 1)
	case "month":
		fromDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		toDate = now.AddDate(0, 0, 1)
	default:
		return errors.NewValidationError(
			"period must be 'day', 'week', or 'month'",
			"Use --period day, --period week, or --period month",
		)
	}

	// Health check
	checker := health.NewChecker(5 * time.Second)
	requiredServices := map[string]string{
		"analytics-service": cfg.AnalyticsEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("analytics-service", cfg.AnalyticsEndpoint)
	}

	// Create client and query usage
	analyticsClient := analytics.NewClient(cfg.AnalyticsEndpoint, cfg.APIKey)

	params := analytics.UsageQueryParams{
		OrgID:       flagOrgID,
		Start:       fromDate,
		End:         toDate,
		Granularity: "day",
	}

	usageData, err := analyticsClient.QueryUsage(cmd.Context(), params)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to query usage: %v", err),
			"Verify your API key is valid and you have permission to view usage data.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:     "usage_summary",
		Command:  fmt.Sprintf("usage summary --org-id=%s --period=%s", flagOrgID, flagPeriod),
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Build summary response
	summary := map[string]interface{}{
		"orgId":             usageData.OrgID,
		"period":            flagPeriod,
		"periodStart":       fromDate.Format("2006-01-02"),
		"periodEnd":         toDate.AddDate(0, 0, -1).Format("2006-01-02"),
		"totalInvocations":  usageData.Totals.Invocations,
		"totalInputTokens":  usageData.Totals.InputTokens,
		"totalOutputTokens": usageData.Totals.OutputTokens,
		"estimatedCost":     float64(usageData.Totals.CostEstimateCents) / 100,
	}

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(summary)
	} else {
		if !cfg.Quiet {
			fmt.Printf("Organization: %s\n", usageData.OrgID)
			fmt.Printf("Period: %s (%s to %s)\n", flagPeriod, fromDate.Format("2006-01-02"), toDate.AddDate(0, 0, -1).Format("2006-01-02"))
			fmt.Println("─────────────────────────────")
			fmt.Printf("Total Requests:  %d\n", usageData.Totals.Invocations)
			fmt.Printf("Total Tokens:    %d\n", usageData.Totals.InputTokens+usageData.Totals.OutputTokens)
			fmt.Printf("  - Input:       %d\n", usageData.Totals.InputTokens)
			fmt.Printf("  - Output:      %d\n", usageData.Totals.OutputTokens)
			fmt.Printf("Estimated Cost:  $%.2f\n", float64(usageData.Totals.CostEstimateCents)/100)

			// Show freshness info in verbose mode
			if cfg.Verbose {
				fmt.Println("─────────────────────────────")
				fmt.Printf("Data Freshness:  %s\n", usageData.Freshness.Status)
				fmt.Printf("Lag:             %d seconds\n", usageData.Freshness.LagSeconds)
			}
		}
		return nil
	}
}
