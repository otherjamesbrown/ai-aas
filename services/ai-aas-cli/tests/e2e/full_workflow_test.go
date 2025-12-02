//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the ai-aas-cli.
//
// These tests run against a live development environment and exercise
// the complete workflow from organization creation through inference.
//
// Prerequisites:
//   - Development environment running (Kubernetes cluster, services deployed)
//   - Environment variables configured (see env.sh)
//   - ai-aas-cli binary built
//
// Run with: go test -v -tags=e2e ./tests/e2e/...
package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig holds e2e test configuration from environment variables
type TestConfig struct {
	APIKey            string
	UserOrgEndpoint   string
	InferenceEndpoint string
	AnalyticsEndpoint string
	TLSInsecure       bool
	TestOrgPrefix     string
}

// TestState holds state across test steps
type TestState struct {
	OrgID      string
	OrgSlug    string
	UserID     string
	UserEmail  string
	APIKeyID   string
	APISecret  string
	ModelID    string
	RequestID  string
}

// loadTestConfig loads test configuration from environment
func loadTestConfig(t *testing.T) *TestConfig {
	t.Helper()

	apiKey := os.Getenv("AI_AAS_API_KEY")
	if apiKey == "" {
		t.Skip("AI_AAS_API_KEY not set - skipping e2e tests. Run: source tests/e2e/env.sh")
	}

	userOrgEndpoint := os.Getenv("AI_AAS_USER_ORG_ENDPOINT")
	if userOrgEndpoint == "" {
		userOrgEndpoint = "https://user-org.dev.otherjamesbrown.com"
	}

	inferenceEndpoint := os.Getenv("AI_AAS_INFERENCE_ENDPOINT")
	if inferenceEndpoint == "" {
		inferenceEndpoint = "https://api.dev.otherjamesbrown.com"
	}

	analyticsEndpoint := os.Getenv("AI_AAS_ANALYTICS_ENDPOINT")
	if analyticsEndpoint == "" {
		analyticsEndpoint = "https://analytics.dev.otherjamesbrown.com"
	}

	tlsInsecure := os.Getenv("AI_AAS_TLS_INSECURE") == "true"

	testOrgPrefix := os.Getenv("E2E_TEST_ORG_PREFIX")
	if testOrgPrefix == "" {
		testOrgPrefix = "e2e-test-"
	}

	return &TestConfig{
		APIKey:            apiKey,
		UserOrgEndpoint:   userOrgEndpoint,
		InferenceEndpoint: inferenceEndpoint,
		AnalyticsEndpoint: analyticsEndpoint,
		TLSInsecure:       tlsInsecure,
		TestOrgPrefix:     testOrgPrefix,
	}
}

// runCLI executes the ai-aas-cli with the given arguments
func runCLI(t *testing.T, cfg *TestConfig, args ...string) (string, error) {
	t.Helper()

	// Find the CLI binary
	cliBinary := findCLIBinary(t)

	// Build command with common flags
	cmdArgs := append(args,
		"--user-org-endpoint", cfg.UserOrgEndpoint,
		"--api-key", cfg.APIKey,
		"--format", "json",
	)

	cmd := exec.Command(cliBinary, cmdArgs...)

	// Set environment
	cmd.Env = append(os.Environ(),
		"AI_AAS_TLS_INSECURE="+fmt.Sprintf("%v", cfg.TLSInsecure),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running: %s %s", cliBinary, strings.Join(cmdArgs, " "))

	err := cmd.Run()
	output := stdout.String()

	if err != nil {
		t.Logf("CLI error: %v\nStderr: %s\nStdout: %s", err, stderr.String(), output)
		return output, fmt.Errorf("cli error: %w, stderr: %s", err, stderr.String())
	}

	return output, nil
}

// runCLIWithInferenceEndpoint executes cli with inference endpoint
func runCLIWithInferenceEndpoint(t *testing.T, cfg *TestConfig, apiSecret string, args ...string) (string, error) {
	t.Helper()

	cliBinary := findCLIBinary(t)

	cmdArgs := append(args,
		"--inference-endpoint", cfg.InferenceEndpoint,
		"--api-key", apiSecret,
		"--format", "json",
	)

	cmd := exec.Command(cliBinary, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"AI_AAS_TLS_INSECURE="+fmt.Sprintf("%v", cfg.TLSInsecure),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running: %s %s", cliBinary, strings.Join(cmdArgs, " "))

	err := cmd.Run()
	output := stdout.String()

	if err != nil {
		t.Logf("CLI error: %v\nStderr: %s\nStdout: %s", err, stderr.String(), output)
		return output, fmt.Errorf("cli error: %w, stderr: %s", err, stderr.String())
	}

	return output, nil
}

// findCLIBinary locates the ai-aas-cli binary
func findCLIBinary(t *testing.T) string {
	t.Helper()

	// Check common locations
	locations := []string{
		"./ai-aas-cli",
		"../../ai-aas-cli",
		"../../../bin/ai-aas-cli",
		"../../../../bin/ai-aas-cli",
		os.Getenv("AI_AAS_CLI_PATH"),
	}

	for _, loc := range locations {
		if loc == "" {
			continue
		}
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Try to find it in PATH
	path, err := exec.LookPath("ai-aas-cli")
	if err == nil {
		return path
	}

	t.Fatal("ai-aas-cli binary not found. Build with: go build -o ai-aas-cli ./main.go")
	return ""
}

// TestFullWorkflow tests the complete CLI workflow:
// 1. Create organization
// 2. Create user
// 3. Create API key
// 4. List models
// 5. Send inference request
// 6. Verify metrics
func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	cfg := loadTestConfig(t)
	state := &TestState{}

	// Generate unique identifiers for this test run
	timestamp := time.Now().Unix()
	state.OrgSlug = fmt.Sprintf("%s%d", cfg.TestOrgPrefix, timestamp)
	state.UserEmail = fmt.Sprintf("admin-%d@test.ai-aas.dev", timestamp)

	// Cleanup function
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, state)
	})

	// Run test steps in order
	t.Run("Step1_CreateOrganization", func(t *testing.T) {
		testCreateOrganization(t, cfg, state)
	})

	t.Run("Step2_CreateUser", func(t *testing.T) {
		testCreateUser(t, cfg, state)
	})

	t.Run("Step3_CreateAPIKey", func(t *testing.T) {
		testCreateAPIKey(t, cfg, state)
	})

	t.Run("Step4_ListModels", func(t *testing.T) {
		testListModels(t, cfg, state)
	})

	t.Run("Step5_SendInferenceRequest", func(t *testing.T) {
		testSendInferenceRequest(t, cfg, state)
	})

	t.Run("Step6_VerifyMetrics", func(t *testing.T) {
		testVerifyMetrics(t, cfg, state)
	})
}

// testCreateOrganization creates a test organization
func testCreateOrganization(t *testing.T, cfg *TestConfig, state *TestState) {
	orgName := fmt.Sprintf("E2E Test Org %s", state.OrgSlug)

	output, err := runCLI(t, cfg,
		"org", "create",
		"--name", orgName,
		"--slug", state.OrgSlug,
	)
	require.NoError(t, err, "Failed to create organization")

	// Parse JSON response
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Failed to parse org create response")

	state.OrgID = getString(result, "orgId")
	require.NotEmpty(t, state.OrgID, "Organization ID should not be empty")

	t.Logf("Created organization: ID=%s, Slug=%s", state.OrgID, state.OrgSlug)

	// Verify by listing orgs
	listOutput, err := runCLI(t, cfg, "org", "list")
	require.NoError(t, err, "Failed to list organizations")

	assert.Contains(t, listOutput, state.OrgSlug, "Created org should appear in list")
}

// testCreateUser invites a user to the organization
func testCreateUser(t *testing.T, cfg *TestConfig, state *TestState) {
	require.NotEmpty(t, state.OrgSlug, "Organization must be created first")

	output, err := runCLI(t, cfg,
		"user", "create",
		"--org-id", state.OrgSlug,
		"--email", state.UserEmail,
		"--roles", "admin",
	)
	require.NoError(t, err, "Failed to create user")

	// Parse JSON response
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Failed to parse user create response")

	state.UserID = getString(result, "userId")
	require.NotEmpty(t, state.UserID, "User ID should not be empty")

	t.Logf("Created user: ID=%s, Email=%s", state.UserID, state.UserEmail)

	// Activate the user (newly created users are in "invited" status)
	// The API key creation requires users to be "active"
	_, err = runCLI(t, cfg,
		"user", "update",
		"--org-id", state.OrgSlug,
		"--user-id", state.UserID,
		"--status", "active",
	)
	require.NoError(t, err, "Failed to activate user")
	t.Logf("Activated user: ID=%s", state.UserID)

	// Verify by listing users
	listOutput, err := runCLI(t, cfg, "user", "list", "--org-id", state.OrgSlug)
	require.NoError(t, err, "Failed to list users")

	assert.Contains(t, listOutput, state.UserEmail, "Created user should appear in list")
}

// testCreateAPIKey creates an API key for the user
func testCreateAPIKey(t *testing.T, cfg *TestConfig, state *TestState) {
	require.NotEmpty(t, state.OrgSlug, "Organization must be created first")
	require.NotEmpty(t, state.UserID, "User must be created first")

	output, err := runCLI(t, cfg,
		"apikey", "create",
		"--org-id", state.OrgSlug,
		"--user-id", state.UserID,
		"--scopes", "inference:read,inference:write,models:read",
	)
	require.NoError(t, err, "Failed to create API key")

	// Parse JSON response
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Failed to parse apikey create response")

	state.APIKeyID = getString(result, "apiKeyId")
	state.APISecret = getString(result, "secret")

	require.NotEmpty(t, state.APIKeyID, "API Key ID should not be empty")
	require.NotEmpty(t, state.APISecret, "API Secret should not be empty")

	t.Logf("Created API key: ID=%s, Secret=****%s", state.APIKeyID, state.APISecret[len(state.APISecret)-4:])

	// Verify by listing API keys
	listOutput, err := runCLI(t, cfg, "apikey", "list", "--org-id", state.OrgSlug)
	require.NoError(t, err, "Failed to list API keys")

	assert.Contains(t, listOutput, state.APIKeyID, "Created API key should appear in list")
}

// testListModels lists available models using the new API key
func testListModels(t *testing.T, cfg *TestConfig, state *TestState) {
	require.NotEmpty(t, state.APISecret, "API key must be created first")

	output, err := runCLIWithInferenceEndpoint(t, cfg, state.APISecret,
		"inference", "get-models",
	)
	require.NoError(t, err, "Failed to list models")

	// Parse JSON response
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Failed to parse models response")

	data, ok := result["data"].([]interface{})
	require.True(t, ok, "Response should have 'data' array")

	if len(data) == 0 {
		t.Skip("No models available - skipping inference test")
	}

	// Get first model
	firstModel := data[0].(map[string]interface{})
	state.ModelID = getString(firstModel, "id")
	require.NotEmpty(t, state.ModelID, "Model ID should not be empty")

	t.Logf("Found %d models, using first: %s", len(data), state.ModelID)
}

// testSendInferenceRequest sends a chat completion request
func testSendInferenceRequest(t *testing.T, cfg *TestConfig, state *TestState) {
	require.NotEmpty(t, state.APISecret, "API key must be created first")
	require.NotEmpty(t, state.ModelID, "Model must be identified first")

	output, err := runCLIWithInferenceEndpoint(t, cfg, state.APISecret,
		"inference", "send-request",
		"--model", state.ModelID,
		"--max-tokens", "50",
		"What is 2+2? Reply with just the number.",
	)
	require.NoError(t, err, "Failed to send inference request")

	// Parse JSON response
	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Failed to parse inference response")

	response, ok := result["response"].(map[string]interface{})
	require.True(t, ok, "Response should have 'response' object")

	// Check for choices
	choices, ok := response["choices"].([]interface{})
	require.True(t, ok, "Response should have 'choices' array")
	require.Greater(t, len(choices), 0, "Should have at least one choice")

	// Get the response content
	firstChoice := choices[0].(map[string]interface{})
	message := firstChoice["message"].(map[string]interface{})
	content := getString(message, "content")
	require.NotEmpty(t, content, "Response content should not be empty")

	// Get usage info
	usage, ok := response["usage"].(map[string]interface{})
	require.True(t, ok, "Response should have 'usage' object")

	totalTokens := int(usage["total_tokens"].(float64))
	require.Greater(t, totalTokens, 0, "Should have used some tokens")

	// Store request ID for metrics verification
	if id, ok := response["id"].(string); ok {
		state.RequestID = id
	}

	t.Logf("Inference successful: tokens=%d, content=%q", totalTokens, truncate(content, 50))
}

// testVerifyMetrics checks that the inference request was logged
func testVerifyMetrics(t *testing.T, cfg *TestConfig, state *TestState) {
	require.NotEmpty(t, state.OrgSlug, "Organization must be created first")

	// Wait a bit for metrics to be aggregated
	t.Log("Waiting 5 seconds for metrics aggregation...")
	time.Sleep(5 * time.Second)

	// Query usage for today
	today := time.Now().Format("2006-01-02")

	// Use direct HTTP call since analytics endpoint might need special handling
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/usage?org_id=%s&start=%s&end=%s",
		cfg.AnalyticsEndpoint, state.OrgSlug, today, today)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.TLSInsecure,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Warning: Could not verify metrics - analytics service may not be available: %v", err)
		t.Skip("Analytics service not available for metrics verification")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable {
		t.Skip("Analytics service not available for metrics verification")
		return
	}

	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			t.Logf("Metrics response: %+v", result)

			// Check for invocations
			if totals, ok := result["totals"].(map[string]interface{}); ok {
				invocations := int(totals["invocations"].(float64))
				t.Logf("Found %d invocations in metrics for org %s", invocations, state.OrgSlug)
				assert.GreaterOrEqual(t, invocations, 1, "Should have at least 1 invocation logged")
			}
		}
	} else {
		t.Logf("Metrics check returned status %d - may need time to aggregate", resp.StatusCode)
	}
}

// cleanupTestOrg deletes the test organization and all associated resources
func cleanupTestOrg(t *testing.T, cfg *TestConfig, state *TestState) {
	if state.OrgSlug == "" {
		return
	}

	t.Logf("Cleaning up test organization: %s", state.OrgSlug)

	_, err := runCLI(t, cfg,
		"org", "delete",
		"--org-id", state.OrgSlug,
		"--force",
	)

	if err != nil {
		t.Logf("Warning: Failed to cleanup org %s: %v", state.OrgSlug, err)
	} else {
		t.Logf("Successfully cleaned up org: %s", state.OrgSlug)
	}
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
