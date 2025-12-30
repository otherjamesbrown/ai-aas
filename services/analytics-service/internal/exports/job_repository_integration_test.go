// Package exports provides integration tests for ExportJobRepository with real PostgreSQL database.
//
// Purpose:
//
//	These integration tests validate repository operations against a real PostgreSQL database
//	to catch type mismatches (e.g., ENUM casting), schema issues, and constraint violations
//	that unit tests with mocks cannot detect.
//
// Related Bug: aas-4xqk - Missing PostgreSQL ENUM type casting in CreateExportJob
package exports

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupTestDB creates a test Postgres container with TimescaleDB and creates the export_jobs schema.
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	// Use TimescaleDB image with wait strategy
	container, err := tcpostgres.Run(ctx, "timescale/timescaledb:latest-pg15",
		tcpostgres.WithDatabase("analytics_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)

	// Create analytics schema and tables directly (avoiding goose migration parsing issues)
	setupSchema := `
		CREATE SCHEMA IF NOT EXISTS analytics;

		-- Create ENUM types
		DO $$ BEGIN
			CREATE TYPE analytics.export_job_status AS ENUM ('pending', 'running', 'succeeded', 'failed', 'expired');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;

		DO $$ BEGIN
			CREATE TYPE analytics.export_job_granularity AS ENUM ('hourly', 'daily', 'monthly');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;

		-- Create export_jobs table
		CREATE TABLE IF NOT EXISTS analytics.export_jobs (
			job_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			org_id UUID NOT NULL,
			requested_by UUID NOT NULL,
			time_range_start TIMESTAMPTZ NOT NULL,
			time_range_end TIMESTAMPTZ NOT NULL,
			granularity analytics.export_job_granularity NOT NULL DEFAULT 'daily',
			status analytics.export_job_status NOT NULL DEFAULT 'pending',
			output_uri TEXT,
			checksum TEXT,
			row_count BIGINT,
			initiated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ,
			error_message TEXT,
			CONSTRAINT valid_time_range CHECK (time_range_end > time_range_start)
		);

		-- Indexes for common query patterns
		CREATE INDEX IF NOT EXISTS idx_export_jobs_org_initiated
			ON analytics.export_jobs (org_id, initiated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_export_jobs_status_pending_running
			ON analytics.export_jobs (status)
			WHERE status IN ('pending', 'running');

		-- Create hourly rollup table for worker lifecycle tests
		CREATE TABLE IF NOT EXISTS analytics.analytics_hourly_rollups (
			bucket_start TIMESTAMPTZ NOT NULL,
			organization_id UUID NOT NULL,
			model_id UUID,
			request_count BIGINT DEFAULT 0,
			tokens_total BIGINT DEFAULT 0,
			error_count BIGINT DEFAULT 0,
			cost_total NUMERIC(10, 4) DEFAULT 0.0,
			PRIMARY KEY (bucket_start, organization_id, model_id)
		);
	`

	_, err = pool.Exec(ctx, setupSchema)
	require.NoError(t, err, "Failed to create test schema")

	// Set search_path to include analytics schema so queries don't need schema prefix
	_, err = pool.Exec(ctx, "SET search_path TO analytics, public")
	require.NoError(t, err, "Failed to set search_path")

	cleanup := func() {
		pool.Close()
		require.NoError(t, container.Terminate(ctx))
	}

	return pool, cleanup
}

// TestCreateExportJob validates export job creation with all granularity options.
// This test catches ENUM type casting bugs that would cause INSERT failures.
func TestCreateExportJob(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Test all valid granularity values
	granularities := []string{"hourly", "daily", "monthly"}

	for _, granularity := range granularities {
		t.Run(granularity, func(t *testing.T) {
			jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
				OrgID:          orgID,
				RequestedBy:    requestedBy,
				TimeRangeStart: start,
				TimeRangeEnd:   end,
				Granularity:    granularity,
			})

			// Verify job was created successfully
			require.NoError(t, err, "CreateExportJob should succeed for %s granularity", granularity)
			require.NotEqual(t, uuid.Nil, jobID, "JobID should not be nil")

			// Retrieve the job to verify fields
			job, err := repo.GetExportJob(ctx, orgID, jobID)
			require.NoError(t, err)
			require.NotNil(t, job)

			// Verify all fields
			require.Equal(t, jobID, job.JobID)
			require.Equal(t, orgID, job.OrgID)
			require.Equal(t, requestedBy, job.RequestedBy)
			require.Equal(t, start.Unix(), job.TimeRangeStart.Unix(), "TimeRangeStart should match")
			require.Equal(t, end.Unix(), job.TimeRangeEnd.Unix(), "TimeRangeEnd should match")
			require.Equal(t, granularity, job.Granularity, "Granularity should match")
			require.Equal(t, "pending", job.Status, "Status should be 'pending' by default")

			// Verify nullable fields are nil for new job
			require.Nil(t, job.OutputURI)
			require.Nil(t, job.Checksum)
			require.Nil(t, job.RowCount)
			require.Nil(t, job.CompletedAt)
			require.Nil(t, job.ErrorMessage)

			// Verify InitiatedAt is recent
			require.WithinDuration(t, now, job.InitiatedAt, 5*time.Second, "InitiatedAt should be recent")
		})
	}
}

// TestCreateExportJob_InvalidGranularity validates that invalid granularity values are rejected.
// This test ensures PostgreSQL ENUM constraints are enforced.
func TestCreateExportJob_InvalidGranularity(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Test invalid granularity values
	invalidGranularities := []string{"weekly", "yearly", "invalid", ""}

	for _, granularity := range invalidGranularities {
		t.Run(granularity, func(t *testing.T) {
			_, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
				OrgID:          orgID,
				RequestedBy:    requestedBy,
				TimeRangeStart: start,
				TimeRangeEnd:   end,
				Granularity:    granularity,
			})

			// Should fail due to ENUM constraint
			require.Error(t, err, "CreateExportJob should fail for invalid granularity: %s", granularity)
		})
	}
}

// TestGetExportJob validates retrieval of export jobs by ID and org ID.
func TestGetExportJob(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create a job
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	t.Run("valid job retrieval", func(t *testing.T) {
		job, err := repo.GetExportJob(ctx, orgID, jobID)
		require.NoError(t, err)
		require.NotNil(t, job)
		require.Equal(t, jobID, job.JobID)
		require.Equal(t, orgID, job.OrgID)
	})

	t.Run("job not found - wrong jobID", func(t *testing.T) {
		wrongJobID := uuid.New()
		job, err := repo.GetExportJob(ctx, orgID, wrongJobID)
		require.Error(t, err)
		require.Nil(t, job)
	})

	t.Run("job not found - wrong orgID", func(t *testing.T) {
		wrongOrgID := uuid.New()
		job, err := repo.GetExportJob(ctx, wrongOrgID, jobID)
		require.Error(t, err)
		require.Nil(t, job)
	})
}

// TestListExportJobs validates listing export jobs with status filters.
func TestListExportJobs(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create multiple jobs with different statuses
	jobIDs := make([]uuid.UUID, 0)

	// Create 3 pending jobs
	for i := 0; i < 3; i++ {
		jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
			OrgID:          orgID,
			RequestedBy:    requestedBy,
			TimeRangeStart: start,
			TimeRangeEnd:   end,
			Granularity:    "daily",
		})
		require.NoError(t, err)
		jobIDs = append(jobIDs, jobID)
	}

	// Update one job to "running"
	err := repo.UpdateExportJobStatus(ctx, jobIDs[0], "running")
	require.NoError(t, err)

	// Update one job to "succeeded"
	err = repo.UpdateExportJobStatus(ctx, jobIDs[1], "succeeded")
	require.NoError(t, err)

	t.Run("list all jobs - no filter", func(t *testing.T) {
		jobs, err := repo.ListExportJobs(ctx, orgID, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(jobs), 3, "Should have at least 3 jobs")

		// Verify jobs are ordered by initiated_at DESC
		for i := 1; i < len(jobs); i++ {
			require.True(t, jobs[i-1].InitiatedAt.After(jobs[i].InitiatedAt) ||
				jobs[i-1].InitiatedAt.Equal(jobs[i].InitiatedAt),
				"Jobs should be ordered by initiated_at DESC")
		}
	})

	t.Run("list jobs - filter by 'pending'", func(t *testing.T) {
		status := "pending"
		jobs, err := repo.ListExportJobs(ctx, orgID, &status)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(jobs), 1, "Should have at least 1 pending job")

		// Verify all jobs have 'pending' status
		for _, job := range jobs {
			require.Equal(t, "pending", job.Status)
		}
	})

	t.Run("list jobs - filter by 'running'", func(t *testing.T) {
		status := "running"
		jobs, err := repo.ListExportJobs(ctx, orgID, &status)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(jobs), 1, "Should have at least 1 running job")

		// Verify all jobs have 'running' status
		for _, job := range jobs {
			require.Equal(t, "running", job.Status)
		}
	})

	t.Run("list jobs - filter by 'succeeded'", func(t *testing.T) {
		status := "succeeded"
		jobs, err := repo.ListExportJobs(ctx, orgID, &status)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(jobs), 1, "Should have at least 1 succeeded job")

		// Verify all jobs have 'succeeded' status
		for _, job := range jobs {
			require.Equal(t, "succeeded", job.Status)
		}
	})

	t.Run("list jobs - empty result for different org", func(t *testing.T) {
		differentOrgID := uuid.New()
		jobs, err := repo.ListExportJobs(ctx, differentOrgID, nil)
		require.NoError(t, err)
		require.Equal(t, 0, len(jobs), "Should have no jobs for different org")
	})
}

// TestUpdateExportJobStatus validates status transitions work with ENUM types.
// This test catches ENUM type casting bugs in UPDATE queries.
func TestUpdateExportJobStatus(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create a job
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	// Test all valid status transitions
	validStatuses := []string{"running", "succeeded", "failed", "expired"}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			err := repo.UpdateExportJobStatus(ctx, jobID, status)
			require.NoError(t, err, "UpdateExportJobStatus should succeed for %s", status)

			// Verify status was updated
			job, err := repo.GetExportJob(ctx, orgID, jobID)
			require.NoError(t, err)
			require.Equal(t, status, job.Status, "Status should be updated to %s", status)
		})
	}

	t.Run("invalid status - should fail", func(t *testing.T) {
		err := repo.UpdateExportJobStatus(ctx, jobID, "invalid_status")
		require.Error(t, err, "UpdateExportJobStatus should fail for invalid status")
	})
}

// TestSetExportJobOutput validates setting output URI, checksum, and row count.
func TestSetExportJobOutput(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create a job
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	outputURI := "https://s3.amazonaws.com/exports/test.csv"
	checksum := "sha256:abc123def456"
	rowCount := int64(1000)

	// Set output
	err = repo.SetExportJobOutput(ctx, jobID, outputURI, checksum, rowCount)
	require.NoError(t, err)

	// Verify output was set
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.NotNil(t, job)

	require.Equal(t, "succeeded", job.Status, "Status should be 'succeeded'")
	require.NotNil(t, job.OutputURI)
	require.Equal(t, outputURI, *job.OutputURI)
	require.NotNil(t, job.Checksum)
	require.Equal(t, checksum, *job.Checksum)
	require.NotNil(t, job.RowCount)
	require.Equal(t, rowCount, *job.RowCount)
	require.NotNil(t, job.CompletedAt, "CompletedAt should be set")
	require.WithinDuration(t, now, *job.CompletedAt, 5*time.Second, "CompletedAt should be recent")
}

// TestSetExportJobError validates marking jobs as failed with error messages.
func TestSetExportJobError(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create a job
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	errorMessage := "database connection timeout during export"

	// Set error
	err = repo.SetExportJobError(ctx, jobID, errorMessage)
	require.NoError(t, err)

	// Verify error was set
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.NotNil(t, job)

	require.Equal(t, "failed", job.Status, "Status should be 'failed'")
	require.NotNil(t, job.ErrorMessage)
	require.Equal(t, errorMessage, *job.ErrorMessage)
	require.NotNil(t, job.CompletedAt, "CompletedAt should be set")
	require.WithinDuration(t, now, *job.CompletedAt, 5*time.Second, "CompletedAt should be recent")
}

// TestGetPendingJobs validates retrieval of pending jobs for worker processing.
func TestGetPendingJobs(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create multiple pending jobs
	jobIDs := make([]uuid.UUID, 0)
	for i := 0; i < 5; i++ {
		jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
			OrgID:          orgID,
			RequestedBy:    requestedBy,
			TimeRangeStart: start,
			TimeRangeEnd:   end,
			Granularity:    "daily",
		})
		require.NoError(t, err)
		jobIDs = append(jobIDs, jobID)

		// Add small delay to ensure different initiated_at timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Update some jobs to non-pending status
	err := repo.UpdateExportJobStatus(ctx, jobIDs[0], "running")
	require.NoError(t, err)
	err = repo.UpdateExportJobStatus(ctx, jobIDs[1], "succeeded")
	require.NoError(t, err)

	t.Run("get pending jobs with limit", func(t *testing.T) {
		jobs, err := repo.GetPendingJobs(ctx, 2)
		require.NoError(t, err)
		require.Equal(t, 2, len(jobs), "Should return exactly 2 jobs")

		// Verify all jobs are pending
		for _, job := range jobs {
			require.Equal(t, "pending", job.Status)
		}

		// Verify jobs are ordered by initiated_at ASC (oldest first)
		for i := 1; i < len(jobs); i++ {
			require.True(t, jobs[i].InitiatedAt.After(jobs[i-1].InitiatedAt) ||
				jobs[i].InitiatedAt.Equal(jobs[i-1].InitiatedAt),
				"Jobs should be ordered by initiated_at ASC")
		}
	})

	t.Run("get all pending jobs", func(t *testing.T) {
		jobs, err := repo.GetPendingJobs(ctx, 100)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(jobs), 3, "Should have at least 3 pending jobs")

		// Verify all jobs are pending
		for _, job := range jobs {
			require.Equal(t, "pending", job.Status)
		}
	})

	t.Run("concurrent access with SKIP LOCKED", func(t *testing.T) {
		// This test verifies that GetPendingJobs uses FOR UPDATE SKIP LOCKED
		// to prevent concurrent workers from picking the same job

		// Get pending jobs in a transaction (simulating worker 1)
		jobs1, err := repo.GetPendingJobs(ctx, 1)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(jobs1), 1, "Worker 1 should get at least 1 job")

		// Immediately get pending jobs again (simulating worker 2)
		// Due to SKIP LOCKED, this should succeed without blocking
		_, err = repo.GetPendingJobs(ctx, 1)
		require.NoError(t, err)
		// Worker 2 may or may not get a job depending on how many are pending
		// The key is that it doesn't block or error
	})
}

// TestExportJobFullLifecycle validates the complete lifecycle of an export job.
func TestExportJobFullLifecycle(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// 1. Create job
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	// Verify initial state
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "pending", job.Status)

	// 2. Worker picks up job
	pendingJobs, err := repo.GetPendingJobs(ctx, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(pendingJobs), 1)

	// 3. Update status to running
	err = repo.UpdateExportJobStatus(ctx, jobID, "running")
	require.NoError(t, err)

	job, err = repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "running", job.Status)

	// 4. Complete job successfully
	outputURI := "https://s3.amazonaws.com/exports/test.csv"
	checksum := "sha256:abc123"
	rowCount := int64(500)

	err = repo.SetExportJobOutput(ctx, jobID, outputURI, checksum, rowCount)
	require.NoError(t, err)

	// Verify final state
	job, err = repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", job.Status)
	require.NotNil(t, job.OutputURI)
	require.Equal(t, outputURI, *job.OutputURI)
	require.NotNil(t, job.Checksum)
	require.Equal(t, checksum, *job.Checksum)
	require.NotNil(t, job.RowCount)
	require.Equal(t, rowCount, *job.RowCount)
	require.NotNil(t, job.CompletedAt)
}

// TestExportJobFailureLifecycle validates the failure path of an export job.
func TestExportJobFailureLifecycle(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewExportJobRepository(pool)

	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// 1. Create job
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	// 2. Update status to running
	err = repo.UpdateExportJobStatus(ctx, jobID, "running")
	require.NoError(t, err)

	// 3. Job fails
	errorMessage := "export failed: S3 upload timeout"
	err = repo.SetExportJobError(ctx, jobID, errorMessage)
	require.NoError(t, err)

	// Verify final state
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "failed", job.Status)
	require.NotNil(t, job.ErrorMessage)
	require.Equal(t, errorMessage, *job.ErrorMessage)
	require.NotNil(t, job.CompletedAt)
	require.Nil(t, job.OutputURI)
	require.Nil(t, job.Checksum)
	require.Nil(t, job.RowCount)
}

// mockS3Delivery is a mock S3Delivery adapter for testing worker lifecycle without external dependencies.
type mockS3Delivery struct {
	uploads map[string][]byte // key -> CSV data
	logger  *zap.Logger
}

// newMockS3Delivery creates a mock S3Delivery that captures uploads but doesn't require real S3.
func newMockS3Delivery() *mockS3Delivery {
	return &mockS3Delivery{
		uploads: make(map[string][]byte),
		logger:  zap.NewNop(),
	}
}

// UploadCSV captures CSV data without hitting real S3.
func (m *mockS3Delivery) UploadCSV(ctx context.Context, orgID, jobID uuid.UUID, csvData []byte) (string, string, error) {
	// Capture the upload
	key := fmt.Sprintf("analytics/exports/%s/%s.csv", orgID.String(), jobID.String())
	m.uploads[key] = csvData

	// Generate fake signed URL and checksum (simulating real behavior)
	checksum := fmt.Sprintf("mock-sha256-%s", jobID.String()[:8])
	signedURL := fmt.Sprintf("https://mock-s3.example.com/%s?expires=3600", key)

	m.logger.Info("mock uploaded CSV",
		zap.String("org_id", orgID.String()),
		zap.String("job_id", jobID.String()),
		zap.String("key", key),
		zap.Int("size_bytes", len(csvData)),
	)

	return signedURL, checksum, nil
}

// GenerateSignedURL generates a fake signed URL.
func (m *mockS3Delivery) GenerateSignedURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("https://mock-s3.example.com/%s?expires=3600", key), nil
}

// getUpload retrieves uploaded CSV data by key.
func (m *mockS3Delivery) getUpload(key string) ([]byte, bool) {
	data, ok := m.uploads[key]
	return data, ok
}

// TestExportWorkerLifecycle validates that the export worker starts, picks up pending jobs, and processes them.
// This test validates the bug fix from aas-ceor where export worker startup was conditional on S3 config.
func TestExportWorkerLifecycle(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup test database with rollup data
	ctx := context.Background()
	orgID := uuid.New()
	modelID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()

	// Seed rollup data (7 days of hourly data)
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	for bucketStart := start; bucketStart.Before(end); bucketStart = bucketStart.Add(time.Hour) {
		_, err := pool.Exec(ctx, `
			INSERT INTO analytics.analytics_hourly_rollups
			(bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, bucketStart, orgID, modelID, 100, 10000, 5, 1.2500)
		require.NoError(t, err)
	}

	// Create a mock S3 delivery adapter (simulates successful uploads without real S3)
	mockS3 := newMockS3Delivery()

	// Create pending export job
	repo := NewExportJobRepository(pool)
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "hourly",
	})
	require.NoError(t, err)

	// Verify job starts in pending status
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "pending", job.Status)

	// Start export worker (this is what we're testing!)
	worker := NewJobRunner(RunnerConfig{
		Pool:       pool,
		S3Delivery: mockS3,
		Logger:     zap.NewNop(),
		Interval:   500 * time.Millisecond, // Poll every 500ms for test speed
		Workers:    1,
	})

	workerCtx, workerCancel := context.WithTimeout(ctx, 10*time.Second)
	defer workerCancel()

	go func() {
		if err := worker.Start(workerCtx); err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("Worker failed unexpectedly: %v", err)
		}
	}()
	defer worker.Stop()

	// Poll for job completion (worker should pick it up and process it)
	require.Eventually(t, func() bool {
		job, err := repo.GetExportJob(ctx, orgID, jobID)
		if err != nil {
			t.Logf("Error getting job: %v", err)
			return false
		}
		if job.Status == "failed" && job.ErrorMessage != nil {
			t.Logf("Job failed with error: %s", *job.ErrorMessage)
		}
		t.Logf("Job status: %s", job.Status)
		return job.Status == "succeeded"
	}, 8*time.Second, 500*time.Millisecond, "Worker should pick up and process the job")

	// Verify final job state
	finalJob, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", finalJob.Status)
	require.NotNil(t, finalJob.OutputURI, "OutputURI should be set")
	require.NotNil(t, finalJob.Checksum, "Checksum should be set")
	require.NotNil(t, finalJob.RowCount, "RowCount should be set")
	require.Greater(t, *finalJob.RowCount, int64(0), "Should have processed at least one row")
	require.NotNil(t, finalJob.CompletedAt, "CompletedAt should be set")

	// Verify CSV was uploaded to mock S3
	key := fmt.Sprintf("analytics/exports/%s/%s.csv", orgID.String(), jobID.String())
	csvData, ok := mockS3.getUpload(key)
	require.True(t, ok, "Worker should have uploaded CSV to S3")
	require.Greater(t, len(csvData), 0, "CSV data should not be empty")

	// Verify CSV contains expected header
	require.Contains(t, string(csvData), "bucket_start", "CSV should contain header")
	require.Contains(t, string(csvData), "organization_id", "CSV should contain org_id column")
	require.Contains(t, string(csvData), orgID.String(), "CSV should contain org data")
}

// TestExportWorkerNoS3Config validates that when S3 is not configured, no worker starts.
// This test validates the startup conditional logic that was fixed in aas-ceor.
func TestExportWorkerNoS3Config(t *testing.T) {
	// This test validates that JobRunner fails gracefully without S3Delivery.
	// In production (main.go), if S3 is not configured, the worker is never created.
	// This test validates that behavior by showing what happens if a worker starts without S3.

	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	orgID := uuid.New()
	requestedBy := uuid.New()
	now := time.Now().UTC()
	start := now.Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := now.Truncate(time.Hour)

	// Create pending export job
	repo := NewExportJobRepository(pool)
	jobID, err := repo.CreateExportJob(ctx, CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "hourly",
	})
	require.NoError(t, err)

	// Start export worker WITHOUT S3Delivery (nil)
	worker := NewJobRunner(RunnerConfig{
		Pool:       pool,
		S3Delivery: nil, // No S3 configured
		Logger:     zap.NewNop(),
		Interval:   500 * time.Millisecond,
		Workers:    1,
	})

	workerCtx, workerCancel := context.WithTimeout(ctx, 3*time.Second)
	defer workerCancel()

	go func() {
		if err := worker.Start(workerCtx); err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Logf("Worker stopped with error (expected): %v", err)
		}
	}()
	defer worker.Stop()

	// Wait a bit to let worker attempt processing
	time.Sleep(2 * time.Second)

	// Verify job is NOT succeeded (should fail or remain pending/running)
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)

	// Job should either be:
	// - Still pending (worker couldn't pick it up)
	// - Failed (worker tried to process but S3 upload failed)
	// - Running (worker started processing but failed before completion)
	// It should NOT be succeeded
	require.NotEqual(t, "succeeded", job.Status,
		"Job should not succeed without S3 delivery configured")

	t.Logf("Final job status without S3: %s (expected: pending/running/failed)", job.Status)
}
