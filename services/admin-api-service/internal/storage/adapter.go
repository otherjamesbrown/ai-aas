// Package storage provides object storage operations for model files.
package storage

import (
	"context"
)

// Ensure S3Client implements the interface expected by the models service
var _ interface {
	MovePrefix(ctx context.Context, oldPrefix, newPrefix string) (totalSize int64, fileCount int, err error)
} = (*S3Client)(nil)
