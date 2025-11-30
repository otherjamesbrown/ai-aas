// Package cmd provides the CLI command implementations for ai-aas-cli.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/cmd/model"
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
	cfgFile     string
	verbose     bool
	quiet       bool
	outputFormat string
)

// NewRootCommand creates the root command for ai-aas-cli
func NewRootCommand(version, buildTime, gitCommit string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ai-aas-cli",
		Short: "AI-AAS Platform CLI for model and platform management",
		Long: `ai-aas-cli provides comprehensive management for the AI-AAS platform.

Quick Start:
  ai-aas-cli --init                       Initialize CLI configuration
  ai-aas-cli model registry list          List registered models
  ai-aas-cli model deploy create <model>  Deploy a model
  ai-aas-cli org list                     List organizations

Model Workflow:
  ai-aas-cli model registry add meta-llama/Llama-2-7b-hf   # Register
  ai-aas-cli model cache pull meta-llama/Llama-2-7b-hf     # Cache locally
  ai-aas-cli model deploy create meta-llama/Llama-2-7b-hf  # Deploy

Getting Help:
  ai-aas-cli --help                       Show this help
  ai-aas-cli model --help                 Show model management commands
  ai-aas-cli model deploy --help          Show deployment subcommands
  ai-aas-cli model deploy create --help   Show create deployment options

Environment Variables:
  AI_AAS_ENVIRONMENT    Target environment (dev/staging/prod)
  AI_AAS_API_KEY        API key for authentication
  AI_AAS_API_ENDPOINT   Admin API endpoint URL`,
		Version: fmt.Sprintf("%s (build: %s, commit: %s)", version, buildTime, gitCommit),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ai-aas-cli.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "table", "output format (table, json, csv)")

	// Init flag - handled via RunE since it's on the root command
	var initFlag bool
	rootCmd.Flags().BoolVar(&initFlag, "init", false, "run initialization wizard")
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if initFlag {
			return runInit()
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
			fmt.Printf("  API Endpoint:  %s\n", cfg.APIEndpoint)
			fmt.Printf("  Environment:   %s\n", cfg.Environment)
			fmt.Printf("  API Key:       %s\n", config.MaskSecret(cfg.APIKey))
			fmt.Printf("  Output Format: %s\n", cfg.OutputFormat)
			fmt.Printf("  Verbose:       %v\n", cfg.Verbose)

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
			fmt.Printf("  API Endpoint: %s\n", cfg.APIEndpoint)
			fmt.Printf("  API Key:      %s\n", config.MaskSecret(cfg.APIKey))
			fmt.Printf("  Environment:  %s\n", cfg.Environment)
			fmt.Println()

			// Test API connection
			fmt.Print("  API connection: testing... ")
			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			client := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
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
				if err == nil && apiCfg.APIEndpoint != "" && apiCfg.APIKey != "" {
					apiClient := api.NewClient(apiCfg.APIEndpoint, apiCfg.APIKey)
					credClient := credentials.NewClient(apiClient)
					if err := credClient.Set(ctx, "hf-token", value, nil); err != nil {
						fmt.Printf("  (Note: Could not sync to Admin API: %v)\n", err)
					} else {
						fmt.Println("✓ HuggingFace token synced to Admin API")
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

Typical Workflow:
  1. ai-aas-cli model registry add meta-llama/Llama-2-7b-hf
  2. ai-aas-cli model cache pull meta-llama/Llama-2-7b-hf
  3. ai-aas-cli model deploy create meta-llama/Llama-2-7b-hf
  4. ai-aas-cli model troubleshoot status meta-llama/Llama-2-7b-hf

Getting Help:
  ai-aas-cli model --help                    Show this help
  ai-aas-cli model registry --help           Show registry commands
  ai-aas-cli model deploy create --help      Show deploy create options

For more information, see: https://docs.ai-aas.io/cli/model`,
	}

	// Add nested parent commands (each has its own subcommands)
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
