package usecases_test

import (
	"fmt"
	"os"

	"github.com/ai-aas/shared-go/testconfig"
	"gopkg.in/yaml.v3"
)

// init loads test configuration from secrets/env/.env if environment variables
// are not already set. This enables running tests without manual env setup.
func init() {
	// Skip if already configured via env vars
	if os.Getenv(envAPIEndpoint) != "" && os.Getenv(envAPIKey) != "" {
		return
	}

	// Try to load from secrets/env/.env
	cfg, err := testconfig.Load()
	if err != nil {
		// No config available - tests will skip via skipIfNoLiveAPI
		return
	}

	// Set environment variables for CLI subprocesses
	// UC tests use AI_AAS_API_KEY which maps to MASTER_ADMIN_API_KEY
	if cfg.MasterAdminAPIKey != "" && os.Getenv(envAPIKey) == "" {
		os.Setenv(envAPIKey, cfg.MasterAdminAPIKey)
	}
	// Also set org ID if available
	if cfg.MasterAdminOrgID != "" && os.Getenv(envOrgID) == "" {
		os.Setenv(envOrgID, cfg.MasterAdminOrgID)
	}
}

// TestConfig holds configuration for UC tests
type TestConfig struct {
	APIEndpoint string `yaml:"api_endpoint"`
	APIKey      string `yaml:"api_key"`
	OrgID       string `yaml:"org_id,omitempty"`
}

// LoadConfig loads test configuration from environment or optional config file
// Priority: Environment variables > secrets/env/.env > Config file > Error
func LoadConfig() (*TestConfig, error) {
	cfg := &TestConfig{}

	// First, try to load from environment variables
	if endpoint := os.Getenv(envAPIEndpoint); endpoint != "" {
		cfg.APIEndpoint = endpoint
		cfg.APIKey = os.Getenv(envAPIKey)
		cfg.OrgID = os.Getenv(envOrgID)
		return cfg, nil
	}

	// If env vars not set, try to load from config file
	configPath := os.Getenv("AI_AAS_TEST_CONFIG")
	if configPath == "" {
		configPath = ".ai-aas-test.yaml" // Default config file
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no configuration found: set AI_AAS_API_ENDPOINT and AI_AAS_API_KEY or create %s", configPath)
	}

	// Load from config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}

// Validate validates the test configuration
func (c *TestConfig) Validate() error {
	if c.APIEndpoint == "" {
		return fmt.Errorf("api_endpoint is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	return nil
}
