package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-org/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/errors"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
	"github.com/otherjamesbrown/ai-aas/services/cli-shared/prompt"
)

var (
	bootstrapKey string
	apiEndpoint  string
	forceInit    bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the CLI with your bootstrap key",
	Long: `Initialize the ai-aas-org CLI by redeeming your bootstrap key.

Your organization's platform administrator will provide you with a one-time
bootstrap key. This command exchanges that key for permanent API credentials.

Example:
  ai-aas-org init --key bsk_abc123xyz...

The bootstrap key can only be used once and expires after 7 days.
After initialization, your credentials will be saved to ~/.ai-aas-org.yaml`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&bootstrapKey, "key", "k", "", "bootstrap key provided by your platform administrator")
	initCmd.Flags().StringVar(&apiEndpoint, "api-endpoint", "", "API endpoint (usually auto-configured)")
	initCmd.Flags().BoolVar(&forceInit, "force", false, "overwrite existing configuration")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Check if already configured
	if config.IsConfigured() && !forceInit {
		output.WarningMsg("CLI is already configured for organization: %s", config.GetOrgName())
		fmt.Println()
		fmt.Println("Use --force to overwrite existing configuration.")
		return nil
	}

	// Get bootstrap key
	key := bootstrapKey
	if key == "" {
		if IsGuidedMode() || isInteractive() {
			var err error
			key, err = prompt.InputRequired("Enter your bootstrap key")
			if err != nil {
				return errors.NewOperationError("failed to read bootstrap key", "Please provide the key using --key flag")
			}
		} else {
			return errors.NewUsageError("--key is required")
		}
	}

	// Validate bootstrap key format
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, "bsk_") {
		return errors.NewValidationError(
			"invalid bootstrap key format",
			"Bootstrap keys start with 'bsk_'. Please check your key and try again.",
		)
	}

	// Determine API endpoint
	endpoint := apiEndpoint
	if endpoint == "" {
		endpoint = viper.GetString("api_endpoint")
	}
	if endpoint == "" {
		// Try to extract from bootstrap key or use default
		endpoint = extractEndpointFromKey(key)
		if endpoint == "" {
			endpoint = "https://api.ai-aas.example.com" // Default - will be configured properly in deployment
		}
	}

	output.InfoMsg("Redeeming bootstrap key...")

	// Create API client and redeem key
	client := api.NewClient(endpoint, "")
	result, err := client.RedeemBootstrapKey(ctx, key)
	if err != nil {
		return errors.NewOperationError(
			err.Error(),
			"Please verify your bootstrap key is correct and hasn't expired. Contact your platform administrator if the issue persists.",
		)
	}

	// Save configuration
	cfg := &config.Config{
		APIEndpoint: endpoint,
		APIKey:      result.APIKey,
		OrgID:       result.OrgID,
		OrgName:     result.OrgName,
		AdminEmail:  result.AdminEmail,
		AdminUserID: result.AdminUserID,
	}

	if err := config.Save(cfg); err != nil {
		return errors.NewOperationError(
			"failed to save configuration",
			fmt.Sprintf("Error: %v. Try running with sudo or check file permissions.", err),
		)
	}

	// Display success
	fmt.Println()
	output.SuccessMsg("Successfully initialized!")
	fmt.Println()
	output.Header("Organization Details")
	output.KeyValue("Organization", result.OrgName)
	output.KeyValue("Org ID", result.OrgID)
	output.KeyValue("Admin Email", result.AdminEmail)
	output.KeyValue("Config File", config.DefaultConfigFile())
	fmt.Println()

	// Show next steps
	output.Header("Next Steps")
	fmt.Println()
	fmt.Println("  1. View your organization:    ai-aas-org org show")
	fmt.Println("  2. List available models:     ai-aas-org model list")
	fmt.Println("  3. Create a user:             ai-aas-org user create --guided")
	fmt.Println("  4. View usage:                ai-aas-org usage summary")
	fmt.Println()
	fmt.Println("For help with any command, run:")
	fmt.Println("  ai-aas-org <command> --help")
	fmt.Println()

	return nil
}

// isInteractive checks if stdin is a terminal.
func isInteractive() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// extractEndpointFromKey attempts to extract API endpoint from bootstrap key metadata.
// Bootstrap keys may contain encoded endpoint info.
func extractEndpointFromKey(key string) string {
	// For now, return empty - endpoint encoding in keys is a future enhancement
	return ""
}
