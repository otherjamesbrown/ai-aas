// Package huggingface provides a client for the HuggingFace Hub API.
package huggingface

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DownloadOptions configures model download behavior
type DownloadOptions struct {
	Revision     string
	DestDir      string
	Resume       bool
	ShowProgress bool
	Concurrency  int
	OnProgress   func(downloaded, total int64)
}

// DefaultDownloadOptions returns default download options
func DefaultDownloadOptions(destDir string) DownloadOptions {
	return DownloadOptions{
		Revision:     "main",
		DestDir:      destDir,
		Resume:       true,
		ShowProgress: true,
		Concurrency:  3,
	}
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	ModelID      string
	Revision     string
	DestDir      string
	Files        []DownloadedFile
	TotalSize    int64
	Duration     time.Duration
	ResumedFiles int
}

// DownloadedFile represents a downloaded file
type DownloadedFile struct {
	Path     string
	Size     int64
	Checksum string
	Resumed  bool
}

// DownloadModel downloads a model from HuggingFace to a local directory
func (c *Client) DownloadModel(ctx context.Context, modelID string, opts DownloadOptions) (*DownloadResult, error) {
	start := time.Now()

	if opts.Revision == "" {
		opts.Revision = "main"
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 3
	}

	// Get list of files
	files, err := c.ListFiles(ctx, modelID, opts.Revision)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	// Filter to only files (not directories)
	var downloadFiles []RepoFile
	var totalSize int64
	for _, f := range files {
		if f.Type == "file" {
			downloadFiles = append(downloadFiles, f)
			totalSize += f.Size
		}
	}

	// Create destination directory
	if err := os.MkdirAll(opts.DestDir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	result := &DownloadResult{
		ModelID:   modelID,
		Revision:  opts.Revision,
		DestDir:   opts.DestDir,
		TotalSize: totalSize,
	}

	// Download files with concurrency limit
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadErr error
	var downloadedBytes int64

	for _, f := range downloadFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(file RepoFile) {
			defer wg.Done()
			defer func() { <-sem }()

			downloaded, err := c.downloadFile(ctx, modelID, opts.Revision, file, opts)
			
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if downloadErr == nil {
					downloadErr = fmt.Errorf("download %s: %w", file.Path, err)
				}
				return
			}

			result.Files = append(result.Files, *downloaded)
			if downloaded.Resumed {
				result.ResumedFiles++
			}

			downloadedBytes += downloaded.Size
			if opts.OnProgress != nil {
				opts.OnProgress(downloadedBytes, totalSize)
			}
		}(f)
	}

	wg.Wait()

	if downloadErr != nil {
		return nil, downloadErr
	}

	result.Duration = time.Since(start)
	return result, nil
}

func (c *Client) downloadFile(ctx context.Context, modelID, revision string, file RepoFile, opts DownloadOptions) (*DownloadedFile, error) {
	destPath := filepath.Join(opts.DestDir, file.Path)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Check for existing file (resume support)
	var existingSize int64
	if opts.Resume {
		if info, err := os.Stat(destPath); err == nil {
			existingSize = info.Size()
			if existingSize == file.Size {
				// File already complete, verify checksum
				checksum, err := computeFileChecksum(destPath)
				if err == nil {
					return &DownloadedFile{
						Path:     file.Path,
						Size:     file.Size,
						Checksum: checksum,
						Resumed:  true,
					}, nil
				}
			}
		}
	}

	// Build download URL
	// Note: Downloads use huggingface.co directly, not the /api endpoint
	url := fmt.Sprintf("https://huggingface.co/%s/resolve/%s/%s", modelID, revision, file.Path)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Resume support
	if existingSize > 0 && existingSize < file.Size {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	// Use a dedicated download client with no timeout (large files need time)
	// and explicit redirect following
	downloadClient := &http.Client{
		Timeout: 0, // No timeout for downloads
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Follow up to 10 redirects, preserving auth header
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Preserve authorization header on redirects to same host
			if len(via) > 0 && req.URL.Host == via[0].URL.Host {
				if auth := via[0].Header.Get("Authorization"); auth != "" {
					req.Header.Set("Authorization", auth)
				}
			}
			return nil
		},
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("download failed with status %d for URL: %s", resp.StatusCode, url)
	}

	// Open file for writing
	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		existingSize = 0
	}

	f, err := os.OpenFile(destPath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Copy data
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	// Verify size
	totalWritten := existingSize + written
	if totalWritten != file.Size {
		return nil, fmt.Errorf("size mismatch: expected %d, got %d", file.Size, totalWritten)
	}

	// Compute checksum
	checksum, err := computeFileChecksum(destPath)
	if err != nil {
		return nil, fmt.Errorf("compute checksum: %w", err)
	}

	return &DownloadedFile{
		Path:     file.Path,
		Size:     file.Size,
		Checksum: checksum,
		Resumed:  existingSize > 0,
	}, nil
}

// computeFileChecksum computes SHA256 checksum of a file
func computeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetModelSize returns the total size of a model
func (c *Client) GetModelSize(ctx context.Context, modelID, revision string) (int64, error) {
	if revision == "" {
		revision = "main"
	}

	files, err := c.ListFiles(ctx, modelID, revision)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, f := range files {
		if f.Type == "file" {
			total += f.Size
		}
	}

	return total, nil
}

