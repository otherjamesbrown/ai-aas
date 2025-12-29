// Package commands provides user management commands.
//
// Purpose:
//
//	User lifecycle commands: list, create, update, delete with idempotent operations
//	and batch processing.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-002 (Day-2 Management)
package admin

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/errors"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// UserCommand creates the user command group.
func UserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
		Long: `Manage users within organizations.

Users are members of organizations who can access models and platform features
based on their assigned roles and model access permissions.

User Creation Methods:
  1. INVITE (default): Send an email invitation that user must accept
     ai-aas-cli user create --org-id acme --email user@example.com

  2. DIRECT: Create user with temporary password (for manual distribution)
     ai-aas-cli user create --org-id acme --email user@example.com --direct

Examples:
  # List users in an organization
  ai-aas-cli user list --org-id acme

  # Invite a new user (sends email)
  ai-aas-cli user create --org-id acme --email user@example.com

  # Create user directly with temporary password
  ai-aas-cli user create --org-id acme --email user@example.com --direct

  # Update user status
  ai-aas-cli user update --org-id acme --email user@example.com --status suspended

  # Delete a user
  ai-aas-cli user delete --org-id acme --email user@example.com --confirm

  # Manage model access for a user
  ai-aas-cli user model-access get --org-id acme --email user@example.com

Workflow:
  1. Create org        ai-aas-cli org create --name <name> --slug <slug>
  2. Add users         ai-aas-cli user create --org-id <org> --email <email>
  3. Create API key    ai-aas-cli apikey create --org-id <org> --user-id <user>

See Also:
  ai-aas-cli org list             List organizations
  ai-aas-cli apikey list          List API keys for user
  ai-aas-cli user model-access    Manage user model access`,
	}

	cmd.AddCommand(userListCommand())
	cmd.AddCommand(userCreateCommand())
	cmd.AddCommand(userUpdateCommand())
	cmd.AddCommand(userDeleteCommand())
	cmd.AddCommand(ModelAccessCommand())

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
		Long: `List all users in an organization.

Output includes user ID, email, display name, status, and MFA enrollment.

Examples:
  # List users in table format (default)
  ai-aas-cli user list --org-id acme

  # List in JSON format
  ai-aas-cli user list --org-id acme --format json

  # List in CSV format for export
  ai-aas-cli user list --org-id acme --format csv > users.csv

See Also:
  ai-aas-cli user create     Add a new user
  ai-aas-cli apikey list     List API keys for organization`,
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

	// Use default org if not specified
	orgID := flagOrgID
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, or set a default with 'ai-aas-cli org use <org-id>'",
		)
	}

	// Create HTTP client with TLS config (reused for all operations)
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
	}

	// Health check
	checker := createHealthCheckerWithHTTPClient(httpClient)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	// Create client and list users
	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)
	users, err := userOrgClient.ListUsers(cmd.Context(), orgID)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to list users: %v", err),
			"Verify your API key is valid and you have permission to list users in this organization.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:     "user_list",
		Command:  fmt.Sprintf("user list --org-id=%s", orgID),
		Outcome:  "success",
		Duration: time.Since(startTime),
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
	var flagDisplayName string
	var flagRoles []string
	var flagExpiresInHours int
	var flagDirect bool
	var flagForcePwdChange bool
	var flagUpsert bool
	var flagFormat string
	var flagVerbose bool
	var flagQuiet bool
	var flagUserOrgEndpoint string
	var flagAPIKey string
	var flagProfile string
	var flagModelAccess string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create user",
		Long: `Create a new user in an organization.

Two creation methods are available:

1. INVITE (default): Send an email invitation that user must accept.
   The invite expires after 72 hours by default (customizable with --expires-in-hours).

2. DIRECT (--direct): Create user immediately with a temporary password.
   The password is shown once and must be distributed to the user manually.
   By default, no password change is required on first login.
   Use --force-pwd-change to require the user to change their password.

Model Access Control:
  Use --model-access to set the user's model access mode:
    all:        User has access to all models in the organization (auto_grant mode)
    restricted: User only has access to explicitly granted models (restricted mode)
  Default: restricted for non-admin users, all for users with admin role

Examples:
  # Invite a user (sends email)
  ai-aas-cli user create --org-id acme --email user@example.com

  # Create user directly with temporary password (no change required)
  ai-aas-cli user create --org-id acme --email user@example.com --direct

  # Create user and require password change on first login
  ai-aas-cli user create --org-id acme --email user@example.com --direct \
    --force-pwd-change

  # Create with display name and roles
  ai-aas-cli user create --org-id acme --email admin@example.com \
    --display-name "Admin User" --roles admin,developer --direct

  # Invite with custom expiration
  ai-aas-cli user create --org-id acme --email user@example.com \
    --expires-in-hours 168

  # Idempotent create (won't fail if user exists)
  ai-aas-cli user create --org-id acme --email user@example.com --upsert

  # Create user and save to profile (uses org_id from profile)
  ai-aas-cli user create --email user@example.com --direct --profile acme-admin

  # Create user with model access mode
  ai-aas-cli user create --org-id acme --email user@example.com --direct \
    --model-access all

  # Create restricted user (explicit)
  ai-aas-cli user create --org-id acme --email user@example.com --direct \
    --model-access restricted

Next Steps:
  After creating a user with --direct:
  1. Securely share the password with the user
  2. User logs in (password change required only if --force-pwd-change was used)
  3. Create API keys:  ai-aas-cli apikey create --org-id <org> --user-id <user>

  After inviting a user:
  1. User accepts invite via email link
  2. Optionally assign more roles
  3. Create API keys for the user

See Also:
  ai-aas-cli user list       List organization users
  ai-aas-cli apikey create   Create API key for user`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUserCreate(cmd, args, flagOrgID, flagEmail, flagDisplayName, flagRoles, flagExpiresInHours, flagDirect, flagForcePwdChange, flagUpsert,
				flagFormat, flagVerbose, flagQuiet, flagUserOrgEndpoint, flagAPIKey, flagProfile, flagModelAccess)
		},
	}

	cmd.Flags().StringVar(&flagOrgID, "org-id", "", "Organization ID or slug (required)")
	cmd.Flags().StringVar(&flagEmail, "email", "", "User email (required)")
	cmd.Flags().StringVar(&flagDisplayName, "display-name", "", "Display name for the user")
	cmd.Flags().StringSliceVar(&flagRoles, "roles", []string{}, "User roles")
	cmd.Flags().IntVar(&flagExpiresInHours, "expires-in-hours", 72, "Invite expiration in hours (default: 72, invite mode only)")
	cmd.Flags().BoolVar(&flagDirect, "direct", false, "Create user directly with temporary password (bypasses invite flow)")
	cmd.Flags().BoolVar(&flagForcePwdChange, "force-pwd-change", false, "Require password change on first login (--direct mode only)")
	cmd.Flags().BoolVar(&flagUpsert, "upsert", false, "Update user if already exists (idempotent)")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json, csv")
	cmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-error output")
	cmd.Flags().StringVar(&flagUserOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for authentication (overrides config)")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Use org_id from profile and save user_id to it")
	cmd.Flags().StringVarP(&flagModelAccess, "model-access", "m", "", "Model access mode: 'all' or 'restricted' (default: auto for admin, restricted otherwise)")

	return cmd
}

func runUserCreate(cmd *cobra.Command, args []string, flagOrgID, flagEmail, flagDisplayName string, flagRoles []string, flagExpiresInHours int, flagDirect, flagForcePwdChange, flagUpsert bool,
	flagFormat string, flagVerbose, flagQuiet bool, flagUserOrgEndpoint, flagAPIKey string, flagProfile string, flagModelAccess string) error {
	startTime := time.Now()

	// Load configuration with profile support
	cfg, _, err := config.GetEffectiveConfig(flagProfile)
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

	// Use profile org_id if --profile is set
	orgID := flagOrgID
	if orgID == "" && flagProfile != "" {
		if profile, err := config.GetProfile(flagProfile); err == nil && profile.OrgID != "" {
			orgID = profile.OrgID
		}
	}
	// Fall back to default org
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, or set a default with 'ai-aas-cli org use <org-id>'",
		)
	}
	if flagEmail == "" {
		return errors.NewValidationError(
			"--email is required",
			"Provide user email with --email flag",
		)
	}

	// Validate model-access flag if provided
	if flagModelAccess != "" && flagModelAccess != "all" && flagModelAccess != "restricted" {
		return errors.NewValidationError(
			"--model-access must be 'all' or 'restricted'",
			"Use --model-access all for auto_grant mode or --model-access restricted for restricted mode",
		)
	}

	// Create HTTP client with TLS config (reused for all operations)
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
	}

	// Health check
	checker := createHealthCheckerWithHTTPClient(httpClient)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

	// Validate --model-access flag if provided
	if flagModelAccess != "" && flagModelAccess != "all" && flagModelAccess != "restricted" {
		return errors.NewValidationError(
			"--model-access must be 'all' or 'restricted'",
			"Provide a valid model access mode with --model-access flag",
		)
	}

	// Handle DIRECT user creation (bypasses invite flow)
	if flagDirect {
		return runDirectUserCreate(cmd, cfg, userOrgClient, orgID, flagEmail, flagDisplayName, flagRoles, flagForcePwdChange, flagUpsert, flagProfile, flagModelAccess, startTime)
	}

	// Handle INVITE user creation (default)
	return runInviteUserCreate(cmd, cfg, userOrgClient, orgID, flagEmail, flagRoles, flagExpiresInHours, flagUpsert, flagProfile, flagModelAccess, startTime)
}

// runDirectUserCreate handles direct user creation with temporary password.
func runDirectUserCreate(cmd *cobra.Command, cfg *config.Config, userOrgClient *userorg.Client, orgID, email, displayName string, roles []string, forcePwdChange, upsert bool, profileName string, modelAccess string, startTime time.Time) error {
	// Check if user exists (for upsert)
	if upsert {
		existingUser, err := userOrgClient.GetUserByEmail(cmd.Context(), orgID, email)
		if err == nil {
			// User exists, return existing user
			if !cfg.Quiet {
				fmt.Printf("User already exists (upsert mode):\n")
				fmt.Printf("  User ID: %s\n", existingUser.UserID)
				fmt.Printf("  Email: %s\n", existingUser.Email)
				fmt.Printf("  Status: %s\n", existingUser.Status)
				fmt.Println()
				fmt.Println("Note: No new password generated for existing user.")
			}
			return nil
		}
	}

	// Create user directly
	req := userorg.CreateUserRequest{
		Email:          email,
		DisplayName:    displayName,
		Roles:          roles,
		ForcePwdChange: &forcePwdChange,
	}

	createdUser, err := userOrgClient.CreateUser(cmd.Context(), orgID, req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to create user: %v", err),
			"Verify your API key is valid and you have permission to create users.",
		)
	}

	// Set model access mode if specified
	if modelAccess != "" {
		accessMode := determineAccessMode(modelAccess, roles)
		_, err := userOrgClient.SetUserAccessMode(cmd.Context(), orgID, createdUser.UserID, accessMode)
		if err != nil {
			// Log warning but don't fail user creation
			if !cfg.Quiet {
				fmt.Printf("Warning: failed to set model access mode: %v\n", err)
			}
		} else if !cfg.Quiet && cfg.Verbose {
			fmt.Printf("Model access mode set to: %s\n", accessMode)
		}
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:    "user_create_direct",
		Command: fmt.Sprintf("user create --org-id=%s --email=%s --direct --force-pwd-change=%v", orgID, email, forcePwdChange),
		Parameters: map[string]interface{}{
			"orgId":          orgID,
			"email":          email,
			"userId":         createdUser.UserID,
			"method":         "direct",
			"forcePwdChange": forcePwdChange,
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Save to profile if --profile flag is set
	if profileName != "" {
		if err := config.UpdateProfile(profileName, func(p *config.Profile) {
			p.UserID = createdUser.UserID
			p.Email = createdUser.Email
		}); err != nil {
			if !cfg.Quiet {
				fmt.Printf("Warning: failed to update profile '%s': %v\n", profileName, err)
			}
		} else if !cfg.Quiet {
			fmt.Printf("\nSaved to profile '%s'\n", profileName)
			if profile, err := config.GetProfile(profileName); err == nil {
				fmt.Printf("  org_id:   %s\n", valueOrNotSet(profile.OrgID))
				fmt.Printf("  user_id:  %s\n", profile.UserID)
				fmt.Printf("  email:    %s\n", profile.Email)
				fmt.Printf("  api_key:  %s\n", valueOrNotSet(profile.APIKey))
			}
		}
	}

	// Format output
	if cfg.OutputFormat == "json" {
		return output.PrintJSON(createdUser)
	} else if cfg.OutputFormat == "csv" {
		headers := []string{"userId", "email", "displayName", "status", "temporaryPassword", "createdAt"}
		rows := [][]string{{
			createdUser.UserID,
			createdUser.Email,
			createdUser.DisplayName,
			createdUser.Status,
			createdUser.TemporaryPassword,
			createdUser.CreatedAt,
		}}
		return output.PrintTable(headers, rows)
	}

	// Default table output with prominent password display
	if !cfg.Quiet {
		fmt.Println()
		fmt.Println("User created successfully!")
		fmt.Println()
		fmt.Printf("  User ID:      %s\n", createdUser.UserID)
		fmt.Printf("  Email:        %s\n", createdUser.Email)
		if createdUser.DisplayName != "" {
			fmt.Printf("  Display Name: %s\n", createdUser.DisplayName)
		}
		fmt.Printf("  Status:       %s\n", createdUser.Status)
		fmt.Printf("  Created At:   %s\n", createdUser.CreatedAt)
		fmt.Println()
		fmt.Println("  ╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("  ║  TEMPORARY PASSWORD (shown once - save it now!)              ║")
		fmt.Println("  ╠══════════════════════════════════════════════════════════════╣")
		fmt.Printf("  ║  %s%-52s║\n", createdUser.TemporaryPassword, "")
		fmt.Println("  ╚══════════════════════════════════════════════════════════════╝")
		fmt.Println()
		if forcePwdChange {
			fmt.Println("  The user must change this password on first login (--force-pwd-change).")
		} else {
			fmt.Println("  No password change required on first login.")
		}
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Printf("  1. Share the password securely with the user\n")
		fmt.Printf("  2. Create API key: ai-aas-cli apikey create --org-id %s --user-id %s\n", orgID, createdUser.UserID)
		fmt.Println()
	}
	return nil
}

// runInviteUserCreate handles invite-based user creation (sends email).
func runInviteUserCreate(cmd *cobra.Command, cfg *config.Config, userOrgClient *userorg.Client, orgID, email string, roles []string, expiresInHours int, upsert bool, profileName string, modelAccess string, startTime time.Time) error {
	// Check if user exists (for upsert)
	var user *userorg.UserResponse
	if upsert {
		existingUser, err := userOrgClient.GetUserByEmail(cmd.Context(), orgID, email)
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
			Email:          email,
			Roles:          roles,
			ExpiresInHours: expiresInHours,
		}

		invitedUser, err := userOrgClient.InviteUser(cmd.Context(), orgID, req)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to invite user: %v", err),
				"Verify your API key is valid and you have permission to invite users.",
			)
		}
		user = invitedUser
	}

	// Set model access mode if specified
	if modelAccess != "" {
		accessMode := determineAccessMode(modelAccess, roles)
		_, err := userOrgClient.SetUserAccessMode(cmd.Context(), orgID, user.UserID, accessMode)
		if err != nil {
			// Log warning but don't fail user creation
			if !cfg.Quiet {
				fmt.Printf("Warning: failed to set model access mode: %v\n", err)
			}
		} else if !cfg.Quiet && cfg.Verbose {
			fmt.Printf("Model access mode set to: %s\n", accessMode)
		}
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:    "user_create",
		Command: fmt.Sprintf("user create --org-id=%s --email=%s", orgID, email),
		Parameters: map[string]interface{}{
			"orgId":  orgID,
			"email":  email,
			"userId": user.UserID,
			"upsert": upsert,
			"method": "invite",
		},
		Outcome:  "success",
		Duration: time.Since(startTime),
	})

	// Save to profile if --profile flag is set
	if profileName != "" {
		if err := config.UpdateProfile(profileName, func(p *config.Profile) {
			p.UserID = user.UserID
			p.Email = user.Email
		}); err != nil {
			if !cfg.Quiet {
				fmt.Printf("Warning: failed to update profile '%s': %v\n", profileName, err)
			}
		} else if !cfg.Quiet {
			fmt.Printf("\nSaved to profile '%s'\n", profileName)
			if profile, err := config.GetProfile(profileName); err == nil {
				fmt.Printf("  org_id:   %s\n", valueOrNotSet(profile.OrgID))
				fmt.Printf("  user_id:  %s\n", profile.UserID)
				fmt.Printf("  email:    %s\n", profile.Email)
				fmt.Printf("  api_key:  %s\n", valueOrNotSet(profile.APIKey))
			}
		}
	}

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
			if upsert && user != nil {
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
		Long: `Update a user's settings.

You can identify the user by --user-id or --email. Update display name or status.

Examples:
  # Update display name
  ai-aas-cli user update --org-id acme --email user@example.com \
    --display-name "Jane Doe"

  # Suspend a user
  ai-aas-cli user update --org-id acme --email user@example.com \
    --status suspended

  # Reactivate a user
  ai-aas-cli user update --org-id acme --user-id u_123 --status active

See Also:
  ai-aas-cli user list      List organization users
  ai-aas-cli user delete    Remove a user`,
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

	// Use default org if not specified
	orgID := flagOrgID
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, or set a default with 'ai-aas-cli org use <org-id>'",
		)
	}

	// Create HTTP client with TLS config (reused for all operations)
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
	}

	// Health check
	checker := createHealthCheckerWithHTTPClient(httpClient)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

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
		user, err := userOrgClient.GetUserByEmail(cmd.Context(), orgID, flagEmail)
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

	// Execute update
	user, err := userOrgClient.UpdateUser(cmd.Context(), orgID, resolvedUserID, req)
	if err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to update user: %v", err),
			"Verify your API key is valid and the user exists.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:    "user_update",
		Command: fmt.Sprintf("user update --org-id=%s --user-id=%s", orgID, resolvedUserID),
		Parameters: map[string]interface{}{
			"orgId":   orgID,
			"userId":  resolvedUserID,
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

// determineAccessMode converts the --model-access flag value to the API access mode.
// Returns "auto_grant" for "all" or "restricted" for "restricted".
// If the flag value is empty, returns default based on roles:
// - "auto_grant" if roles contain "admin"
// - "restricted" otherwise
func determineAccessMode(flagValue string, roles []string) string {
	// If flag is explicitly set
	if flagValue != "" {
		if flagValue == "all" {
			return "auto_grant"
		}
		return "restricted"
	}

	// Default: check if user has admin role
	for _, role := range roles {
		if role == "admin" {
			return "auto_grant"
		}
	}

	// Default for non-admin users
	return "restricted"
}

// hasAdminRole checks if the roles slice contains "admin"
func hasAdminRole(roles []string) bool {
	for _, role := range roles {
		if role == "admin" {
			return true
		}
	}
	return false
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
		Long: `Delete a user from an organization.

This is a destructive operation that cannot be undone. The user's access will
be immediately revoked.

Examples:
  # Delete by email with confirmation
  ai-aas-cli user delete --org-id acme --email user@example.com --confirm

  # Delete by user ID
  ai-aas-cli user delete --org-id acme --user-id u_123 --confirm

  # Force delete (for scripts)
  ai-aas-cli user delete --org-id acme --email user@example.com --force

Warning:
  Deleting a user will also revoke:
  - All API keys owned by the user
  - All active sessions
  - Access to all organization resources

See Also:
  ai-aas-cli user update    Suspend instead of delete
  ai-aas-cli user list      List organization users`,
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

	// Use default org if not specified
	orgID := flagOrgID
	if orgID == "" {
		orgID = cfg.DefaultOrgID
	}
	if orgID == "" {
		return errors.NewValidationError(
			"--org-id is required",
			"Provide organization ID or slug with --org-id flag, or set a default with 'ai-aas-cli org use <org-id>'",
		)
	}

	// Create HTTP client with TLS config (reused for all operations)
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		return errors.NewOperationError(fmt.Sprintf("create http client: %v", err), "Check TLS configuration")
	}

	// Health check
	checker := createHealthCheckerWithHTTPClient(httpClient)
	requiredServices := map[string]string{
		"user-org-service": cfg.UserOrgEndpoint,
	}
	if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
		return errors.NewServiceUnavailableError("user-org-service", cfg.UserOrgEndpoint)
	}

	userOrgClient := createUserOrgClientWithHTTPClient(cfg, httpClient)

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
		user, err := userOrgClient.GetUserByEmail(cmd.Context(), orgID, flagEmail)
		if err != nil {
			return errors.NewOperationError(
				fmt.Sprintf("failed to find user by email: %v", err),
				"Verify the email exists in this organization.",
			)
		}
		resolvedUserID = user.UserID
	}

	// Confirmation check (non-interactive mode: require --confirm or --force)
	if !flagForce && !flagConfirm {
		return errors.NewValidationError(
			"confirmation required for destructive operation",
			"Use --confirm to confirm deletion or --force to skip confirmation (non-interactive mode).",
		)
	}

	// Get user details for confirmation display
	var userName string
	user, err := userOrgClient.GetUser(cmd.Context(), orgID, resolvedUserID)
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
	if err := userOrgClient.DeleteUser(cmd.Context(), orgID, resolvedUserID); err != nil {
		return errors.NewOperationError(
			fmt.Sprintf("failed to delete user: %v", err),
			"Verify your API key is valid and the user exists.",
		)
	}

	// Audit logging
	auditLogger := audit.NewLogger(nil)
	_ = auditLogger.LogOperation(audit.Operation{
		Type:    "user_delete",
		Command: fmt.Sprintf("user delete --org-id=%s --user-id=%s --confirm", orgID, resolvedUserID),
		Parameters: map[string]interface{}{
			"orgId":  orgID,
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
