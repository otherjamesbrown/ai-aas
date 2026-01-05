package testconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	// Set up environment variables
	os.Setenv(EnvMasterAdminAPIKey, "test-api-key")
	os.Setenv(EnvMasterAdminOrgID, "test-org-id")
	defer func() {
		os.Unsetenv(EnvMasterAdminAPIKey)
		os.Unsetenv(EnvMasterAdminOrgID)
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.MasterAdminAPIKey != "test-api-key" {
		t.Errorf("MasterAdminAPIKey = %q, want %q", cfg.MasterAdminAPIKey, "test-api-key")
	}
	if cfg.MasterAdminOrgID != "test-org-id" {
		t.Errorf("MasterAdminOrgID = %q, want %q", cfg.MasterAdminOrgID, "test-org-id")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Clear environment variables
	os.Unsetenv(EnvMasterAdminAPIKey)
	os.Unsetenv(EnvMasterAdminOrgID)
	os.Unsetenv(EnvStagingMasterAdminAPIKey)

	// Create temporary .env file
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `# Test env file
DEVELOP_MASTER_ADMIN_API_KEY=file-api-key
MASTER_ADMIN_ORG_ID=file-org-id
STAGING_MASTER_ADMIN_API_KEY=staging-api-key
HF_TOKEN=hf_test_token
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	cfg, err := LoadWithEnvPath(envPath)
	if err != nil {
		t.Fatalf("LoadWithEnvPath() failed: %v", err)
	}

	if cfg.MasterAdminAPIKey != "file-api-key" {
		t.Errorf("MasterAdminAPIKey = %q, want %q", cfg.MasterAdminAPIKey, "file-api-key")
	}
	if cfg.MasterAdminOrgID != "file-org-id" {
		t.Errorf("MasterAdminOrgID = %q, want %q", cfg.MasterAdminOrgID, "file-org-id")
	}
	if cfg.StagingMasterAdminAPIKey != "staging-api-key" {
		t.Errorf("StagingMasterAdminAPIKey = %q, want %q", cfg.StagingMasterAdminAPIKey, "staging-api-key")
	}
	if cfg.HFToken != "hf_test_token" {
		t.Errorf("HFToken = %q, want %q", cfg.HFToken, "hf_test_token")
	}
}

func TestEnvPrecedenceOverFile(t *testing.T) {
	// Set environment variable
	os.Setenv(EnvMasterAdminAPIKey, "env-api-key")
	defer os.Unsetenv(EnvMasterAdminAPIKey)

	// Create temporary .env file with different value
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `DEVELOP_MASTER_ADMIN_API_KEY=file-api-key`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	cfg, err := LoadWithEnvPath(envPath)
	if err != nil {
		t.Fatalf("LoadWithEnvPath() failed: %v", err)
	}

	// Environment variable should take precedence
	if cfg.MasterAdminAPIKey != "env-api-key" {
		t.Errorf("MasterAdminAPIKey = %q, want %q (env should take precedence)", cfg.MasterAdminAPIKey, "env-api-key")
	}
}

func TestLoadNoConfig(t *testing.T) {
	// Clear environment variables
	os.Unsetenv(EnvMasterAdminAPIKey)

	// Try to load from non-existent path
	_, err := LoadWithEnvPath("/non/existent/path/.env")
	if err != ErrNoConfig {
		t.Errorf("LoadWithEnvPath() error = %v, want %v", err, ErrNoConfig)
	}
}

func TestValidate(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should fail when MasterAdminAPIKey is empty")
	}

	cfg.MasterAdminAPIKey = "test-key"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should pass when MasterAdminAPIKey is set: %v", err)
	}
}

func TestForEnvironment(t *testing.T) {
	cfg := &Config{
		MasterAdminAPIKey:        "dev-key",
		MasterAdminOrgID:         "dev-org",
		StagingMasterAdminAPIKey: "staging-key",
		StagingMasterAdminOrgID:  "staging-org",
	}

	tests := []struct {
		env        string
		wantAPIKey string
		wantOrgID  string
	}{
		{"development", "dev-key", "dev-org"},
		{"Development", "dev-key", "dev-org"},
		{"staging", "staging-key", "staging-org"},
		{"Staging", "staging-key", "staging-org"},
		{"", "dev-key", "dev-org"}, // default to development
	}

	for _, tt := range tests {
		apiKey, orgID := cfg.ForEnvironment(tt.env)
		if apiKey != tt.wantAPIKey {
			t.Errorf("ForEnvironment(%q) apiKey = %q, want %q", tt.env, apiKey, tt.wantAPIKey)
		}
		if orgID != tt.wantOrgID {
			t.Errorf("ForEnvironment(%q) orgID = %q, want %q", tt.env, orgID, tt.wantOrgID)
		}
	}
}

func TestSetEnvFromConfig(t *testing.T) {
	// Clear environment first
	envVars := []string{
		EnvMasterAdminAPIKey,
		EnvMasterAdminOrgID,
		EnvStagingMasterAdminAPIKey,
		EnvStagingMasterAdminOrgID,
		EnvHFToken,
		EnvLinodeObjectStorageAccessKey,
		EnvLinodeObjectStorageSecretKey,
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
	defer func() {
		for _, v := range envVars {
			os.Unsetenv(v)
		}
	}()

	cfg := &Config{
		MasterAdminAPIKey:            "exported-key",
		MasterAdminOrgID:             "exported-org",
		StagingMasterAdminAPIKey:     "staging-key",
		StagingMasterAdminOrgID:      "staging-org",
		HFToken:                      "hf-token",
		LinodeObjectStorageAccessKey: "linode-access",
		LinodeObjectStorageSecretKey: "linode-secret",
	}

	if err := cfg.SetEnvFromConfig(); err != nil {
		t.Fatalf("SetEnvFromConfig() failed: %v", err)
	}

	// Verify all fields are exported
	tests := map[string]string{
		EnvMasterAdminAPIKey:            "exported-key",
		EnvMasterAdminOrgID:             "exported-org",
		EnvStagingMasterAdminAPIKey:     "staging-key",
		EnvStagingMasterAdminOrgID:      "staging-org",
		EnvHFToken:                      "hf-token",
		EnvLinodeObjectStorageAccessKey: "linode-access",
		EnvLinodeObjectStorageSecretKey: "linode-secret",
	}
	for envVar, want := range tests {
		if got := os.Getenv(envVar); got != want {
			t.Errorf("%s = %q, want %q", envVar, got, want)
		}
	}
}

func TestParseQuotedValues(t *testing.T) {
	// Clear environment variables
	os.Unsetenv(EnvMasterAdminAPIKey)

	// Create temporary .env file with quoted values
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `DEVELOP_MASTER_ADMIN_API_KEY="quoted-key"
MASTER_ADMIN_ORG_ID='single-quoted'
HF_TOKEN=unquoted
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test .env file: %v", err)
	}

	cfg, err := LoadWithEnvPath(envPath)
	if err != nil {
		t.Fatalf("LoadWithEnvPath() failed: %v", err)
	}

	if cfg.MasterAdminAPIKey != "quoted-key" {
		t.Errorf("MasterAdminAPIKey = %q, want %q (quotes should be stripped)", cfg.MasterAdminAPIKey, "quoted-key")
	}
	if cfg.MasterAdminOrgID != "single-quoted" {
		t.Errorf("MasterAdminOrgID = %q, want %q (quotes should be stripped)", cfg.MasterAdminOrgID, "single-quoted")
	}
	if cfg.HFToken != "unquoted" {
		t.Errorf("HFToken = %q, want %q", cfg.HFToken, "unquoted")
	}
}
