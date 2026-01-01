// Package testconfig provides configuration loading for acceptance tests.
// It loads test credentials from a single source of truth: secrets/env/.env
//
// Priority:
//  1. Environment variables (for CI)
//  2. secrets/env/.env file (for local development)
//
// Usage:
//
//	cfg, err := testconfig.Load()
//	if err != nil {
//	    // Handle missing configuration
//	}
//	apiKey := cfg.MasterAdminAPIKey
package testconfig

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds test configuration loaded from environment or .env file.
type Config struct {
	// MasterAdminAPIKey is the development cluster master admin API key.
	MasterAdminAPIKey string

	// MasterAdminOrgID is the development cluster master admin organization ID.
	MasterAdminOrgID string

	// StagingMasterAdminAPIKey is the staging cluster master admin API key.
	StagingMasterAdminAPIKey string

	// StagingMasterAdminOrgID is the staging cluster master admin organization ID.
	StagingMasterAdminOrgID string

	// HFToken is the HuggingFace API token for model downloads.
	HFToken string

	// LinodeObjectStorageAccessKey is the Linode S3-compatible storage access key.
	LinodeObjectStorageAccessKey string

	// LinodeObjectStorageSecretKey is the Linode S3-compatible storage secret key.
	LinodeObjectStorageSecretKey string
}

// Environment variable names for test configuration.
const (
	EnvMasterAdminAPIKey            = "MASTER_ADMIN_API_KEY"
	EnvMasterAdminOrgID             = "MASTER_ADMIN_ORG_ID"
	EnvStagingMasterAdminAPIKey     = "STAGING_MASTER_ADMIN_API_KEY"
	EnvStagingMasterAdminOrgID      = "STAGING_MASTER_ADMIN_ORG_ID"
	EnvHFToken                      = "HF_TOKEN"
	EnvLinodeObjectStorageAccessKey = "LINODE_OBJECT_STORAGE_ACCESS_KEY"
	EnvLinodeObjectStorageSecretKey = "LINODE_OBJECT_STORAGE_SECRET_KEY"
)

// Default paths to look for the .env file.
var defaultEnvPaths = []string{
	"secrets/env/.env",
	"../../secrets/env/.env",
	"../../../secrets/env/.env",
	"../../../../secrets/env/.env",
}

// ErrNoConfig is returned when no configuration source is available.
var ErrNoConfig = errors.New("no configuration found: set environment variables or ensure secrets/env/.env exists")

// Load loads test configuration from environment variables or .env file.
// Environment variables take precedence over the .env file.
func Load() (*Config, error) {
	return LoadWithEnvPath("")
}

// LoadWithEnvPath loads test configuration, looking for .env at the specified path.
// If envPath is empty, it searches default paths relative to the working directory.
func LoadWithEnvPath(envPath string) (*Config, error) {
	cfg := &Config{}

	// First, try to load from environment variables (CI mode)
	if loadFromEnv(cfg) {
		return cfg, nil
	}

	// Fall back to .env file (local development mode)
	var paths []string
	if envPath != "" {
		paths = []string{envPath}
	} else {
		paths = defaultEnvPaths
	}

	envFile, err := findEnvFile(paths)
	if err != nil {
		return nil, ErrNoConfig
	}

	if err := loadFromFile(cfg, envFile); err != nil {
		return nil, fmt.Errorf("parse .env file: %w", err)
	}

	return cfg, nil
}

// loadFromEnv tries to load configuration from environment variables.
// Returns true if at least the master admin API key is set.
func loadFromEnv(cfg *Config) bool {
	cfg.MasterAdminAPIKey = os.Getenv(EnvMasterAdminAPIKey)
	cfg.MasterAdminOrgID = os.Getenv(EnvMasterAdminOrgID)
	cfg.StagingMasterAdminAPIKey = os.Getenv(EnvStagingMasterAdminAPIKey)
	cfg.StagingMasterAdminOrgID = os.Getenv(EnvStagingMasterAdminOrgID)
	cfg.HFToken = os.Getenv(EnvHFToken)
	cfg.LinodeObjectStorageAccessKey = os.Getenv(EnvLinodeObjectStorageAccessKey)
	cfg.LinodeObjectStorageSecretKey = os.Getenv(EnvLinodeObjectStorageSecretKey)

	// Consider env vars loaded if the master admin key is set
	return cfg.MasterAdminAPIKey != ""
}

// findEnvFile looks for a .env file at the given paths.
// Returns the first path that exists.
func findEnvFile(paths []string) (string, error) {
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath, nil
		}
	}
	return "", os.ErrNotExist
}

// loadFromFile parses a .env file and populates the config.
func loadFromFile(cfg *Config, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)

		switch key {
		case EnvMasterAdminAPIKey:
			cfg.MasterAdminAPIKey = value
		case EnvMasterAdminOrgID:
			cfg.MasterAdminOrgID = value
		case EnvStagingMasterAdminAPIKey:
			cfg.StagingMasterAdminAPIKey = value
		case EnvStagingMasterAdminOrgID:
			cfg.StagingMasterAdminOrgID = value
		case EnvHFToken:
			cfg.HFToken = value
		case EnvLinodeObjectStorageAccessKey:
			cfg.LinodeObjectStorageAccessKey = value
		case EnvLinodeObjectStorageSecretKey:
			cfg.LinodeObjectStorageSecretKey = value
		}
	}

	return scanner.Err()
}

// SetEnvFromConfig exports config values to environment variables.
// This is useful for passing credentials to CLI subprocesses.
func (c *Config) SetEnvFromConfig() error {
	vars := map[string]string{
		EnvMasterAdminAPIKey:            c.MasterAdminAPIKey,
		EnvMasterAdminOrgID:             c.MasterAdminOrgID,
		EnvStagingMasterAdminAPIKey:     c.StagingMasterAdminAPIKey,
		EnvStagingMasterAdminOrgID:      c.StagingMasterAdminOrgID,
		EnvHFToken:                      c.HFToken,
		EnvLinodeObjectStorageAccessKey: c.LinodeObjectStorageAccessKey,
		EnvLinodeObjectStorageSecretKey: c.LinodeObjectStorageSecretKey,
	}

	for key, value := range vars {
		if value != "" {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("failed to set env var %s: %w", key, err)
			}
		}
	}
	return nil
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.MasterAdminAPIKey == "" {
		return errors.New("MASTER_ADMIN_API_KEY is required")
	}
	return nil
}

// ForEnvironment returns the appropriate API key and org ID for the given environment.
// environment should be "development" or "staging".
func (c *Config) ForEnvironment(environment string) (apiKey, orgID string) {
	switch strings.ToLower(environment) {
	case "staging":
		return c.StagingMasterAdminAPIKey, c.StagingMasterAdminOrgID
	default:
		return c.MasterAdminAPIKey, c.MasterAdminOrgID
	}
}
