// Package config handles configuration for the ai-aas-org CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds the CLI configuration.
type Config struct {
	// API endpoint for the AI-as-a-Service platform
	APIEndpoint string `yaml:"api_endpoint" mapstructure:"api_endpoint"`
	// API key for authentication
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
	// Organization ID
	OrgID string `yaml:"org_id" mapstructure:"org_id"`
	// Organization name
	OrgName string `yaml:"org_name" mapstructure:"org_name"`
	// Admin user email
	AdminEmail string `yaml:"admin_email" mapstructure:"admin_email"`
	// Admin user ID
	AdminUserID string `yaml:"admin_user_id" mapstructure:"admin_user_id"`
}

// DefaultConfigFile returns the default config file path.
func DefaultConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai-aas-org.yaml"
	}
	return filepath.Join(home, ".ai-aas-org.yaml")
}

// Load loads configuration from file and environment.
func Load() (*Config, error) {
	cfg := &Config{}

	// Set defaults
	viper.SetDefault("api_endpoint", "")
	viper.SetDefault("api_key", "")
	viper.SetDefault("org_id", "")
	viper.SetDefault("org_name", "")
	viper.SetDefault("admin_email", "")
	viper.SetDefault("admin_user_id", "")

	// Unmarshal into config struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to file.
func Save(cfg *Config) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = DefaultConfigFile()
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file with secure permissions
	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// IsConfigured returns true if the CLI has been configured with credentials.
func IsConfigured() bool {
	return viper.GetString("api_endpoint") != "" && viper.GetString("api_key") != ""
}

// GetAPIEndpoint returns the configured API endpoint.
func GetAPIEndpoint() string {
	return viper.GetString("api_endpoint")
}

// GetAPIKey returns the configured API key.
func GetAPIKey() string {
	return viper.GetString("api_key")
}

// GetOrgID returns the configured organization ID.
func GetOrgID() string {
	return viper.GetString("org_id")
}

// GetOrgName returns the configured organization name.
func GetOrgName() string {
	return viper.GetString("org_name")
}

// GetAdminEmail returns the configured admin email.
func GetAdminEmail() string {
	return viper.GetString("admin_email")
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.APIEndpoint == "" {
		return fmt.Errorf("api_endpoint is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if c.OrgID == "" {
		return fmt.Errorf("org_id is required")
	}
	return nil
}

// Update updates specific fields in the config and saves.
func Update(updates map[string]string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	for key, value := range updates {
		switch key {
		case "api_endpoint":
			cfg.APIEndpoint = value
		case "api_key":
			cfg.APIKey = value
		case "org_id":
			cfg.OrgID = value
		case "org_name":
			cfg.OrgName = value
		case "admin_email":
			cfg.AdminEmail = value
		case "admin_user_id":
			cfg.AdminUserID = value
		default:
			return fmt.Errorf("unknown config key: %s", key)
		}
	}

	return Save(cfg)
}
