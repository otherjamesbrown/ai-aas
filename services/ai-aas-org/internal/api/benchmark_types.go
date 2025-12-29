// Package api provides the API client for the AI-as-a-Service platform.
package api

import (
	"time"
)

// BenchmarkScenario represents a benchmark scenario configuration synced from config repo
type BenchmarkScenario struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config"`
	SyncedAt    time.Time              `json:"synced_at"`
}

// BenchmarkTarget represents a benchmark target configuration
type BenchmarkTarget struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       string                 `json:"model_name"`
	ScenarioName    string                 `json:"scenario_name"`
	Environment     string                 `json:"environment"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	OrgID           *string                `json:"org_id,omitempty"`
	Status          string                 `json:"status"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled bool                   `json:"schedule_enabled"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
	LastRunAt       *time.Time             `json:"last_run_at,omitempty"`
	LastRunStatus   *string                `json:"last_run_status,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// BenchmarkRun represents a benchmark run instance
type BenchmarkRun struct {
	ID              string            `json:"id"`
	TargetID        string            `json:"target_id"`
	ScenarioName    string            `json:"scenario_name"`
	Status          string            `json:"status"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	DurationSeconds *int              `json:"duration_seconds,omitempty"`
	Results         *BenchmarkResults `json:"results,omitempty"`
	ErrorMessage    *string           `json:"error_message,omitempty"`
	TriggeredBy     string            `json:"triggered_by"`
	TriggeredByUser *string           `json:"triggered_by_user,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// BenchmarkResults represents the results of a benchmark run
type BenchmarkResults struct {
	// Throughput metrics
	RequestsTotal     int     `json:"requests_total,omitempty"`
	RequestsPerSecond float64 `json:"requests_per_second,omitempty"`
	TokensPerSecond   float64 `json:"tokens_per_second,omitempty"`

	// Latency metrics (in milliseconds)
	LatencyP50 float64 `json:"latency_p50_ms,omitempty"`
	LatencyP90 float64 `json:"latency_p90_ms,omitempty"`
	LatencyP95 float64 `json:"latency_p95_ms,omitempty"`
	LatencyP99 float64 `json:"latency_p99_ms,omitempty"`
	LatencyAvg float64 `json:"latency_avg_ms,omitempty"`
	LatencyMin float64 `json:"latency_min_ms,omitempty"`
	LatencyMax float64 `json:"latency_max_ms,omitempty"`

	// Time to First Token (TTFT) metrics
	TTFTP50 float64 `json:"ttft_p50_ms,omitempty"`
	TTFTP90 float64 `json:"ttft_p90_ms,omitempty"`
	TTFTP99 float64 `json:"ttft_p99_ms,omitempty"`
	TTFTAvg float64 `json:"ttft_avg_ms,omitempty"`

	// Inter-Token Latency (ITL) metrics
	ITLP50 float64 `json:"itl_p50_ms,omitempty"`
	ITLP90 float64 `json:"itl_p90_ms,omitempty"`
	ITLP99 float64 `json:"itl_p99_ms,omitempty"`
	ITLAvg float64 `json:"itl_avg_ms,omitempty"`

	// Error metrics
	ErrorCount int     `json:"error_count,omitempty"`
	ErrorRate  float64 `json:"error_rate,omitempty"`

	// Token metrics
	TotalInputTokens  int `json:"total_input_tokens,omitempty"`
	TotalOutputTokens int `json:"total_output_tokens,omitempty"`

	// Additional raw data
	RawMetrics map[string]interface{} `json:"raw_metrics,omitempty"`
}

// BenchmarkPagination represents pagination info in API responses
type BenchmarkPagination struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset,omitempty"`
}

// ListBenchmarkScenariosResponse represents the API response for listing scenarios
type ListBenchmarkScenariosResponse struct {
	Scenarios  []BenchmarkScenario `json:"scenarios"`
	Pagination BenchmarkPagination `json:"pagination"`
}

// ListBenchmarkTargetsResponse represents the API response for listing targets
type ListBenchmarkTargetsResponse struct {
	Targets    []BenchmarkTarget   `json:"targets"`
	Pagination BenchmarkPagination `json:"pagination"`
}

// ListBenchmarkRunsResponse represents the API response for listing runs
type ListBenchmarkRunsResponse struct {
	Runs       []BenchmarkRun      `json:"runs"`
	Pagination BenchmarkPagination `json:"pagination"`
}

// CreateBenchmarkTargetRequest represents the request to create a benchmark target
type CreateBenchmarkTargetRequest struct {
	Name            string                 `json:"name"`
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       string                 `json:"model_name"`
	ScenarioName    string                 `json:"scenario_name"`
	Environment     string                 `json:"environment"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled *bool                  `json:"schedule_enabled,omitempty"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
}

// UpdateBenchmarkTargetRequest represents the request to update a benchmark target
type UpdateBenchmarkTargetRequest struct {
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       *string                `json:"model_name,omitempty"`
	ScenarioName    *string                `json:"scenario_name,omitempty"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	Status          *string                `json:"status,omitempty"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled *bool                  `json:"schedule_enabled,omitempty"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
}

// TriggerBenchmarkRunRequest represents the request to trigger a benchmark run
type TriggerBenchmarkRunRequest struct {
	TriggeredByUser *string `json:"triggered_by_user,omitempty"`
}

// ListBenchmarkTargetsOptions represents options for listing targets
type ListBenchmarkTargetsOptions struct {
	Environment  string
	ModelName    string
	ScenarioName string
	Status       string
	Limit        int
	Offset       int
}

// ListBenchmarkRunsOptions represents options for listing runs
type ListBenchmarkRunsOptions struct {
	TargetID     string
	ScenarioName string
	Status       string
	TriggeredBy  string
	Limit        int
	Offset       int
}
