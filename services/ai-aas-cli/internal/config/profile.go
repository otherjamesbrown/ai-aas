// Package config provides configuration management for the CLI.
package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Profile represents a saved identity configuration (org + user + credentials + endpoints).
// Profiles bundle everything needed to connect to a specific environment.
// This follows the AWS CLI pattern where each profile is self-contained.
type Profile struct {
	// Environment label (development, staging, production) - for display/organization
	Environment string `yaml:"environment" json:"environment"`

	// Organization ID or slug
	OrgID string `yaml:"org_id,omitempty" json:"org_id,omitempty"`

	// User ID (UUID)
	UserID string `yaml:"user_id,omitempty" json:"user_id,omitempty"`

	// User email (for display/convenience)
	Email string `yaml:"email,omitempty" json:"email,omitempty"`

	// API key token (the secret)
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"`

	// Endpoints - when set, these override the global defaults
	AdminAPIEndpoint  string `yaml:"admin_api_endpoint,omitempty" json:"admin_api_endpoint,omitempty"`
	InferenceEndpoint string `yaml:"inference_endpoint,omitempty" json:"inference_endpoint,omitempty"`
	UserOrgEndpoint   string `yaml:"user_org_endpoint,omitempty" json:"user_org_endpoint,omitempty"`

	// Kubeconfig path for kubectl operations
	Kubeconfig string `yaml:"kubeconfig,omitempty" json:"kubeconfig,omitempty"`
}

// ProfileConfig holds all profiles and the active profile name.
// This is stored separately from the main config for clarity.
type ProfileConfig struct {
	ActiveProfile string              `yaml:"active_profile,omitempty"`
	Profiles      map[string]*Profile `yaml:"profiles,omitempty"`
}

// IsComplete returns true if the profile has all required fields set.
func (p *Profile) IsComplete() bool {
	return p.Environment != "" && p.OrgID != "" && p.UserID != "" && p.APIKey != ""
}

// Status returns a human-readable status of what's set/missing.
func (p *Profile) Status() string {
	if p.IsComplete() {
		return "complete"
	}
	missing := []string{}
	if p.Environment == "" {
		missing = append(missing, "environment")
	}
	if p.OrgID == "" {
		missing = append(missing, "org_id")
	}
	if p.UserID == "" {
		missing = append(missing, "user_id")
	}
	if p.APIKey == "" {
		missing = append(missing, "api_key")
	}
	return fmt.Sprintf("incomplete (missing: %v)", missing)
}

// LoadProfiles loads the profile configuration from the config file.
func LoadProfiles() (*ProfileConfig, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &ProfileConfig{
				Profiles: make(map[string]*Profile),
			}, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg ProfileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]*Profile)
	}

	return &cfg, nil
}

// SaveProfiles saves the profile configuration to the config file.
// It preserves other config values that aren't profile-related.
func SaveProfiles(pc *ProfileConfig) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Read existing config to preserve non-profile fields
	existing := make(map[string]interface{})
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}

	// Update profile-related fields
	if pc.ActiveProfile != "" {
		existing["active_profile"] = pc.ActiveProfile
	} else {
		delete(existing, "active_profile")
	}

	if len(pc.Profiles) > 0 {
		existing["profiles"] = pc.Profiles
	} else {
		delete(existing, "profiles")
	}

	// Write back
	output, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// GetProfile returns a profile by name.
func GetProfile(name string) (*Profile, error) {
	pc, err := LoadProfiles()
	if err != nil {
		return nil, err
	}

	profile, exists := pc.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return profile, nil
}

// GetActiveProfile returns the currently active profile.
func GetActiveProfile() (*Profile, string, error) {
	pc, err := LoadProfiles()
	if err != nil {
		return nil, "", err
	}

	if pc.ActiveProfile == "" {
		return nil, "", nil // No active profile set
	}

	profile, exists := pc.Profiles[pc.ActiveProfile]
	if !exists {
		return nil, pc.ActiveProfile, fmt.Errorf("active profile '%s' not found", pc.ActiveProfile)
	}

	return profile, pc.ActiveProfile, nil
}

// SetActiveProfile sets the active profile by name.
func SetActiveProfile(name string) error {
	pc, err := LoadProfiles()
	if err != nil {
		return err
	}

	if _, exists := pc.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	pc.ActiveProfile = name
	return SaveProfiles(pc)
}

// CreateProfile creates a new empty profile.
func CreateProfile(name string, environment string) error {
	pc, err := LoadProfiles()
	if err != nil {
		return err
	}

	if _, exists := pc.Profiles[name]; exists {
		return fmt.Errorf("profile '%s' already exists", name)
	}

	pc.Profiles[name] = &Profile{
		Environment: normalizeEnv(environment),
	}

	return SaveProfiles(pc)
}

// UpdateProfile updates a profile field and saves.
func UpdateProfile(name string, updates func(*Profile)) error {
	pc, err := LoadProfiles()
	if err != nil {
		return err
	}

	profile, exists := pc.Profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	updates(profile)
	return SaveProfiles(pc)
}

// DeleteProfile removes a profile.
func DeleteProfile(name string) error {
	pc, err := LoadProfiles()
	if err != nil {
		return err
	}

	if _, exists := pc.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(pc.Profiles, name)

	// Clear active profile if it was the deleted one
	if pc.ActiveProfile == name {
		pc.ActiveProfile = ""
	}

	return SaveProfiles(pc)
}

// ListProfiles returns all profile names.
func ListProfiles() ([]string, string, error) {
	pc, err := LoadProfiles()
	if err != nil {
		return nil, "", err
	}

	names := make([]string, 0, len(pc.Profiles))
	for name := range pc.Profiles {
		names = append(names, name)
	}

	return names, pc.ActiveProfile, nil
}

// GetEffectiveConfig returns a Config populated from the specified profile
// (or active profile if name is empty), merged with environment defaults.
func GetEffectiveConfig(profileName string) (*Config, *Profile, error) {
	var profile *Profile
	var err error

	if profileName != "" {
		profile, err = GetProfile(profileName)
		if err != nil {
			return nil, nil, err
		}
	} else {
		profile, profileName, err = GetActiveProfile()
		if err != nil {
			return nil, nil, err
		}
	}

	// Load base config
	cfg, err := Load()
	if err != nil {
		return nil, nil, err
	}

	// Override with profile values if profile exists
	// Profile values take precedence over global config (AWS CLI pattern)
	if profile != nil {
		if profile.Environment != "" {
			cfg.Environment = profile.Environment
		}
		if profile.OrgID != "" {
			cfg.DefaultOrgID = profile.OrgID
		}
		if profile.APIKey != "" {
			cfg.APIKey = profile.APIKey
		}

		// Endpoint overrides from profile
		if profile.AdminAPIEndpoint != "" {
			cfg.AdminAPIEndpoint = profile.AdminAPIEndpoint
		}
		if profile.InferenceEndpoint != "" {
			cfg.InferenceEndpoint = profile.InferenceEndpoint
		}
		if profile.UserOrgEndpoint != "" {
			cfg.UserOrgEndpoint = profile.UserOrgEndpoint
		}

		// Kubeconfig override from profile - set directly in viper for GetKubeconfig() to pick up
		if profile.Kubeconfig != "" {
			viper.Set(fmt.Sprintf("environments.%s.kubeconfig", profile.Environment), profile.Kubeconfig)
		}

		// Fallback: Get endpoints from environments config if not set in profile
		envEndpoints := viper.GetStringMapString(fmt.Sprintf("environments.%s", profile.Environment))
		if cfg.UserOrgEndpoint == "" && envEndpoints["user_org_endpoint"] != "" {
			cfg.UserOrgEndpoint = envEndpoints["user_org_endpoint"]
		}
		if cfg.AdminAPIEndpoint == "" && envEndpoints["admin_api_endpoint"] != "" {
			cfg.AdminAPIEndpoint = envEndpoints["admin_api_endpoint"]
		}
		if cfg.InferenceEndpoint == "" && envEndpoints["inference_endpoint"] != "" {
			cfg.InferenceEndpoint = envEndpoints["inference_endpoint"]
		}
	}

	return cfg, profile, nil
}
