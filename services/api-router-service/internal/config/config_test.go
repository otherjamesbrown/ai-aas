package config

import (
	"testing"
	"time"
)

// TestBackendEndpointConfig_GetHealthPath verifies that the correct health check path
// is detected based on backend URI patterns.
//
// Background:
// - vLLM backends use /health (OpenAI-compatible)
// - Triton/TensorRT-LLM backends use /v2/health/ready (KServe v2 protocol)
//
// Related bead: aas-gadbx
func TestBackendEndpointConfig_GetHealthPath(t *testing.T) {
	tests := []struct {
		name           string
		config         BackendEndpointConfig
		expectedPath   string
		expectedReason string
	}{
		// Explicit health path should be respected
		{
			name: "Explicit health path set",
			config: BackendEndpointConfig{
				ID:         "custom",
				URI:        "http://custom-backend.svc.cluster.local:80",
				HealthPath: "/custom/health",
				Timeout:    5 * time.Second,
			},
			expectedPath:   "/custom/health",
			expectedReason: "explicit HealthPath should be used",
		},

		// Triton/TensorRT-LLM patterns - should use /v2/health/ready
		{
			name: "TensorRT-LLM explicit",
			config: BackendEndpointConfig{
				ID:      "llama-trtllm",
				URI:     "http://llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'trtllm'",
		},
		{
			name: "TensorRT-LLM with TRT shorthand",
			config: BackendEndpointConfig{
				ID:      "model-trt",
				URI:     "http://model-trt-llm-predictor.system.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains '-trt-'",
		},
		{
			name: "Multi-model Triton deployment",
			config: BackendEndpointConfig{
				ID:      "multi-model",
				URI:     "http://multi-model-3x8b-blackwell-predictor.system.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'multi-model'",
		},
		{
			name: "Triton explicit naming",
			config: BackendEndpointConfig{
				ID:      "triton",
				URI:     "http://triton-inference-server.system.svc.cluster.local:8000",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'triton'",
		},
		{
			name: "TensorRT explicit naming",
			config: BackendEndpointConfig{
				ID:      "tensorrt",
				URI:     "http://tensorrt-server.system.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'tensorrt'",
		},

		// vLLM and other OpenAI-compatible backends - should use /health
		{
			name: "vLLM explicit",
			config: BackendEndpointConfig{
				ID:      "llama-vllm",
				URI:     "http://llama-3-1-8b-instruct-vllm-predictor.development.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
		{
			name: "Generic model service",
			config: BackendEndpointConfig{
				ID:      "gpt-oss-20b",
				URI:     "http://unsloth-gpt-oss-20b-predictor.development.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
		{
			name: "Vision model (Qwen)",
			config: BackendEndpointConfig{
				ID:      "qwen2-vl",
				URI:     "http://qwen2-vl-7b-instruct-predictor.development.svc.cluster.local:80",
				Timeout: 5 * time.Second,
			},
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
		{
			name: "External OpenAI-compatible service",
			config: BackendEndpointConfig{
				ID:      "openai",
				URI:     "https://api.openai.com/v1",
				Timeout: 30 * time.Second,
			},
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetHealthPath()
			if got != tt.expectedPath {
				t.Errorf("BackendEndpointConfig.GetHealthPath() = %q, want %q (reason: %s)",
					got, tt.expectedPath, tt.expectedReason)
			}
		})
	}
}
