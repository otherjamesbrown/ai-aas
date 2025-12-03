// Package worker implements the background job worker for server-side model caching.
package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Config contains worker configuration
type Config struct {
	PollInterval           time.Duration
	ProgressUpdateInterval time.Duration
}

// DefaultConfig returns the default worker configuration
func DefaultConfig() Config {
	return Config{
		PollInterval:           5 * time.Second,
		ProgressUpdateInterval: 10 * time.Second,
	}
}

// CredentialGetter retrieves credentials from storage
type CredentialGetter interface {
	GetCredential(ctx context.Context, credType string) (string, error)
}

// Worker processes pull jobs in the background
type Worker struct {
	pool       *pgxpool.Pool
	credGetter CredentialGetter
	cfg        Config
	logger     *zap.Logger

	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.Mutex
	running  bool
	currentJob *uuid.UUID
}

// New creates a new background worker
func New(pool *pgxpool.Pool, credGetter CredentialGetter, cfg Config, logger *zap.Logger) *Worker {
	return &Worker{
		pool:       pool,
		credGetter: credGetter,
		cfg:        cfg,
		logger:     logger,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start begins the worker loop
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return errors.New("worker already running")
	}
	w.running = true
	w.mu.Unlock()

	w.logger.Info("starting pull job worker",
		zap.Duration("poll_interval", w.cfg.PollInterval),
		zap.Duration("progress_update_interval", w.cfg.ProgressUpdateInterval),
	)

	go w.run(ctx)
	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	w.logger.Info("stopping pull job worker")
	close(w.stopCh)
	<-w.doneCh
	w.logger.Info("pull job worker stopped")
}

// run is the main worker loop
func (w *Worker) run(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker context cancelled")
			return
		case <-w.stopCh:
			w.logger.Info("worker stop signal received")
			return
		case <-ticker.C:
			if err := w.processNextJob(ctx); err != nil {
				if !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, context.Canceled) {
					w.logger.Error("failed to process job", zap.Error(err))
				}
			}
		}
	}
}

// Job represents a pull job from the database
type Job struct {
	ID        uuid.UUID
	ModelID   uuid.UUID
	ModelName string
	HFModelID string
	Revision  string
}

// processNextJob claims and processes the next pending job
func (w *Worker) processNextJob(ctx context.Context) error {
	// Check if we're stopping
	select {
	case <-w.stopCh:
		return context.Canceled
	default:
	}

	// Claim a pending job atomically
	job, err := w.claimJob(ctx)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.currentJob = &job.ID
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.currentJob = nil
		w.mu.Unlock()
	}()

	w.logger.Info("claimed pull job",
		zap.String("job_id", job.ID.String()),
		zap.String("model_name", job.ModelName),
		zap.String("hf_model_id", job.HFModelID),
		zap.String("revision", job.Revision),
	)

	// Process the job
	if err := w.executeJob(ctx, job); err != nil {
		w.logger.Error("job execution failed",
			zap.String("job_id", job.ID.String()),
			zap.Error(err),
		)
		w.failJob(ctx, job.ID, err.Error())
		return err
	}

	w.logger.Info("job completed successfully",
		zap.String("job_id", job.ID.String()),
	)

	return nil
}

// claimJob atomically claims the next pending job
func (w *Worker) claimJob(ctx context.Context) (*Job, error) {
	query := `
		UPDATE pull_jobs
		SET status = 'downloading', started_at = NOW()
		WHERE id = (
			SELECT j.id FROM pull_jobs j
			JOIN model_registry m ON j.model_id = m.id
			WHERE j.status = 'pending'
			ORDER BY j.started_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, model_id, revision,
			(SELECT name FROM model_registry WHERE id = model_id),
			(SELECT hf_model_id FROM model_registry WHERE id = model_id)
	`

	var job Job
	err := w.pool.QueryRow(ctx, query).Scan(
		&job.ID, &job.ModelID, &job.Revision, &job.ModelName, &job.HFModelID,
	)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

// executeJob performs the actual model pull
func (w *Worker) executeJob(ctx context.Context, job *Job) error {
	// Get credentials
	hfToken, err := w.credGetter.GetCredential(ctx, "huggingface_token")
	if err != nil && !errors.Is(err, ErrCredentialNotFound) {
		return fmt.Errorf("get HuggingFace token: %w", err)
	}

	s3AccessKey, err := w.credGetter.GetCredential(ctx, "s3_access_key")
	if err != nil {
		return fmt.Errorf("get S3 access key: %w", err)
	}

	s3SecretKey, err := w.credGetter.GetCredential(ctx, "s3_secret_key")
	if err != nil {
		return fmt.Errorf("get S3 secret key: %w", err)
	}

	s3Endpoint, err := w.credGetter.GetCredential(ctx, "s3_endpoint")
	if err != nil {
		return fmt.Errorf("get S3 endpoint: %w", err)
	}

	s3Bucket, err := w.credGetter.GetCredential(ctx, "s3_bucket")
	if err != nil {
		return fmt.Errorf("get S3 bucket: %w", err)
	}

	s3Region, _ := w.credGetter.GetCredential(ctx, "s3_region")
	if s3Region == "" {
		s3Region = "us-east-1"
	}

	// Create pull service configuration
	pullCfg := PullConfig{
		HFToken:      hfToken,
		S3Endpoint:   s3Endpoint,
		S3AccessKey:  s3AccessKey,
		S3SecretKey:  s3SecretKey,
		S3Bucket:     s3Bucket,
		S3Region:     s3Region,
		Concurrency:  3,
	}

	// Create the pull executor
	executor := NewPullExecutor(w.pool, pullCfg, w.logger)

	// Build S3 path
	s3Prefix := fmt.Sprintf("models/%s/%s", job.ModelName, job.Revision)

	// Execute the pull with progress tracking
	result, err := executor.Execute(ctx, ExecuteOptions{
		JobID:     job.ID,
		ModelName: job.ModelName,
		HFModelID: job.HFModelID,
		Revision:  job.Revision,
		S3Prefix:  s3Prefix,
		ProgressUpdateInterval: w.cfg.ProgressUpdateInterval,
		CheckCancelled: func() bool {
			return w.isJobCancelled(ctx, job.ID)
		},
	})
	if err != nil {
		return err
	}

	// Create cache entry
	if err := w.createCacheEntry(ctx, job, result, s3Prefix); err != nil {
		return fmt.Errorf("create cache entry: %w", err)
	}

	// Mark job as complete
	if err := w.completeJob(ctx, job.ID, result); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	return nil
}

// isJobCancelled checks if a job has been cancelled
func (w *Worker) isJobCancelled(ctx context.Context, jobID uuid.UUID) bool {
	// Check if worker is stopping
	select {
	case <-w.stopCh:
		return true
	default:
	}

	var status string
	err := w.pool.QueryRow(ctx, `SELECT status FROM pull_jobs WHERE id = $1`, jobID).Scan(&status)
	if err != nil {
		return true // Assume cancelled on error
	}

	return status == "cancelled"
}

// failJob marks a job as failed
func (w *Worker) failJob(ctx context.Context, jobID uuid.UUID, errorMsg string) {
	query := `
		UPDATE pull_jobs
		SET status = 'failed', error_message = $2, completed_at = NOW()
		WHERE id = $1
	`
	_, err := w.pool.Exec(ctx, query, jobID, errorMsg)
	if err != nil {
		w.logger.Error("failed to mark job as failed", zap.Error(err))
	}
}

// completeJob marks a job as complete
func (w *Worker) completeJob(ctx context.Context, jobID uuid.UUID, result *PullResult) error {
	query := `
		UPDATE pull_jobs
		SET status = 'complete', progress = 100,
		    bytes_total = $2, bytes_completed = $2, completed_at = NOW()
		WHERE id = $1
	`
	_, err := w.pool.Exec(ctx, query, jobID, result.TotalSize)
	return err
}

// createCacheEntry creates a cache entry for the completed pull
func (w *Worker) createCacheEntry(ctx context.Context, job *Job, result *PullResult, s3Path string) error {
	query := `
		INSERT INTO model_cache (model_id, version, hf_revision, object_storage_path,
		                         size_bytes, file_count, checksum_sha256, status, verified_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'ready', NOW())
		ON CONFLICT (model_id, version) DO UPDATE
		SET object_storage_path = $4, size_bytes = $5, file_count = $6,
		    checksum_sha256 = $7, status = 'ready', verified_at = NOW()
	`
	_, err := w.pool.Exec(ctx, query,
		job.ModelID, job.Revision, job.Revision, s3Path,
		result.TotalSize, result.FileCount, result.ManifestDigest,
	)
	return err
}

// ErrCredentialNotFound indicates a credential was not found
var ErrCredentialNotFound = errors.New("credential not found")
