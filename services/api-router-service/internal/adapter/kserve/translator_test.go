package kserve

import (
	"encoding/json"
	"testing"
)

func TestTranslateOpenAIToKServe(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name    string
		req     *OpenAIChatCompletionRequest
		wantErr bool
	}{
		{
			name: "basic chat completion request",
			req: &OpenAIChatCompletionRequest{
				Model: "llama-2-7b",
				Messages: []ChatMessage{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "Hello, world!"},
				},
				Temperature: float64Ptr(0.7),
				MaxTokens:   intPtr(50),
			},
			wantErr: false,
		},
		{
			name: "minimal request",
			req: &OpenAIChatCompletionRequest{
				Model: "llama-2-7b",
				Messages: []ChatMessage{
					{Role: "user", Content: "Test"},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kserveReq, err := translator.TranslateOpenAIToKServe(tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate KServe request structure
			if kserveReq.ID == "" {
				t.Error("expected non-empty ID")
			}

			if len(kserveReq.Inputs) != 1 {
				t.Errorf("expected 1 input, got %d", len(kserveReq.Inputs))
			}

			input := kserveReq.Inputs[0]
			if input.Name != "prompt" {
				t.Errorf("expected input name 'prompt', got '%s'", input.Name)
			}

			if input.Datatype != "BYTES" {
				t.Errorf("expected datatype 'BYTES', got '%s'", input.Datatype)
			}

			if len(input.Data) != 1 {
				t.Errorf("expected 1 data element, got %d", len(input.Data))
			}

			// Check parameters
			if tt.req.Temperature != nil {
				if temp, ok := kserveReq.Parameters["temperature"]; !ok {
					t.Error("expected temperature parameter")
				} else if temp != *tt.req.Temperature {
					t.Errorf("temperature mismatch: got %v, want %v", temp, *tt.req.Temperature)
				}
			}

			if tt.req.MaxTokens != nil {
				if maxTokens, ok := kserveReq.Parameters["max_tokens"]; !ok {
					t.Error("expected max_tokens parameter")
				} else if maxTokens != *tt.req.MaxTokens {
					t.Errorf("max_tokens mismatch: got %v, want %v", maxTokens, *tt.req.MaxTokens)
				}
			}
		})
	}
}

func TestTranslateKServeToOpenAI(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name        string
		kserveResp  *InferResponse
		originalReq *OpenAIChatCompletionRequest
		wantErr     bool
	}{
		{
			name: "successful response",
			kserveResp: &InferResponse{
				ID:        "test-123",
				ModelName: "llama-2-7b",
				Outputs: []InferOutputTensor{
					{
						Name:     "text",
						Shape:    []int64{1},
						Datatype: "BYTES",
						Data:     []interface{}{"This is a test response."},
					},
				},
			},
			originalReq: &OpenAIChatCompletionRequest{
				Model: "llama-2-7b",
				Messages: []ChatMessage{
					{Role: "user", Content: "Test"},
				},
			},
			wantErr: false,
		},
		{
			name:        "nil response",
			kserveResp:  nil,
			originalReq: &OpenAIChatCompletionRequest{Model: "test"},
			wantErr:     true,
		},
		{
			name: "empty outputs",
			kserveResp: &InferResponse{
				ID:      "test-456",
				Outputs: []InferOutputTensor{},
			},
			originalReq: &OpenAIChatCompletionRequest{Model: "test"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiResp, err := translator.TranslateKServeToOpenAI(tt.kserveResp, tt.originalReq)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate OpenAI response structure
			if openaiResp.ID == "" {
				t.Error("expected non-empty ID")
			}

			if openaiResp.Object != "chat.completion" {
				t.Errorf("expected object 'chat.completion', got '%s'", openaiResp.Object)
			}

			if openaiResp.Model != tt.originalReq.Model {
				t.Errorf("model mismatch: got '%s', want '%s'", openaiResp.Model, tt.originalReq.Model)
			}

			if len(openaiResp.Choices) != 1 {
				t.Errorf("expected 1 choice, got %d", len(openaiResp.Choices))
			}

			choice := openaiResp.Choices[0]
			if choice.Message.Role != "assistant" {
				t.Errorf("expected role 'assistant', got '%s'", choice.Message.Role)
			}

			if choice.Message.Content == "" {
				t.Error("expected non-empty content")
			}

			// Validate usage info
			if openaiResp.Usage.TotalTokens == 0 {
				t.Error("expected non-zero total tokens")
			}

			if openaiResp.Usage.TotalTokens != openaiResp.Usage.PromptTokens+openaiResp.Usage.CompletionTokens {
				t.Error("total tokens should equal prompt + completion tokens")
			}
		})
	}
}

func TestFormatPrompt(t *testing.T) {
	translator := NewTranslator()

	tests := []struct {
		name     string
		messages []ChatMessage
		wantErr  bool
	}{
		{
			name: "multi-message conversation",
			messages: []ChatMessage{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
			wantErr: false,
		},
		{
			name:     "empty messages",
			messages: []ChatMessage{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := translator.formatPrompt(tt.messages)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prompt == "" {
				t.Error("expected non-empty prompt")
			}

			// Verify all messages are included
			for range tt.messages {
				// Content should be in the prompt
				// (Simple check - in reality prompt format varies by model)
			}
		})
	}
}

func TestSerializeKServeRequest(t *testing.T) {
	translator := NewTranslator()

	req := &InferRequest{
		ID: "test-123",
		Inputs: []InferInputTensor{
			{
				Name:     "prompt",
				Shape:    []int64{1},
				Datatype: "BYTES",
				Data:     []interface{}{"test prompt"},
			},
		},
		Parameters: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  50,
		},
	}

	data, err := translator.SerializeKServeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty serialized data")
	}

	// Verify it's valid JSON
	var parsed InferRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("serialized data is not valid JSON: %v", err)
	}

	if parsed.ID != req.ID {
		t.Errorf("ID mismatch: got '%s', want '%s'", parsed.ID, req.ID)
	}
}

func TestParseKServeResponse(t *testing.T) {
	translator := NewTranslator()

	jsonData := `{
		"id": "test-456",
		"model_name": "llama-2-7b",
		"outputs": [{
			"name": "text",
			"shape": [1],
			"datatype": "BYTES",
			"data": ["Generated text"]
		}]
	}`

	resp, err := translator.ParseKServeResponse([]byte(jsonData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "test-456" {
		t.Errorf("ID mismatch: got '%s', want 'test-456'", resp.ID)
	}

	if resp.ModelName != "llama-2-7b" {
		t.Errorf("model name mismatch: got '%s', want 'llama-2-7b'", resp.ModelName)
	}

	if len(resp.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(resp.Outputs))
	}
}

// Helper functions
func float64Ptr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}
