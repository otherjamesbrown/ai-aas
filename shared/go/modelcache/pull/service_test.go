// Package pull provides model pull orchestration for downloading from HuggingFace
// and uploading to object storage.
package pull

import (
	"testing"
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
