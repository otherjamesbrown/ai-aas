// Package cache provides model caching operations for the CLI.
// This package contains shared logic for downloading models from HuggingFace
// and uploading them to S3 object storage cache.
package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/huggingface"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/registry"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/storage"
)

// PullOptions configures model pull behavior.
type PullOptions struct {
	ModelName  string
	Revision   string
	DryRun     bool
	SkipVerify bool
}

// FormatBytes formats bytes to human-readable string.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// PullModelToCache downloads a model from HuggingFace and uploads it to S3 cache.
// This is the core logic used by both the pull command and version update command.
func PullModelToCache(ctx context.Context, opts PullOptions) error {
	// Get configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// S3 config still from viper (not in main config struct)
	s3Endpoint := viper.GetString("s3.endpoint")
	s3AccessKey := viper.GetString("s3.access_key")
	s3SecretKey := viper.GetString("s3.secret_key")
	s3Bucket := viper.GetString("s3.bucket")

	// Use Admin API endpoint for registry operations
	adminEndpoint := cfg.AdminAPIEndpoint
	if adminEndpoint == "" {
		adminEndpoint = cfg.APIEndpoint // fallback for backward compatibility
	}

	if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
		return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
	}

	// Get model from registry
	apiOpts := []api.ClientOption{}
	if cfg.TLSInsecure {
		apiOpts = append(apiOpts, api.WithInsecureSkipVerify())
	}
	apiClient := api.NewClient(adminEndpoint, cfg.APIKey, apiOpts...)
	regClient := registry.NewClient(apiClient)

	fmt.Printf("Looking up model: %s\n", opts.ModelName)
	model, err := regClient.Get(ctx, opts.ModelName)
	if err != nil {
		return fmt.Errorf("get model: %w", err)
	}

	// Determine revision
	revision := opts.Revision
	if revision == "" {
		revision = "main"
	}

	// Get model size and file list
	hfClient := huggingface.NewClient(huggingface.WithToken(cfg.HFToken))
	size, err := hfClient.GetModelSize(ctx, model.HFModelID, revision)
	if err != nil {
		return fmt.Errorf("get model size: %w", err)
	}

	fmt.Printf("Model: %s (%s)\n", model.Name, model.HFModelID)
	fmt.Printf("Revision: %s\n", revision)
	fmt.Printf("Total size: %s\n", FormatBytes(size))

	if opts.DryRun {
		fmt.Println("\n[DRY RUN] Would download and upload to:")
		fmt.Printf("  S3 path: s3://%s/models/%s/%s/\n", s3Bucket, opts.ModelName, revision)
		return nil
	}

	// Validate S3 credentials
	if s3Endpoint == "" || s3AccessKey == "" || s3SecretKey == "" || s3Bucket == "" {
		return fmt.Errorf("S3 credentials not configured. Run 'ai-aas-cli credentials set s3 ...' first")
	}

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "ai-aas-model-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download from HuggingFace
	fmt.Println("\nDownloading from HuggingFace...")
	progress := output.NewDownloadProgressBar("Downloading", size)

	downloadOpts := huggingface.DownloadOptions{
		Revision:     revision,
		DestDir:      tempDir,
		Resume:       true,
		ShowProgress: true,
		Concurrency:  3,
		OnProgress: func(downloaded, total int64) {
			_ = progress.Set64(downloaded)
		},
	}

	result, err := hfClient.DownloadModel(ctx, model.HFModelID, downloadOpts)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	progress.Finish()

	fmt.Printf("Downloaded %d files (%s) in %s\n",
		len(result.Files),
		FormatBytes(result.TotalSize),
		result.Duration.Round(time.Second))

	if result.ResumedFiles > 0 {
		fmt.Printf("  (resumed %d files from previous download)\n", result.ResumedFiles)
	}

	// Generate manifest
	fmt.Println("\nGenerating manifest...")
	manifest, err := storage.GenerateManifest(ctx, model.HFModelID, revision, revision, tempDir)
	if err != nil {
		return fmt.Errorf("generate manifest: %w", err)
	}

	manifestPath := filepath.Join(tempDir, storage.ManifestFileName)
	if err := storage.SaveManifest(manifest, manifestPath); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	// Create S3 client
	s3Client, err := storage.NewS3Client(ctx, storage.S3Config{
		Endpoint:       s3Endpoint,
		AccessKey:      s3AccessKey,
		SecretKey:      s3SecretKey,
		Bucket:         s3Bucket,
		ForcePathStyle: true,
	})
	if err != nil {
		return fmt.Errorf("create S3 client: %w", err)
	}

	// Upload to S3
	fmt.Println("\nUploading to object storage...")
	s3Prefix := fmt.Sprintf("models/%s/%s", opts.ModelName, revision)

	var uploaded int64
	uploadProgress := output.NewDownloadProgressBar("Uploading", result.TotalSize)

	for _, file := range result.Files {
		localPath := filepath.Join(tempDir, file.Path)
		remotePath := fmt.Sprintf("%s/%s", s3Prefix, file.Path)

		// Use multipart for large files (>100MB)
		if file.Size > 100*1024*1024 {
			err = s3Client.UploadMultipart(ctx, localPath, remotePath, 50*1024*1024, func(partUploaded, total int64) {
				_ = uploadProgress.Set64(uploaded + partUploaded)
			})
		} else {
			err = s3Client.Upload(ctx, localPath, remotePath)
		}

		if err != nil {
			return fmt.Errorf("upload %s: %w", file.Path, err)
		}

		uploaded += file.Size
		_ = uploadProgress.Set64(uploaded)
	}

	// Upload manifest
	remoteManifestPath := fmt.Sprintf("%s/%s", s3Prefix, storage.ManifestFileName)
	if err := s3Client.Upload(ctx, manifestPath, remoteManifestPath); err != nil {
		return fmt.Errorf("upload manifest: %w", err)
	}

	uploadProgress.Finish()

	// Verify upload if requested
	if !opts.SkipVerify {
		fmt.Println("\nVerifying upload...")
		exists, err := s3Client.Exists(ctx, remoteManifestPath)
		if err != nil {
			return fmt.Errorf("verify manifest: %w", err)
		}
		if !exists {
			return fmt.Errorf("manifest not found in S3 after upload")
		}
		fmt.Println("Verification passed!")
	}

	fmt.Printf("\nModel cached successfully!\n")
	fmt.Printf("   S3 path: s3://%s/%s/\n", s3Bucket, s3Prefix)
	fmt.Printf("   Files: %d\n", manifest.FileCount)
	fmt.Printf("   Size: %s\n", FormatBytes(manifest.TotalSize))

	return nil
}
