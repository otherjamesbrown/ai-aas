package usecases_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// Environment variable names for API configuration
const (
	envAPIEndpoint     = "AI_AAS_API_ENDPOINT"
	envAPIKey          = "AI_AAS_API_KEY"
	envOrgID           = "AI_AAS_ORG_ID"
	envAPIRouterURL    = "AI_AAS_API_ROUTER_URL"
	envAnalyticsURL    = "AI_AAS_ANALYTICS_URL"
	envAdminAPIKey     = "AI_AAS_ADMIN_API_KEY"
	envTestModel       = "AI_AAS_TEST_MODEL"
	// Persistent metrics org for analytics/usage tests that need historical data
	envMetricsOrgID  = "AI_AAS_METRICS_ORG_ID"
	envMetricsAPIKey = "AI_AAS_METRICS_API_KEY"
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

// PTYInteraction represents a prompt/response pair for interactive CLI testing
type PTYInteraction struct {
	// WaitFor is the string to wait for before sending input (e.g., "[y/N]:")
	WaitFor string
	// SendInput is the input to send when WaitFor is detected (e.g., "y\n")
	SendInput string
}

// runPlatformCLIWithPTY executes an ai-aas-cli command with a PTY for interactive input.
// It allows testing commands that prompt for confirmation (y/N).
//
// Example usage:
//
//	result := runPlatformCLIWithPTY(
//	    []PTYInteraction{{WaitFor: "[y/N]:", SendInput: "y\n"}},
//	    "model", "deploy", "delete", "llama-7b", "-e", "development",
//	)
func runPlatformCLIWithPTY(interactions []PTYInteraction, args ...string) CLIResult {
	cmd := exec.Command("ai-aas-cli", args...)
	cmd.Env = os.Environ()

	// Start the command with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return CLIResult{
			Error:    "failed to start PTY: " + err.Error(),
			ExitCode: -1,
		}
	}
	defer ptmx.Close()

	// Collect all output
	var outputBuf bytes.Buffer

	// Channel to signal when we're done reading
	done := make(chan error, 1)

	// Read output in a goroutine
	go func() {
		buf := make([]byte, 1024)
		interactionIdx := 0

		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err == io.EOF {
					done <- nil
				} else {
					done <- err
				}
				return
			}

			if n > 0 {
				outputBuf.Write(buf[:n])

				// Check if we need to send any interaction
				if interactionIdx < len(interactions) {
					currentOutput := outputBuf.String()
					if strings.Contains(currentOutput, interactions[interactionIdx].WaitFor) {
						// Small delay to ensure the prompt is fully written
						time.Sleep(50 * time.Millisecond)
						_, writeErr := ptmx.Write([]byte(interactions[interactionIdx].SendInput))
						if writeErr != nil {
							done <- writeErr
							return
						}
						interactionIdx++
					}
				}
			}
		}
	}()

	// Wait for the command to complete with timeout
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	// Wait for either completion or timeout
	var cmdErr error
	select {
	case cmdErr = <-cmdDone:
		// Command completed, give a moment for final output
		time.Sleep(100 * time.Millisecond)
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		return CLIResult{
			Output:   outputBuf.String(),
			Error:    "command timed out after 30 seconds",
			ExitCode: -1,
		}
	}

	// Get exit code
	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return CLIResult{
		Output:   outputBuf.String(),
		ExitCode: exitCode,
	}
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
		apiRouterURL := getAPIRouterURL()

		// Build config with all required endpoints
		configContent := "api_endpoint: " + endpoint + "\n"
		configContent += "admin_endpoint: " + endpoint + "\n" // Use same endpoint for admin operations
		configContent += "api_key: " + apiKey + "\n"
		if orgID != "" {
			configContent += "org_id: " + orgID + "\n"
		}
		if apiRouterURL != "" {
			configContent += "inference_endpoint: " + apiRouterURL + "\n"
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

// generateUniqueID creates a unique identifier for test data using timestamp + random suffix
// This ensures uniqueness even across test runs in the same second
func generateUniqueID() string {
	// Use timestamp (YYYYMMDDHHMMSS) + random 6-char hex suffix for guaranteed uniqueness
	// Random suffix prevents collisions when tests run in parallel or in same second
	randomBytes := make([]byte, 3) // 3 bytes = 6 hex chars
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to nanosecond timestamp if crypto/rand fails
		return getCurrentTimestamp() + strings.ReplaceAll(time.Now().Format("150405.000000"), ".", "")[:6]
	}

	timestamp := getCurrentTimestamp()
	return timestamp + hex.EncodeToString(randomBytes)
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

// skipIfNoOrgCLI skips the test if ai-aas-org is not available.
// Tests should call this at the start to conditionally run when the org CLI is installed.
func skipIfNoOrgCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ai-aas-org"); err != nil {
		t.Skip("ai-aas-org not found in PATH: install or build it first")
	}
}

// getAPIRouterURL returns the API router service URL.
// Defaults to AI_AAS_API_ENDPOINT if AI_AAS_API_ROUTER_URL not set.
func getAPIRouterURL() string {
	url := os.Getenv(envAPIRouterURL)
	if url == "" {
		// Fallback to main API endpoint
		url = os.Getenv(envAPIEndpoint)
	}
	return url
}

// getAnalyticsServiceURL returns the analytics service URL.
func getAnalyticsServiceURL() string {
	return os.Getenv(envAnalyticsURL)
}

// getAdminAPIKey returns the admin API key for administrative operations.
// Defaults to AI_AAS_API_KEY if AI_AAS_ADMIN_API_KEY not set.
func getAdminAPIKey() string {
	adminKey := os.Getenv(envAdminAPIKey)
	if adminKey == "" {
		// Fallback to regular API key
		return os.Getenv(envAPIKey)
	}
	return adminKey
}

// getTestModel returns the model name to use for testing.
// Defaults to "gpt-4" if not specified.
func getTestModel() string {
	model := os.Getenv(envTestModel)
	if model == "" {
		return "gpt-4" // Default test model
	}
	return model
}

// skipIfNoVLLMBackend skips the test if vLLM backend is not available.
// Tests that require inference capability should call this to gracefully skip
// when the infrastructure (GPU nodes, vLLM deployments) is not available.
func skipIfNoVLLMBackend(t *testing.T) {
	t.Helper()

	apiRouterURL := getAPIRouterURL()
	if apiRouterURL == "" {
		t.Skip("Skipping: API router URL not configured (AI_AAS_API_ROUTER_URL)")
		return
	}

	// Use admin API key to check if models are available via /v1/models
	// This doesn't consume quota like an inference request would
	adminKey := getAdminAPIKey()
	if adminKey == "" {
		// No credentials available - cannot verify backend availability
		// Skip the test to avoid false failures
		t.Skip("Skipping: no admin API key available to verify vLLM backend availability")
		return
	}

	client := NewTestClient(apiRouterURL, adminKey)

	// Check /v1/models to see if any models are available
	// This is a read-only endpoint that doesn't consume quota
	resp, err := client.GET("/v1/models")

	// If we get a network error or connection refused, backend is not available
	if err != nil {
		t.Skipf("Skipping: vLLM backend not reachable (%v)", err)
		return
	}

	// Check response body for model availability
	bodyStr := resp.String()
	lowerBody := strings.ToLower(bodyStr)

	// If we get a 404 or auth error, check if it's a "no models" situation
	if resp.StatusCode == 404 {
		t.Skip("Skipping: no vLLM backend available in this environment (models endpoint not found)")
		return
	}

	// If we get a successful response, check if there are any models
	if resp.StatusCode == 200 {
		var modelsResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := resp.DecodeJSON(&modelsResp); err == nil {
			if len(modelsResp.Data) == 0 {
				t.Skip("Skipping: no models available in this environment")
				return
			}
			// Models are available, tests can proceed
			return
		}
	}

	// If we get a "no routing policy configured" error, there are no backends configured
	if strings.Contains(lowerBody, "no routing policy") || strings.Contains(lowerBody, "routing_error") {
		t.Skip("Skipping: no vLLM backend available in this environment (no routing policy configured)")
		return
	}

	// Any other response (401 unauthorized, 400 bad request, 500 other error)
	// means the endpoint exists and tests should proceed
	// The actual tests will handle authentication and validation
}

// TestOrgContext provides a fresh organization context for CLI tests.
// Each test gets its own isolated org, service account, and API key.
// This prevents quota collisions between tests using the master admin org.
type TestOrgContext struct {
	t          *testing.T
	OrgID      string
	OrgName    string
	APIKey     string
	configPath string
	client     *TestClient
}

// NewTestOrgContext creates a fresh org with service account and API key for testing.
// The org and all associated resources are automatically cleaned up after the test.
// Use this instead of runOrgCLI when you need an isolated org that won't hit quota limits.
func NewTestOrgContext(t *testing.T) *TestOrgContext {
	t.Helper()

	adminEndpoint := getAPIEndpoint()
	adminKey := getAdminAPIKey()
	apiRouterURL := getAPIRouterURL()

	if adminEndpoint == "" || adminKey == "" {
		t.Skip("requires live API: set AI_AAS_API_ENDPOINT and AI_AAS_ADMIN_API_KEY")
	}

	client := NewTestClient(adminEndpoint, adminKey)

	// Create a unique org name
	uniqueID := generateUniqueID()
	orgName := "test-org-" + uniqueID
	orgSlug := "test-" + uniqueID

	// Create organization via Admin API
	orgResp, err := client.POST("/v1/orgs", map[string]interface{}{
		"name": orgName,
		"slug": orgSlug,
	})
	if err != nil {
		t.Fatalf("failed to create test org: %v", err)
	}
	if orgResp.StatusCode != 200 && orgResp.StatusCode != 201 {
		t.Fatalf("failed to create test org: status %d, body: %s", orgResp.StatusCode, orgResp.String())
	}

	var org struct {
		OrgID string `json:"orgId"`
	}
	if err := orgResp.DecodeJSON(&org); err != nil {
		t.Fatalf("failed to parse org response: %v", err)
	}

	// Create service account
	saResp, err := client.POST("/v1/orgs/"+org.OrgID+"/service-accounts", map[string]interface{}{
		"name": "test-sa-" + uniqueID,
	})
	if err != nil {
		// Cleanup org before failing
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to create service account: %v", err)
	}
	if saResp.StatusCode != 200 && saResp.StatusCode != 201 {
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to create service account: status %d, body: %s", saResp.StatusCode, saResp.String())
	}

	var sa struct {
		ServiceAccountID string `json:"serviceAccountId"`
	}
	if err := saResp.DecodeJSON(&sa); err != nil {
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to parse service account response: %v", err)
	}

	// Create API key with full scopes
	keyResp, err := client.POST("/v1/orgs/"+org.OrgID+"/service-accounts/"+sa.ServiceAccountID+"/api-keys", map[string]interface{}{
		"name":   "test-key-" + uniqueID,
		"scopes": []string{"inference:read", "inference:write", "org:read", "org:write", "admin:read", "admin:write"},
	})
	if err != nil {
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to create API key: %v", err)
	}
	if keyResp.StatusCode != 200 && keyResp.StatusCode != 201 {
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to create API key: status %d, body: %s", keyResp.StatusCode, keyResp.String())
	}

	var apiKey struct {
		Token string `json:"token"`
	}
	if err := keyResp.DecodeJSON(&apiKey); err != nil {
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to parse API key response: %v", err)
	}

	// Create temp config file for CLI
	configContent := "api_endpoint: " + adminEndpoint + "\n"
	configContent += "admin_endpoint: " + adminEndpoint + "\n"
	configContent += "api_key: " + apiKey.Token + "\n"
	configContent += "org_id: " + org.OrgID + "\n"
	if apiRouterURL != "" {
		configContent += "inference_endpoint: " + apiRouterURL + "\n"
	}

	tmpFile, err := os.CreateTemp("", "ai-aas-org-test-*.yaml")
	if err != nil {
		client.DELETE("/v1/orgs/" + org.OrgID)
		t.Fatalf("failed to create temp config: %v", err)
	}
	tmpFile.WriteString(configContent)
	tmpFile.Close()

	ctx := &TestOrgContext{
		t:          t,
		OrgID:      org.OrgID,
		OrgName:    orgName,
		APIKey:     apiKey.Token,
		configPath: tmpFile.Name(),
		client:     client,
	}

	// Register cleanup
	t.Cleanup(func() {
		// Delete org (cascades to service accounts and API keys)
		resp, err := client.DELETE("/v1/orgs/" + ctx.OrgID)
		if err != nil {
			t.Logf("Warning: failed to cleanup test org %s: %v", ctx.OrgID, err)
		} else if resp.StatusCode != 200 && resp.StatusCode != 204 && resp.StatusCode != 404 {
			t.Logf("Warning: cleanup returned status %d for org %s", resp.StatusCode, ctx.OrgID)
		}
		// Remove temp config file
		os.Remove(ctx.configPath)
	})

	return ctx
}

// RunCLI executes an ai-aas-org command in the context of this test org.
// The command automatically uses the test org's credentials.
func (ctx *TestOrgContext) RunCLI(args ...string) CLIResult {
	return runOrgCLIWithConfig(ctx.configPath, args...)
}

// TestUserInContext represents a user created within a TestOrgContext
type TestUserInContext struct {
	Email       string
	DisplayName string
	UserID      string
	Role        string
}

// CreateUser creates a user within this test org and registers cleanup.
// Returns the created user's details.
func (ctx *TestOrgContext) CreateUser(emailPrefix string, role string) *TestUserInContext {
	ctx.t.Helper()

	timestamp := generateUniqueID()
	email := emailPrefix + "-" + timestamp + "@example.com"
	displayName := "Test User " + timestamp

	args := []string{"user", "create", "--user-email", email, "--user-display-name", displayName}
	if role != "" && role != "user" {
		args = append(args, "--role", role)
	}

	result := ctx.RunCLI(args...)
	if result.ExitCode != 0 {
		ctx.t.Fatalf("Failed to create test user: %s", result.Output)
	}

	// Extract user ID from output using JSON
	listResult := ctx.RunCLI("user", "list", "--json")
	if listResult.ExitCode != 0 {
		ctx.t.Fatalf("Failed to list users to find created user: %s", listResult.Output)
	}

	var users []struct {
		ID    string `json:"userId"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.Unmarshal([]byte(listResult.Output), &users); err != nil {
		ctx.t.Fatalf("Failed to parse user list: %v\nOutput: %s", err, listResult.Output)
	}

	var userID string
	for _, u := range users {
		if u.Email == email {
			userID = u.ID
			break
		}
	}

	if userID == "" {
		ctx.t.Fatalf("Created user %s not found in user list. Found %d users total.", email, len(users))
	}

	testUser := &TestUserInContext{
		Email:       email,
		DisplayName: displayName,
		UserID:      userID,
		Role:        role,
	}

	// Register cleanup to delete the user after the test
	ctx.t.Cleanup(func() {
		cleanupResult := ctx.RunCLI("user", "delete", email, "--force")
		if cleanupResult.ExitCode != 0 && cleanupResult.ExitCode != 5 {
			ctx.t.Logf("Warning: Failed to cleanup test user %s: %s", email, cleanupResult.Output)
		}
	})

	return testUser
}

// GetMetricsOrgID returns the persistent metrics org ID for tests that need historical data.
// Returns empty string if not configured (test should skip).
func getMetricsOrgID() string {
	return os.Getenv(envMetricsOrgID)
}

// GetMetricsAPIKey returns the API key for the persistent metrics org.
// Returns empty string if not configured (test should skip).
func getMetricsAPIKey() string {
	return os.Getenv(envMetricsAPIKey)
}

// skipIfNoMetricsOrg skips the test if the persistent metrics org is not configured.
// Use this for tests that require historical usage data.
func skipIfNoMetricsOrg(t *testing.T) {
	t.Helper()
	if os.Getenv(envMetricsOrgID) == "" {
		t.Skip("requires persistent metrics org: set AI_AAS_METRICS_ORG_ID and AI_AAS_METRICS_API_KEY")
	}
}

// RunMetricsOrgCLI executes an ai-aas-org command against the persistent metrics org.
// Use this for tests that need to verify historical usage/analytics data.
func runMetricsOrgCLI(args ...string) CLIResult {
	cmd := exec.Command("ai-aas-org", args...)
	cmd.Env = os.Environ()

	// Create temp config with metrics org credentials
	endpoint := os.Getenv(envAPIEndpoint)
	apiKey := os.Getenv(envMetricsAPIKey)
	orgID := os.Getenv(envMetricsOrgID)
	apiRouterURL := getAPIRouterURL()

	if endpoint == "" || apiKey == "" || orgID == "" {
		return CLIResult{
			Error:    "metrics org not configured",
			ExitCode: 1,
		}
	}

	configContent := "api_endpoint: " + endpoint + "\n"
	configContent += "admin_endpoint: " + endpoint + "\n"
	configContent += "api_key: " + apiKey + "\n"
	configContent += "org_id: " + orgID + "\n"
	if apiRouterURL != "" {
		configContent += "inference_endpoint: " + apiRouterURL + "\n"
	}

	tmpFile, err := os.CreateTemp("", "ai-aas-metrics-org-*.yaml")
	if err != nil {
		return CLIResult{
			Error:    "failed to create temp config: " + err.Error(),
			ExitCode: 1,
		}
	}
	tmpFile.WriteString(configContent)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	args = append([]string{"--config", tmpFile.Name()}, args...)
	cmd = exec.Command("ai-aas-org", args...)
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
