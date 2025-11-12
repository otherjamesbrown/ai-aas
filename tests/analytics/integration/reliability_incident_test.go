// Package integration provides end-to-end integration tests for reliability and incident export.
//
// Purpose:
//   This package validates that the analytics service correctly calculates error rates,
//   latency percentiles, and generates incident CSV exports for reliability analysis.
//
package integration

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/exports"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
	"github.com/otherjamesbrown/ai-aas/shared/go/observability"
)

// seedReliabilityTestData inserts usage events with varying error rates and latencies.
func seedReliabilityTestData(t *testing.T, store *postgres.Store, orgID uuid.UUID, modelID uuid.UUID, successCount, errorCount int) {
	t.Helper()

	ctx := context.Background()
	now := time.Now().UTC()

	events := make([]postgres.UsageEvent, successCount+errorCount)
	idx := 0

	// Add successful events with varying latencies
	for i := 0; i < successCount; i++ {
		events[idx] = postgres.UsageEvent{
			EventID:           uuid.New(),
			OrgID:             orgID,
			OccurredAt:        now.Add(-time.Duration(i) * time.Minute),
			ReceivedAt:        now,
			ModelID:           modelID,
			ActorID:           uuid.Nil,
			InputTokens:       100 + int64(i*10),
			OutputTokens:      50 + int64(i*5),
			LatencyMS:         100 + i*10, // Varying latency: 100ms to 1000ms+
			Status:            "success",
			ErrorCode:         "",
			CostEstimateCents: float64(i) * 0.01,
			Metadata:          map[string]interface{}{},
		}
		idx++
	}

	// Add error events
	for i := 0; i < errorCount; i++ {
		events[idx] = postgres.UsageEvent{
			EventID:           uuid.New(),
			OrgID:             orgID,
			OccurredAt:        now.Add(-time.Duration(successCount+i) * time.Minute),
			ReceivedAt:        now,
			ModelID:           modelID,
			ActorID:           uuid.Nil,
			InputTokens:       100,
			OutputTokens:      0,
			LatencyMS:         50 + i*5,
			Status:            "error",
			ErrorCode:         "TIMEOUT",
			CostEstimateCents: 0,
			Metadata:          map[string]interface{}{},
		}
		idx++
	}

	batchID, err := store.CreateIngestionBatch(ctx, 0, []uuid.UUID{orgID})
	require.NoError(t, err)

	inserted, err := store.InsertUsageEvents(ctx, events, batchID)
	require.NoError(t, err)
	require.Equal(t, len(events), inserted)

	err = store.CompleteIngestionBatch(ctx, batchID, 0)
	require.NoError(t, err)
}

// TestReliabilityAPI validates that reliability metrics are correctly calculated.
func TestReliabilityAPI(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Seed test data: 90 success, 10 errors (10% error rate)
	seedReliabilityTestData(t, store, orgID, modelID, 90, 10)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	handler := api.NewReliabilityHandler(store, obs.Logger)

	// Query reliability data
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	end := now.Add(1 * time.Hour)

	req := httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/reliability?start=%s&end=%s&granularity=day",
		orgID.String(),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	), nil)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetOrgReliability(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response api.ReliabilitySeriesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, orgID.String(), response.OrgID)
	require.Equal(t, "day", response.Granularity)
	require.Greater(t, len(response.Series), 0, "should have at least one data point")

	// Verify error rate is approximately 10% (10 errors / 100 total)
	var totalErrorRate float64
	var pointCount int
	for _, point := range response.Series {
		if point.ModelID != nil && *point.ModelID == modelID.String() {
			totalErrorRate += point.ErrorRate
			pointCount++
		}
	}
	if pointCount > 0 {
		avgErrorRate := totalErrorRate / float64(pointCount)
		require.InDelta(t, 0.10, avgErrorRate, 0.05, "error rate should be approximately 10%")
	}

	// Verify latency percentiles are present
	for _, point := range response.Series {
		require.Greater(t, point.LatencyMs.P50, 0, "p50 latency should be present")
		require.Greater(t, point.LatencyMs.P95, 0, "p95 latency should be present")
		require.Greater(t, point.LatencyMs.P99, 0, "p99 latency should be present")
		require.GreaterOrEqual(t, point.LatencyMs.P95, point.LatencyMs.P50, "p95 should be >= p50")
		require.GreaterOrEqual(t, point.LatencyMs.P99, point.LatencyMs.P95, "p99 should be >= p95")
	}
}

// TestReliabilityAPIModelFilter validates model filtering in reliability API.
func TestReliabilityAPIModelFilter(t *testing.T) {
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
	seedReliabilityTestData(t, store, orgID, model1ID, 80, 20) // 20% error rate
	seedReliabilityTestData(t, store, orgID, model2ID, 95, 5)  // 5% error rate

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	handler := api.NewReliabilityHandler(store, obs.Logger)

	// Query with model1 filter
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	end := now.Add(1 * time.Hour)

	req := httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/reliability?start=%s&end=%s&granularity=day&modelId=%s",
		orgID.String(),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		model1ID.String(),
	), nil)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetOrgReliability(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response api.ReliabilitySeriesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify all points are for model1
	for _, point := range response.Series {
		require.NotNil(t, point.ModelID, "all points should have model_id")
		require.Equal(t, model1ID.String(), *point.ModelID, "all points should be for model1")
		// Model1 has 20% error rate
		require.InDelta(t, 0.20, point.ErrorRate, 0.05, "model1 error rate should be approximately 20%")
	}
}

// TestIncidentExport validates CSV export generation.
func TestIncidentExport(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Seed test data
	seedReliabilityTestData(t, store, orgID, modelID, 50, 10)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	exporter := exports.NewIncidentExporter(store, obs.Logger)

	// Export incident data
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	end := now.Add(1 * time.Hour)

	csvData, err := exporter.Export(ctx, exports.ExportRequest{
		OrgID:   orgID,
		ModelID: &modelID,
		Start:   start,
		End:     end,
		MaxRows: 10000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, csvData, "CSV data should not be empty")

	// Parse CSV
	reader := csv.NewReader(strings.NewReader(string(csvData)))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Greater(t, len(records), 1, "should have header + data rows")

	// Verify header
	header := records[0]
	expectedHeaders := []string{
		"event_id", "org_id", "occurred_at", "received_at", "model_id",
		"actor_id", "input_tokens", "output_tokens", "latency_ms",
		"status", "error_code", "cost_estimate_cents", "metadata",
	}
	require.Equal(t, expectedHeaders, header, "CSV header should match expected format")

	// Verify data rows
	dataRows := records[1:]
	require.Greater(t, len(dataRows), 0, "should have data rows")

	// Verify all rows are for the correct org and model
	for _, row := range dataRows {
		require.Equal(t, orgID.String(), row[1], "org_id should match")
		require.Equal(t, modelID.String(), row[4], "model_id should match")
	}

	// Verify we have both success and error events
	hasSuccess := false
	hasError := false
	for _, row := range dataRows {
		status := row[9] // status column
		if status == "success" {
			hasSuccess = true
		}
		if status == "error" {
			hasError = true
		}
	}
	require.True(t, hasSuccess, "should have success events")
	require.True(t, hasError, "should have error events")
}

// TestIncidentExportTimeRange validates that export respects time range.
func TestIncidentExportTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store, err := postgres.NewStore(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer store.Close()

	orgID := uuid.New()
	modelID := uuid.New()

	// Seed test data
	seedReliabilityTestData(t, store, orgID, modelID, 50, 10)

	// Initialize observability
	obsCfg := observability.Config{
		ServiceName: "analytics-service-test",
		Environment: "test",
	}
	obs, err := observability.Init(ctx, obsCfg)
	require.NoError(t, err)
	defer obs.Shutdown(ctx)

	exporter := exports.NewIncidentExporter(store, obs.Logger)

	// Export with narrow time range (should return fewer rows)
	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour) // Only last hour
	end := now.Add(1 * time.Hour)

	csvData, err := exporter.Export(ctx, exports.ExportRequest{
		OrgID:   orgID,
		ModelID: &modelID,
		Start:   start,
		End:     end,
		MaxRows: 10000,
	})
	require.NoError(t, err)

	// Parse CSV
	reader := csv.NewReader(strings.NewReader(string(csvData)))
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Should have fewer rows than full 24-hour export
	dataRows := records[1:]
	require.LessOrEqual(t, len(dataRows), 2, "narrow time range should return fewer rows")
}

// TestReliabilityAPIErrorHandling validates error handling in reliability API.
func TestReliabilityAPIErrorHandling(t *testing.T) {
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

	handler := api.NewReliabilityHandler(store, obs.Logger)

	// Test: Invalid org ID
	req := httptest.NewRequest("GET", "/analytics/v1/orgs/invalid-uuid/reliability?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.GetOrgReliability(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Missing start parameter
	orgID := uuid.New()
	req = httptest.NewRequest("GET", fmt.Sprintf("/analytics/v1/orgs/%s/reliability?end=2024-01-02T00:00:00Z", orgID.String()), nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	handler.GetOrgReliability(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Invalid granularity
	req = httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/reliability?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&granularity=invalid",
		orgID.String(),
	), nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	handler.GetOrgReliability(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// Test: Invalid percentile
	req = httptest.NewRequest("GET", fmt.Sprintf(
		"/analytics/v1/orgs/%s/reliability?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&percentile=invalid",
		orgID.String(),
	), nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	handler.GetOrgReliability(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

