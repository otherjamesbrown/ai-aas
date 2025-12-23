// Package config provides configuration management for the CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// ConfigFileName is the name of the config file without extension
	ConfigFileName = ".ai-aas-cli"
	// ConfigFileType is the config file format
	ConfigFileType = "yaml"
)

// Config holds the CLI configuration
type Config struct {
	// Primary Admin API endpoint - this is the main API endpoint for all CLI operations
	// Set via config file key 'api_endpoint' or environment variable 'AI_AAS_API_ENDPOINT'
	APIEndpoint string `mapstructure:"api_endpoint" json:"api_endpoint"`
	APIKey      string `mapstructure:"api_key" json:"api_key"`
	Environment string `mapstructure:"environment" json:"environment"`
	HFToken     string `mapstructure:"hf_token" json:"hf_token,omitempty"`
	Verbose     bool   `mapstructure:"verbose" json:"verbose"`
	Quiet       bool   `mapstructure:"quiet" json:"quiet"`
	OutputFormat string `mapstructure:"output_format" json:"output_format"`

	// Default organization (set with 'ai-aas-cli org use <org-id>')
	DefaultOrgID string `mapstructure:"default_org_id" json:"default_org_id,omitempty"`

	// Service-specific endpoints (optional overrides for advanced use cases)
	// These are rarely needed - APIEndpoint is used for Admin API operations by default
	UserOrgEndpoint   string `mapstructure:"user_org_endpoint" json:"user_org_endpoint,omitempty"`
	AnalyticsEndpoint string `mapstructure:"analytics_endpoint" json:"analytics_endpoint,omitempty"`
	InferenceEndpoint string `mapstructure:"inference_endpoint" json:"inference_endpoint,omitempty"`
	// AdminAPIEndpoint is deprecated - use APIEndpoint instead. Kept for backward compatibility only.
	// If set, this overrides APIEndpoint for admin operations.
	// Set via config file key 'admin_api_endpoint' or environment variable 'AI_AAS_ADMIN_API_ENDPOINT'
	AdminAPIEndpoint  string `mapstructure:"admin_api_endpoint" json:"admin_api_endpoint,omitempty"`

	// TLS Configuration
	CACertFile     string `mapstructure:"ca_cert_file" json:"ca_cert_file,omitempty"`
	TLSInsecure    bool   `mapstructure:"tls_insecure" json:"tls_insecure,omitempty"`

	// Database (for direct DB commands)
	DatabaseURL string `mapstructure:"database_url" json:"database_url,omitempty"`

	// Retry Settings
	MaxRetries int `mapstructure:"max_retries" json:"max_retries,omitempty"`
	Timeout    int `mapstructure:"timeout" json:"timeout,omitempty"`

	// Inference Timeout (seconds) - for GPU model validation
	// Default: 90s (allows for cold starts on large GPU models)
	// Set via config file key 'inference_timeout' or environment variable 'AI_AAS_CLI_INFERENCE_TIMEOUT'
	InferenceTimeout int `mapstructure:"inference_timeout" json:"inference_timeout,omitempty"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		APIEndpoint:      "http://localhost:8080",
		Environment:      "development",
		Verbose:          false,
		OutputFormat:     "table",
		InferenceTimeout: 90, // 90 seconds for GPU model cold starts
	}
}

// Load loads configuration from file and environment
func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("api_endpoint", "http://localhost:8080")
	viper.SetDefault("environment", "development")
	viper.SetDefault("verbose", false)
	viper.SetDefault("output_format", "table")
	viper.SetDefault("inference_timeout", 90)

	// Config file settings
	viper.SetConfigName(ConfigFileName)
	viper.SetConfigType(ConfigFileType)

	// Search paths
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(home)
	}
	viper.AddConfigPath(".")

	// Environment variables (AI_AAS_ prefix)
	viper.SetEnvPrefix("AI_AAS")
	viper.AutomaticEnv()

	// Read config file (ignore not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Environment variable overrides (highest precedence)
	// AI_AAS_ENVIRONMENT overrides config file environment
	if envOverride := os.Getenv("AI_AAS_ENVIRONMENT"); envOverride != "" {
		cfg.Environment = normalizeEnv(envOverride)
	}

	// AI_AAS_API_KEY overrides config file API key
	if keyOverride := os.Getenv("AI_AAS_API_KEY"); keyOverride != "" {
		cfg.APIKey = keyOverride
	}

	// AI_AAS_API_ENDPOINT overrides config file endpoint
	if endpointOverride := os.Getenv("AI_AAS_API_ENDPOINT"); endpointOverride != "" {
		cfg.APIEndpoint = endpointOverride
	}

	// AI_AAS_ADMIN_API_ENDPOINT overrides admin API endpoint (advanced override)
	if adminEndpointOverride := os.Getenv("AI_AAS_ADMIN_API_ENDPOINT"); adminEndpointOverride != "" {
		cfg.AdminAPIEndpoint = adminEndpointOverride
	}

	// AI_AAS_DEFAULT_ORG_ID overrides config file default org
	if orgOverride := os.Getenv("AI_AAS_DEFAULT_ORG_ID"); orgOverride != "" {
		cfg.DefaultOrgID = orgOverride
	}

	// AI_AAS_INFERENCE_TIMEOUT overrides config file inference timeout
	if timeoutOverride := os.Getenv("AI_AAS_INFERENCE_TIMEOUT"); timeoutOverride != "" {
		var timeout int
		if _, err := fmt.Sscanf(timeoutOverride, "%d", &timeout); err == nil && timeout > 0 {
			cfg.InferenceTimeout = timeout
		}
	}

	// Ensure inference timeout has a reasonable value
	if cfg.InferenceTimeout <= 0 {
		cfg.InferenceTimeout = 90
	}

	return &cfg, nil
}

// normalizeEnv normalizes environment names (shorthand support)
func normalizeEnv(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "dev":
		return "development"
	case "prod":
		return "production"
	case "stage":
		return "staging"
	default:
		return env
	}
}

// Save saves the configuration to file
func Save(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	configPath := filepath.Join(home, ConfigFileName+"."+ConfigFileType)

	// Set values in viper
	viper.Set("api_endpoint", cfg.APIEndpoint)
	viper.Set("api_key", cfg.APIKey)
	viper.Set("environment", cfg.Environment)
	viper.Set("verbose", cfg.Verbose)
	viper.Set("output_format", cfg.OutputFormat)
	if cfg.DefaultOrgID != "" {
		viper.Set("default_org_id", cfg.DefaultOrgID)
	}

	// Service-specific endpoints (optional, derived from base domain)
	if cfg.UserOrgEndpoint != "" {
		viper.Set("user_org_endpoint", cfg.UserOrgEndpoint)
	}
	if cfg.InferenceEndpoint != "" {
		viper.Set("inference_endpoint", cfg.InferenceEndpoint)
	}
	if cfg.AdminAPIEndpoint != "" {
		viper.Set("admin_api_endpoint", cfg.AdminAPIEndpoint)
	}

	// Store HF token locally as well (also synced to server via credentials command)
	if cfg.HFToken != "" {
		viper.Set("hf_token", cfg.HFToken)
	}

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ConfigFileName+"."+ConfigFileType), nil
}

// Exists checks if a config file exists
func Exists() bool {
	path, err := GetConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Get retrieves a config value by key
func Get(key string) interface{} {
	return viper.Get(key)
}

// GetString retrieves a string config value
func GetString(key string) string {
	return viper.GetString(key)
}

// Set sets a config value
func Set(key string, value interface{}) error {
	viper.Set(key, value)
	
	// Persist to file
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}
	
	return viper.WriteConfigAs(configPath)
}

// MaskSecret masks all but the last 4 characters of a secret
func MaskSecret(s string) string {
	if len(s) <= 4 {
		if len(s) == 0 {
			return "(not set)"
		}
		return "****"
	}
	return "****" + s[len(s)-4:]
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.APIEndpoint == "" {
		return fmt.Errorf("api_endpoint is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}

// IsConfigured returns true if essential config is set
func (c *Config) IsConfigured() bool {
	return c.APIEndpoint != "" && c.APIKey != ""
}

