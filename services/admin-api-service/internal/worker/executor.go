// Package worker implements the background job worker for server-side model caching.
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ai-aas/shared-go/modelcache/pull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PullConfig contains configuration for the pull service
type PullConfig struct {
	HFToken     string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string
	Concurrency int
}

// PullResult contains the result of a pull operation
type PullResult struct {
	TotalSize      int64
	FileCount      int
	ManifestDigest string
	Duration       time.Duration
}

// ExecuteOptions configures a pull execution
type ExecuteOptions struct {
	JobID                  uuid.UUID
	ModelName              string
	HFModelID              string
	Revision               string
	S3Prefix               string
	ProgressUpdateInterval time.Duration
	CheckCancelled         func() bool
}

// PullExecutor executes pull jobs using the shared library
type PullExecutor struct {
	pool   *pgxpool.Pool
	cfg    PullConfig
	logger *zap.Logger
}

// NewPullExecutor creates a new pull executor
func NewPullExecutor(pool *pgxpool.Pool, cfg PullConfig, logger *zap.Logger) *PullExecutor {
	return &PullExecutor{
		pool:   pool,
		cfg:    cfg,
		logger: logger,
	}
}

// Execute performs a model pull operation
func (e *PullExecutor) Execute(ctx context.Context, opts ExecuteOptions) (*PullResult, error) {
	// Create the pull service
	svc, err := pull.NewService(ctx, pull.Config{
		HFToken:          e.cfg.HFToken,
		S3Endpoint:       e.cfg.S3Endpoint,
		S3AccessKey:      e.cfg.S3AccessKey,
		S3SecretKey:      e.cfg.S3SecretKey,
		S3Bucket:         e.cfg.S3Bucket,
		S3Region:         e.cfg.S3Region,
		S3ForcePathStyle: true, // Required for Linode Object Storage
		Concurrency:      e.cfg.Concurrency,
	})
	if err != nil {
		return nil, fmt.Errorf("create pull service: %w", err)
	}

	// Set up progress tracking
	var (
		lastUpdate     time.Time
		progressMu     sync.Mutex
		bytesCompleted int64
		bytesTotal     int64
		currentPhase   string
	)

	updateProgress := func() {
		progressMu.Lock()
		bc := bytesCompleted
		bt := bytesTotal
		phase := currentPhase
		progressMu.Unlock()

		if bt == 0 {
			return
		}

		progress := float64(bc) / float64(bt) * 100
		if phase == "uploading" {
			// Uploading is the second half (50-100%)
			progress = 50 + progress/2
		} else {
			// Downloading is the first half (0-50%)
			progress = progress / 2
		}

		query := `
			UPDATE pull_jobs
			SET progress = $2, bytes_completed = $3, bytes_total = $4, status = $5
			WHERE id = $1
		`

		status := "downloading"
		if phase == "uploading" {
			status = "uploading"
		}

		_, err := e.pool.Exec(ctx, query, opts.JobID, progress, bc, bt, status)
		if err != nil {
			e.logger.Warn("failed to update progress", zap.Error(err))
		}
	}

	// Execute the pull
	result, err := svc.Pull(ctx, pull.PullOptions{
		ModelName: opts.ModelName,
		HFModelID: opts.HFModelID,
		Revision:  opts.Revision,
		S3Prefix:  opts.S3Prefix,
		OnProgress: func(phase string, completed, total int64) {
			progressMu.Lock()
			currentPhase = phase
			bytesCompleted = completed
			bytesTotal = total
			progressMu.Unlock()

			// Update progress at configured interval
			if time.Since(lastUpdate) >= opts.ProgressUpdateInterval {
				lastUpdate = time.Now()
				updateProgress()
			}
		},
		OnFileStart: func(filename string) {
			e.logger.Debug("starting file", zap.String("filename", filename))
		},
		OnFileComplete: func(filename string, size int64) {
			e.logger.Debug("completed file",
				zap.String("filename", filename),
				zap.Int64("size", size),
			)
		},
		CheckCancelled: opts.CheckCancelled,
	})

	if err != nil {
		return nil, fmt.Errorf("pull model: %w", err)
	}

	return &PullResult{
		TotalSize:      result.TotalSize,
		FileCount:      result.FileCount,
		ManifestDigest: result.ManifestDigest,
		Duration:       result.Duration,
	}, nil
}
