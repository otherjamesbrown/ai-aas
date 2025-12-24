// Package triton provides protocol translation for Triton Inference Server.
//
// This file implements translation between OpenAI Chat Completions format and
// Triton gRPC proto format for streaming inference with TensorRT-LLM backends.
package triton

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton/pb"
)

// GRPCTranslator handles protocol translation between OpenAI and Triton gRPC formats.
// It is specifically designed for streaming inference with TensorRT-LLM backends.
type GRPCTranslator struct {
	tokenizer *Tokenizer
}

// NewGRPCTranslator creates a new gRPC protocol translator with the specified tokenizer encoding.
// Supported encodings: cl100k_base, o200k_base, llama3
func NewGRPCTranslator(tokenizerEncoding string) (*GRPCTranslator, error) {
	tokenizer, err := NewTokenizer(tokenizerEncoding)
	if err != nil {
		return nil, fmt.Errorf("create tokenizer: %w", err)
	}

	return &GRPCTranslator{
		tokenizer: tokenizer,
	}, nil
}

// OpenAIStreamChunk represents a streaming chunk in OpenAI format.
type OpenAIStreamChunk struct {
	ID      string                     `json:"id"`
	Object  string                     `json:"object"`
	Created int64                      `json:"created"`
	Model   string                     `json:"model"`
	Choices []StreamChunkChoice        `json:"choices"`
	Usage   *UsageInfo                 `json:"usage,omitempty"` // Only on final chunk
}

// StreamChunkChoice represents a choice in a streaming chunk.
type StreamChunkChoice struct {
	Index        int           `json:"index"`
	Delta        StreamDelta   `json:"delta"`
	FinishReason *string       `json:"finish_reason"` // nil until final chunk
}

// StreamDelta represents the delta content in a streaming chunk.
type StreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// TranslateOpenAIToGRPC converts an OpenAI Chat Completion request to Triton gRPC ModelInferRequest.
// This is designed for TensorRT-LLM backends which expect specific tensor names.
func (t *GRPCTranslator) TranslateOpenAIToGRPC(req *OpenAIChatCompletionRequest, modelName string) (*pb.ModelInferRequest, string, error) {
	if req == nil {
		return nil, "", fmt.Errorf("request is nil")
	}

	if len(req.Messages) == 0 {
		return nil, "", fmt.Errorf("no messages provided")
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Format messages into a prompt string
	prompt, err := t.formatPrompt(req.Messages)
	if err != nil {
		return nil, "", fmt.Errorf("format prompt: %w", err)
	}

	// Build inputs for TensorRT-LLM
	inputs := []*pb.ModelInferRequest_InferInputTensor{
		t.createStringInput(TensorInputTextInput, prompt),
	}

	// Add max_tokens (REQUIRED by TRT-LLM - defaults to 512 if not specified)
	// TRT-LLM ensemble model marks max_tokens as required (optional: false)
	maxTokens := int32(512) // Default value matching TRT-LLM expectations
	if req.MaxTokens != nil {
		maxTokens = int32(*req.MaxTokens)
	}
	inputs = append(inputs, t.createInt32Input(TensorInputMaxTokens, maxTokens))

	// Add optional parameters as input tensors
	if req.Temperature != nil {
		inputs = append(inputs, t.createFloat32Input(TensorInputTemperature, float32(*req.Temperature)))
	}

	if req.TopP != nil {
		inputs = append(inputs, t.createFloat32Input(TensorInputTopP, float32(*req.TopP)))
	}

	// Add stop words if provided
	if len(req.Stop) > 0 {
		stopWordsJSON, _ := json.Marshal(req.Stop)
		inputs = append(inputs, t.createStringInput(TensorInputStopWords, string(stopWordsJSON)))
	}

	// Enable streaming
	inputs = append(inputs, t.createBoolInput(TensorInputStream, true))

	// Request the text_output tensor
	outputs := []*pb.ModelInferRequest_InferRequestedOutputTensor{
		{Name: TensorOutputTextOutput},
	}

	return &pb.ModelInferRequest{
		ModelName: modelName,
		Id:        requestID,
		Inputs:    inputs,
		Outputs:   outputs,
	}, requestID, nil
}

// TranslateStreamChunk converts a Triton gRPC streaming response to an OpenAI streaming chunk.
func (t *GRPCTranslator) TranslateStreamChunk(
	resp *pb.ModelStreamInferResponse,
	requestID string,
	model string,
	chunkIndex int,
	created int64,
) (*OpenAIStreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	// Check for error in response
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("triton error: %s", resp.ErrorMessage)
	}

	// Extract text content from the response
	text, err := t.extractTextFromGRPCResponse(resp)
	if err != nil {
		return nil, err
	}

	// Build the streaming chunk
	delta := StreamDelta{}

	// First chunk includes role
	if chunkIndex == 0 {
		delta.Role = "assistant"
	}

	// Add content if present
	if text != "" {
		delta.Content = text
	}

	chunk := &OpenAIStreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%s", requestID),
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []StreamChunkChoice{
			{
				Index: 0,
				Delta: delta,
			},
		},
	}

	return chunk, nil
}

// TranslateFinalChunk creates the final streaming chunk with finish reason and usage info.
func (t *GRPCTranslator) TranslateFinalChunk(
	requestID string,
	model string,
	created int64,
	finishReason string,
	promptTokens int,
	completionTokens int,
) *OpenAIStreamChunk {
	reason := finishReason
	return &OpenAIStreamChunk{
		ID:      fmt.Sprintf("chatcmpl-%s", requestID),
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []StreamChunkChoice{
			{
				Index:        0,
				Delta:        StreamDelta{},
				FinishReason: &reason,
			},
		},
		Usage: &UsageInfo{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

// ExtractTextFromGRPCResponse extracts the generated text from a Triton gRPC response.
// This is the public API for non-streaming gRPC inference.
func (t *GRPCTranslator) ExtractTextFromGRPCResponse(resp *pb.ModelStreamInferResponse) (string, error) {
	return t.extractTextFromGRPCResponse(resp)
}

// extractTextFromGRPCResponse extracts the generated text from a Triton gRPC response.
func (t *GRPCTranslator) extractTextFromGRPCResponse(resp *pb.ModelStreamInferResponse) (string, error) {
	if resp.InferResponse == nil {
		return "", nil // Empty response, no text
	}

	// Look for the text_output tensor in outputs
	for i, output := range resp.InferResponse.Outputs {
		if output.Name == TensorOutputTextOutput || output.Name == "text" || output.Name == "output" {
			return t.extractTextFromOutput(output, resp.InferResponse, i)
		}
	}

	// Fallback: try first output
	if len(resp.InferResponse.Outputs) > 0 {
		return t.extractTextFromOutput(resp.InferResponse.Outputs[0], resp.InferResponse, 0)
	}

	return "", nil
}

// extractTextFromOutput extracts text from a gRPC output tensor.
// It handles both structured contents (output.Contents.BytesContents) and raw format (resp.RawOutputContents).
func (t *GRPCTranslator) extractTextFromOutput(
	output *pb.ModelInferResponse_InferOutputTensor,
	resp *pb.ModelInferResponse,
	outputIndex int,
) (string, error) {
	if output == nil {
		return "", nil
	}

	// Check contents field first (structured data)
	if output.Contents != nil {
		// BYTES type contains string data
		if len(output.Contents.BytesContents) > 0 {
			return string(output.Contents.BytesContents[0]), nil
		}
	}

	// Check raw_output_contents in parent response
	// TRT-LLM uses this format for better performance
	if resp != nil && len(resp.RawOutputContents) > outputIndex {
		rawBytes := resp.RawOutputContents[outputIndex]
		if len(rawBytes) > 0 {
			return string(rawBytes), nil
		}
	}

	return "", nil
}

// createStringInput creates a string/BYTES input tensor for gRPC.
func (t *GRPCTranslator) createStringInput(name, value string) *pb.ModelInferRequest_InferInputTensor {
	return &pb.ModelInferRequest_InferInputTensor{
		Name:     name,
		Datatype: DatatypeBYTES,
		Shape:    []int64{1},
		Contents: &pb.InferTensorContents{
			BytesContents: [][]byte{[]byte(value)},
		},
	}
}

// createInt32Input creates an INT32 input tensor for gRPC.
func (t *GRPCTranslator) createInt32Input(name string, value int32) *pb.ModelInferRequest_InferInputTensor {
	return &pb.ModelInferRequest_InferInputTensor{
		Name:     name,
		Datatype: DatatypeINT32,
		Shape:    []int64{1},
		Contents: &pb.InferTensorContents{
			IntContents: []int32{value},
		},
	}
}

// createFloat32Input creates an FP32 input tensor for gRPC.
func (t *GRPCTranslator) createFloat32Input(name string, value float32) *pb.ModelInferRequest_InferInputTensor {
	return &pb.ModelInferRequest_InferInputTensor{
		Name:     name,
		Datatype: DatatypeFP32,
		Shape:    []int64{1},
		Contents: &pb.InferTensorContents{
			Fp32Contents: []float32{value},
		},
	}
}

// createBoolInput creates a BOOL input tensor for gRPC.
func (t *GRPCTranslator) createBoolInput(name string, value bool) *pb.ModelInferRequest_InferInputTensor {
	return &pb.ModelInferRequest_InferInputTensor{
		Name:     name,
		Datatype: DatatypeBOOL,
		Shape:    []int64{1},
		Contents: &pb.InferTensorContents{
			BoolContents: []bool{value},
		},
	}
}

// formatPrompt converts OpenAI messages to a single prompt string.
// Uses Llama-style chat template format for TensorRT-LLM.
func (t *GRPCTranslator) formatPrompt(messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages provided")
	}

	// Use Llama 3 chat template format
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

// CountPromptTokens counts the tokens in the request messages.
func (t *GRPCTranslator) CountPromptTokens(messages []ChatMessage) int {
	return t.tokenizer.CountMessagesTokens(messages)
}

// CountCompletionTokens counts the tokens in a completion string.
func (t *GRPCTranslator) CountCompletionTokens(text string) int {
	return t.tokenizer.CountTokens(text)
}

// GetTokenizer returns the translator's tokenizer for direct use.
func (t *GRPCTranslator) GetTokenizer() *Tokenizer {
	return t.tokenizer
}

// SerializeChunk serializes an OpenAI streaming chunk to JSON bytes.
func (t *GRPCTranslator) SerializeChunk(chunk *OpenAIStreamChunk) ([]byte, error) {
	return json.Marshal(chunk)
}

// StreamingConfig holds configuration for streaming responses.
type StreamingConfig struct {
	// RequestID is the unique identifier for this streaming session
	RequestID string

	// Model is the model name to include in responses
	Model string

	// Created is the Unix timestamp when the streaming session started
	Created int64
}

// NewStreamingConfig creates a new streaming configuration.
func NewStreamingConfig(model string) *StreamingConfig {
	return &StreamingConfig{
		RequestID: uuid.New().String(),
		Model:     model,
		Created:   time.Now().Unix(),
	}
}
