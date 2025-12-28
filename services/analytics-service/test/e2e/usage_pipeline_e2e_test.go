// Package e2e provides end-to-end integration tests for the token usage pipeline.
//
// Purpose:
//   This package validates the complete usage tracking pipeline from ingestion to query:
//   1. UsageRecord published to Kafka (simulating api-router)
//   2. analytics-service Kafka consumer ingests the record
//   3. Record is stored in TimescaleDB usage_events table
//   4. Rollup workers aggregate data into hourly/daily rollups
//   5. Data can be queried via REST API
//
// Test Strategy:
//   - Use testcontainers for Kafka and TimescaleDB
//   - Run real analytics-service components (consumer, rollup workers, API)
//   - Verify end-to-end data flow with realistic scenarios
//   - Ensure idempotency and deduplication work correctly
//
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver for goose
	"github.com/pressly/goose/v3"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/aggregation"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/ingestion"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/observability"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
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

	// Find migrations directory by walking up from test location
	_, filename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filename)

	// Walk up to find go.work (project root marker)
	projectRoot := testDir
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.work")); err == nil {
			break
		}
		projectRoot = filepath.Dir(projectRoot)
	}

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

// setupTestKafka creates a test Kafka container.
func setupTestKafka(t *testing.T) ([]string, func()) {
	t.Helper()

	ctx := context.Background()

	// Use Kafka container
	container, err := tckafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err)

	brokers, err := container.Brokers(ctx)
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, container.Terminate(ctx))
	}

	return brokers, cleanup
}

// publishUsageRecord publishes a UsageRecord to Kafka topic.
func publishUsageRecord(t *testing.T, brokers []string, topic string, record UsageRecord) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	recordJSON, err := json.Marshal(record)
	require.NoError(t, err)

	err = writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(record.RecordID),
		Value: recordJSON,
	})
	require.NoError(t, err)
}

// UsageRecord matches the schema from api-router-service.
type UsageRecord struct {
	RecordID       string                 `json:"record_id"`
	RequestID      string                 `json:"request_id"`
	OrganizationID string                 `json:"organization_id"`
	APIKeyID       string                 `json:"api_key_id"`
	Timestamp      time.Time              `json:"timestamp"`
	Model          string                 `json:"model"`
	BackendID      string                 `json:"backend_id"`
	TokensInput    int                    `json:"tokens_input"`
	TokensOutput   int                    `json:"tokens_output"`
	CostUSD        float64                `json:"cost_usd"`
	LatencyMS      int                    `json:"latency_ms"`
	LimitState     string                 `json:"limit_state"`
	DecisionReason string                 `json:"decision_reason"`
	RetryCount     int                    `json:"retry_count,omitempty"`
	TraceID        string                 `json:"trace_id,omitempty"`
	SpanID         string                 `json:"span_id,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// APIKeyUsageSeriesResponse matches the API response structure.
type APIKeyUsageSeriesResponse struct {
	OrgID       string               `json:"orgId"`
	APIKeyID    string               `json:"apiKeyId"`
	Granularity string               `json:"granularity"`
	Series      []UsageDataPoint     `json:"series"`
	Totals      UsageTotalsResponse  `json:"totals"`
	Freshness   FreshnessIndicator   `json:"freshness"`
}

type UsageDataPoint struct {
	Timestamp     string  `json:"timestamp"`
	Invocations   int64   `json:"invocations"`
	InputTokens   int64   `json:"inputTokens"`
	OutputTokens  int64   `json:"outputTokens"`
	TotalTokens   int64   `json:"totalTokens"`
	AverageCostUSD float64 `json:"averageCostUsd"`
	ErrorCount    int64   `json:"errorCount"`
}

type UsageTotalsResponse struct {
	Invocations       int64   `json:"invocations"`
	InputTokens       int64   `json:"inputTokens"`
	OutputTokens      int64   `json:"outputTokens"`
	TotalTokens       int64   `json:"totalTokens"`
	TotalCostUSD      float64 `json:"totalCostUsd"`
	AverageCostUSD    float64 `json:"averageCostUsd"`
	AverageLatencyMS  float64 `json:"averageLatencyMs"`
	ErrorCount        int64   `json:"errorCount"`
}

type FreshnessIndicator struct {
	Status       string    `json:"status"`
	LagSeconds   int       `json:"lagSeconds"`
	LastEventAt  time.Time `json:"lastEventAt"`
	LastRollupAt time.Time `json:"lastRollupAt"`
}

// TestUsagePipelineE2E validates the complete usage tracking pipeline.
func TestUsagePipelineE2E(t *testing.T) {
	// Setup test infrastructure
	pool, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	brokers, cleanupKafka := setupTestKafka(t)
	defer cleanupKafka()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-e2e-test",
		Environment: "test",
		Endpoint:    "http://localhost:4318",
		Protocol:    "http",
		LogLevel:    "info",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create Kafka topic
	topic := "usage.records.v1"
	conn, err := kafka.Dial("tcp", brokers[0])
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	require.NoError(t, err)
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)

	// Start Kafka consumer
	consumer, err := ingestion.NewConsumer(ingestion.Config{
		Brokers:      brokers,
		Topic:        topic,
		GroupID:      "analytics-e2e-test",
		BatchSize:    10,
		Workers:      1,
		BatchTimeout: 1 * time.Second,
		Logger:       obs.Logger,
		Store:        store,
	})
	require.NoError(t, err)

	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	err = consumer.Start(consumerCtx)
	require.NoError(t, err)
	defer consumer.Stop(ctx)

	// Start rollup worker
	rollupWorker := aggregation.NewWorker(aggregation.Config{
		Store:    store,
		Interval: 5 * time.Second, // Fast interval for testing
		Workers:  1,
		Logger:   obs.Logger,
	})

	rollupCtx, cancelRollup := context.WithCancel(ctx)
	defer cancelRollup()

	go rollupWorker.Start(rollupCtx)
	defer rollupWorker.Stop()

	// Test data
	orgID := uuid.New()
	apiKeyID := uuid.New()
	recordID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().UTC().Truncate(time.Second)

	// Publish UsageRecord to Kafka
	record := UsageRecord{
		RecordID:       recordID.String(),
		RequestID:      requestID.String(),
		OrganizationID: orgID.String(),
		APIKeyID:       apiKeyID.String(),
		Timestamp:      timestamp,
		Model:          "gpt-4o",
		BackendID:      "backend-test-1",
		TokensInput:    150,
		TokensOutput:   300,
		CostUSD:        0.0045,
		LatencyMS:      250,
		LimitState:     "WITHIN_LIMIT",
		DecisionReason: "PRIMARY",
		TraceID:        "trace-e2e-test",
		SpanID:         "span-e2e-test",
		Metadata: map[string]interface{}{
			"test": "e2e",
		},
	}

	publishUsageRecord(t, brokers, topic, record)

	// Wait for ingestion to process (consumer batches every 5s or when batch size is reached)
	time.Sleep(7 * time.Second)

	// Verify record exists in usage_events table
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM usage_events WHERE record_id = $1 AND organization_id = $2)`
	err = pool.QueryRow(ctx, query, recordID.String(), orgID.String()).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "usage record should be stored in usage_events table")

	// Wait for rollup workers to aggregate data
	time.Sleep(6 * time.Second)

	// Verify hourly rollup exists
	var hourlyExists bool
	hourlyQuery := `SELECT EXISTS(SELECT 1 FROM analytics_hourly_rollups WHERE organization_id = $1)`
	err = pool.QueryRow(ctx, hourlyQuery, orgID.String()).Scan(&hourlyExists)
	require.NoError(t, err)
	require.True(t, hourlyExists, "hourly rollup should exist after aggregation")

	// Verify daily rollup exists
	var dailyExists bool
	dailyQuery := `SELECT EXISTS(SELECT 1 FROM analytics_daily_rollups WHERE organization_id = $1)`
	err = pool.QueryRow(ctx, dailyQuery, orgID.String()).Scan(&dailyExists)
	require.NoError(t, err)
	require.True(t, dailyExists, "daily rollup should exist after aggregation")

	// Setup API server for testing (without freshness cache for simplicity)
	usageHandler := api.NewUsageHandler(store, obs.Logger, nil)

	router := chi.NewRouter()
	router.Get("/analytics/v1/orgs/{orgId}/apikeys/{apiKeyId}/usage", usageHandler.GetAPIKeyUsage)

	server := httptest.NewServer(router)
	defer server.Close()

	// Query usage via API
	start := timestamp.Add(-1 * time.Hour).Format(time.RFC3339)
	end := timestamp.Add(1 * time.Hour).Format(time.RFC3339)
	apiURL := fmt.Sprintf("%s/analytics/v1/orgs/%s/apikeys/%s/usage?start=%s&end=%s&granularity=hour",
		server.URL, orgID.String(), apiKeyID.String(), start, end)

	resp, err := http.Get(apiURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "API should return 200 OK")

	// Parse response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp APIKeyUsageSeriesResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)

	// Verify API response
	require.Equal(t, orgID.String(), apiResp.OrgID)
	require.Equal(t, apiKeyID.String(), apiResp.APIKeyID)
	require.Equal(t, "hour", apiResp.Granularity)

	// Verify totals match the published record
	require.Equal(t, int64(1), apiResp.Totals.Invocations, "should have 1 invocation")
	require.Equal(t, int64(150), apiResp.Totals.InputTokens, "should have 150 input tokens")
	require.Equal(t, int64(300), apiResp.Totals.OutputTokens, "should have 300 output tokens")
	require.Equal(t, int64(450), apiResp.Totals.TotalTokens, "should have 450 total tokens")
	require.InDelta(t, 0.0045, apiResp.Totals.TotalCostUSD, 0.0001, "should have correct total cost")
	require.InDelta(t, 250.0, apiResp.Totals.AverageLatencyMS, 1.0, "should have correct average latency")

	// Verify time series data points
	require.NotEmpty(t, apiResp.Series, "should have at least one data point in time series")

	t.Logf("✅ E2E test passed: UsageRecord → Kafka → Consumer → TimescaleDB → Rollups → API query")
}

// TestUsagePipelineDeduplication validates that duplicate records are deduplicated.
func TestUsagePipelineDeduplication(t *testing.T) {
	// Setup test infrastructure
	pool, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	brokers, cleanupKafka := setupTestKafka(t)
	defer cleanupKafka()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-dedup-test",
		Environment: "test",
		Endpoint:    "http://localhost:4318",
		Protocol:    "http",
		LogLevel:    "info",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create Kafka topic
	topic := "usage.records.dedup.v1"
	conn, err := kafka.Dial("tcp", brokers[0])
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	require.NoError(t, err)
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)

	// Start Kafka consumer
	consumer, err := ingestion.NewConsumer(ingestion.Config{
		Brokers:      brokers,
		Topic:        topic,
		GroupID:      "analytics-dedup-test",
		BatchSize:    10,
		Workers:      1,
		BatchTimeout: 1 * time.Second,
		Logger:       obs.Logger,
		Store:        store,
	})
	require.NoError(t, err)

	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	err = consumer.Start(consumerCtx)
	require.NoError(t, err)
	defer consumer.Stop(ctx)

	// Test data - same record published twice
	orgID := uuid.New()
	apiKeyID := uuid.New()
	recordID := uuid.New()
	requestID := uuid.New()
	timestamp := time.Now().UTC().Truncate(time.Second)

	record := UsageRecord{
		RecordID:       recordID.String(),
		RequestID:      requestID.String(),
		OrganizationID: orgID.String(),
		APIKeyID:       apiKeyID.String(),
		Timestamp:      timestamp,
		Model:          "gpt-4o",
		BackendID:      "backend-test-1",
		TokensInput:    100,
		TokensOutput:   200,
		CostUSD:        0.003,
		LatencyMS:      200,
		LimitState:     "WITHIN_LIMIT",
		DecisionReason: "PRIMARY",
	}

	// Publish the same record twice
	publishUsageRecord(t, brokers, topic, record)
	publishUsageRecord(t, brokers, topic, record)

	// Wait for ingestion
	time.Sleep(7 * time.Second)

	// Verify only one record exists (deduplication by record_id + organization_id)
	var count int
	query := `SELECT COUNT(*) FROM usage_events WHERE record_id = $1 AND organization_id = $2`
	err = pool.QueryRow(ctx, query, recordID.String(), orgID.String()).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "should only have 1 record (duplicates deduplicated)")

	t.Logf("✅ Deduplication test passed: duplicate records were properly deduplicated")
}

// TestUsagePipelineMultipleRecords validates handling of multiple records.
func TestUsagePipelineMultipleRecords(t *testing.T) {
	// Setup test infrastructure
	pool, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	brokers, cleanupKafka := setupTestKafka(t)
	defer cleanupKafka()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-multi-test",
		Environment: "test",
		Endpoint:    "http://localhost:4318",
		Protocol:    "http",
		LogLevel:    "info",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create Kafka topic
	topic := "usage.records.multi.v1"
	conn, err := kafka.Dial("tcp", brokers[0])
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	require.NoError(t, err)
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)

	// Start Kafka consumer
	consumer, err := ingestion.NewConsumer(ingestion.Config{
		Brokers:      brokers,
		Topic:        topic,
		GroupID:      "analytics-multi-test",
		BatchSize:    10,
		Workers:      1,
		BatchTimeout: 1 * time.Second,
		Logger:       obs.Logger,
		Store:        store,
	})
	require.NoError(t, err)

	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	err = consumer.Start(consumerCtx)
	require.NoError(t, err)
	defer consumer.Stop(ctx)

	// Start rollup worker
	rollupWorker := aggregation.NewWorker(aggregation.Config{
		Store:    store,
		Interval: 5 * time.Second,
		Workers:  1,
		Logger:   obs.Logger,
	})

	rollupCtx, cancelRollup := context.WithCancel(ctx)
	defer cancelRollup()

	go rollupWorker.Start(rollupCtx)
	defer rollupWorker.Stop()

	// Test data - publish 10 records for the same org and API key
	orgID := uuid.New()
	apiKeyID := uuid.New()
	timestamp := time.Now().UTC().Truncate(time.Second)

	expectedTotalTokens := int64(0)
	expectedInvocations := int64(10)

	for i := 0; i < 10; i++ {
		recordID := uuid.New()
		requestID := uuid.New()

		inputTokens := 100 + i*10
		outputTokens := 200 + i*20
		expectedTotalTokens += int64(inputTokens + outputTokens)

		record := UsageRecord{
			RecordID:       recordID.String(),
			RequestID:      requestID.String(),
			OrganizationID: orgID.String(),
			APIKeyID:       apiKeyID.String(),
			Timestamp:      timestamp.Add(time.Duration(i) * time.Minute),
			Model:          "gpt-4o",
			BackendID:      "backend-test-1",
			TokensInput:    inputTokens,
			TokensOutput:   outputTokens,
			CostUSD:        0.003 + float64(i)*0.001,
			LatencyMS:      200 + i*10,
			LimitState:     "WITHIN_LIMIT",
			DecisionReason: "PRIMARY",
		}

		publishUsageRecord(t, brokers, topic, record)
	}

	// Wait for ingestion
	time.Sleep(7 * time.Second)

	// Verify all records exist
	var count int
	query := `SELECT COUNT(*) FROM usage_events WHERE organization_id = $1 AND api_key_id = $2`
	err = pool.QueryRow(ctx, query, orgID.String(), apiKeyID.String()).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 10, count, "should have 10 records")

	// Wait for rollup
	time.Sleep(6 * time.Second)

	// Verify rollup aggregates correct totals
	var rollupInvocations, rollupTokensTotal int64
	rollupQuery := `
		SELECT
			COALESCE(SUM(request_count), 0),
			COALESCE(SUM(tokens_total), 0)
		FROM analytics_hourly_rollups
		WHERE organization_id = $1
	`
	err = pool.QueryRow(ctx, rollupQuery, orgID.String()).Scan(&rollupInvocations, &rollupTokensTotal)
	require.NoError(t, err)
	require.Equal(t, expectedInvocations, rollupInvocations, "rollup should have correct invocation count")
	require.Equal(t, expectedTotalTokens, rollupTokensTotal, "rollup should have correct total tokens")

	t.Logf("✅ Multiple records test passed: 10 records ingested and aggregated correctly")
	t.Logf("   Expected invocations: %d, got: %d", expectedInvocations, rollupInvocations)
	t.Logf("   Expected total tokens: %d, got: %d", expectedTotalTokens, rollupTokensTotal)
}

// TestUsagePipelineNullModelID validates the pipeline with NULL model_id.
// This test explicitly verifies that events with NULL model_id (the default case
// when no model registry mapping exists) flow correctly through the entire pipeline:
// 1. Events are ingested with model_id=NULL (processor.go sets modelID to Nil)
// 2. Rollup worker aggregates using COALESCE(model_id, '00000000-...')
// 3. Query endpoint returns correct results despite NULL model_id
func TestUsagePipelineNullModelID(t *testing.T) {
	// Setup test infrastructure
	pool, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	brokers, cleanupKafka := setupTestKafka(t)
	defer cleanupKafka()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-nullmodel-test",
		Environment: "test",
		Endpoint:    "http://localhost:4318",
		Protocol:    "http",
		LogLevel:    "info",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	// Create Kafka topic
	topic := "usage.records.nullmodel.v1"
	conn, err := kafka.Dial("tcp", brokers[0])
	require.NoError(t, err)
	defer conn.Close()

	controller, err := conn.Controller()
	require.NoError(t, err)

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	require.NoError(t, err)
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)

	// Start Kafka consumer
	consumer, err := ingestion.NewConsumer(ingestion.Config{
		Brokers:      brokers,
		Topic:        topic,
		GroupID:      "analytics-nullmodel-test",
		BatchSize:    10,
		Workers:      1,
		BatchTimeout: 1 * time.Second,
		Logger:       obs.Logger,
		Store:        store,
	})
	require.NoError(t, err)

	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	err = consumer.Start(consumerCtx)
	require.NoError(t, err)
	defer consumer.Stop(ctx)

	// Start rollup worker
	rollupWorker := aggregation.NewWorker(aggregation.Config{
		Store:    store,
		Interval: 5 * time.Second,
		Workers:  1,
		Logger:   obs.Logger,
	})

	rollupCtx, cancelRollup := context.WithCancel(ctx)
	defer cancelRollup()

	go rollupWorker.Start(rollupCtx)
	defer rollupWorker.Stop()

	// Test data - events that will result in NULL model_id
	orgID := uuid.New()
	apiKeyID := uuid.New()
	timestamp := time.Now().UTC().Truncate(time.Second)

	// Publish multiple records with model names (string)
	// These will be converted to NULL model_id by processor.go
	expectedTokens := int64(0)
	recordCount := 5

	for i := 0; i < recordCount; i++ {
		recordID := uuid.New()
		requestID := uuid.New()

		inputTokens := 100 + i*10
		outputTokens := 200 + i*20
		expectedTokens += int64(inputTokens + outputTokens)

		record := UsageRecord{
			RecordID:       recordID.String(),
			RequestID:      requestID.String(),
			OrganizationID: orgID.String(),
			APIKeyID:       apiKeyID.String(),
			Timestamp:      timestamp.Add(time.Duration(i) * time.Minute),
			Model:          "gpt-4o", // String model name, NOT a UUID
			BackendID:      "backend-test-1",
			TokensInput:    inputTokens,
			TokensOutput:   outputTokens,
			CostUSD:        0.003 + float64(i)*0.001,
			LatencyMS:      200 + i*10,
			LimitState:     "WITHIN_LIMIT",
			DecisionReason: "PRIMARY",
		}

		publishUsageRecord(t, brokers, topic, record)
	}

	// Wait for ingestion
	time.Sleep(7 * time.Second)

	// Verify events were inserted with NULL model_id
	var nullModelCount int
	query := `SELECT COUNT(*) FROM analytics.usage_events WHERE org_id = $1 AND model_id IS NULL`
	err = pool.QueryRow(ctx, query, orgID).Scan(&nullModelCount)
	require.NoError(t, err)
	require.Equal(t, recordCount, nullModelCount, "all events should have NULL model_id")

	t.Logf("✅ Verified: %d events inserted with model_id=NULL", nullModelCount)

	// Wait for rollup worker to aggregate
	time.Sleep(6 * time.Second)

	// Verify rollup succeeded despite NULL model_id
	var rollupCount int64
	var rollupTokens int64
	rollupQuery := `
		SELECT
			COALESCE(SUM(request_count), 0),
			COALESCE(SUM(tokens_total), 0)
		FROM analytics_hourly_rollups
		WHERE organization_id = $1
		AND model_id IS NULL
	`
	err = pool.QueryRow(ctx, rollupQuery, orgID).Scan(&rollupCount, &rollupTokens)
	require.NoError(t, err)
	require.Equal(t, int64(recordCount), rollupCount, "rollup should have correct count")
	require.Equal(t, expectedTokens, rollupTokens, "rollup should have correct token total")

	t.Logf("✅ Verified: Rollup aggregated %d requests with %d tokens (model_id=NULL)", rollupCount, rollupTokens)

	// Verify query endpoint works with NULL model_id data
	usageHandler := api.NewUsageHandler(store, obs.Logger, nil)

	router := chi.NewRouter()
	router.Get("/analytics/v1/orgs/{orgId}/apikeys/{apiKeyId}/usage", usageHandler.GetAPIKeyUsage)

	server := httptest.NewServer(router)
	defer server.Close()

	start := timestamp.Add(-1 * time.Hour).Format(time.RFC3339)
	end := timestamp.Add(1 * time.Hour).Format(time.RFC3339)
	apiURL := fmt.Sprintf("%s/analytics/v1/orgs/%s/apikeys/%s/usage?start=%s&end=%s&granularity=hour",
		server.URL, orgID.String(), apiKeyID.String(), start, end)

	resp, err := http.Get(apiURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "API should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp APIKeyUsageSeriesResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)

	// Verify totals match (despite NULL model_id)
	require.Equal(t, int64(recordCount), apiResp.Totals.Invocations, "should have correct invocation count")
	require.Equal(t, expectedTokens, apiResp.Totals.TotalTokens, "should have correct token total")

	t.Logf("✅ E2E test passed: NULL model_id → ingestion → rollup → query")
	t.Logf("   Events with NULL model_id: %d", nullModelCount)
	t.Logf("   Rollup aggregated: %d invocations, %d tokens", rollupCount, rollupTokens)
	t.Logf("   API query returned: %d invocations, %d tokens", apiResp.Totals.Invocations, apiResp.Totals.TotalTokens)
}
