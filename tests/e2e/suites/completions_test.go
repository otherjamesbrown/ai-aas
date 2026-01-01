//go:build (smoke || nightly || full) && e2e_tier || !e2e_tier

package suites

// DEPRECATED: Integration tests in this file have been migrated to UC structure.
// See tests/usecases/inference_flow_test.go for:
//   - TestUC_INF_001_EndToEndChatCompletion (includes chat completions)
//   - TestUC_INF_002_StreamingCompletion (replaces TestTextCompletionsStreaming)
//   - TestUC_INF_004_TokenUsageTracking (replaces TestTextCompletionsUsageAnalytics)
//
// These E2E tests remain for backwards compatibility but should not be extended.
// All new integration tests should be added to tests/usecases/ following UC specs.

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// CompletionResponse represents the response from POST /v1/completions
type CompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Text         string `json:"text"`
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// TestTextCompletions tests the text completions endpoint (non-chat)
func TestTextCompletions(t *testing.T) {
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

	// Step 4: Get available models
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
		t.Skip("No models available - skipping text completions test")
	}

	// Find a text completion model (prefer gpt-oss-20b if available)
	modelID := ""
	for _, model := range models.Data {
		if strings.Contains(strings.ToLower(model.ID), "gpt-oss-20b") {
			modelID = model.ID
			break
		}
	}
	// Fallback to first model if gpt-oss-20b not found
	if modelID == "" {
		modelID = models.Data[0].ID
	}

	t.Logf("Using model for text completions: %s", modelID)

	// Step 5: Send text completion request
	reqBody := map[string]interface{}{
		"model":       modelID,
		"prompt":      "The capital of France is",
		"max_tokens":  50,
		"temperature": 0.7,
	}

	inferenceResp, err := routerClient.POST("/v1/completions", reqBody)
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

	// Step 6: Parse and validate response
	var completionResp CompletionResponse
	if err := json.Unmarshal(inferenceResp.Body, &completionResp); err != nil {
		t.Fatalf("Failed to parse inference response: %v", err)
	}

	// Validate response format
	if completionResp.Object != "text_completion" {
		t.Errorf("Expected object type 'text_completion', got '%s'", completionResp.Object)
	}

	if len(completionResp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	text := completionResp.Choices[0].Text
	if text == "" {
		t.Error("Response text is empty")
	}

	t.Logf("Completion text: %s", truncateString(text, 100))

	// Validate token usage is reported
	if completionResp.Usage.TotalTokens == 0 {
		t.Log("Warning: Total tokens is 0 - usage tracking may not be working")
	} else {
		t.Logf("Token usage - Prompt: %d, Completion: %d, Total: %d",
			completionResp.Usage.PromptTokens,
			completionResp.Usage.CompletionTokens,
			completionResp.Usage.TotalTokens)
	}

	// Check for expected keyword (Paris) - informational only
	if strings.Contains(strings.ToLower(text), "paris") {
		t.Logf("Found expected keyword 'Paris' in response")
	}

	t.Log("Text completions test passed")
}

// TestTextCompletionsStreaming tests the text completions endpoint with streaming
func TestTextCompletionsStreaming(t *testing.T) {
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

	// Step 4: Get available models
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
		t.Skip("No models available - skipping streaming text completions test")
	}

	// Find a text completion model (prefer gpt-oss-20b if available)
	modelID := ""
	for _, model := range models.Data {
		if strings.Contains(strings.ToLower(model.ID), "gpt-oss-20b") {
			modelID = model.ID
			break
		}
	}
	// Fallback to first model if gpt-oss-20b not found
	if modelID == "" {
		modelID = models.Data[0].ID
	}

	t.Logf("Using model for streaming text completions: %s", modelID)

	// Step 5: Send streaming text completion request
	reqBody := map[string]interface{}{
		"model":       modelID,
		"prompt":      "The capital of France is",
		"max_tokens":  50,
		"temperature": 0.7,
		"stream":      true,
	}

	resp, err := routerClient.POSTStream("/v1/completions", reqBody)
	if err != nil {
		t.Logf("Streaming request failed: %v", err)
		t.Skip("Backend may be unavailable")
	}
	defer resp.Close()

	// Read the streaming body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read streaming response: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Logf("Streaming returned status %d: %s", resp.StatusCode, string(bodyBytes))
		if resp.StatusCode == 503 {
			t.Skip("Backend service unavailable")
		}
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 6: Validate SSE response format
	if !strings.HasPrefix(resp.Headers.Get("Content-Type"), "text/event-stream") {
		t.Errorf("Expected Content-Type to start with 'text/event-stream', got '%s'",
			resp.Headers.Get("Content-Type"))
	}

	// Parse SSE chunks
	body := string(bodyBytes)
	chunks := strings.Split(body, "\n\n")

	var totalText string
	chunkCount := 0

	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" || chunk == "data: [DONE]" {
			continue
		}

		// Parse SSE data line
		if !strings.HasPrefix(chunk, "data: ") {
			continue
		}

		jsonData := strings.TrimPrefix(chunk, "data: ")

		var chunkResp CompletionResponse
		if err := json.Unmarshal([]byte(jsonData), &chunkResp); err != nil {
			t.Logf("Failed to parse chunk: %v", err)
			continue
		}

		if len(chunkResp.Choices) > 0 {
			totalText += chunkResp.Choices[0].Text
			chunkCount++
		}
	}

	if chunkCount == 0 {
		t.Error("No streaming chunks received")
	}

	if totalText == "" {
		t.Error("No text received from streaming response")
	}

	t.Logf("Received %d streaming chunks", chunkCount)
	t.Logf("Total streamed text: %s", truncateString(totalText, 100))

	// Check for expected keyword (Paris) - informational only
	if strings.Contains(strings.ToLower(totalText), "paris") {
		t.Logf("Found expected keyword 'Paris' in streamed response")
	}

	t.Log("Streaming text completions test passed")
}

// TestTextCompletionsUsageAnalytics verifies usage is tracked in analytics for text completions
func TestTextCompletionsUsageAnalytics(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	if ctx.Config.APIURLs.AnalyticsService == "" {
		t.Skip("Analytics service not configured")
	}

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create org and API key
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Send text completion request
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Get models
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models: %v", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil {
		t.Fatalf("Failed to parse models: %v", err)
	}

	if len(models.Data) == 0 {
		t.Skip("No models available")
	}

	modelID := models.Data[0].ID

	// Send completion request
	reqBody := map[string]interface{}{
		"model":       modelID,
		"prompt":      "The capital of France is",
		"max_tokens":  50,
		"temperature": 0.7,
	}

	inferenceResp, err := routerClient.POST("/v1/completions", reqBody)
	if err != nil || inferenceResp.StatusCode != 200 {
		t.Skip("Inference request failed")
	}

	var completionResp CompletionResponse
	if err := json.Unmarshal(inferenceResp.Body, &completionResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Wait for usage to be recorded
	time.Sleep(2 * time.Second)

	// Query analytics
	analyticsClient := harness.NewClient(ctx.Config.APIURLs.AnalyticsService, ctx.Config.Timeouts.RequestTimeout)
	analyticsClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	analyticsClient.SetHeader("X-API-Key", apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.AnalyticsService) {
		analyticsClient.SetHeader("Host", "analytics.dev.otherjamesbrown.com")
	}

	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour).Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	usagePath := "/analytics/v1/orgs/" + org.ID + "/usage?start=" + start + "&end=" + end

	usageResp, err := analyticsClient.GET(usagePath)
	if err != nil {
		t.Fatalf("Failed to get usage from analytics: %v", err)
	}

	if usageResp.StatusCode != 200 {
		t.Logf("Analytics returned status %d: %s", usageResp.StatusCode, string(usageResp.Body))
		t.Skip("Analytics not available")
	}

	var usage UsageResponse
	if err := json.Unmarshal(usageResp.Body, &usage); err != nil {
		t.Fatalf("Failed to parse usage: %v", err)
	}

	t.Logf("Usage from analytics service:")
	t.Logf("  - Request count: %d", usage.RequestCount)
	t.Logf("  - Total tokens: %d", usage.TotalTokens)

	if usage.RequestCount == 0 {
		t.Error("Expected at least 1 request in analytics")
	}

	if usage.TotalTokens == 0 {
		t.Error("Expected non-zero token usage in analytics")
	}

	t.Log("Text completions usage analytics test passed")
}
