// Package pod provides client for pod health operations via Admin API.
package pod

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// Client provides pod health operations
type Client struct {
	api *api.Client
}

// NewClient creates a new pod client
func NewClient(apiClient *api.Client) *Client {
	return &Client{api: apiClient}
}

// PodHealth represents health information for a single pod
type PodHealth struct {
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	ModelName         string             `json:"model_name"`
	InferenceService  string             `json:"inferenceservice"`
	Phase             string             `json:"phase"`
	Ready             bool               `json:"ready"`
	Node              string             `json:"node"`
	RestartCount      int                `json:"restart_count"`
	LastRestartTime   *time.Time         `json:"last_restart_time,omitempty"`
	LastTermination   *TerminationInfo   `json:"last_termination,omitempty"`
	Containers        []ContainerHealth  `json:"containers"`
	Conditions        []PodCondition     `json:"conditions"`
	AgeSeconds        int64              `json:"age_seconds"`
	CreatedAt         time.Time          `json:"created_at"`
}

// TerminationInfo contains details about the last container termination
type TerminationInfo struct {
	Reason     string     `json:"reason"`
	ExitCode   int        `json:"exit_code"`
	Message    string     `json:"message"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// ContainerHealth represents health information for a container
type ContainerHealth struct {
	Name         string              `json:"name"`
	State        string              `json:"state"`
	Ready        bool                `json:"ready"`
	RestartCount int                 `json:"restart_count"`
	Resources    *ContainerResources `json:"resources,omitempty"`
}

// ContainerResources represents container resource requests and limits
type ContainerResources struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// PodCondition represents a Kubernetes pod condition
type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// HealthSummary provides aggregate statistics
type HealthSummary struct {
	TotalPods        int `json:"total_pods"`
	HealthyPods      int `json:"healthy_pods"`
	UnhealthyPods    int `json:"unhealthy_pods"`
	TotalRestarts    int `json:"total_restarts"`
	PodsWithRestarts int `json:"pods_with_restarts"`
}

// HealthResponse represents the API response for pod health
type HealthResponse struct {
	Pods    []PodHealth   `json:"pods"`
	Summary HealthSummary `json:"summary"`
}

// HealthOptions configures the health query
type HealthOptions struct {
	Namespace      string
	ModelName      string
	Environment    string
	IncludeHealthy bool
}

// GetHealth retrieves pod health information
func (c *Client) GetHealth(ctx context.Context, opts HealthOptions) (*HealthResponse, error) {
	// Build query parameters
	params := url.Values{}
	if opts.Namespace != "" {
		params.Set("namespace", opts.Namespace)
	}
	if opts.ModelName != "" {
		params.Set("model_name", opts.ModelName)
	}
	if opts.Environment != "" {
		params.Set("environment", opts.Environment)
	}
	params.Set("include_healthy", fmt.Sprintf("%t", opts.IncludeHealthy))

	path := "/v1/pods/health"
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	var resp HealthResponse
	if err := c.api.Get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("get pod health: %w", err)
	}

	return &resp, nil
}
