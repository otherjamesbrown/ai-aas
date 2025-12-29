package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "http://localhost:8080", cfg.APIEndpoint)
	assert.Equal(t, "development", cfg.Environment)
	assert.False(t, cfg.Verbose)
	assert.Equal(t, "table", cfg.OutputFormat)
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "(not set)",
		},
		{
			name:     "short secret",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "exactly 4 chars",
			input:    "abcd",
			expected: "****",
		},
		{
			name:     "longer secret",
			input:    "super-secret-key-12345",
			expected: "****2345",
		},
		{
			name:     "api key format",
			input:    "sk_live_abcdefghijklmnop",
			expected: "****mnop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSecret(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				APIEndpoint: "http://localhost:8080",
				APIKey:      "test-key",
			},
			expectErr: false,
		},
		{
			name: "missing endpoint",
			config: &Config{
				APIKey: "test-key",
			},
			expectErr: true,
		},
		{
			name: "missing api key",
			config: &Config{
				APIEndpoint: "http://localhost:8080",
			},
			expectErr: true,
		},
		{
			name:      "empty config",
			config:    &Config{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "fully configured",
			config: &Config{
				APIEndpoint: "http://localhost:8080",
				APIKey:      "test-key",
			},
			expected: true,
		},
		{
			name: "missing endpoint",
			config: &Config{
				APIKey: "test-key",
			},
			expected: false,
		},
		{
			name: "missing api key",
			config: &Config{
				APIEndpoint: "http://localhost:8080",
			},
			expected: false,
		},
		{
			name:     "empty config",
			config:   &Config{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsConfigured()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory for test config
	tmpDir := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create test config
	cfg := &Config{
		APIEndpoint:  "https://api.test.com",
		APIKey:       "test-api-key-12345",
		Environment:  "staging",
		Verbose:      true,
		OutputFormat: "json",
	}

	// Save config
	err := Save(cfg)
	require.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(tmpDir, ConfigFileName+"."+ConfigFileType)
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load config
	loaded, err := Load()
	require.NoError(t, err)

	// Verify loaded values
	assert.Equal(t, cfg.APIEndpoint, loaded.APIEndpoint)
	assert.Equal(t, cfg.APIKey, loaded.APIKey)
	assert.Equal(t, cfg.Environment, loaded.Environment)
	assert.Equal(t, cfg.Verbose, loaded.Verbose)
	assert.Equal(t, cfg.OutputFormat, loaded.OutputFormat)
}

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	require.NoError(t, err)

	// Should end with config file name
	assert.Contains(t, path, ConfigFileName)
	assert.Contains(t, path, ConfigFileType)
}

func TestExists(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Override home for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Should not exist initially
	assert.False(t, Exists())

	// Create config file
	configPath := filepath.Join(tmpDir, ConfigFileName+"."+ConfigFileType)
	err := os.WriteFile(configPath, []byte("api_endpoint: test"), 0644)
	require.NoError(t, err)

	// Should exist now
	assert.True(t, Exists())
}
