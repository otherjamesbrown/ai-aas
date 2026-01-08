// Package benchmark provides a client for benchmark operations via the Admin API.
package benchmark

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// Client provides benchmark operations
type Client struct {
	api *api.Client
}

// NewClient creates a new benchmark client
func NewClient(apiClient *api.Client) *Client {
	return &Client{api: apiClient}
}

// ============================================================================
// Scenario Operations
// ============================================================================

// ListScenarios returns all available benchmark scenarios
func (c *Client) ListScenarios(ctx context.Context) ([]Scenario, error) {
	var resp ListScenariosResponse

	if err := c.api.Get(ctx, "/v1/benchmarks/scenarios", &resp); err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}

	return resp.Scenarios, nil
}

// GetScenario returns a scenario by name
func (c *Client) GetScenario(ctx context.Context, name string) (*Scenario, error) {
	var scenario Scenario

	path := fmt.Sprintf("/v1/benchmarks/scenarios/%s", url.PathEscape(name))
	if err := c.api.Get(ctx, path, &scenario); err != nil {
		return nil, fmt.Errorf("get scenario: %w", err)
	}

	return &scenario, nil
}

// SyncScenarios syncs scenarios to the Admin API
func (c *Client) SyncScenarios(ctx context.Context, scenarios []ScenarioUpsert, deleteOrphans bool) (*SyncScenariosResponse, error) {
	req := SyncScenariosRequest{
		Scenarios:     scenarios,
		DeleteOrphans: deleteOrphans,
	}

	var resp SyncScenariosResponse
	if err := c.api.Post(ctx, "/v1/benchmarks/scenarios/sync", req, &resp); err != nil {
		return nil, fmt.Errorf("sync scenarios: %w", err)
	}
	return &resp, nil
}

// ============================================================================
// Target Operations
// ============================================================================

// ListTargets returns benchmark targets with optional filtering
func (c *Client) ListTargets(ctx context.Context, opts ListTargetsOptions) ([]Target, error) {
	// Build query parameters
	params := url.Values{}
	if opts.Environment != "" {
		params.Set("environment", opts.Environment)
	}
	if opts.ModelName != "" {
		params.Set("model_name", opts.ModelName)
	}
	if opts.ScenarioName != "" {
		params.Set("scenario_name", opts.ScenarioName)
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}
	if opts.OrgID != "" {
		params.Set("org_id", opts.OrgID)
	}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		params.Set("offset", strconv.Itoa(opts.Offset))
	}

	path := "/v1/benchmarks/targets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp ListTargetsResponse
	if err := c.api.Get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}

	return resp.Targets, nil
}

// CreateTarget creates a new benchmark target
func (c *Client) CreateTarget(ctx context.Context, req CreateTargetRequest) (*Target, error) {
	var target Target

	if err := c.api.Post(ctx, "/v1/benchmarks/targets", req, &target); err != nil {
		return nil, fmt.Errorf("create target: %w", err)
	}

	return &target, nil
}

// ResolveTargetID resolves a target name or ID to a target ID
// If the input looks like a UUID, it returns it as-is
// Otherwise, it looks up the target by name
func (c *Client) ResolveTargetID(ctx context.Context, nameOrID string) (string, error) {
	// Check if it looks like a UUID (simple heuristic: contains dashes and is ~36 chars)
	if len(nameOrID) == 36 && nameOrID[8] == '-' && nameOrID[13] == '-' {
		return nameOrID, nil
	}

	// Look up by name
	targets, err := c.ListTargets(ctx, ListTargetsOptions{})
	if err != nil {
		return "", fmt.Errorf("list targets: %w", err)
	}

	for _, t := range targets {
		if t.Name == nameOrID {
			return t.ID, nil
		}
	}

	return "", fmt.Errorf("target not found: %s", nameOrID)
}

// GetTarget returns a target by ID
func (c *Client) GetTarget(ctx context.Context, id string) (*Target, error) {
	var target Target

	path := fmt.Sprintf("/v1/benchmarks/targets/%s", url.PathEscape(id))
	if err := c.api.Get(ctx, path, &target); err != nil {
		return nil, fmt.Errorf("get target: %w", err)
	}

	return &target, nil
}

// GetTargetByName returns a target by name and environment
func (c *Client) GetTargetByName(ctx context.Context, name, environment string) (*Target, error) {
	var target Target

	params := url.Values{}
	params.Set("environment", environment)
	path := fmt.Sprintf("/v1/benchmarks/targets/by-name/%s?%s", url.PathEscape(name), params.Encode())

	if err := c.api.Get(ctx, path, &target); err != nil {
		return nil, fmt.Errorf("get target by name: %w", err)
	}

	return &target, nil
}

// UpdateTarget updates a benchmark target
func (c *Client) UpdateTarget(ctx context.Context, id string, req UpdateTargetRequest) (*Target, error) {
	var target Target

	path := fmt.Sprintf("/v1/benchmarks/targets/%s", url.PathEscape(id))
	if err := c.api.Put(ctx, path, req, &target); err != nil {
		return nil, fmt.Errorf("update target: %w", err)
	}

	return &target, nil
}

// DeleteTarget removes a benchmark target
func (c *Client) DeleteTarget(ctx context.Context, id string) error {
	path := fmt.Sprintf("/v1/benchmarks/targets/%s", url.PathEscape(id))
	if err := c.api.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	return nil
}

// StartTarget starts benchmarking for a target via the Admin API
// This calls the /start endpoint which registers the target with guidellm-runner
func (c *Client) StartTarget(ctx context.Context, id string) (*Target, error) {
	var target Target
	path := fmt.Sprintf("/v1/benchmarks/targets/%s/start", url.PathEscape(id))
	if err := c.api.Post(ctx, path, nil, &target); err != nil {
		return nil, fmt.Errorf("start target: %w", err)
	}
	return &target, nil
}

// StopTarget stops benchmarking for a target via the Admin API
// This calls the /stop endpoint which removes the target from guidellm-runner
func (c *Client) StopTarget(ctx context.Context, id string) (*Target, error) {
	var target Target
	path := fmt.Sprintf("/v1/benchmarks/targets/%s/stop", url.PathEscape(id))
	if err := c.api.Post(ctx, path, nil, &target); err != nil {
		return nil, fmt.Errorf("stop target: %w", err)
	}
	return &target, nil
}

// ============================================================================
// Run Operations
// ============================================================================

// ListRuns returns benchmark runs with optional filtering
func (c *Client) ListRuns(ctx context.Context, opts ListRunsOptions) ([]Run, error) {
	// Build query parameters
	params := url.Values{}
	if opts.TargetID != "" {
		params.Set("target_id", opts.TargetID)
	}
	if opts.ScenarioName != "" {
		params.Set("scenario_name", opts.ScenarioName)
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}
	if opts.TriggeredBy != "" {
		params.Set("triggered_by", opts.TriggeredBy)
	}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		params.Set("offset", strconv.Itoa(opts.Offset))
	}

	path := "/v1/benchmarks/runs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp ListRunsResponse
	if err := c.api.Get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	return resp.Runs, nil
}

// GetRun returns a run by ID
func (c *Client) GetRun(ctx context.Context, id string) (*Run, error) {
	var run Run

	path := fmt.Sprintf("/v1/benchmarks/runs/%s", url.PathEscape(id))
	if err := c.api.Get(ctx, path, &run); err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}

	return &run, nil
}

// TriggerRun triggers a manual benchmark run for a target
func (c *Client) TriggerRun(ctx context.Context, targetID string) (*Run, error) {
	var run Run

	req := TriggerRunRequest{
		TriggeredBy: "manual",
	}

	path := fmt.Sprintf("/v1/benchmarks/targets/%s/trigger", url.PathEscape(targetID))
	if err := c.api.Post(ctx, path, req, &run); err != nil {
		return nil, fmt.Errorf("trigger run: %w", err)
	}

	return &run, nil
}

// ============================================================================
// Status Operations
// ============================================================================

// GetStatus returns an overall benchmark status summary
func (c *Client) GetStatus(ctx context.Context, environment string) (*StatusSummary, error) {
	path := "/v1/benchmarks/status"
	if environment != "" {
		path += "?environment=" + url.QueryEscape(environment)
	}

	var summary StatusSummary
	if err := c.api.Get(ctx, path, &summary); err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}

	return &summary, nil
}

// ============================================================================
// Scheduler Control Operations (guidellm-runner daemon)
// ============================================================================

// PauseScheduler pauses the benchmark scheduler
func (c *Client) PauseScheduler(ctx context.Context) error {
	if err := c.api.Post(ctx, "/v1/benchmark/pause", nil, nil); err != nil {
		return fmt.Errorf("pause scheduler: %w", err)
	}
	return nil
}

// ResumeScheduler resumes the benchmark scheduler
func (c *Client) ResumeScheduler(ctx context.Context) error {
	if err := c.api.Post(ctx, "/v1/benchmark/resume", nil, nil); err != nil {
		return fmt.Errorf("resume scheduler: %w", err)
	}
	return nil
}

// TriggerManualRun triggers a manual benchmark run (auto-pauses scheduler)
func (c *Client) TriggerManualRun(ctx context.Context, model string) error {
	req := map[string]interface{}{}
	if model != "" {
		req["model"] = model
	}
	if err := c.api.Post(ctx, "/v1/benchmark/run", req, nil); err != nil {
		return fmt.Errorf("trigger manual run: %w", err)
	}
	return nil
}

// GetSchedulerStatus returns the scheduler status
func (c *Client) GetSchedulerStatus(ctx context.Context) (*SchedulerStatus, error) {
	var status SchedulerStatus
	if err := c.api.Get(ctx, "/v1/benchmark/status", &status); err != nil {
		return nil, fmt.Errorf("get scheduler status: %w", err)
	}
	return &status, nil
}
