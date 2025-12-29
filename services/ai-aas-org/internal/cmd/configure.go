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

  # Configure inference endpoint for model listing
  ai-aas-org configure --inference-endpoint https://inference.ai-aas.example.com

Or run interactively:
  ai-aas-org configure --guided`,
	RunE: runConfigure,
}

var (
	configAPIEndpoint       string
	configInferenceEndpoint string
	configAPIKey            string
	configOrgID             string
)

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVar(&configAPIEndpoint, "api-endpoint", "", "API endpoint URL (user-org-service)")
	configureCmd.Flags().StringVar(&configInferenceEndpoint, "inference-endpoint", "", "Inference API endpoint URL (for model listing)")
	configureCmd.Flags().StringVar(&configAPIKey, "api-key", "", "API key for authentication")
	configureCmd.Flags().StringVar(&configOrgID, "org-id", "", "Organization ID")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	// If only inference-endpoint is provided, update just that field
	if configInferenceEndpoint != "" && configAPIEndpoint == "" && configAPIKey == "" {
		if err := config.Update(map[string]string{
			"inference_endpoint": configInferenceEndpoint,
		}); err != nil {
			return errors.NewOperationError(
				"failed to update configuration",
				fmt.Sprintf("Error: %v", err),
			)
		}
		output.SuccessMsg("Inference endpoint configured: %s", configInferenceEndpoint)
		return nil
	}

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
			APIEndpoint:       configAPIEndpoint,
			InferenceEndpoint: configInferenceEndpoint,
			APIKey:            configAPIKey,
			OrgID:             configOrgID,
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
	endpoint, err := prompt.Input("API Endpoint (user-org-service)", "https://api.ai-aas.example.com")
	if err != nil {
		return cfg, errors.NewOperationError("failed to read input", err.Error())
	}
	cfg.APIEndpoint = endpoint

	// Inference Endpoint
	inferenceEndpoint, err := prompt.Input("Inference Endpoint (for model listing)", "https://inference.ai-aas.example.com")
	if err != nil {
		return cfg, errors.NewOperationError("failed to read input", err.Error())
	}
	cfg.InferenceEndpoint = inferenceEndpoint

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
