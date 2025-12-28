package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigFile(t *testing.T) {
	path := DefaultConfigFile()
	if path == "" {
		t.Error("DefaultConfigFile() returned empty string")
	}

	// Should be in home directory
	home, err := os.UserHomeDir()
	if err == nil {
		expected := filepath.Join(home, ".ai-aas-org.yaml")
		if path != expected {
			t.Errorf("DefaultConfigFile() = %q, want %q", path, expected)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				APIEndpoint: "https://api.example.com",
				APIKey:      "key_abc123",
				OrgID:       "org_123",
			},
			wantErr: false,
		},
		{
			name: "missing api_endpoint",
			config: Config{
				APIKey: "key_abc123",
				OrgID:  "org_123",
			},
			wantErr: true,
		},
		{
			name: "missing api_key",
			config: Config{
				APIEndpoint: "https://api.example.com",
				OrgID:       "org_123",
			},
			wantErr: true,
		},
		{
			name: "missing org_id",
			config: Config{
				APIEndpoint: "https://api.example.com",
				APIKey:      "key_abc123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, ".ai-aas-org.yaml")

	// Write test config to temp file
	testConfig := `api_endpoint: https://api.test.com
api_key: test_key_123
org_id: test_org
org_name: Test Organization
admin_email: admin@test.com
admin_user_id: usr_admin
`
	if err := os.WriteFile(tmpFile, []byte(testConfig), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config using LoadFrom
	cfg, err := LoadFrom(tmpFile)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	// Verify loaded values
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"APIEndpoint", cfg.APIEndpoint, "https://api.test.com"},
		{"APIKey", cfg.APIKey, "test_key_123"},
		{"OrgID", cfg.OrgID, "test_org"},
		{"OrgName", cfg.OrgName, "Test Organization"},
		{"AdminEmail", cfg.AdminEmail, "admin@test.com"},
		{"AdminUserID", cfg.AdminUserID, "usr_admin"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestLoadFrom_FileNotFound(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/.ai-aas-org.yaml")
	if err == nil {
		t.Error("LoadFrom() expected error for non-existent file, got nil")
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	// Create a temp file with invalid YAML
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, ".ai-aas-org.yaml")

	invalidYAML := `[[[invalid yaml content`
	if err := os.WriteFile(tmpFile, []byte(invalidYAML), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadFrom(tmpFile)
	if err == nil {
		t.Error("LoadFrom() expected error for invalid YAML, got nil")
	}
}
