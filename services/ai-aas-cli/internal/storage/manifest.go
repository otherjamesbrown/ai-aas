// Package storage re-exports the shared manifest functionality for backward compatibility.
// New code should import github.com/ai-aas/shared-go/modelcache/storage directly.
package storage

import (
	"context"

	sharedStorage "github.com/ai-aas/shared-go/modelcache/storage"
)

// Re-export manifest types
type (
	Manifest     = sharedStorage.Manifest
	ManifestFile = sharedStorage.ManifestFile
	VerifyResult = sharedStorage.VerifyResult
)

// Re-export constants
const ManifestFileName = sharedStorage.ManifestFileName

// Re-export functions
var (
	GenerateManifest = sharedStorage.GenerateManifest
	SaveManifest     = sharedStorage.SaveManifest
	LoadManifest     = sharedStorage.LoadManifest
	VerifyManifest   = sharedStorage.VerifyManifest
	QuickVerify      = sharedStorage.QuickVerify
)

// Ensure function signatures match at compile time
var _ func(ctx context.Context, modelID, version, hfRevision, dir string) (*Manifest, error) = GenerateManifest
var _ func(manifest *Manifest, path string) error = SaveManifest
var _ func(path string) (*Manifest, error) = LoadManifest
