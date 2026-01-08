package main

import (
	"testing"
)

// TestGetHealthPathForBackend verifies that the correct health check path
// is detected based on backend URI patterns.
//
// Background:
// - vLLM backends use /health (OpenAI-compatible)
// - Triton/TensorRT-LLM backends use /v2/health/ready (KServe v2 protocol)
//
// Related bead: aas-gadbx
func TestGetHealthPathForBackend(t *testing.T) {
	tests := []struct {
		name           string
		uri            string
		expectedPath   string
		expectedReason string
	}{
		// Triton/TensorRT-LLM patterns - should use /v2/health/ready
		{
			name:           "TensorRT-LLM explicit",
			uri:            "http://llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80",
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'trtllm'",
		},
		{
			name:           "TensorRT-LLM with TRT shorthand",
			uri:            "http://model-trt-llm-predictor.system.svc.cluster.local:80",
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains '-trt-'",
		},
		{
			name:           "Multi-model Triton deployment (bug case)",
			uri:            "http://multi-model-3x8b-blackwell-predictor.system.svc.cluster.local:80",
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'multi-model'",
		},
		{
			name:           "Triton explicit naming",
			uri:            "http://triton-inference-server.system.svc.cluster.local:8000",
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'triton'",
		},
		{
			name:           "TensorRT explicit naming",
			uri:            "http://tensorrt-server.system.svc.cluster.local:80",
			expectedPath:   "/v2/health/ready",
			expectedReason: "contains 'tensorrt'",
		},
		// vLLM and other OpenAI-compatible backends - should use /health
		{
			name:           "vLLM explicit",
			uri:            "http://llama-3-1-8b-instruct-vllm-predictor.development.svc.cluster.local:80",
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
		{
			name:           "Generic model service",
			uri:            "http://unsloth-gpt-oss-20b-predictor.development.svc.cluster.local:80",
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
		{
			name:           "Vision model (Qwen)",
			uri:            "http://qwen2-vl-7b-instruct-predictor.development.svc.cluster.local:80",
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
		{
			name:           "External OpenAI-compatible service",
			uri:            "https://api.openai.com/v1",
			expectedPath:   "/health",
			expectedReason: "no Triton patterns detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getHealthPathForBackend(tt.uri)
			if got != tt.expectedPath {
				t.Errorf("getHealthPathForBackend(%q) = %q, want %q (reason: %s)",
					tt.uri, got, tt.expectedPath, tt.expectedReason)
			}
		})
	}
}
