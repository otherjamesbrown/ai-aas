// Package huggingface re-exports the shared huggingface client for backward compatibility.
// New code should import github.com/ai-aas/shared-go/modelcache/huggingface directly.
package huggingface

import (
	"context"
	"time"

	sharedHF "github.com/ai-aas/shared-go/modelcache/huggingface"
)

// Re-export constants
const (
	DefaultBaseURL = sharedHF.DefaultBaseURL
	DefaultTimeout = sharedHF.DefaultTimeout
)

// Re-export types
type (
	Client       = sharedHF.Client
	ClientOption = sharedHF.ClientOption
	ModelInfo    = sharedHF.ModelInfo
	CardData     = sharedHF.CardData
	Sibling      = sharedHF.Sibling
	LFSInfo      = sharedHF.LFSInfo
	UserInfo     = sharedHF.UserInfo
	OrgInfo      = sharedHF.OrgInfo
	RepoFile     = sharedHF.RepoFile
)

// Re-export constructor and options
var (
	NewClient   = sharedHF.NewClient
	WithToken   = sharedHF.WithToken
	WithBaseURL = sharedHF.WithBaseURL
	WithTimeout = func(timeout time.Duration) ClientOption {
		return sharedHF.WithTimeout(timeout)
	}
)

// Re-export download types
type (
	DownloadOptions  = sharedHF.DownloadOptions
	DownloadResult   = sharedHF.DownloadResult
	DownloadedFile   = sharedHF.DownloadedFile
)

// Re-export functions
var (
	DefaultDownloadOptions = sharedHF.DefaultDownloadOptions
	ComputeFileChecksum    = sharedHF.ComputeFileChecksum
)

// Ensure interface compatibility at compile time
var _ interface {
	GetModel(ctx context.Context, modelID string) (*ModelInfo, error)
	ModelExists(ctx context.Context, modelID string) (bool, error)
	ListFiles(ctx context.Context, modelID, revision string) ([]RepoFile, error)
	DownloadModel(ctx context.Context, modelID string, opts DownloadOptions) (*DownloadResult, error)
	GetModelSize(ctx context.Context, modelID, revision string) (int64, error)
} = (*Client)(nil)
