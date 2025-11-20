// Package config provides configuration management for the Admin CLI.
//
// Purpose:
//
//	Load configuration from multiple sources: environment variables, config files
//	(YAML/JSON), and command-line flags. Uses Viper for configuration management
//	with clear precedence: flags > environment variables > config file > defaults.
//
// Dependencies:
//   - github.com/spf13/viper: Configuration management
//   - internal/config/defaults: Default configuration values
//
// Configuration Sources:
//   - Environment variables: ADMIN_CLI_* prefix (e.g., ADMIN_CLI_USER_ORG_ENDPOINT)
//   - Config file: ~/.admin-cli/config.yaml (or explicit path via --config flag)
//   - Command-line flags: Take precedence over all other sources
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#NFR-024 (configuration file support)
//   - specs/009-admin-cli/plan.md#config
//
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all CLI configuration.
type Config struct {
	// API Endpoints
	UserOrgEndpoint   string
	AnalyticsEndpoint string

	// Authentication
	APIKey string

	// Database
	DatabaseURL string

	// Output Settings
	OutputFormat string // table, json, csv
	Verbose      bool
	Quiet        bool

	// Retry Settings
	MaxRetries int
	Timeout    int // seconds

	// Config File Path (for discovery)
	ConfigFile string
}

// Load loads configuration from all sources with proper precedence.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	ApplyDefaults(v)

	// Set environment variable prefix
	v.SetEnvPrefix("ADMIN_CLI")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// Config file discovery
	homeDir, err := os.UserHomeDir()
	if err == nil {
		v.AddConfigPath(filepath.Join(homeDir, ".admin-cli"))
	}
	v.AddConfigPath(".") // Current directory
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Read config file (optional - ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	cfg := &Config{
		UserOrgEndpoint:   v.GetString("api-endpoints.user-org-service"),
		AnalyticsEndpoint: v.GetString("api-endpoints.analytics-service"),
		APIKey:            v.GetString("auth.api-key"),
		DatabaseURL:       v.GetString("database.url"),
		OutputFormat:      v.GetString("defaults.output-format"),
		Verbose:           v.GetBool("defaults.verbose"),
		Quiet:             v.GetBool("defaults.quiet"),
		MaxRetries:        v.GetInt("retry.max-attempts"),
		Timeout:           v.GetInt("retry.timeout"),
		ConfigFile:        v.ConfigFileUsed(),
	}

	return cfg, nil
}

// LoadWithFlags loads configuration and applies flag overrides.
func LoadWithFlags(flagOverrides map[string]interface{}) (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	// Apply flag overrides
	for key, value := range flagOverrides {
		switch key {
		case "user-org-endpoint":
			if v, ok := value.(string); ok {
				cfg.UserOrgEndpoint = v
			}
		case "analytics-endpoint":
			if v, ok := value.(string); ok {
				cfg.AnalyticsEndpoint = v
			}
		case "api-key":
			if v, ok := value.(string); ok {
				cfg.APIKey = v
			}
		case "format":
			if v, ok := value.(string); ok {
				cfg.OutputFormat = v
			}
		case "verbose":
			if v, ok := value.(bool); ok {
				cfg.Verbose = v
			}
		case "quiet":
			if v, ok := value.(bool); ok {
				cfg.Quiet = v
			}
		}
	}

	return cfg, nil
}

