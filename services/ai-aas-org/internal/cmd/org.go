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

// orgCmd represents the org command
var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "View organization details",
	Long: `View details about your organization.

See your organization's plan, limits, and current usage statistics.

Examples:
  ai-aas-org org show`,
}

func init() {
	rootCmd.AddCommand(orgCmd)
}

// --- org show ---

var orgShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show organization details",
	Long: `Show detailed information about your organization.

Displays:
  • Organization name and ID
  • Current plan
  • Usage limits
  • Resource counts (users, API keys, models)

Examples:
  ai-aas-org org show
  ai-aas-org org show --json`,
	RunE: runOrgShow,
}

func init() {
	orgCmd.AddCommand(orgShowCmd)
}

func runOrgShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	details, err := client.GetOrganization(ctx, config.GetOrgID())
	if err != nil {
		return errors.NewOperationError("failed to get organization", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(details)
	}

	org := details.Organization
	limits := details.Limits

	output.Header("Organization Details")
	output.KeyValue("Name", org.Name)
	output.KeyValue("ID", org.ID)
	output.KeyValue("Status", output.StatusBadge(org.Status))
	output.KeyValue("Plan", org.Plan)
	output.KeyValue("Created", org.CreatedAt.Format("2006-01-02"))

	fmt.Println()
	output.Header("Resources")
	output.KeyValue("Users", fmt.Sprintf("%d / %d", org.UserCount, limits.MaxUsers))
	output.KeyValue("API Keys", fmt.Sprintf("%d / %d", org.APIKeyCount, limits.MaxAPIKeys))
	output.KeyValue("Models", fmt.Sprintf("%d", org.ModelCount))

	fmt.Println()
	output.Header("Rate Limits")
	output.KeyValue("Requests/min", formatNumber(int64(limits.RateLimitRPM)))
	output.KeyValue("Tokens/min", formatNumber(limits.RateLimitTPM))
	output.KeyValue("Monthly Tokens", formatNumber(limits.MonthlyTokens))

	return nil
}
