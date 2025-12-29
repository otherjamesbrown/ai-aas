package fixtures

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// BenchmarkScenario represents a benchmark scenario
type BenchmarkScenario struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config"`
	SyncedAt    time.Time              `json:"synced_at"`
}

// BenchmarkScenariosResponse represents the list scenarios response
type BenchmarkScenariosResponse struct {
	Scenarios  []BenchmarkScenario `json:"scenarios"`
	Pagination PaginationInfo      `json:"pagination"`
}

// BenchmarkTarget represents a benchmark target
type BenchmarkTarget struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	ModelName       string     `json:"model_name"`
	ScenarioName    string     `json:"scenario_name"`
	Environment     string     `json:"environment"`
	EndpointURL     *string    `json:"endpoint_url,omitempty"`
	OrgID           *string    `json:"org_id,omitempty"`
	Status          string     `json:"status"`
	IntervalSeconds *int       `json:"interval_seconds,omitempty"`
	ScheduleEnabled bool       `json:"schedule_enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// BenchmarkTargetsResponse represents the list targets response
type BenchmarkTargetsResponse struct {
	Targets    []BenchmarkTarget `json:"targets"`
	Pagination PaginationInfo    `json:"pagination"`
}

// BenchmarkRun represents a benchmark run
type BenchmarkRun struct {
	ID          string                 `json:"id"`
	TargetID    string                 `json:"target_id"`
	Status      string                 `json:"status"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Results     map[string]interface{} `json:"results,omitempty"`
	TriggeredBy string                 `json:"triggered_by"`
	CreatedAt   time.Time              `json:"created_at"`
}

// BenchmarkRunsResponse represents the list runs response
type BenchmarkRunsResponse struct {
	Runs       []BenchmarkRun `json:"runs"`
	Pagination PaginationInfo `json:"pagination"`
}

// BenchmarkStatus represents overall benchmark status
type BenchmarkStatus struct {
	Running       bool `json:"running"`
	TargetsCount  int  `json:"targets_count"`
	ActiveCount   int  `json:"active_count"`
	StoppedCount  int  `json:"stopped_count"`
	UptimeSeconds int  `json:"uptime_seconds"`
}

// RunnerTarget represents a target as seen by guidellm-runner
type RunnerTarget struct {
	Name        string                 `json:"name"`
	Model       string                 `json:"model"`
	URL         string                 `json:"url"`
	Environment string                 `json:"environment"`
	Status      string                 `json:"status"`
	Profile     string                 `json:"profile"`
	Rate        int                    `json:"rate"`
	MaxSeconds  int                    `json:"max_seconds"`
	RequestType string                 `json:"request_type"`
	LastRunAt   *time.Time             `json:"last_run_at,omitempty"`
	LastResults map[string]interface{} `json:"last_results,omitempty"`
}

// RunnerTargetsResponse represents the response from guidellm-runner /api/targets
type RunnerTargetsResponse struct {
	Targets []RunnerTarget `json:"targets"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset"`
}

// BenchmarkFixture provides benchmark test operations
type BenchmarkFixture struct {
	client   *harness.Client
	fixtures *harness.FixtureManager
}

// NewBenchmarkFixture creates a new benchmark fixture
func NewBenchmarkFixture(client *harness.Client, fixtures *harness.FixtureManager) *BenchmarkFixture {
	return &BenchmarkFixture{
		client:   client,
		fixtures: fixtures,
	}
}

// ListScenarios lists all benchmark scenarios
func (f *BenchmarkFixture) ListScenarios() ([]BenchmarkScenario, error) {
	resp, err := f.client.GET("/v1/benchmarks/scenarios")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var result BenchmarkScenariosResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result.Scenarios, nil
}

// GetScenario gets a specific scenario by name
func (f *BenchmarkFixture) GetScenario(name string) (*BenchmarkScenario, error) {
	resp, err := f.client.GET("/v1/benchmarks/scenarios/" + name)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var scenario BenchmarkScenario
	if err := json.Unmarshal(resp.Body, &scenario); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &scenario, nil
}

// SyncScenarios syncs scenarios from the config repository
func (f *BenchmarkFixture) SyncScenarios() error {
	resp, err := f.client.POST("/v1/benchmarks/scenarios/sync", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// CreateTarget creates a new benchmark target
func (f *BenchmarkFixture) CreateTarget(ctx *harness.Context, name, modelName, scenarioName, environment string) (*BenchmarkTarget, error) {
	reqBody := map[string]interface{}{
		"name":          name,
		"model_name":    modelName,
		"scenario_name": scenarioName,
		"environment":   environment,
	}

	resp, err := f.client.POST("/v1/benchmarks/targets", reqBody)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var target BenchmarkTarget
	if err := json.Unmarshal(resp.Body, &target); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Register for cleanup
	f.fixtures.Register("benchmark_target", target.ID, nil)

	return &target, nil
}

// ListTargets lists benchmark targets with optional environment filter
func (f *BenchmarkFixture) ListTargets(environment string) ([]BenchmarkTarget, error) {
	path := "/v1/benchmarks/targets"
	if environment != "" {
		path += "?environment=" + environment
	}

	resp, err := f.client.GET(path)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var result BenchmarkTargetsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result.Targets, nil
}

// GetTarget gets a specific target by ID
func (f *BenchmarkFixture) GetTarget(id string) (*BenchmarkTarget, error) {
	resp, err := f.client.GET("/v1/benchmarks/targets/" + id)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var target BenchmarkTarget
	if err := json.Unmarshal(resp.Body, &target); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &target, nil
}

// StartTarget starts a benchmark target
func (f *BenchmarkFixture) StartTarget(id string) (*BenchmarkTarget, error) {
	resp, err := f.client.POST("/v1/benchmarks/targets/"+id+"/start", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var target BenchmarkTarget
	if err := json.Unmarshal(resp.Body, &target); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &target, nil
}

// StopTarget stops a benchmark target
func (f *BenchmarkFixture) StopTarget(id string) (*BenchmarkTarget, error) {
	resp, err := f.client.POST("/v1/benchmarks/targets/"+id+"/stop", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var target BenchmarkTarget
	if err := json.Unmarshal(resp.Body, &target); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &target, nil
}

// DeleteTarget deletes a benchmark target
func (f *BenchmarkFixture) DeleteTarget(id string) error {
	resp, err := f.client.DELETE("/v1/benchmarks/targets/" + id)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// TriggerRun manually triggers a benchmark run
func (f *BenchmarkFixture) TriggerRun(targetID string) (*BenchmarkRun, error) {
	resp, err := f.client.POST("/v1/benchmarks/targets/"+targetID+"/trigger", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 202 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var run BenchmarkRun
	if err := json.Unmarshal(resp.Body, &run); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &run, nil
}

// ListRuns lists benchmark runs with optional target filter
func (f *BenchmarkFixture) ListRuns(targetID string) ([]BenchmarkRun, error) {
	path := "/v1/benchmarks/runs"
	if targetID != "" {
		path += "?target_id=" + targetID
	}

	resp, err := f.client.GET(path)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var result BenchmarkRunsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result.Runs, nil
}

// GetStatus gets the overall benchmark status
func (f *BenchmarkFixture) GetStatus() (*BenchmarkStatus, error) {
	resp, err := f.client.GET("/v1/benchmarks/status")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var status BenchmarkStatus
	if err := json.Unmarshal(resp.Body, &status); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &status, nil
}
