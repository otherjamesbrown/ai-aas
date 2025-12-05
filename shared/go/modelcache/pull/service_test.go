// Package pull provides model pull orchestration for downloading from HuggingFace
// and uploading to object storage.
package pull

import (
	"testing"
	"time"
)

func TestPullOptionsDefaults(t *testing.T) {
	opts := PullOptions{
		ModelName: "test-model",
		HFModelID: "org/model",
		S3Prefix:  "models/test",
	}

	// Revision should default to empty (will be set to "main" in Pull)
	if opts.Revision != "" {
		t.Errorf("expected empty Revision, got %s", opts.Revision)
	}
}

func TestPullResultFields(t *testing.T) {
	result := PullResult{
		ModelName:      "test-model",
		HFModelID:      "org/model",
		Revision:       "main",
		S3Path:         "s3://bucket/models/test",
		TotalSize:      1024 * 1024 * 100, // 100MB
		FileCount:      5,
		ManifestDigest: "sha256:abc123",
	}

	if result.FileCount != 5 {
		t.Errorf("expected 5 files, got %d", result.FileCount)
	}

	if result.TotalSize != 104857600 {
		t.Errorf("expected 104857600 bytes, got %d", result.TotalSize)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		S3Bucket: "test-bucket",
	}

	// Concurrency defaults to 0, will be set to 3 in NewService
	if cfg.Concurrency != 0 {
		t.Errorf("expected 0 concurrency default, got %d", cfg.Concurrency)
	}
}

func TestConfig(t *testing.T) {
	cfg := Config{
		HFToken:          "hf_token",
		S3Endpoint:       "https://s3.example.com",
		S3AccessKey:      "access-key",
		S3SecretKey:      "secret-key",
		S3Bucket:         "models",
		S3Region:         "us-east-1",
		S3ForcePathStyle: true,
		Concurrency:      5,
	}

	if cfg.HFToken != "hf_token" {
		t.Errorf("expected HFToken hf_token, got %s", cfg.HFToken)
	}
	if cfg.S3Endpoint != "https://s3.example.com" {
		t.Errorf("expected S3Endpoint, got %s", cfg.S3Endpoint)
	}
	if cfg.S3AccessKey != "access-key" {
		t.Errorf("expected S3AccessKey, got %s", cfg.S3AccessKey)
	}
	if cfg.S3Bucket != "models" {
		t.Errorf("expected S3Bucket models, got %s", cfg.S3Bucket)
	}
	if cfg.Concurrency != 5 {
		t.Errorf("expected Concurrency 5, got %d", cfg.Concurrency)
	}
}

func TestPullOptions(t *testing.T) {
	var progressCalled, fileStartCalled, fileCompleteCalled, cancelledCalled bool

	opts := PullOptions{
		ModelName:  "llama-7b",
		HFModelID:  "meta-llama/Llama-2-7b-hf",
		Revision:   "main",
		S3Prefix:   "models/llama-7b/main",
		OnProgress: func(phase string, bytesCompleted, bytesTotal int64) {
			progressCalled = true
		},
		OnFileStart: func(filename string) {
			fileStartCalled = true
		},
		OnFileComplete: func(filename string, size int64) {
			fileCompleteCalled = true
		},
		CheckCancelled: func() bool {
			cancelledCalled = true
			return false
		},
	}

	if opts.ModelName != "llama-7b" {
		t.Errorf("expected ModelName llama-7b, got %s", opts.ModelName)
	}
	if opts.HFModelID != "meta-llama/Llama-2-7b-hf" {
		t.Errorf("expected HFModelID meta-llama/Llama-2-7b-hf, got %s", opts.HFModelID)
	}

	// Test callbacks
	if opts.OnProgress != nil {
		opts.OnProgress("downloading", 100, 1000)
	}
	if !progressCalled {
		t.Error("expected OnProgress callback to be called")
	}

	if opts.OnFileStart != nil {
		opts.OnFileStart("model.bin")
	}
	if !fileStartCalled {
		t.Error("expected OnFileStart callback to be called")
	}

	if opts.OnFileComplete != nil {
		opts.OnFileComplete("model.bin", 1024)
	}
	if !fileCompleteCalled {
		t.Error("expected OnFileComplete callback to be called")
	}

	if opts.CheckCancelled != nil {
		opts.CheckCancelled()
	}
	if !cancelledCalled {
		t.Error("expected CheckCancelled callback to be called")
	}
}

func TestPullResult(t *testing.T) {
	result := PullResult{
		ModelName:      "llama-7b",
		HFModelID:      "meta-llama/Llama-2-7b-hf",
		Revision:       "main",
		S3Path:         "s3://models/llama-7b/main",
		TotalSize:      1024 * 1024 * 1024 * 7, // 7GB
		FileCount:      42,
		Duration:       5 * time.Minute,
		ManifestPath:   "models/llama-7b/main/.manifest.json",
		ManifestDigest: "sha256:abc123",
	}

	if result.ModelName != "llama-7b" {
		t.Errorf("expected ModelName llama-7b, got %s", result.ModelName)
	}
	if result.Duration != 5*time.Minute {
		t.Errorf("expected Duration 5m, got %v", result.Duration)
	}
	if result.ManifestPath != "models/llama-7b/main/.manifest.json" {
		t.Errorf("expected ManifestPath, got %s", result.ManifestPath)
	}
}

func TestPullOptions_CancellationCheck(t *testing.T) {
	cancelled := false

	opts := PullOptions{
		CheckCancelled: func() bool {
			return cancelled
		},
	}

	// Initially not cancelled
	if opts.CheckCancelled() {
		t.Error("expected not cancelled initially")
	}

	// Set cancelled
	cancelled = true
	if !opts.CheckCancelled() {
		t.Error("expected cancelled after setting flag")
	}
}

func TestPullOptions_NilCallbacks(t *testing.T) {
	opts := PullOptions{
		ModelName: "test",
		HFModelID: "org/model",
	}

	// Nil callbacks should not panic when checked
	if opts.OnProgress != nil {
		t.Error("expected nil OnProgress")
	}
	if opts.OnFileStart != nil {
		t.Error("expected nil OnFileStart")
	}
	if opts.OnFileComplete != nil {
		t.Error("expected nil OnFileComplete")
	}
	if opts.CheckCancelled != nil {
		t.Error("expected nil CheckCancelled")
	}
}
