// Package config provides configuration management for the CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
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
	APIEndpoint  string `mapstructure:"api_endpoint" json:"api_endpoint"`
	APIKey       string `mapstructure:"api_key" json:"api_key"`
	Environment  string `mapstructure:"environment" json:"environment"`
	HFToken      string `mapstructure:"hf_token" json:"hf_token,omitempty"`
	Verbose      bool   `mapstructure:"verbose" json:"verbose"`
	Quiet        bool   `mapstructure:"quiet" json:"quiet"`
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
	AdminAPIEndpoint string `mapstructure:"admin_api_endpoint" json:"admin_api_endpoint,omitempty"`

	// TLS Configuration
	CACertFile  string `mapstructure:"ca_cert_file" json:"ca_cert_file,omitempty"`
	TLSInsecure bool   `mapstructure:"tls_insecure" json:"tls_insecure,omitempty"`

	// Retry Settings
	MaxRetries int `mapstructure:"max_retries" json:"max_retries,omitempty"`
	Timeout    int `mapstructure:"timeout" json:"timeout,omitempty"`

	// GitOps Configuration
	// Path to the ai-aas-config repository for GitOps-based deployments
	// Set via config file key 'config_repo_path' or environment variable 'AI_AAS_CONFIG_REPO_PATH'
	ConfigRepoPath string `mapstructure:"config_repo_path" json:"config_repo_path,omitempty"`

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
		InferenceTimeout: 90,            // 90 seconds for GPU model cold starts
		ConfigRepoPath:   "~/ai-aas-config", // Default GitOps config repo path
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
	viper.SetDefault("config_repo_path", "~/ai-aas-config")

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

	// AI_AAS_HF_TOKEN overrides config file HuggingFace token
	if hfTokenOverride := os.Getenv("AI_AAS_HF_TOKEN"); hfTokenOverride != "" {
		cfg.HFToken = hfTokenOverride
	}

	// AI_AAS_CONFIG_REPO_PATH overrides config file GitOps config repo path
	if configRepoOverride := os.Getenv("AI_AAS_CONFIG_REPO_PATH"); configRepoOverride != "" {
		cfg.ConfigRepoPath = configRepoOverride
	}

	// Ensure inference timeout has a reasonable value
	if cfg.InferenceTimeout <= 0 {
		cfg.InferenceTimeout = 90
	}

	// Warn about secrets in config file (deprecation notice)
	// Skip warning in JSON mode to avoid polluting machine-readable output
	warnSecretsInConfigFile(&cfg)

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

// warnSecretsInConfigFile checks for secrets in the config file and warns the user.
// This is a deprecation notice to encourage migration to environment variables.
// The warning is suppressed in JSON output mode to avoid polluting machine-readable output.
func warnSecretsInConfigFile(cfg *Config) {
	// Skip warning in JSON mode to avoid polluting parseable output
	// Check both config file setting and environment variable (which has higher precedence)
	outputFormat := cfg.OutputFormat
	if envFormat := os.Getenv("AI_AAS_OUTPUT_FORMAT"); envFormat != "" {
		outputFormat = envFormat
	}

	// Also check common command-line flag patterns
	for _, arg := range os.Args {
		if arg == "--format=json" || arg == "-f=json" {
			outputFormat = "json"
			break
		}
		// Check for --format json (two separate args)
		if arg == "--format" || arg == "-f" {
			for i, a := range os.Args {
				if a == arg && i+1 < len(os.Args) && os.Args[i+1] == "json" {
					outputFormat = "json"
					break
				}
			}
		}
	}

	if outputFormat == "json" {
		return
	}

	configPath, err := GetConfigPath()
	if err != nil {
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return // Config file doesn't exist or can't be read
	}

	content := string(data)
	var warnings []string

	// Check for secrets in config file (not from env vars)
	if strings.Contains(content, "api_key:") && os.Getenv("AI_AAS_API_KEY") == "" {
		warnings = append(warnings, "api_key (use AI_AAS_API_KEY environment variable)")
	}
	if strings.Contains(content, "hf_token:") && os.Getenv("AI_AAS_HF_TOKEN") == "" {
		warnings = append(warnings, "hf_token (use AI_AAS_HF_TOKEN environment variable)")
	}
	if strings.Contains(content, "database_url:") {
		warnings = append(warnings, "database_url (no longer needed, remove from config)")
	}

	// Check for S3 credentials
	if (strings.Contains(content, "access_key:") || strings.Contains(content, "secret_key:")) &&
		os.Getenv("AWS_ACCESS_KEY_ID") == "" && os.Getenv("LINODE_OBJECT_STORAGE_ACCESS_KEY") == "" {
		warnings = append(warnings, "s3 credentials (use AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY)")
	}

	if len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "\nWARNING: Secrets found in config file %s\n", configPath)
		fmt.Fprintf(os.Stderr, "  For security, please migrate to environment variables:\n")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "    - %s\n", w)
		}
		fmt.Fprintf(os.Stderr, "\n  Add to your shell profile (~/.bashrc or ~/.zshrc):\n")
		fmt.Fprintf(os.Stderr, "    export AI_AAS_API_KEY=\"your-api-key\"\n")
		fmt.Fprintf(os.Stderr, "    export AI_AAS_HF_TOKEN=\"your-hf-token\"\n\n")
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.APIEndpoint == "" {
		return fmt.Errorf("api_endpoint is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key is required. Set AI_AAS_API_KEY environment variable")
	}
	return nil
}

// IsConfigured returns true if essential config is set
func (c *Config) IsConfigured() bool {
	return c.APIEndpoint != "" && c.APIKey != ""
}

// GetAdminEndpoint returns the Admin API endpoint to use.
// If AdminAPIEndpoint is set (legacy), it takes precedence.
// Otherwise, falls back to APIEndpoint (recommended).
func (c *Config) GetAdminEndpoint() string {
	if c.AdminAPIEndpoint != "" {
		return c.AdminAPIEndpoint
	}
	return c.APIEndpoint
}

// NewAPIClient creates a new API client with appropriate options based on config.
// This centralizes client creation logic to ensure consistent TLS and timeout handling.
func (c *Config) NewAPIClient(endpoint string) *api.Client {
	opts := []api.ClientOption{}

	if c.TLSInsecure {
		opts = append(opts, api.WithInsecureSkipVerify())
	}

	if c.Timeout > 0 {
		opts = append(opts, api.WithTimeout(time.Duration(c.Timeout)*time.Second))
	}

	return api.NewClient(endpoint, c.APIKey, opts...)
}
