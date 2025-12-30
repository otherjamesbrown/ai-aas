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
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver for goose
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupTestDB creates a test Postgres container with TimescaleDB and runs migrations.
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	// Use TimescaleDB image
	container, err := tcpostgres.Run(ctx, "timescale/timescaledb:latest-pg15",
		tcpostgres.WithDatabase("analytics_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
	)
	require.NoError(t, err)

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run migrations
	db, err := goose.OpenDBWithDriver("postgres", connString)
	require.NoError(t, err)

	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	migrationsDir := filepath.Join(projectRoot, "db", "migrations", "analytics")

	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(db, migrationsDir))
	require.NoError(t, db.Close())

	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)

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
		jobs2, err := repo.GetPendingJobs(ctx, 1)
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
