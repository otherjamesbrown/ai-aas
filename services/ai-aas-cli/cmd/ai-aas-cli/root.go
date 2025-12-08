// Package cmd provides the CLI command implementations for ai-aas-cli.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/cmd/engine"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/cmd/model"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/cmd/profile"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/cmd/status"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/admin"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/credentials"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/logging"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	profileName  string
	verbose      bool
	quiet        bool
	outputFormat string
)

// ProfileName returns the current profile name (for use by subcommands).
func ProfileName() string {
	// Check environment variable first
	if envProfile := os.Getenv("AI_AAS_PROFILE"); envProfile != "" {
		return envProfile
	}
	return profileName
}

// NewRootCommand creates the root command for ai-aas-cli
func NewRootCommand(version, buildTime, gitCommit string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ai-aas-cli",
		Short: "AI-AAS Platform CLI for model and platform management",
		Long: `ai-aas-cli provides comprehensive management for the AI-AAS platform.

Quick Start:
  ai-aas-cli --init                       Initialize CLI configuration
  ai-aas-cli --init-status                Show current configuration status
  ai-aas-cli model registry list          List registered models
  ai-aas-cli model deploy create <model>  Deploy a model
  ai-aas-cli org list                     List organizations

Configuration:
  ai-aas-cli --init                           Full interactive setup wizard
  ai-aas-cli --init-status                    Show what's configured
  ai-aas-cli --init-hf-token <token>          Set HuggingFace token
  ai-aas-cli --init-api-key <key>             Set API key
  ai-aas-cli --init-domain <domain>           Set base domain (derives endpoints)
  ai-aas-cli --init-environment <env>         Set environment (dev/staging/prod)

Model Workflow:
  ai-aas-cli model registry add meta-llama/Llama-2-7b-hf   # Register
  ai-aas-cli model cache pull meta-llama/Llama-2-7b-hf     # Cache (server-side)
  ai-aas-cli model deploy create meta-llama/Llama-2-7b-hf  # Deploy

Getting Help:
  ai-aas-cli --help                       Show this help
  ai-aas-cli model --help                 Show model management commands
  ai-aas-cli model deploy --help          Show deployment subcommands
  ai-aas-cli model deploy create --help   Show create deployment options

Environment Variables:
  AI_AAS_ENVIRONMENT        Target environment (dev/staging/prod)
  AI_AAS_API_KEY            API key for authentication
  AI_AAS_API_ENDPOINT       Admin API endpoint URL (primary endpoint for all operations)
  AI_AAS_ADMIN_API_ENDPOINT Advanced override for Admin API endpoint (rarely needed)
  AI_AAS_PROFILE            Active profile name`,
		Version: fmt.Sprintf("%s (build: %s, commit: %s)", version, buildTime, gitCommit),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ai-aas-cli.yaml)")
	rootCmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "", "use named profile for org/user/credentials")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "table", "output format (table, json, csv)")

	// Init flags - handled via RunE since they're on the root command
	var (
		initFlag       bool
		initStatus     bool
		initDomain     string
		initAPIKey     string
		initEnv        string
		initHFToken    string
		initAdminAPI   string
		initUserOrgAPI string
	)
	rootCmd.Flags().BoolVar(&initFlag, "init", false, "run initialization wizard")
	rootCmd.Flags().BoolVar(&initStatus, "init-status", false, "show current configuration status")
	rootCmd.Flags().StringVar(&initDomain, "init-domain", "", "set base domain (e.g., dev.example.com)")
	rootCmd.Flags().StringVar(&initAPIKey, "init-api-key", "", "set API key")
	rootCmd.Flags().StringVar(&initEnv, "init-environment", "", "set environment (development/staging/production)")
	rootCmd.Flags().StringVar(&initHFToken, "init-hf-token", "", "set HuggingFace token")
	rootCmd.Flags().StringVar(&initAdminAPI, "init-admin-api", "", "set Admin API endpoint")
	rootCmd.Flags().StringVar(&initUserOrgAPI, "init-user-org-api", "", "set User-Org Service endpoint")
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// --init-status: Show configuration status
		if initStatus {
			return showInitStatus()
		}
		// --init: Run full wizard
		if initFlag {
			return runInit()
		}
		// Individual config updates
		if initDomain != "" || initAPIKey != "" || initEnv != "" || initHFToken != "" || initAdminAPI != "" || initUserOrgAPI != "" {
			return runPartialInit(initDomain, initAPIKey, initEnv, initHFToken, initAdminAPI, initUserOrgAPI)
		}
		// If no --init flag and no subcommand, show help
		return cmd.Help()
	}

	// Define command groups
	modelGroup := &cobra.Group{ID: "model", Title: "Model Management:"}
	accessGroup := &cobra.Group{ID: "access", Title: "Access Control:"}
	platformGroup := &cobra.Group{ID: "platform", Title: "Platform Operations:"}
	utilGroup := &cobra.Group{ID: "util", Title: "Utilities:"}

	rootCmd.AddGroup(modelGroup, accessGroup, platformGroup, utilGroup)

	// Model Management commands
	modelCmd := newModelCommand()
	modelCmd.GroupID = "model"
	rootCmd.AddCommand(modelCmd)

	deploymentCmd := admin.DeploymentCommand()
	deploymentCmd.GroupID = "model"
	rootCmd.AddCommand(deploymentCmd)

	inferenceCmd := admin.InferenceCommand()
	inferenceCmd.GroupID = "model"
	rootCmd.AddCommand(inferenceCmd)

	engineCmd := engine.NewEngineCommand()
	engineCmd.GroupID = "model"
	rootCmd.AddCommand(engineCmd)

	// Access Control commands
	orgCmd := admin.OrgCommand()
	orgCmd.GroupID = "access"
	rootCmd.AddCommand(orgCmd)

	userCmd := admin.UserCommand()
	userCmd.GroupID = "access"
	rootCmd.AddCommand(userCmd)

	apikeyCmd := admin.APIKeyCommand()
	apikeyCmd.GroupID = "access"
	rootCmd.AddCommand(apikeyCmd)

	// Platform Operations commands
	bootstrapCmd := admin.BootstrapCommand()
	bootstrapCmd.GroupID = "platform"
	rootCmd.AddCommand(bootstrapCmd)

	registryCmd := admin.RegistryCommand()
	registryCmd.GroupID = "platform"
	rootCmd.AddCommand(registryCmd)

	routingCmd := admin.RoutingCommand()
	routingCmd.GroupID = "platform"
	rootCmd.AddCommand(routingCmd)

	syncCmd := admin.SyncCommand()
	syncCmd.GroupID = "platform"
	rootCmd.AddCommand(syncCmd)

	credentialsCmd := newCredentialsCommand()
	credentialsCmd.GroupID = "platform"
	rootCmd.AddCommand(credentialsCmd)

	// Utility commands
	statusCmd := status.NewCommand()
	statusCmd.GroupID = "util"
	rootCmd.AddCommand(statusCmd)

	configCmd := newConfigCommand()
	configCmd.GroupID = "util"
	rootCmd.AddCommand(configCmd)

	profileCmd := profile.NewProfileCmd()
	profileCmd.GroupID = "util"
	rootCmd.AddCommand(profileCmd)

	exportCmd := admin.ExportCommand()
	exportCmd.GroupID = "util"
	rootCmd.AddCommand(exportCmd)

	return rootCmd
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ai-aas-cli")
	}

	viper.SetEnvPrefix("AI_AAS")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
		// Config file not found is OK - will use defaults or env vars
	}

	return nil
}

func runInit() error {
	// Initialize logging
	var logger *logging.Logger
	var err error
	if verbose {
		logger, err = logging.NewVerboseLogger()
	} else {
		logger, err = logging.NewDefaultLogger()
	}
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()

	wizard := config.NewInitWizard()
	return wizard.Run()
}

// showInitStatus displays the current configuration status
func showInitStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	success := color.New(color.FgGreen, color.Bold)
	warning := color.New(color.FgYellow)
	muted := color.New(color.FgHiBlack)
	label := color.New(color.FgWhite, color.Bold)

	checkMark := success.Sprint("✓")
	crossMark := color.New(color.FgRed, color.Bold).Sprint("✗")

	fmt.Println()
	label.Println("AI-AAS CLI Configuration Status")
	muted.Println("─────────────────────────────────────────────")
	fmt.Println()

	// Track what's configured for summary
	configuredCount := 0
	totalRequired := 3 // API endpoint, API key, environment
	totalOptional := 4 // HF token, User-Org, Admin API override, TLS

	// Required settings
	label.Println("Required Settings:")

	// API Endpoint (Admin API)
	if cfg.APIEndpoint != "" && cfg.APIEndpoint != "http://localhost:8080" {
		fmt.Printf("  %s Admin API Endpoint: %s\n", checkMark, cfg.APIEndpoint)
		configuredCount++
	} else if cfg.APIEndpoint == "http://localhost:8080" {
		fmt.Printf("  %s Admin API Endpoint: %s %s\n", warning.Sprint("~"), cfg.APIEndpoint, muted.Sprint("(default)"))
	} else {
		fmt.Printf("  %s Admin API Endpoint: %s\n", crossMark, muted.Sprint("(not set)"))
	}

	// API Key
	if cfg.APIKey != "" {
		fmt.Printf("  %s API Key:          %s\n", checkMark, config.MaskSecret(cfg.APIKey))
		configuredCount++
	} else {
		fmt.Printf("  %s API Key:          %s\n", crossMark, muted.Sprint("(not set)"))
	}

	// Environment
	if cfg.Environment != "" {
		fmt.Printf("  %s Environment:      %s\n", checkMark, cfg.Environment)
		configuredCount++
	} else {
		fmt.Printf("  %s Environment:      %s\n", crossMark, muted.Sprint("(not set)"))
	}

	fmt.Println()
	label.Println("Optional Settings:")

	optionalConfigured := 0

	// HuggingFace Token
	if cfg.HFToken != "" {
		fmt.Printf("  %s HuggingFace Token: %s\n", checkMark, config.MaskSecret(cfg.HFToken))
		optionalConfigured++
	} else {
		fmt.Printf("  %s HuggingFace Token: %s\n", muted.Sprint("-"), muted.Sprint("(not set - needed for model caching)"))
	}

	// User-Org Endpoint
	if cfg.UserOrgEndpoint != "" {
		fmt.Printf("  %s User-Org Service: %s\n", checkMark, cfg.UserOrgEndpoint)
		optionalConfigured++
	} else {
		fmt.Printf("  %s User-Org Service: %s\n", muted.Sprint("-"), muted.Sprint("(not set - uses API endpoint)"))
	}

	// Admin API Endpoint override (advanced)
	if cfg.AdminAPIEndpoint != "" {
		fmt.Printf("  %s Admin API Override: %s\n", checkMark, cfg.AdminAPIEndpoint)
		optionalConfigured++
	} else {
		fmt.Printf("  %s Admin API Override: %s\n", muted.Sprint("-"), muted.Sprint("(not set - uses main endpoint)"))
	}

	// TLS Settings
	if cfg.TLSInsecure {
		fmt.Printf("  %s TLS:              %s\n", warning.Sprint("!"), warning.Sprint("insecure (skip verify)"))
		optionalConfigured++
	} else if cfg.CACertFile != "" {
		fmt.Printf("  %s TLS:              %s\n", checkMark, fmt.Sprintf("CA: %s", cfg.CACertFile))
		optionalConfigured++
	} else {
		fmt.Printf("  %s TLS:              %s\n", muted.Sprint("-"), muted.Sprint("(default system CA)"))
	}

	// Summary
	fmt.Println()
	muted.Println("─────────────────────────────────────────────")

	if configuredCount >= totalRequired {
		success.Printf("Ready: %d/%d required settings configured\n", configuredCount, totalRequired)
	} else {
		warning.Printf("Incomplete: %d/%d required settings configured\n", configuredCount, totalRequired)
		fmt.Println()
		fmt.Println("Run 'ai-aas-cli --init' to complete setup")
	}

	if optionalConfigured > 0 {
		fmt.Printf("Optional: %d/%d settings configured\n", optionalConfigured, totalOptional)
	}

	// Config file location
	configPath, _ := config.GetConfigPath()
	fmt.Println()
	muted.Printf("Config file: %s\n", configPath)

	// Quick commands hint
	fmt.Println()
	label.Println("Quick Commands:")
	fmt.Printf("  %s %s\n", muted.Sprint("Run wizard:"), "ai-aas-cli --init")
	fmt.Printf("  %s %s\n", muted.Sprint("Set HF token:"), "ai-aas-cli --init-hf-token <token>")
	fmt.Printf("  %s %s\n", muted.Sprint("Test config:"), "ai-aas-cli config test")

	return nil
}

// runPartialInit updates specific configuration values without running the full wizard
func runPartialInit(domain, apiKey, env, hfToken, adminAPI, userOrgAPI string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	success := color.New(color.FgGreen, color.Bold)
	checkMark := success.Sprint("✓")
	updated := false

	// Update domain-based endpoints
	if domain != "" {
		// Derive endpoints from domain
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimSuffix(domain, "/")
		domain = strings.TrimPrefix(domain, "admin-api.")
		domain = strings.TrimPrefix(domain, "api.")

		cfg.APIEndpoint = fmt.Sprintf("https://admin-api.%s", domain)
		cfg.UserOrgEndpoint = fmt.Sprintf("https://user-org.%s", domain)
		fmt.Printf("%s Base domain set: %s\n", checkMark, domain)
		fmt.Printf("    → Admin API Endpoint: %s\n", cfg.APIEndpoint)
		fmt.Printf("    → User-Org Service:   %s\n", cfg.UserOrgEndpoint)
		updated = true
	}

	// Update API key
	if apiKey != "" {
		cfg.APIKey = apiKey
		fmt.Printf("%s API Key set: %s\n", checkMark, config.MaskSecret(apiKey))
		updated = true
	}

	// Update environment
	if env != "" {
		// Normalize environment
		switch strings.ToLower(env) {
		case "dev":
			env = "development"
		case "prod":
			env = "production"
		case "stage":
			env = "staging"
		}
		cfg.Environment = env
		fmt.Printf("%s Environment set: %s\n", checkMark, env)
		updated = true
	}

	// Update HuggingFace token
	if hfToken != "" {
		cfg.HFToken = hfToken
		fmt.Printf("%s HuggingFace token set: %s\n", checkMark, config.MaskSecret(hfToken))
		updated = true

		// Also sync to Admin API if configured
		adminEndpoint := cfg.AdminAPIEndpoint
		if adminEndpoint == "" {
			adminEndpoint = cfg.APIEndpoint
		}
		if adminEndpoint != "" && cfg.APIKey != "" {
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			credClient := credentials.NewClient(apiClient)
			ctx := context.Background()
			if err := credClient.Set(ctx, "hf-token", hfToken, nil); err != nil {
				output.WarningMsg("Could not sync HF token to Admin API: %v", err)
			} else {
				fmt.Printf("%s HuggingFace token synced to Admin API\n", checkMark)
			}
		}
	}

	// Update Admin API endpoint
	if adminAPI != "" {
		cfg.AdminAPIEndpoint = adminAPI
		fmt.Printf("%s Admin API endpoint set: %s\n", checkMark, adminAPI)
		updated = true
	}

	// Update User-Org API endpoint
	if userOrgAPI != "" {
		cfg.UserOrgEndpoint = userOrgAPI
		fmt.Printf("%s User-Org API endpoint set: %s\n", checkMark, userOrgAPI)
		updated = true
	}

	if !updated {
		return fmt.Errorf("no configuration values provided. Use --help for available options")
	}

	// Save configuration
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	output.SuccessMsg("Configuration updated successfully")
	fmt.Println("Run 'ai-aas-cli --init-status' to see current configuration")

	return nil
}

// newConfigCommand creates the config subcommand
func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  "View and modify ai-aas-cli configuration settings.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if outputFormat == "json" {
				return output.PrintJSON(cfg, true)
			}

			fmt.Println("Configuration:")
			fmt.Printf("  Admin API Endpoint: %s\n", cfg.APIEndpoint)
			fmt.Printf("  Environment:        %s\n", cfg.Environment)
			fmt.Printf("  API Key:            %s\n", config.MaskSecret(cfg.APIKey))
			fmt.Printf("  Output Format:      %s\n", cfg.OutputFormat)
			fmt.Printf("  Verbose:            %v\n", cfg.Verbose)
			if cfg.AdminAPIEndpoint != "" {
				fmt.Printf("  Admin API Override: %s\n", cfg.AdminAPIEndpoint)
			}

			configPath, _ := config.GetConfigPath()
			fmt.Printf("\n  Config File:   %s\n", configPath)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Set(args[0], args[1]); err != nil {
				return fmt.Errorf("set config: %w", err)
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Test configuration connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			fmt.Println("Testing configuration...")
			fmt.Printf("  Admin API Endpoint: %s\n", cfg.APIEndpoint)
			fmt.Printf("  API Key:            %s\n", config.MaskSecret(cfg.APIKey))
			fmt.Printf("  Environment:        %s\n", cfg.Environment)
			if cfg.AdminAPIEndpoint != "" {
				fmt.Printf("  Admin API Override: %s\n", cfg.AdminAPIEndpoint)
			}
			fmt.Println()

			// Test API connection (use AdminAPIEndpoint if set, otherwise APIEndpoint)
			testEndpoint := cfg.APIEndpoint
			if cfg.AdminAPIEndpoint != "" {
				testEndpoint = cfg.AdminAPIEndpoint
			}
			fmt.Print("  Admin API connection: testing... ")
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			client := api.NewClient(testEndpoint, cfg.APIKey, opts...)
			ctx := context.Background()

			if err := client.Ping(ctx); err != nil {
				fmt.Println("FAILED")
				fmt.Printf("\n  Error: %v\n", err)
				fmt.Println("\n  Troubleshooting:")
				fmt.Println("    - Verify the API endpoint is correct")
				fmt.Println("    - Check if the API service is running")
				fmt.Println("    - Verify your API key is valid")
				return fmt.Errorf("API connection failed: %w", err)
			}
			fmt.Println("OK")

			fmt.Println()
			fmt.Println("All configuration tests passed!")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "check-path",
		Short: "Check if CLI is in PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			config.PrintPathInstructions()
			return nil
		},
	})

	return cmd
}

// newCredentialsCommand creates the credentials subcommand
func newCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Manage platform credentials",
		Long:  "Store and manage HuggingFace tokens and object storage credentials.",
	}

	// Set command with flags for S3
	var s3Endpoint, s3AccessKey, s3SecretKey, s3Bucket string
	setCmd := &cobra.Command{
		Use:   "set <type> [value]",
		Short: "Set a credential (hf-token, s3)",
		Long: `Set a credential for platform operations.

Examples:
  # Set HuggingFace token
  ai-aas-cli credentials set hf-token hf_xxxxxxxxxxxxx

  # Set S3 credentials
  ai-aas-cli credentials set s3 --endpoint https://s3.example.com --access-key AKIAXXXX --secret-key xxxxx --bucket models`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credType := args[0]
			ctx := context.Background()

			switch credType {
			case "hf-token":
				if len(args) < 2 {
					return fmt.Errorf("usage: credentials set hf-token <token>")
				}
				value := args[1]
				// Store HuggingFace token locally
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				cfg.HFToken = value
				if err := config.Save(cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
				fmt.Println("✓ HuggingFace token saved locally")

				// Also store in Admin API for server-side operations
				apiCfg, err := config.Load()
				if err == nil {
					adminEndpoint := apiCfg.AdminAPIEndpoint
					if adminEndpoint == "" {
						adminEndpoint = apiCfg.APIEndpoint
					}
					if adminEndpoint != "" && apiCfg.APIKey != "" {
						opts := []api.ClientOption{}
						if apiCfg.TLSInsecure {
							opts = append(opts, api.WithInsecureSkipVerify())
						}
						apiClient := api.NewClient(adminEndpoint, apiCfg.APIKey, opts...)
						credClient := credentials.NewClient(apiClient)
						if err := credClient.Set(ctx, "hf-token", value, nil); err != nil {
							fmt.Printf("  (Note: Could not sync to Admin API: %v)\n", err)
						} else {
							fmt.Println("✓ HuggingFace token synced to Admin API")
						}
					}
				}
			case "s3":
				if s3Endpoint == "" || s3AccessKey == "" || s3SecretKey == "" {
					return fmt.Errorf("S3 credentials require --endpoint, --access-key, and --secret-key flags")
				}
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if cfg.APIEndpoint == "" || cfg.APIKey == "" {
					return fmt.Errorf("API not configured. Run 'ai-aas-cli --init' first")
				}

				apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
				credClient := credentials.NewClient(apiClient)

				// Store S3 credentials via Admin API
				metadata := map[string]interface{}{
					"endpoint": s3Endpoint,
					"bucket":   s3Bucket,
				}
				// Combine access key and secret for storage
				s3Value := fmt.Sprintf("%s:%s", s3AccessKey, s3SecretKey)
				if err := credClient.Set(ctx, "s3", s3Value, metadata); err != nil {
					return fmt.Errorf("set S3 credentials: %w", err)
				}
				fmt.Println("✓ S3 credentials saved via Admin API")
			default:
				return fmt.Errorf("unknown credential type: %s (supported: hf-token, s3)", credType)
			}
			return nil
		},
	}
	setCmd.Flags().StringVar(&s3Endpoint, "endpoint", "", "S3 endpoint URL")
	setCmd.Flags().StringVar(&s3AccessKey, "access-key", "", "S3 access key")
	setCmd.Flags().StringVar(&s3SecretKey, "secret-key", "", "S3 secret key")
	setCmd.Flags().StringVar(&s3Bucket, "bucket", "", "S3 bucket name")
	cmd.AddCommand(setCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			fmt.Println("Local credentials:")
			if cfg.HFToken != "" {
				fmt.Printf("  hf-token: %s\n", config.MaskSecret(cfg.HFToken))
			} else {
				fmt.Println("  hf-token: (not set)")
			}

			// List credentials from Admin API
			if cfg.APIEndpoint != "" && cfg.APIKey != "" {
				fmt.Println("\nAdmin API credentials:")
				apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
				credClient := credentials.NewClient(apiClient)

				ctx := context.Background()
				creds, err := credClient.List(ctx)
				if err != nil {
					fmt.Printf("  (could not fetch: %v)\n", err)
				} else if len(creds) == 0 {
					fmt.Println("  (none configured)")
				} else {
					for _, c := range creds {
						fmt.Printf("  %s: %s (updated: %s)\n", c.Type, c.Masked, c.UpdatedAt.Format("2006-01-02"))
					}
				}
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "test <type>",
		Short: "Test a credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credType := args[0]
			ctx := context.Background()

			switch credType {
			case "hf-token":
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if cfg.HFToken == "" {
					return fmt.Errorf("HuggingFace token not configured")
				}

				fmt.Println("Testing HuggingFace token...")
				client := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))

				user, err := client.WhoAmI(ctx)
				if err != nil {
					fmt.Printf("✗ Token validation failed: %v\n", err)
					return nil
				}

				fmt.Printf("✓ Token valid for user: %s (%s)\n", user.Name, user.Email)
			case "s3":
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if cfg.APIEndpoint == "" || cfg.APIKey == "" {
					return fmt.Errorf("API not configured. Run 'ai-aas-cli --init' first")
				}

				fmt.Println("Testing S3 credentials via Admin API...")
				apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
				credClient := credentials.NewClient(apiClient)

				result, err := credClient.Test(ctx, "s3")
				if err != nil {
					fmt.Printf("✗ Test failed: %v\n", err)
					return nil
				}

				if result.Valid {
					fmt.Printf("✓ S3 credentials valid: %s\n", result.Message)
				} else {
					fmt.Printf("✗ S3 credentials invalid: %s\n", result.Message)
				}
			default:
				return fmt.Errorf("unknown credential type: %s", credType)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <type>",
		Short: "Delete a credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credType := args[0]
			ctx := context.Background()

			switch credType {
			case "hf-token":
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				cfg.HFToken = ""
				if err := config.Save(cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
				fmt.Println("✓ HuggingFace token deleted locally")

				// Also delete from Admin API
				if cfg.APIEndpoint != "" && cfg.APIKey != "" {
					apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
					credClient := credentials.NewClient(apiClient)
					if err := credClient.Delete(ctx, "hf-token"); err != nil {
						fmt.Printf("  (Note: Could not delete from Admin API: %v)\n", err)
					} else {
						fmt.Println("✓ HuggingFace token deleted from Admin API")
					}
				}
			case "s3":
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if cfg.APIEndpoint == "" || cfg.APIKey == "" {
					return fmt.Errorf("API not configured. Run 'ai-aas-cli --init' first")
				}

				apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
				credClient := credentials.NewClient(apiClient)

				if err := credClient.Delete(ctx, "s3"); err != nil {
					return fmt.Errorf("delete S3 credentials: %w", err)
				}
				fmt.Println("✓ S3 credentials deleted from Admin API")
			default:
				return fmt.Errorf("unknown credential type: %s", credType)
			}
			return nil
		},
	})

	return cmd
}

// newModelCommand creates the model subcommand with nested command structure
func newModelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Model lifecycle management",
		Long: `Comprehensive AI model management for the AI-AAS platform.

The model command provides nested subcommands organized by lifecycle stage:

  registry      Discover and register models from HuggingFace Hub
  cache         Download and manage local model files in S3 storage
  deploy        Create and manage KServe InferenceService deployments
  troubleshoot  Debug deployment issues with logs, events, and diagnostics
  version       Manage model versions, updates, and pinning
  library       Manage organization's enabled model library

Quick Deploy (recommended):
  ai-aas-cli model hf-deploy https://huggingface.co/unsloth/gpt-oss-20b

Manual Workflow:
  1. ai-aas-cli model registry add meta-llama/Llama-2-7b-hf
  2. ai-aas-cli model cache pull meta-llama/Llama-2-7b-hf
  3. ai-aas-cli model deploy create meta-llama/Llama-2-7b-hf
  4. ai-aas-cli model deploy status meta-llama/Llama-2-7b-hf

Getting Help:
  ai-aas-cli model --help                    Show this help
  ai-aas-cli model registry --help           Show registry commands
  ai-aas-cli model deploy create --help      Show deploy create options

For more information, see: https://docs.ai-aas.io/cli/model`,
	}

	// Add nested parent commands (each has its own subcommands)
	cmd.AddCommand(model.NewHFDeployCommand())
	cmd.AddCommand(model.NewRegistryCommand())
	cmd.AddCommand(model.NewCacheParentCommand())
	cmd.AddCommand(model.NewDeployParentCommand())
	cmd.AddCommand(model.NewTroubleshootParentCommand())
	cmd.AddCommand(model.NewVersionParentCommand())
	cmd.AddCommand(model.NewLibraryParentCommand())

	return cmd
}

// maskSecret masks all but last 4 characters of a secret
func maskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}
