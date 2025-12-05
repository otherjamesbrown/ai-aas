// Package pull provides model pull orchestration for downloading from HuggingFace
// and uploading to object storage.
package pull

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ai-aas/shared-go/modelcache/huggingface"
	"github.com/ai-aas/shared-go/modelcache/storage"
)

// Config contains configuration for the pull service
type Config struct {
	// HuggingFace configuration
	HFToken string

	// S3 configuration
	S3Endpoint       string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string
	S3ForcePathStyle bool

	// Download options
	Concurrency int // Number of concurrent downloads (default: 3)
}

// Service orchestrates model downloads from HuggingFace to S3
type Service struct {
	cfg      Config
	hfClient *huggingface.Client
	s3Client *storage.S3Client
}

// NewService creates a new pull service
func NewService(ctx context.Context, cfg Config) (*Service, error) {
	// Create HuggingFace client
	hfOpts := []huggingface.ClientOption{}
	if cfg.HFToken != "" {
		hfOpts = append(hfOpts, huggingface.WithToken(cfg.HFToken))
	}
	hfClient := huggingface.NewClient(hfOpts...)

	// Create S3 client
	s3Client, err := storage.NewS3Client(ctx, storage.S3Config{
		Endpoint:       cfg.S3Endpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		Bucket:         cfg.S3Bucket,
		Region:         cfg.S3Region,
		ForcePathStyle: cfg.S3ForcePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}

	return &Service{
		cfg:      cfg,
		hfClient: hfClient,
		s3Client: s3Client,
	}, nil
}

// PullOptions configures a pull operation
type PullOptions struct {
	ModelName  string // Name in our registry
	HFModelID  string // HuggingFace model ID (e.g., "meta-llama/Llama-2-7b-hf")
	Revision   string // HuggingFace revision (default: "main")
	S3Prefix   string // S3 path prefix (e.g., "models/llama-7b/main")

	// Progress callbacks
	OnProgress     func(phase string, bytesCompleted, bytesTotal int64)
	OnFileStart    func(filename string)
	OnFileComplete func(filename string, size int64)

	// Cancellation check - called periodically to check if operation should stop
	CheckCancelled func() bool
}

// PullResult contains the result of a pull operation
type PullResult struct {
	ModelName      string
	HFModelID      string
	Revision       string
	S3Path         string
	TotalSize      int64
	FileCount      int
	Duration       time.Duration
	ManifestPath   string
	ManifestDigest string
}

// Pull downloads a model from HuggingFace and uploads it to S3
func (s *Service) Pull(ctx context.Context, opts PullOptions) (*PullResult, error) {
	start := time.Now()

	if opts.Revision == "" {
		opts.Revision = "main"
	}

	concurrency := s.cfg.Concurrency
	if concurrency < 1 {
		concurrency = 3
	}

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "modelcache-pull-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Get model size first
	totalSize, err := s.hfClient.GetModelSize(ctx, opts.HFModelID, opts.Revision)
	if err != nil {
		return nil, fmt.Errorf("get model size: %w", err)
	}

	// Phase 1: Download from HuggingFace
	if opts.OnProgress != nil {
		opts.OnProgress("downloading", 0, totalSize)
	}

	downloadOpts := huggingface.DownloadOptions{
		Revision:    opts.Revision,
		DestDir:     tempDir,
		Resume:      true,
		Concurrency: concurrency,
		OnProgress: func(downloaded, total int64) {
			if opts.OnProgress != nil {
				opts.OnProgress("downloading", downloaded, total)
			}
		},
		OnFileStart:    opts.OnFileStart,
		OnFileComplete: opts.OnFileComplete,
		CheckCancelled: opts.CheckCancelled,
	}

	downloadResult, err := s.hfClient.DownloadModel(ctx, opts.HFModelID, downloadOpts)
	if err != nil {
		return nil, fmt.Errorf("download model: %w", err)
	}

	// Phase 2: Generate manifest
	if opts.OnProgress != nil {
		opts.OnProgress("generating_manifest", 0, 0)
	}

	manifest, err := storage.GenerateManifest(ctx, opts.HFModelID, opts.Revision, opts.Revision, tempDir)
	if err != nil {
		return nil, fmt.Errorf("generate manifest: %w", err)
	}

	manifestPath := filepath.Join(tempDir, storage.ManifestFileName)
	if err := storage.SaveManifest(manifest, manifestPath); err != nil {
		return nil, fmt.Errorf("save manifest: %w", err)
	}

	// Phase 3: Upload to S3
	if opts.OnProgress != nil {
		opts.OnProgress("uploading", 0, downloadResult.TotalSize)
	}

	var uploaded int64
	for _, file := range downloadResult.Files {
		// Check for cancellation
		if opts.CheckCancelled != nil && opts.CheckCancelled() {
			return nil, context.Canceled
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		localPath := filepath.Join(tempDir, file.Path)
		remotePath := fmt.Sprintf("%s/%s", opts.S3Prefix, file.Path)

		// Use multipart for large files (>100MB)
		if file.Size > 100*1024*1024 {
			err = s.s3Client.UploadMultipart(ctx, localPath, remotePath, 50*1024*1024, func(partUploaded, total int64) {
				if opts.OnProgress != nil {
					opts.OnProgress("uploading", uploaded+partUploaded, downloadResult.TotalSize)
				}
			})
		} else {
			err = s.s3Client.Upload(ctx, localPath, remotePath)
		}

		if err != nil {
			return nil, fmt.Errorf("upload %s: %w", file.Path, err)
		}

		uploaded += file.Size
		if opts.OnProgress != nil {
			opts.OnProgress("uploading", uploaded, downloadResult.TotalSize)
		}
	}

	// Upload manifest
	remoteManifestPath := fmt.Sprintf("%s/%s", opts.S3Prefix, storage.ManifestFileName)
	if err := s.s3Client.Upload(ctx, manifestPath, remoteManifestPath); err != nil {
		return nil, fmt.Errorf("upload manifest: %w", err)
	}

	// Phase 4: Verify
	if opts.OnProgress != nil {
		opts.OnProgress("verifying", 0, 0)
	}

	exists, err := s.s3Client.Exists(ctx, remoteManifestPath)
	if err != nil {
		return nil, fmt.Errorf("verify manifest: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("manifest not found in S3 after upload")
	}

	return &PullResult{
		ModelName:      opts.ModelName,
		HFModelID:      opts.HFModelID,
		Revision:       opts.Revision,
		S3Path:         fmt.Sprintf("s3://%s/%s", s.s3Client.Bucket(), opts.S3Prefix),
		TotalSize:      manifest.TotalSize,
		FileCount:      manifest.FileCount,
		Duration:       time.Since(start),
		ManifestPath:   remoteManifestPath,
		ManifestDigest: manifest.Checksum,
	}, nil
}

// GetModelSize returns the size of a model on HuggingFace
func (s *Service) GetModelSize(ctx context.Context, hfModelID, revision string) (int64, error) {
	return s.hfClient.GetModelSize(ctx, hfModelID, revision)
}

// ValidateHFToken validates the HuggingFace token
func (s *Service) ValidateHFToken(ctx context.Context) (*huggingface.UserInfo, error) {
	return s.hfClient.ValidateToken(ctx)
}
