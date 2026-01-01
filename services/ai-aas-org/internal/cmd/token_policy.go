package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/prompt"
)

// tokenPolicyCmd represents the token-policy command
var tokenPolicyCmd = &cobra.Command{
	Use:   "token-policy",
	Short: "Manage token rate-limit policies",
	Long: `Manage token rate-limit policies for your organization.

Token policies define rate limits for API token usage across different time windows.
You can set limits for 1-hour, 24-hour, and 7-day rolling windows.

Examples:
  ai-aas-org token-policy list
  ai-aas-org token-policy create --name "Standard" --1h 10000 --24h 100000 --7d 500000
  ai-aas-org token-policy show "Standard"
  ai-aas-org token-policy set-default --policy "Standard"
  ai-aas-org token-policy delete "Standard"`,
}

func init() {
	rootCmd.AddCommand(tokenPolicyCmd)
}

// --- token-policy list ---

var tokenPolicyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List token rate-limit policies",
	Long: `List all token rate-limit policies for your organization.

Includes both custom policies and the built-in "No Token Rate-Limit" policy.

Examples:
  ai-aas-org token-policy list
  ai-aas-org token-policy list --json`,
	RunE: runTokenPolicyList,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicyListCmd)
}

func runTokenPolicyList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	policies, err := client.ListTokenPolicies(ctx, config.GetOrgID())
	if err != nil {
		return wrapAPIError(err, "failed to list token policies")
	}

	if IsJSONOutput() {
		return output.PrintJSON(policies)
	}

	if len(policies) == 0 {
		output.InfoMsg("No token policies found.")
		fmt.Println()
		fmt.Println("Create a policy with:")
		fmt.Println("  ai-aas-org token-policy create --name <name> --1h <limit>")
		return nil
	}

	headers := []string{"NAME", "1H LIMIT", "24H LIMIT", "7D LIMIT", "BUILTIN", "CREATED"}
	var rows [][]string
	for _, p := range policies {
		rows = append(rows, []string{
			p.Name,
			formatLimit(p.Limit1h),
			formatLimit(p.Limit24h),
			formatLimit(p.Limit7d),
			formatBool(p.IsBuiltin),
			formatDate(p.CreatedAt),
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d policies\n", len(policies))
	return nil
}

// --- token-policy create ---

var (
	tokenPolicyCreateName string
	tokenPolicyCreateDesc string
	tokenPolicyCreate1h   int64
	tokenPolicyCreate24h  int64
	tokenPolicyCreate7d   int64
)

var tokenPolicyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new token rate-limit policy",
	Long: `Create a new token rate-limit policy.

At least one limit must be specified. Use 0 or omit to set no limit for a window.

Examples:
  ai-aas-org token-policy create --name "Standard" --1h 10000 --24h 100000
  ai-aas-org token-policy create --name "Premium" --24h 500000 --7d 2000000
  ai-aas-org token-policy create --guided`,
	RunE: runTokenPolicyCreate,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicyCreateCmd)

	tokenPolicyCreateCmd.Flags().StringVarP(&tokenPolicyCreateName, "name", "n", "", "policy name (required)")
	tokenPolicyCreateCmd.Flags().StringVarP(&tokenPolicyCreateDesc, "description", "d", "", "policy description")
	tokenPolicyCreateCmd.Flags().Int64Var(&tokenPolicyCreate1h, "1h", 0, "1-hour token limit")
	tokenPolicyCreateCmd.Flags().Int64Var(&tokenPolicyCreate24h, "24h", 0, "24-hour token limit")
	tokenPolicyCreateCmd.Flags().Int64Var(&tokenPolicyCreate7d, "7d", 0, "7-day token limit")
}

func runTokenPolicyCreate(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	name := tokenPolicyCreateName
	desc := tokenPolicyCreateDesc
	limit1h := tokenPolicyCreate1h
	limit24h := tokenPolicyCreate24h
	limit7d := tokenPolicyCreate7d

	// Guided mode
	if IsGuidedMode() || name == "" {
		output.Header("Create Token Rate-Limit Policy")
		fmt.Println()

		var err error
		name, err = prompt.InputRequired("Policy name")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}

		desc, err = prompt.Input("Description (optional)", "")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}

		limit1hStr, err := prompt.Input("1-hour limit (tokens, empty for no limit)", "")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}
		if limit1hStr != "" {
			limit1h, err = strconv.ParseInt(limit1hStr, 10, 64)
			if err != nil {
				return errors.NewValidationError("invalid 1-hour limit", "must be a number")
			}
		}

		limit24hStr, err := prompt.Input("24-hour limit (tokens, empty for no limit)", "")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}
		if limit24hStr != "" {
			limit24h, err = strconv.ParseInt(limit24hStr, 10, 64)
			if err != nil {
				return errors.NewValidationError("invalid 24-hour limit", "must be a number")
			}
		}

		limit7dStr, err := prompt.Input("7-day limit (tokens, empty for no limit)", "")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}
		if limit7dStr != "" {
			limit7d, err = strconv.ParseInt(limit7dStr, 10, 64)
			if err != nil {
				return errors.NewValidationError("invalid 7-day limit", "must be a number")
			}
		}

		fmt.Println()
	}

	// Validate
	if name == "" {
		return errors.NewUsageError("--name is required")
	}
	if limit1h == 0 && limit24h == 0 && limit7d == 0 {
		return errors.NewValidationError("at least one limit must be specified", "use --1h, --24h, or --7d")
	}

	req := &api.CreateTokenPolicyRequest{
		Name: name,
	}
	if desc != "" {
		req.Description = &desc
	}
	if limit1h > 0 {
		req.Limit1h = &limit1h
	}
	if limit24h > 0 {
		req.Limit24h = &limit24h
	}
	if limit7d > 0 {
		req.Limit7d = &limit7d
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	policy, err := client.CreateTokenPolicy(ctx, config.GetOrgID(), req)
	if err != nil {
		return wrapAPIError(err, "failed to create token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(policy)
	}

	output.SuccessMsg("Token policy created successfully!")
	fmt.Println()
	printTokenPolicy(policy)

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  • Set as org default:  ai-aas-org token-policy set-default --policy %q\n", policy.Name)
	fmt.Printf("  • Assign to a user:    ai-aas-org user set-token-policy --user <email> --policy %q\n", policy.Name)

	return nil
}

// --- token-policy show ---

var tokenPolicyShowCmd = &cobra.Command{
	Use:   "show <name-or-id>",
	Short: "Show details for a token policy",
	Long: `Show detailed information about a token rate-limit policy.

You can specify the policy by name or ID.

Examples:
  ai-aas-org token-policy show "Standard"
  ai-aas-org token-policy show 12345678-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: runTokenPolicyShow,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicyShowCmd)
}

func runTokenPolicyShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	policyID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Try to find by name first
	policy, err := findPolicyByNameOrID(ctx, client, policyID)
	if err != nil {
		return wrapAPIError(err, "failed to get token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(policy)
	}

	output.Header("Token Policy Details")
	printTokenPolicy(policy)
	return nil
}

// --- token-policy update ---

var (
	tokenPolicyUpdateName string
	tokenPolicyUpdateDesc string
	tokenPolicyUpdate1h   int64
	tokenPolicyUpdate24h  int64
	tokenPolicyUpdate7d   int64
)

var tokenPolicyUpdateCmd = &cobra.Command{
	Use:   "update <name-or-id>",
	Short: "Update a token rate-limit policy",
	Long: `Update an existing token rate-limit policy.

Only specified fields will be updated.

Examples:
  ai-aas-org token-policy update "Standard" --1h 20000
  ai-aas-org token-policy update "Standard" --name "Standard V2" --24h 200000`,
	Args: cobra.ExactArgs(1),
	RunE: runTokenPolicyUpdate,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicyUpdateCmd)

	tokenPolicyUpdateCmd.Flags().StringVarP(&tokenPolicyUpdateName, "name", "n", "", "new policy name")
	tokenPolicyUpdateCmd.Flags().StringVarP(&tokenPolicyUpdateDesc, "description", "d", "", "new description")
	tokenPolicyUpdateCmd.Flags().Int64Var(&tokenPolicyUpdate1h, "1h", 0, "new 1-hour token limit")
	tokenPolicyUpdateCmd.Flags().Int64Var(&tokenPolicyUpdate24h, "24h", 0, "new 24-hour token limit")
	tokenPolicyUpdateCmd.Flags().Int64Var(&tokenPolicyUpdate7d, "7d", 0, "new 7-day token limit")
}

func runTokenPolicyUpdate(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	policyID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Find policy first
	existing, err := findPolicyByNameOrID(ctx, client, policyID)
	if err != nil {
		return wrapAPIError(err, "failed to find token policy")
	}

	// Check if any updates specified
	nameChanged := cmd.Flags().Changed("name")
	descChanged := cmd.Flags().Changed("description")
	limit1hChanged := cmd.Flags().Changed("1h")
	limit24hChanged := cmd.Flags().Changed("24h")
	limit7dChanged := cmd.Flags().Changed("7d")

	if !nameChanged && !descChanged && !limit1hChanged && !limit24hChanged && !limit7dChanged {
		return errors.NewValidationError("no updates specified", "use --name, --description, --1h, --24h, or --7d")
	}

	req := &api.UpdateTokenPolicyRequest{}
	if nameChanged {
		req.Name = &tokenPolicyUpdateName
	}
	if descChanged {
		req.Description = &tokenPolicyUpdateDesc
	}
	if limit1hChanged {
		req.Limit1h = &tokenPolicyUpdate1h
	}
	if limit24hChanged {
		req.Limit24h = &tokenPolicyUpdate24h
	}
	if limit7dChanged {
		req.Limit7d = &tokenPolicyUpdate7d
	}

	policy, err := client.UpdateTokenPolicy(ctx, config.GetOrgID(), existing.ID, req)
	if err != nil {
		return wrapAPIError(err, "failed to update token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(policy)
	}

	output.SuccessMsg("Token policy updated successfully!")
	fmt.Println()
	printTokenPolicy(policy)
	return nil
}

// --- token-policy delete ---

var tokenPolicyDeleteForce bool

var tokenPolicyDeleteCmd = &cobra.Command{
	Use:   "delete <name-or-id>",
	Short: "Delete a token rate-limit policy",
	Long: `Delete a token rate-limit policy.

The policy cannot be deleted if it is in use (set as org default or user override).
Built-in policies cannot be deleted.

Examples:
  ai-aas-org token-policy delete "Old Policy"
  ai-aas-org token-policy delete "Old Policy" --force`,
	Args: cobra.ExactArgs(1),
	RunE: runTokenPolicyDelete,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicyDeleteCmd)

	tokenPolicyDeleteCmd.Flags().BoolVar(&tokenPolicyDeleteForce, "force", false, "skip confirmation prompt")
}

func runTokenPolicyDelete(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	policyID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Find policy first
	policy, err := findPolicyByNameOrID(ctx, client, policyID)
	if err != nil {
		return wrapAPIError(err, "failed to find token policy")
	}

	if policy.IsBuiltin {
		return errors.NewValidationError("cannot delete built-in policy", "built-in policies cannot be modified")
	}

	// Confirm deletion
	if !tokenPolicyDeleteForce {
		output.WarningMsg("You are about to delete token policy: %s", policy.Name)
		fmt.Println()
		fmt.Println("This action cannot be undone.")
		fmt.Println()

		confirmed, err := prompt.ConfirmWithWord(
			"To confirm, type 'DELETE'",
			"DELETE",
		)
		if err != nil {
			return errors.NewOperationError("failed to read confirmation", err.Error())
		}
		if !confirmed {
			output.InfoMsg("Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteTokenPolicy(ctx, config.GetOrgID(), policy.ID); err != nil {
		return wrapAPIError(err, "failed to delete token policy")
	}

	output.SuccessMsg("Token policy %q has been deleted.", policy.Name)
	return nil
}

// --- token-policy get-default ---

var tokenPolicyGetDefaultCmd = &cobra.Command{
	Use:   "get-default",
	Short: "Get the organization's default token policy",
	Long: `Get the default token rate-limit policy for your organization.

New users inherit this policy unless they have an override.

Examples:
  ai-aas-org token-policy get-default
  ai-aas-org token-policy get-default --json`,
	RunE: runTokenPolicyGetDefault,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicyGetDefaultCmd)
}

func runTokenPolicyGetDefault(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	policy, err := client.GetOrgDefaultTokenPolicy(ctx, config.GetOrgID())
	if err != nil {
		return wrapAPIError(err, "failed to get default token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(policy)
	}

	output.Header("Organization Default Token Policy")
	printTokenPolicy(policy)
	return nil
}

// --- token-policy set-default ---

var tokenPolicySetDefaultPolicy string

var tokenPolicySetDefaultCmd = &cobra.Command{
	Use:   "set-default",
	Short: "Set the organization's default token policy",
	Long: `Set the default token rate-limit policy for your organization.

New users will inherit this policy. Use "no-limit" for the built-in no-limit policy.

Examples:
  ai-aas-org token-policy set-default --policy "Standard"
  ai-aas-org token-policy set-default --policy no-limit`,
	RunE: runTokenPolicySetDefault,
}

func init() {
	tokenPolicyCmd.AddCommand(tokenPolicySetDefaultCmd)

	tokenPolicySetDefaultCmd.Flags().StringVar(&tokenPolicySetDefaultPolicy, "policy", "", "policy name or ID (required)")
	tokenPolicySetDefaultCmd.MarkFlagRequired("policy")
}

func runTokenPolicySetDefault(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve policy ID (can be name, ID, or "no-limit" alias)
	policyID := tokenPolicySetDefaultPolicy
	if policyID != "no-limit" {
		policy, err := findPolicyByNameOrID(ctx, client, policyID)
		if err != nil {
			return wrapAPIError(err, "failed to find token policy")
		}
		policyID = policy.ID
	}

	result, err := client.SetOrgDefaultTokenPolicy(ctx, config.GetOrgID(), policyID)
	if err != nil {
		return wrapAPIError(err, "failed to set default token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	output.SuccessMsg("Default token policy set to %q", result.Name)
	fmt.Println()
	printTokenPolicy(result)
	return nil
}

// --- Helpers ---

func findPolicyByNameOrID(ctx context.Context, client *api.Client, nameOrID string) (*api.TokenRateLimitPolicy, error) {
	// First try to get by ID directly
	policy, err := client.GetTokenPolicy(ctx, config.GetOrgID(), nameOrID)
	if err == nil {
		return policy, nil
	}

	// If not found, search by name
	policies, err := client.ListTokenPolicies(ctx, config.GetOrgID())
	if err != nil {
		return nil, err
	}

	for i := range policies {
		if policies[i].Name == nameOrID {
			return &policies[i], nil
		}
	}

	return nil, errors.NewNotFoundError("Token policy", nameOrID)
}

func printTokenPolicy(p *api.TokenRateLimitPolicy) {
	output.KeyValue("Name", p.Name)
	output.KeyValue("ID", p.ID)
	if p.Description != nil && *p.Description != "" {
		output.KeyValue("Description", *p.Description)
	}
	output.KeyValue("1h Limit", formatLimit(p.Limit1h))
	output.KeyValue("24h Limit", formatLimit(p.Limit24h))
	output.KeyValue("7d Limit", formatLimit(p.Limit7d))
	output.KeyValue("Built-in", formatBool(p.IsBuiltin))
	output.KeyValue("Created", formatDate(p.CreatedAt))
	output.KeyValue("Updated", formatDate(p.UpdatedAt))
}

func formatLimit(limit *int64) string {
	if limit == nil || *limit == 0 {
		return "unlimited"
	}
	// Format with thousands separators for readability
	return formatTokenCount(*limit)
}

func formatTokenCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func formatDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}
