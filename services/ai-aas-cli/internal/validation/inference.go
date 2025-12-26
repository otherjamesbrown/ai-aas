// Package validation provides model validation functionality.
package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultInferenceTimeout is the default timeout for inference validation
// NOTE: This is now 90 seconds to accommodate GPU model cold starts (70B models can take 10-30s)
const DefaultInferenceTimeout = 90 * time.Second

// InferenceValidator validates model inference endpoints
type InferenceValidator struct {
	Timeout time.Duration
	Client  *http.Client
}

// NewInferenceValidator creates a new inference validator with default timeout
func NewInferenceValidator() *InferenceValidator {
	return NewInferenceValidatorWithTimeout(DefaultInferenceTimeout)
}

// NewInferenceValidatorWithTimeout creates a new inference validator with custom timeout
func NewInferenceValidatorWithTimeout(timeout time.Duration) *InferenceValidator {
	return &InferenceValidator{
		Timeout: timeout,
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

// InferenceResult contains the result of inference validation
type InferenceResult struct {
	Passed       bool          `json:"passed"`
	StatusCode   int           `json:"status_code"`
	ResponseTime time.Duration `json:"response_time"`
	Message      string        `json:"message"`
	Response     interface{}   `json:"response,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// String returns a string representation of the result
func (r *InferenceResult) String() string {
	status := "FAIL"
	if r.Passed {
		status = "PASS"
	}
	return fmt.Sprintf("[%s] %dms - %s", status, r.ResponseTime.Milliseconds(), r.Message)
}

// ChatCompletionRequest is the OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse is the OpenAI-compatible chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ValidateEndpoint validates an inference endpoint
func (v *InferenceValidator) ValidateEndpoint(ctx context.Context, endpoint, apiKey string) *InferenceResult {
	start := time.Now()
	result := &InferenceResult{}

	// Build URL
	url := endpoint
	if !strings.HasSuffix(url, "/v1/chat/completions") {
		url = strings.TrimSuffix(url, "/") + "/v1/chat/completions"
	}

	// Create request
	reqBody := ChatCompletionRequest{
		Model: "default",
		Messages: []Message{
			{Role: "user", Content: "Say hello in one word."},
		},
		MaxTokens:   10,
		Temperature: 0,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to marshal request: %v", err)
		result.Error = err.Error()
		result.ResponseTime = time.Since(start)
		return result
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Message = fmt.Sprintf("Failed to create request: %v", err)
		result.Error = err.Error()
		result.ResponseTime = time.Since(start)
		return result
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Execute request
	resp, err := v.Client.Do(req)
	result.ResponseTime = time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			result.Message = "Request timeout exceeded"
			result.Error = "timeout"
		} else {
			result.Message = fmt.Sprintf("Request failed: %v", err)
			result.Error = err.Error()
		}
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to read response: %v", err)
		result.Error = err.Error()
		return result
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("Endpoint returned status %d", resp.StatusCode)
		result.Error = string(body)
		return result
	}

	// Parse response
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		result.Message = fmt.Sprintf("Failed to parse response: %v", err)
		result.Error = err.Error()
		return result
	}

	// Validate response structure
	if chatResp.ID == "" {
		result.Message = "Response missing ID field"
		return result
	}

	if len(chatResp.Choices) == 0 {
		result.Message = "Response has no choices"
		return result
	}

	// Success
	result.Passed = true
	result.Message = fmt.Sprintf("Inference successful, %d tokens used", chatResp.Usage.TotalTokens)
	result.Response = chatResp

	return result
}

// ValidateStreaming validates streaming inference
func (v *InferenceValidator) ValidateStreaming(ctx context.Context, endpoint, apiKey string) *InferenceResult {
	start := time.Now()
	result := &InferenceResult{}

	// Build URL
	url := endpoint
	if !strings.HasSuffix(url, "/v1/chat/completions") {
		url = strings.TrimSuffix(url, "/") + "/v1/chat/completions"
	}

	// Create streaming request
	reqBody := ChatCompletionRequest{
		Model: "default",
		Messages: []Message{
			{Role: "user", Content: "Count to 3."},
		},
		MaxTokens:   20,
		Temperature: 0,
		Stream:      true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to marshal request: %v", err)
		result.Error = err.Error()
		result.ResponseTime = time.Since(start)
		return result
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Message = fmt.Sprintf("Failed to create request: %v", err)
		result.Error = err.Error()
		result.ResponseTime = time.Since(start)
		return result
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Execute request
	resp, err := v.Client.Do(req)
	result.ResponseTime = time.Since(start)

	if err != nil {
		result.Message = fmt.Sprintf("Request failed: %v", err)
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		result.Message = fmt.Sprintf("Endpoint returned status %d", resp.StatusCode)
		result.Error = string(body)
		return result
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		result.Message = fmt.Sprintf("Expected SSE content type, got: %s", contentType)
		return result
	}

	// Read a few chunks to verify streaming works
	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		result.Message = fmt.Sprintf("Failed to read stream: %v", err)
		result.Error = err.Error()
		return result
	}

	if n == 0 {
		result.Message = "Empty streaming response"
		return result
	}

	// Verify SSE format
	chunk := string(buf[:n])
	if !strings.Contains(chunk, "data:") {
		result.Message = "Response not in SSE format"
		return result
	}

	result.Passed = true
	result.Message = "Streaming inference successful"
	result.ResponseTime = time.Since(start)

	return result
}

// RunAllChecks runs all inference validation checks
func (v *InferenceValidator) RunAllChecks(ctx context.Context, endpoint, apiKey string) []InferenceResult {
	var results []InferenceResult

	// Basic inference check
	basic := v.ValidateEndpoint(ctx, endpoint, apiKey)
	basic.Message = "Basic inference: " + basic.Message
	results = append(results, *basic)

	// Only run streaming check if basic passed
	if basic.Passed {
		streaming := v.ValidateStreaming(ctx, endpoint, apiKey)
		streaming.Message = "Streaming: " + streaming.Message
		results = append(results, *streaming)
	}

	return results
}

