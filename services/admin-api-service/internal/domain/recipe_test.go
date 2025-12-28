package domain

import (
	"testing"
)

func TestIsValidRuntime(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		want    bool
	}{
		{
			name:    "valid runtime - vllm",
			runtime: "vllm",
			want:    true,
		},
		{
			name:    "valid runtime - triton",
			runtime: "triton",
			want:    true,
		},
		{
			name:    "valid runtime - tensorrt-llm",
			runtime: "tensorrt-llm",
			want:    true,
		},
		{
			name:    "valid runtime - tgi",
			runtime: "tgi",
			want:    true,
		},
		{
			name:    "invalid runtime - unknown",
			runtime: "unknown",
			want:    false,
		},
		{
			name:    "invalid runtime - pytorch",
			runtime: "pytorch",
			want:    false,
		},
		{
			name:    "invalid runtime - tensorflow",
			runtime: "tensorflow",
			want:    false,
		},
		{
			name:    "invalid runtime - empty string",
			runtime: "",
			want:    false,
		},
		{
			name:    "invalid runtime - VLLM uppercase",
			runtime: "VLLM",
			want:    false,
		},
		{
			name:    "invalid runtime - Vllm mixed case",
			runtime: "Vllm",
			want:    false,
		},
		{
			name:    "invalid runtime - with whitespace",
			runtime: " vllm ",
			want:    false,
		},
		{
			name:    "invalid runtime - vLLM",
			runtime: "vLLM",
			want:    false,
		},
		{
			name:    "invalid runtime - TensorRT-LLM mixed case",
			runtime: "TensorRT-LLM",
			want:    false,
		},
		{
			name:    "invalid runtime - tensorrt_llm underscore",
			runtime: "tensorrt_llm",
			want:    false,
		},
		{
			name:    "invalid runtime - TGI uppercase",
			runtime: "TGI",
			want:    false,
		},
		{
			name:    "invalid runtime - onnx",
			runtime: "onnx",
			want:    false,
		},
		{
			name:    "invalid runtime - custom",
			runtime: "custom",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidRuntime(tt.runtime)
			if got != tt.want {
				t.Errorf("IsValidRuntime(%q) = %v, want %v", tt.runtime, got, tt.want)
			}
		})
	}
}

func TestValidRuntimes(t *testing.T) {
	// Ensure the list contains expected values
	expected := []string{"vllm", "triton", "tensorrt-llm", "tgi"}

	if len(ValidRuntimes) != len(expected) {
		t.Errorf("ValidRuntimes length = %d, want %d", len(ValidRuntimes), len(expected))
	}

	// Check all expected values are present
	for _, exp := range expected {
		found := false
		for _, runtime := range ValidRuntimes {
			if runtime == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidRuntimes missing expected runtime: %q", exp)
		}
	}
}

func TestValidRuntimesNoDuplicates(t *testing.T) {
	// Ensure there are no duplicate entries in ValidRuntimes
	seen := make(map[string]bool)
	for _, runtime := range ValidRuntimes {
		if seen[runtime] {
			t.Errorf("ValidRuntimes contains duplicate: %q", runtime)
		}
		seen[runtime] = true
	}
}

func TestAllValidRuntimesPassValidation(t *testing.T) {
	// Ensure all entries in ValidRuntimes pass IsValidRuntime
	for _, runtime := range ValidRuntimes {
		if !IsValidRuntime(runtime) {
			t.Errorf("ValidRuntimes contains %q but IsValidRuntime(%q) returns false", runtime, runtime)
		}
	}
}
