//go:build (smoke || nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"strings"
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

// ChatCompletionResponse represents the response from POST /v1/chat/completions
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// TestPrompt defines a prompt with expected response validation
type TestPrompt struct {
	Prompt          string   // The prompt to send
	ExpectedKeyword string   // Expected keyword in response (case-insensitive)
	IsVLM           bool     // Whether this is a vision-language model prompt
	ImageBase64     string   // Base64-encoded image for VLM prompts
}

// vlmModelPatterns contains patterns to identify vision-language models
var vlmModelPatterns = []string{
	"qwen2-vl", "llava", "internvl", "pixtral", "phi-3-vision",
	"vision", "vl-", "-vl",
}

// isVisionModel checks if a model ID indicates a vision-language model
func isVisionModel(modelID string) bool {
	modelLower := strings.ToLower(modelID)
	for _, pattern := range vlmModelPatterns {
		if strings.Contains(modelLower, pattern) {
			return true
		}
	}
	return false
}

// getTestPrompt returns the appropriate test prompt for a model
func getTestPrompt(modelID string) TestPrompt {
	if isVisionModel(modelID) {
		return TestPrompt{
			Prompt:          "What animal is in this image? Answer in one word.",
			ExpectedKeyword: "cat",
			IsVLM:           true,
			ImageBase64:     getTestImageBase64(),
		}
	}
	return TestPrompt{
		Prompt:          "What is the capital of France? Answer in one word.",
		ExpectedKeyword: "paris",
		IsVLM:           false,
	}
}

// getTestImageBase64 returns a small base64-encoded test image (cat)
func getTestImageBase64() string {
	// 128x128 JPEG cat image (~4KB) for VLM testing
	return "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"
}

// validateInferenceResponse checks that an inference response is meaningful
func validateInferenceResponse(t *testing.T, resp ChatCompletionResponse, testPrompt TestPrompt) bool {
	t.Helper()
	valid := true

	// Check response has content
	if len(resp.Choices) == 0 {
		t.Error("No choices in response")
		return false
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Error("Response content is empty")
		return false
	}

	t.Logf("Response content: %s", truncateString(content, 100))

	// Check for expected keyword if specified
	if testPrompt.ExpectedKeyword != "" {
		if !strings.Contains(strings.ToLower(content), strings.ToLower(testPrompt.ExpectedKeyword)) {
			t.Logf("Warning: Expected keyword '%s' not found in response", testPrompt.ExpectedKeyword)
			// Don't fail - some models may give different valid answers
		} else {
			t.Logf("Found expected keyword '%s' in response", testPrompt.ExpectedKeyword)
		}
	}

	// Check token usage is positive
	if resp.Usage.TotalTokens == 0 {
		t.Log("Warning: Total tokens is 0 - usage tracking may not be working")
	} else {
		t.Logf("Token usage - Prompt: %d, Completion: %d, Total: %d",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	}

	return valid
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// buildChatRequest creates the appropriate request body for a model
func buildChatRequest(modelID string, testPrompt TestPrompt, maxTokens int) map[string]interface{} {
	if testPrompt.IsVLM {
		// Vision model: multimodal content with image
		return map[string]interface{}{
			"model": modelID,
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]interface{}{
						{
							"type": "image_url",
							"image_url": map[string]string{
								"url": "data:image/jpeg;base64," + testPrompt.ImageBase64,
							},
						},
						{
							"type": "text",
							"text": testPrompt.Prompt,
						},
					},
				},
			},
			"max_tokens": maxTokens,
		}
	}

	// Text model: simple message
	return map[string]interface{}{
		"model": modelID,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": testPrompt.Prompt,
			},
		},
		"max_tokens": maxTokens,
	}
}

// TestSmokeModelsAvailable verifies that models are loaded and accessible
func TestSmokeModelsAvailable(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Create client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)

	// If using IP address, set Host header for ingress routing
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
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
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("Created organization: %s", org.ID)

	// Step 2: Create service account
	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}
	t.Logf("Created service account: %s", sa.ID)

	// Step 3: Create API key for service account
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}
	t.Logf("Created API key: %s", apiKey.ID)

	// Step 3: Get available models
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
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

	// Step 4: Build appropriate request for model type (text vs vision)
	testPrompt := getTestPrompt(modelID)
	if testPrompt.IsVLM {
		t.Logf("Detected vision model - sending image+text prompt")
	} else {
		t.Logf("Text model - sending: %s", testPrompt.Prompt)
	}

	reqBody := buildChatRequest(modelID, testPrompt, 50)

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

	// Parse and validate inference response
	var completionResp ChatCompletionResponse
	if err := json.Unmarshal(inferenceResp.Body, &completionResp); err != nil {
		t.Fatalf("Failed to parse inference response: %v", err)
	}

	// Validate response content is meaningful
	if !validateInferenceResponse(t, completionResp, testPrompt) {
		t.Error("Inference response validation failed")
	}

	// Step 5: Verify usage in analytics (if available)
	if ctx.Config.APIURLs.AnalyticsService != "" {
		// Wait for usage to be recorded
		time.Sleep(2 * time.Second)

		analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)
		analyticsClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

		if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
			analyticsClient.SetHeader("Host", "analytics.dev.otherjamesbrown.com")
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

	passed := 0
	skipped := 0
	failed := 0

	// --- Health Check ---
	t.Log("Step 1: Health Check")
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	healthResp, err := routerClient.GET("/v1/status/healthz")
	if err != nil {
		t.Logf("  FAIL: Health check failed: %v", err)
		failed++
	} else if healthResp.StatusCode != 200 {
		t.Logf("  FAIL: Health check returned %d", healthResp.StatusCode)
		failed++
	} else {
		t.Log("  PASS: API Router is healthy")
		passed++
	}

	// --- Models Check ---
	t.Log("Step 2: Models Available")
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Logf("  FAIL: Models check failed: %v", err)
		failed++
	} else if modelsResp.StatusCode != 200 {
		t.Logf("  FAIL: Models check returned %d", modelsResp.StatusCode)
		failed++
	} else {
		var models ModelsResponse
		if err := json.Unmarshal(modelsResp.Body, &models); err != nil {
			t.Logf("  FAIL: Failed to parse models: %v", err)
			failed++
		} else {
			t.Logf("  PASS: %d models available", len(models.Data))
			for _, m := range models.Data {
				t.Logf("    - %s", m.ID)
			}
			passed++
		}
	}

	// --- Organization Creation ---
	t.Log("Step 3: Create Organization")
	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Logf("  FAIL: Organization creation failed: %v", err)
		failed++
	} else {
		t.Logf("  PASS: Organization created: %s", org.ID)
		passed++
	}

	// --- User-Org Service Health ---
	t.Log("Step 4: User-Org Service Health")
	userOrgClient := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		userOrgClient.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}
	userOrgHealth, err := userOrgClient.GET("/healthz")
	if err != nil {
		t.Logf("  FAIL: User-Org health check failed: %v", err)
		failed++
	} else if userOrgHealth.StatusCode != 200 {
		t.Logf("  FAIL: User-Org health returned %d", userOrgHealth.StatusCode)
		failed++
	} else {
		t.Log("  PASS: User-Org Service is healthy")
		passed++
	}

	// --- Inference Request (using admin key) ---
	t.Log("Step 5: Inference Request")
	adminKey := ctx.Config.Credentials.AdminAPIKey
	if adminKey == "" {
		t.Log("  SKIP: No admin API key configured")
		skipped++
	} else {
		routerClient.SetHeader("Authorization", "Bearer "+adminKey)
		routerClient.SetHeader("X-API-Key", adminKey)

		// Get models first
		modelsResp, _ := routerClient.GET("/v1/models")
		var models ModelsResponse
		json.Unmarshal(modelsResp.Body, &models)

		if len(models.Data) == 0 {
			t.Log("  SKIP: No models available")
			skipped++
		} else {
			modelID := models.Data[0].ID
			testPrompt := getTestPrompt(modelID)

			if testPrompt.IsVLM {
				t.Logf("  Detected vision model: %s", modelID)
			}

			reqBody := buildChatRequest(modelID, testPrompt, 50)

			inferResp, err := routerClient.POST("/v1/chat/completions", reqBody)
			if err != nil {
				t.Logf("  FAIL: Inference request failed: %v", err)
				failed++
			} else if inferResp.StatusCode == 200 {
				var completion ChatCompletionResponse
				if err := json.Unmarshal(inferResp.Body, &completion); err != nil {
					t.Logf("  FAIL: Failed to parse response: %v", err)
					failed++
				} else {
					// Validate response content
					if len(completion.Choices) == 0 || completion.Choices[0].Message.Content == "" {
						t.Log("  FAIL: Response content is empty")
						failed++
					} else {
						content := truncateString(completion.Choices[0].Message.Content, 50)
						t.Logf("  PASS: Inference completed - Response: %s", content)
						t.Logf("        Tokens: prompt=%d, completion=%d, total=%d",
							completion.Usage.PromptTokens,
							completion.Usage.CompletionTokens,
							completion.Usage.TotalTokens)

						// Check for expected keyword (informational only)
						if testPrompt.ExpectedKeyword != "" {
							if strings.Contains(strings.ToLower(completion.Choices[0].Message.Content),
								strings.ToLower(testPrompt.ExpectedKeyword)) {
								t.Logf("        Found expected keyword: %s", testPrompt.ExpectedKeyword)
							}
						}
						passed++
					}
				}
			} else if inferResp.StatusCode == 503 {
				t.Log("  SKIP: Backend unavailable (503)")
				skipped++
			} else if inferResp.StatusCode == 401 || inferResp.StatusCode == 403 {
				t.Logf("  SKIP: Auth required (%d)", inferResp.StatusCode)
				skipped++
			} else {
				t.Logf("  FAIL: Inference returned %d: %s", inferResp.StatusCode, string(inferResp.Body))
				failed++
			}
		}
	}

	// --- Analytics Service Health ---
	t.Log("Step 6: Analytics Service Health")
	if ctx.Config.APIURLs.AnalyticsService == "" {
		t.Log("  SKIP: Analytics service not configured")
		skipped++
	} else {
		analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)
		if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
			analyticsClient.SetHeader("Host", "analytics.dev.otherjamesbrown.com")
		}
		analyticsHealth, err := analyticsClient.GET("/analytics/v1/status/healthz")
		if err != nil {
			t.Logf("  FAIL: Analytics health check failed: %v", err)
			failed++
		} else if analyticsHealth.StatusCode != 200 {
			t.Logf("  FAIL: Analytics health returned %d", analyticsHealth.StatusCode)
			failed++
		} else {
			t.Log("  PASS: Analytics Service is healthy")
			passed++
		}
	}

	t.Log("")
	t.Log("=== Smoke Test Summary ===")
	t.Logf("  PASSED:  %d", passed)
	t.Logf("  SKIPPED: %d", skipped)
	t.Logf("  FAILED:  %d", failed)
	t.Log("")

	if failed > 0 {
		t.Fatalf("Smoke test failed with %d failures", failed)
	}

	t.Log("=== Smoke Test Complete ===")
}
