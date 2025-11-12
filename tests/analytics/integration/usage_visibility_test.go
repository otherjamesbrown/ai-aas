// Package integration provides end-to-end integration tests for analytics service.
//
// Purpose:
//   This package validates that the analytics service correctly ingests events,
//   aggregates them into rollups, and serves usage/spend data via the API with
//   proper freshness indicators.
//
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/freshness"
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

// setupTestRedis creates a test Redis container.
func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()

	ctx := context.Background()

	container, err := tcredis.RunContainer(ctx,
		tcredis.WithImage("redis:7-alpine"),
		tcredis.WithWaitStrategy(wait.ForListeningPort("6379/tcp")),
	)
	require.NoError(t, err)

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: endpoint,
	})

	// Verify connection
	_, err = client.Ping(ctx).Result()
	require.NoError(t, err)

	cleanup := func() {
		client.Close()
		require.NoError(t, container.Terminate(ctx))
	}

	return client, cleanup
}

// seedTestData inserts sample usage events for testing.
func seedTestData(t *testing.T, store *postgres.Store, orgID uuid.UUID, modelID uuid.UUID, count int) {
	t.Helper()

	ctx := context.Background()
	now := time.Now().UTC()

	events := make([]postgres.UsageEvent, count)
	for i := 0; i < count; i++ {
		events[i] = postgres.UsageEvent{
			EventID:           uuid.New(),
			OrgID:             orgID,
			OccurredAt:        now.Add(-time.Duration(i) * time.Minute),
			ReceivedAt:        now,
			ModelID:           modelID,
			ActorID:           uuid.Nil,
			InputTokens:       100 + int64(i*10),
			OutputTokens:      50 + int64(i*5),
			LatencyMS:         100 + i,
			Status:            "success",
			ErrorCode:         "",
			CostEstimateCents: float64(i) * 0.01,
			Metadata:          map[string]interface{}{},
		}
	}

	batchID, err := store.CreateIngestionBatch(ctx, 0, []uuid.UUID{orgID})
	require.NoError(t, err)

	inserted, err := store.InsertUsageEvents(ctx, events, batchID)
	require.NoError(t, err)
	require.Equal(t, count, inserted)

	err = store.CompleteIngestionBatch(ctx, batchID, 0)
	require.NoError(t, err)
}

// TestUsageVisibility validates that usage aggregates are correctly calculated and returned.
func TestUsageVisibility(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	// Create test organization and model IDs
	orgID := uuid.New()
	modelID := uuid.New()

	// Seed test data
	seedTestData(t, store, orgID, modelID, 10)

	// Run rollup manually to populate aggregates
	// (In production, rollup worker would do this)
	now := time.Now().UTC()
	hourEnd := now.Truncate(time.Hour)
	hourStart := hourEnd.Add(-1 * time.Hour)

	_, err = pool.Exec(ctx, `
		INSERT INTO analytics_hourly_rollups (
			bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at
		)
		SELECT
			date_trunc('hour', occurred_at) AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`, hourStart, hourEnd)
	require.NoError(t, err)

	// Initialize observability (minimal for testing)
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create freshness cache (can be nil for this test)
	var cache *freshness.Cache

	// Create usage handler
	handler := api.NewUsageHandler(store, obs.Logger, cache)

	// Test: Query usage for the test org
	start := hourStart.Add(-24 * time.Hour)
	end := hourEnd.Add(24 * time.Hour)

	req := httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/usage?start=%s&end=%s&granularity=hour",
		orgID.String(),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	), nil)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetOrgUsage(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response api.UsageSeriesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, orgID.String(), response.OrgID)
	require.Equal(t, "hour", response.Granularity)
	require.Greater(t, len(response.Series), 0, "should have at least one data point")

	// Verify totals
	require.Greater(t, response.Totals.Invocations, int64(0), "should have invocations")
	require.Greater(t, response.Totals.CostEstimateCents, int64(0), "should have cost")
}

// TestFreshnessLag validates that freshness indicators are correctly calculated.
func TestFreshnessLag(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Seed recent events
	seedTestData(t, store, orgID, modelID, 5)

	// Update freshness status
	_, err = pool.Exec(ctx, `
		INSERT INTO analytics.freshness_status (
			org_id, model_id, last_event_at, last_rollup_at, lag_seconds, status, updated_at
		)
		SELECT
			org_id,
			model_id,
			MAX(occurred_at) AS last_event_at,
			NOW() AS last_rollup_at,
			EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER AS lag_seconds,
			CASE
				WHEN EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER < 300 THEN 'fresh'
				WHEN EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER < 600 THEN 'stale'
				ELSE 'delayed'
			END AS status,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE org_id = $1 AND model_id = $2
		GROUP BY org_id, model_id
		ON CONFLICT (org_id, model_id)
		DO UPDATE SET
			last_event_at = EXCLUDED.last_event_at,
			last_rollup_at = EXCLUDED.last_rollup_at,
			lag_seconds = EXCLUDED.lag_seconds,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, orgID, modelID)
	require.NoError(t, err)

	// Query freshness status
	indicator, err := store.GetFreshnessStatus(ctx, orgID, &modelID)
	require.NoError(t, err)

	require.Equal(t, orgID, indicator.OrgID)
	require.NotNil(t, indicator.ModelID)
	require.Equal(t, *indicator.ModelID, modelID)
	require.Less(t, indicator.LagSeconds, 300, "lag should be less than 5 minutes for recent events")
	require.Equal(t, "fresh", indicator.Status)
}

// TestOrgIsolation validates that orgs cannot see each other's data.
func TestOrgIsolation(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	org1ID := uuid.New()
	org2ID := uuid.New()
	modelID := uuid.New()

	// Seed data for both orgs
	seedTestData(t, store, org1ID, modelID, 5)
	seedTestData(t, store, org2ID, modelID, 3)

	// Create rollups for both orgs
	now := time.Now().UTC()
	dayEnd := now.Truncate(24 * time.Hour)
	dayStart := dayEnd.Add(-24 * time.Hour)

	// Create daily rollups
	_, err = pool.Exec(ctx, `
		INSERT INTO analytics_daily_rollups (
			bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at
		)
		SELECT
			date_trunc('day', occurred_at)::date AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`, dayStart, dayEnd)
	require.NoError(t, err)

	// Query usage for org1
	points, err := store.GetUsageSeries(ctx, org1ID, dayStart, dayEnd, "day", &modelID)
	require.NoError(t, err)

	// Verify org1 only sees its own data (5 events)
	var totalInvocations int64
	for _, p := range points {
		if p.ModelID != nil && *p.ModelID == modelID {
			totalInvocations += p.Invocations
		}
	}
	require.Equal(t, int64(5), totalInvocations, "org1 should see exactly 5 invocations")

	// Query usage for org2
	points2, err := store.GetUsageSeries(ctx, org2ID, dayStart, dayEnd, "day", &modelID)
	require.NoError(t, err)

	// Verify org2 only sees its own data (3 events)
	var totalInvocations2 int64
	for _, p := range points2 {
		if p.ModelID != nil && *p.ModelID == modelID {
			totalInvocations2 += p.Invocations
		}
	}
	require.Equal(t, int64(3), totalInvocations2, "org2 should see exactly 3 invocations")
}

// TestDailyGranularity validates that daily rollups are correctly returned.
func TestDailyGranularity(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Seed test data across multiple days
	now := time.Now().UTC()
	events := make([]postgres.UsageEvent, 20)
	for i := 0; i < 20; i++ {
		events[i] = postgres.UsageEvent{
			EventID:           uuid.New(),
			OrgID:             orgID,
			OccurredAt:        now.Add(-time.Duration(i) * 12 * time.Hour), // Spread across days
			ReceivedAt:        now,
			ModelID:           modelID,
			ActorID:           uuid.Nil,
			InputTokens:       100 + int64(i*10),
			OutputTokens:      50 + int64(i*5),
			LatencyMS:         100 + i,
			Status:            "success",
			ErrorCode:         "",
			CostEstimateCents: float64(i) * 0.01,
			Metadata:          map[string]interface{}{},
		}
	}

	batchID, err := store.CreateIngestionBatch(ctx, 0, []uuid.UUID{orgID})
	require.NoError(t, err)

	inserted, err := store.InsertUsageEvents(ctx, events, batchID)
	require.NoError(t, err)
	require.Equal(t, 20, inserted)

	err = store.CompleteIngestionBatch(ctx, batchID, 0)
	require.NoError(t, err)

	// Create daily rollups
	dayEnd := now.Truncate(24 * time.Hour)
	dayStart := dayEnd.Add(-7 * 24 * time.Hour)

	_, err = pool.Exec(ctx, `
		INSERT INTO analytics_daily_rollups (
			bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at
		)
		SELECT
			date_trunc('day', occurred_at)::date AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`, dayStart, dayEnd)
	require.NoError(t, err)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	handler := api.NewUsageHandler(store, obs.Logger, nil)

	// Query with daily granularity
	req := httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/usage?start=%s&end=%s&granularity=day",
		orgID.String(),
		dayStart.Format(time.RFC3339),
		dayEnd.Format(time.RFC3339),
	), nil)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetOrgUsage(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response api.UsageSeriesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, orgID.String(), response.OrgID)
	require.Equal(t, "day", response.Granularity)
	require.Greater(t, len(response.Series), 0, "should have at least one data point")
	require.Equal(t, int64(20), response.Totals.Invocations, "should have 20 total invocations")
}

// TestModelFiltering validates that model filtering works correctly.
func TestModelFiltering(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	model1ID := uuid.New()
	model2ID := uuid.New()

	// Seed data for both models
	seedTestData(t, store, orgID, model1ID, 5)
	seedTestData(t, store, orgID, model2ID, 3)

	// Create rollups
	now := time.Now().UTC()
	hourEnd := now.Truncate(time.Hour)
	hourStart := hourEnd.Add(-1 * time.Hour)

	_, err = pool.Exec(ctx, `
		INSERT INTO analytics_hourly_rollups (
			bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at
		)
		SELECT
			date_trunc('hour', occurred_at) AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`, hourStart, hourEnd)
	require.NoError(t, err)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	handler := api.NewUsageHandler(store, obs.Logger, nil)

	// Query with model1 filter
	start := hourStart.Add(-24 * time.Hour)
	end := hourEnd.Add(24 * time.Hour)

	req := httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/usage?start=%s&end=%s&granularity=hour&modelId=%s",
		orgID.String(),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		model1ID.String(),
	), nil)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetOrgUsage(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response api.UsageSeriesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, orgID.String(), response.OrgID)
	require.Equal(t, int64(5), response.Totals.Invocations, "should have 5 invocations for model1")

	// Verify all series points are for model1
	for _, point := range response.Series {
		require.NotNil(t, point.ModelID, "all points should have model_id")
		require.Equal(t, model1ID.String(), *point.ModelID, "all points should be for model1")
	}
}

// TestFreshnessCache validates that freshness cache is used when available.
func TestFreshnessCache(t *testing.T) {
	pool, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	redisClient, cleanupRedis := setupTestRedis(t)
	defer cleanupRedis()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Seed test data
	seedTestData(t, store, orgID, modelID, 5)

	// Update freshness status in database
	_, err = pool.Exec(ctx, `
		INSERT INTO analytics.freshness_status (
			org_id, model_id, last_event_at, last_rollup_at, lag_seconds, status, updated_at
		)
		SELECT
			org_id,
			model_id,
			MAX(occurred_at) AS last_event_at,
			NOW() AS last_rollup_at,
			EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER AS lag_seconds,
			CASE
				WHEN EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER < 300 THEN 'fresh'
				WHEN EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER < 600 THEN 'stale'
				ELSE 'delayed'
			END AS status,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE org_id = $1 AND model_id = $2
		GROUP BY org_id, model_id
		ON CONFLICT (org_id, model_id)
		DO UPDATE SET
			last_event_at = EXCLUDED.last_event_at,
			last_rollup_at = EXCLUDED.last_rollup_at,
			lag_seconds = EXCLUDED.lag_seconds,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, orgID, modelID)
	require.NoError(t, err)

	// Create freshness cache
	logger := zap.NewNop()
	cache := freshness.NewCache(freshness.Config{
		Client: redisClient,
		Logger: logger,
		TTL:    5 * time.Minute,
	})

	// Get freshness from DB and cache it
	dbFreshness, err := store.GetFreshnessStatus(ctx, orgID, &modelID)
	require.NoError(t, err)
	err = cache.Set(ctx, dbFreshness)
	require.NoError(t, err)

	// Verify cache hit
	cached, err := cache.Get(ctx, orgID, &modelID)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.Equal(t, dbFreshness.Status, cached.Status)
	require.Equal(t, dbFreshness.LagSeconds, cached.LagSeconds)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create rollups for API test
	now := time.Now().UTC()
	dayEnd := now.Truncate(24 * time.Hour)
	dayStart := dayEnd.Add(-24 * time.Hour)

	_, err = pool.Exec(ctx, `
		INSERT INTO analytics_daily_rollups (
			bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at
		)
		SELECT
			date_trunc('day', occurred_at)::date AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`, dayStart, dayEnd)
	require.NoError(t, err)

	handler := api.NewUsageHandler(store, obs.Logger, cache)

	// Query usage API - should use cached freshness
	req := httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/usage?start=%s&end=%s&granularity=day",
		orgID.String(),
		dayStart.Format(time.RFC3339),
		dayEnd.Format(time.RFC3339),
	), nil)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetOrgUsage(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response api.UsageSeriesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, orgID.String(), response.OrgID)
	require.Equal(t, "fresh", response.Freshness.Status, "freshness status should be fresh")
	require.Less(t, response.Freshness.LagSeconds, 300, "lag should be less than 5 minutes")
}

// TestAPIErrorHandling validates error handling in the API.
func TestAPIErrorHandling(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	handler := api.NewUsageHandler(store, obs.Logger, nil)

	// Test: Invalid org ID
	req := httptest.NewRequest("GET", "/analytics/v1/orgs/invalid-uuid/usage?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.GetOrgUsage(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Missing start parameter
	orgID := uuid.New()
	req = httptest.NewRequest("GET", fmt.Sprintf("/analytics/v1/orgs/%s/usage?end=2024-01-02T00:00:00Z", orgID.String()), nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	handler.GetOrgUsage(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Invalid granularity
	req = httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/usage?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&granularity=invalid",
		orgID.String(),
	), nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	handler.GetOrgUsage(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Test: End before start
	req = httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/usage?start=2024-01-02T00:00:00Z&end=2024-01-01T00:00:00Z",
		orgID.String(),
	), nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	handler.GetOrgUsage(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

