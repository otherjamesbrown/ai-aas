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

// userModelsCmd represents the user models command
var userModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage model access for a user",
	Long: `Manage which AI models a user can access.

By default, new users have access to all models available to your organization.
You can restrict or grant access to specific models.

Examples:
  ai-aas-org user models list --user user@example.com
  ai-aas-org user models add --user user@example.com --model gpt-4
  ai-aas-org user models remove --user user@example.com --model gpt-4`,
}

func init() {
	userCmd.AddCommand(userModelsCmd)
}

// --- user models list ---

var userModelsListUser string

var userModelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List models a user can access",
	Long: `List all AI models that a user has access to.

Examples:
  ai-aas-org user models list --user user@example.com`,
	RunE: runUserModelsList,
}

func init() {
	userModelsCmd.AddCommand(userModelsListCmd)

	userModelsListCmd.Flags().StringVar(&userModelsListUser, "user", "", "user email or ID")
	userModelsListCmd.MarkFlagRequired("user")
}

func runUserModelsList(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user ID
	userID, err := resolveUserID(ctx, client, userModelsListUser)
	if err != nil {
		return err
	}

	// List user's model access
	result, err := client.ListUserModels(ctx, config.GetOrgID(), userID)
	if err != nil {
		return errors.NewOperationError("failed to list user models", err.Error())
	}

	if IsJSONOutput() {
		return output.PrintJSON(result.Models)
	}

	if len(result.Models) == 0 {
		output.InfoMsg("User has no model access configured.")
		fmt.Println()
		fmt.Println("Grant access with:")
		fmt.Println("  ai-aas-org user models add --user-id", userModelsListUser, "--model <model-id>")
		return nil
	}

	headers := []string{"Model", "Granted At", "Granted By"}
	var rows [][]string
	for _, m := range result.Models {
		rows = append(rows, []string{
			m.ModelName,
			m.GrantedAt,
			m.GrantedBy,
		})
	}

	output.PrintTable(headers, rows)
	fmt.Printf("\nTotal: %d models\n", len(result.Models))
	return nil
}

// --- user models add ---

var (
	userModelsAddUser  string
	userModelsAddModel string
	userModelsAddAll   bool
)

var userModelsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Grant a user access to a model",
	Long: `Grant a user access to an AI model.

Examples:
  ai-aas-org user models add --user user@example.com --model gpt-4
  ai-aas-org user models add --user user@example.com --all`,
	RunE: runUserModelsAdd,
}

func init() {
	userModelsCmd.AddCommand(userModelsAddCmd)

	userModelsAddCmd.Flags().StringVar(&userModelsAddUser, "user", "", "user email or ID")
	userModelsAddCmd.Flags().StringVar(&userModelsAddModel, "model", "", "model ID to grant access to")
	userModelsAddCmd.Flags().BoolVar(&userModelsAddAll, "all", false, "grant access to all available models")
	userModelsAddCmd.MarkFlagRequired("user")
}

func runUserModelsAdd(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	if userModelsAddModel == "" && !userModelsAddAll {
		// Guided mode
		if IsGuidedMode() {
			return runUserModelsAddGuided()
		}
		return errors.NewUsageError("--model or --all is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user ID
	userID, err := resolveUserID(ctx, client, userModelsAddUser)
	if err != nil {
		return err
	}

	if userModelsAddAll {
		// Grant access to all models
		if err := client.GrantAllModelsAccess(ctx, config.GetOrgID(), userID); err != nil {
			return errors.NewOperationError("failed to grant access", err.Error())
		}
		output.SuccessMsg("Granted access to all models for user %s", userModelsAddUser)
	} else {
		// Grant access to specific model
		if err := client.GrantModelAccess(ctx, config.GetOrgID(), userID, userModelsAddModel); err != nil {
			return errors.NewOperationError("failed to grant access", err.Error())
		}
		output.SuccessMsg("Granted access to model %s for user %s", userModelsAddModel, userModelsAddUser)
	}

	return nil
}

func runUserModelsAddGuided() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := newAPIClient()

	output.Header("Grant Model Access")
	fmt.Println()

	// Get user
	userEmail, err := prompt.InputRequired("User email")
	if err != nil {
		return errors.NewOperationError("failed to read input", err.Error())
	}

	userID, err := resolveUserID(ctx, client, userEmail)
	if err != nil {
		return err
	}

	// List available models
	models, err := client.ListModels(ctx, config.GetOrgID())
	if err != nil {
		return errors.NewOperationError("failed to list models", err.Error())
	}

	if len(models.Models) == 0 {
		output.WarningMsg("No models available in your organization.")
		return nil
	}

	// Build options
	options := make([]prompt.SelectOption, 0, len(models.Models)+1)
	options = append(options, prompt.SelectOption{
		Label:       "All Models",
		Value:       "__all__",
		Description: "Grant access to all available models",
	})
	for _, m := range models.Models {
		options = append(options, prompt.SelectOption{
			Label:       m.Name,
			Value:       m.ID,
			Description: m.Description,
		})
	}

	selected, err := prompt.Select("Select model to grant access", options)
	if err != nil {
		return errors.NewOperationError("failed to read selection", err.Error())
	}

	if selected.Value == "__all__" {
		if err := client.GrantAllModelsAccess(ctx, config.GetOrgID(), userID); err != nil {
			return errors.NewOperationError("failed to grant access", err.Error())
		}
		output.SuccessMsg("Granted access to all models for user %s", userEmail)
	} else {
		if err := client.GrantModelAccess(ctx, config.GetOrgID(), userID, selected.Value); err != nil {
			return errors.NewOperationError("failed to grant access", err.Error())
		}
		output.SuccessMsg("Granted access to model %s for user %s", selected.Label, userEmail)
	}

	return nil
}

// --- user models remove ---

var (
	userModelsRemoveUser  string
	userModelsRemoveModel string
)

var userModelsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Revoke a user's access to a model",
	Long: `Revoke a user's access to an AI model.

Examples:
  ai-aas-org user models remove --user user@example.com --model gpt-4`,
	RunE: runUserModelsRemove,
}

func init() {
	userModelsCmd.AddCommand(userModelsRemoveCmd)

	userModelsRemoveCmd.Flags().StringVar(&userModelsRemoveUser, "user", "", "user email or ID")
	userModelsRemoveCmd.Flags().StringVar(&userModelsRemoveModel, "model", "", "model ID to revoke access from")
	userModelsRemoveCmd.MarkFlagRequired("user")
	userModelsRemoveCmd.MarkFlagRequired("model")
}

func runUserModelsRemove(cmd *cobra.Command, args []string) error {
	if err := requireConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := newAPIClient()

	// Resolve user ID
	userID, err := resolveUserID(ctx, client, userModelsRemoveUser)
	if err != nil {
		return err
	}

	// Confirm
	confirmed, err := prompt.Confirm(
		fmt.Sprintf("Revoke access to model %s for user %s?", userModelsRemoveModel, userModelsRemoveUser),
		false,
	)
	if err != nil {
		return errors.NewOperationError("failed to read confirmation", err.Error())
	}
	if !confirmed {
		output.InfoMsg("Cancelled.")
		return nil
	}

	if err := client.RevokeModelAccess(ctx, config.GetOrgID(), userID, userModelsRemoveModel); err != nil {
		return errors.NewOperationError("failed to revoke access", err.Error())
	}

	output.SuccessMsg("Revoked access to model %s for user %s", userModelsRemoveModel, userModelsRemoveUser)
	return nil
}

// resolveUserID resolves a user identifier (email or ID) to a user ID.
func resolveUserID(ctx context.Context, client *api.Client, identifier string) (string, error) {
	if isEmail(identifier) {
		user, err := client.GetUserByEmail(ctx, config.GetOrgID(), identifier)
		if err != nil {
			return "", errors.NewOperationError("failed to find user", err.Error())
		}
		return user.ID, nil
	}
	// Assume it's already a user ID
	return identifier, nil
}
