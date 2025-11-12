// Package integration provides end-to-end integration tests for export reconciliation.
//
// Purpose:
//   This package validates that export CSV totals reconcile with rollup aggregates
//   within acceptable tolerance (1%), ensuring finance stakeholders receive accurate data.
//
package integration

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/exports"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
	"github.com/otherjamesbrown/ai-aas/shared/go/observability"
)

// setupTestDB creates a test Postgres container with TimescaleDB and runs migrations.
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	ctx := context.Background()

	// Use TimescaleDB image
	container, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithImage("timescale/timescaledb:latest-pg15"),
		tcpostgres.WithDatabase("analytics_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
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

// mockS3Delivery is an in-memory S3 delivery adapter for testing.
type mockS3Delivery struct {
	uploads map[string][]byte // key -> CSV data
}

func newMockS3Delivery() *mockS3Delivery {
	return &mockS3Delivery{
		uploads: make(map[string][]byte),
	}
}

func (m *mockS3Delivery) UploadCSV(ctx context.Context, orgID, jobID uuid.UUID, csvData []byte) (string, string, error) {
	key := fmt.Sprintf("%s/%s", orgID.String(), jobID.String())
	m.uploads[key] = csvData
	// Return a mock signed URL
	signedURL := fmt.Sprintf("file://test-exports/%s", key)
	checksum := "mock-checksum" // In real implementation, this would be SHA-256
	return signedURL, checksum, nil
}

func (m *mockS3Delivery) GenerateSignedURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("file://test-exports/%s", key), nil
}

func (m *mockS3Delivery) GetCSV(key string) ([]byte, bool) {
	data, ok := m.uploads[key]
	return data, ok
}

// seedRollupData inserts rollup data directly for testing.
func seedRollupData(t *testing.T, pool *postgres.Store, orgID uuid.UUID, modelID uuid.UUID, granularity string, start, end time.Time) {
	t.Helper()

	ctx := context.Background()
	var tableName string
	var bucketFormat string

	switch granularity {
	case "hourly":
		tableName = "analytics_hourly_rollups"
		bucketFormat = "date_trunc('hour', $1::timestamptz)"
	case "daily":
		tableName = "analytics_daily_rollups"
		bucketFormat = "date_trunc('day', $1::timestamptz)::date"
	default:
		t.Fatalf("unsupported granularity: %s", granularity)
	}

	// Insert test rollup data
	// Create buckets for each hour/day in the range
	current := start
	requestCount := int64(100)
	tokensTotal := int64(10000)
	errorCount := int64(5)
	costTotal := 10.50

	for current.Before(end) {
		var bucketStart interface{}
		if granularity == "hourly" {
			bucketStart = current.Truncate(time.Hour)
		} else {
			bucketStart = current.Truncate(24 * time.Hour).Format("2006-01-02")
		}

		query := fmt.Sprintf(`
			INSERT INTO %s (
				bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (bucket_start, organization_id, model_id)
			DO UPDATE SET
				request_count = EXCLUDED.request_count,
				tokens_total = EXCLUDED.tokens_total,
				error_count = EXCLUDED.error_count,
				cost_total = EXCLUDED.cost_total,
				updated_at = NOW()
		`, tableName)

		_, err := pool.Pool().Exec(ctx, query, bucketStart, orgID, modelID, requestCount, tokensTotal, errorCount, costTotal)
		require.NoError(t, err)

		// Increment for next bucket
		requestCount += 10
		tokensTotal += 1000
		errorCount += 1
		costTotal += 1.25

		if granularity == "hourly" {
			current = current.Add(1 * time.Hour)
		} else {
			current = current.Add(24 * time.Hour)
		}
	}
}

// parseCSVTotals parses CSV and calculates totals.
func parseCSVTotals(csvData []byte) (requestCount, tokensTotal, errorCount int64, costTotal float64, err error) {
	reader := csv.NewReader(strings.NewReader(string(csvData)))
	records, err := reader.ReadAll()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	if len(records) < 2 {
		return 0, 0, 0, 0, fmt.Errorf("CSV has no data rows")
	}

	// Skip header row
	for i := 1; i < len(records); i++ {
		row := records[i]
		if len(row) < 7 {
			continue
		}

		// Parse row: bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total
		reqCount, _ := strconv.ParseInt(row[3], 10, 64)
		tokens, _ := strconv.ParseInt(row[4], 10, 64)
		errors, _ := strconv.ParseInt(row[5], 10, 64)
		cost, _ := strconv.ParseFloat(row[6], 64)

		requestCount += reqCount
		tokensTotal += tokens
		errorCount += errors
		costTotal += cost
	}

	return requestCount, tokensTotal, errorCount, costTotal, nil
}

// queryRollupTotals queries rollup tables directly for comparison.
func queryRollupTotals(ctx context.Context, pool *postgres.Store, orgID uuid.UUID, granularity string, start, end time.Time) (requestCount, tokensTotal, errorCount int64, costTotal float64, err error) {
	var query string
	var args []interface{}

	switch granularity {
	case "hourly":
		query = `
			SELECT 
				COALESCE(SUM(request_count), 0),
				COALESCE(SUM(tokens_total), 0),
				COALESCE(SUM(error_count), 0),
				COALESCE(SUM(cost_total), 0)
			FROM analytics_hourly_rollups
			WHERE organization_id = $1
				AND bucket_start >= $2
				AND bucket_start < $3
		`
		args = []interface{}{orgID, start, end}
	case "daily":
		query = `
			SELECT 
				COALESCE(SUM(request_count), 0),
				COALESCE(SUM(tokens_total), 0),
				COALESCE(SUM(error_count), 0),
				COALESCE(SUM(cost_total), 0)
			FROM analytics_daily_rollups
			WHERE organization_id = $1
				AND bucket_start >= $2::date
				AND bucket_start < $3::date
		`
		args = []interface{}{orgID, start, end}
	case "monthly":
		query = `
			SELECT 
				COALESCE(SUM(request_count), 0),
				COALESCE(SUM(tokens_total), 0),
				COALESCE(SUM(error_count), 0),
				COALESCE(SUM(cost_total), 0)
			FROM analytics_daily_rollups
			WHERE organization_id = $1
				AND bucket_start >= date_trunc('month', $2::date)
				AND bucket_start < date_trunc('month', $3::date) + INTERVAL '1 month'
		`
		args = []interface{}{orgID, start, end}
	default:
		return 0, 0, 0, 0, fmt.Errorf("unsupported granularity: %s", granularity)
	}

	err = pool.Pool().QueryRow(ctx, query, args...).Scan(&requestCount, &tokensTotal, &errorCount, &costTotal)
	return requestCount, tokensTotal, errorCount, costTotal, err
}

// ExportJobResponse matches the API response structure
type ExportJobResponse struct {
	JobID       string          `json:"jobId"`
	OrgID       string          `json:"orgId"`
	Status      string          `json:"status"`
	Granularity string          `json:"granularity"`
	TimeRange   TimeRangeResponse `json:"timeRange"`
	CreatedAt   string          `json:"createdAt"`
	CompletedAt *string          `json:"completedAt,omitempty"`
	OutputURI   *string          `json:"outputUri,omitempty"`
	Checksum    *string          `json:"checksum,omitempty"`
	RowCount    *int64          `json:"rowCount,omitempty"`
	InitiatedBy string          `json:"initiatedBy"`
	Error       *string         `json:"error,omitempty"`
}

type TimeRangeResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// TestExportJobCreation validates that export jobs can be created via API.
func TestExportJobCreation(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	start := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := time.Now().UTC().Truncate(time.Hour)

	// Create export job directly via repository (testing the repository layer)
	repo := exports.NewExportJobRepository(pool)
	
	jobID, err := repo.CreateExportJob(ctx, exports.CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    uuid.New(),
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, jobID)

	// Verify job was created
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, orgID, job.OrgID)
	require.Equal(t, "pending", job.Status)
	require.Equal(t, "daily", job.Granularity)
}

// TestCSVReconciliation validates that CSV totals reconcile with rollup queries.
func TestCSVReconciliation(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()
	start := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Hour)
	end := time.Now().UTC().Truncate(time.Hour)

	// Seed rollup data
	seedRollupData(t, store, orgID, modelID, "daily", start, end)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create mock S3 delivery
	mockS3 := newMockS3Delivery()

	// Create export job repository
	repo := exports.NewExportJobRepository(pool)

	// Create export job
	jobID, err := repo.CreateExportJob(ctx, exports.CreateExportJobRequest{
		OrgID:          orgID,
		RequestedBy:    uuid.New(),
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Granularity:    "daily",
	})
	require.NoError(t, err)

	// Process job manually (simulating worker)
	job, err := repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)

	// Update status to running
	err = repo.UpdateExportJobStatus(ctx, jobID, "running")
	require.NoError(t, err)

	// Process job using job runner (this will generate CSV and upload to S3)
	jobRunner := exports.NewJobRunner(exports.RunnerConfig{
		Pool:       pool,
		S3Delivery: mockS3,
		Logger:     obs.Logger,
		Interval:   30 * time.Second,
		Workers:    1,
	})

	// Process the job (generates CSV, uploads, updates status)
	err = jobRunner.ProcessJob(ctx, *job)
	require.NoError(t, err)

	// Get updated job to retrieve output URI
	job, err = repo.GetExportJob(ctx, orgID, jobID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", job.Status)
	require.NotNil(t, job.OutputURI)

	// Retrieve CSV from mock S3
	key := fmt.Sprintf("%s/%s", orgID.String(), jobID.String())
	csvData, ok := mockS3.GetCSV(key)
	require.True(t, ok, "CSV should be uploaded to mock S3")

	// Parse CSV totals
	csvReqCount, csvTokens, csvErrors, csvCost, err := parseCSVTotals(csvData)
	require.NoError(t, err)

	// Query rollup totals directly
	rollupReqCount, rollupTokens, rollupErrors, rollupCost, err := queryRollupTotals(ctx, store, orgID, "daily", start, end)
	require.NoError(t, err)

	// Verify reconciliation within 1% tolerance
	tolerance := 0.01 // 1%

	require.InDelta(t, float64(rollupReqCount), float64(csvReqCount), float64(rollupReqCount)*tolerance,
		"request_count should reconcile within 1%%")
	require.InDelta(t, float64(rollupTokens), float64(csvTokens), float64(rollupTokens)*tolerance,
		"tokens_total should reconcile within 1%%")
	require.InDelta(t, float64(rollupErrors), float64(csvErrors), float64(rollupErrors)*tolerance,
		"error_count should reconcile within 1%%")
	require.InDelta(t, rollupCost, csvCost, rollupCost*tolerance,
		"cost_total should reconcile within 1%%")
}

// TestExportGranularities validates all granularities (hourly, daily, monthly).
func TestExportGranularities(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Test each granularity
	granularities := []string{"hourly", "daily", "monthly"}

	for _, granularity := range granularities {
		t.Run(granularity, func(t *testing.T) {
			var start, end time.Time
			if granularity == "monthly" {
				start = time.Now().UTC().AddDate(0, -3, 0).Truncate(24 * time.Hour)
				end = time.Now().UTC().Truncate(24 * time.Hour)
			} else {
				start = time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Hour)
				end = time.Now().UTC().Truncate(time.Hour)
			}

			// Seed rollup data
			if granularity != "monthly" {
				seedRollupData(t, store, orgID, modelID, granularity, start, end)
			} else {
				// For monthly, seed daily rollups
				seedRollupData(t, store, orgID, modelID, "daily", start, end)
			}

			// Create export job
			repo := exports.NewExportJobRepository(pool)
			jobID, err := repo.CreateExportJob(ctx, exports.CreateExportJobRequest{
				OrgID:          orgID,
				RequestedBy:    uuid.New(),
				TimeRangeStart: start,
				TimeRangeEnd:   end,
				Granularity:    granularity,
			})
			require.NoError(t, err)

			// Get job
			job, err := repo.GetExportJob(ctx, orgID, jobID)
			require.NoError(t, err)

			// Generate CSV
			obsCfg := observability.Config{
				ServiceName: "analytics-service-test",
				Environment: "test",
			}
			obs, err := observability.Init(ctx, obsCfg)
			require.NoError(t, err)
			defer obs.Shutdown(ctx)

			mockS3 := newMockS3Delivery()
			jobRunner := exports.NewJobRunner(exports.RunnerConfig{
				Pool:       pool,
				S3Delivery: mockS3,
				Logger:     obs.Logger,
				Interval:   30 * time.Second,
				Workers:    1,
			})

			// Process the job
			err = jobRunner.ProcessJob(ctx, *job)
			require.NoError(t, err)

			// Get updated job
			job, err = repo.GetExportJob(ctx, orgID, jobID)
			require.NoError(t, err)
			require.Equal(t, "succeeded", job.Status)

			// Retrieve CSV from mock S3
			key := fmt.Sprintf("%s/%s", orgID.String(), jobID.String())
			csvData, ok := mockS3.GetCSV(key)
			require.True(t, ok, "CSV should be uploaded for %s", granularity)
			require.Greater(t, len(csvData), 0, "CSV should have content for %s", granularity)

			// Verify CSV can be parsed
			_, _, _, _, err = parseCSVTotals(csvData)
			require.NoError(t, err, "CSV should be parseable for %s", granularity)
		})
	}
}

