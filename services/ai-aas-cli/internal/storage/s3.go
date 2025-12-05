// Package storage re-exports the shared storage client for backward compatibility.
// New code should import github.com/ai-aas/shared-go/modelcache/storage directly.
package storage

import (
	"context"

	sharedStorage "github.com/ai-aas/shared-go/modelcache/storage"
)

// Re-export types
type (
	S3Client   = sharedStorage.S3Client
	S3Config   = sharedStorage.S3Config
	ObjectInfo = sharedStorage.ObjectInfo
)

// Re-export constructor
var NewS3Client = sharedStorage.NewS3Client

// Ensure interface compatibility at compile time
var _ interface {
	Upload(ctx context.Context, localPath, remotePath string) error
	UploadMultipart(ctx context.Context, localPath, remotePath string, partSize int64, onProgress func(uploaded, total int64)) error
	Download(ctx context.Context, remotePath, localPath string) error
	Delete(ctx context.Context, remotePath string) error
	ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error)
	Exists(ctx context.Context, remotePath string) (bool, error)
	GetObjectSize(ctx context.Context, remotePath string) (int64, error)
} = (*S3Client)(nil)
