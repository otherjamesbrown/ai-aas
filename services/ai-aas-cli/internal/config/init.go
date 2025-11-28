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

	// Check for existing config
	if Exists() {
		fmt.Println("⚠ Existing configuration found.")
		overwrite, err := w.prompt("Overwrite existing configuration? (y/N): ")
		if err != nil {
			return err
		}
		if !strings.HasPrefix(strings.ToLower(overwrite), "y") {
			fmt.Println("Keeping existing configuration.")
			return nil
		}
	}

	// Collect configuration
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
	fmt.Println("    - Base domain:  172.232.58.222.nip.io")
	fmt.Println("    - Full API URL: https://api.172.232.58.222.nip.io")
	fmt.Println()
	fmt.Println("  If you provide a base domain, endpoints will be constructed as:")
	fmt.Println("    api.<domain>, user-org.<domain>, etc.")
	fmt.Println()

	input, err := w.prompt("Domain or API Endpoint: ")
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	if input == "" {
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

		// Construct all endpoints from base domain
		w.config.APIEndpoint = fmt.Sprintf("https://api.%s", baseDomain)
		w.config.UserOrgEndpoint = fmt.Sprintf("https://user-org.%s", baseDomain)
		w.config.InferenceEndpoint = fmt.Sprintf("https://api.%s", baseDomain)

		fmt.Println()
		fmt.Println("  Constructed endpoints from base domain:")
		fmt.Printf("    API Endpoint:      %s\n", w.config.APIEndpoint)
		fmt.Printf("    User-Org Service:  %s\n", w.config.UserOrgEndpoint)
		fmt.Printf("    Inference API:     %s\n", w.config.InferenceEndpoint)
	}

	fmt.Println()
	return nil
}

func (w *InitWizard) collectAPIKey() error {
	fmt.Println("API Key")
	fmt.Println("  Your Admin API key for authentication")
	fmt.Println("  (This will be stored securely)")
	fmt.Println()

	key, err := w.prompt("API Key: ")
	if err != nil {
		return err
	}

	if key == "" {
		return fmt.Errorf("API key is required")
	}

	w.config.APIKey = strings.TrimSpace(key)

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

	env, err := w.prompt("Environment (required): ")
	if err != nil {
		return err
	}

	if env == "" {
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

