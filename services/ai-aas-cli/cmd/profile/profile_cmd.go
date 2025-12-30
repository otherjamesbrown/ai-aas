// Package profile provides CLI commands for managing profiles.
package profile

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/client/userorg"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/health"
	"github.com/spf13/cobra"
)

// NewProfileCmd creates the profile command group.
func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage CLI profiles",
		Long: `Manage CLI profiles for different org/user/environment combinations.

Profiles store identity information (org, user, API key) along with
environment settings. Use profiles to quickly switch between different
accounts or environments.

Workflow:
  1. Create an empty profile:     ai-aas-cli profile create my-profile --environment development
  2. Create org (saves to profile): ai-aas-cli org create my-org --profile my-profile
  3. Create user (saves to profile): ai-aas-cli user create --email me@example.com --profile my-profile
  4. Create key (saves to profile):  ai-aas-cli apikey create --profile my-profile
  5. Switch to profile:            ai-aas-cli profile use my-profile`,
	}

	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newCreateFullCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newUseCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newCurrentCmd())

	return cmd
}

func newCreateCmd() *cobra.Command {
	var (
		environment       string
		adminAPIEndpoint  string
		inferenceEndpoint string
		userOrgEndpoint   string
		kubeconfig        string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
		Long: `Create a new profile with environment and endpoint configuration.

Profiles follow the AWS CLI pattern - each profile is self-contained with
all the endpoints and credentials needed to connect to an environment.

Examples:
  # Create a development profile
  ai-aas-cli profile create dev \
    --environment development \
    --admin-api-endpoint https://admin-api.dev.example.com \
    --inference-endpoint https://api.dev.example.com

  # Create a staging profile for Singapore
  ai-aas-cli profile create staging-sg \
    --environment staging \
    --admin-api-endpoint https://admin-api.staging.example.com \
    --inference-endpoint https://api.staging.example.com \
    --kubeconfig ~/.kube/staging-sg.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := config.CreateProfile(name, environment); err != nil {
				return err
			}

			// Update profile with endpoint overrides
			if adminAPIEndpoint != "" || inferenceEndpoint != "" || userOrgEndpoint != "" || kubeconfig != "" {
				if err := config.UpdateProfile(name, func(p *config.Profile) {
					if adminAPIEndpoint != "" {
						p.AdminAPIEndpoint = adminAPIEndpoint
					}
					if inferenceEndpoint != "" {
						p.InferenceEndpoint = inferenceEndpoint
					}
					if userOrgEndpoint != "" {
						p.UserOrgEndpoint = userOrgEndpoint
					}
					if kubeconfig != "" {
						p.Kubeconfig = kubeconfig
					}
				}); err != nil {
					return err
				}
			}

			fmt.Printf("Profile '%s' created\n", name)
			fmt.Printf("  environment:  %s\n", environment)
			if adminAPIEndpoint != "" {
				fmt.Printf("  admin_api:    %s\n", adminAPIEndpoint)
			}
			if inferenceEndpoint != "" {
				fmt.Printf("  inference:    %s\n", inferenceEndpoint)
			}
			if userOrgEndpoint != "" {
				fmt.Printf("  user_org:     %s\n", userOrgEndpoint)
			}
			if kubeconfig != "" {
				fmt.Printf("  kubeconfig:   %s\n", kubeconfig)
			}
			fmt.Printf("  org_id:       (not set)\n")
			fmt.Printf("  user_id:      (not set)\n")
			fmt.Printf("  api_key:      (not set)\n")
			fmt.Println()
			fmt.Printf("Next: Use --profile %s with org/user/apikey commands to populate credentials\n", name)

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "development", "Environment label (development, staging, production)")
	cmd.Flags().StringVar(&adminAPIEndpoint, "admin-api-endpoint", "", "Admin API endpoint URL")
	cmd.Flags().StringVar(&inferenceEndpoint, "inference-endpoint", "", "Inference API endpoint URL")
	cmd.Flags().StringVar(&userOrgEndpoint, "user-org-endpoint", "", "User-org service endpoint URL")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file for this environment")

	return cmd
}

func newCreateFullCmd() *cobra.Command {
	var (
		environment     string
		orgName         string
		orgSlug         string
		email           string
		displayName     string
		userOrgEndpoint string
		apiKey          string
		insecureSkipTLS bool
		setActive       bool
	)

	cmd := &cobra.Command{
		Use:   "create-full <profile-name>",
		Short: "Create a complete profile with org, user, and API key",
		Long: `Create a complete profile in one command.

This command creates:
  1. A new organization
  2. A new user in that organization
  3. An API key for that user
  4. A profile storing all credentials

Example:
  ai-aas-cli profile create-full acme-admin \
    --environment development \
    --org-name "Acme Corp" \
    --org-slug acme-corp \
    --user-email admin@acme.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			// Load config for defaults
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Apply overrides
			if userOrgEndpoint != "" {
				cfg.UserOrgEndpoint = userOrgEndpoint
			}
			if apiKey != "" {
				cfg.APIKey = apiKey
			}

			// Validate required fields
			if cfg.UserOrgEndpoint == "" {
				return fmt.Errorf("user-org-service endpoint is required (set via --user-org-endpoint or config)")
			}
			if orgName == "" {
				return fmt.Errorf("--org-name is required")
			}
			if orgSlug == "" {
				return fmt.Errorf("--org-slug is required")
			}
			if email == "" {
				return fmt.Errorf("--user-email is required")
			}

			// Create HTTP client
			httpClient := &http.Client{
				Timeout: 30 * time.Second,
			}
			if insecureSkipTLS {
				httpClient.Transport = &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
			}

			// Health check
			checker := health.NewCheckerWithClient(5*time.Second, httpClient)
			requiredServices := map[string]string{
				"user-org-service": cfg.UserOrgEndpoint,
			}
			if _, err := checker.CheckRequired(cmd.Context(), requiredServices); err != nil {
				return fmt.Errorf("user-org-service unavailable at %s", cfg.UserOrgEndpoint)
			}

			// Create userorg client
			client := userorg.NewClientWithHTTPClient(cfg.UserOrgEndpoint, cfg.APIKey, httpClient)

			ctx := cmd.Context()

			// Step 1: Create organization
			fmt.Printf("Creating organization '%s'...\n", orgName)
			org, err := client.CreateOrg(ctx, userorg.CreateOrgRequest{
				Name: orgName,
				Slug: orgSlug,
			})
			if err != nil {
				return fmt.Errorf("failed to create organization: %w", err)
			}
			fmt.Printf("  ✓ Organization created: %s (slug: %s)\n", org.OrgID, org.Slug)

			// Step 2: Create user
			fmt.Printf("Creating user '%s'...\n", email)
			user, err := client.CreateUser(ctx, org.Slug, userorg.CreateUserRequest{
				Email:       email,
				DisplayName: displayName,
			})
			if err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}
			fmt.Printf("  ✓ User created: %s\n", user.UserID)
			fmt.Printf("  ⚠ Temporary password: %s\n", user.TemporaryPassword)

			// Step 3: Create API key
			fmt.Printf("Creating API key...\n")
			apiKeyResp, err := client.IssueUserAPIKey(ctx, org.Slug, user.UserID, userorg.IssueAPIKeyRequest{})
			if err != nil {
				return fmt.Errorf("failed to create API key: %w", err)
			}
			fmt.Printf("  ✓ API key created: %s\n", apiKeyResp.KeyID)

			// Step 4: Create and populate profile
			if err := config.CreateProfile(profileName, environment); err != nil {
				return fmt.Errorf("failed to create profile: %w", err)
			}

			if err := config.UpdateProfile(profileName, func(p *config.Profile) {
				p.OrgID = org.Slug
				p.UserID = user.UserID
				p.Email = email
				p.APIKey = apiKeyResp.Token
				p.UserOrgEndpoint = cfg.UserOrgEndpoint
			}); err != nil {
				return fmt.Errorf("failed to update profile: %w", err)
			}

			// Optionally set as active profile
			if setActive {
				if err := config.SetActiveProfile(profileName); err != nil {
					return fmt.Errorf("failed to set active profile: %w", err)
				}
			}

			// Print summary
			fmt.Println()
			fmt.Printf("Profile '%s' created successfully!\n", profileName)
			fmt.Println()
			fmt.Printf("  environment: %s\n", environment)
			fmt.Printf("  org_id:      %s\n", org.Slug)
			fmt.Printf("  user_id:     %s\n", user.UserID)
			fmt.Printf("  email:       %s\n", email)
			fmt.Printf("  api_key:     %s\n", config.MaskSecret(apiKeyResp.Token))
			fmt.Println()
			fmt.Println("╔══════════════════════════════════════════════════════════════╗")
			fmt.Println("║  SAVE THESE CREDENTIALS - shown only once!                   ║")
			fmt.Println("╠══════════════════════════════════════════════════════════════╣")
			fmt.Printf("║  API Token:  %s\n", apiKeyResp.Token)
			fmt.Printf("║  Password:   %s\n", user.TemporaryPassword)
			fmt.Println("╚══════════════════════════════════════════════════════════════╝")
			fmt.Println()
			if setActive {
				fmt.Printf("Profile '%s' is now active.\n", profileName)
			} else {
				fmt.Printf("To use this profile: ai-aas-cli profile use %s\n", profileName)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&environment, "environment", "e", "development", "Environment (development, staging, production)")
	cmd.Flags().StringVar(&orgName, "org-name", "", "Organization name (required)")
	cmd.Flags().StringVar(&orgSlug, "org-slug", "", "Organization slug (required)")
	cmd.Flags().StringVar(&email, "user-email", "", "User email (required)")
	cmd.Flags().StringVar(&displayName, "user-display-name", "", "User display name")
	cmd.Flags().StringVar(&userOrgEndpoint, "user-org-endpoint", "", "User-org-service endpoint (overrides config)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication (overrides config)")
	cmd.Flags().BoolVar(&insecureSkipTLS, "insecure-skip-tls-verify", false, "Skip TLS certificate verification")
	cmd.Flags().BoolVar(&setActive, "use", false, "Set as active profile after creation")

	cmd.MarkFlagRequired("org-name")
	cmd.MarkFlagRequired("org-slug")
	cmd.MarkFlagRequired("user-email")

	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			pc, err := config.LoadProfiles()
			if err != nil {
				return err
			}

			if len(pc.Profiles) == 0 {
				fmt.Println("No profiles configured.")
				fmt.Println("Create one with: ai-aas-cli profile create <name> --environment <env>")
				return nil
			}

			// Sort profile names
			names := make([]string, 0, len(pc.Profiles))
			for name := range pc.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tENVIRONMENT\tORG\tUSER\tSTATUS")

			for _, name := range names {
				p := pc.Profiles[name]
				active := ""
				if name == pc.ActiveProfile {
					active = " *"
				}

				orgDisplay := p.OrgID
				if orgDisplay == "" {
					orgDisplay = "(not set)"
				}

				userDisplay := p.Email
				if userDisplay == "" && p.UserID != "" {
					userDisplay = p.UserID[:8] + "..." // Truncate UUID
				}
				if userDisplay == "" {
					userDisplay = "(not set)"
				}

				fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\n",
					name, active,
					p.Environment,
					orgDisplay,
					userDisplay,
					p.Status(),
				)
			}

			w.Flush()
			return nil
		},
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show profile details",
		Long:  "Show details of a profile. If no name given, shows the active profile.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var profile *Profile
			var name string
			var err error

			if len(args) > 0 {
				name = args[0]
				profile, err = config.GetProfile(name)
			} else {
				var p *config.Profile
				p, name, err = config.GetActiveProfile()
				if p != nil {
					profile = (*Profile)(p)
				}
			}

			if err != nil {
				return err
			}

			if profile == nil {
				if name == "" {
					return fmt.Errorf("no active profile set. Use 'profile use <name>' or specify a profile name")
				}
				return fmt.Errorf("profile '%s' not found", name)
			}

			fmt.Printf("Profile: %s\n", name)
			fmt.Printf("  environment: %s\n", valueOrNotSet(profile.Environment))
			fmt.Printf("  org_id:      %s\n", valueOrNotSet(profile.OrgID))
			fmt.Printf("  user_id:     %s\n", valueOrNotSet(profile.UserID))
			fmt.Printf("  email:       %s\n", valueOrNotSet(profile.Email))
			fmt.Printf("  api_key:     %s\n", config.MaskSecret(profile.APIKey))
			fmt.Printf("  status:      %s\n", profile.Status())

			// Show endpoints
			fmt.Println()
			fmt.Println("Endpoints:")
			fmt.Printf("  admin_api:   %s\n", valueOrNotSet(profile.AdminAPIEndpoint))
			fmt.Printf("  inference:   %s\n", valueOrNotSet(profile.InferenceEndpoint))
			fmt.Printf("  user_org:    %s\n", valueOrNotSet(profile.UserOrgEndpoint))
			if profile.Kubeconfig != "" {
				fmt.Printf("  kubeconfig:  %s\n", profile.Kubeconfig)
			}

			return nil
		},
	}
}

// Profile is a type alias for template compatibility
type Profile = config.Profile

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := config.SetActiveProfile(name); err != nil {
				return err
			}

			profile, err := config.GetProfile(name)
			if err != nil {
				return err
			}

			fmt.Printf("Switched to profile '%s'\n", name)
			fmt.Printf("  environment: %s\n", valueOrNotSet(profile.Environment))
			fmt.Printf("  org_id:      %s\n", valueOrNotSet(profile.OrgID))

			if !profile.IsComplete() {
				fmt.Printf("\nNote: Profile is %s\n", profile.Status())
			}

			return nil
		},
	}
}

func newDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			profile, err := config.GetProfile(name)
			if err != nil {
				return err
			}

			if !force && profile.APIKey != "" {
				return fmt.Errorf("profile '%s' contains an API key. Use --force to delete", name)
			}

			if err := config.DeleteProfile(name); err != nil {
				return err
			}

			fmt.Printf("Profile '%s' deleted\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force delete even if profile contains credentials")

	return cmd
}

func newCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the current active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, name, err := config.GetActiveProfile()
			if err != nil {
				return err
			}

			if profile == nil {
				fmt.Println("No active profile set.")
				fmt.Println("Use 'ai-aas-cli profile use <name>' to set one.")
				return nil
			}

			fmt.Printf("Active profile: %s\n", name)
			fmt.Printf("  environment: %s\n", valueOrNotSet(profile.Environment))
			fmt.Printf("  org_id:      %s\n", valueOrNotSet(profile.OrgID))
			fmt.Printf("  user_id:     %s\n", valueOrNotSet(profile.UserID))
			fmt.Printf("  api_key:     %s\n", config.MaskSecret(profile.APIKey))

			return nil
		},
	}
}

func valueOrNotSet(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}
