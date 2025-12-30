package model

import "testing"

func TestSanitizeModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "periods in version numbers",
			input:    "Mistral-7B-Instruct-v0.3",
			expected: "mistral-7b-instruct-v0-3",
		},
		{
			name:     "underscores",
			input:    "gpt_oss_20b",
			expected: "gpt-oss-20b",
		},
		{
			name:     "mixed periods and underscores",
			input:    "Model_Name.v1.2",
			expected: "model-name-v1-2",
		},
		{
			name:     "consecutive hyphens from periods and underscores",
			input:    "model_.name",
			expected: "model-name",
		},
		{
			name:     "uppercase to lowercase",
			input:    "LLAMA-3-8B",
			expected: "llama-3-8b",
		},
		{
			name:     "leading and trailing special chars",
			input:    "_model.name_",
			expected: "model-name",
		},
		{
			name:     "already sanitized",
			input:    "llama-3-8b",
			expected: "llama-3-8b",
		},
		{
			name:     "complex real-world example",
			input:    "Mistral-7B-Instruct-v0.3",
			expected: "mistral-7b-instruct-v0-3",
		},
		{
			name:     "multiple consecutive periods",
			input:    "model...name",
			expected: "model-name",
		},
		{
			name:     "multiple consecutive underscores",
			input:    "model___name",
			expected: "model-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeModelName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeModelName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
