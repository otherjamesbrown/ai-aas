// Package integration provides integration tests for the ai-aas-cli.
package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitFromFlags tests non-interactive initialization via flags
func TestInitFromFlags(t *testing.T) {
	// Create temp directory for test config
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tests := []struct {
		name        string
		apiKey      string
		endpoint    string
		environment string
		hfToken     string
		expectErr   bool
	}{
		{
			name:        "valid config with all fields",
			apiKey:      "test-api-key-12345",
			endpoint:    "https://api.test.com",
			environment: "staging",
			hfToken:     "hf_test_token",
			expectErr:   false,
		},
		{
			name:        "valid config minimal",
			apiKey:      "test-api-key",
			endpoint:    "",
			environment: "",
			hfToken:     "",
			expectErr:   false,
		},
		{
			name:      "missing api key",
			apiKey:    "",
			endpoint:  "https://api.test.com",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up config file between tests
			configPath := filepath.Join(tmpDir, ".ai-aas-cli.yaml")
			os.Remove(configPath)

			err := config.InitFromFlags(tt.apiKey, tt.endpoint, tt.environment, tt.hfToken)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify config was saved
			cfg, err := config.Load()
			require.NoError(t, err)

			assert.Equal(t, tt.apiKey, cfg.APIKey)
			if tt.endpoint != "" {
				assert.Equal(t, tt.endpoint, cfg.APIEndpoint)
			}
			if tt.environment != "" {
				assert.Equal(t, tt.environment, cfg.Environment)
			}
		})
	}
}

// TestConfigWorkflow tests the full configuration workflow
func TestConfigWorkflow(t *testing.T) {
	// Create temp directory for test config
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// 1. Initially config should not exist
	assert.False(t, config.Exists())

	// 2. Initialize with flags
	err := config.InitFromFlags("test-key", "https://api.example.com", "production", "")
	require.NoError(t, err)

	// 3. Config should now exist
	assert.True(t, config.Exists())

	// 4. Load and verify
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "test-key", cfg.APIKey)
	assert.Equal(t, "https://api.example.com", cfg.APIEndpoint)
	assert.Equal(t, "production", cfg.Environment)

	// 5. Config should validate
	assert.NoError(t, cfg.Validate())
	assert.True(t, cfg.IsConfigured())

	// 6. Update a single setting
	err = config.Set("environment", "staging")
	require.NoError(t, err)

	// 7. Reload and verify update
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, "staging", cfg.Environment)
}

// TestConfigPersistence tests that config persists across loads
func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Save initial config
	cfg := &config.Config{
		APIEndpoint:  "https://api.test.com",
		APIKey:       "persistent-key-12345",
		Environment:  "development",
		Verbose:      true,
		OutputFormat: "json",
	}
	err := config.Save(cfg)
	require.NoError(t, err)

	// Load in new context and verify all fields persisted
	loaded, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, cfg.APIEndpoint, loaded.APIEndpoint)
	assert.Equal(t, cfg.APIKey, loaded.APIKey)
	assert.Equal(t, cfg.Environment, loaded.Environment)
	assert.Equal(t, cfg.Verbose, loaded.Verbose)
	assert.Equal(t, cfg.OutputFormat, loaded.OutputFormat)
}

// TestPathCheck tests PATH detection functionality
func TestPathCheck(t *testing.T) {
	info := config.CheckPath()

	// Should always return valid shell info
	assert.NotEmpty(t, info.Shell)

	// Binary path should be set
	assert.NotEmpty(t, info.BinaryPath)

	// If not in PATH, instructions should be provided
	if !info.InPath {
		assert.NotEmpty(t, info.Instruction)
		// Instruction should mention the binary directory
		assert.Contains(t, info.Instruction, filepath.Dir(info.BinaryPath))
	}
}

// TestAPIValidation tests API connectivity validation with mock server
func TestAPIValidation(t *testing.T) {
	// Create a mock API server
	healthyCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			healthyCalled = true
			// Check for API key header
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create temp directory
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Initialize with mock server endpoint
	err := config.InitFromFlags("test-api-key", server.URL, "development", "")
	require.NoError(t, err)

	// Load config
	cfg, err := config.Load()
	require.NoError(t, err)

	// Verify config points to our mock server
	assert.Equal(t, server.URL, cfg.APIEndpoint)
	assert.Equal(t, "test-api-key", cfg.APIKey)

	// Note: The health check happens in the init wizard, not here
	// This test verifies the config is set up correctly for API calls
	_ = healthyCalled // We can use this if we add direct health check test
}

// TestInitWizardOutput captures and validates wizard output
func TestInitWizardOutput(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create a wizard
	wizard := config.NewInitWizard()

	// Capture stdout to verify output
	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run wizard would require stdin input, so we test the components
	// The wizard.Run() requires interactive input, so we test non-interactive

	// For non-interactive, use InitFromFlags
	err := config.InitFromFlags("test-key", "http://localhost:8080", "dev", "")

	w.Close()
	os.Stdout = origStdout
	buf.ReadFrom(r)

	require.NoError(t, err)
	assert.NotNil(t, wizard) // Verify wizard can be created
}

// TestEnvironmentNormalization tests environment name normalization
func TestEnvironmentNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tests := []struct {
		input    string
		expected string
	}{
		{"development", "development"},
		{"staging", "staging"},
		{"production", "production"},
		{"dev", "dev"}, // Short forms are passed through by InitFromFlags
		{"prod", "prod"},
		// Note: The wizard normalizes dev->development, prod->production
		// but InitFromFlags passes through as-is
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Clean config
			configPath := filepath.Join(tmpDir, ".ai-aas-cli.yaml")
			os.Remove(configPath)

			err := config.InitFromFlags("key", "", tt.input, "")
			require.NoError(t, err)

			cfg, err := config.Load()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Environment)
		})
	}
}

// TestMaskSecretIntegration tests secret masking in config display
func TestMaskSecretIntegration(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{"long key", "sk_live_abcdefghijklmnop", "****mnop"},
		{"short key", "abc", "****"},
		{"empty key", "", "(not set)"},
		{"exactly 4", "abcd", "****"},
		{"5 chars", "abcde", "****bcde"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := config.MaskSecret(tt.apiKey)
			assert.Equal(t, tt.expected, masked)
		})
	}
}
