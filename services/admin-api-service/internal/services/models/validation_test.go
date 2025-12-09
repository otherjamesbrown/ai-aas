// Package models provides the business logic for model management operations.
package models

import (
	"testing"
)

func TestValidateKServeName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid lowercase with hyphens",
			input:   "llama-3-8b",
			wantErr: false,
		},
		{
			name:    "valid single letter",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "gpt-4o-mini",
			wantErr: false,
		},
		{
			name:    "invalid - starts with number",
			input:   "123-model",
			wantErr: true,
		},
		{
			name:    "invalid - uppercase letters",
			input:   "GPT-4",
			wantErr: true,
		},
		{
			name:    "invalid - contains underscore",
			input:   "llama_3",
			wantErr: true,
		},
		{
			name:    "invalid - starts with hyphen",
			input:   "-model",
			wantErr: true,
		},
		{
			name:    "invalid - ends with hyphen",
			input:   "model-",
			wantErr: true,
		},
		{
			name:    "invalid - empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "valid - complex name",
			input:   "mistral-7b-instruct-v03",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKServeName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKServeName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
