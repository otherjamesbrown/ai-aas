// Package triton provides protocol translation between OpenAI and Triton V2 inference protocols.
//
// Purpose:
//
//	This package implements bidirectional translation between the OpenAI Chat Completions API
//	and the Triton Inference Server V2 Protocol, enabling the API Router to communicate with
//	TensorRT-LLM models deployed on Triton while maintaining OpenAI API compatibility for clients.
//
// Requirements Reference:
//   - specs/032-triton-api-support/spec.md#protocol-translation
package triton

import (
	"encoding/base64"
	"strings"
)

// InferRequest represents a Triton V2 inference request.
// Reference: https://github.com/triton-inference-server/server/blob/main/docs/protocol/extension_generate.md
type InferRequest struct {
	ID         string                 `json:"id,omitempty"`
	Inputs     []InferInputTensor     `json:"inputs"`
	Outputs    []InferRequestedOutput `json:"outputs,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// InferInputTensor represents an input tensor in a Triton V2 request.
type InferInputTensor struct {
	Name       string                 `json:"name"`
	Shape      []int64                `json:"shape"`
	Datatype   string                 `json:"datatype"`
	Data       []interface{}          `json:"data,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// InferRequestedOutput represents a requested output in a Triton V2 request.
type InferRequestedOutput struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// InferResponse represents a Triton V2 inference response.
type InferResponse struct {
	ModelName    string                 `json:"model_name"`
	ModelVersion string                 `json:"model_version,omitempty"`
	ID           string                 `json:"id"`
	Outputs      []InferOutputTensor    `json:"outputs"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

// InferOutputTensor represents an output tensor in a Triton V2 response.
type InferOutputTensor struct {
	Name       string                 `json:"name"`
	Shape      []int64                `json:"shape"`
	Datatype   string                 `json:"datatype"`
	Data       []interface{}          `json:"data,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// OpenAIChatCompletionRequest represents an OpenAI Chat Completions API request.
type OpenAIChatCompletionRequest struct {
	Model            string             `json:"model"`
	Messages         []ChatMessage      `json:"messages"`
	Temperature      *float64           `json:"temperature,omitempty"`
	MaxTokens        *int               `json:"max_tokens,omitempty"`
	TopP             *float64           `json:"top_p,omitempty"`
	N                *int               `json:"n,omitempty"`
	Stream           bool               `json:"stream,omitempty"`
	Stop             []string           `json:"stop,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	User             string             `json:"user,omitempty"`
}

// ChatMessage represents a message in the OpenAI chat format.
// Content can be either:
// - A plain string: "Hello"
// - An array of content parts: [{"type": "text", "text": "..."}, {"type": "image_url", "image_url": {...}}]
// Reasoning models may return content in Reasoning or ReasoningContent fields instead.
type ChatMessage struct {
	Role             string      `json:"role"`                        // "system", "user", "assistant"
	Content          interface{} `json:"content"`                     // string OR []ContentPart
	Reasoning        interface{} `json:"reasoning,omitempty"`         // Some models return reasoning instead of content
	ReasoningContent interface{} `json:"reasoning_content,omitempty"` // Alternative reasoning field
}

// ContentPart represents a single part of multimodal message content.
// Supports text and image_url content types for vision models.
type ContentPart struct {
	Type     string           `json:"type"` // "text" or "image_url"
	Text     string           `json:"text,omitempty"`
	ImageURL *ImageURLContent `json:"image_url,omitempty"`
}

// ImageURLContent represents an image URL in a multimodal message.
type ImageURLContent struct {
	URL    string `json:"url"`              // Image URL (data URI or HTTP URL)
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}

// GetTextContent extracts text from Content, handling both string and multimodal formats.
// For multimodal content, it concatenates all text parts with spaces.
// This is useful for text-only backends that need to extract prompts from potentially multimodal messages.
func (m *ChatMessage) GetTextContent() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []interface{}:
		var texts []string
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if partMap["type"] == "text" {
					if text, ok := partMap["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, " ")
	default:
		return ""
	}
}

// NormalizeContent extracts content from Content, Reasoning, or ReasoningContent fields.
// Reasoning models (like gpt-oss-20b) may return content in reasoning_content field instead of content.
// This method provides backward compatibility by extracting from the first non-empty field.
// Returns the content as interface{} to preserve the original type (string or []ContentPart).
func (m *ChatMessage) NormalizeContent() interface{} {
	// Try Content first (standard field)
	if m.Content != nil {
		if str, ok := m.Content.(string); ok && str != "" {
			return m.Content
		}
		// For non-string content (multimodal), if it's non-nil and non-empty array, use it
		if arr, ok := m.Content.([]interface{}); ok && len(arr) > 0 {
			return m.Content
		}
	}

	// Fallback to Reasoning field
	if m.Reasoning != nil {
		if str, ok := m.Reasoning.(string); ok && str != "" {
			return m.Reasoning
		}
		if arr, ok := m.Reasoning.([]interface{}); ok && len(arr) > 0 {
			return m.Reasoning
		}
	}

	// Fallback to ReasoningContent field
	if m.ReasoningContent != nil {
		if str, ok := m.ReasoningContent.(string); ok && str != "" {
			return m.ReasoningContent
		}
		if arr, ok := m.ReasoningContent.([]interface{}); ok && len(arr) > 0 {
			return m.ReasoningContent
		}
	}

	// All fields are empty, return empty string for backward compatibility
	return ""
}

// OpenAIChatCompletionResponse represents an OpenAI Chat Completions API response.
type OpenAIChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"` // "chat.completion"
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   UsageInfo              `json:"usage"`
}

// ChatCompletionChoice represents a completion choice in the OpenAI response.
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"` // "stop", "length", "content_filter"
}

// UsageInfo represents token usage information.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIErrorResponse represents an OpenAI-style error response.
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// OpenAIError represents an OpenAI-style error.
type OpenAIError struct {
	Message       string `json:"message"`
	Type          string `json:"type"`
	Code          string `json:"code,omitempty"`
	Param         string `json:"param,omitempty"`
	TritonDetails string `json:"triton_details,omitempty"` // Preserve Triton error details for debugging
}

// TritonErrorResponse represents a Triton error response.
type TritonErrorResponse struct {
	Error string `json:"error"`
}

// TensorRTLLMInputs defines the input tensor names for TensorRT-LLM models.
const (
	// TensorInputTextInput is the input tensor name for the text prompt.
	TensorInputTextInput = "text_input"
	// TensorInputMaxTokens is the input tensor name for max output tokens.
	TensorInputMaxTokens = "max_tokens"
	// TensorInputTemperature is the input tensor name for temperature.
	TensorInputTemperature = "temperature"
	// TensorInputTopP is the input tensor name for top_p.
	TensorInputTopP = "top_p"
	// TensorInputStopWords is the input tensor name for stop words.
	TensorInputStopWords = "stop_words"
	// TensorInputBadWords is the input tensor name for bad words.
	TensorInputBadWords = "bad_words"
	// TensorInputStream is the input tensor name for streaming flag.
	TensorInputStream = "stream"
)

// TensorRTLLMOutputs defines the output tensor names for TensorRT-LLM models.
const (
	// TensorOutputTextOutput is the output tensor name for the generated text.
	TensorOutputTextOutput = "text_output"
)

// Triton datatype constants
const (
	DatatypeBYTES  = "BYTES"
	DatatypeINT32  = "INT32"
	DatatypeFP32   = "FP32"
	DatatypeBOOL   = "BOOL"
	DatatypeSTRING = "BYTES" // Triton uses BYTES for string data
)

// OpenAICompletionRequest represents an OpenAI Text Completions API request.
// This format uses a simple prompt string instead of the messages array used by chat completions.
type OpenAICompletionRequest struct {
	Model            string             `json:"model"`
	Prompt           string             `json:"prompt"`
	Temperature      *float64           `json:"temperature,omitempty"`
	MaxTokens        *int               `json:"max_tokens,omitempty"`
	TopP             *float64           `json:"top_p,omitempty"`
	N                *int               `json:"n,omitempty"`
	Stream           bool               `json:"stream,omitempty"`
	Stop             []string           `json:"stop,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	User             string             `json:"user,omitempty"`
}

// OpenAICompletionResponse represents an OpenAI Text Completions API response.
type OpenAICompletionResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"` // "text_completion"
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []CompletionChoice  `json:"choices"`
	Usage   UsageInfo           `json:"usage"`
}

// CompletionChoice represents a completion choice in the text completion response.
type CompletionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"` // "stop", "length", "content_filter"
}

// DecodeBase64Data decodes base64-encoded tensor data from Triton response.
// Some Triton configurations return data as base64-encoded strings.
func DecodeBase64Data(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
