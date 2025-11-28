// Package models provides types for model management API.
package models

import "time"

// Model represents a registered model in the platform
type Model struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	HFModelID              string                 `json:"hf_model_id"`
	HFRevision             string                 `json:"hf_revision"`
	RequiresAuth           bool                   `json:"requires_auth"`
	IsGated                bool                   `json:"is_gated"`
	LicenseType            string                 `json:"license_type,omitempty"`
	LicenseURL             string                 `json:"license_url,omitempty"`
	LicenseAcceptedAt      *time.Time             `json:"license_accepted_at,omitempty"`
	LicenseAcceptedBy      string                 `json:"license_accepted_by,omitempty"`
	RecommendedGPUMemoryGB int                    `json:"recommended_gpu_memory_gb,omitempty"`
	RecommendedCPUMemoryGB int                    `json:"recommended_cpu_memory_gb,omitempty"`
	PinnedVersion          string                 `json:"pinned_version,omitempty"`
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`

	// Computed fields from joins
	CacheStatus      string `json:"cache_status,omitempty"`
	DeploymentStatus string `json:"deployment_status,omitempty"`
}

// AddModelRequest represents a request to add a new model
type AddModelRequest struct {
	HFModelID      string `json:"hf_model_id"`
	Name           string `json:"name"`
	RequiresAuth   bool   `json:"requires_auth"`
	LicenseType    string `json:"license_type,omitempty"`
	AcceptLicense  bool   `json:"accept_license"`
	GPUMemoryGB    int    `json:"gpu_memory_gb,omitempty"`
	CPUMemoryGB    int    `json:"cpu_memory_gb,omitempty"`
}

// ListModelsOptions represents query options for listing models
type ListModelsOptions struct {
	Cached      bool
	Deployed    bool
	Orphaned    bool
	Environment string
}

// CacheEntry represents a cached model version
type CacheEntry struct {
	ID                string     `json:"id"`
	ModelID           string     `json:"model_id"`
	Version           string     `json:"version"`
	HFRevision        string     `json:"hf_revision,omitempty"`
	ObjectStoragePath string     `json:"object_storage_path"`
	SizeBytes         int64      `json:"size_bytes,omitempty"`
	FileCount         int        `json:"file_count,omitempty"`
	ChecksumSHA256    string     `json:"checksum_sha256,omitempty"`
	Status            string     `json:"status"`
	CachedAt          time.Time  `json:"cached_at"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
}

// PullOptions represents options for pulling a model
type PullOptions struct {
	Revision string `json:"revision,omitempty"`
	DryRun   bool   `json:"dry_run,omitempty"`
}

// PullJob represents an in-progress or completed pull operation
type PullJob struct {
	ID             string     `json:"id"`
	ModelName      string     `json:"model_name"`
	Status         string     `json:"status"` // pending, downloading, uploading, complete, failed
	Progress       float64    `json:"progress"`
	BytesTotal     int64      `json:"bytes_total,omitempty"`
	BytesCompleted int64      `json:"bytes_completed,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Error          string     `json:"error,omitempty"`
}

// VerifyResult represents the result of cache verification
type VerifyResult struct {
	Valid         bool     `json:"valid"`
	FilesChecked  int      `json:"files_checked"`
	FilesMissing  []string `json:"files_missing,omitempty"`
	FilesCorrupt  []string `json:"files_corrupt,omitempty"`
	ChecksumMatch bool     `json:"checksum_match"`
	VerifiedAt    time.Time `json:"verified_at"`
}

// Credential represents a stored credential (value is never exposed)
type Credential struct {
	Type      string                 `json:"type"`
	Masked    string                 `json:"masked"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SetCredentialRequest represents a request to set a credential
type SetCredentialRequest struct {
	Type     string                 `json:"type"`
	Value    string                 `json:"value"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CredentialTestResult represents the result of testing a credential
type CredentialTestResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
	// For HF token, includes account info
	AccountInfo map[string]interface{} `json:"account_info,omitempty"`
}

