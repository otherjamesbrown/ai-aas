package triton

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Translator handles protocol translation between OpenAI and Triton V2.
type Translator struct {
	tokenizer *Tokenizer
}

// NewTranslator creates a new protocol translator with the specified tokenizer encoding.
// Supported encodings: cl100k_base, o200k_base, llama3
func NewTranslator(tokenizerEncoding string) (*Translator, error) {
	tokenizer, err := NewTokenizer(tokenizerEncoding)
	if err != nil {
		return nil, fmt.Errorf("create tokenizer: %w", err)
	}

	return &Translator{
		tokenizer: tokenizer,
	}, nil
}

// TranslateOpenAIToTriton converts an OpenAI Chat Completion request to Triton V2 format.
// This is designed for TensorRT-LLM backends which expect specific tensor names.
func (t *Translator) TranslateOpenAIToTriton(req *OpenAIChatCompletionRequest) (*InferRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Format messages into a prompt string
	prompt, err := t.formatPrompt(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("format prompt: %w", err)
	}

	// Build inputs for TensorRT-LLM
	inputs := []InferInputTensor{
		{
			Name:     TensorInputTextInput,
			Shape:    []int64{1},
			Datatype: DatatypeBYTES,
			Data:     []interface{}{prompt},
		},
	}

	// Add max_tokens (REQUIRED by TRT-LLM - defaults to 512 if not specified)
	// TRT-LLM ensemble model marks max_tokens as required (optional: false)
	maxTokens := 512 // Default value matching TRT-LLM expectations
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	inputs = append(inputs, InferInputTensor{
		Name:     TensorInputMaxTokens,
		Shape:    []int64{1},
		Datatype: DatatypeINT32,
		Data:     []interface{}{maxTokens},
	})

	// Note: We intentionally do NOT add optional parameters like temperature, top_p, stop_words, or stream.
	// TensorRT-LLM ensemble models only accept required inputs (text_input, max_tokens).
	// Adding extra inputs causes "expected N inputs but got N+1" errors.
	// Optional parameters like temperature are configured at model deployment time, not per-request.
	// Streaming is controlled at the protocol level (SSE for HTTP), not via input tensors.

	// Request the text_output tensor
	outputs := []InferRequestedOutput{
		{Name: TensorOutputTextOutput},
	}

	return &InferRequest{
		ID:      requestID,
		Inputs:  inputs,
		Outputs: outputs,
	}, nil
}

// TranslateOpenAICompletionToTriton converts an OpenAI Text Completion request to Triton V2 format.
// It converts the prompt string into a chat message format internally, then reuses the
// existing chat completion translation logic.
func (t *Translator) TranslateOpenAICompletionToTriton(req *OpenAICompletionRequest) (*InferRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Convert text completion request to chat completion format
	// Use a single user message containing the prompt text
	chatReq := &OpenAIChatCompletionRequest{
		Model: req.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: req.Prompt,
			},
		},
		Temperature:      req.Temperature,
		MaxTokens:        req.MaxTokens,
		TopP:             req.TopP,
		N:                req.N,
		Stream:           req.Stream,
		Stop:             req.Stop,
		PresencePenalty:  req.PresencePenalty,
		FrequencyPenalty: req.FrequencyPenalty,
		LogitBias:        req.LogitBias,
		User:             req.User,
	}

	// Reuse the existing chat completion translation
	return t.TranslateOpenAIToTriton(chatReq)
}

// TranslateTritonToOpenAI converts a Triton V2 response to OpenAI Chat Completion format.
func (t *Translator) TranslateTritonToOpenAI(
	resp *InferResponse,
	originalReq *OpenAIChatCompletionRequest,
) (*OpenAIChatCompletionResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	if len(resp.Outputs) == 0 {
		return nil, fmt.Errorf("no outputs in Triton response")
	}

	// Extract completion text from outputs
	completionText, err := t.extractCompletionText(resp)
	if err != nil {
		return nil, fmt.Errorf("extract completion text: %w", err)
	}

	// Calculate token counts using tiktoken
	// Use CountMessagesTokens for accurate prompt token counting with message overhead
	promptTokens := t.tokenizer.CountMessagesTokens(originalReq.Messages)
	completionTokens := t.tokenizer.CountTokens(completionText)

	// Determine finish reason
	finishReason := t.determineFinishReason(resp, originalReq, completionTokens)

	// Normalize completion text to prevent nil content
	content := interface{}(completionText)
	if content == nil {
		content = ""
	}

	// Build OpenAI response
	choice := ChatCompletionChoice{
		Index: 0,
		Message: ChatMessage{
			Role:    "assistant",
			Content: content,
		},
		FinishReason: finishReason,
	}

	return &OpenAIChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", resp.ID),
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

// TranslateTritonToOpenAICompletion converts a Triton V2 response to OpenAI Text Completion format.
func (t *Translator) TranslateTritonToOpenAICompletion(
	resp *InferResponse,
	originalReq *OpenAICompletionRequest,
) (*OpenAICompletionResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	if len(resp.Outputs) == 0 {
		return nil, fmt.Errorf("no outputs in Triton response")
	}

	// Extract completion text from outputs
	completionText, err := t.extractCompletionText(resp)
	if err != nil {
		return nil, fmt.Errorf("extract completion text: %w", err)
	}

	// Calculate token counts using tiktoken
	promptTokens := t.tokenizer.CountTokens(originalReq.Prompt)
	completionTokens := t.tokenizer.CountTokens(completionText)

	// Determine finish reason
	// Convert the completion request to chat format for finish reason logic
	chatReq := &OpenAIChatCompletionRequest{
		MaxTokens: originalReq.MaxTokens,
	}
	finishReason := t.determineFinishReason(resp, chatReq, completionTokens)

	// Build OpenAI text completion response
	choice := CompletionChoice{
		Index:        0,
		Text:         completionText,
		FinishReason: finishReason,
	}

	return &OpenAICompletionResponse{
		ID:      fmt.Sprintf("cmpl-%s", resp.ID),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   originalReq.Model,
		Choices: []CompletionChoice{choice},
		Usage: UsageInfo{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}, nil
}

// extractCompletionText extracts the generated text from Triton response outputs.
func (t *Translator) extractCompletionText(resp *InferResponse) (string, error) {
	// Look for the text_output tensor
	for _, output := range resp.Outputs {
		if output.Name == TensorOutputTextOutput {
			if len(output.Data) > 0 {
				return t.extractStringFromData(output.Data[0])
			}
		}
	}

	// Fallback: try common output names
	for _, output := range resp.Outputs {
		if output.Name == "text" || output.Name == "output" || output.Name == "generated_text" {
			if len(output.Data) > 0 {
				return t.extractStringFromData(output.Data[0])
			}
		}
	}

	// Last resort: use first output
	if len(resp.Outputs) > 0 && len(resp.Outputs[0].Data) > 0 {
		return t.extractStringFromData(resp.Outputs[0].Data[0])
	}

	return "", fmt.Errorf("no completion text found in Triton response")
}

// extractStringFromData extracts a string from tensor data.
// Handles both direct strings and base64-encoded data.
func (t *Translator) extractStringFromData(data interface{}) (string, error) {
	switch v := data.(type) {
	case string:
		// Check if it's base64 encoded (common with Triton BYTES type)
		// If decoding fails, assume it's already plain text
		decoded, err := DecodeBase64Data(v)
		if err == nil && decoded != "" {
			return decoded, nil
		}
		return v, nil
	case []byte:
		return string(v), nil
	default:
		// Try JSON conversion for other types
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cannot convert data to string: %T", data)
		}
		return string(bytes), nil
	}
}

// formatPrompt converts OpenAI messages to a single prompt string.
// Uses Llama-style chat template format for TensorRT-LLM.
func (t *Translator) formatPrompt(messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages provided")
	}

	// Use Llama 3 chat template format
	// Reference: https://llama.meta.com/docs/model-cards-and-prompt-formats/meta-llama-3
	var sb strings.Builder

	// Always start with BOS token - required for Llama 3 models
	// Without this, the model doesn't recognize the conversation format
	// and immediately outputs EOS, resulting in empty completions
	sb.WriteString("<|begin_of_text|>")

	for _, msg := range messages {
		// Extract text content from message (handles both string and multimodal)
		textContent := msg.GetTextContent()

		switch msg.Role {
		case "system":
			sb.WriteString("<|start_header_id|>system<|end_header_id|>\n\n")
			sb.WriteString(textContent)
			sb.WriteString("<|eot_id|>")
		case "user":
			sb.WriteString("<|start_header_id|>user<|end_header_id|>\n\n")
			sb.WriteString(textContent)
			sb.WriteString("<|eot_id|>")
		case "assistant":
			sb.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")
			sb.WriteString(textContent)
			sb.WriteString("<|eot_id|>")
		default:
			sb.WriteString(textContent)
		}
	}

	// Prime for assistant response
	sb.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")

	return sb.String(), nil
}

// determineFinishReason determines the finish reason based on response and parameters.
func (t *Translator) determineFinishReason(resp *InferResponse, req *OpenAIChatCompletionRequest, completionTokens int) string {
	// Check if max_tokens was reached
	if req.MaxTokens != nil && completionTokens >= *req.MaxTokens {
		return "length"
	}

	// Check response parameters for finish reason
	if resp.Parameters != nil {
		if reason, ok := resp.Parameters["finish_reason"].(string); ok {
			switch reason {
			case "stop", "eos_token":
				return "stop"
			case "length", "max_tokens":
				return "length"
			case "content_filter":
				return "content_filter"
			}
		}
	}

	// Default to "stop"
	return "stop"
}

// SerializeTritonRequest serializes an InferRequest to JSON bytes.
func (t *Translator) SerializeTritonRequest(req *InferRequest) ([]byte, error) {
	return json.Marshal(req)
}

// ParseTritonResponse parses a Triton V2 response from JSON bytes.
func (t *Translator) ParseTritonResponse(data []byte) (*InferResponse, error) {
	var resp InferResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal Triton response: %w", err)
	}
	return &resp, nil
}

// GetTokenizer returns the translator's tokenizer for direct use.
func (t *Translator) GetTokenizer() *Tokenizer {
	return t.tokenizer
}
