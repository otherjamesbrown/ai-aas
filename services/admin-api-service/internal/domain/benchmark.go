package domain

import (
	"time"

	"github.com/google/uuid"
)

// BenchmarkScenario represents a benchmark scenario configuration synced from config repo
type BenchmarkScenario struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config"`
	SyncedAt    time.Time              `json:"synced_at"`
}

// BenchmarkScenarioUpsert represents a request to create or update a scenario
type BenchmarkScenarioUpsert struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config"`
}

// BenchmarkScenarioListParams represents query parameters for listing scenarios
type BenchmarkScenarioListParams struct {
	Limit  int
	Offset int
}

// BenchmarkScenarioListResponse represents a paginated list of scenarios
type BenchmarkScenarioListResponse struct {
	Scenarios  []BenchmarkScenario `json:"scenarios"`
	Pagination Pagination          `json:"pagination"`
}

// BenchmarkTarget represents a benchmark target configuration
type BenchmarkTarget struct {
	ID              uuid.UUID              `json:"id"`
	Name            string                 `json:"name"`
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       string                 `json:"model_name"`
	ScenarioName    string                 `json:"scenario_name"`
	Environment     string                 `json:"environment"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	OrgID           *uuid.UUID             `json:"org_id,omitempty"`
	Status          string                 `json:"status"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled bool                   `json:"schedule_enabled"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
	LastRunAt       *time.Time             `json:"last_run_at,omitempty"`
	LastRunStatus   *string                `json:"last_run_status,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// BenchmarkTargetCreate represents a request to create a benchmark target
type BenchmarkTargetCreate struct {
	Name            string                 `json:"name"`
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       string                 `json:"model_name"`
	ScenarioName    string                 `json:"scenario_name"`
	Environment     string                 `json:"environment"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	OrgID           *uuid.UUID             `json:"org_id,omitempty"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled *bool                  `json:"schedule_enabled,omitempty"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
}

// BenchmarkTargetUpdate represents a partial update to a benchmark target
type BenchmarkTargetUpdate struct {
	DisplayName     *string                `json:"display_name,omitempty"`
	ModelName       *string                `json:"model_name,omitempty"`
	ScenarioName    *string                `json:"scenario_name,omitempty"`
	EndpointURL     *string                `json:"endpoint_url,omitempty"`
	OrgID           *uuid.UUID             `json:"org_id,omitempty"`
	Status          *string                `json:"status,omitempty"`
	IntervalSeconds *int                   `json:"interval_seconds,omitempty"`
	ScheduleEnabled *bool                  `json:"schedule_enabled,omitempty"`
	ConfigOverrides map[string]interface{} `json:"config_overrides,omitempty"`
	LastRunAt       *time.Time             `json:"last_run_at,omitempty"`
	LastRunStatus   *string                `json:"last_run_status,omitempty"`
}

// BenchmarkTargetListParams represents query parameters for listing targets
type BenchmarkTargetListParams struct {
	Environment  string
	ModelName    string
	ScenarioName string
	Status       string
	OrgID        *uuid.UUID
	Limit        int
	Offset       int
}

// BenchmarkTargetListResponse represents a paginated list of targets
type BenchmarkTargetListResponse struct {
	Targets    []BenchmarkTarget `json:"targets"`
	Pagination Pagination        `json:"pagination"`
}

// BenchmarkRun represents a benchmark run instance
type BenchmarkRun struct {
	ID              uuid.UUID         `json:"id"`
	TargetID        uuid.UUID         `json:"target_id"`
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

// BenchmarkRunCreate represents a request to create a benchmark run
type BenchmarkRunCreate struct {
	TargetID        uuid.UUID `json:"target_id"`
	ScenarioName    string    `json:"scenario_name"`
	TriggeredBy     string    `json:"triggered_by,omitempty"`
	TriggeredByUser *string   `json:"triggered_by_user,omitempty"`
}

// BenchmarkRunUpdate represents a partial update to a benchmark run
type BenchmarkRunUpdate struct {
	Status          *string           `json:"status,omitempty"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	DurationSeconds *int              `json:"duration_seconds,omitempty"`
	Results         *BenchmarkResults `json:"results,omitempty"`
	ErrorMessage    *string           `json:"error_message,omitempty"`
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

// BenchmarkRunListParams represents query parameters for listing runs
type BenchmarkRunListParams struct {
	TargetID     *uuid.UUID
	ScenarioName string
	Status       string
	TriggeredBy  string
	StartedAfter *time.Time
	Limit        int
	Offset       int
}

// BenchmarkRunListResponse represents a paginated list of runs
type BenchmarkRunListResponse struct {
	Runs       []BenchmarkRun `json:"runs"`
	Pagination Pagination     `json:"pagination"`
}

// Valid benchmark target statuses
var ValidBenchmarkTargetStatuses = []string{"active", "paused", "disabled"}

// Valid benchmark run statuses
var ValidBenchmarkRunStatuses = []string{"pending", "running", "completed", "failed", "cancelled"}

// Valid benchmark trigger types
var ValidBenchmarkTriggerTypes = []string{"schedule", "manual", "api", "ci"}

// IsValidBenchmarkTargetStatus checks if a target status is valid
func IsValidBenchmarkTargetStatus(status string) bool {
	for _, s := range ValidBenchmarkTargetStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsValidBenchmarkRunStatus checks if a run status is valid
func IsValidBenchmarkRunStatus(status string) bool {
	for _, s := range ValidBenchmarkRunStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsValidBenchmarkTriggerType checks if a trigger type is valid
func IsValidBenchmarkTriggerType(triggerType string) bool {
	for _, t := range ValidBenchmarkTriggerTypes {
		if t == triggerType {
			return true
		}
	}
	return false
}
