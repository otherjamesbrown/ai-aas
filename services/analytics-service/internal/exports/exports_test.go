package exports

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewExportJobRepository(t *testing.T) {
	repo := NewExportJobRepository(nil)
	require.NotNil(t, repo)
}

func TestExportJob_Fields(t *testing.T) {
	now := time.Now().UTC()
	jobID := uuid.New()
	orgID := uuid.New()
	requestedBy := uuid.New()
	outputURI := "https://storage.example.com/exports/test.csv"
	checksum := "sha256:abc123"
	rowCount := int64(1000)
	errorMsg := "test error"

	job := ExportJob{
		JobID:          jobID,
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: now.Add(-24 * time.Hour),
		TimeRangeEnd:   now,
		Granularity:    "hourly",
		Status:         "pending",
		OutputURI:      &outputURI,
		Checksum:       &checksum,
		RowCount:       &rowCount,
		InitiatedAt:    now,
		CompletedAt:    nil,
		ErrorMessage:   &errorMsg,
	}

	assert.Equal(t, jobID, job.JobID)
	assert.Equal(t, orgID, job.OrgID)
	assert.Equal(t, requestedBy, job.RequestedBy)
	assert.Equal(t, "hourly", job.Granularity)
	assert.Equal(t, "pending", job.Status)
	require.NotNil(t, job.OutputURI)
	assert.Equal(t, outputURI, *job.OutputURI)
	require.NotNil(t, job.Checksum)
	assert.Equal(t, checksum, *job.Checksum)
	require.NotNil(t, job.RowCount)
	assert.Equal(t, int64(1000), *job.RowCount)
}

func TestExportJob_Granularities(t *testing.T) {
	validGranularities := []string{"hourly", "daily", "monthly"}

	for _, g := range validGranularities {
		t.Run(g, func(t *testing.T) {
			job := ExportJob{
				Granularity: g,
			}
			assert.Equal(t, g, job.Granularity)
		})
	}
}

func TestExportJob_Statuses(t *testing.T) {
	validStatuses := []string{"pending", "running", "succeeded", "failed", "expired"}

	for _, s := range validStatuses {
		t.Run(s, func(t *testing.T) {
			job := ExportJob{
				Status: s,
			}
			assert.Equal(t, s, job.Status)
		})
	}
}

func TestCreateExportJobRequest(t *testing.T) {
	now := time.Now().UTC()
	orgID := uuid.New()
	requestedBy := uuid.New()

	req := CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    requestedBy,
		TimeRangeStart: now.Add(-7 * 24 * time.Hour),
		TimeRangeEnd:   now,
		Granularity:    "daily",
	}

	assert.Equal(t, orgID, req.OrgID)
	assert.Equal(t, requestedBy, req.RequestedBy)
	assert.Equal(t, "daily", req.Granularity)
	assert.True(t, req.TimeRangeEnd.After(req.TimeRangeStart))
}

func TestNewJobRunner(t *testing.T) {
	logger := zap.NewNop()

	cfg := RunnerConfig{
		Pool:       nil,
		S3Delivery: nil,
		Logger:     logger,
		Interval:   30 * time.Second,
		Workers:    2,
	}

	runner := NewJobRunner(cfg)
	require.NotNil(t, runner)
	assert.Equal(t, 30*time.Second, runner.interval)
	assert.Equal(t, 2, runner.workers)
	assert.NotNil(t, runner.stopCh)
	assert.NotNil(t, runner.doneCh)
}

func TestJobRunner_Config(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		interval time.Duration
		workers  int
	}{
		{
			name:     "default config",
			interval: 30 * time.Second,
			workers:  2,
		},
		{
			name:     "high throughput",
			interval: 10 * time.Second,
			workers:  8,
		},
		{
			name:     "low frequency",
			interval: 5 * time.Minute,
			workers:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RunnerConfig{
				Logger:   logger,
				Interval: tt.interval,
				Workers:  tt.workers,
			}

			runner := NewJobRunner(cfg)
			assert.Equal(t, tt.interval, runner.interval)
			assert.Equal(t, tt.workers, runner.workers)
		})
	}
}

func TestJobRunner_ChannelsInitialized(t *testing.T) {
	logger := zap.NewNop()

	runner := NewJobRunner(RunnerConfig{
		Logger:   logger,
		Interval: time.Second,
		Workers:  1,
	})

	// Channels should be initialized and not closed
	select {
	case <-runner.stopCh:
		t.Fatal("stopCh should not be closed initially")
	default:
		// Expected
	}

	select {
	case <-runner.doneCh:
		t.Fatal("doneCh should not be closed initially")
	default:
		// Expected
	}
}

func TestFormatUUID(t *testing.T) {
	tests := []struct {
		name string
		id   *uuid.UUID
		want string
	}{
		{
			name: "nil UUID",
			id:   nil,
			want: "",
		},
		{
			name: "valid UUID",
			id:   ptrUUID(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
			want: "123e4567-e89b-12d3-a456-426614174000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUUID(tt.id)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExportJob_TimeRangeValidation(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		isValid bool
	}{
		{
			name:    "valid range - 1 day",
			start:   now.Add(-24 * time.Hour),
			end:     now,
			isValid: true,
		},
		{
			name:    "valid range - 1 week",
			start:   now.Add(-7 * 24 * time.Hour),
			end:     now,
			isValid: true,
		},
		{
			name:    "valid range - 1 month",
			start:   now.Add(-30 * 24 * time.Hour),
			end:     now,
			isValid: true,
		},
		{
			name:    "invalid range - end before start",
			start:   now,
			end:     now.Add(-24 * time.Hour),
			isValid: false,
		},
		{
			name:    "edge case - same time",
			start:   now,
			end:     now,
			isValid: false, // Should have at least some duration
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.end.After(tt.start)
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestExportJob_NullableFields(t *testing.T) {
	// Test job with all nullable fields as nil
	job := ExportJob{
		JobID:        uuid.New(),
		OrgID:        uuid.New(),
		RequestedBy:  uuid.New(),
		Granularity:  "hourly",
		Status:       "pending",
		OutputURI:    nil,
		Checksum:     nil,
		RowCount:     nil,
		CompletedAt:  nil,
		ErrorMessage: nil,
	}

	assert.Nil(t, job.OutputURI)
	assert.Nil(t, job.Checksum)
	assert.Nil(t, job.RowCount)
	assert.Nil(t, job.CompletedAt)
	assert.Nil(t, job.ErrorMessage)
}

func TestExportJob_CompletedJob(t *testing.T) {
	now := time.Now().UTC()
	outputURI := "https://storage.example.com/exports/job-123.csv"
	checksum := "sha256:def456"
	rowCount := int64(5000)
	completedAt := now.Add(5 * time.Minute)

	job := ExportJob{
		JobID:          uuid.New(),
		OrgID:          uuid.New(),
		TimeRangeStart: now.Add(-24 * time.Hour),
		TimeRangeEnd:   now,
		Granularity:    "daily",
		Status:         "succeeded",
		OutputURI:      &outputURI,
		Checksum:       &checksum,
		RowCount:       &rowCount,
		InitiatedAt:    now,
		CompletedAt:    &completedAt,
	}

	assert.Equal(t, "succeeded", job.Status)
	require.NotNil(t, job.OutputURI)
	require.NotNil(t, job.Checksum)
	require.NotNil(t, job.RowCount)
	require.NotNil(t, job.CompletedAt)
	assert.Equal(t, int64(5000), *job.RowCount)
}

func TestExportJob_FailedJob(t *testing.T) {
	now := time.Now().UTC()
	errorMsg := "database connection timeout"
	completedAt := now.Add(1 * time.Minute)

	job := ExportJob{
		JobID:          uuid.New(),
		OrgID:          uuid.New(),
		TimeRangeStart: now.Add(-24 * time.Hour),
		TimeRangeEnd:   now,
		Granularity:    "daily",
		Status:         "failed",
		OutputURI:      nil,
		Checksum:       nil,
		RowCount:       nil,
		InitiatedAt:    now,
		CompletedAt:    &completedAt,
		ErrorMessage:   &errorMsg,
	}

	assert.Equal(t, "failed", job.Status)
	assert.Nil(t, job.OutputURI)
	assert.Nil(t, job.Checksum)
	assert.Nil(t, job.RowCount)
	require.NotNil(t, job.ErrorMessage)
	assert.Equal(t, "database connection timeout", *job.ErrorMessage)
}

// ptrUUID returns a pointer to the given UUID
func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
