// Package commands provides user management commands.
//
// Purpose:
//
//	User lifecycle commands: list, create, update, delete with idempotent operations
//	and batch processing.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-002 (Day-2 Management)
//
package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/health"
	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/output"
)

// UserCommand creates the user command group.
func UserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
		Long:  "Manage users: list, create, update, delete",
	}

	cmd.AddCommand(userListCommand())
	cmd.AddCommand(userCreateCommand())
	cmd.AddCommand(userUpdateCommand())
	cmd.AddCommand(userDeleteCommand())

	return cmd
}

func userListCommand() *cobra.Command {
	var flagOrgID string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Long:  "List users in an organization with structured output (table, json, csv)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserList(cmd, args, flagOrgID, flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
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

func runUserList(cmd *cobra.Command, args []string, flagOrgID, flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string) error {
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

	// Create client and list users
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	users, err := userOrgClient.ListUsers(cmd.Context(), flagOrgID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list users: %v", err),
			"Verify your API key is valid and you have permission to list users in this organization.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "user_list",
		Command:     fmt.Sprintf("user list --org-id=%s", flagOrgID),
		Outcome:     "success",
		Duration:    time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(users)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"userId", "email", "displayName", "status", "mfaEnrolled", "createdAt"}
		var rows [][]string
		for _, user := range users {
			mfaEnrolled := "false"
			if user.MFAEnrolled {
				mfaEnrolled = "true"
			}
			rows = append(rows, []string{
				user.UserID,
				user.Email,
				user.DisplayName,
				user.Status,
				mfaEnrolled,
				user.CreatedAt,
			})
		}
		return output.PrintTable(headers, rows)
	} else {
		headers := []string{"User ID", "Email", "Display Name", "Status", "MFA Enrolled", "Created At"}
		var rows [][]string
		for _, user := range users {
			mfaEnrolled := "No"
			if user.MFAEnrolled {
				mfaEnrolled = "Yes"
			}
			rows = append(rows, []string{
				user.UserID,
				user.Email,
				user.DisplayName,
				user.Status,
				mfaEnrolled,
				user.CreatedAt,
			})
		}
		if len(rows) == 0 && !cfg.Quiet {
			fmt.Println("No users found.")
			return nil
		}
		return output.PrintTable(headers, rows)
	}
}

func userCreateCommand() *cobra.Command {
	var flagOrgID string
	var flagEmail string
	var flagRoles []string
	var flagExpiresInHours int
	var flagUpsert bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create user",
		Long:  "Create (invite) a user with idempotent support (--upsert flag)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserCreate(cmd, args, flagOrgID, flagEmail, flagRoles, flagExpiresInHours, flagUpsert,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email (required)")
	cmd.Flags().StringSliceVar(&flagRoles, "roles", []string{}, "User roles")
	cmd.Flags().IntVar(&flagExpiresInHours, "expires-in-hours", 72, "Invite expiration in hours (default: 72)")
	cmd.Flags().BoolVar(&flagUpsert, "upsert", false, "Update user if already exists (idempotent)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runUserCreate(cmd *cobra.Command, args []string, flagOrgID, flagEmail string, flagRoles []string, flagExpiresInHours int, flagUpsert bool,
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
	if flagEmail == "" {
		return errors.NewValidationError(
			"--email is required",
			"Provide user email with --email flag",
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

	// Check if user exists (for upsert)
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	var user *userorg.UserResponse
	if flagUpsert {
		existingUser, err := userOrgClient.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
		if err == nil {
			// User exists, return existing user
			user = existingUser
			if !cfg.Quiet {
				fmt.Printf("User already exists (upsert mode):\n")
				fmt.Printf("  User ID: %s\n", user.UserID)
				fmt.Printf("  Email: %s\n", user.Email)
			}
		}
	}

	// Create/invite user if not found or not upsert
	if user == nil {
		req := userorg.InviteUserRequest{
			Email:          flagEmail,
			Roles:          flagRoles,
			ExpiresInHours: flagExpiresInHours,
		}

		invitedUser, err := userOrgClient.InviteUser(cmd.Context(), flagOrgID, req)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to invite user: %v", err),
				"Verify your API key is valid and you have permission to invite users.",
			)
		}
		user = invitedUser
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "user_create",
		Command:     fmt.Sprintf("user create --org-id=%s --email=%s", flagOrgID, flagEmail),
		Parameters: map[string]interface{}{
			"orgId":  flagOrgID,
			"email":  flagEmail,
			"userId": user.UserID,
			"upsert": flagUpsert,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(user)
	} else if cfg.OutputFormat == "csv" {
		mfaEnrolled := "false"
		if user.MFAEnrolled {
			mfaEnrolled = "true"
		}
		headers := []string{"userId", "email", "displayName", "status", "mfaEnrolled", "createdAt"}
		rows := [][]string{{
			user.UserID,
			user.Email,
			user.DisplayName,
			user.Status,
			mfaEnrolled,
			user.CreatedAt,
		}}
		return output.PrintTable(headers, rows)
	} else {
		if !cfg.Quiet {
			if flagUpsert && user != nil {
				fmt.Printf("User exists (upsert mode):\n")
			} else {
				fmt.Printf("User invited successfully:\n")
			}
			fmt.Printf("  User ID: %s\n", user.UserID)
			fmt.Printf("  Email: %s\n", user.Email)
			fmt.Printf("  Status: %s\n", user.Status)
		}
		if cfg.OutputFormat == "table" {
			mfaEnrolled := "No"
			if user.MFAEnrolled {
				mfaEnrolled = "Yes"
			}
			headers := []string{"User ID", "Email", "Display Name", "Status", "MFA Enrolled", "Created At"}
			rows := [][]string{{
				user.UserID,
				user.Email,
				user.DisplayName,
				user.Status,
				mfaEnrolled,
				user.CreatedAt,
			}}
			return output.PrintTable(headers, rows)
		}
	}
	return nil
}

func userUpdateCommand() *cobra.Command {
	var flagOrgID string
	var flagUserID string
	var flagEmail string
	var flagDisplayName string
	var flagStatus string
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update user",
		Long:  "Update a user's display name, status, or metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserUpdate(cmd, args, flagOrgID, flagUserID, flagEmail, flagDisplayName, flagStatus,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID (required if --email not provided)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email (required if --user-id not provided)")
	cmd.Flags().StringVar(&flagDisplayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&flagStatus, "status", "", "User status (active, suspended)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runUserUpdate(cmd *cobra.Command, args []string, flagOrgID, flagUserID, flagEmail, flagDisplayName, flagStatus string,
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

	// Resolve user ID
	resolvedUserID := flagUserID
	if resolvedUserID == "" {
		if flagEmail == "" {
			return errors.NewValidationError(
				"either --user-id or --email is required",
				"Provide user ID with --user-id or email with --email flag",
			)
		}

		// Get user by email
		userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
		user, err := userOrgClient.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to find user by email: %v", err),
				"Verify the email exists in this organization.",
			)
		}
		resolvedUserID = user.UserID
	}

	// Build update request
	req := userorg.UpdateUserRequest{}
	if flagDisplayName != "" {
		req.DisplayName = &flagDisplayName
	}
	if flagStatus != "" {
		req.Status = &flagStatus
	}

	// Validate at least one field to update
	if req.DisplayName == nil && req.Status == nil && req.Metadata == nil {
		return errors.NewValidationError(
			"no fields to update",
			"Provide at least one field to update (--display-name, --status).",
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

	// Execute update
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	user, err := userOrgClient.UpdateUser(cmd.Context(), flagOrgID, resolvedUserID, req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to update user: %v", err),
			"Verify your API key is valid and the user exists.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "user_update",
		Command:     fmt.Sprintf("user update --org-id=%s --user-id=%s", flagOrgID, resolvedUserID),
		Parameters: map[string]interface{}{
			"orgId":  flagOrgID,
			"userId": resolvedUserID,
			"request": req,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(user)
	} else if cfg.OutputFormat == "csv" {
		mfaEnrolled := "false"
		if user.MFAEnrolled {
			mfaEnrolled = "true"
		}
		headers := []string{"userId", "email", "displayName", "status", "mfaEnrolled", "updatedAt"}
		rows := [][]string{{
			user.UserID,
			user.Email,
			user.DisplayName,
			user.Status,
			mfaEnrolled,
			user.UpdatedAt,
		}}
		return output.PrintTable(headers, rows)
	} else {
		if !cfg.Quiet {
			fmt.Printf("User updated successfully:\n")
			fmt.Printf("  User ID: %s\n", user.UserID)
			fmt.Printf("  Email: %s\n", user.Email)
			fmt.Printf("  Status: %s\n", user.Status)
		}
		if cfg.OutputFormat == "table" {
			mfaEnrolled := "No"
			if user.MFAEnrolled {
				mfaEnrolled = "Yes"
			}
			headers := []string{"User ID", "Email", "Display Name", "Status", "MFA Enrolled", "Updated At"}
			rows := [][]string{{
				user.UserID,
				user.Email,
				user.DisplayName,
				user.Status,
				mfaEnrolled,
				user.UpdatedAt,
			}}
			return output.PrintTable(headers, rows)
		}
	}
	return nil
}

func userDeleteCommand() *cobra.Command {
	var flagOrgID string
	var flagUserID string
	var flagEmail string
	var flagConfirm bool
	var flagForce bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete user",
		Long:  "Delete a user with confirmation and force flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserDelete(cmd, args, flagOrgID, flagUserID, flagEmail, flagConfirm, flagForce,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagUserID, "user-id", "", "User ID (required if --email not provided)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email (required if --user-id not provided)")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Confirm deletion (required unless --force)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Force deletion without confirmation prompt")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")

	return cmd
}

func runUserDelete(cmd *cobra.Command, args []string, flagOrgID, flagUserID, flagEmail string, flagConfirm, flagForce bool,
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

	// Resolve user ID
	resolvedUserID := flagUserID
	var userEmail string
	if resolvedUserID == "" {
		if flagEmail == "" {
			return errors.NewValidationError(
				"either --user-id or --email is required",
				"Provide user ID with --user-id or email with --email flag",
			)
		}

		// Get user by email
		userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
		user, err := userOrgClient.GetUserByEmail(cmd.Context(), flagOrgID, flagEmail)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to find user by email: %v", err),
				"Verify the email exists in this organization.",
			)
		}
		resolvedUserID = user.UserID
		userEmail = user.Email
	}

	// Confirmation check (non-interactive mode: require --confirm or --force)
	if !flagForce && !flagConfirm {
		return errors.NewValidationError(
			"confirmation required for destructive operation",
			"Use --confirm to confirm deletion or --force to skip confirmation (non-interactive mode).",
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

	// Get user details for confirmation display
	userOrgClient := userorg.NewClient(cfg.UserOrgEndpoint, cfg.APIKey)
	var userName string
	user, err := userOrgClient.GetUser(cmd.Context(), flagOrgID, resolvedUserID)
	if err == nil {
		userName = user.Email
	}

	// Show confirmation warning (unless forced or quiet)
	if !flagForce && !cfg.Quiet {
		fmt.Printf("⚠️  WARNING: This will delete user: %s\n", resolvedUserID)
		if userName != "" {
			fmt.Printf("   Email: %s\n", userName)
		}
		fmt.Println("   This action cannot be undone.")
	}

	// Execute delete
	if err := userOrgClient.DeleteUser(cmd.Context(), flagOrgID, resolvedUserID); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to delete user: %v", err),
			"Verify your API key is valid and the user exists.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:        "user_delete",
		Command:     fmt.Sprintf("user delete --org-id=%s --user-id=%s --confirm", flagOrgID, resolvedUserID),
		Parameters: map[string]interface{}{
			"orgId":  flagOrgID,
			"userId": resolvedUserID,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(map[string]interface{}{
			"success": true,
			"userId":  resolvedUserID,
			"message": "User deleted successfully",
		})
	} else {
		if !cfg.Quiet {
			fmt.Printf("User deleted successfully: %s\n", resolvedUserID)
		}
	}
	return nil
}

