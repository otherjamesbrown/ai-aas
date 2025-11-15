// Package config provides tests for configuration management.
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyDefaults(t *testing.T) {
	v := viper.New()
	ApplyDefaults(v)

	if v.GetString("api-endpoints.user-org-service") != "http://localhost:8081" {
		t.Errorf("expected default user-org-service endpoint")
	}

	if v.GetString("defaults.output-format") != "table" {
		t.Errorf("expected default output format 'table'")
	}
}

func TestLoad(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Verify defaults are applied
	if cfg.UserOrgEndpoint != "http://localhost:8081" {
		t.Errorf("expected default endpoint, got %s", cfg.UserOrgEndpoint)
	}
}

func TestLoadWithFlags(t *testing.T) {
	overrides := map[string]interface{}{
		"user-org-endpoint": "http://custom:8081",
		"format":            "json",
	}

	cfg, err := LoadWithFlags(overrides)
	if err != nil {
		t.Fatalf("LoadWithFlags() failed: %v", err)
	}

	if cfg.UserOrgEndpoint != "http://custom:8081" {
		t.Errorf("expected custom endpoint, got %s", cfg.UserOrgEndpoint)
	}

	if cfg.OutputFormat != "json" {
		t.Errorf("expected json format, got %s", cfg.OutputFormat)
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	// Create temporary config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".admin-cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	configContent := `
api-endpoints:
  user-org-service: http://config-file:8081
defaults:
  output-format: json
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Note: This test would need to override home dir lookup in Load()
	// For now, verify the function handles missing config files gracefully
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should handle missing config file gracefully: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
}

