// Package api provides the HTTP client for communicating with the Admin API.
package api

import (
	"context"
	"fmt"
	"time"
)

// PullJob represents a server-side pull job
type PullJob struct {
	ID             string     `json:"id"`
	ModelName      string     `json:"model_name"`
	Status         string     `json:"status"` // pending, downloading, uploading, complete, failed, cancelled
	Progress       float64    `json:"progress"`
	BytesTotal     int64      `json:"bytes_total,omitempty"`
	BytesCompleted int64      `json:"bytes_completed,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Error          string     `json:"error,omitempty"`
}

// PullJobList contains a list of pull jobs
type PullJobList struct {
	Jobs []PullJob `json:"jobs"`
}

// PullOptions contains options for creating a pull job
type PullOptions struct {
	Revision string `json:"revision,omitempty"`
}

// CreatePullJob creates a new server-side pull job
func (c *Client) CreatePullJob(ctx context.Context, modelName string, opts PullOptions) (*PullJob, error) {
	path := fmt.Sprintf("/v1/models/%s/pull", modelName)

	var job PullJob
	if err := c.Request(ctx, "POST", path, opts, &job); err != nil {
		return nil, fmt.Errorf("create pull job: %w", err)
	}

	return &job, nil
}

// GetPullJob gets a specific pull job by ID
func (c *Client) GetPullJob(ctx context.Context, modelName, jobID string) (*PullJob, error) {
	path := fmt.Sprintf("/v1/models/%s/pull/%s", modelName, jobID)

	var job PullJob
	if err := c.Request(ctx, "GET", path, nil, &job); err != nil {
		return nil, fmt.Errorf("get pull job: %w", err)
	}

	return &job, nil
}

// ListPullJobs lists all pull jobs for a model
func (c *Client) ListPullJobs(ctx context.Context, modelName string) ([]PullJob, error) {
	path := fmt.Sprintf("/v1/models/%s/pull", modelName)

	var result PullJobList
	if err := c.Request(ctx, "GET", path, nil, &result); err != nil {
		return nil, fmt.Errorf("list pull jobs: %w", err)
	}

	return result.Jobs, nil
}

// CancelPullJob cancels a running pull job
func (c *Client) CancelPullJob(ctx context.Context, modelName, jobID string) error {
	path := fmt.Sprintf("/v1/models/%s/pull/%s", modelName, jobID)

	if err := c.Request(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("cancel pull job: %w", err)
	}

	return nil
}
