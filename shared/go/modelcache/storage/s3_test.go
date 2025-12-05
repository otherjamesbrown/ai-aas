// Package storage provides S3-compatible object storage operations.
package storage

import (
	"testing"
)

func TestS3Config(t *testing.T) {
	cfg := S3Config{
		Endpoint:       "https://s3.example.com",
		AccessKey:      "access-key",
		SecretKey:      "secret-key",
		Bucket:         "test-bucket",
		Region:         "us-east-1",
		PathPrefix:     "models/",
		ForcePathStyle: true,
	}

	if cfg.Endpoint != "https://s3.example.com" {
		t.Errorf("expected endpoint https://s3.example.com, got %s", cfg.Endpoint)
	}
	if cfg.AccessKey != "access-key" {
		t.Errorf("expected access key, got %s", cfg.AccessKey)
	}
	if cfg.Bucket != "test-bucket" {
		t.Errorf("expected bucket test-bucket, got %s", cfg.Bucket)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", cfg.Region)
	}
	if cfg.PathPrefix != "models/" {
		t.Errorf("expected path prefix models/, got %s", cfg.PathPrefix)
	}
	if !cfg.ForcePathStyle {
		t.Error("expected ForcePathStyle to be true")
	}
}

func TestObjectInfo(t *testing.T) {
	info := ObjectInfo{
		Key:  "models/llama/model.bin",
		Size: 1024 * 1024 * 1024,
		ETag: "abc123",
	}

	if info.Key != "models/llama/model.bin" {
		t.Errorf("expected key models/llama/model.bin, got %s", info.Key)
	}
	if info.Size != 1073741824 {
		t.Errorf("expected size 1073741824, got %d", info.Size)
	}
	if info.ETag != "abc123" {
		t.Errorf("expected ETag abc123, got %s", info.ETag)
	}
}

func TestFullPath(t *testing.T) {
	tests := []struct {
		name       string
		pathPrefix string
		path       string
		want       string
	}{
		{
			name:       "no prefix",
			pathPrefix: "",
			path:       "model.bin",
			want:       "model.bin",
		},
		{
			name:       "with prefix no trailing slash",
			pathPrefix: "models",
			path:       "llama/model.bin",
			want:       "models/llama/model.bin",
		},
		{
			name:       "with prefix trailing slash",
			pathPrefix: "models/",
			path:       "llama/model.bin",
			want:       "models/llama/model.bin",
		},
		{
			name:       "path with leading slash",
			pathPrefix: "models",
			path:       "/llama/model.bin",
			want:       "models/llama/model.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &S3Client{
				pathPrefix: tt.pathPrefix,
			}
			got := c.fullPath(tt.path)
			if got != tt.want {
				t.Errorf("fullPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestS3Client_Bucket(t *testing.T) {
	c := &S3Client{
		bucket: "test-bucket",
	}

	if c.Bucket() != "test-bucket" {
		t.Errorf("expected bucket test-bucket, got %s", c.Bucket())
	}
}

func TestS3Client_Endpoint(t *testing.T) {
	c := &S3Client{
		endpoint: "https://s3.example.com",
	}

	if c.Endpoint() != "https://s3.example.com" {
		t.Errorf("expected endpoint https://s3.example.com, got %s", c.Endpoint())
	}
}
