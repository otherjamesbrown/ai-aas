package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/prompt"
)

// apikeyCmd represents the apikey command
var apikeyCmd = &cobra.Command{
	Use:     "apikey",
	Aliases: []string{"key", "keys"},
	Short:   "Manage API keys for users",
	Long: `Manage API keys for users in your organization.

API keys allow users to authenticate with the AI-as-a-Service platform.
Each user can have multiple API keys with different names and expiration dates.

Examples:
  ai-aas-org apikey list
  ai-aas-org apikey create --user-id <user-id> --key-name "Production Key"
  ai-aas-org apikey create --email user@example.com --key-name "Production Key"
  ai-aas-org apikey delete key_abc123`,
}

func init() {
	rootCmd.AddCommand(apikeyCmd)
}

// --- apikey list ---

var apikeyListUser string

var apikeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	Long: `List API keys in your organization.

Without --user-id, lists all API keys in the organization.
With --user-id, lists only keys for that user.

Examples:
  ai-aas-org apikey list
  ai-aas-org apikey list --user-id usr_abc123`,
	RunE: runAPIKeyList,
}

func init() {
	apikeyCmd.AddCommand(apikeyListCmd)

	apikeyListCmd.Flags().StringVar(&apikeyListUser, "user-id", "", "filter by user ID")
}

func runAPIKeyList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	var result *api.ListAPIKeysResponse
	var err error

	if apikeyListUser != "" {
		userID, resolveErr := resolveUserID(ctx, client, apikeyListUser)
		if resolveErr != nil {
			return resolveErr
		}
		result, err = client.ListUserAPIKeys(ctx, config.GetOrgID(), userID)
	} else {
		result, err = client.ListAPIKeys(ctx, config.GetOrgID())
	}

	if err != nil {
		return errors.NewOperationError("failed to list API keys", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.APIKeys)
	}

	if len(result.APIKeys) == 0 {
		output.InfoMsg("No API keys found.")
		fmt.Println()
		fmt.Println("Create an API key with:")
		fmt.Println("  ai-aas-org apikey create --user-id <user-id> --key-name <key-name>")
		fmt.Println("  ai-aas-org apikey create --email <email> --key-name <key-name>")
		return nil
	}

	headers := []string{"Prefix", "Name", "User", "Status", "Last Used", "Expires"}
	var rows [][]string
	for _, k := range result.APIKeys {
		lastUsed := "Never"
		if k.LastUsedAt != nil {
			lastUsed = k.LastUsedAt.Format("2006-01-02")
		}
		expires := "Never"
		if k.ExpiresAt != nil {
			expires = k.ExpiresAt.Format("2006-01-02")
		}
		rows = append(rows, []string{
			k.Prefix + "...",
			k.Name,
			k.UserEmail,
			output.StatusBadge(k.Status),
			lastUsed,
			expires,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d API keys\n", len(result.APIKeys))
	return nil
}

// --- apikey create ---

var (
	apikeyCreateUser    string
	apikeyCreateEmail   string
	apikeyCreateName    string
	apikeyCreateExpires string
)

var apikeyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key for a user",
	Long: `Create a new API key for a user.

The key will only be displayed once after creation. Make sure to copy it
and share it securely with the user.

You can identify the user by either --user-id or --email.

Examples:
  ai-aas-org apikey create --user-id <user-id> --key-name "Production Key"
  ai-aas-org apikey create --email user@example.com --key-name "Production Key"
  ai-aas-org apikey create --user-id <user-id> --key-name "Dev Key" --expires 30d
  ai-aas-org apikey create --guided`,
	RunE: runAPIKeyCreate,
}

func init() {
	apikeyCmd.AddCommand(apikeyCreateCmd)

	apikeyCreateCmd.Flags().StringVar(&apikeyCreateUser, "user-id", "", "user ID (use 'ai-aas-org user list' to find)")
	apikeyCreateCmd.Flags().StringVar(&apikeyCreateEmail, "email", "", "user email (alternative to --user-id)")
	apikeyCreateCmd.Flags().StringVar(&apikeyCreateName, "key-name", "", "key name/description")
	apikeyCreateCmd.Flags().StringVar(&apikeyCreateExpires, "expires", "", "expiration (e.g., 30d, 90d, 1y, never)")
}

func runAPIKeyCreate(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	userID := apikeyCreateUser
	email := apikeyCreateEmail
	name := apikeyCreateName
	expires := apikeyCreateExpires

	// Guided mode
	if IsGuidedMode() || (userID == "" && email == "" && name == "") {
		output.Header("Create API Key")
		fmt.Println()

		var err error
		email, err = prompt.InputRequired("User email")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}

		name, err = prompt.InputRequired("Key name (e.g., 'Production', 'Development')")
		if err != nil {
			return errors.NewOperationError("failed to read input", err.Error())
		}

		expiresOpt, err := prompt.Select("Expiration", []prompt.SelectOption{
			{Label: "Never", Value: "never", Description: "Key never expires"},
			{Label: "30 days", Value: "30d", Description: "Expires in 30 days"},
			{Label: "90 days", Value: "90d", Description: "Expires in 90 days"},
			{Label: "1 year", Value: "365d", Description: "Expires in 1 year"},
		})
		if err != nil {
			return errors.NewOperationError("failed to read selection", err.Error())
		}
		expires = expiresOpt.Value
		fmt.Println()
	}

	// Validate - need either user-id or email
	if userID == "" && email == "" {
		return errors.NewUsageError("either --user-id or --email is required")
	}
	if name == "" {
		return errors.NewUsageError("--key-name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user ID from email if needed
	resolvedUserID := userID
	if resolvedUserID == "" && email != "" {
		var err error
		resolvedUserID, err = resolveUserID(ctx, client, email)
		if err != nil {
			return err
		}
	}

	result, err := client.CreateAPIKey(ctx, config.GetOrgID(), &api.CreateAPIKeyRequest{
		Name:      name,
		UserID:    resolvedUserID,
		ExpiresIn: expires,
	})
	if err != nil {
		return errors.NewOperationError("failed to create API key", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	output.SuccessMsg("API key created successfully!")
	fmt.Println()

	output.Box("API Key", result.Token)
	fmt.Println()
	output.WarningMsg("This key will only be shown once. Copy it now and share it securely.")
	fmt.Println()

	output.KeyValue("Key ID", result.KeyID)
	output.KeyValue("Name", name)
	output.KeyValue("Status", result.Status)
	output.KeyValue("Expires", "Never") // TODO: Parse from response when supported

	fmt.Println()
	fmt.Println("To use this key:")
	fmt.Printf("  export AI_AAS_API_KEY=\"%s\"\n", result.Token)

	return nil
}

// --- apikey delete ---

var apikeyDeleteForce bool

var apikeyDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete (revoke) an API key",
	Long: `Delete (revoke) an API key.

The key will be immediately invalidated and cannot be used anymore.
This action cannot be undone.

Examples:
  ai-aas-org apikey delete key_abc123
  ai-aas-org apikey delete key_abc123 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runAPIKeyDelete,
}

func init() {
	apikeyCmd.AddCommand(apikeyDeleteCmd)

	apikeyDeleteCmd.Flags().BoolVar(&apikeyDeleteForce, "force", false, "skip confirmation")
}

func runAPIKeyDelete(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	keyID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Get key details for confirmation
	key, err := client.GetAPIKey(ctx, config.GetOrgID(), keyID)
	if err != nil {
		return errors.NewOperationError("failed to find API key", err.Error())
	}

	// Confirm
	if !apikeyDeleteForce {
		output.WarningMsg("You are about to delete API key: %s (%s)", key.Name, key.Prefix+"...")
		fmt.Println()
		fmt.Println("This will:")
		fmt.Println("  • Immediately invalidate the key")
		fmt.Println("  • Affect user:", key.UserEmail)
		fmt.Println("  • This action cannot be undone")
		fmt.Println()

		confirmed, err := prompt.Confirm("Delete this API key?", false)
		if err != nil {
			return errors.NewOperationError("failed to read confirmation", err.Error())
		}
		if !confirmed {
			output.InfoMsg("Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteAPIKey(ctx, config.GetOrgID(), keyID); err != nil {
		return errors.NewOperationError("failed to delete API key", err.Error())
	}

	output.SuccessMsg("API key %s has been deleted.", key.Name)
	return nil
}

// --- apikey rotate ---

var apikeyRotateCmd = &cobra.Command{
	Use:   "rotate <key-id>",
	Short: "Rotate an API key",
	Long: `Rotate an API key by creating a new one and revoking the old one.

This is useful for security best practices or if you suspect a key
may have been compromised.

Examples:
  ai-aas-org apikey rotate key_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runAPIKeyRotate,
}

func init() {
	apikeyCmd.AddCommand(apikeyRotateCmd)
}

func runAPIKeyRotate(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	keyID := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Get key details for confirmation
	key, err := client.GetAPIKey(ctx, config.GetOrgID(), keyID)
	if err != nil {
		return errors.NewOperationError("failed to find API key", err.Error())
	}

	// Confirm
	output.WarningMsg("You are about to rotate API key: %s (%s)", key.Name, key.Prefix+"...")
	fmt.Println()
	fmt.Println("This will:")
	fmt.Println("  • Create a new API key with the same name")
	fmt.Println("  • Immediately revoke the old key")
	fmt.Println("  • Affect user:", key.UserEmail)
	fmt.Println()

	confirmed, err := prompt.Confirm("Rotate this API key?", false)
	if err != nil {
		return errors.NewOperationError("failed to read confirmation", err.Error())
	}
	if !confirmed {
		output.InfoMsg("Rotation cancelled.")
		return nil
	}

	result, err := client.RotateAPIKey(ctx, config.GetOrgID(), keyID)
	if err != nil {
		return errors.NewOperationError("failed to rotate API key", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result)
	}

	output.SuccessMsg("API key rotated successfully!")
	fmt.Println()

	output.Box("New API Key", result.NewAPIKey.Token)
	fmt.Println()
	output.WarningMsg("This key will only be shown once. Copy it now and share it securely.")
	fmt.Println()

	output.KeyValue("Old Key ID", result.OldKeyID)
	output.KeyValue("New Key ID", result.NewAPIKey.KeyID)

	return nil
}
