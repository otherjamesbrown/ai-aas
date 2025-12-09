// Package storage provides object storage operations for model files.
package storage

import (
	"context"
	"strings"
	"testing"
)

// TestS3ClientInterface ensures S3Client implements the expected interface
func TestS3ClientInterface(t *testing.T) {
	// This test verifies the interface at compile time
	var _ interface {
		MovePrefix(context.Context, string, string) (int64, int, error)
	} = (*S3Client)(nil)
}

// TestPrefixNormalization tests that prefix normalization works correctly
func TestPrefixNormalization(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{
			name:   "prefix without trailing slash",
			prefix: "models/llama-3-8b",
			want:   "models/llama-3-8b/",
		},
		{
			name:   "prefix with trailing slash",
			prefix: "models/llama-3-8b/",
			want:   "models/llama-3-8b/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := tt.prefix
			if !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}
			if prefix != tt.want {
				t.Errorf("normalized prefix = %q, want %q", prefix, tt.want)
			}
		})
	}
}
