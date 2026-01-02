package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/prompt"
)

// userCmd represents the user command
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users in your organization",
	Long: `Manage users in your organization.

Users are members of your organization who can access AI models
through API keys. You can create users, view their details, and
manage their access.

Examples:
  ai-aas-org user list
  ai-aas-org user create --user-email user@example.com --user-display-name "John Doe"
  ai-aas-org user show user@example.com
  ai-aas-org user delete user@example.com`,
}

func init() {
	rootCmd.AddCommand(userCmd)
}

// requireConfig checks if the CLI is configured and returns an error if not.
func requireConfig() error {
	if !config.IsConfigured() {
		return errors.NewOperationError(
			"CLI is not configured",
			"Run 'ai-aas-org init --key <bootstrap-key>' to set up the CLI.",
		)
	}
	return nil
}

// newAPIClient creates a new API client from the current configuration.
func newAPIClient() *api.Client {
	return api.NewClient(config.GetAPIEndpoint(), config.GetAPIKey())
}

// newAdminAPIClient creates a new API client for admin operations (benchmarks, etc).
func newAdminAPIClient() *api.Client {
	return api.NewClient(config.GetAdminEndpoint(), config.GetAPIKey())
}

// wrapAPIError wraps an API error with a user-friendly message while preserving semantic exit codes.
// If the error is already a CLIError with an exit code, it passes through unchanged.
// Otherwise, it wraps it in an OperationError.
func wrapAPIError(err error, context string) error {
	if err == nil {
		return nil
	}
	// Pass through CLI errors with semantic exit codes from API client
	if errors.IsCLIError(err) {
		return err
	}
	return errors.NewOperationError(context, err.Error())
}

// --- user list ---

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users in your organization",
	Long: `List all users in your organization.

Displays a table of users with their email, name, role, and status.

Examples:
  ai-aas-org user list
  ai-aas-org user list --json`,
	RunE: runUserList,
}

func init() {
	userCmd.AddCommand(userListCmd)
}

func runUserList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	result, err := client.ListUsers(ctx, config.GetOrgID(), 0, 0)
	if err != nil {
		return wrapAPIError(err, "failed to list users")
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Users)
	}

	if len(result.Users) == 0 {
		output.InfoMsg("No users found in your organization.")
		fmt.Println()
		fmt.Println("Create a user with:")
		fmt.Println("  ai-aas-org user create --guided")
		return nil
	}

	// Build table
	headers := []string{"User ID", "Email", "Name", "Roles", "Status", "Created"}
	var rows [][]string
	for _, u := range result.Users {
		roles := strings.Join(u.Metadata.Roles, ", ")
		if roles == "" {
			roles = "-"
		}
		rows = append(rows, []string{
			u.ID,
			u.Email,
			u.Name,
			roles,
			output.StatusBadge(u.Status),
			u.CreatedAt.Format("2006-01-02"),
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d users\n", len(result.Users))
	return nil
}

// --- user create ---

var (
	userCreateEmail string
	userCreateName  string
	userCreateRole  string
)

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user in your organization",
	Long: `Create a new user in your organization.

The user will be created with the specified email and display name.
By default, users are created with the 'user' role.

Examples:
  ai-aas-org user create --user-email user@example.com --user-display-name "John Doe"
  ai-aas-org user create --guided`,
	RunE: runUserCreate,
}

func init() {
	userCmd.AddCommand(userCreateCmd)

	userCreateCmd.Flags().StringVarP(&userCreateEmail, "user-email", "e", "", "user email address")
	userCreateCmd.Flags().StringVarP(&userCreateName, "user-display-name", "n", "", "user display name")
	userCreateCmd.Flags().StringVarP(&userCreateRole, "role", "r", "user", "user role (user, admin)")
}

func runUserCreate(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	email := userCreateEmail
	name := userCreateName
	role := userCreateRole

	// Guided mode
	if IsGuidedMode() || (email == "" && name == "") {
		output.Header("Create New User")
		fmt.Println()

		var err error
		email, err = prompt.InputRequired("Email address")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}

		name, err = prompt.InputRequired("Display name")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}

		roleOpt, err := prompt.Select("Role", []prompt.SelectOption{
			{Label: "User", Value: "user", Description: "Standard user access"},
			{Label: "Admin", Value: "admin", Description: "Organization administrator"},
		})
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}
		role = roleOpt.Value

		fmt.Println()
	}

	// Validate
	if email == "" {
		return errors.NewUsageError("--user-email is required")
	}
	if name == "" {
		return errors.NewUsageError("--user-display-name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()
	result, err := client.CreateUser(ctx, config.GetOrgID(), &api.CreateUserRequest{
		Email: email,
		Name:  name,
		Role:  role,
	})
	if err != nil {
		return wrapAPIError(err, "failed to create user")
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	output.SuccessMsg("User created successfully!")
	fmt.Println()
	output.KeyValue("Email", result.User.Email)
	output.KeyValue("Name", result.User.Name)
	output.KeyValue("User ID", result.User.ID)

	if result.TemporaryPassword != "" {
		fmt.Println()
		output.Header("Temporary Password")
		fmt.Println(result.TemporaryPassword)
		fmt.Println()
		output.WarningMsg("Share this password with the user securely. It will only be shown once.")
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  • Create an API key for this user:  ai-aas-org apikey create --user-id %s --key-name \"API Key\"\n", result.User.ID)
	fmt.Printf("  • Grant model access:               ai-aas-org user models add --user %s\n", result.User.Email)

	return nil
}

// --- user show ---

var userShowCmd = &cobra.Command{
	Use:   "show <email-or-id>",
	Short: "Show details for a user",
	Long: `Show detailed information about a user.

You can specify the user by email address or user ID.

Examples:
  ai-aas-org user show user@example.com
  ai-aas-org user show usr_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runUserShow,
}

func init() {
	userCmd.AddCommand(userShowCmd)
}

func runUserShow(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	identifier := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	var user *api.User
	var err error

	// Try by email first, then by ID
	if isEmail(identifier) {
		user, err = client.GetUserByEmail(ctx, config.GetOrgID(), identifier)
	} else {
		user, err = client.GetUser(ctx, config.GetOrgID(), identifier)
	}

	if err != nil {
		return wrapAPIError(err, "failed to get user")
	}

	if IsJSONOutput() {
		return output.PrintJSON(user)
	}

	output.Header("User Details")
	output.KeyValue("Email", user.Email)
	output.KeyValue("Name", user.Name)
	output.KeyValue("User ID", user.ID)
	userRoles := strings.Join(user.Metadata.Roles, ", ")
	if userRoles == "" {
		userRoles = "-"
	}
	output.KeyValue("Roles", userRoles)
	output.KeyValue("Status", output.StatusBadge(user.Status))
	output.KeyValue("Created", user.CreatedAt.Format("2006-01-02 15:04:05"))
	output.KeyValue("Updated", user.UpdatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

// --- user delete ---

var (
	userDeleteForce bool
)

var userDeleteCmd = &cobra.Command{
	Use:   "delete <email-or-id>",
	Short: "Delete a user from your organization",
	Long: `Delete a user from your organization.

This will remove the user and revoke all their API keys.
This action cannot be undone.

Examples:
  ai-aas-org user delete user@example.com
  ai-aas-org user delete usr_abc123 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runUserDelete,
}

func init() {
	userCmd.AddCommand(userDeleteCmd)

	userDeleteCmd.Flags().BoolVar(&userDeleteForce, "force", false, "skip confirmation prompt")
}

func runUserDelete(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	identifier := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Get user first to confirm
	var user *api.User
	var err error

	if isEmail(identifier) {
		user, err = client.GetUserByEmail(ctx, config.GetOrgID(), identifier)
	} else {
		user, err = client.GetUser(ctx, config.GetOrgID(), identifier)
	}

	if err != nil {
		return wrapAPIError(err, "failed to find user")
	}

	// Confirm deletion
	if !userDeleteForce {
		output.WarningMsg("You are about to delete user: %s (%s)", user.Name, user.Email)
		fmt.Println()
		fmt.Println("This will:")
		fmt.Println("  • Remove the user from your organization")
		fmt.Println("  • Revoke all their API keys")
		fmt.Println("  • This action cannot be undone")
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

	// Delete user
	if err := client.DeleteUser(ctx, config.GetOrgID(), user.ID); err != nil {
		return wrapAPIError(err, "failed to delete user")
	}

	output.SuccessMsg("User %s has been deleted.", user.Email)
	return nil
}

// isEmail checks if a string looks like an email address.
func isEmail(s string) bool {
	for _, c := range s {
		if c == '@' {
			return true
		}
	}
	return false
}

// --- user set-token-policy ---

var (
	userSetTokenPolicyUser   string
	userSetTokenPolicyPolicy string
)

var userSetTokenPolicyCmd = &cobra.Command{
	Use:   "set-token-policy",
	Short: "Set a user's token rate-limit policy",
	Long: `Set a token rate-limit policy override for a user.

Use "inherit" to clear the override and use the org default.

Examples:
  ai-aas-org user set-token-policy --user user@example.com --policy "Standard"
  ai-aas-org user set-token-policy --user user@example.com --policy inherit
  ai-aas-org user set-token-policy --user user@example.com --policy no-limit`,
	RunE: runUserSetTokenPolicy,
}

func init() {
	userCmd.AddCommand(userSetTokenPolicyCmd)

	userSetTokenPolicyCmd.Flags().StringVar(&userSetTokenPolicyUser, "user", "", "user email or ID (required)")
	userSetTokenPolicyCmd.Flags().StringVar(&userSetTokenPolicyPolicy, "policy", "", "policy name, ID, 'inherit', or 'no-limit' (required)")
	userSetTokenPolicyCmd.MarkFlagRequired("user")
	userSetTokenPolicyCmd.MarkFlagRequired("policy")
}

func runUserSetTokenPolicy(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user
	var user *api.User
	var err error
	if isEmail(userSetTokenPolicyUser) {
		user, err = client.GetUserByEmail(ctx, config.GetOrgID(), userSetTokenPolicyUser)
	} else {
		user, err = client.GetUser(ctx, config.GetOrgID(), userSetTokenPolicyUser)
	}
	if err != nil {
		return wrapAPIError(err, "failed to find user")
	}

	// Handle "inherit" to clear override
	if userSetTokenPolicyPolicy == "inherit" {
		result, err := client.ClearUserTokenPolicy(ctx, config.GetOrgID(), user.ID)
		if err != nil {
			return wrapAPIError(err, "failed to clear token policy override")
		}

		if IsJSONOutput() {
			return output.PrintJSON(result)
		}

		output.SuccessMsg("Token policy override cleared for %s", user.Email)
		fmt.Println()
		output.KeyValue("Effective Policy", result.Policy.Name)
		output.KeyValue("Source", result.Source)
		return nil
	}

	// Resolve policy ID
	policyID := userSetTokenPolicyPolicy
	if policyID != "no-limit" {
		policy, err := findPolicyByNameOrID(ctx, client, policyID)
		if err != nil {
			return wrapAPIError(err, "failed to find token policy")
		}
		policyID = policy.ID
	}

	result, err := client.SetUserTokenPolicy(ctx, config.GetOrgID(), user.ID, policyID)
	if err != nil {
		return wrapAPIError(err, "failed to set token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	output.SuccessMsg("Token policy set for %s", user.Email)
	fmt.Println()
	output.KeyValue("Policy", result.Policy.Name)
	output.KeyValue("Source", result.Source)
	return nil
}

// --- user get-token-policy ---

var userGetTokenPolicyUser string

var userGetTokenPolicyCmd = &cobra.Command{
	Use:   "get-token-policy",
	Short: "Get a user's effective token policy",
	Long: `Get the effective token rate-limit policy for a user.

Shows whether the policy is from an override or inherited from the org default.

Examples:
  ai-aas-org user get-token-policy --user user@example.com
  ai-aas-org user get-token-policy --user usr_abc123`,
	RunE: runUserGetTokenPolicy,
}

func init() {
	userCmd.AddCommand(userGetTokenPolicyCmd)

	userGetTokenPolicyCmd.Flags().StringVar(&userGetTokenPolicyUser, "user", "", "user email or ID (required)")
	userGetTokenPolicyCmd.MarkFlagRequired("user")
}

func runUserGetTokenPolicy(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user
	var user *api.User
	var err error
	if isEmail(userGetTokenPolicyUser) {
		user, err = client.GetUserByEmail(ctx, config.GetOrgID(), userGetTokenPolicyUser)
	} else {
		user, err = client.GetUser(ctx, config.GetOrgID(), userGetTokenPolicyUser)
	}
	if err != nil {
		return wrapAPIError(err, "failed to find user")
	}

	result, err := client.GetUserTokenPolicy(ctx, config.GetOrgID(), user.ID)
	if err != nil {
		return wrapAPIError(err, "failed to get token policy")
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	output.Header("User Token Policy")
	output.KeyValue("User", user.Email)
	output.KeyValue("Policy", result.Policy.Name)
	output.KeyValue("Source", result.Source)
	fmt.Println()
	output.Header("Policy Limits")
	output.KeyValue("1h Limit", formatLimit(result.Policy.Limit1h))
	output.KeyValue("24h Limit", formatLimit(result.Policy.Limit24h))
	output.KeyValue("7d Limit", formatLimit(result.Policy.Limit7d))
	return nil
}

// --- user usage ---

var userUsageUser string

var userUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show a user's token usage",
	Long: `Show token usage for a user across all rate-limit windows.

Displays current usage, limits, and percentage for each window.

Examples:
  ai-aas-org user usage --user user@example.com
  ai-aas-org user usage --user usr_abc123`,
	RunE: runUserUsage,
}

func init() {
	userCmd.AddCommand(userUsageCmd)

	userUsageCmd.Flags().StringVar(&userUsageUser, "user", "", "user email or ID (required)")
	userUsageCmd.MarkFlagRequired("user")
}

func runUserUsage(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user
	var user *api.User
	var err error
	if isEmail(userUsageUser) {
		user, err = client.GetUserByEmail(ctx, config.GetOrgID(), userUsageUser)
	} else {
		user, err = client.GetUser(ctx, config.GetOrgID(), userUsageUser)
	}
	if err != nil {
		return wrapAPIError(err, "failed to find user")
	}

	usage, err := client.GetUserTokenUsage(ctx, config.GetOrgID(), user.ID)
	if err != nil {
		return wrapAPIError(err, "failed to get token usage")
	}

	if IsJSONOutput() {
		return output.PrintJSON(usage)
	}

	output.Header("Token Usage for " + user.Email)
	output.KeyValue("Policy", usage.PolicyName)
	fmt.Println()

	if len(usage.Windows) == 0 {
		output.InfoMsg("No usage data available.")
		return nil
	}

	headers := []string{"WINDOW", "USED", "LIMIT", "REMAINING", "USAGE %", "RESETS AT"}
	var rows [][]string
	for _, w := range usage.Windows {
		limitStr := "unlimited"
		remainingStr := "unlimited"
		pctStr := "-"
		if w.Limit > 0 {
			limitStr = formatTokenCount(w.Limit)
			remainingStr = formatTokenCount(w.Remaining)
			pctStr = fmt.Sprintf("%.1f%%", w.Percentage)
		}
		rows = append(rows, []string{
			w.Window,
			formatTokenCount(w.Used),
			limitStr,
			remainingStr,
			pctStr,
			w.ResetsAt,
		})
	}

	output.PrintTable(headers, rows)
	return nil
}
