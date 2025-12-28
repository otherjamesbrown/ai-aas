package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/prompt"
)

// configureCmd represents the configure command
var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure CLI credentials manually",
	Long: `Configure the ai-aas-org CLI with API credentials.

This command is for advanced users who already have API credentials.
Most users should use 'ai-aas-org init --key <bootstrap-key>' instead.

Example:
  ai-aas-org configure --api-endpoint https://api.ai-aas.example.com --api-key <key>

Or run interactively:
  ai-aas-org configure --guided`,
	RunE: runConfigure,
}

var (
	configAPIEndpoint string
	configAPIKey      string
	configOrgID       string
)

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVar(&configAPIEndpoint, "api-endpoint", "", "API endpoint URL")
	configureCmd.Flags().StringVar(&configAPIKey, "api-key", "", "API key for authentication")
	configureCmd.Flags().StringVar(&configOrgID, "org-id", "", "Organization ID")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	var cfg config.Config
	var err error

	// Use guided mode if enabled or if missing required values
	if IsGuidedMode() || (configAPIEndpoint == "" && configAPIKey == "") {
		cfg, err = runConfigureGuided()
		if err != nil {
			return err
		}
	} else {
		// Use flag values
		if configAPIEndpoint == "" {
			return errors.NewUsageError("--api-endpoint is required")
		}
		if configAPIKey == "" {
			return errors.NewUsageError("--api-key is required")
		}

		cfg = config.Config{
			APIEndpoint: configAPIEndpoint,
			APIKey:      configAPIKey,
			OrgID:       configOrgID,
		}
	}

	// Save configuration
	if err := config.Save(&cfg); err != nil {
		return errors.NewOperationError(
			"failed to save configuration",
			fmt.Sprintf("Error: %v", err),
		)
	}

	output.SuccessMsg("Configuration saved to %s", config.DefaultConfigFile())
	return nil
}

func runConfigureGuided() (config.Config, error) {
	var cfg config.Config

	output.Header("Configure ai-aas-org CLI")
	fmt.Println()
	fmt.Println("Enter your API credentials. If you don't have these,")
	fmt.Println("use 'ai-aas-org init --key <bootstrap-key>' instead.")
	fmt.Println()

	// API Endpoint
	endpoint, err := prompt.Input("API Endpoint", "https://api.ai-aas.example.com")
	if err != nil {
		return cfg, errors.NewOperationError("failed to read input", err.Error())
	}
	cfg.APIEndpoint = endpoint

	// API Key
	apiKey, err := prompt.Password("API Key")
	if err != nil {
		return cfg, errors.NewOperationError("failed to read input", err.Error())
	}
	if apiKey == "" {
		return cfg, errors.NewValidationError("API key is required", "")
	}
	cfg.APIKey = apiKey

	// Organization ID (optional)
	orgID, err := prompt.Input("Organization ID (optional)", "")
	if err != nil {
		return cfg, errors.NewOperationError("failed to read input", err.Error())
	}
	cfg.OrgID = orgID

	return cfg, nil
}
