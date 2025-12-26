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

	// Add optional parameters as input tensors
	if req.MaxTokens != nil {
		inputs = append(inputs, InferInputTensor{
			Name:     TensorInputMaxTokens,
			Shape:    []int64{1},
			Datatype: DatatypeINT32,
			Data:     []interface{}{*req.MaxTokens},
		})
	}

	if req.Temperature != nil {
		inputs = append(inputs, InferInputTensor{
			Name:     TensorInputTemperature,
			Shape:    []int64{1},
			Datatype: DatatypeFP32,
			Data:     []interface{}{*req.Temperature},
		})
	}

	if req.TopP != nil {
		inputs = append(inputs, InferInputTensor{
			Name:     TensorInputTopP,
			Shape:    []int64{1},
			Datatype: DatatypeFP32,
			Data:     []interface{}{*req.TopP},
		})
	}

	// Add stop words if provided
	if len(req.Stop) > 0 {
		// TensorRT-LLM expects stop words as a JSON-encoded array
		stopWordsJSON, _ := json.Marshal(req.Stop)
		inputs = append(inputs, InferInputTensor{
			Name:     TensorInputStopWords,
			Shape:    []int64{1},
			Datatype: DatatypeBYTES,
			Data:     []interface{}{string(stopWordsJSON)},
		})
	}

	// Set stream to false (streaming not supported in MVP)
	inputs = append(inputs, InferInputTensor{
		Name:     TensorInputStream,
		Shape:    []int64{1},
		Datatype: DatatypeBOOL,
		Data:     []interface{}{false},
	})

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

	// Build OpenAI response
	choice := ChatCompletionChoice{
		Index: 0,
		Message: ChatMessage{
			Role:    "assistant",
			Content: completionText,
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
		switch msg.Role {
		case "system":
			sb.WriteString("<|start_header_id|>system<|end_header_id|>\n\n")
			sb.WriteString(msg.Content)
			sb.WriteString("<|eot_id|>")
		case "user":
			sb.WriteString("<|start_header_id|>user<|end_header_id|>\n\n")
			sb.WriteString(msg.Content)
			sb.WriteString("<|eot_id|>")
		case "assistant":
			sb.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")
			sb.WriteString(msg.Content)
			sb.WriteString("<|eot_id|>")
		default:
			sb.WriteString(msg.Content)
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
