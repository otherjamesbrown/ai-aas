package triton

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChatMessage_NormalizeContent tests the NormalizeContent method for reasoning models.
// Reasoning models (like gpt-oss-20b) may return content in reasoning_content field instead of content.
// This test validates the fallback logic: content -> reasoning -> reasoning_content -> empty string.
func TestChatMessage_NormalizeContent(t *testing.T) {
	tests := []struct {
		name     string
		message  ChatMessage
		expected interface{}
	}{
		{
			name: "content field has value",
			message: ChatMessage{
				Role:             "assistant",
				Content:          "Hello from content",
				Reasoning:        "Hello from reasoning",
				ReasoningContent: "Hello from reasoning_content",
			},
			expected: "Hello from content",
		},
		{
			name: "content is empty, use reasoning",
			message: ChatMessage{
				Role:             "assistant",
				Content:          "",
				Reasoning:        "Hello from reasoning",
				ReasoningContent: "Hello from reasoning_content",
			},
			expected: "Hello from reasoning",
		},
		{
			name: "content and reasoning empty, use reasoning_content",
			message: ChatMessage{
				Role:             "assistant",
				Content:          "",
				Reasoning:        "",
				ReasoningContent: "Hello from reasoning_content",
			},
			expected: "Hello from reasoning_content",
		},
		{
			name: "content is nil, use reasoning",
			message: ChatMessage{
				Role:             "assistant",
				Content:          nil,
				Reasoning:        "Hello from reasoning",
				ReasoningContent: "Hello from reasoning_content",
			},
			expected: "Hello from reasoning",
		},
		{
			name: "content and reasoning nil, use reasoning_content",
			message: ChatMessage{
				Role:             "assistant",
				Content:          nil,
				Reasoning:        nil,
				ReasoningContent: "Hello from reasoning_content",
			},
			expected: "Hello from reasoning_content",
		},
		{
			name: "all fields empty, return empty string",
			message: ChatMessage{
				Role:             "assistant",
				Content:          "",
				Reasoning:        "",
				ReasoningContent: "",
			},
			expected: "",
		},
		{
			name: "all fields nil, return empty string",
			message: ChatMessage{
				Role:             "assistant",
				Content:          nil,
				Reasoning:        nil,
				ReasoningContent: nil,
			},
			expected: "",
		},
		{
			name: "multimodal content with parts",
			message: ChatMessage{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "Hello"},
				},
				Reasoning:        "",
				ReasoningContent: "",
			},
			expected: []interface{}{
				map[string]interface{}{"type": "text", "text": "Hello"},
			},
		},
		{
			name: "empty array content, use reasoning",
			message: ChatMessage{
				Role:             "assistant",
				Content:          []interface{}{},
				Reasoning:        "Hello from reasoning",
				ReasoningContent: "",
			},
			expected: "Hello from reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.message.NormalizeContent()
			assert.Equal(t, tt.expected, result, "NormalizeContent() should return the expected value")
		})
	}
}

// TestChatMessage_GetTextContent tests the GetTextContent method with various content types.
func TestChatMessage_GetTextContent(t *testing.T) {
	tests := []struct {
		name     string
		message  ChatMessage
		expected string
	}{
		{
			name: "simple string content",
			message: ChatMessage{
				Role:    "user",
				Content: "Hello, world!",
			},
			expected: "Hello, world!",
		},
		{
			name: "multimodal content with text parts",
			message: ChatMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "First part"},
					map[string]interface{}{"type": "text", "text": "Second part"},
				},
			},
			expected: "First part Second part",
		},
		{
			name: "multimodal content with mixed types",
			message: ChatMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "Text content"},
					map[string]interface{}{"type": "image_url", "image_url": map[string]string{"url": "http://example.com/image.png"}},
				},
			},
			expected: "Text content",
		},
		{
			name: "empty content",
			message: ChatMessage{
				Role:    "user",
				Content: "",
			},
			expected: "",
		},
		{
			name: "nil content",
			message: ChatMessage{
				Role:    "user",
				Content: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.message.GetTextContent()
			assert.Equal(t, tt.expected, result, "GetTextContent() should extract text correctly")
		})
	}
}
