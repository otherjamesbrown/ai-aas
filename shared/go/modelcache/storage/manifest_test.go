// Package storage provides S3-compatible object storage operations.
package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateManifest(t *testing.T) {
	t.Run("generate manifest from directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "manifest-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test files
		files := map[string][]byte{
			"config.json":          []byte(`{"test": true}`),
			"model.bin":            []byte("model data here"),
			"subdir/tokenizer.json": []byte(`{"vocab_size": 32000}`),
		}

		for path, content := range files {
			fullPath := filepath.Join(tempDir, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}
			if err := os.WriteFile(fullPath, content, 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
		}

		manifest, err := GenerateManifest(context.Background(), "org/model", "v1.0", "main", tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if manifest.ModelID != "org/model" {
			t.Errorf("expected ModelID org/model, got %s", manifest.ModelID)
		}
		if manifest.Version != "v1.0" {
			t.Errorf("expected Version v1.0, got %s", manifest.Version)
		}
		if manifest.HFRevision != "main" {
			t.Errorf("expected HFRevision main, got %s", manifest.HFRevision)
		}
		if manifest.FileCount != 3 {
			t.Errorf("expected 3 files, got %d", manifest.FileCount)
		}
		if manifest.Checksum == "" {
			t.Error("expected non-empty checksum")
		}

		// Check total size
		var expectedSize int64
		for _, content := range files {
			expectedSize += int64(len(content))
		}
		if manifest.TotalSize != expectedSize {
			t.Errorf("expected TotalSize %d, got %d", expectedSize, manifest.TotalSize)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "manifest-cancel-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create a file
		if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = GenerateManifest(ctx, "org/model", "v1.0", "main", tempDir)
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "manifest-empty-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		manifest, err := GenerateManifest(context.Background(), "org/model", "v1.0", "main", tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if manifest.FileCount != 0 {
			t.Errorf("expected 0 files, got %d", manifest.FileCount)
		}
		if manifest.TotalSize != 0 {
			t.Errorf("expected TotalSize 0, got %d", manifest.TotalSize)
		}
	})

	t.Run("skips manifest file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "manifest-skip-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create files including a manifest
		os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0644)
		os.WriteFile(filepath.Join(tempDir, ManifestFileName), []byte("{}"), 0644)

		manifest, err := GenerateManifest(context.Background(), "org/model", "v1.0", "main", tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should only include test.txt, not the manifest
		if manifest.FileCount != 1 {
			t.Errorf("expected 1 file, got %d", manifest.FileCount)
		}
	})
}

func TestSaveAndLoadManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "manifest-save-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	original := &Manifest{
		ModelID:    "org/model",
		Version:    "v1.0",
		HFRevision: "main",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
		TotalSize:  1024,
		FileCount:  2,
		Files: []ManifestFile{
			{Path: "file1.txt", Size: 512, Checksum: "abc123"},
			{Path: "file2.txt", Size: 512, Checksum: "def456"},
		},
		Checksum: "manifest-checksum",
	}

	manifestPath := filepath.Join(tempDir, ManifestFileName)

	// Save
	if err := SaveManifest(original, manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Load
	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	// Verify
	if loaded.ModelID != original.ModelID {
		t.Errorf("expected ModelID %s, got %s", original.ModelID, loaded.ModelID)
	}
	if loaded.Version != original.Version {
		t.Errorf("expected Version %s, got %s", original.Version, loaded.Version)
	}
	if loaded.FileCount != original.FileCount {
		t.Errorf("expected FileCount %d, got %d", original.FileCount, loaded.FileCount)
	}
	if len(loaded.Files) != len(original.Files) {
		t.Errorf("expected %d files, got %d", len(original.Files), len(loaded.Files))
	}
}

func TestLoadManifest_Errors(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, err := LoadManifest("/nonexistent/manifest.json")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "manifest-invalid-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		invalidPath := filepath.Join(tempDir, "invalid.json")
		os.WriteFile(invalidPath, []byte("not valid json"), 0644)

		_, err = LoadManifest(invalidPath)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestManifestToJSON(t *testing.T) {
	manifest := &Manifest{
		ModelID:   "org/model",
		Version:   "v1.0",
		FileCount: 1,
		Files: []ManifestFile{
			{Path: "test.txt", Size: 100, Checksum: "abc"},
		},
	}

	data, err := ManifestToJSON(manifest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON data")
	}
}

func TestManifestFromJSON(t *testing.T) {
	jsonData := []byte(`{
		"model_id": "org/model",
		"version": "v1.0",
		"file_count": 2,
		"total_size": 1024,
		"files": [
			{"path": "a.txt", "size": 512, "checksum": "abc"},
			{"path": "b.txt", "size": 512, "checksum": "def"}
		]
	}`)

	manifest, err := ManifestFromJSON(jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manifest.ModelID != "org/model" {
		t.Errorf("expected ModelID org/model, got %s", manifest.ModelID)
	}
	if manifest.FileCount != 2 {
		t.Errorf("expected FileCount 2, got %d", manifest.FileCount)
	}
}

func TestManifestFromJSON_Error(t *testing.T) {
	_, err := ManifestFromJSON([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestVerifyManifest(t *testing.T) {
	t.Run("valid manifest", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "verify-valid-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test file
		testFile := filepath.Join(tempDir, "test.txt")
		content := []byte("test content")
		os.WriteFile(testFile, content, 0644)

		// Generate manifest
		manifest, err := GenerateManifest(context.Background(), "org/model", "v1.0", "main", tempDir)
		if err != nil {
			t.Fatalf("failed to generate manifest: %v", err)
		}

		// Verify
		result, err := VerifyManifest(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Valid {
			t.Error("expected valid result")
		}
		if !result.ManifestValid {
			t.Error("expected manifest to be valid")
		}
		if len(result.FilesMissing) > 0 {
			t.Errorf("expected no missing files, got %v", result.FilesMissing)
		}
		if len(result.FilesCorrupt) > 0 {
			t.Errorf("expected no corrupt files, got %v", result.FilesCorrupt)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "verify-missing-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		manifest := &Manifest{
			Files: []ManifestFile{
				{Path: "missing.txt", Size: 100, Checksum: "abc"},
			},
		}
		manifest.Checksum = computeManifestChecksum(manifest)

		result, err := VerifyManifest(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid result")
		}
		if len(result.FilesMissing) != 1 {
			t.Errorf("expected 1 missing file, got %d", len(result.FilesMissing))
		}
	})

	t.Run("corrupt file - size mismatch", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "verify-corrupt-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create file with different size
		testFile := filepath.Join(tempDir, "test.txt")
		os.WriteFile(testFile, []byte("short"), 0644)

		manifest := &Manifest{
			Files: []ManifestFile{
				{Path: "test.txt", Size: 1000, Checksum: "abc"},
			},
		}
		manifest.Checksum = computeManifestChecksum(manifest)

		result, err := VerifyManifest(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid result")
		}
		if len(result.FilesCorrupt) != 1 {
			t.Errorf("expected 1 corrupt file, got %d", len(result.FilesCorrupt))
		}
	})

	t.Run("corrupt file - checksum mismatch", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "verify-checksum-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create file
		testFile := filepath.Join(tempDir, "test.txt")
		content := []byte("content")
		os.WriteFile(testFile, content, 0644)

		manifest := &Manifest{
			Files: []ManifestFile{
				{Path: "test.txt", Size: int64(len(content)), Checksum: "wrong-checksum"},
			},
		}
		manifest.Checksum = computeManifestChecksum(manifest)

		result, err := VerifyManifest(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid result")
		}
		if len(result.FilesCorrupt) != 1 {
			t.Errorf("expected 1 corrupt file, got %d", len(result.FilesCorrupt))
		}
	})

	t.Run("invalid manifest checksum", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "verify-manifest-checksum-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		manifest := &Manifest{
			ModelID:  "org/model",
			Checksum: "invalid-checksum",
		}

		result, err := VerifyManifest(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ManifestValid {
			t.Error("expected manifest to be invalid")
		}
		if result.Valid {
			t.Error("expected overall result to be invalid")
		}
	})
}

func TestQuickVerify(t *testing.T) {
	t.Run("valid files", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "quick-valid-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		content := []byte("test content")
		os.WriteFile(filepath.Join(tempDir, "test.txt"), content, 0644)

		manifest := &Manifest{
			TotalSize: int64(len(content)),
			Files: []ManifestFile{
				{Path: "test.txt", Size: int64(len(content))},
			},
		}

		result, err := QuickVerify(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Valid {
			t.Error("expected valid result")
		}
		if !result.SizeMatches {
			t.Error("expected size to match")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "quick-missing-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		manifest := &Manifest{
			Files: []ManifestFile{
				{Path: "missing.txt", Size: 100},
			},
		}

		result, err := QuickVerify(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid result")
		}
		if len(result.FilesMissing) != 1 {
			t.Errorf("expected 1 missing file, got %d", len(result.FilesMissing))
		}
	})

	t.Run("size mismatch", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "quick-size-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("short"), 0644)

		manifest := &Manifest{
			Files: []ManifestFile{
				{Path: "test.txt", Size: 1000},
			},
		}

		result, err := QuickVerify(manifest, tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid result")
		}
		if len(result.FilesCorrupt) != 1 {
			t.Errorf("expected 1 corrupt file, got %d", len(result.FilesCorrupt))
		}
	})
}

func TestManifestFileName(t *testing.T) {
	if ManifestFileName != ".manifest.json" {
		t.Errorf("expected ManifestFileName .manifest.json, got %s", ManifestFileName)
	}
}

func TestManifestFile(t *testing.T) {
	mf := ManifestFile{
		Path:     "models/layer.bin",
		Size:     1024 * 1024 * 100,
		Checksum: "sha256:abc123",
	}

	if mf.Path != "models/layer.bin" {
		t.Errorf("expected path models/layer.bin, got %s", mf.Path)
	}
	if mf.Size != 104857600 {
		t.Errorf("expected size 104857600, got %d", mf.Size)
	}
}

func TestVerifyResult(t *testing.T) {
	result := VerifyResult{
		Valid:         false,
		ManifestValid: true,
		FilesChecked:  10,
		FilesMissing:  []string{"a.txt", "b.txt"},
		FilesCorrupt:  []string{"c.txt"},
		SizeMatches:   false,
		ExpectedSize:  1000,
		ActualSize:    500,
	}

	if result.Valid {
		t.Error("expected invalid")
	}
	if len(result.FilesMissing) != 2 {
		t.Errorf("expected 2 missing files, got %d", len(result.FilesMissing))
	}
	if len(result.FilesCorrupt) != 1 {
		t.Errorf("expected 1 corrupt file, got %d", len(result.FilesCorrupt))
	}
}
