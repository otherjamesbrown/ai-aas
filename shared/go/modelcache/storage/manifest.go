// Package storage provides S3-compatible object storage operations.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Manifest represents a model cache manifest
type Manifest struct {
	ModelID    string         `json:"model_id"`
	Version    string         `json:"version"`
	HFRevision string         `json:"hf_revision"`
	CreatedAt  time.Time      `json:"created_at"`
	TotalSize  int64          `json:"total_size"`
	FileCount  int            `json:"file_count"`
	Files      []ManifestFile `json:"files"`
	Checksum   string         `json:"checksum"` // SHA256 of manifest content
}

// ManifestFile represents a file in the manifest
type ManifestFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"` // SHA256 of file content
}

// ManifestFileName is the name of the manifest file in the cache
const ManifestFileName = ".manifest.json"

// GenerateManifest creates a manifest for a directory of model files
func GenerateManifest(ctx context.Context, modelID, version, hfRevision, dir string) (*Manifest, error) {
	manifest := &Manifest{
		ModelID:    modelID,
		Version:    version,
		HFRevision: hfRevision,
		CreatedAt:  time.Now().UTC(),
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories and the manifest file itself
		if info.IsDir() || info.Name() == ManifestFileName {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		// Compute checksum
		checksum, err := computeFileChecksum(path)
		if err != nil {
			return fmt.Errorf("compute checksum for %s: %w", relPath, err)
		}

		manifest.Files = append(manifest.Files, ManifestFile{
			Path:     relPath,
			Size:     info.Size(),
			Checksum: checksum,
		})

		manifest.TotalSize += info.Size()
		manifest.FileCount++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	// Compute manifest checksum
	manifest.Checksum = computeManifestChecksum(manifest)

	return manifest, nil
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

// computeManifestChecksum computes checksum of manifest content (excluding checksum field)
func computeManifestChecksum(m *Manifest) string {
	// Create a copy without checksum
	copy := *m
	copy.Checksum = ""

	data, _ := json.Marshal(copy)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SaveManifest saves the manifest to a file
func SaveManifest(manifest *Manifest, path string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// LoadManifest loads a manifest from a file
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

// ManifestToJSON serializes a manifest to JSON bytes
func ManifestToJSON(manifest *Manifest) ([]byte, error) {
	return json.MarshalIndent(manifest, "", "  ")
}

// ManifestFromJSON deserializes a manifest from JSON bytes
func ManifestFromJSON(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return &manifest, nil
}

// VerifyResult contains the result of manifest verification
type VerifyResult struct {
	Valid         bool
	ManifestValid bool
	FilesChecked  int
	FilesMissing  []string
	FilesCorrupt  []string
	SizeMatches   bool
	ExpectedSize  int64
	ActualSize    int64
}

// VerifyManifest verifies all files against the manifest
func VerifyManifest(manifest *Manifest, dir string) (*VerifyResult, error) {
	result := &VerifyResult{
		Valid:         true,
		ManifestValid: true,
		ExpectedSize:  manifest.TotalSize,
	}

	// Verify manifest checksum
	expectedChecksum := manifest.Checksum
	actualChecksum := computeManifestChecksum(manifest)
	if expectedChecksum != actualChecksum {
		result.ManifestValid = false
		result.Valid = false
	}

	// Verify each file
	for _, file := range manifest.Files {
		result.FilesChecked++
		filePath := filepath.Join(dir, file.Path)

		// Check if file exists
		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			result.FilesMissing = append(result.FilesMissing, file.Path)
			result.Valid = false
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat file %s: %w", file.Path, err)
		}

		result.ActualSize += info.Size()

		// Check size
		if info.Size() != file.Size {
			result.FilesCorrupt = append(result.FilesCorrupt, file.Path)
			result.Valid = false
			continue
		}

		// Verify checksum
		checksum, err := computeFileChecksum(filePath)
		if err != nil {
			return nil, fmt.Errorf("compute checksum for %s: %w", file.Path, err)
		}

		if checksum != file.Checksum {
			result.FilesCorrupt = append(result.FilesCorrupt, file.Path)
			result.Valid = false
		}
	}

	result.SizeMatches = result.ActualSize == result.ExpectedSize

	return result, nil
}

// QuickVerify performs a quick verification (size only, no checksum)
func QuickVerify(manifest *Manifest, dir string) (*VerifyResult, error) {
	result := &VerifyResult{
		Valid:        true,
		ExpectedSize: manifest.TotalSize,
	}

	for _, file := range manifest.Files {
		result.FilesChecked++
		filePath := filepath.Join(dir, file.Path)

		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			result.FilesMissing = append(result.FilesMissing, file.Path)
			result.Valid = false
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat file %s: %w", file.Path, err)
		}

		result.ActualSize += info.Size()

		if info.Size() != file.Size {
			result.FilesCorrupt = append(result.FilesCorrupt, file.Path)
			result.Valid = false
		}
	}

	result.SizeMatches = result.ActualSize == result.ExpectedSize

	return result, nil
}
