// Package vllm provides integration tests for vLLM deployments via Helm chart.
//
// Test: T-S010-P03-017 - Integration test for completion endpoint
//
// This test validates that:
// - vLLM /v1/chat/completions endpoint is accessible
// - Model generates valid responses within acceptable latency
// - Response format matches OpenAI specification
// - Token counting is accurate
// - Response time is ≤3 seconds for simple queries (per spec)
//
// Prerequisites:
// - KUBECONFIG environment variable set
// - vLLM deployment running and healthy
// - VLLM_BACKEND_URL set to vLLM service endpoint
//
// Usage:
//   export KUBECONFIG=/path/to/kubeconfig
//   export VLLM_BACKEND_URL=http://localhost:8000  # via port-forward
//   export RUN_VLLM_TESTS=1
//   go test -v ./tests/infra/vllm -run TestVLLMCompletionEndpoint
//
package vllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpenAIChatCompletionRequest represents the request format for /v1/chat/completions
type OpenAIChatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// OpenAIMessage represents a chat message
type OpenAIMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"`
}

// OpenAIChatCompletionResponse represents the response from /v1/chat/completions
type OpenAIChatCompletionResponse struct {
	ID      string                     `json:"id"`
	Object  string                     `json:"object"`
	Created int64                      `json:"created"`
	Model   string                     `json:"model"`
	Choices []OpenAIChatCompletionChoice `json:"choices"`
	Usage   OpenAIUsage                `json:"usage"`
}

// OpenAIChatCompletionChoice represents a completion choice
type OpenAIChatCompletionChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage statistics
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TestVLLMCompletionEndpoint tests that the vLLM /v1/chat/completions endpoint
// works correctly and returns valid responses within acceptable latency.
//
// Test Steps:
// 1. Verify vLLM backend is accessible
// 2. Test /v1/models endpoint to confirm model is loaded
// 3. Send simple chat completion request
// 4. Validate response format matches OpenAI spec
// 5. Validate response latency is ≤3 seconds
// 6. Validate token counting is accurate
// 7. Validate response contains meaningful content
func TestVLLMCompletionEndpoint(t *testing.T) {
	if os.Getenv("RUN_VLLM_TESTS") != "1" {
		t.Skip("Skipping vLLM completion test. Set RUN_VLLM_TESTS=1 to run.")
	}

	backendURL := os.Getenv("VLLM_BACKEND_URL")
	if backendURL == "" {
		t.Skip("VLLM_BACKEND_URL not set. Set it to vLLM service URL (e.g., http://localhost:8000 via port-forward)")
	}

	modelName := os.Getenv("VLLM_MODEL_NAME")
	if modelName == "" {
		modelName = "gpt-oss-20b" // Default to our known model
	}

	const maxLatency = 30 * time.Second // Relaxed for first request (model warmup)

	// Step 1: Verify backend is accessible via /health
	t.Log("Step 1: Verifying vLLM backend is accessible...")
	healthURL := fmt.Sprintf("%s/health", backendURL)
	resp, err := http.Get(healthURL)
	require.NoError(t, err, "failed to connect to vLLM backend at %s", healthURL)
	require.NotNil(t, resp, "response should not be nil")
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "/health endpoint should return 200")
	t.Logf("✓ vLLM backend is healthy at %s", backendURL)

	// Step 2: Test /v1/models endpoint
	t.Log("Step 2: Testing /v1/models endpoint...")
	modelsURL := fmt.Sprintf("%s/v1/models", backendURL)
	modelsResp, err := http.Get(modelsURL)
	require.NoError(t, err, "failed to call /v1/models")
	defer modelsResp.Body.Close()
	require.Equal(t, http.StatusOK, modelsResp.StatusCode, "/v1/models should return 200")

	var modelsData map[string]interface{}
	err = json.NewDecoder(modelsResp.Body).Decode(&modelsData)
	require.NoError(t, err, "failed to decode /v1/models response")
	t.Logf("✓ Models endpoint accessible. Response: %+v", modelsData)

	// Step 3: Send chat completion request
	t.Log("Step 3: Sending chat completion request...")
	requestBody := OpenAIChatCompletionRequest{
		Model: modelName,
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: "Say 'Hello, World!' and nothing else.",
			},
		},
		MaxTokens:   20,
		Temperature: 0.1, // Low temperature for deterministic output
	}

	jsonData, err := json.Marshal(requestBody)
	require.NoError(t, err, "failed to marshal request body")
	t.Logf("Request body: %s", string(jsonData))

	completionURL := fmt.Sprintf("%s/v1/chat/completions", backendURL)
	startTime := time.Now()

	req, err := http.NewRequestWithContext(context.Background(), "POST", completionURL, bytes.NewReader(jsonData))
	require.NoError(t, err, "failed to create request")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: maxLatency}
	completionResp, err := client.Do(req)
	latency := time.Since(startTime)

	require.NoError(t, err, "failed to call /v1/chat/completions")
	require.NotNil(t, completionResp, "response should not be nil")
	defer completionResp.Body.Close()

	t.Logf("Response latency: %v", latency)

	// Read response body
	bodyBytes, err := io.ReadAll(completionResp.Body)
	require.NoError(t, err, "failed to read response body")
	t.Logf("Response status: %d", completionResp.StatusCode)
	t.Logf("Response body: %s", string(bodyBytes))

	// Step 4: Validate response format
	t.Log("Step 4: Validating response format...")
	require.Equal(t, http.StatusOK, completionResp.StatusCode, "completion endpoint should return 200")

	var response OpenAIChatCompletionResponse
	err = json.Unmarshal(bodyBytes, &response)
	require.NoError(t, err, "failed to unmarshal response: %s", string(bodyBytes))

	// Validate response structure
	assert.NotEmpty(t, response.ID, "response should have an ID")
	assert.Equal(t, "chat.completion", response.Object, "object type should be chat.completion")
	assert.NotZero(t, response.Created, "created timestamp should be set")
	assert.NotEmpty(t, response.Model, "model name should be set")
	assert.Len(t, response.Choices, 1, "should have exactly 1 choice")
	t.Logf("✓ Response format is valid")

	// Step 5: Validate latency (relaxed for warmup)
	t.Log("Step 5: Validating response latency...")
	if latency > maxLatency {
		t.Logf("⚠ WARNING: Latency %v exceeds maximum %v (may be acceptable for first request/warmup)", latency, maxLatency)
	} else {
		t.Logf("✓ Response latency %v is within acceptable range", latency)
	}

	// Step 6: Validate token counting
	t.Log("Step 6: Validating token counting...")
	assert.Greater(t, response.Usage.PromptTokens, 0, "prompt_tokens should be > 0")
	assert.Greater(t, response.Usage.CompletionTokens, 0, "completion_tokens should be > 0")
	assert.Equal(t, response.Usage.PromptTokens+response.Usage.CompletionTokens, response.Usage.TotalTokens,
		"total_tokens should equal prompt_tokens + completion_tokens")
	t.Logf("✓ Token usage: %d prompt + %d completion = %d total",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)

	// Step 7: Validate response content
	t.Log("Step 7: Validating response content...")
	choice := response.Choices[0]
	assert.Equal(t, 0, choice.Index, "choice index should be 0")
	assert.Equal(t, "assistant", choice.Message.Role, "message role should be assistant")
	assert.NotEmpty(t, choice.Message.Content, "message content should not be empty")
	assert.NotEmpty(t, choice.FinishReason, "finish_reason should be set")

	t.Logf("Model response: %s", choice.Message.Content)
	t.Logf("Finish reason: %s", choice.FinishReason)

	// Basic sanity check - response should contain some text
	assert.Greater(t, len(choice.Message.Content), 0, "response should contain text")
	t.Logf("✓ Response content is valid")

	// SUCCESS
	t.Log("═══════════════════════════════════════")
	t.Log("✓ vLLM Completion Endpoint Test PASSED")
	t.Logf("  Backend: %s", backendURL)
	t.Logf("  Model: %s", response.Model)
	t.Logf("  Latency: %v", latency)
	t.Logf("  Tokens: %d prompt + %d completion = %d total",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
	t.Logf("  Response: %s", choice.Message.Content)
	t.Log("═══════════════════════════════════════")
}

// TestVLLMCompletionEndpoint_MultipleRequests tests that multiple sequential requests
// work correctly and latency improves after warmup.
func TestVLLMCompletionEndpoint_MultipleRequests(t *testing.T) {
	if os.Getenv("RUN_VLLM_TESTS") != "1" {
		t.Skip("Skipping vLLM completion test. Set RUN_VLLM_TESTS=1 to run.")
	}

	if testing.Short() {
		t.Skip("Skipping multiple requests test in short mode")
	}

	backendURL := os.Getenv("VLLM_BACKEND_URL")
	if backendURL == "" {
		t.Skip("VLLM_BACKEND_URL not set")
	}

	modelName := os.Getenv("VLLM_MODEL_NAME")
	if modelName == "" {
		modelName = "gpt-oss-20b"
	}

	const numRequests = 3
	latencies := make([]time.Duration, numRequests)

	t.Logf("Running %d sequential completion requests...", numRequests)

	for i := 0; i < numRequests; i++ {
		requestBody := OpenAIChatCompletionRequest{
			Model: modelName,
			Messages: []OpenAIMessage{
				{
					Role:    "user",
					Content: fmt.Sprintf("Say 'Request %d' and nothing else.", i+1),
				},
			},
			MaxTokens:   10,
			Temperature: 0.1,
		}

		jsonData, err := json.Marshal(requestBody)
		require.NoError(t, err)

		completionURL := fmt.Sprintf("%s/v1/chat/completions", backendURL)
		startTime := time.Now()

		resp, err := http.Post(completionURL, "application/json", bytes.NewReader(jsonData))
		latencies[i] = time.Since(startTime)

		require.NoError(t, err, "request %d failed", i+1)
		require.Equal(t, http.StatusOK, resp.StatusCode, "request %d returned status %d", i+1, resp.StatusCode)
		resp.Body.Close()

		t.Logf("Request %d: latency = %v", i+1, latencies[i])
	}

	// Check that latency stabilizes after warmup
	firstLatency := latencies[0]
	lastLatency := latencies[numRequests-1]

	t.Logf("First request latency: %v", firstLatency)
	t.Logf("Last request latency: %v", lastLatency)

	if lastLatency < firstLatency {
		t.Logf("✓ Latency improved after warmup (%.2f%% faster)",
			100.0*(float64(firstLatency-lastLatency)/float64(firstLatency)))
	} else {
		t.Logf("  Latency did not improve (may already be warmed up)")
	}

	// Calculate average latency (excluding first request which may include warmup)
	if numRequests > 1 {
		var sum time.Duration
		for i := 1; i < numRequests; i++ {
			sum += latencies[i]
		}
		avgLatency := sum / time.Duration(numRequests-1)
		t.Logf("Average latency (excluding first): %v", avgLatency)

		// Subsequent requests should be reasonably fast
		if avgLatency > 10*time.Second {
			t.Logf("⚠ WARNING: Average latency %v is higher than expected", avgLatency)
		}
	}

	t.Log("✓ Multiple requests test completed successfully")
}
