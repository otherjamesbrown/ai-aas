package benchmark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryPrometheus(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		expectedValues map[string]float64
		expectError    bool
	}{
		{
			name:           "successful query with single result",
			responseStatus: http.StatusOK,
			responseBody: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {"model": "llama-7b"},
							"value": [1234567890, "0.123"]
						}
					]
				}
			}`,
			expectedValues: map[string]float64{
				"llama-7b": 0.123,
			},
			expectError: false,
		},
		{
			name:           "successful query with multiple results",
			responseStatus: http.StatusOK,
			responseBody: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {"model": "llama-7b"},
							"value": [1234567890, "0.123"]
						},
						{
							"metric": {"model": "llama-13b"},
							"value": [1234567890, "0.456"]
						}
					]
				}
			}`,
			expectedValues: map[string]float64{
				"llama-7b":  0.123,
				"llama-13b": 0.456,
			},
			expectError: false,
		},
		{
			name:           "empty result set",
			responseStatus: http.StatusOK,
			responseBody: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": []
				}
			}`,
			expectedValues: map[string]float64{},
			expectError:    false,
		},
		{
			name:           "prometheus returns error status",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error": "internal server error"}`,
			expectedValues: nil,
			expectError:    true,
		},
		{
			name:           "query failed status",
			responseStatus: http.StatusOK,
			responseBody: `{
				"status": "error",
				"errorType": "bad_data",
				"error": "invalid query"
			}`,
			expectedValues: nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify it's a GET to /api/v1/query
				assert.Equal(t, "/api/v1/query", r.URL.Path)
				assert.Equal(t, "GET", r.Method)

				// Verify query parameter exists
				query := r.URL.Query().Get("query")
				assert.NotEmpty(t, query)

				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Execute query
			ctx := context.Background()
			values, err := queryPrometheus(ctx, server.URL, "test_metric")

			// Check error expectation
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedValues, values)
		})
	}
}

func TestQueryBenchmarkResults(t *testing.T) {
	// Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		var response interface{}

		switch {
		case query == `guidellm_benchmark_ttft_seconds{quantile="0.5"}`:
			response = map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result": []map[string]interface{}{
						{
							"metric": map[string]string{"model": "llama-7b"},
							"value":  []interface{}{float64(time.Now().Unix()), "0.050"},
						},
					},
				},
			}
		case query == `guidellm_benchmark_tokens_per_second`:
			response = map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result": []map[string]interface{}{
						{
							"metric": map[string]string{"model": "llama-7b"},
							"value":  []interface{}{float64(time.Now().Unix()), "120.5"},
						},
					},
				},
			}
		default:
			// Return empty result for other queries
			response = map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result":     []interface{}{},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	results, err := queryBenchmarkResults(ctx, server.URL, "")

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "llama-7b", result.Model)
	assert.InDelta(t, 50.0, result.TTFT, 0.1)        // 0.050s -> 50ms
	assert.InDelta(t, 120.5, result.TokensPerSec, 0.1)
}

func TestQueryBenchmarkResultsWithModelFilter(t *testing.T) {
	// Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []map[string]interface{}{
					{
						"metric": map[string]string{"model": "llama-7b"},
						"value":  []interface{}{float64(time.Now().Unix()), "100.0"},
					},
					{
						"metric": map[string]string{"model": "llama-13b"},
						"value":  []interface{}{float64(time.Now().Unix()), "200.0"},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	results, err := queryBenchmarkResults(ctx, server.URL, "llama-7b")

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "llama-7b", results[0].Model)
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{name: "zero", input: 0, expected: "-"},
		{name: "integer", input: 100, expected: "100.00"},
		{name: "decimal", input: 123.456, expected: "123.46"},
		{name: "small", input: 0.001, expected: "0.00"},
		{name: "negative", input: -50.5, expected: "-50.50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "-",
		},
		{
			name:     "valid time",
			input:    time.Date(2025, 1, 8, 15, 30, 0, 0, time.UTC),
			expected: "2025-01-08 15:30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimestamp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
