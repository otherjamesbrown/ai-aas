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
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentPart for multimodal
}

// ContentPart represents a part of multimodal content
type ContentPart struct {
	Type     string    `json:"type"` // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in multimodal content
type ImageURL struct {
	URL string `json:"url"` // Can be HTTP URL or data URI (data:image/jpeg;base64,...)
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

// IsVLM detects if a model name indicates a vision-language model
func IsVLM(modelName string) bool {
	modelLower := strings.ToLower(modelName)
	vlmPatterns := []string{
		"qwen2-vl",
		"llava",
		"internvl",
		"pixtral",
		"phi-3-vision",
	}
	for _, pattern := range vlmPatterns {
		if strings.Contains(modelLower, pattern) {
			return true
		}
	}
	return false
}

// GetTestImageBase64 returns a base64-encoded test image (128x128 JPEG cat, ~4KB)
func GetTestImageBase64() string {
	// Embedded 128x128 cat image as JPEG base64 (from cataas.com, CC0)
	return "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"
}

// ValidateEndpoint validates an inference endpoint
func (v *InferenceValidator) ValidateEndpoint(ctx context.Context, endpoint, apiKey string) *InferenceResult {
	return v.ValidateEndpointWithModel(ctx, endpoint, apiKey, "default")
}

// ValidateEndpointWithModel validates an inference endpoint with a specific model name
func (v *InferenceValidator) ValidateEndpointWithModel(ctx context.Context, endpoint, apiKey, modelName string) *InferenceResult {
	start := time.Now()
	result := &InferenceResult{}

	// Build URL
	url := endpoint
	if !strings.HasSuffix(url, "/v1/chat/completions") {
		url = strings.TrimSuffix(url, "/") + "/v1/chat/completions"
	}

	// Detect if this is a VLM and create appropriate message content
	var messageContent interface{}
	if IsVLM(modelName) {
		// VLM: Send multimodal content (image + text)
		imageBase64 := GetTestImageBase64()
		messageContent = []ContentPart{
			{
				Type: "image_url",
				ImageURL: &ImageURL{
					URL: "data:image/jpeg;base64," + imageBase64,
				},
			},
			{
				Type: "text",
				Text: "What animal is in this image? Answer in one word.",
			},
		}
	} else {
		// Regular LLM: Send simple text
		messageContent = "Say hello in one word."
	}

	// Create request
	reqBody := ChatCompletionRequest{
		Model: modelName,
		Messages: []Message{
			{Role: "user", Content: messageContent},
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
