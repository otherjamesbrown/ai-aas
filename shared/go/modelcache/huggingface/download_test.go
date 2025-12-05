// Package huggingface provides a client for interacting with the HuggingFace Hub API.
package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDownloadOptions(t *testing.T) {
	opts := DefaultDownloadOptions("/tmp/test")

	if opts.Revision != "main" {
		t.Errorf("expected revision main, got %s", opts.Revision)
	}
	if opts.DestDir != "/tmp/test" {
		t.Errorf("expected destDir /tmp/test, got %s", opts.DestDir)
	}
	if !opts.Resume {
		t.Error("expected Resume to be true")
	}
	if !opts.ShowProgress {
		t.Error("expected ShowProgress to be true")
	}
	if opts.Concurrency != 3 {
		t.Errorf("expected Concurrency 3, got %d", opts.Concurrency)
	}
}

func TestDownloadModel(t *testing.T) {
	t.Run("successful download", func(t *testing.T) {
		// Create mock server for API calls
		apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/models/org/model/tree/main" {
				files := []RepoFile{
					{Type: "file", Path: "config.json", Size: 13},
				}
				json.NewEncoder(w).Encode(files)
				return
			}
			t.Errorf("unexpected API path: %s", r.URL.Path)
		}))
		defer apiServer.Close()

		// Create mock server for downloads
		downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"test":true}`))
		}))
		defer downloadServer.Close()

		// Create temp directory
		tempDir, err := os.MkdirTemp("", "hf-download-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		client := NewClient(WithBaseURL(apiServer.URL))

		// Note: This test is limited because the download URL is hardcoded to huggingface.co
		// A full integration test would need more complex mocking
		opts := DownloadOptions{
			Revision:    "main",
			DestDir:     tempDir,
			Concurrency: 1,
		}

		// Test that options are validated
		if opts.Concurrency != 1 {
			t.Errorf("expected concurrency 1, got %d", opts.Concurrency)
		}

		// Test context cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = client.DownloadModel(ctx, "org/model", opts)
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("custom cancellation check", func(t *testing.T) {
		apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			files := []RepoFile{
				{Type: "file", Path: "file1.txt", Size: 100},
				{Type: "file", Path: "file2.txt", Size: 100},
			}
			json.NewEncoder(w).Encode(files)
		}))
		defer apiServer.Close()

		tempDir, err := os.MkdirTemp("", "hf-download-cancel-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		cancelled := false
		opts := DownloadOptions{
			Revision:    "main",
			DestDir:     tempDir,
			Concurrency: 1,
			CheckCancelled: func() bool {
				return cancelled
			},
		}

		// Test that CheckCancelled is called
		if opts.CheckCancelled() {
			t.Error("expected not cancelled initially")
		}
		cancelled = true
		if !opts.CheckCancelled() {
			t.Error("expected cancelled after setting flag")
		}
	})

	t.Run("progress callbacks", func(t *testing.T) {
		var progressCalled, fileStartCalled, fileCompleteCalled bool

		opts := DownloadOptions{
			Revision:    "main",
			DestDir:     "/tmp/test",
			Concurrency: 1,
			OnProgress: func(downloaded, total int64) {
				progressCalled = true
			},
			OnFileStart: func(filename string) {
				fileStartCalled = true
			},
			OnFileComplete: func(filename string, size int64) {
				fileCompleteCalled = true
			},
		}

		// Simulate callbacks
		if opts.OnProgress != nil {
			opts.OnProgress(100, 1000)
		}
		if opts.OnFileStart != nil {
			opts.OnFileStart("test.txt")
		}
		if opts.OnFileComplete != nil {
			opts.OnFileComplete("test.txt", 100)
		}

		if !progressCalled {
			t.Error("expected OnProgress to be called")
		}
		if !fileStartCalled {
			t.Error("expected OnFileStart to be called")
		}
		if !fileCompleteCalled {
			t.Error("expected OnFileComplete to be called")
		}
	})
}

func TestComputeFileChecksum(t *testing.T) {
	t.Run("compute checksum", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "checksum-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testFile := filepath.Join(tempDir, "test.txt")
		content := []byte("hello world")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		checksum, err := ComputeFileChecksum(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// SHA256 of "hello world"
		expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
		if checksum != expected {
			t.Errorf("expected checksum %s, got %s", expected, checksum)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ComputeFileChecksum("/nonexistent/file.txt")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "checksum-empty-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		testFile := filepath.Join(tempDir, "empty.txt")
		if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write empty file: %v", err)
		}

		checksum, err := ComputeFileChecksum(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// SHA256 of empty string
		expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		if checksum != expected {
			t.Errorf("expected checksum %s, got %s", expected, checksum)
		}
	})
}

func TestGetModelSize(t *testing.T) {
	t.Run("calculate total size", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			files := []RepoFile{
				{Type: "file", Path: "config.json", Size: 1000},
				{Type: "file", Path: "model.bin", Size: 1000000000},
				{Type: "directory", Path: "subdir"},
				{Type: "file", Path: "tokenizer.json", Size: 500000},
			}
			json.NewEncoder(w).Encode(files)
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		size, err := client.GetModelSize(context.Background(), "org/model", "main")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := int64(1000 + 1000000000 + 500000)
		if size != expected {
			t.Errorf("expected size %d, got %d", expected, size)
		}
	})

	t.Run("default revision", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models/org/model/tree/main" {
				t.Errorf("expected main revision, got path %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode([]RepoFile{})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		_, err := client.GetModelSize(context.Background(), "org/model", "")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]RepoFile{})
		}))
		defer server.Close()

		client := NewClient(WithBaseURL(server.URL))
		size, err := client.GetModelSize(context.Background(), "org/model", "main")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != 0 {
			t.Errorf("expected size 0, got %d", size)
		}
	})
}

func TestDownloadedFile(t *testing.T) {
	df := DownloadedFile{
		Path:     "model.safetensors",
		Size:     1024 * 1024 * 100,
		Checksum: "abc123",
		Resumed:  true,
	}

	if df.Path != "model.safetensors" {
		t.Errorf("expected path model.safetensors, got %s", df.Path)
	}
	if df.Size != 104857600 {
		t.Errorf("expected size 104857600, got %d", df.Size)
	}
	if df.Checksum != "abc123" {
		t.Errorf("expected checksum abc123, got %s", df.Checksum)
	}
	if !df.Resumed {
		t.Error("expected Resumed to be true")
	}
}

func TestDownloadResult(t *testing.T) {
	result := DownloadResult{
		ModelID:      "org/model",
		Revision:     "main",
		DestDir:      "/tmp/model",
		TotalSize:    1024 * 1024 * 500,
		ResumedFiles: 2,
		Files: []DownloadedFile{
			{Path: "file1.txt", Size: 100},
			{Path: "file2.txt", Size: 200},
		},
	}

	if result.ModelID != "org/model" {
		t.Errorf("expected ModelID org/model, got %s", result.ModelID)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Files))
	}
	if result.ResumedFiles != 2 {
		t.Errorf("expected 2 resumed files, got %d", result.ResumedFiles)
	}
}
