package triton

import (
	"testing"
)

func TestNewTokenizer(t *testing.T) {
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
			encoding:    "invalid",
			wantErr:     true,
			errContains: "unsupported",
		},
		{
			name:        "empty encoding",
			encoding:    "",
			wantErr:     true,
			errContains: "unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTokenizer(tt.encoding)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewTokenizer() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewTokenizer() unexpected error: %v", err)
				return
			}
			if tokenizer == nil {
				t.Error("NewTokenizer() returned nil tokenizer")
			}
			if tokenizer.EncodingName() != tt.encoding {
				t.Errorf("EncodingName() = %s, want %s", tokenizer.EncodingName(), tt.encoding)
			}
		})
	}
}

func TestCountTokens(t *testing.T) {
	tokenizer, err := NewTokenizer("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name    string
		text    string
		wantMin int // Minimum expected tokens (exact count varies by tokenizer)
		wantMax int // Maximum expected tokens
	}{
		{
			name:    "empty string",
			text:    "",
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "single word",
			text:    "Hello",
			wantMin: 1,
			wantMax: 2,
		},
		{
			name:    "simple sentence",
			text:    "Hello, world!",
			wantMin: 2,
			wantMax: 5,
		},
		{
			name:    "longer text",
			text:    "The quick brown fox jumps over the lazy dog.",
			wantMin: 5,
			wantMax: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := tokenizer.CountTokens(tt.text)
			if count < tt.wantMin || count > tt.wantMax {
				t.Errorf("CountTokens() = %d, want between %d and %d", count, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCountMessagesTokens(t *testing.T) {
	tokenizer, err := NewTokenizer("cl100k_base")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name     string
		messages []ChatMessage
		wantMin  int
	}{
		{
			name:     "empty messages",
			messages: []ChatMessage{},
			wantMin:  0,
		},
		{
			name: "single message",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			wantMin: 5, // Role + content + overhead
		},
		{
			name: "multiple messages",
			messages: []ChatMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hi there"},
			},
			wantMin: 10, // Multiple messages with overhead
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := tokenizer.CountMessagesTokens(tt.messages)
			if count < tt.wantMin {
				t.Errorf("CountMessagesTokens() = %d, want at least %d", count, tt.wantMin)
			}
		})
	}
}

func TestTokenizerCaching(t *testing.T) {
	// First call should create tokenizer
	t1, err := NewTokenizer("cl100k_base")
	if err != nil {
		t.Fatalf("first NewTokenizer() error: %v", err)
	}

	// Second call should return cached tokenizer
	t2, err := NewTokenizer("cl100k_base")
	if err != nil {
		t.Fatalf("second NewTokenizer() error: %v", err)
	}

	// Should be the same instance
	if t1 != t2 {
		t.Error("expected cached tokenizer to be returned")
	}
}
