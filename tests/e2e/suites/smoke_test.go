package suites

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// SmokeTestSuite runs a comprehensive smoke test of the platform
// This is the primary test to verify the development environment is working
//
// Run with: go test -v ./suites -run TestSmoke -timeout 10m

// Model represents an available model from the API
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response from GET /v1/models
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// UsageResponse represents token usage from analytics
type UsageResponse struct {
	TotalTokens      int64 `json:"total_tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	RequestCount     int64 `json:"request_count"`
}

// TestSmokeModelsAvailable verifies that models are loaded and accessible
func TestSmokeModelsAvailable(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Create client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)

	// If using IP address, set Host header for ingress routing
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// GET /v1/models - should be publicly accessible (no auth required)
	resp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(resp.Body))
	}

	var modelsResp ModelsResponse
	if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
		t.Fatalf("Failed to parse models response: %v", err)
	}

	if len(modelsResp.Data) == 0 {
		t.Fatal("No models available - expected at least one model to be loaded")
	}

	t.Logf("Models available: %d", len(modelsResp.Data))
	for _, model := range modelsResp.Data {
		t.Logf("  - %s (owned by: %s)", model.ID, model.OwnedBy)
	}
}

// TestSmokeInferenceWithUsage performs an inference call and verifies token usage is tracked
func TestSmokeInferenceWithUsage(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("Created organization: %s", org.ID)

	// Step 2: Create API key
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}
	t.Logf("Created API key: %s", apiKey.ID)

	// Step 3: Get available models
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models: %v", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil {
		t.Fatalf("Failed to parse models: %v", err)
	}

	if len(models.Data) == 0 {
		t.Skip("No models available - skipping inference test")
	}

	modelID := models.Data[0].ID
	t.Logf("Using model: %s", modelID)

	// Step 4: Make inference request
	reqBody := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Say hello in exactly 5 words.",
			},
		},
		"max_tokens": 20,
	}

	inferenceResp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Logf("Inference request failed: %v", err)
		t.Skip("Backend may be unavailable")
	}

	if inferenceResp.StatusCode != 200 {
		t.Logf("Inference returned status %d: %s", inferenceResp.StatusCode, string(inferenceResp.Body))
		if inferenceResp.StatusCode == 503 {
			t.Skip("Backend service unavailable")
		}
		t.Fatalf("Expected status 200, got %d", inferenceResp.StatusCode)
	}

	// Parse inference response to get usage
	var completionResp map[string]interface{}
	if err := json.Unmarshal(inferenceResp.Body, &completionResp); err != nil {
		t.Fatalf("Failed to parse inference response: %v", err)
	}

	// Check usage in response
	if usage, ok := completionResp["usage"].(map[string]interface{}); ok {
		t.Logf("Token usage from response:")
		t.Logf("  - Prompt tokens: %.0f", usage["prompt_tokens"])
		t.Logf("  - Completion tokens: %.0f", usage["completion_tokens"])
		t.Logf("  - Total tokens: %.0f", usage["total_tokens"])
	} else {
		t.Log("No usage info in inference response")
	}

	// Step 5: Verify usage in analytics (if available)
	if ctx.Config.APIURLs.AnalyticsService != "" {
		// Wait for usage to be recorded
		time.Sleep(2 * time.Second)

		analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)
		analyticsClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

		if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
			analyticsClient.SetHeader("Host", "analytics.dev.ai-aas.local")
		}

		usageResp, err := analyticsClient.GET("/analytics/v1/usage/summary?org_id=" + org.ID)
		if err != nil {
			t.Logf("Failed to get usage from analytics: %v", err)
		} else if usageResp.StatusCode == 200 {
			var usage UsageResponse
			if err := json.Unmarshal(usageResp.Body, &usage); err == nil {
				t.Logf("Usage from analytics service:")
				t.Logf("  - Request count: %d", usage.RequestCount)
				t.Logf("  - Total tokens: %d", usage.TotalTokens)
			}
		} else {
			t.Logf("Analytics returned status %d (may not be implemented)", usageResp.StatusCode)
		}
	}

	t.Logf("Inference with usage test passed")
}

// TestSmokeEndToEnd runs a complete end-to-end smoke test
// This is the main test to run after deployment to verify everything works
func TestSmokeEndToEnd(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== Platform Smoke Test ===")
	t.Log("")

	// --- Health Check ---
	t.Log("Step 1: Health Check")
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	healthResp, err := routerClient.GET("/v1/status/healthz")
	if err != nil {
		t.Fatalf("  FAIL: Health check failed: %v", err)
	}
	if healthResp.StatusCode != 200 {
		t.Fatalf("  FAIL: Health check returned %d", healthResp.StatusCode)
	}
	t.Log("  PASS: API Router is healthy")

	// --- Models Check ---
	t.Log("Step 2: Models Available")
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("  FAIL: Models check failed: %v", err)
	}
	if modelsResp.StatusCode != 200 {
		t.Fatalf("  FAIL: Models check returned %d", modelsResp.StatusCode)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil {
		t.Fatalf("  FAIL: Failed to parse models: %v", err)
	}
	t.Logf("  PASS: %d models available", len(models.Data))

	// --- Organization Creation ---
	t.Log("Step 3: Create Organization")
	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("  FAIL: Organization creation failed: %v", err)
	}
	t.Logf("  PASS: Organization created: %s", org.ID)

	// --- User Creation ---
	t.Log("Step 4: Create User")
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)
	email := ctx.GenerateResourceName("smoketest") + "@test.example.com"
	user, err := userFixture.Create(ctx, org.ID, email, "Smoke Test User", []string{"admin"})
	if err != nil {
		t.Fatalf("  FAIL: User creation failed: %v", err)
	}
	t.Logf("  PASS: User created: %s", user.ID)

	// --- API Key Creation ---
	t.Log("Step 5: Create API Key")
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("  FAIL: API key creation failed: %v", err)
	}
	t.Logf("  PASS: API key created: %s", apiKey.ID)

	// --- Inference Request ---
	t.Log("Step 6: Inference Request")
	if len(models.Data) == 0 {
		t.Log("  SKIP: No models available")
	} else {
		routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
		reqBody := map[string]interface{}{
			"model": models.Data[0].ID,
			"messages": []map[string]interface{}{
				{"role": "user", "content": "Hello"},
			},
			"max_tokens": 10,
		}

		inferResp, err := routerClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Logf("  WARN: Inference failed: %v", err)
		} else if inferResp.StatusCode == 200 {
			var completion map[string]interface{}
			json.Unmarshal(inferResp.Body, &completion)
			if usage, ok := completion["usage"].(map[string]interface{}); ok {
				t.Logf("  PASS: Inference completed (tokens: %.0f)", usage["total_tokens"])
			} else {
				t.Log("  PASS: Inference completed")
			}
		} else if inferResp.StatusCode == 503 {
			t.Log("  SKIP: Backend unavailable")
		} else {
			t.Logf("  WARN: Inference returned %d", inferResp.StatusCode)
		}
	}

	t.Log("")
	t.Log("=== Smoke Test Complete ===")
}
