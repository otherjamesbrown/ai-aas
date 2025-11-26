// Package kserve provides protocol translation between OpenAI and KServe V2 inference protocols.
//
// Purpose:
//   This package implements bidirectional translation between the OpenAI Chat Completions API
//   and the KServe V2 Inference Protocol, enabling the API Router to communicate with
//   KServe-deployed models while maintaining OpenAI API compatibility for clients.
//
// Requirements Reference:
//   - specs/016-kserve-migration/spec.md#protocol-translation
//
package kserve

// InferRequest represents a KServe V2 inference request.
// Reference: https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md
type InferRequest struct {
	ID         string              `json:"id"`
	Inputs     []InferInputTensor  `json:"inputs"`
	Outputs    []InferRequestedOutput `json:"outputs,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// InferInputTensor represents an input tensor in a KServe V2 request.
type InferInputTensor struct {
	Name     string        `json:"name"`
	Shape    []int64       `json:"shape"`
	Datatype string        `json:"datatype"`
	Data     []interface{} `json:"data"`
}

// InferRequestedOutput represents a requested output in a KServe V2 request.
type InferRequestedOutput struct {
	Name string `json:"name"`
}

// InferResponse represents a KServe V2 inference response.
type InferResponse struct {
	ModelName    string                 `json:"model_name"`
	ModelVersion string                 `json:"model_version,omitempty"`
	ID           string                 `json:"id"`
	Outputs      []InferOutputTensor    `json:"outputs"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

// InferOutputTensor represents an output tensor in a KServe V2 response.
type InferOutputTensor struct {
	Name     string        `json:"name"`
	Shape    []int64       `json:"shape"`
	Datatype string        `json:"datatype"`
	Data     []interface{} `json:"data"`
}

// OpenAIChatCompletionRequest represents an OpenAI Chat Completions API request.
type OpenAIChatCompletionRequest struct {
	Model       string                 `json:"model"`
	Messages    []ChatMessage          `json:"messages"`
	Temperature *float64               `json:"temperature,omitempty"`
	MaxTokens   *int                   `json:"max_tokens,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	N           *int                   `json:"n,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Stop        []string               `json:"stop,omitempty"`
	PresencePenalty  *float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"`
	LogitBias   map[string]float64     `json:"logit_bias,omitempty"`
	User        string                 `json:"user,omitempty"`
}

// ChatMessage represents a message in the OpenAI chat format.
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// OpenAIChatCompletionResponse represents an OpenAI Chat Completions API response.
type OpenAIChatCompletionResponse struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"` // "chat.completion"
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   UsageInfo             `json:"usage"`
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

// ErrorResponse represents a KServe error response.
type ErrorResponse struct {
	Error string `json:"error"`
}
