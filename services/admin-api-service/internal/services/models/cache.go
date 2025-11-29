// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrCacheEntryNotFound indicates the requested cache entry was not found
var ErrCacheEntryNotFound = errors.New("cache entry not found")

// ErrPullJobNotFound indicates the requested pull job was not found
var ErrPullJobNotFound = errors.New("pull job not found")

// CacheEntry represents a cached model version in object storage
type CacheEntry struct {
	ID                uuid.UUID
	ModelID           uuid.UUID
	Version           string
	HFRevision        string
	ObjectStoragePath string
	SizeBytes         *int64
	FileCount         *int
	ChecksumSHA256    *string
	Status            string
	CachedAt          time.Time
	VerifiedAt        *time.Time
}

// CacheStatus constants
const (
	CacheStatusDownloading = "downloading"
	CacheStatusReady       = "ready"
	CacheStatusFailed      = "failed"
	CacheStatusDeleted     = "deleted"
)

// PullJob represents a model pull operation
type PullJob struct {
	ID             uuid.UUID
	ModelID        uuid.UUID
	Revision       string
	Status         string
	Progress       float64
	BytesTotal     *int64
	BytesCompleted int64
	ErrorMessage   *string
	StartedAt      time.Time
	CompletedAt    *time.Time
	CreatedBy      *string
}

// PullJobStatus constants
const (
	PullStatusPending     = "pending"
	PullStatusDownloading = "downloading"
	PullStatusUploading   = "uploading"
	PullStatusComplete    = "complete"
	PullStatusFailed      = "failed"
	PullStatusCancelled   = "cancelled"
)

// GetModelCache returns all cache entries for a model by name
func (s *Service) GetModelCache(ctx context.Context, modelName string) ([]CacheEntry, error) {
	query := `
		SELECT c.id, c.model_id, c.version, c.hf_revision, c.object_storage_path,
		       c.size_bytes, c.file_count, c.checksum_sha256, c.status,
		       c.cached_at, c.verified_at
		FROM model_cache c
		JOIN model_registry r ON c.model_id = r.id
		WHERE r.name = $1
		ORDER BY c.cached_at DESC
	`

	rows, err := s.pool.Query(ctx, query, modelName)
	if err != nil {
		return nil, fmt.Errorf("query model cache: %w", err)
	}
	defer rows.Close()

	var entries []CacheEntry
	for rows.Next() {
		var e CacheEntry
		err := rows.Scan(
			&e.ID, &e.ModelID, &e.Version, &e.HFRevision, &e.ObjectStoragePath,
			&e.SizeBytes, &e.FileCount, &e.ChecksumSHA256, &e.Status,
			&e.CachedAt, &e.VerifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan cache entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// GetCacheEntry returns a specific cache entry by model name and version
func (s *Service) GetCacheEntry(ctx context.Context, modelName, version string) (*CacheEntry, error) {
	query := `
		SELECT c.id, c.model_id, c.version, c.hf_revision, c.object_storage_path,
		       c.size_bytes, c.file_count, c.checksum_sha256, c.status,
		       c.cached_at, c.verified_at
		FROM model_cache c
		JOIN model_registry r ON c.model_id = r.id
		WHERE r.name = $1 AND c.version = $2
	`

	var e CacheEntry
	err := s.pool.QueryRow(ctx, query, modelName, version).Scan(
		&e.ID, &e.ModelID, &e.Version, &e.HFRevision, &e.ObjectStoragePath,
		&e.SizeBytes, &e.FileCount, &e.ChecksumSHA256, &e.Status,
		&e.CachedAt, &e.VerifiedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCacheEntryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query cache entry: %w", err)
	}

	return &e, nil
}

// CreateCacheEntryRequest contains data for creating a cache entry
type CreateCacheEntryRequest struct {
	ModelID           uuid.UUID
	Version           string
	HFRevision        string
	ObjectStoragePath string
}

// CreateCacheEntry creates a new cache entry (typically in downloading status)
func (s *Service) CreateCacheEntry(ctx context.Context, req CreateCacheEntryRequest) (*CacheEntry, error) {
	query := `
		INSERT INTO model_cache (model_id, version, hf_revision, object_storage_path, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, cached_at
	`

	var e CacheEntry
	e.ModelID = req.ModelID
	e.Version = req.Version
	e.HFRevision = req.HFRevision
	e.ObjectStoragePath = req.ObjectStoragePath
	e.Status = CacheStatusDownloading

	err := s.pool.QueryRow(ctx, query,
		e.ModelID, e.Version, e.HFRevision, e.ObjectStoragePath, e.Status,
	).Scan(&e.ID, &e.CachedAt)

	if err != nil {
		return nil, fmt.Errorf("insert cache entry: %w", err)
	}

	return &e, nil
}

// UpdateCacheEntryRequest contains data for updating a cache entry
type UpdateCacheEntryRequest struct {
	SizeBytes      *int64
	FileCount      *int
	ChecksumSHA256 *string
	Status         string
}

// UpdateCacheEntry updates a cache entry's status and metadata
func (s *Service) UpdateCacheEntry(ctx context.Context, id uuid.UUID, req UpdateCacheEntryRequest) error {
	query := `
		UPDATE model_cache
		SET status = $2, size_bytes = COALESCE($3, size_bytes),
		    file_count = COALESCE($4, file_count),
		    checksum_sha256 = COALESCE($5, checksum_sha256),
		    verified_at = CASE WHEN $2 = 'ready' THEN NOW() ELSE verified_at END
		WHERE id = $1
	`

	result, err := s.pool.Exec(ctx, query, id, req.Status, req.SizeBytes, req.FileCount, req.ChecksumSHA256)
	if err != nil {
		return fmt.Errorf("update cache entry: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrCacheEntryNotFound
	}

	return nil
}

// DeleteCacheEntry marks a cache entry as deleted
func (s *Service) DeleteCacheEntry(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE model_cache SET status = 'deleted' WHERE id = $1`

	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete cache entry: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrCacheEntryNotFound
	}

	return nil
}

// VerifyResult contains the result of a cache verification
type VerifyResult struct {
	Valid         bool
	FilesChecked  int
	FilesMissing  []string
	FilesCorrupt  []string
	ChecksumMatch bool
	VerifiedAt    time.Time
}

// VerifyCache verifies the cache integrity for a model version
// Note: Actual file verification would require object storage access
// This is a placeholder that verifies database records
func (s *Service) VerifyCache(ctx context.Context, modelName, version string) (*VerifyResult, error) {
	entry, err := s.GetCacheEntry(ctx, modelName, version)
	if err != nil {
		return nil, err
	}

	result := &VerifyResult{
		VerifiedAt: time.Now(),
	}

	// Basic validation: entry exists and has expected metadata
	if entry.Status == CacheStatusReady {
		result.Valid = true
		result.ChecksumMatch = entry.ChecksumSHA256 != nil
		if entry.FileCount != nil {
			result.FilesChecked = *entry.FileCount
		}
	} else {
		result.Valid = false
	}

	// Update verified_at timestamp
	if result.Valid {
		_, err = s.pool.Exec(ctx, `UPDATE model_cache SET verified_at = NOW() WHERE id = $1`, entry.ID)
		if err != nil {
			return nil, fmt.Errorf("update verified_at: %w", err)
		}
	}

	return result, nil
}

// Pull job operations

// CreatePullJobRequest contains data for creating a pull job
type CreatePullJobRequest struct {
	ModelName string
	Revision  string
	CreatedBy string
}

// CreatePullJob creates a new pull job for a model
func (s *Service) CreatePullJob(ctx context.Context, req CreatePullJobRequest) (*PullJob, error) {
	// First, get the model ID
	model, err := s.GetModel(ctx, req.ModelName)
	if err != nil {
		return nil, err
	}

	revision := req.Revision
	if revision == "" {
		revision = "main"
	}

	query := `
		INSERT INTO pull_jobs (model_id, revision, status, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, started_at
	`

	var job PullJob
	job.ModelID = model.ID
	job.Revision = revision
	job.Status = PullStatusPending
	if req.CreatedBy != "" {
		job.CreatedBy = &req.CreatedBy
	}

	err = s.pool.QueryRow(ctx, query,
		job.ModelID, job.Revision, job.Status, job.CreatedBy,
	).Scan(&job.ID, &job.StartedAt)

	if err != nil {
		return nil, fmt.Errorf("insert pull job: %w", err)
	}

	return &job, nil
}

// GetPullJob returns a pull job by ID
func (s *Service) GetPullJob(ctx context.Context, id uuid.UUID) (*PullJob, error) {
	query := `
		SELECT id, model_id, revision, status, progress, bytes_total,
		       bytes_completed, error_message, started_at, completed_at, created_by
		FROM pull_jobs
		WHERE id = $1
	`

	var job PullJob
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&job.ID, &job.ModelID, &job.Revision, &job.Status, &job.Progress,
		&job.BytesTotal, &job.BytesCompleted, &job.ErrorMessage,
		&job.StartedAt, &job.CompletedAt, &job.CreatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPullJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query pull job: %w", err)
	}

	return &job, nil
}

// ListPullJobs returns pull jobs for a model
func (s *Service) ListPullJobs(ctx context.Context, modelName string) ([]PullJob, error) {
	query := `
		SELECT j.id, j.model_id, j.revision, j.status, j.progress, j.bytes_total,
		       j.bytes_completed, j.error_message, j.started_at, j.completed_at, j.created_by
		FROM pull_jobs j
		JOIN model_registry r ON j.model_id = r.id
		WHERE r.name = $1
		ORDER BY j.started_at DESC
		LIMIT 10
	`

	rows, err := s.pool.Query(ctx, query, modelName)
	if err != nil {
		return nil, fmt.Errorf("query pull jobs: %w", err)
	}
	defer rows.Close()

	var jobs []PullJob
	for rows.Next() {
		var job PullJob
		err := rows.Scan(
			&job.ID, &job.ModelID, &job.Revision, &job.Status, &job.Progress,
			&job.BytesTotal, &job.BytesCompleted, &job.ErrorMessage,
			&job.StartedAt, &job.CompletedAt, &job.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan pull job: %w", err)
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// UpdatePullJobRequest contains data for updating a pull job
type UpdatePullJobRequest struct {
	Status         string
	Progress       float64
	BytesTotal     *int64
	BytesCompleted int64
	ErrorMessage   *string
}

// UpdatePullJob updates a pull job's status and progress
func (s *Service) UpdatePullJob(ctx context.Context, id uuid.UUID, req UpdatePullJobRequest) error {
	query := `
		UPDATE pull_jobs
		SET status = $2, progress = $3, bytes_total = COALESCE($4, bytes_total),
		    bytes_completed = $5, error_message = $6,
		    completed_at = CASE WHEN $2 IN ('complete', 'failed', 'cancelled') THEN NOW() ELSE completed_at END
		WHERE id = $1
	`

	result, err := s.pool.Exec(ctx, query,
		id, req.Status, req.Progress, req.BytesTotal, req.BytesCompleted, req.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("update pull job: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPullJobNotFound
	}

	return nil
}

// CancelPullJob cancels a pending or in-progress pull job
func (s *Service) CancelPullJob(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE pull_jobs
		SET status = 'cancelled', completed_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'downloading', 'uploading')
	`

	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("cancel pull job: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("pull job not found or already completed")
	}

	return nil
}
