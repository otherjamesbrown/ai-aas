// User model access CLI commands.
//
// Purpose:
//   CLI commands for managing user-level model access control (Spec 022).
//
// Requirements Reference:
//   - specs/022-user-model-access-control/spec.md#US-004 (CLI Commands)

package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// ModelAccessCommand creates the model-access subcommand group.
func ModelAccessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model-access",
		Short: "Manage user model access",
		Long: `Manage user-level model access control.

Users can have two access modes:
  - auto_grant: Access to all models in the organization (default)
  - restricted: Access only to explicitly granted models

Examples:
  # Show user's model access
  ai-aas-cli user model-access show --org-id myorg --email user@example.com

  # Set user to restricted mode
  ai-aas-cli user model-access set-mode --org-id myorg --email user@example.com --mode restricted

  # Grant access to a model
  ai-aas-cli user model-access grant --org-id myorg --email user@example.com --model gpt-4

  # Revoke access to a model
  ai-aas-cli user model-access revoke --org-id myorg --email user@example.com --model gpt-4`,
	}

	cmd.AddCommand(modelAccessShowCommand())
	cmd.AddCommand(modelAccessSetModeCommand())
	cmd.AddCommand(modelAccessGrantCommand())
	cmd.AddCommand(modelAccessRevokeCommand())
	cmd.AddCommand(modelAccessListCommand())
	cmd.AddCommand(modelAccessGrantAllCommand())
	cmd.AddCommand(modelAccessMigrateCommand())

	return cmd
}

func modelAccessShowCommand() *cobra.Command {
	var flagOrgID, flagUserID, flagEmail, flagFormat string
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show user's model access configuration",
		Long:  "Display the access mode and granted models for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagUserID == "" && flagEmail == "" {
				return errors.NewValidationError("--user-id or --email required", "Provide user identifier")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			// Resolve user ID from email if needed
			userID := flagUserID
			if userID == "" {
				user, err := client.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
				if err != nil {
					return errors.NewOperationError(fmt.Sprintf("find user: %v", err), "Verify email exists")
				}
				userID = user.UserID
			}

			access, err := client.GetUserModelAccess(cmd.Context(), flagOrgID, userID)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("get model access: %v", err), "Verify permissions")
			}

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(access)
			}

			// Table output
			fmt.Printf("User Model Access:\n")
			fmt.Printf("  User ID:     %s\n", access.UserID)
			fmt.Printf("  Org ID:      %s\n", access.OrgID)
			fmt.Printf("  Access Mode: %s\n", access.AccessMode)
			fmt.Println()

			if len(access.GrantedModels) == 0 {
				fmt.Println("  No model grants configured.")
			} else {
				fmt.Printf("  Granted Models (%d):\n", len(access.GrantedModels))
				headers := []string{"Model", "Granted At", "Expires At"}
				var rows [][]string
				for _, g := range access.GrantedModels {
					expiresAt := "-"
					if g.ExpiresAt != nil {
						expiresAt = *g.ExpiresAt
					}
					rows = append(rows, []string{g.ModelName, g.GrantedAt, expiresAt})
				}
				return output.PrintTable(headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	return cmd
}

func modelAccessSetModeCommand() *cobra.Command {
	var flagOrgID, flagUserID, flagEmail, flagMode, flagFormat string
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "set-mode",
		Short: "Set user's access mode",
		Long: `Set the access mode for a user.

Modes:
  - auto_grant: User has access to all models in the organization
  - restricted: User only has access to explicitly granted models`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagUserID == "" && flagEmail == "" {
				return errors.NewValidationError("--user-id or --email required", "Provide user identifier")
			}
			if flagMode != "restricted" && flagMode != "auto_grant" {
				return errors.NewValidationError("--mode must be 'restricted' or 'auto_grant'", "")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			userID := flagUserID
			if userID == "" {
				user, err := client.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
				if err != nil {
					return errors.NewOperationError(fmt.Sprintf("find user: %v", err), "Verify email exists")
				}
				userID = user.UserID
			}

			access, err := client.SetUserAccessMode(cmd.Context(), flagOrgID, userID, flagMode)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("set access mode: %v", err), "Verify permissions")
			}

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(access)
			}

			fmt.Printf("Access mode updated successfully:\n")
			fmt.Printf("  User ID:     %s\n", access.UserID)
			fmt.Printf("  Access Mode: %s\n", access.AccessMode)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email")
	cmd.Flags().StringVar(&flagMode, "mode", "", "Access mode: restricted or auto_grant (required)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	_ = cmd.MarkFlagRequired("mode")

	return cmd
}

func modelAccessGrantCommand() *cobra.Command {
	var flagOrgID, flagUserID, flagEmail, flagModel, flagExpiresIn, flagFormat string
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "grant",
		Short: "Grant model access to a user",
		Long:  "Grant access to a specific model for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagUserID == "" && flagEmail == "" {
				return errors.NewValidationError("--user-id or --email required", "Provide user identifier")
			}
			if flagModel == "" {
				return errors.NewValidationError("--model is required", "Provide model name")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			userID := flagUserID
			if userID == "" {
				user, err := client.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
				if err != nil {
					return errors.NewOperationError(fmt.Sprintf("find user: %v", err), "Verify email exists")
				}
				userID = user.UserID
			}

			// Parse expires-in duration if provided
			var expiresAt *string
			if flagExpiresIn != "" {
				duration, err := time.ParseDuration(flagExpiresIn)
				if err != nil {
					return errors.NewValidationError(fmt.Sprintf("invalid --expires-in: %v", err), "Use format like '24h', '7d', '30d'")
				}
				expiry := time.Now().Add(duration).Format(time.RFC3339)
				expiresAt = &expiry
			}

			grant, err := client.GrantModelAccess(cmd.Context(), flagOrgID, userID, flagModel, expiresAt)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("grant model access: %v", err), "Verify permissions")
			}

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(grant)
			}

			fmt.Printf("Model access granted successfully:\n")
			fmt.Printf("  Grant ID:   %s\n", grant.GrantID)
			fmt.Printf("  Model:      %s\n", grant.ModelName)
			fmt.Printf("  Granted At: %s\n", grant.GrantedAt)
			if grant.ExpiresAt != nil {
				fmt.Printf("  Expires At: %s\n", *grant.ExpiresAt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email")
	cmd.Flags().StringVar(&flagModel, "model", "", "Model name (required)")
	cmd.Flags().StringVar(&flagExpiresIn, "expires-in", "", "Grant expiration duration (e.g., '24h', '7d')")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	return cmd
}

func modelAccessRevokeCommand() *cobra.Command {
	var flagOrgID, flagUserID, flagEmail, flagModel, flagFormat string
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke model access from a user",
		Long:  "Revoke access to a specific model from a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagUserID == "" && flagEmail == "" {
				return errors.NewValidationError("--user-id or --email required", "Provide user identifier")
			}
			if flagModel == "" {
				return errors.NewValidationError("--model is required", "Provide model name")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			userID := flagUserID
			if userID == "" {
				user, err := client.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
				if err != nil {
					return errors.NewOperationError(fmt.Sprintf("find user: %v", err), "Verify email exists")
				}
				userID = user.UserID
			}

			if err := client.RevokeModelAccess(cmd.Context(), flagOrgID, userID, flagModel); err != nil {
				return errors.NewOperationError(fmt.Sprintf("revoke model access: %v", err), "Verify permissions")
			}

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(map[string]interface{}{
					"success": true,
					"model":   flagModel,
					"message": "Model access revoked",
				})
			}

			fmt.Printf("Model access revoked successfully: %s\n", flagModel)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email")
	cmd.Flags().StringVar(&flagModel, "model", "", "Model name (required)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	return cmd
}

func modelAccessListCommand() *cobra.Command {
	var flagOrgID, flagUserID, flagEmail, flagFormat string
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user's model grants",
		Long:  "List all model grants for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagUserID == "" && flagEmail == "" {
				return errors.NewValidationError("--user-id or --email required", "Provide user identifier")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			userID := flagUserID
			if userID == "" {
				user, err := client.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
				if err != nil {
					return errors.NewOperationError(fmt.Sprintf("find user: %v", err), "Verify email exists")
				}
				userID = user.UserID
			}

			grants, err := client.ListUserModelGrants(cmd.Context(), flagOrgID, userID)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("list grants: %v", err), "Verify permissions")
			}

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(grants)
			}

			if len(grants) == 0 {
				fmt.Println("No model grants found.")
				return nil
			}

			headers := []string{"Model", "Grant ID", "Granted At", "Expires At"}
			var rows [][]string
			for _, g := range grants {
				expiresAt := "-"
				if g.ExpiresAt != nil {
					expiresAt = *g.ExpiresAt
				}
				rows = append(rows, []string{g.ModelName, g.GrantID, g.GrantedAt, expiresAt})
			}
			return output.PrintTable(headers, rows)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	return cmd
}

func modelAccessGrantAllCommand() *cobra.Command {
	var flagOrgID, flagUserID, flagEmail, flagFormat string
	var flagModels []string
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "grant-all",
		Short: "Grant multiple models to a user",
		Long: `Grant access to multiple models at once.

This is useful when setting a user to restricted mode and wanting to grant
them access to all currently available models.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagUserID == "" && flagEmail == "" {
				return errors.NewValidationError("--user-id or --email required", "Provide user identifier")
			}
			if len(flagModels) == 0 {
				return errors.NewValidationError("--models is required", "Provide at least one model name")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			userID := flagUserID
			if userID == "" {
				user, err := client.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
				if err != nil {
					return errors.NewOperationError(fmt.Sprintf("find user: %v", err), "Verify email exists")
				}
				userID = user.UserID
			}

			result, err := client.GrantAllCurrentModels(cmd.Context(), flagOrgID, userID, flagModels)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("grant models: %v", err), "Verify permissions")
			}

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(result)
			}

			fmt.Printf("Models granted successfully:\n")
			fmt.Printf("  Granted Count: %d\n", result.GrantedCount)
			fmt.Printf("  Granted At:    %s\n", result.GrantedAt)
			fmt.Printf("  Models:\n")
			for _, m := range result.Models {
				fmt.Printf("    - %s\n", m)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email")
	cmd.Flags().StringSliceVar(&flagModels, "models", nil, "Model names to grant (comma-separated or multiple flags)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	return cmd
}

func modelAccessMigrateCommand() *cobra.Command {
	var flagOrgID, flagMode, flagFormat string
	var flagModels []string
	var flagDryRun bool
	var flagUserOrgEndpoint, flagAPIKey string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate existing users to model access control",
		Long: `Migrate all users in an organization to the model access control system.

This command is useful when enabling model access control for an existing org.
It can either:
  1. Set all users to auto_grant mode (legacy behavior, no restrictions)
  2. Set all users to restricted mode and grant them specific models

Examples:
  # Set all users to auto_grant (no restrictions)
  ai-aas-cli user model-access migrate --org-id myorg --mode auto_grant

  # Set all users to restricted mode with specific models
  ai-aas-cli user model-access migrate --org-id myorg --mode restricted --models gpt-4,gpt-3.5

  # Dry run to see what would happen
  ai-aas-cli user model-access migrate --org-id myorg --mode restricted --models gpt-4 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("load config: %v", err), "Check configuration")
			}

			applyOverrides(cfg, flagUserOrgEndpoint, flagAPIKey, flagFormat)

			if cfg.UserOrgEndpoint == "" {
				return errors.NewValidationError("user-org-service endpoint required", "Set via --user-org-endpoint or config")
			}
			if flagOrgID == "" {
				return errors.NewValidationError("--org-id is required", "Provide organization ID or slug")
			}
			if flagMode != "restricted" && flagMode != "auto_grant" {
				return errors.NewValidationError("--mode must be 'restricted' or 'auto_grant'", "")
			}
			if flagMode == "restricted" && len(flagModels) == 0 {
				return errors.NewValidationError("--models required when using restricted mode", "Provide models to grant")
			}

			httpClient, err := createHTTPClient(cfg)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
			}
			checker := createHealthCheckerWithHTTPClient(httpClient)
			if _, err := checker.CheckRequired(cmd.Context(), map[string]string{"user-org-service": cfg.UserOrgEndpoint}); err != nil {
				return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
			}

			client := createUserOrgClientWithHTTPClient(cfg, httpClient)

			// List all users in the org
			users, err := client.ListUsers(cmd.Context(), flagOrgID)
			if err != nil {
				return errors.NewOperationError(fmt.Sprintf("list users: %v", err), "Verify permissions")
			}

			if len(users) == 0 {
				fmt.Println("No users found in organization.")
				return nil
			}

			fmt.Printf("Found %d users in organization %s\n", len(users), flagOrgID)
			if flagDryRun {
				fmt.Println("[DRY RUN] No changes will be made.")
			}
			fmt.Println()

			var successCount, errorCount int
			for i, user := range users {
				fmt.Printf("[%d/%d] Processing user: %s (%s)\n", i+1, len(users), user.Email, user.UserID)

				if flagDryRun {
					fmt.Printf("  Would set mode to: %s\n", flagMode)
					if flagMode == "restricted" && len(flagModels) > 0 {
						fmt.Printf("  Would grant models: %v\n", flagModels)
					}
					successCount++
					continue
				}

				// Set access mode
				_, err := client.SetUserAccessMode(cmd.Context(), flagOrgID, user.UserID, flagMode)
				if err != nil {
					fmt.Printf("  ERROR setting access mode: %v\n", err)
					errorCount++
					continue
				}
				fmt.Printf("  Set access mode: %s\n", flagMode)

				// Grant models if in restricted mode
				if flagMode == "restricted" && len(flagModels) > 0 {
					result, err := client.GrantAllCurrentModels(cmd.Context(), flagOrgID, user.UserID, flagModels)
					if err != nil {
						fmt.Printf("  ERROR granting models: %v\n", err)
						errorCount++
						continue
					}
					fmt.Printf("  Granted %d models\n", result.GrantedCount)
				}

				successCount++
			}

			fmt.Println()
			fmt.Printf("Migration complete:\n")
			fmt.Printf("  Successful: %d\n", successCount)
			fmt.Printf("  Errors:     %d\n", errorCount)

			if cfg.OutputFormat == "json" {
				return output.PrintJSON(map[string]interface{}{
					"totalUsers":   len(users),
					"successful":   successCount,
					"errors":       errorCount,
					"mode":         flagMode,
					"models":       flagModels,
					"dryRun":       flagDryRun,
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagMode, "mode", "", "Access mode: restricted or auto_grant (required)")
	cmd.Flags().StringSliceVar(&flagModels, "models", nil, "Models to grant when using restricted mode")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without making them")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key")

	_ = cmd.MarkFlagRequired("mode")

	return cmd
}

// applyOverrides applies flag overrides to config
func applyOverrides(cfg *config.Config, endpoint, apiKey, format string) {
	if endpoint != "" {
		cfg.UserOrgEndpoint = endpoint
	}
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	if format != "" {
		cfg.OutputFormat = format
	}
}

// createHTTPClient creates an HTTP client with TLS configuration from config.
// This should be called once per command and the client reused for all HTTP operations.
func createHTTPClient(cfg *config.Config) (*http.Client, error) {
	return config.CreateHTTPClientWithOptions(cfg.CACertFile, cfg.TLSInsecure)
}

// createUserOrgClientWithHTTPClient creates a user-org client with a pre-configured HTTP client.
func createUserOrgClientWithHTTPClient(cfg *config.Config, httpClient *http.Client) *userorg.Client {
	return userorg.NewClientWithHTTPClient(cfg.UserOrgEndpoint, cfg.APIKey, httpClient)
}

// createHealthCheckerWithHTTPClient creates a health checker with a pre-configured HTTP client.
func createHealthCheckerWithHTTPClient(httpClient *http.Client) *health.Checker {
	return health.NewCheckerWithClient(5*time.Second, httpClient)
}
