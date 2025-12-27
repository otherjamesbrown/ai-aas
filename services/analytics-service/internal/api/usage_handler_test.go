package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

func TestConvertPoints(t *testing.T) {
	now := time.Now().UTC()
	modelID := uuid.New()

	tests := []struct {
		name   string
		points []postgres.UsagePoint
		want   []UsagePointResponse
	}{
		{
			name:   "empty points",
			points: []postgres.UsagePoint{},
			want:   []UsagePointResponse{},
		},
		{
			name: "single point without model",
			points: []postgres.UsagePoint{
				{
					BucketStart:       now,
					ModelID:           nil,
					Invocations:       100,
					InputTokens:       5000,
					OutputTokens:      2500,
					CostEstimateCents: 0.75, // In dollars
				},
			},
			want: []UsagePointResponse{
				{
					BucketStart:       now.Format(time.RFC3339),
					ModelID:           nil,
					Invocations:       100,
					InputTokens:       5000,
					OutputTokens:      2500,
					CostEstimateCents: 75, // Converted to cents
				},
			},
		},
		{
			name: "single point with model",
			points: []postgres.UsagePoint{
				{
					BucketStart:       now,
					ModelID:           &modelID,
					Invocations:       50,
					InputTokens:       2500,
					OutputTokens:      1250,
					CostEstimateCents: 0.50,
				},
			},
			want: []UsagePointResponse{
				{
					BucketStart:       now.Format(time.RFC3339),
					ModelID:           ptrString(modelID.String()),
					Invocations:       50,
					InputTokens:       2500,
					OutputTokens:      1250,
					CostEstimateCents: 50,
				},
			},
		},
		{
			name: "multiple points",
			points: []postgres.UsagePoint{
				{
					BucketStart:       now,
					Invocations:       100,
					InputTokens:       5000,
					OutputTokens:      2500,
					CostEstimateCents: 0.75,
				},
				{
					BucketStart:       now.Add(time.Hour),
					Invocations:       200,
					InputTokens:       10000,
					OutputTokens:      5000,
					CostEstimateCents: 1.50,
				},
			},
			want: []UsagePointResponse{
				{
					BucketStart:       now.Format(time.RFC3339),
					Invocations:       100,
					InputTokens:       5000,
					OutputTokens:      2500,
					CostEstimateCents: 75,
				},
				{
					BucketStart:       now.Add(time.Hour).Format(time.RFC3339),
					Invocations:       200,
					InputTokens:       10000,
					OutputTokens:      5000,
					CostEstimateCents: 150,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPoints(tt.points)
			require.Equal(t, len(tt.want), len(result))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].BucketStart, result[i].BucketStart)
				assert.Equal(t, tt.want[i].Invocations, result[i].Invocations)
				assert.Equal(t, tt.want[i].InputTokens, result[i].InputTokens)
				assert.Equal(t, tt.want[i].OutputTokens, result[i].OutputTokens)
				assert.Equal(t, tt.want[i].CostEstimateCents, result[i].CostEstimateCents)
				if tt.want[i].ModelID == nil {
					assert.Nil(t, result[i].ModelID)
				} else {
					require.NotNil(t, result[i].ModelID)
					assert.Equal(t, *tt.want[i].ModelID, *result[i].ModelID)
				}
			}
		})
	}
}

func TestUsageHandler_respondError(t *testing.T) {
	logger := zap.NewNop()
	handler := &UsageHandler{
		logger: logger,
	}

	tests := []struct {
		name       string
		status     int
		message    string
		wantStatus int
		wantTitle  string
	}{
		{
			name:       "bad request",
			status:     http.StatusBadRequest,
			message:    "invalid parameter",
			wantStatus: 400,
			wantTitle:  "Bad Request",
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			message:    "resource not found",
			wantStatus: 404,
			wantTitle:  "Not Found",
		},
		{
			name:       "internal server error",
			status:     http.StatusInternalServerError,
			message:    "database error",
			wantStatus: 500,
			wantTitle:  "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.respondError(w, tt.status, tt.message, nil)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, float64(tt.wantStatus), response["status"])
			assert.Equal(t, tt.wantTitle, response["title"])
			assert.Equal(t, tt.message, response["detail"])
		})
	}
}

func TestUsageHandler_respondJSON(t *testing.T) {
	logger := zap.NewNop()
	handler := &UsageHandler{
		logger: logger,
	}

	t.Run("responds with JSON content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"key": "value"}
		handler.respondJSON(w, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "value", response["key"])
	})

	t.Run("responds with struct", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := UsageTotalsResponse{
			Invocations:       100,
			InputTokens:       5000,
			OutputTokens:      2500,
			CostEstimateCents: 75,
		}
		handler.respondJSON(w, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, w.Code)

		var response UsageTotalsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, int64(100), response.Invocations)
		assert.Equal(t, int64(5000), response.InputTokens)
		assert.Equal(t, int64(2500), response.OutputTokens)
		assert.Equal(t, int64(75), response.CostEstimateCents)
	})

	t.Run("handles different status codes", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.respondJSON(w, http.StatusCreated, map[string]string{"id": "123"})
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestUsageHandler_GetOrgUsage_Validation(t *testing.T) {
	logger := zap.NewNop()
	handler := NewUsageHandler(nil, logger, nil)

	tests := []struct {
		name       string
		orgID      string
		query      string
		wantStatus int
		wantDetail string
	}{
		{
			name:       "invalid org_id",
			orgID:      "not-a-uuid",
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid org_id",
		},
		{
			name:       "invalid start parameter",
			orgID:      uuid.New().String(),
			query:      "?start=invalid&end=2024-01-02T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid start parameter",
		},
		{
			name:       "invalid end parameter",
			orgID:      uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=invalid",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid end parameter",
		},
		{
			name:       "end before start",
			orgID:      uuid.New().String(),
			query:      "?start=2024-01-02T00:00:00Z&end=2024-01-01T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "end must be after start",
		},
		{
			name:       "invalid granularity",
			orgID:      uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&granularity=minute",
			wantStatus: http.StatusBadRequest,
			wantDetail: "granularity must be 'hour' or 'day'",
		},
		{
			name:       "invalid model_id",
			orgID:      uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&modelId=not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid model_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/analytics/v1/orgs/{orgId}/usage", handler.GetOrgUsage)

			req := httptest.NewRequest("GET", "/analytics/v1/orgs/"+tt.orgID+"/usage"+tt.query, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["detail"], tt.wantDetail)
		})
	}
}

func TestNewUsageHandler(t *testing.T) {
	logger := zap.NewNop()

	handler := NewUsageHandler(nil, logger, nil)
	require.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
}

func TestFreshnessIndicator(t *testing.T) {
	now := time.Now().UTC()

	indicator := FreshnessIndicator{
		Status:       "fresh",
		LagSeconds:   120,
		LastEventAt:  now.Add(-2 * time.Minute),
		LastRollupAt: now.Add(-1 * time.Minute),
	}

	assert.Equal(t, "fresh", indicator.Status)
	assert.Equal(t, 120, indicator.LagSeconds)
	assert.WithinDuration(t, now.Add(-2*time.Minute), indicator.LastEventAt, time.Second)
	assert.WithinDuration(t, now.Add(-1*time.Minute), indicator.LastRollupAt, time.Second)
}

func TestUsageSeriesResponse(t *testing.T) {
	now := time.Now().UTC()

	response := UsageSeriesResponse{
		OrgID:       uuid.New().String(),
		Granularity: "hour",
		Series: []UsagePointResponse{
			{
				BucketStart:       now.Format(time.RFC3339),
				Invocations:       100,
				CostEstimateCents: 75,
			},
		},
		Totals: UsageTotalsResponse{
			Invocations:       100,
			InputTokens:       5000,
			OutputTokens:      2500,
			CostEstimateCents: 75,
		},
		Freshness: FreshnessIndicator{
			Status:     "fresh",
			LagSeconds: 60,
		},
	}

	// Verify JSON serialization
	jsonBytes, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded UsageSeriesResponse
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.OrgID, decoded.OrgID)
	assert.Equal(t, response.Granularity, decoded.Granularity)
	assert.Len(t, decoded.Series, 1)
	assert.Equal(t, response.Totals.Invocations, decoded.Totals.Invocations)
	assert.Equal(t, response.Freshness.Status, decoded.Freshness.Status)
}

// TestUsageHandler_GetAPIKeyUsage_Validation tests input validation for the API key usage endpoint.
func TestUsageHandler_GetAPIKeyUsage_Validation(t *testing.T) {
	logger := zap.NewNop()
	handler := NewUsageHandler(nil, logger, nil)

	tests := []struct {
		name       string
		orgID      string
		apiKeyID   string
		query      string
		wantStatus int
		wantDetail string
	}{
		{
			name:       "invalid org_id",
			orgID:      "not-a-uuid",
			apiKeyID:   uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid org_id",
		},
		{
			name:       "invalid api_key_id",
			orgID:      uuid.New().String(),
			apiKeyID:   "not-a-uuid",
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid api_key_id",
		},
		{
			name:       "invalid start parameter",
			orgID:      uuid.New().String(),
			apiKeyID:   uuid.New().String(),
			query:      "?start=invalid&end=2024-01-02T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid start parameter",
		},
		{
			name:       "invalid end parameter",
			orgID:      uuid.New().String(),
			apiKeyID:   uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=invalid",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid end parameter",
		},
		{
			name:       "end before start",
			orgID:      uuid.New().String(),
			apiKeyID:   uuid.New().String(),
			query:      "?start=2024-01-02T00:00:00Z&end=2024-01-01T00:00:00Z",
			wantStatus: http.StatusBadRequest,
			wantDetail: "end must be after start",
		},
		{
			name:       "invalid granularity",
			orgID:      uuid.New().String(),
			apiKeyID:   uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&granularity=minute",
			wantStatus: http.StatusBadRequest,
			wantDetail: "granularity must be 'hour' or 'day'",
		},
		{
			name:       "invalid model_id",
			orgID:      uuid.New().String(),
			apiKeyID:   uuid.New().String(),
			query:      "?start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&modelId=not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantDetail: "invalid model_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/analytics/v1/orgs/{orgId}/apikeys/{apiKeyId}/usage", handler.GetAPIKeyUsage)

			req := httptest.NewRequest("GET", "/analytics/v1/orgs/"+tt.orgID+"/apikeys/"+tt.apiKeyID+"/usage"+tt.query, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response["detail"], tt.wantDetail)
		})
	}
}

// TestAPIKeyUsageSeriesResponse tests serialization of API key usage response.
func TestAPIKeyUsageSeriesResponse(t *testing.T) {
	now := time.Now().UTC()
	orgID := uuid.New()
	apiKeyID := uuid.New()

	response := APIKeyUsageSeriesResponse{
		OrgID:       orgID.String(),
		APIKeyID:    apiKeyID.String(),
		Granularity: "hour",
		Series: []UsagePointResponse{
			{
				BucketStart:       now.Format(time.RFC3339),
				Invocations:       100,
				InputTokens:       5000,
				OutputTokens:      2500,
				CostEstimateCents: 75,
			},
		},
		Totals: UsageTotalsResponse{
			Invocations:       100,
			InputTokens:       5000,
			OutputTokens:      2500,
			CostEstimateCents: 75,
		},
		Freshness: FreshnessIndicator{
			Status:     "fresh",
			LagSeconds: 60,
		},
	}

	// Verify JSON serialization
	jsonBytes, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded APIKeyUsageSeriesResponse
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.OrgID, decoded.OrgID)
	assert.Equal(t, response.APIKeyID, decoded.APIKeyID)
	assert.Equal(t, response.Granularity, decoded.Granularity)
	assert.Len(t, decoded.Series, 1)
	assert.Equal(t, response.Totals.Invocations, decoded.Totals.Invocations)
	assert.Equal(t, response.Freshness.Status, decoded.Freshness.Status)

	// Verify apiKeyId is present in JSON
	var rawJSON map[string]interface{}
	err = json.Unmarshal(jsonBytes, &rawJSON)
	require.NoError(t, err)
	assert.Equal(t, apiKeyID.String(), rawJSON["apiKeyId"])
}

// ptrString returns a pointer to the given string
func ptrString(s string) *string {
	return &s
}
