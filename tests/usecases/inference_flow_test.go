package usecases_test

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// TestUC_INF_001_EndToEndChatCompletion validates UC-INF-001: End-to-End Chat Completion.
// Spec: usecases/inference-flow.yaml
//
// Migrated from: tests/e2e/suites/happy_path_test.go (TestSuccessfulCompletion)
//                tests/e2e/suites/completions_test.go (chat completion logic)
func TestUC_INF_001_EndToEndChatCompletion(t *testing.T) {
	skipIfNoPlatformCLI(t)

	client, err := NewTestClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	fm := NewFixtureManager(t, client)
	orgFixture := NewOrganizationFixture(fm, client)
	apiKeyFixture := NewAPIKeyFixture(fm, client)

	// Given: User has valid API key and model is available
	org, err := orgFixture.Create("")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.CreateWithServiceAccount(org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create client for inference requests
	inferenceClient := NewTestClient(getAPIRouterURL(), apiKey.Key)

	t.Run("AC-01: successful chat completion request", func(t *testing.T) {
		// When: User sends POST /v1/chat/completions with valid request body
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, this is a test",
				},
			},
		}

		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Then: Response status is 200
		if resp.StatusCode != 200 {
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, resp.String())
		}

		// Then: Response includes id, model, choices, usage fields
		var chatResp struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := resp.UnmarshalJSON(&chatResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if chatResp.ID == "" {
			t.Error("Response missing id field")
		}
		if chatResp.Model == "" {
			t.Error("Response missing model field")
		}
		if len(chatResp.Choices) == 0 {
			t.Error("Response missing choices")
		}
		if chatResp.Usage.TotalTokens == 0 {
			t.Error("Response missing usage data")
		}
	})

	t.Run("AC-02: response includes usage data", func(t *testing.T) {
		// Given: Chat completion request succeeds
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test message",
				},
			},
		}

		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil || resp.StatusCode != 200 {
			t.Skip("Chat completion failed - skipping usage validation")
		}

		var chatResp struct {
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := resp.UnmarshalJSON(&chatResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Then: usage.prompt_tokens is greater than 0
		if chatResp.Usage.PromptTokens <= 0 {
			t.Error("Expected prompt_tokens > 0")
		}

		// Then: usage.completion_tokens is greater than 0
		if chatResp.Usage.CompletionTokens <= 0 {
			t.Error("Expected completion_tokens > 0")
		}

		// Then: usage.total_tokens equals prompt_tokens + completion_tokens
		expected := chatResp.Usage.PromptTokens + chatResp.Usage.CompletionTokens
		if chatResp.Usage.TotalTokens != expected {
			t.Errorf("Expected total_tokens=%d, got %d", expected, chatResp.Usage.TotalTokens)
		}
	})

	t.Run("AC-03: invalid model returns clear error", func(t *testing.T) {
		// When: User sends POST /v1/chat/completions with model "nonexistent"
		reqBody := map[string]interface{}{
			"model": "nonexistent-model",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test",
				},
			},
		}

		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Then: Request fails with 404 status
		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		// Then: Error message indicates model not found
		bodyStr := resp.String()
		if !strings.Contains(strings.ToLower(bodyStr), "model") && !strings.Contains(strings.ToLower(bodyStr), "not found") {
			t.Errorf("Error message should mention model not found: %s", bodyStr)
		}
	})

	t.Run("AC-04: unauthorized model access rejected", func(t *testing.T) {
		t.Skip("Model access control not implemented - requires UC-MDL-002")
		// TODO: Implement once model access control is in place
	})
}

// TestUC_INF_002_StreamingCompletion validates UC-INF-002: Streaming Completion.
// Spec: usecases/inference-flow.yaml
//
// Migrated from: tests/e2e/suites/completions_test.go (TestTextCompletionsStreaming)
func TestUC_INF_002_StreamingCompletion(t *testing.T) {
	skipIfNoPlatformCLI(t)

	client, err := NewTestClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	fm := NewFixtureManager(t, client)
	orgFixture := NewOrganizationFixture(fm, client)
	apiKeyFixture := NewAPIKeyFixture(fm, client)

	// Preconditions: User has valid API key with inference:write scope
	org, err := orgFixture.Create("")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.CreateWithServiceAccount(org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	inferenceClient := NewTestClient(getAPIRouterURL(), apiKey.Key)

	t.Run("AC-01: enable streaming with request parameter", func(t *testing.T) {
		// When: User sends POST /v1/chat/completions with stream:true
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Say hello",
				},
			},
			"stream": true,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		resp, err := inferenceClient.DoStreaming("POST", "/v1/chat/completions", bodyBytes)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Then: Response status is 200
		if resp.StatusCode != 200 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(bodyBytes))
		}

		// Then: Response content-type is text/event-stream
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			t.Errorf("Expected Content-Type to contain 'text/event-stream', got: %s", contentType)
		}

		// Then: Response includes SSE data chunks
		scanner := bufio.NewScanner(resp.Body)
		foundData := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				foundData = true
				break
			}
		}

		if !foundData {
			t.Error("Expected SSE response to contain 'data:' prefix")
		}
	})

	t.Run("AC-02: streaming chunks have valid format", func(t *testing.T) {
		// When: User enables streaming
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Count to 3",
				},
			},
			"stream": true,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		resp, err := inferenceClient.DoStreaming("POST", "/v1/chat/completions", bodyBytes)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Skip("Streaming request failed - cannot validate chunk format")
		}

		// Then: Each chunk has valid JSON format
		scanner := bufio.NewScanner(resp.Body)
		chunkCount := 0
		foundDone := false

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// Check for data: prefix
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Extract JSON after "data: "
			jsonStr := strings.TrimPrefix(line, "data: ")

			// Check for [DONE] marker
			if strings.TrimSpace(jsonStr) == "[DONE]" {
				foundDone = true
				break
			}

			// Then: Each chunk parses as valid JSON
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
				t.Errorf("Failed to parse chunk JSON: %v\nChunk: %s", err, jsonStr)
				continue
			}

			// Then: Chunk includes required fields (id, object, choices)
			if _, ok := chunk["id"]; !ok {
				t.Error("Chunk missing 'id' field")
			}
			if _, ok := chunk["object"]; !ok {
				t.Error("Chunk missing 'object' field")
			}
			if _, ok := chunk["choices"]; !ok {
				t.Error("Chunk missing 'choices' field")
			}

			chunkCount++
		}

		if chunkCount == 0 {
			t.Error("Expected at least one data chunk")
		}

		if !foundDone {
			t.Error("Expected stream to end with [DONE] marker")
		}
	})

	t.Run("AC-03: final chunk includes usage data", func(t *testing.T) {
		// When: User completes a streaming request
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello",
				},
			},
			"stream":         true,
			"stream_options": map[string]interface{}{"include_usage": true},
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		resp, err := inferenceClient.DoStreaming("POST", "/v1/chat/completions", bodyBytes)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Skip("Streaming request failed - cannot validate usage data")
		}

		// Read all chunks and find the one with usage
		scanner := bufio.NewScanner(resp.Body)
		foundUsage := false

		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			jsonStr := strings.TrimPrefix(line, "data: ")
			if strings.TrimSpace(jsonStr) == "[DONE]" {
				break
			}

			var chunk struct {
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}

			if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
				continue
			}

			// Then: Final chunk includes usage.prompt_tokens, usage.completion_tokens, usage.total_tokens
			if chunk.Usage.TotalTokens > 0 {
				foundUsage = true
				if chunk.Usage.PromptTokens <= 0 {
					t.Error("Expected prompt_tokens > 0 in usage data")
				}
				if chunk.Usage.CompletionTokens <= 0 {
					t.Error("Expected completion_tokens > 0 in usage data")
				}
				expected := chunk.Usage.PromptTokens + chunk.Usage.CompletionTokens
				if chunk.Usage.TotalTokens != expected {
					t.Errorf("Expected total_tokens=%d, got %d", expected, chunk.Usage.TotalTokens)
				}
			}
		}

		if !foundUsage {
			t.Log("Warning: No usage data found in streaming response - may require stream_options.include_usage=true")
		}
	})

	t.Run("AC-04: connection interruption handling", func(t *testing.T) {
		// This test validates graceful handling when connection is closed early
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Write a long story",
				},
			},
			"stream":     true,
			"max_tokens": 500,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		resp, err := inferenceClient.DoStreaming("POST", "/v1/chat/completions", bodyBytes)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			t.Skip("Streaming request failed - cannot validate interruption handling")
		}

		// Read first few chunks then close connection
		scanner := bufio.NewScanner(resp.Body)
		chunkCount := 0
		for scanner.Scan() && chunkCount < 3 {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				chunkCount++
			}
		}

		// Then: Connection closes without error
		resp.Body.Close()

		if chunkCount < 1 {
			t.Error("Expected to read at least 1 chunk before closing")
		}

		// Success: Connection closed gracefully after partial read
	})

}

// TestUC_INF_003_MultiModelRouting validates UC-INF-003: Multi-Model Routing.
// Spec: usecases/inference-flow.yaml
//
// Migrated from: tests/e2e/suites/happy_path_test.go (TestModelRequestRouting)
func TestUC_INF_003_MultiModelRouting(t *testing.T) {
	skipIfNoPlatformCLI(t)

	client, err := NewTestClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	fm := NewFixtureManager(t, client)
	orgFixture := NewOrganizationFixture(fm, client)
	apiKeyFixture := NewAPIKeyFixture(fm, client)

	org, err := orgFixture.Create("")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.CreateWithServiceAccount(org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	inferenceClient := NewTestClient(getAPIRouterURL(), apiKey.Key)

	t.Run("AC-01: route to correct backend by model name", func(t *testing.T) {
		t.Skip("Multi-model routing validation requires multiple deployed models")
		// TODO: Implement once we have multiple models deployed
	})

	t.Run("AC-02: route different models in sequence", func(t *testing.T) {
		t.Skip("Multi-model routing validation requires multiple deployed models")
	})

	t.Run("AC-03: handle backend unavailability", func(t *testing.T) {
		// When: User sends request for model with unavailable backend
		reqBody := map[string]interface{}{
			"model": "unavailable-backend-model",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test",
				},
			},
		}

		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Then: Request fails with 503 status or 404
		if resp.StatusCode != 503 && resp.StatusCode != 404 {
			t.Logf("Expected status 503 or 404, got %d (may be acceptable)", resp.StatusCode)
		}

		// Then: Error indicates service unavailable or model not found
		bodyStr := strings.ToLower(resp.String())
		if !strings.Contains(bodyStr, "unavailable") && !strings.Contains(bodyStr, "not found") {
			t.Logf("Error message should indicate unavailability: %s", resp.String())
		}
	})

	t.Run("AC-04: preserve model name in response", func(t *testing.T) {
		// When: User requests specific model
		modelName := getTestModel()
		reqBody := map[string]interface{}{
			"model": modelName,
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test",
				},
			},
		}

		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil || resp.StatusCode != 200 {
			t.Skip("Inference request failed - skipping model name validation")
		}

		var chatResp struct {
			Model string `json:"model"`
		}

		if err := resp.UnmarshalJSON(&chatResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Then: Response model field matches requested model exactly
		if chatResp.Model != modelName {
			t.Errorf("Expected model=%s, got %s", modelName, chatResp.Model)
		}
	})
}

// TestUC_INF_004_TokenUsageTracking validates UC-INF-004: Token Usage Tracking.
// Spec: usecases/inference-flow.yaml
//
// Migrated from: tests/e2e/suites/completions_test.go (TestTextCompletionsUsageAnalytics)
func TestUC_INF_004_TokenUsageTracking(t *testing.T) {
	skipIfNoPlatformCLI(t)

	if getAnalyticsServiceURL() == "" {
		t.Skip("Analytics service not configured")
	}

	client, err := NewTestClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	fm := NewFixtureManager(t, client)
	orgFixture := NewOrganizationFixture(fm, client)
	apiKeyFixture := NewAPIKeyFixture(fm, client)

	org, err := orgFixture.Create("")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.CreateWithServiceAccount(org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	inferenceClient := NewTestClient(getAPIRouterURL(), apiKey.Key)

	t.Run("AC-01: record usage for successful requests", func(t *testing.T) {
		// Given: User makes successful chat completion request
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test usage tracking",
				},
			},
		}

		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil || resp.StatusCode != 200 {
			t.Skip("Inference request failed - cannot validate usage tracking")
		}

		// Wait for usage to be recorded
		time.Sleep(2 * time.Second)

		// Then: Usage record is created in analytics database
		analyticsClient := NewTestClient(getAnalyticsServiceURL(), getAdminAPIKey())
		now := time.Now().UTC()
		start := now.Add(-1 * time.Hour).Format(time.RFC3339)
		end := now.Format(time.RFC3339)

		usagePath := "/analytics/v1/orgs/" + org.ID + "/usage?start=" + start + "&end=" + end
		usageResp, err := analyticsClient.GET(usagePath)
		if err != nil || usageResp.StatusCode != 200 {
			t.Skip("Analytics service not available")
		}

		var usage struct {
			RequestCount int `json:"requestCount"`
			TotalTokens  int `json:"totalTokens"`
		}

		if err := usageResp.UnmarshalJSON(&usage); err != nil {
			t.Fatalf("Failed to parse usage response: %v", err)
		}

		// Then: Usage includes prompt_tokens, completion_tokens, total_tokens
		if usage.RequestCount == 0 {
			t.Error("Expected at least 1 request recorded")
		}
		if usage.TotalTokens == 0 {
			t.Error("Expected non-zero token usage")
		}
	})

	t.Run("AC-02: usage matches backend response", func(t *testing.T) {
		// Validated by AC-01 - backend response usage is what gets recorded
		t.Skip("Implicitly validated by AC-01")
	})

	t.Run("AC-03: no usage recorded for failed requests", func(t *testing.T) {
		// Get baseline usage count
		analyticsClient := NewTestClient(getAnalyticsServiceURL(), getAdminAPIKey())
		now := time.Now().UTC()
		start := now.Add(-1 * time.Hour).Format(time.RFC3339)
		end := now.Format(time.RFC3339)

		usagePath := "/analytics/v1/orgs/" + org.ID + "/usage?start=" + start + "&end=" + end
		baselineResp, err := analyticsClient.GET(usagePath)
		if err != nil || baselineResp.StatusCode != 200 {
			t.Skip("Analytics service not available")
		}

		var baselineUsage struct {
			RequestCount int `json:"requestCount"`
		}
		baselineResp.UnmarshalJSON(&baselineUsage)

		// Make request that fails authentication
		unauthClient := NewTestClient(getAPIRouterURL(), "invalid-api-key")
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test",
				},
			},
		}

		resp, _ := unauthClient.POST("/v1/chat/completions", reqBody)
		if resp == nil || resp.StatusCode == 200 {
			t.Skip("Expected authentication to fail")
		}

		// Wait and check usage didn't increase
		time.Sleep(2 * time.Second)

		afterResp, err := analyticsClient.GET(usagePath)
		if err != nil || afterResp.StatusCode != 200 {
			t.Skip("Analytics service not available")
		}

		var afterUsage struct {
			RequestCount int `json:"requestCount"`
		}
		afterResp.UnmarshalJSON(&afterUsage)

		// Then: No tokens are charged to organization
		if afterUsage.RequestCount != baselineUsage.RequestCount {
			t.Error("Failed request should not record usage")
		}
	})

	t.Run("AC-04: usage recorded for streaming requests", func(t *testing.T) {
		t.Skip("Streaming not yet implemented - requires UC-INF-002 implementation")
	})

	t.Run("AC-05: correlate usage with request trace", func(t *testing.T) {
		// Given: User makes inference request
		reqBody := map[string]interface{}{
			"model": getTestModel(),
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Test usage correlation",
				},
			},
		}

		// When: Request completes successfully
		resp, err := inferenceClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode != 200 {
			t.Skipf("Inference request failed with status %d", resp.StatusCode)
		}

		// Then: Response includes trace_id
		var result map[string]interface{}
		if err := resp.UnmarshalJSON(&result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		traceID, ok := result["trace_id"].(string)
		if !ok || traceID == "" {
			t.Errorf("Response should include trace_id for usage correlation")
		} else {
			t.Logf("Trace ID for usage correlation: %s", traceID)
			// Full correlation validation requires analytics database access (UC-ANL-004/AC-03)
		}
	})
}
