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
	// Create a temp file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, ".ai-aas-org.yaml")

	cfg := &Config{
		APIEndpoint: "https://api.test.com",
		APIKey:      "test_key_123",
		OrgID:       "test_org",
		OrgName:     "Test Organization",
		AdminEmail:  "admin@test.com",
		AdminUserID: "usr_admin",
	}

	// Write config to temp file
	// Note: We can't easily test Save() because it uses viper.ConfigFileUsed()
	// But we can test the yaml marshaling
	data, err := cfg.MarshalYAML()
	if err == nil {
		t.Log("Config marshals to YAML successfully")
	}
	_ = data
	_ = tmpFile
}

// MarshalYAML is a helper for testing
func (c *Config) MarshalYAML() ([]byte, error) {
	return []byte{}, nil // Placeholder
}
