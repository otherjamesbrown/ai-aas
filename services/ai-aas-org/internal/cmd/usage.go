package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// usageCmd represents the usage command
var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "View token usage and costs",
	Long: `View token usage statistics and costs for your organization.

Track how many tokens your organization is consuming, broken down by
model and user. Monitor costs and identify usage patterns.

Examples:
  ai-aas-org usage summary
  ai-aas-org usage by-model --period 30d
  ai-aas-org usage by-user --period 7d`,
}

func init() {
	rootCmd.AddCommand(usageCmd)
}

// --- usage summary ---

var usageSummaryPeriod string

var usageSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show usage summary",
	Long: `Show a summary of token usage and costs for your organization.

Periods:
  today, yesterday, 7d, 30d, 90d, this-month, last-month

Examples:
  ai-aas-org usage summary
  ai-aas-org usage summary --period 30d`,
	RunE: runUsageSummary,
}

func init() {
	usageCmd.AddCommand(usageSummaryCmd)

	usageSummaryCmd.Flags().StringVar(&usageSummaryPeriod, "period", "30d", "time period (today, 7d, 30d, 90d, this-month)")
}

func runUsageSummary(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	summary, err := client.GetUsageSummary(ctx, config.GetOrgID(), usageSummaryPeriod)
	if err != nil {
		return errors.NewOperationError("failed to get usage summary", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(summary)
	}

	output.Header(fmt.Sprintf("Usage Summary (%s)", usageSummaryPeriod))
	fmt.Println()

	output.KeyValue("Period", fmt.Sprintf("%s to %s",
		summary.StartDate.Format("2006-01-02"),
		summary.EndDate.Format("2006-01-02")))
	fmt.Println()

	output.KeyValue("Total Requests", formatNumber(summary.TotalRequests))
	output.KeyValue("Total Tokens", formatNumber(summary.TotalTokens))
	output.KeyValue("  Prompt Tokens", formatNumber(summary.PromptTokens))
	output.KeyValue("  Completion Tokens", formatNumber(summary.CompletionTokens))
	fmt.Println()

	if summary.TotalCost > 0 {
		output.KeyValue("Estimated Cost", fmt.Sprintf("$%.2f %s", summary.TotalCost, summary.Currency))
	}

	return nil
}

// --- usage by-model ---

var usageByModelPeriod string

var usageByModelCmd = &cobra.Command{
	Use:   "by-model",
	Short: "Show usage by model",
	Long: `Show token usage broken down by AI model.

Examples:
  ai-aas-org usage by-model
  ai-aas-org usage by-model --period 7d`,
	RunE: runUsageByModel,
}

func init() {
	usageCmd.AddCommand(usageByModelCmd)

	usageByModelCmd.Flags().StringVar(&usageByModelPeriod, "period", "30d", "time period")
}

func runUsageByModel(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	models, err := client.GetUsageByModel(ctx, config.GetOrgID(), usageByModelPeriod)
	if err != nil {
		return errors.NewOperationError("failed to get usage by model", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(models)
	}

	if len(models) == 0 {
		output.InfoMsg("No usage data for the selected period.")
		return nil
	}

	output.Header(fmt.Sprintf("Usage by Model (%s)", usageByModelPeriod))
	fmt.Println()

	headers := []string{"MODEL", "REQUESTS", "TOKENS", "COST"}
	var rows [][]string
	for _, m := range models {
		cost := "-"
		if m.TotalCost > 0 {
			cost = fmt.Sprintf("$%.2f", m.TotalCost)
		}
		rows = append(rows, []string{
			m.ModelName,
			formatNumber(m.TotalRequests),
			formatNumber(m.TotalTokens),
			cost,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

// --- usage by-user ---

var usageByUserPeriod string

var usageByUserCmd = &cobra.Command{
	Use:   "by-user",
	Short: "Show usage by user",
	Long: `Show token usage broken down by user.

Examples:
  ai-aas-org usage by-user
  ai-aas-org usage by-user --period 7d`,
	RunE: runUsageByUser,
}

func init() {
	usageCmd.AddCommand(usageByUserCmd)

	usageByUserCmd.Flags().StringVar(&usageByUserPeriod, "period", "30d", "time period")
}

func runUsageByUser(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	users, err := client.GetUsageByUser(ctx, config.GetOrgID(), usageByUserPeriod)
	if err != nil {
		return errors.NewOperationError("failed to get usage by user", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(users)
	}

	if len(users) == 0 {
		output.InfoMsg("No usage data for the selected period.")
		return nil
	}

	output.Header(fmt.Sprintf("Usage by User (%s)", usageByUserPeriod))
	fmt.Println()

	headers := []string{"USER", "REQUESTS", "TOKENS", "COST"}
	var rows [][]string
	for _, u := range users {
		cost := "-"
		if u.TotalCost > 0 {
			cost = fmt.Sprintf("$%.2f", u.TotalCost)
		}
		rows = append(rows, []string{
			u.UserEmail,
			formatNumber(u.TotalRequests),
			formatNumber(u.TotalTokens),
			cost,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

// formatNumber formats a number with thousands separators.
func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}
