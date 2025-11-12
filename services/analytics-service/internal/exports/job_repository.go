// Package exports provides export job lifecycle management.
package exports

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExportJobRepository manages export job lifecycle in the database.
type ExportJobRepository struct {
	pool *pgxpool.Pool
}

// NewExportJobRepository creates a new export job repository.
func NewExportJobRepository(pool *pgxpool.Pool) *ExportJobRepository {
	return &ExportJobRepository{pool: pool}
}

// ExportJob represents an export job record.
type ExportJob struct {
	JobID          uuid.UUID
	OrgID          uuid.UUID
	RequestedBy    uuid.UUID
	TimeRangeStart time.Time
	TimeRangeEnd   time.Time
	Granularity    string // "hourly", "daily", "monthly"
	Status         string // "pending", "running", "succeeded", "failed", "expired"
	OutputURI      *string
	Checksum       *string
	RowCount       *int64
	InitiatedAt    time.Time
	CompletedAt    *time.Time
	ErrorMessage   *string
}

// CreateExportJobRequest specifies parameters for creating a new export job.
type CreateExportJobRequest struct {
	OrgID          uuid.UUID
	RequestedBy    uuid.UUID
	TimeRangeStart time.Time
	TimeRangeEnd   time.Time
	Granularity    string // "hourly", "daily", "monthly"
}

// CreateExportJob creates a new export job with status "pending".
func (r *ExportJobRepository) CreateExportJob(ctx context.Context, req CreateExportJobRequest) (uuid.UUID, error) {
	query := `
		INSERT INTO analytics.export_jobs (
			org_id, requested_by, time_range_start, time_range_end, granularity, status
		) VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING job_id
	`

	var jobID uuid.UUID
	err := r.pool.QueryRow(ctx, query,
		req.OrgID,
		req.RequestedBy,
		req.TimeRangeStart,
		req.TimeRangeEnd,
		req.Granularity,
	).Scan(&jobID)

	if err != nil {
		return uuid.Nil, fmt.Errorf("create export job: %w", err)
	}

	return jobID, nil
}

// GetExportJob retrieves an export job by ID and org ID.
func (r *ExportJobRepository) GetExportJob(ctx context.Context, orgID, jobID uuid.UUID) (*ExportJob, error) {
	query := `
		SELECT 
			job_id, org_id, requested_by, time_range_start, time_range_end,
			granularity, status, output_uri, checksum, row_count,
			initiated_at, completed_at, error_message
		FROM analytics.export_jobs
		WHERE job_id = $1 AND org_id = $2
	`

	var job ExportJob
	var outputURI, checksum, errorMessage *string
	var rowCount *int64
	var completedAt *time.Time

	err := r.pool.QueryRow(ctx, query, jobID, orgID).Scan(
		&job.JobID,
		&job.OrgID,
		&job.RequestedBy,
		&job.TimeRangeStart,
		&job.TimeRangeEnd,
		&job.Granularity,
		&job.Status,
		&outputURI,
		&checksum,
		&rowCount,
		&job.InitiatedAt,
		&completedAt,
		&errorMessage,
	)

	if err != nil {
		return nil, fmt.Errorf("get export job: %w", err)
	}

	job.OutputURI = outputURI
	job.Checksum = checksum
	job.RowCount = rowCount
	job.CompletedAt = completedAt
	job.ErrorMessage = errorMessage

	return &job, nil
}

// ListExportJobs retrieves export jobs for an organization, optionally filtered by status.
func (r *ExportJobRepository) ListExportJobs(ctx context.Context, orgID uuid.UUID, statusFilter *string) ([]ExportJob, error) {
	query := `
		SELECT 
			job_id, org_id, requested_by, time_range_start, time_range_end,
			granularity, status, output_uri, checksum, row_count,
			initiated_at, completed_at, error_message
		FROM analytics.export_jobs
		WHERE org_id = $1
	`

	args := []interface{}{orgID}
	argIdx := 2

	if statusFilter != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *statusFilter)
		argIdx++
	}

	query += " ORDER BY initiated_at DESC LIMIT 100"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list export jobs: %w", err)
	}
	defer rows.Close()

	var jobs []ExportJob
	for rows.Next() {
		var job ExportJob
		var outputURI, checksum, errorMessage *string
		var rowCount *int64
		var completedAt *time.Time

		err := rows.Scan(
			&job.JobID,
			&job.OrgID,
			&job.RequestedBy,
			&job.TimeRangeStart,
			&job.TimeRangeEnd,
			&job.Granularity,
			&job.Status,
			&outputURI,
			&checksum,
			&rowCount,
			&job.InitiatedAt,
			&completedAt,
			&errorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("scan export job: %w", err)
		}

		job.OutputURI = outputURI
		job.Checksum = checksum
		job.RowCount = rowCount
		job.CompletedAt = completedAt
		job.ErrorMessage = errorMessage

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// UpdateExportJobStatus updates the status of an export job.
func (r *ExportJobRepository) UpdateExportJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	query := `
		UPDATE analytics.export_jobs
		SET status = $1
		WHERE job_id = $2
	`

	_, err := r.pool.Exec(ctx, query, status, jobID)
	if err != nil {
		return fmt.Errorf("update export job status: %w", err)
	}

	return nil
}

// SetExportJobOutput sets the output URI, checksum, and row count for a completed export job.
func (r *ExportJobRepository) SetExportJobOutput(ctx context.Context, jobID uuid.UUID, outputURI, checksum string, rowCount int64) error {
	query := `
		UPDATE analytics.export_jobs
		SET output_uri = $1, checksum = $2, row_count = $3, completed_at = NOW(), status = 'succeeded'
		WHERE job_id = $4
	`

	_, err := r.pool.Exec(ctx, query, outputURI, checksum, rowCount, jobID)
	if err != nil {
		return fmt.Errorf("set export job output: %w", err)
	}

	return nil
}

// SetExportJobError marks an export job as failed with an error message.
func (r *ExportJobRepository) SetExportJobError(ctx context.Context, jobID uuid.UUID, errorMessage string) error {
	query := `
		UPDATE analytics.export_jobs
		SET status = 'failed', error_message = $1, completed_at = NOW()
		WHERE job_id = $2
	`

	_, err := r.pool.Exec(ctx, query, errorMessage, jobID)
	if err != nil {
		return fmt.Errorf("set export job error: %w", err)
	}

	return nil
}

// GetPendingJobs retrieves pending export jobs for processing (used by worker).
func (r *ExportJobRepository) GetPendingJobs(ctx context.Context, limit int) ([]ExportJob, error) {
	query := `
		SELECT 
			job_id, org_id, requested_by, time_range_start, time_range_end,
			granularity, status, output_uri, checksum, row_count,
			initiated_at, completed_at, error_message
		FROM analytics.export_jobs
		WHERE status = 'pending'
		ORDER BY initiated_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []ExportJob
	for rows.Next() {
		var job ExportJob
		var outputURI, checksum, errorMessage *string
		var rowCount *int64
		var completedAt *time.Time

		err := rows.Scan(
			&job.JobID,
			&job.OrgID,
			&job.RequestedBy,
			&job.TimeRangeStart,
			&job.TimeRangeEnd,
			&job.Granularity,
			&job.Status,
			&outputURI,
			&checksum,
			&rowCount,
			&job.InitiatedAt,
			&completedAt,
			&errorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("scan export job: %w", err)
		}

		job.OutputURI = outputURI
		job.Checksum = checksum
		job.RowCount = rowCount
		job.CompletedAt = completedAt
		job.ErrorMessage = errorMessage

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

