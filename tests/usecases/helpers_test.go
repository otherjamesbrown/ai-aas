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
			// Add --config flag to use temp config
			args = append([]string{"--config", tmpFile.Name()}, args...)
			cmd = exec.Command("ai-aas-org", args...)
			cmd.Env = os.Environ()
			defer os.Remove(tmpFile.Name())
		}
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
