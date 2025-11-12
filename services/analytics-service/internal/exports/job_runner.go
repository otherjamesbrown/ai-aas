// Package exports provides export job processing worker.
package exports

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// JobRunner processes export jobs and generates CSVs from rollup tables.
type JobRunner struct {
	repo       *ExportJobRepository
	pool       *pgxpool.Pool
	s3Delivery *S3Delivery
	logger     *zap.Logger
	interval   time.Duration
	workers    int
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// RunnerConfig holds job runner configuration.
type RunnerConfig struct {
	Pool       *pgxpool.Pool
	S3Delivery *S3Delivery
	Logger     *zap.Logger
	Interval   time.Duration
	Workers    int
}

// NewJobRunner creates a new export job runner.
func NewJobRunner(cfg RunnerConfig) *JobRunner {
	repo := NewExportJobRepository(cfg.Pool)
	return &JobRunner{
		repo:       repo,
		pool:       cfg.Pool,
		s3Delivery: cfg.S3Delivery,
		logger:     cfg.Logger,
		interval:   cfg.Interval,
		workers:    cfg.Workers,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start begins the export job processing loop.
func (r *JobRunner) Start(ctx context.Context) error {
	r.logger.Info("starting export job runner",
		zap.Duration("interval", r.interval),
		zap.Int("workers", r.workers),
	)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Start worker goroutines
	workerDone := make(chan struct{}, r.workers)
	for i := 0; i < r.workers; i++ {
		go r.worker(ctx, i, workerDone)
	}

	// Wait for all workers to finish
	go func() {
		for i := 0; i < r.workers; i++ {
			<-workerDone
		}
		close(r.doneCh)
	}()

	// Main loop - ticker is used to trigger periodic checks, but workers poll continuously
	select {
	case <-ctx.Done():
		r.logger.Info("export job runner stopping due to context cancellation")
		close(r.stopCh)
		<-r.doneCh
		return nil
	case <-r.stopCh:
		r.logger.Info("export job runner stopping")
		<-r.doneCh
		return nil
	}
}

// Stop gracefully stops the runner.
func (r *JobRunner) Stop() {
	close(r.stopCh)
	<-r.doneCh
}

// worker processes export jobs in a loop.
func (r *JobRunner) worker(ctx context.Context, id int, done chan struct{}) {
	defer func() { done <- struct{}{} }()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("export worker stopping", zap.Int("worker_id", id))
			return
		case <-r.stopCh:
			r.logger.Info("export worker stopping", zap.Int("worker_id", id))
			return
		case <-ticker.C:
			// Poll for pending jobs
			jobs, err := r.repo.GetPendingJobs(ctx, 1) // Process one job at a time per worker
			if err != nil {
				r.logger.Error("failed to get pending jobs", zap.Error(err), zap.Int("worker_id", id))
				continue
			}

			if len(jobs) == 0 {
				continue // No jobs to process
			}

			// Process each job
			for _, job := range jobs {
				if err := r.ProcessJob(ctx, job); err != nil {
					r.logger.Error("failed to process export job",
						zap.String("job_id", job.JobID.String()),
						zap.Error(err),
						zap.Int("worker_id", id),
					)
					// Mark job as failed
					if err := r.repo.SetExportJobError(ctx, job.JobID, err.Error()); err != nil {
						r.logger.Error("failed to mark job as failed",
							zap.String("job_id", job.JobID.String()),
							zap.Error(err),
						)
					}
				}
			}
		}
	}
}

// ProcessJob processes a single export job.
// This method is public to allow testing and manual job processing.
func (r *JobRunner) ProcessJob(ctx context.Context, job ExportJob) error {
	// Mark job as running
	if err := r.repo.UpdateExportJobStatus(ctx, job.JobID, "running"); err != nil {
		return fmt.Errorf("update job status to running: %w", err)
	}

	r.logger.Info("processing export job",
		zap.String("job_id", job.JobID.String()),
		zap.String("org_id", job.OrgID.String()),
		zap.String("granularity", job.Granularity),
		zap.Time("start", job.TimeRangeStart),
		zap.Time("end", job.TimeRangeEnd),
	)

	// Generate CSV from rollup tables
	csvData, rowCount, err := r.generateCSV(ctx, job)
	if err != nil {
		return fmt.Errorf("generate CSV: %w", err)
	}

	// Upload to Linode Object Storage
	signedURL, checksum, err := r.s3Delivery.UploadCSV(ctx, job.OrgID, job.JobID, csvData)
	if err != nil {
		return fmt.Errorf("upload CSV: %w", err)
	}

	// Update job with output
	if err := r.repo.SetExportJobOutput(ctx, job.JobID, signedURL, checksum, rowCount); err != nil {
		return fmt.Errorf("set export job output: %w", err)
	}

	r.logger.Info("export job completed",
		zap.String("job_id", job.JobID.String()),
		zap.String("org_id", job.OrgID.String()),
		zap.Int64("row_count", rowCount),
		zap.String("checksum", checksum),
	)

	return nil
}

// generateCSV generates CSV data from rollup tables based on granularity.
func (r *JobRunner) generateCSV(ctx context.Context, job ExportJob) ([]byte, int64, error) {
	var query string
	var args []interface{}

	switch job.Granularity {
	case "hourly":
		query = `
			SELECT 
				bucket_start,
				organization_id,
				model_id,
				request_count,
				tokens_total,
				error_count,
				cost_total
			FROM analytics_hourly_rollups
			WHERE organization_id = $1
				AND bucket_start >= $2
				AND bucket_start < $3
			ORDER BY bucket_start ASC, model_id ASC
		`
		args = []interface{}{job.OrgID, job.TimeRangeStart, job.TimeRangeEnd}

	case "daily":
		query = `
			SELECT 
				bucket_start,
				organization_id,
				model_id,
				request_count,
				tokens_total,
				error_count,
				cost_total
			FROM analytics_daily_rollups
			WHERE organization_id = $1
				AND bucket_start >= $2::date
				AND bucket_start < $3::date
			ORDER BY bucket_start ASC, model_id ASC
		`
		args = []interface{}{job.OrgID, job.TimeRangeStart, job.TimeRangeEnd}

	case "monthly":
		// Aggregate from daily rollups for monthly granularity
		query = `
			SELECT 
				date_trunc('month', bucket_start)::date AS bucket_start,
				organization_id,
				model_id,
				SUM(request_count) AS request_count,
				SUM(tokens_total) AS tokens_total,
				SUM(error_count) AS error_count,
				SUM(cost_total) AS cost_total
			FROM analytics_daily_rollups
			WHERE organization_id = $1
				AND bucket_start >= date_trunc('month', $2::date)
				AND bucket_start < date_trunc('month', $3::date) + INTERVAL '1 month'
			GROUP BY 1, 2, 3
			ORDER BY bucket_start ASC, model_id ASC
		`
		args = []interface{}{job.OrgID, job.TimeRangeStart, job.TimeRangeEnd}

	default:
		return nil, 0, fmt.Errorf("unsupported granularity: %s", job.Granularity)
	}

	// Execute query
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query rollup data: %w", err)
	}
	defer rows.Close()

	// Generate CSV
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"bucket_start",
		"organization_id",
		"model_id",
		"request_count",
		"tokens_total",
		"error_count",
		"cost_total",
	}
	if err := writer.Write(header); err != nil {
		return nil, 0, fmt.Errorf("write CSV header: %w", err)
	}

	// Write rows
	rowCount := int64(0)
	for rows.Next() {
		var bucketStart time.Time
		var orgID uuid.UUID
		var modelID *uuid.UUID
		var requestCount, tokensTotal, errorCount int64
		var costTotal float64

		err := rows.Scan(
			&bucketStart,
			&orgID,
			&modelID,
			&requestCount,
			&tokensTotal,
			&errorCount,
			&costTotal,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan rollup row: %w", err)
		}

		// Format row
		row := []string{
			bucketStart.Format(time.RFC3339),
			orgID.String(),
			formatUUID(modelID),
			fmt.Sprintf("%d", requestCount),
			fmt.Sprintf("%d", tokensTotal),
			fmt.Sprintf("%d", errorCount),
			fmt.Sprintf("%.4f", costTotal),
		}

		if err := writer.Write(row); err != nil {
			return nil, 0, fmt.Errorf("write CSV row: %w", err)
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rows: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, 0, fmt.Errorf("flush CSV: %w", err)
	}

	return buf.Bytes(), rowCount, nil
}

