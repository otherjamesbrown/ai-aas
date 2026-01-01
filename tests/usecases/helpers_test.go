package usecases_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Environment variable names for API configuration
const (
	envAPIEndpoint = "AI_AAS_API_ENDPOINT"
	envAPIKey      = "AI_AAS_API_KEY"
	envOrgID       = "AI_AAS_ORG_ID"
)

// skipIfNoLiveAPI skips the test if the live API is not configured.
// Tests should call this at the start to conditionally run against the remote environment.
func skipIfNoLiveAPI(t *testing.T) {
	t.Helper()
	if os.Getenv(envAPIEndpoint) == "" {
		t.Skip("requires live API: set AI_AAS_API_ENDPOINT and AI_AAS_API_KEY")
	}
}

// skipIfNoLiveAPIWithReason skips with a custom reason appended
func skipIfNoLiveAPIWithReason(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv(envAPIEndpoint) == "" {
		t.Skipf("requires live API (%s): set AI_AAS_API_ENDPOINT and AI_AAS_API_KEY", reason)
	}
}

// requireLiveAPI returns true if the live API is available, false otherwise.
// Use this when you need to check availability without skipping.
func requireLiveAPI() bool {
	return os.Getenv(envAPIEndpoint) != ""
}

// getAPIEndpoint returns the configured API endpoint
func getAPIEndpoint() string {
	return os.Getenv(envAPIEndpoint)
}

// getAPIKey returns the configured API key
func getAPIKey() string {
	return os.Getenv(envAPIKey)
}

// assertContains checks if the output contains the expected substring
func assertContains(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, got: %s", expected, output)
	}
}

// assertNotContains checks if the output does not contain the unexpected substring
func assertNotContains(t *testing.T, output, unexpected string) {
	t.Helper()
	if strings.Contains(output, unexpected) {
		t.Errorf("Expected output to NOT contain %q, got: %s", unexpected, output)
	}
}

// CLIResult holds the result of a CLI command execution
type CLIResult struct {
	Output   string
	Error    string
	ExitCode int
}

// runOrgCLI executes an ai-aas-org command and returns the result.
// It automatically uses a temporary config file with the API endpoint and key
// from environment variables if AI_AAS_API_ENDPOINT is set.
func runOrgCLI(args ...string) CLIResult {
	cmd := exec.Command("ai-aas-org", args...)

	// Pass current environment to subprocess
	cmd.Env = os.Environ()

	// Track temp file for cleanup
	var tmpFileName string

	// If live API is configured, create a temp config file and use it
	// This ensures the CLI uses the test configuration, not ~/.ai-aas-org.yaml
	if endpoint := os.Getenv(envAPIEndpoint); endpoint != "" {
		apiKey := os.Getenv(envAPIKey)
		orgID := os.Getenv(envOrgID)
		configContent := "api_endpoint: " + endpoint + "\napi_key: " + apiKey + "\n"
		if orgID != "" {
			configContent += "org_id: " + orgID + "\n"
		}

		tmpFile, err := os.CreateTemp("", "ai-aas-org-test-*.yaml")
		if err == nil {
			tmpFile.WriteString(configContent)
			tmpFile.Close()
			tmpFileName = tmpFile.Name()
			// Add --config flag to use temp config
			args = append([]string{"--config", tmpFileName}, args...)
			cmd = exec.Command("ai-aas-org", args...)
			cmd.Env = os.Environ()
		}
	}

	output, err := cmd.CombinedOutput()

	// Clean up temp file after command completes
	if tmpFileName != "" {
		os.Remove(tmpFileName)
	}

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}

// runOrgCLIWithEnv executes an ai-aas-org command with custom environment variables
func runOrgCLIWithEnv(env map[string]string, args ...string) CLIResult {
	cmd := exec.Command("ai-aas-org", args...)

	// Copy current environment and add custom vars
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	output, err := cmd.CombinedOutput()

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}

// runOrgCLIWithConfig executes an ai-aas-org command with a specific config file.
// This is the preferred way to run tests that need to use a custom config path,
// as the CLI reads the config path from the --config flag, not from environment variables.
func runOrgCLIWithConfig(configPath string, args ...string) CLIResult {
	// Prepend --config flag to args
	fullArgs := append([]string{"--config", configPath}, args...)
	cmd := exec.Command("ai-aas-org", fullArgs...)

	// Pass current environment
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}

// tempConfigFile creates a temporary config file path and returns it
// The temporary directory is automatically cleaned up when the test ends
func tempConfigFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	return filepath.Join(tmpDir, ".ai-aas-org.yaml")
}

// isValidJSON checks if a string is valid JSON
func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// Test Fixture Helpers

// TestUser represents a user created for testing
type TestUser struct {
	Email       string
	DisplayName string
	UserID      string
	Role        string
}

// createTestUser creates a test user and registers cleanup
func createTestUser(t *testing.T, emailPrefix string, role string) *TestUser {
	t.Helper()

	timestamp := generateUniqueID()
	email := emailPrefix + "-" + timestamp + "@example.com"
	displayName := "Test User " + timestamp

	args := []string{"user", "create", "--user-email", email, "--user-display-name", displayName}
	if role != "" && role != "user" {
		args = append(args, "--role", role)
	}

	result := runOrgCLI(args...)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to create test user: %s", result.Output)
	}

	// Extract user ID from output using JSON
	listResult := runOrgCLI("user", "list", "--json")
	if listResult.ExitCode != 0 {
		t.Fatalf("Failed to list users to find created user: %s", listResult.Output)
	}

	var users []struct {
		ID    string `json:"userId"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal([]byte(listResult.Output), &users); err != nil {
		t.Fatalf("Failed to parse user list: %v\nOutput: %s", err, listResult.Output)
	}

	var userID string
	for _, u := range users {
		if u.Email == email {
			userID = u.ID
			break
		}
	}

	if userID == "" {
		t.Fatalf("Created user %s not found in user list. Found %d users total.", email, len(users))
	}

	testUser := &TestUser{
		Email:       email,
		DisplayName: displayName,
		UserID:      userID,
		Role:        role,
	}

	// Register cleanup to delete the user after the test
	t.Cleanup(func() {
		cleanupResult := runOrgCLI("user", "delete", email, "--force")
		if cleanupResult.ExitCode != 0 && cleanupResult.ExitCode != 5 {
			t.Logf("Warning: Failed to cleanup test user %s: %s", email, cleanupResult.Output)
		}
	})

	return testUser
}

// generateUniqueID creates a unique identifier for test data using timestamp and random suffix
func generateUniqueID() string {
	// Using Unix timestamp in microseconds for uniqueness
	return strings.ReplaceAll(strings.ReplaceAll(
		getCurrentTimestamp(),
		":", ""), "-", "")
}

// getCurrentTimestamp returns current time formatted as a unique string
func getCurrentTimestamp() string {
	// Get timestamp by running date command
	cmd := exec.Command("date", "+%Y%m%d%H%M%S")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to process ID if date fails
		return strings.TrimSpace(string(output))
	}
	return strings.TrimSpace(string(output))
}

// Platform CLI Helpers
// These helpers support running the ai-aas-cli (platform CLI) for UC tests.

// skipIfNoPlatformCLI skips the test if ai-aas-cli is not available.
// Tests should call this at the start to conditionally run when the CLI is installed.
func skipIfNoPlatformCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ai-aas-cli"); err != nil {
		t.Skip("ai-aas-cli not found in PATH: install or build it first")
	}
}

// runPlatformCLI executes an ai-aas-cli command and returns the result.
// It automatically uses the API endpoint and key from environment variables
// if AI_AAS_API_ENDPOINT is set.
func runPlatformCLI(args ...string) CLIResult {
	return runPlatformCLIWithProfile("", args...)
}

// runPlatformCLIWithProfile executes an ai-aas-cli command with a specific profile.
// If profile is empty, it uses the default profile or environment variables.
func runPlatformCLIWithProfile(profile string, args ...string) CLIResult {
	cmd := exec.Command("ai-aas-cli", args...)

	// Pass current environment to subprocess
	cmd.Env = os.Environ()

	// If a profile is specified, add --profile flag
	if profile != "" {
		args = append([]string{"--profile", profile}, args...)
		cmd = exec.Command("ai-aas-cli", args...)
		cmd.Env = os.Environ()
	}

	// If live API is configured via env vars, ensure they're passed through
	// The ai-aas-cli will respect AI_AAS_API_ENDPOINT and AI_AAS_API_KEY

	output, err := cmd.CombinedOutput()

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}
