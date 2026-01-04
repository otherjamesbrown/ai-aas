package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListBenchmarkScenarios lists all benchmark scenarios.
func (c *Client) ListBenchmarkScenarios(ctx context.Context, limit, offset int) (*ListBenchmarkScenariosResponse, error) {
	path := "/v1/benchmarks/scenarios"
	if limit > 0 || offset > 0 {
		params := url.Values{}
		if limit > 0 {
			params.Set("limit", strconv.Itoa(limit))
		}
		if offset > 0 {
			params.Set("offset", strconv.Itoa(offset))
		}
		path += "?" + params.Encode()
	}

	var result ListBenchmarkScenariosResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBenchmarkScenario retrieves a benchmark scenario by name.
func (c *Client) GetBenchmarkScenario(ctx context.Context, name string) (*BenchmarkScenario, error) {
	var result BenchmarkScenario
	if err := c.doRequest(ctx, http.MethodGet, "/v1/benchmarks/scenarios/"+url.PathEscape(name), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBenchmarkTargets lists benchmark targets with optional filters.
func (c *Client) ListBenchmarkTargets(ctx context.Context, opts ListBenchmarkTargetsOptions) (*ListBenchmarkTargetsResponse, error) {
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

	var result ListBenchmarkTargetsResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateBenchmarkTarget creates a new benchmark target.
func (c *Client) CreateBenchmarkTarget(ctx context.Context, req CreateBenchmarkTargetRequest) (*BenchmarkTarget, error) {
	var result BenchmarkTarget
	if err := c.doRequest(ctx, http.MethodPost, "/v1/benchmarks/targets", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBenchmarkTarget retrieves a benchmark target by ID.
func (c *Client) GetBenchmarkTarget(ctx context.Context, id string) (*BenchmarkTarget, error) {
	var result BenchmarkTarget
	if err := c.doRequest(ctx, http.MethodGet, "/v1/benchmarks/targets/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateBenchmarkTarget updates a benchmark target.
func (c *Client) UpdateBenchmarkTarget(ctx context.Context, id string, req UpdateBenchmarkTargetRequest) (*BenchmarkTarget, error) {
	var result BenchmarkTarget
	if err := c.doRequest(ctx, http.MethodPatch, "/v1/benchmarks/targets/"+url.PathEscape(id), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteBenchmarkTarget deletes a benchmark target.
func (c *Client) DeleteBenchmarkTarget(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/v1/benchmarks/targets/"+url.PathEscape(id), nil, nil)
}

// StartBenchmarkTarget starts a benchmark target.
func (c *Client) StartBenchmarkTarget(ctx context.Context, id string) (*BenchmarkTarget, error) {
	var result BenchmarkTarget
	if err := c.doRequest(ctx, http.MethodPost, "/v1/benchmarks/targets/"+url.PathEscape(id)+"/start", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// StopBenchmarkTarget stops a benchmark target.
func (c *Client) StopBenchmarkTarget(ctx context.Context, id string) (*BenchmarkTarget, error) {
	var result BenchmarkTarget
	if err := c.doRequest(ctx, http.MethodPost, "/v1/benchmarks/targets/"+url.PathEscape(id)+"/stop", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TriggerBenchmarkRun triggers a benchmark run for the specified target.
func (c *Client) TriggerBenchmarkRun(ctx context.Context, targetID string, req *TriggerBenchmarkRunRequest) (*BenchmarkRun, error) {
	var result BenchmarkRun
	var body interface{}
	if req != nil {
		body = req
	}
	if err := c.doRequest(ctx, http.MethodPost, "/v1/benchmarks/targets/"+url.PathEscape(targetID)+"/trigger", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBenchmarkRuns lists benchmark runs with optional filters.
func (c *Client) ListBenchmarkRuns(ctx context.Context, opts ListBenchmarkRunsOptions) (*ListBenchmarkRunsResponse, error) {
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

	var result ListBenchmarkRunsResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBenchmarkRun retrieves a benchmark run by ID.
func (c *Client) GetBenchmarkRun(ctx context.Context, id string) (*BenchmarkRun, error) {
	var result BenchmarkRun
	if err := c.doRequest(ctx, http.MethodGet, "/v1/benchmarks/runs/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelBenchmarkRun cancels a pending or running benchmark run (UC-BM-006).
func (c *Client) CancelBenchmarkRun(ctx context.Context, id string) (*BenchmarkRun, error) {
	var result BenchmarkRun
	if err := c.doRequest(ctx, http.MethodPost, "/v1/benchmarks/runs/"+url.PathEscape(id)+"/cancel", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBenchmarkTargetByName finds a target by name among the user's targets.
// Returns nil if not found (no error).
func (c *Client) GetBenchmarkTargetByName(ctx context.Context, name string) (*BenchmarkTarget, error) {
	// List targets and find by name
	resp, err := c.ListBenchmarkTargets(ctx, ListBenchmarkTargetsOptions{Limit: 500})
	if err != nil {
		return nil, err
	}
	for _, t := range resp.Targets {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, nil
}

// ResolveBenchmarkTargetID resolves a target name or ID to a target ID.
// If the input is a valid UUID, it returns it directly.
// Otherwise, it looks up the target by name.
func (c *Client) ResolveBenchmarkTargetID(ctx context.Context, nameOrID string) (string, error) {
	// If it looks like a UUID, try to get it directly
	if len(nameOrID) == 36 && nameOrID[8] == '-' && nameOrID[13] == '-' && nameOrID[18] == '-' && nameOrID[23] == '-' {
		target, err := c.GetBenchmarkTarget(ctx, nameOrID)
		if err == nil && target != nil {
			return target.ID, nil
		}
		// Fall through to try as name
	}

	// Try to find by name
	target, err := c.GetBenchmarkTargetByName(ctx, nameOrID)
	if err != nil {
		return "", err
	}
	if target == nil {
		return "", fmt.Errorf("benchmark target not found: %s", nameOrID)
	}
	return target.ID, nil
}
