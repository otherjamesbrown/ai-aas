// Package cmd implements the CLI commands for ai-aas-org.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

var (
	cfgFile string
	guided  bool

	// Version information
	versionInfo struct {
		Version   string
		Commit    string
		BuildTime string
	}
)

// SetVersionInfo sets the version information for display.
func SetVersionInfo(version, commit, buildTime string) {
	versionInfo.Version = version
	versionInfo.Commit = commit
	versionInfo.BuildTime = buildTime
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ai-aas-org",
	Short: "AI-as-a-Service Organization Admin CLI",
	Long: `ai-aas-org is a command-line tool for organization administrators
to manage their AI-as-a-Service organization.

With this CLI you can:
  • Create and manage users in your organization
  • Generate and manage API keys for users
  • View available AI models
  • Monitor token usage and costs

Getting Started:
  1. Redeem your bootstrap key:     ai-aas-org init --key <bootstrap-key>
  2. View available commands:       ai-aas-org --help
  3. Start with guided mode:        ai-aas-org --guided

For detailed help on any command, run:
  ai-aas-org <command> --help`,
	Run: func(cmd *cobra.Command, args []string) {
		if guided {
			runGuidedMode()
			return
		}
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.ai-aas-org.yaml)")
	rootCmd.PersistentFlags().BoolVar(&guided, "guided", false, "run in guided mode with interactive prompts")
	rootCmd.PersistentFlags().Bool("json", false, "output in JSON format")
	rootCmd.PersistentFlags().Bool("quiet", false, "suppress non-essential output")

	// Bind flags to viper
	viper.BindPFlag("output.json", rootCmd.PersistentFlags().Lookup("json"))
	viper.BindPFlag("output.quiet", rootCmd.PersistentFlags().Lookup("quiet"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Search config in home directory with name ".ai-aas-org" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ai-aas-org")
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvPrefix("AI_AAS_ORG")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		// Config loaded successfully - no need to print unless debugging
	}
}

// runGuidedMode runs the interactive guided mode for new users.
func runGuidedMode() {
	output.Header("Welcome to AI-as-a-Service Organization Admin CLI")
	fmt.Println()
	fmt.Println("This guided mode will help you get started.")
	fmt.Println()

	// Check if already configured
	if viper.IsSet("api_endpoint") && viper.IsSet("api_key") {
		output.SuccessMsg("You're already configured!")
		fmt.Println()
		fmt.Println("Here are some things you can do:")
		fmt.Println()
		fmt.Println("  • List users:      ai-aas-org user list")
		fmt.Println("  • Create a user:   ai-aas-org user create --guided")
		fmt.Println("  • View models:     ai-aas-org model list")
		fmt.Println("  • Check usage:     ai-aas-org usage summary")
		fmt.Println()
		return
	}

	// Not configured - guide through setup
	output.WarningMsg("It looks like you haven't set up the CLI yet.")
	fmt.Println()
	fmt.Println("To get started, you need a bootstrap key from your platform administrator.")
	fmt.Println()
	fmt.Println("Once you have your key, run:")
	fmt.Println()
	fmt.Printf("  ai-aas-org init --key <your-bootstrap-key>\n")
	fmt.Println()
	fmt.Println("If you already have API credentials, you can configure manually:")
	fmt.Println()
	fmt.Printf("  ai-aas-org configure\n")
	fmt.Println()
}

// IsGuidedMode returns whether guided mode is enabled.
func IsGuidedMode() bool {
	return guided
}

// IsJSONOutput returns whether JSON output is requested.
func IsJSONOutput() bool {
	return viper.GetBool("output.json")
}

// IsQuiet returns whether quiet mode is enabled.
func IsQuiet() bool {
	return viper.GetBool("output.quiet")
}
