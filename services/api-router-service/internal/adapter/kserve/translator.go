package kserve

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Translator handles protocol translation between OpenAI and KServe V2.
type Translator struct {
	// Future: Add configuration options like token counting, etc.
}

// NewTranslator creates a new protocol translator.
func NewTranslator() *Translator {
	return &Translator{}
}

// TranslateOpenAIToKServe converts an OpenAI Chat Completion request to KServe V2 format.
func (t *Translator) TranslateOpenAIToKServe(req *OpenAIChatCompletionRequest) (*InferRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Serialize messages to a prompt string
	// This is a simple concatenation; vLLM will handle the chat template
	prompt, err := t.formatPrompt(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("format prompt: %w", err)
	}

	// Create input tensor
	input := InferInputTensor{
		Name:     "prompt",
		Shape:    []int64{1}, // Single prompt
		Datatype: "BYTES",
		Data:     []interface{}{prompt},
	}

	// Build parameters from OpenAI request
	parameters := make(map[string]interface{})
	if req.Temperature != nil {
		parameters["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		parameters["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		parameters["top_p"] = *req.TopP
	}
	if req.N != nil {
		parameters["n"] = *req.N
	}
	if req.PresencePenalty != nil {
		parameters["presence_penalty"] = *req.PresencePenalty
	}
	if req.FrequencyPenalty != nil {
		parameters["frequency_penalty"] = *req.FrequencyPenalty
	}
	if len(req.Stop) > 0 {
		parameters["stop"] = req.Stop
	}

	return &InferRequest{
		ID:         requestID,
		Inputs:     []InferInputTensor{input},
		Parameters: parameters,
	}, nil
}

// TranslateKServeToOpenAI converts a KServe V2 response to OpenAI Chat Completion format.
func (t *Translator) TranslateKServeToOpenAI(
	resp *InferResponse,
	originalReq *OpenAIChatCompletionRequest,
) (*OpenAIChatCompletionResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	// Extract completion text from outputs
	if len(resp.Outputs) == 0 {
		return nil, fmt.Errorf("no outputs in KServe response")
	}

	// Find the text output (typically the first output)
	var completionText string
	for _, output := range resp.Outputs {
		if output.Name == "text" || output.Name == "output" {
			if len(output.Data) > 0 {
				if text, ok := output.Data[0].(string); ok {
					completionText = text
					break
				}
			}
		}
	}

	if completionText == "" && len(resp.Outputs) > 0 && len(resp.Outputs[0].Data) > 0 {
		// Fallback: use first output data
		if text, ok := resp.Outputs[0].Data[0].(string); ok {
			completionText = text
		}
	}

	if completionText == "" {
		return nil, fmt.Errorf("no completion text found in KServe response")
	}

	// Estimate token counts (simple approximation: 1 token ~= 4 characters)
	// In production, use a proper tokenizer
	promptTokens := t.estimateTokens(originalReq.Messages)
	completionTokens := t.estimateTokens([]ChatMessage{{Role: "assistant", Content: completionText}})

	// Build OpenAI response
	choice := ChatCompletionChoice{
		Index: 0,
		Message: ChatMessage{
			Role:    "assistant",
			Content: completionText,
		},
		FinishReason: "stop", // Default to "stop"; could be enhanced
	}

	return &OpenAIChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   originalReq.Model,
		Choices: []ChatCompletionChoice{choice},
		Usage: UsageInfo{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}, nil
}

// formatPrompt converts OpenAI messages to a single prompt string.
// This is a simple implementation; production should use model-specific chat templates.
func (t *Translator) formatPrompt(messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages provided")
	}

	// Simple concatenation with role prefixes
	// vLLM will apply the model's chat template internally
	var parts []string
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			parts = append(parts, fmt.Sprintf("System: %s", msg.Content))
		case "user":
			parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
		case "assistant":
			parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
		default:
			parts = append(parts, msg.Content)
		}
	}

	return strings.Join(parts, "\n"), nil
}

// estimateTokens provides a rough token count estimate.
// In production, use tiktoken or the actual tokenizer.
func (t *Translator) estimateTokens(messages []ChatMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	// Rough estimate: 1 token ~= 4 characters
	return (totalChars / 4) + len(messages)*4 // Add overhead for role tokens
}

// TranslateError converts a KServe error to an OpenAI error format.
func (t *Translator) TranslateError(kserveErr string) map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"message": kserveErr,
			"type":    "kserve_error",
			"code":    "internal_error",
		},
	}
}

// SerializeKServeRequest serializes an InferRequest to JSON bytes.
func (t *Translator) SerializeKServeRequest(req *InferRequest) ([]byte, error) {
	return json.Marshal(req)
}

// ParseKServeResponse parses a KServe V2 response from JSON bytes.
func (t *Translator) ParseKServeResponse(data []byte) (*InferResponse, error) {
	var resp InferResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal KServe response: %w", err)
	}
	return &resp, nil
}
