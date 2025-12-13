// Package adminapi provides a client for interacting with the Admin API.
package adminapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides methods for calling the Admin API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Admin API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// deploymentStatusDTO represents the status of a deployment for API transfer
type deploymentStatusDTO struct {
	Status               string     `json:"status"`
	InferenceServiceName string     `json:"inferenceservice_name,omitempty"`
	Endpoint             string     `json:"endpoint,omitempty"`
	ReplicasReady        int        `json:"replicas_ready,omitempty"`
	LastHealthCheckAt    *time.Time `json:"last_health_check_at,omitempty"`
	LastHealthStatus     string     `json:"last_health_status,omitempty"`
}

// createDeploymentRequestDTO contains data for creating a deployment
type createDeploymentRequestDTO struct {
	ModelName   string `json:"model_name"`
	Environment string `json:"environment"`
	Namespace   string `json:"namespace"`
	Replicas    int    `json:"replicas,omitempty"`
}

// CreateDeploymentRequest contains data for creating a deployment
type CreateDeploymentRequest struct {
	ModelName   string
	Environment string
	Namespace   string
	Replicas    int
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	Status               string
	InferenceServiceName string
	Endpoint             string
	ReplicasReady        int
}

// CreateDeployment creates a new deployment record
func (c *Client) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) error {
	dto := createDeploymentRequestDTO{
		ModelName:   req.ModelName,
		Environment: req.Environment,
		Namespace:   req.Namespace,
		Replicas:    req.Replicas,
	}
	return c.request(ctx, http.MethodPost, "/v1/deployments", dto, nil)
}

// UpdateDeploymentStatus updates a deployment's status
func (c *Client) UpdateDeploymentStatus(ctx context.Context, modelName, environment string, status DeploymentStatus) error {
	dto := deploymentStatusDTO{
		Status:               status.Status,
		InferenceServiceName: status.InferenceServiceName,
		Endpoint:             status.Endpoint,
		ReplicasReady:        status.ReplicasReady,
	}
	path := fmt.Sprintf("/v1/deployments/%s/%s", modelName, environment)
	return c.request(ctx, http.MethodPut, path, dto, nil)
}

// DeleteDeployment removes a deployment record
func (c *Client) DeleteDeployment(ctx context.Context, modelName, environment string) error {
	path := fmt.Sprintf("/v1/deployments/%s/%s", modelName, environment)
	return c.request(ctx, http.MethodDelete, path, nil, nil)
}

// request performs an HTTP request to the Admin API
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
