package triton

import (
	"strings"
	"testing"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton/pb"
)

func TestNewGRPCTranslator(t *testing.T) {
	tests := []struct {
		name        string
		encoding    string
		expectError bool
	}{
		{"valid encoding cl100k_base", "cl100k_base", false},
		{"valid encoding llama3", "llama3", false},
		{"valid encoding o200k_base", "o200k_base", false},
		{"invalid encoding", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator, err := NewGRPCTranslator(tt.encoding)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if translator == nil {
				t.Error("expected translator, got nil")
			}
		})
	}
}

func TestGRPCTranslator_TranslateOpenAIToGRPC(t *testing.T) {
	translator, err := NewGRPCTranslator("llama3")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	t.Run("nil request", func(t *testing.T) {
		_, _, err := translator.TranslateOpenAIToGRPC(nil, "test-model")
		if err == nil {
			t.Error("expected error for nil request")
		}
	})

	t.Run("empty messages", func(t *testing.T) {
		req := &OpenAIChatCompletionRequest{
			Model:    "test-model",
			Messages: []ChatMessage{},
		}
		_, _, err := translator.TranslateOpenAIToGRPC(req, "test-model")
		if err == nil {
			t.Error("expected error for empty messages")
		}
	})

	t.Run("valid request with minimal fields", func(t *testing.T) {
		req := &OpenAIChatCompletionRequest{
			Model: "test-model",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		grpcReq, requestID, err := translator.TranslateOpenAIToGRPC(req, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if grpcReq == nil {
			t.Error("expected grpcReq, got nil")
			return
		}
		if requestID == "" {
			t.Error("expected non-empty requestID")
		}
		if grpcReq.ModelName != "test-model" {
			t.Errorf("expected model name 'test-model', got %q", grpcReq.ModelName)
		}
		// Should have text_input, max_tokens (always included), and stream inputs
		if len(grpcReq.Inputs) < 3 {
			t.Errorf("expected at least 3 inputs (text_input, max_tokens, stream), got %d", len(grpcReq.Inputs))
		}
		// Verify max_tokens is present with default value
		foundMaxTokens := false
		for _, input := range grpcReq.Inputs {
			if input.Name == TensorInputMaxTokens {
				foundMaxTokens = true
				if len(input.Contents.IntContents) == 0 || input.Contents.IntContents[0] != 512 {
					t.Errorf("expected default max_tokens of 512, got %v", input.Contents.IntContents)
				}
			}
		}
		if !foundMaxTokens {
			t.Error("expected max_tokens input to always be present")
		}
	})

	t.Run("valid request with all optional fields", func(t *testing.T) {
		maxTokens := 100
		temperature := 0.7
		topP := 0.9
		req := &OpenAIChatCompletionRequest{
			Model: "test-model",
			Messages: []ChatMessage{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Hello"},
			},
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
			TopP:        &topP,
			Stop:        []string{"stop1", "stop2"},
		}
		grpcReq, _, err := translator.TranslateOpenAIToGRPC(req, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Verify inputs: text_input, max_tokens, temperature, top_p, stop_words, stream
		expectedInputs := 6
		if len(grpcReq.Inputs) != expectedInputs {
			t.Errorf("expected %d inputs, got %d", expectedInputs, len(grpcReq.Inputs))
		}

		// Check input names
		inputNames := make(map[string]bool)
		for _, input := range grpcReq.Inputs {
			inputNames[input.Name] = true
		}

		expected := []string{TensorInputTextInput, TensorInputMaxTokens, TensorInputTemperature, TensorInputTopP, TensorInputStopWords, TensorInputStream}
		for _, name := range expected {
			if !inputNames[name] {
				t.Errorf("missing expected input: %s", name)
			}
		}
	})

	t.Run("outputs include text_output", func(t *testing.T) {
		req := &OpenAIChatCompletionRequest{
			Model: "test-model",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		grpcReq, _, err := translator.TranslateOpenAIToGRPC(req, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if len(grpcReq.Outputs) != 1 {
			t.Errorf("expected 1 output, got %d", len(grpcReq.Outputs))
			return
		}
		if grpcReq.Outputs[0].Name != TensorOutputTextOutput {
			t.Errorf("expected output name %q, got %q", TensorOutputTextOutput, grpcReq.Outputs[0].Name)
		}
	})
}

func TestGRPCTranslator_TranslateStreamChunk(t *testing.T) {
	translator, err := NewGRPCTranslator("llama3")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	t.Run("nil response", func(t *testing.T) {
		_, err := translator.TranslateStreamChunk(nil, "req-123", "model", 0, 1234567890)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})

	t.Run("response with error message", func(t *testing.T) {
		resp := &pb.ModelStreamInferResponse{
			ErrorMessage: "something went wrong",
		}
		_, err := translator.TranslateStreamChunk(resp, "req-123", "model", 0, 1234567890)
		if err == nil {
			t.Error("expected error for response with error message")
		}
	})

	t.Run("first chunk includes role", func(t *testing.T) {
		resp := &pb.ModelStreamInferResponse{
			InferResponse: &pb.ModelInferResponse{
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name: TensorOutputTextOutput,
						Contents: &pb.InferTensorContents{
							BytesContents: [][]byte{[]byte("Hello")},
						},
					},
				},
			},
		}
		chunk, err := translator.TranslateStreamChunk(resp, "req-123", "test-model", 0, 1234567890)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if chunk.Choices[0].Delta.Role != "assistant" {
			t.Error("first chunk should have role 'assistant'")
		}
		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("expected object 'chat.completion.chunk', got %q", chunk.Object)
		}
	})

	t.Run("subsequent chunks do not include role", func(t *testing.T) {
		resp := &pb.ModelStreamInferResponse{
			InferResponse: &pb.ModelInferResponse{
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{
					{
						Name: TensorOutputTextOutput,
						Contents: &pb.InferTensorContents{
							BytesContents: [][]byte{[]byte(" world")},
						},
					},
				},
			},
		}
		chunk, err := translator.TranslateStreamChunk(resp, "req-123", "test-model", 1, 1234567890)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if chunk.Choices[0].Delta.Role != "" {
			t.Error("subsequent chunks should not have role")
		}
	})

	t.Run("empty response", func(t *testing.T) {
		resp := &pb.ModelStreamInferResponse{
			InferResponse: &pb.ModelInferResponse{
				Outputs: []*pb.ModelInferResponse_InferOutputTensor{},
			},
		}
		chunk, err := translator.TranslateStreamChunk(resp, "req-123", "test-model", 1, 1234567890)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if chunk.Choices[0].Delta.Content != "" {
			t.Error("expected empty content for empty response")
		}
	})
}

func TestGRPCTranslator_TranslateFinalChunk(t *testing.T) {
	translator, err := NewGRPCTranslator("llama3")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	chunk := translator.TranslateFinalChunk("req-123", "test-model", 1234567890, "stop", 10, 20)

	if chunk == nil {
		t.Fatal("expected chunk, got nil")
	}

	if chunk.Choices[0].FinishReason == nil {
		t.Error("expected finish reason to be set")
		return
	}

	if *chunk.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %q", *chunk.Choices[0].FinishReason)
	}

	if chunk.Usage == nil {
		t.Error("expected usage to be set")
		return
	}

	if chunk.Usage.PromptTokens != 10 {
		t.Errorf("expected prompt tokens 10, got %d", chunk.Usage.PromptTokens)
	}

	if chunk.Usage.CompletionTokens != 20 {
		t.Errorf("expected completion tokens 20, got %d", chunk.Usage.CompletionTokens)
	}

	if chunk.Usage.TotalTokens != 30 {
		t.Errorf("expected total tokens 30, got %d", chunk.Usage.TotalTokens)
	}
}

func TestGRPCTranslator_FormatPrompt(t *testing.T) {
	translator, err := NewGRPCTranslator("llama3")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	t.Run("system and user message", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		}
		prompt, err := translator.formatPrompt(messages)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if prompt == "" {
			t.Error("expected non-empty prompt")
		}
		// Should contain Llama 3 template markers
		if !strings.Contains(prompt, "<|start_header_id|>system<|end_header_id|>") {
			t.Error("expected system header in prompt")
		}
		if !strings.Contains(prompt, "<|start_header_id|>user<|end_header_id|>") {
			t.Error("expected user header in prompt")
		}
		if !strings.Contains(prompt, "<|start_header_id|>assistant<|end_header_id|>") {
			t.Error("expected assistant primer in prompt")
		}
	})
}

func TestGRPCTranslator_TokenCounting(t *testing.T) {
	translator, err := NewGRPCTranslator("llama3")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	t.Run("count prompt tokens", func(t *testing.T) {
		messages := []ChatMessage{
			{Role: "user", Content: "Hello world"},
		}
		tokens := translator.CountPromptTokens(messages)
		if tokens <= 0 {
			t.Error("expected positive token count")
		}
	})

	t.Run("count completion tokens", func(t *testing.T) {
		tokens := translator.CountCompletionTokens("Hello world, this is a test response.")
		if tokens <= 0 {
			t.Error("expected positive token count")
		}
	})
}

func TestNewStreamingConfig(t *testing.T) {
	config := NewStreamingConfig("test-model")

	if config == nil {
		t.Fatal("expected config, got nil")
	}

	if config.RequestID == "" {
		t.Error("expected non-empty request ID")
	}

	if config.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", config.Model)
	}

	if config.Created <= 0 {
		t.Error("expected positive Created timestamp")
	}
}

func TestExtractTextFromGRPCResponse_RawOutputContents(t *testing.T) {
	translator, err := NewGRPCTranslator("llama3")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		resp     *pb.ModelStreamInferResponse
		expected string
		wantErr  bool
	}{
		{
			name: "raw_output_contents format (TRT-LLM style)",
			resp: &pb.ModelStreamInferResponse{
				InferResponse: &pb.ModelInferResponse{
					Outputs: []*pb.ModelInferResponse_InferOutputTensor{
						{
							Name:     TensorOutputTextOutput,
							Datatype: DatatypeBYTES,
							Shape:    []int64{1},
							// Contents is nil - TRT-LLM doesn't use this
							Contents: nil,
						},
					},
					// TRT-LLM puts the text here for performance
					RawOutputContents: [][]byte{
						[]byte("Hello, world!"),
					},
				},
			},
			expected: "Hello, world!",
			wantErr:  false,
		},
		{
			name: "structured contents format (legacy style)",
			resp: &pb.ModelStreamInferResponse{
				InferResponse: &pb.ModelInferResponse{
					Outputs: []*pb.ModelInferResponse_InferOutputTensor{
						{
							Name:     TensorOutputTextOutput,
							Datatype: DatatypeBYTES,
							Shape:    []int64{1},
							Contents: &pb.InferTensorContents{
								BytesContents: [][]byte{
									[]byte("Structured response"),
								},
							},
						},
					},
					RawOutputContents: nil,
				},
			},
			expected: "Structured response",
			wantErr:  false,
		},
		{
			name: "both formats present - prefer structured contents",
			resp: &pb.ModelStreamInferResponse{
				InferResponse: &pb.ModelInferResponse{
					Outputs: []*pb.ModelInferResponse_InferOutputTensor{
						{
							Name:     TensorOutputTextOutput,
							Datatype: DatatypeBYTES,
							Shape:    []int64{1},
							Contents: &pb.InferTensorContents{
								BytesContents: [][]byte{
									[]byte("Structured"),
								},
							},
						},
					},
					RawOutputContents: [][]byte{
						[]byte("Raw"),
					},
				},
			},
			expected: "Structured",
			wantErr:  false,
		},
		{
			name: "empty response",
			resp: &pb.ModelStreamInferResponse{
				InferResponse: &pb.ModelInferResponse{
					Outputs:           []*pb.ModelInferResponse_InferOutputTensor{},
					RawOutputContents: nil,
				},
			},
			expected: "",
			wantErr:  false,
		},
		{
			name: "nil InferResponse",
			resp: &pb.ModelStreamInferResponse{
				InferResponse: nil,
			},
			expected: "",
			wantErr:  false,
		},
		{
			name: "multiple outputs - uses first text_output",
			resp: &pb.ModelStreamInferResponse{
				InferResponse: &pb.ModelInferResponse{
					Outputs: []*pb.ModelInferResponse_InferOutputTensor{
						{
							Name:     "other_output",
							Datatype: DatatypeBYTES,
						},
						{
							Name:     TensorOutputTextOutput,
							Datatype: DatatypeBYTES,
						},
					},
					RawOutputContents: [][]byte{
						[]byte("other data"),
						[]byte("text output data"),
					},
				},
			},
			expected: "text output data",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := translator.ExtractTextFromGRPCResponse(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractTextFromGRPCResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if text != tt.expected {
				t.Errorf("ExtractTextFromGRPCResponse() = %q, want %q", text, tt.expected)
			}
		})
	}
}
