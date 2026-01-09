package triton

import (
	"testing"
)

func TestNewTranslator(t *testing.T) {
	tests := []struct {
		name        string
		encoding    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid encoding cl100k_base",
			encoding: "cl100k_base",
			wantErr:  false,
		},
		{
			name:     "valid encoding llama3",
			encoding: "llama3",
			wantErr:  false,
		},
		{
			name:     "valid encoding o200k_base",
			encoding: "o200k_base",
			wantErr:  false,
		},
		{
			name:        "invalid encoding",
			encoding:    "invalid_encoding",
			wantErr:     true,
			errContains: "unsupported tokenizer encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator, err := NewTranslator(tt.encoding)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewTranslator() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewTranslator() unexpected error: %v", err)
				return
			}
			if translator == nil {
				t.Errorf("NewTranslator() returned nil translator")
			}
		})
	}
}

func TestTranslateOpenAIToTriton(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	tests := []struct {
		name    string
		req     *OpenAIChatCompletionRequest
		wantErr bool
	}{
		{
			name: "basic request",
			req: &OpenAIChatCompletionRequest{
				Model: "test-model",
				Messages: []ChatMessage{
					{Role: "user", Content: "Hello, world!"},
				},
			},
			wantErr: false,
		},
		{
			name: "request with all parameters",
			req: &OpenAIChatCompletionRequest{
				Model: "test-model",
				Messages: []ChatMessage{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "What is 2+2?"},
				},
				Temperature: ptrFloat64(0.7),
				MaxTokens:   ptrInt(100),
				TopP:        ptrFloat64(0.9),
				Stop:        []string{".", "!"},
			},
			wantErr: false,
		},
		{
			name: "ensemble model compatibility - only text_input and max_tokens",
			req: &OpenAIChatCompletionRequest{
				Model: "test-model",
				Messages: []ChatMessage{
					{Role: "user", Content: "Hello"},
				},
				Temperature: ptrFloat64(0.7),
				TopP:        ptrFloat64(0.9),
				Stop:        []string{"STOP"},
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "empty messages",
			req: &OpenAIChatCompletionRequest{
				Model:    "test-model",
				Messages: []ChatMessage{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translator.TranslateOpenAIToTriton(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateOpenAIToTriton() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("TranslateOpenAIToTriton() unexpected error: %v", err)
				return
			}

			// Verify request structure
			if result.ID == "" {
				t.Error("expected non-empty request ID")
			}

			// TensorRT-LLM ensemble models only accept 2 inputs: text_input and max_tokens
			// This prevents "expected 2 inputs but got N inputs" errors
			if len(result.Inputs) != 2 {
				t.Errorf("len(Inputs) = %d, want 2 (text_input, max_tokens only)", len(result.Inputs))
			}

			// Check for text_input tensor
			foundTextInput := false
			foundMaxTokens := false
			for _, input := range result.Inputs {
				if input.Name == TensorInputTextInput {
					foundTextInput = true
					if input.Datatype != DatatypeBYTES {
						t.Errorf("text_input datatype = %s, want BYTES", input.Datatype)
					}
				}
				if input.Name == TensorInputMaxTokens {
					foundMaxTokens = true
					if input.Datatype != DatatypeINT32 {
						t.Errorf("max_tokens datatype = %s, want INT32", input.Datatype)
					}
				}
			}
			if !foundTextInput {
				t.Error("text_input tensor not found in inputs")
			}
			if !foundMaxTokens {
				t.Error("max_tokens tensor not found in inputs")
			}
		})
	}
}

func TestTranslateTritonToOpenAI(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	originalReq := &OpenAIChatCompletionRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello!"},
		},
	}

	tests := []struct {
		name    string
		resp    *InferResponse
		wantErr bool
	}{
		{
			name: "basic response",
			resp: &InferResponse{
				ID:        "test-id",
				ModelName: "test-model",
				Outputs: []InferOutputTensor{
					{
						Name:     TensorOutputTextOutput,
						Shape:    []int64{1},
						Datatype: DatatypeBYTES,
						Data:     []interface{}{"Hello! How can I help you today?"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "response with alternative output name",
			resp: &InferResponse{
				ID:        "test-id",
				ModelName: "test-model",
				Outputs: []InferOutputTensor{
					{
						Name:     "generated_text",
						Shape:    []int64{1},
						Datatype: DatatypeBYTES,
						Data:     []interface{}{"Generated response"},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil response",
			resp:    nil,
			wantErr: true,
		},
		{
			name: "empty outputs",
			resp: &InferResponse{
				ID:        "test-id",
				ModelName: "test-model",
				Outputs:   []InferOutputTensor{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translator.TranslateTritonToOpenAI(tt.resp, originalReq)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateTritonToOpenAI() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("TranslateTritonToOpenAI() unexpected error: %v", err)
				return
			}

			// Verify response structure
			if result.Object != "chat.completion" {
				t.Errorf("Object = %s, want chat.completion", result.Object)
			}
			if len(result.Choices) != 1 {
				t.Errorf("len(Choices) = %d, want 1", len(result.Choices))
				return
			}
			if result.Choices[0].Message.Role != "assistant" {
				t.Errorf("Message.Role = %s, want assistant", result.Choices[0].Message.Role)
			}
			if result.Choices[0].Message.Content == "" {
				t.Error("expected non-empty message content")
			}
			if result.Usage.TotalTokens == 0 {
				t.Error("expected non-zero total tokens")
			}
		})
	}
}

func TestFormatPrompt(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	tests := []struct {
		name     string
		messages []ChatMessage
		wantErr  bool
		contains []string
	}{
		{
			name: "single user message",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			wantErr:  false,
			contains: []string{"user", "Hello", "assistant"},
		},
		{
			name: "system and user messages",
			messages: []ChatMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hi there"},
			},
			wantErr:  false,
			contains: []string{"system", "You are helpful", "user", "Hi there"},
		},
		{
			name:     "empty messages",
			messages: []ChatMessage{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := translator.formatPrompt(tt.messages)
			if tt.wantErr {
				if err == nil {
					t.Errorf("formatPrompt() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("formatPrompt() unexpected error: %v", err)
				return
			}

			for _, substr := range tt.contains {
				if !contains(result, substr) {
					t.Errorf("formatPrompt() result does not contain %q", substr)
				}
			}
		})
	}
}

func TestSerializeAndParseTritonRequest(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	req := &OpenAIChatCompletionRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "Test message"},
		},
		Temperature: ptrFloat64(0.5),
		MaxTokens:   ptrInt(50),
	}

	// Translate to Triton format
	tritonReq, err := translator.TranslateOpenAIToTriton(req)
	if err != nil {
		t.Fatalf("TranslateOpenAIToTriton() error: %v", err)
	}

	// Serialize
	data, err := translator.SerializeTritonRequest(tritonReq)
	if err != nil {
		t.Fatalf("SerializeTritonRequest() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("SerializeTritonRequest() returned empty data")
	}
}

func TestParseTritonResponse(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	jsonData := []byte(`{
		"id": "test-123",
		"model_name": "llama",
		"outputs": [
			{
				"name": "text_output",
				"shape": [1],
				"datatype": "BYTES",
				"data": ["Hello, I am an AI assistant."]
			}
		]
	}`)

	resp, err := translator.ParseTritonResponse(jsonData)
	if err != nil {
		t.Fatalf("ParseTritonResponse() error: %v", err)
	}

	if resp.ID != "test-123" {
		t.Errorf("ID = %s, want test-123", resp.ID)
	}
	if resp.ModelName != "llama" {
		t.Errorf("ModelName = %s, want llama", resp.ModelName)
	}
	if len(resp.Outputs) != 1 {
		t.Errorf("len(Outputs) = %d, want 1", len(resp.Outputs))
	}
}

// Helper functions
func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrInt(v int) *int {
	return &v
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
