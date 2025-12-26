// Package config provides configuration management for the CLI.
package config

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// InitWizard runs the interactive initialization wizard
type InitWizard struct {
	reader  *bufio.Reader
	config  *Config
	verbose bool
}

// NewInitWizard creates a new initialization wizard
func NewInitWizard() *InitWizard {
	return &InitWizard{
		reader: bufio.NewReader(os.Stdin),
		config: DefaultConfig(),
	}
}

// hasExistingValue checks if a config value is set and not default
func hasExistingValue(value, defaultValue string) bool {
	return value != "" && value != defaultValue
}

// Run executes the initialization wizard
func (w *InitWizard) Run() error {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           AI-AAS CLI Initialization Wizard                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Check PATH status (skip in non-interactive mode)
	if isInteractive() {
		w.checkPath()
	}

	// Load existing config if present
	if Exists() {
		existingConfig, err := Load()
		if err != nil {
			fmt.Printf("⚠ Could not load existing config: %v\n", err)
			fmt.Println("  Starting with fresh configuration.")
		} else {
			fmt.Println("✓ Found existing configuration")
			fmt.Println("  For each setting, press Enter to keep existing value or type a new one.")
			w.config = existingConfig
		}
	}

	// Collect configuration (with option to keep existing values)
	if err := w.collectAPIEndpoint(); err != nil {
		return err
	}

	if err := w.collectAPIKey(); err != nil {
		return err
	}

	if err := w.collectEnvironment(); err != nil {
		return err
	}

	// Validate API connection
	fmt.Println()
	fmt.Println("Validating configuration...")
	if err := w.validateConnection(); err != nil {
		fmt.Printf("⚠ Warning: Could not connect to API: %v\n", err)
		proceed, _ := w.prompt("Continue anyway? (y/N): ")
		if !strings.HasPrefix(strings.ToLower(proceed), "y") {
			return fmt.Errorf("configuration validation failed")
		}
	} else {
		fmt.Println("✓ API connection successful")
	}

	// Auto-detect S3/Object Storage credentials from environment
	w.detectS3Credentials()

	// Save configuration
	if err := Save(w.config); err != nil {
		return fmt.Errorf("save configuration: %w", err)
	}

	configPath, _ := GetConfigPath()
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Configuration Complete!                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Configuration saved to: %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  ai-aas-cli config show     # View configuration")
	fmt.Println("  ai-aas-cli config test     # Test API connectivity")
	fmt.Println("  ai-aas-cli model list      # List registered models")
	fmt.Println()

	return nil
}

func (w *InitWizard) checkPath() {
	info := CheckPath()
	if !info.InPath {
		fmt.Println("⚠ ai-aas-cli is not in your PATH")
		fmt.Println()
		fmt.Printf("Detected shell: %s\n", info.Shell)
		fmt.Println()
		fmt.Println(info.Instruction)
		fmt.Println()
		fmt.Println("─────────────────────────────────────────────────────────────────")
		fmt.Println()
	}
}

func (w *InitWizard) collectAPIEndpoint() error {
	fmt.Println("Platform Domain or API Endpoint")
	fmt.Println("  You can provide either:")
	fmt.Println("    - Base domain:  dev.otherjamesbrown.com")
	fmt.Println("    - Full API URL: https://api.dev.otherjamesbrown.com")
	fmt.Println()
	fmt.Println("  If you provide a base domain, endpoints will be constructed as:")
	fmt.Println("    api.<domain>, user-org.<domain>, etc.")
	fmt.Println()

	// Show existing value if present
	existingEndpoint := w.config.APIEndpoint
	if hasExistingValue(existingEndpoint, "http://localhost:8080") {
		fmt.Printf("  Current: %s\n", existingEndpoint)
		fmt.Println("  Press Enter to keep, or type new value:")
		fmt.Println()
	}

	input, err := w.prompt("Domain or API Endpoint: ")
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)

	// If empty and we have existing value, keep it
	if input == "" {
		if hasExistingValue(existingEndpoint, "http://localhost:8080") {
			fmt.Printf("  ✓ Keeping existing endpoint: %s\n", existingEndpoint)
			fmt.Println()
			return nil
		}
		return fmt.Errorf("domain or API endpoint is required")
	}

	// Detect if it's a base domain or full URL
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		// Full URL provided
		w.config.APIEndpoint = input
		fmt.Printf("  Using API endpoint: %s\n", w.config.APIEndpoint)
	} else {
		// Base domain provided - construct endpoints
		baseDomain := strings.TrimPrefix(input, "api.")
		baseDomain = strings.TrimPrefix(baseDomain, "www.")
		baseDomain = strings.TrimPrefix(baseDomain, "admin-api.")

		// Construct all endpoints from base domain
		// APIEndpoint is the primary Admin API endpoint
		w.config.APIEndpoint = fmt.Sprintf("https://admin-api.%s", baseDomain)
		w.config.UserOrgEndpoint = fmt.Sprintf("https://user-org.%s", baseDomain)
		w.config.InferenceEndpoint = fmt.Sprintf("https://api.%s", baseDomain)
		// Don't set AdminAPIEndpoint - it should remain empty (only used for advanced overrides)

		fmt.Println()
		fmt.Println("  Constructed endpoints from base domain:")
		fmt.Printf("    Admin API Endpoint: %s\n", w.config.APIEndpoint)
		fmt.Printf("    User-Org Service:   %s\n", w.config.UserOrgEndpoint)
		fmt.Printf("    Inference API:      %s\n", w.config.InferenceEndpoint)
	}

	fmt.Println()
	return nil
}

func (w *InitWizard) collectAPIKey() error {
	fmt.Println("API Key")
	fmt.Println("  Your Admin API key for authentication")
	fmt.Println()

	// Check if API key is set via environment variable (preferred)
	envKey := os.Getenv("AI_AAS_API_KEY")
	if envKey != "" {
		fmt.Println("  ✓ Found AI_AAS_API_KEY in environment (recommended)")
		fmt.Printf("  Using: %s\n", MaskSecret(envKey))
		w.config.APIKey = envKey
		fmt.Println()
		return nil
	}

	// Show security warning about storing in config file
	fmt.Println("  RECOMMENDED: Set AI_AAS_API_KEY environment variable instead")
	fmt.Println("  Add to your shell profile (~/.bashrc or ~/.zshrc):")
	fmt.Println("    export AI_AAS_API_KEY=\"your-api-key\"")
	fmt.Println()
	fmt.Println("  If you enter a key here, it will be stored in the config file.")
	fmt.Println("  This is less secure on shared systems.")
	fmt.Println()

	// Show existing value if present (masked)
	existingKey := w.config.APIKey
	if existingKey != "" {
		fmt.Printf("  Current: %s\n", MaskSecret(existingKey))
		fmt.Println("  Press Enter to keep, or type new value:")
		fmt.Println()
	}

	key, err := w.prompt("API Key (or press Enter to skip): ")
	if err != nil {
		return err
	}

	key = strings.TrimSpace(key)

	// If empty and we have existing value, keep it
	if key == "" {
		if existingKey != "" {
			fmt.Printf("  ✓ Keeping existing API key: %s\n", MaskSecret(existingKey))
			fmt.Println()
			return nil
		}
		// Allow skipping if they plan to use env var
		fmt.Println("  ⚠ No API key set. Set AI_AAS_API_KEY before using CLI commands.")
		fmt.Println()
		return nil
	}

	w.config.APIKey = key

	fmt.Println()
	return nil
}

func (w *InitWizard) collectEnvironment() error {
	fmt.Println("Target Environment (REQUIRED)")
	fmt.Println("  The deployment environment this CLI will operate on")
	fmt.Println("  Options: development, staging, production")
	fmt.Println("  Can also be set via AI_AAS_ENVIRONMENT env var")
	fmt.Println()

	// Check if environment is set via env var
	envFromVar := os.Getenv("AI_AAS_ENVIRONMENT")
	if envFromVar != "" {
		fmt.Printf("  Found AI_AAS_ENVIRONMENT=%s\n", envFromVar)
		w.config.Environment = normalizeEnvironment(envFromVar)
		fmt.Printf("  Using environment: %s\n", w.config.Environment)
		fmt.Println()
		return nil
	}

	// Show existing value if present
	existingEnv := w.config.Environment
	if existingEnv != "" && existingEnv != "development" {
		fmt.Printf("  Current: %s\n", existingEnv)
		fmt.Println("  Press Enter to keep, or type new value:")
		fmt.Println()
	} else if existingEnv == "development" {
		fmt.Printf("  Current: %s (default)\n", existingEnv)
		fmt.Println("  Press Enter to keep, or type new value:")
		fmt.Println()
	}

	env, err := w.prompt("Environment: ")
	if err != nil {
		return err
	}

	env = strings.TrimSpace(env)

	// If empty and we have existing value, keep it
	if env == "" {
		if existingEnv != "" {
			fmt.Printf("  ✓ Keeping existing environment: %s\n", existingEnv)
			fmt.Println()
			return nil
		}
		return fmt.Errorf("environment is required (set via prompt or AI_AAS_ENVIRONMENT env var)")
	}

	w.config.Environment = normalizeEnvironment(env)
	fmt.Printf("  Using environment: %s\n", w.config.Environment)

	fmt.Println()
	return nil
}

// normalizeEnvironment normalizes environment names
func normalizeEnvironment(env string) string {
	env = strings.TrimSpace(strings.ToLower(env))
	switch env {
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

func (w *InitWizard) validateConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to connect to the API - any HTTP response means the API is reachable
	req, err := http.NewRequestWithContext(ctx, "GET", w.config.APIEndpoint+"/healthz", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", w.config.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Any HTTP response means the API is reachable
	// 401/403 means auth is required but API is working
	// 5xx errors indicate server problems
	if resp.StatusCode >= 500 {
		return fmt.Errorf("API returned server error: %d", resp.StatusCode)
	}

	return nil
}

func (w *InitWizard) prompt(message string) (string, error) {
	fmt.Print(message)
	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func isInteractive() bool {
	// Check if stdin is a terminal
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// InitFromFlags initializes config from command-line flags (non-interactive)
func InitFromFlags(apiKey, endpoint, environment, hfToken string) error {
	cfg := DefaultConfig()

	if apiKey != "" {
		cfg.APIKey = apiKey
	} else {
		return fmt.Errorf("--api-key is required for non-interactive init")
	}

	if endpoint != "" {
		cfg.APIEndpoint = endpoint
	}

	if environment != "" {
		cfg.Environment = environment
	}

	if hfToken != "" {
		cfg.HFToken = hfToken
	}

	return Save(cfg)
}

// detectS3Credentials auto-detects S3/Object Storage credentials from environment variables
// Supports Linode Object Storage (LINODE_OBJECT_STORAGE_*) and standard AWS (AWS_*)
func (w *InitWizard) detectS3Credentials() {
	// Check for Linode Object Storage credentials
	accessKey := os.Getenv("LINODE_OBJECT_STORAGE_ACCESS_KEY")
	secretKey := os.Getenv("LINODE_OBJECT_STORAGE_SECRET_KEY")

	// Fall back to AWS-style credentials
	if accessKey == "" {
		accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if secretKey == "" {
		secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	// Check for S3 endpoint and bucket
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("LINODE_OBJECT_STORAGE_ENDPOINT")
	}
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		bucket = os.Getenv("LINODE_OBJECT_STORAGE_BUCKET")
	}

	// If we have credentials, configure them
	if accessKey != "" && secretKey != "" {
		fmt.Println()
		fmt.Println("Object Storage Configuration")
		fmt.Println("  Found S3/Object Storage credentials in environment")

		// Set via viper for nested s3.* keys
		viper.Set("s3.access_key", accessKey)
		viper.Set("s3.secret_key", secretKey)

		if endpoint != "" {
			viper.Set("s3.endpoint", endpoint)
			fmt.Printf("  Endpoint: %s\n", endpoint)
		} else {
			// Default to Linode Paris (fr-par-1) if not specified
			defaultEndpoint := "https://fr-par-1.linodeobjects.com"
			viper.Set("s3.endpoint", defaultEndpoint)
			fmt.Printf("  Endpoint: %s (default)\n", defaultEndpoint)
		}

		if bucket != "" {
			viper.Set("s3.bucket", bucket)
			fmt.Printf("  Bucket: %s\n", bucket)
		} else {
			// Default bucket name
			defaultBucket := "ai-aas"
			viper.Set("s3.bucket", defaultBucket)
			fmt.Printf("  Bucket: %s (default)\n", defaultBucket)
		}

		fmt.Printf("  Access Key: %s\n", MaskSecret(accessKey))
		fmt.Println("✓ S3 credentials configured")
	}
}

