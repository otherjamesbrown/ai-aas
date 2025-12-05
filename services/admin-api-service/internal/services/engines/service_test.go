package engines

import (
	"encoding/json"
	"testing"
)

func TestEngineConfigMarshal(t *testing.T) {
	cfg := EngineConfig{
		Name:       "vllm/default",
		GPUCount:   1,
		MemoryGB:   16,
		CPUCores:   4,
		EngineArgs: json.RawMessage(`{"max-model-len": 32768}`),
	}

	if cfg.Name != "vllm/default" {
		t.Errorf("expected name vllm/default, got %s", cfg.Name)
	}
	if cfg.GPUCount != 1 {
		t.Errorf("expected gpu_count 1, got %d", cfg.GPUCount)
	}
	if cfg.MemoryGB != 16 {
		t.Errorf("expected memory_gb 16, got %d", cfg.MemoryGB)
	}
}

func TestEngineVersionFields(t *testing.T) {
	minMem := 16
	v := EngineVersion{
		Version:        "0.6.4",
		ContainerImage: "vllm/vllm-openai:v0.6.4",
		IsDefault:      true,
		MinGPUMemoryGB: &minMem,
	}

	if v.Version != "0.6.4" {
		t.Errorf("expected version 0.6.4, got %s", v.Version)
	}
	if v.ContainerImage != "vllm/vllm-openai:v0.6.4" {
		t.Errorf("expected container image, got %s", v.ContainerImage)
	}
	if !v.IsDefault {
		t.Error("expected is_default to be true")
	}
	if v.MinGPUMemoryGB == nil || *v.MinGPUMemoryGB != 16 {
		t.Error("expected min_gpu_memory_gb to be 16")
	}
}

func TestEngineFields(t *testing.T) {
	e := Engine{
		Name:                  "vllm",
		DisplayName:           "vLLM OpenAI Server",
		DefaultVersion:        "0.6.4",
		ProtocolVersions:      []string{"v2"},
		SupportedModelFormats: []string{"vllm", "pytorch"},
	}

	if e.Name != "vllm" {
		t.Errorf("expected name vllm, got %s", e.Name)
	}
	if len(e.ProtocolVersions) != 1 || e.ProtocolVersions[0] != "v2" {
		t.Errorf("expected protocol versions [v2], got %v", e.ProtocolVersions)
	}
	if len(e.SupportedModelFormats) != 2 {
		t.Errorf("expected 2 model formats, got %d", len(e.SupportedModelFormats))
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		input []string
		sep   string
		want  string
	}{
		{[]string{}, ", ", ""},
		{[]string{"a"}, ", ", "a"},
		{[]string{"a", "b"}, ", ", "a, b"},
		{[]string{"a", "b", "c"}, " AND ", "a AND b AND c"},
	}

	for _, tt := range tests {
		got := joinStrings(tt.input, tt.sep)
		if got != tt.want {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.input, tt.sep, got, tt.want)
		}
	}
}
