// Package benchmark provides a client for benchmark operations via the Admin API.
package benchmark

import (
	"time"
)

// Scenario represents a benchmark scenario configuration synced from config repo
type Scenario struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config"`
	SyncedAt    time.Time              `json:"synced_at"`
}

// Target represents a benchmark target configuration
type Target struct {
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

// Run represents a benchmark run instance
type Run struct {
	ID              string     `json:"id"`
	TargetID        string     `json:"target_id"`
	ScenarioName    string     `json:"scenario_name"`
	Status          string     `json:"status"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	DurationSeconds *int       `json:"duration_seconds,omitempty"`
	Results         *Results   `json:"results,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
	TriggeredBy     string     `json:"triggered_by"`
	TriggeredByUser *string    `json:"triggered_by_user,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Results represents the results of a benchmark run
type Results struct {
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

// Pagination represents pagination info in API responses
type Pagination struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset,omitempty"`
}

// ListScenariosResponse represents the API response for listing scenarios
type ListScenariosResponse struct {
	Scenarios  []Scenario `json:"scenarios"`
	Pagination Pagination `json:"pagination"`
}

// ListTargetsResponse represents the API response for listing targets
type ListTargetsResponse struct {
	Targets    []Target   `json:"targets"`
	Pagination Pagination `json:"pagination"`
}

// ListRunsResponse represents the API response for listing runs
type ListRunsResponse struct {
	Runs       []Run      `json:"runs"`
	Pagination Pagination `json:"pagination"`
}

// CreateTargetRequest represents the request to create a benchmark target
type CreateTargetRequest struct {
	Name            string                 `json:"name"`
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       string                 `json:"model_name"`
	ScenarioName    string                 `json:"scenario_name"`
	Environment     string                 `json:"environment"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	OrgID           *string                `json:"org_id,omitempty"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled *bool                  `json:"schedule_enabled,omitempty"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
}

// UpdateTargetRequest represents the request to update a benchmark target
type UpdateTargetRequest struct {
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       *string                `json:"model_name,omitempty"`
	ScenarioName    *string                `json:"scenario_name,omitempty"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	OrgID           *string                `json:"org_id,omitempty"`
	Status          *string                `json:"status,omitempty"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled *bool                  `json:"schedule_enabled,omitempty"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
}

// TriggerRunRequest represents the request to trigger a benchmark run
type TriggerRunRequest struct {
	TriggeredBy     string  `json:"triggered_by,omitempty"`
	TriggeredByUser *string `json:"triggered_by_user,omitempty"`
}

// ListTargetsOptions represents options for listing targets
type ListTargetsOptions struct {
	Environment  string
	ModelName    string
	ScenarioName string
	Status       string
	OrgID        string
	Limit        int
	Offset       int
}

// ListRunsOptions represents options for listing runs
type ListRunsOptions struct {
	TargetID     string
	ScenarioName string
	Status       string
	TriggeredBy  string
	Limit        int
	Offset       int
}

// StatusSummary represents the overall benchmark status summary
type StatusSummary struct {
	TotalTargets  int            `json:"total_targets"`
	ActiveTargets int            `json:"active_targets"`
	PausedTargets int            `json:"paused_targets"`
	TotalRuns     int            `json:"total_runs"`
	RecentRuns    []Run          `json:"recent_runs,omitempty"`
	ByEnvironment map[string]int `json:"by_environment,omitempty"`
}

// ScenarioUpsert represents a scenario to create or update during sync
type ScenarioUpsert struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// SyncScenariosRequest represents the request to sync scenarios
type SyncScenariosRequest struct {
	Scenarios     []ScenarioUpsert `json:"scenarios"`
	DeleteOrphans bool             `json:"delete_orphans,omitempty"`
}

// SyncScenariosResponse represents the response from syncing scenarios
type SyncScenariosResponse struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Deleted []string `json:"deleted"`
}
