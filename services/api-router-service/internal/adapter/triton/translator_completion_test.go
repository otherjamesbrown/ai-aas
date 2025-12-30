package triton

import (
	"testing"
)

func TestTranslateOpenAICompletionToTriton(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	tests := []struct {
		name    string
		req     *OpenAICompletionRequest
		wantErr bool
	}{
		{
			name: "basic text completion request",
			req: &OpenAICompletionRequest{
				Model:  "test-model",
				Prompt: "What is the capital of France?",
			},
			wantErr: false,
		},
		{
			name: "request with all parameters",
			req: &OpenAICompletionRequest{
				Model:       "test-model",
				Prompt:      "Tell me a joke",
				Temperature: ptrFloat64(0.7),
				MaxTokens:   ptrInt(100),
				TopP:        ptrFloat64(0.9),
				Stop:        []string{".", "!"},
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "empty prompt",
			req: &OpenAICompletionRequest{
				Model:  "test-model",
				Prompt: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tritonReq, err := translator.TranslateOpenAICompletionToTriton(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateOpenAICompletionToTriton() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("TranslateOpenAICompletionToTriton() unexpected error: %v", err)
				return
			}
			if tritonReq == nil {
				t.Errorf("TranslateOpenAICompletionToTriton() returned nil request")
				return
			}

			// Verify basic structure
			if tritonReq.ID == "" {
				t.Errorf("TranslateOpenAICompletionToTriton() request has no ID")
			}
			if len(tritonReq.Inputs) == 0 {
				t.Errorf("TranslateOpenAICompletionToTriton() request has no inputs")
			}

			// Verify text_input tensor exists
			foundTextInput := false
			for _, input := range tritonReq.Inputs {
				if input.Name == TensorInputTextInput {
					foundTextInput = true
					if len(input.Data) == 0 {
						t.Errorf("TranslateOpenAICompletionToTriton() text_input has no data")
					}
				}
			}
			if !foundTextInput {
				t.Errorf("TranslateOpenAICompletionToTriton() missing text_input tensor")
			}
		})
	}
}

func TestTranslateTritonToOpenAICompletion(t *testing.T) {
	translator, err := NewTranslator("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	tests := []struct {
		name        string
		tritonResp  *InferResponse
		originalReq *OpenAICompletionRequest
		wantErr     bool
		wantText    string
	}{
		{
			name: "basic response",
			tritonResp: &InferResponse{
				ID: "test-id",
				Outputs: []InferOutputTensor{
					{
						Name:     TensorOutputTextOutput,
						Shape:    []int64{1},
						Datatype: DatatypeBYTES,
						Data:     []interface{}{"Paris"},
					},
				},
			},
			originalReq: &OpenAICompletionRequest{
				Model:  "test-model",
				Prompt: "What is the capital of France?",
			},
			wantErr:  false,
			wantText: "Paris",
		},
		{
			name:       "nil response",
			tritonResp: nil,
			originalReq: &OpenAICompletionRequest{
				Model:  "test-model",
				Prompt: "Test",
			},
			wantErr: true,
		},
		{
			name: "no outputs",
			tritonResp: &InferResponse{
				ID:      "test-id",
				Outputs: []InferOutputTensor{},
			},
			originalReq: &OpenAICompletionRequest{
				Model:  "test-model",
				Prompt: "Test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openAIResp, err := translator.TranslateTritonToOpenAICompletion(tt.tritonResp, tt.originalReq)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateTritonToOpenAICompletion() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("TranslateTritonToOpenAICompletion() unexpected error: %v", err)
				return
			}
			if openAIResp == nil {
				t.Errorf("TranslateTritonToOpenAICompletion() returned nil response")
				return
			}

			// Verify basic structure
			if openAIResp.ID == "" {
				t.Errorf("TranslateTritonToOpenAICompletion() response has no ID")
			}
			if openAIResp.Object != "text_completion" {
				t.Errorf("TranslateTritonToOpenAICompletion() object = %q, want 'text_completion'", openAIResp.Object)
			}
			if len(openAIResp.Choices) == 0 {
				t.Errorf("TranslateTritonToOpenAICompletion() response has no choices")
			}

			// Verify completion text
			if len(openAIResp.Choices) > 0 {
				if openAIResp.Choices[0].Text != tt.wantText {
					t.Errorf("TranslateTritonToOpenAICompletion() text = %q, want %q",
						openAIResp.Choices[0].Text, tt.wantText)
				}
			}

			// Verify usage info
			if openAIResp.Usage.PromptTokens == 0 {
				t.Errorf("TranslateTritonToOpenAICompletion() prompt tokens should not be 0")
			}
		})
	}
}
