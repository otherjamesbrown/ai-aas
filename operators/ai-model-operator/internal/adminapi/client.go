// Package adminapi provides a client for interacting with the Admin API.
package adminapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// Retry configuration for rate limiting (429 responses)
	maxRetries     = 5
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	backoffFactor  = 2
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

// calculateBackoff calculates the exponential backoff duration for retry attempts
func calculateBackoff(attempt int, initial, max time.Duration) time.Duration {
	backoff := initial
	for i := 0; i < attempt; i++ {
		backoff *= backoffFactor
		if backoff > max {
			return max
		}
	}
	return backoff
}

// min returns the minimum of two time.Duration values
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// deploymentStatusDTO represents the status of a deployment for API transfer
type deploymentStatusDTO struct {
	Status               string     `json:"status"`
	ModelID              string     `json:"model_id,omitempty"`
	ExternalName         string     `json:"external_name,omitempty"`
	InferenceServiceName string     `json:"inferenceservice_name,omitempty"`
	Endpoint             string     `json:"endpoint,omitempty"`
	ReplicasReady        int        `json:"replicas_ready,omitempty"`
	LastHealthCheckAt    *time.Time `json:"last_health_check_at,omitempty"`
	LastHealthStatus     string     `json:"last_health_status,omitempty"`
}

// createDeploymentRequestDTO contains data for creating a deployment
type createDeploymentRequestDTO struct {
	ModelName    string `json:"model_name"`
	ModelID      string `json:"model_id,omitempty"`
	ExternalName string `json:"external_name,omitempty"`
	Environment  string `json:"environment"`
	Namespace    string `json:"namespace"`
	Replicas     int    `json:"replicas,omitempty"`
	ModelType    string `json:"model_type,omitempty"`
}

// CreateDeploymentRequest contains data for creating a deployment
type CreateDeploymentRequest struct {
	ModelName    string
	ModelID      string // Full model path (e.g., "unsloth/gpt-oss-20b")
	ExternalName string // Name exposed in OpenAI-compatible APIs
	Environment  string
	Namespace    string
	Replicas     int
	ModelType    string // Model type (text, vision-language, embedding, audio)
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	Status               string
	ModelID              string // Full model path (e.g., "unsloth/gpt-oss-20b")
	ExternalName         string // Name exposed in OpenAI-compatible APIs
	InferenceServiceName string
	Endpoint             string
	ReplicasReady        int
}

// Backend represents a backend in a routing policy
type Backend struct {
	BackendID string `json:"backend_id"`
	Weight    int    `json:"weight"`
}

// RoutingPolicyCreate represents a request to create a routing policy
type RoutingPolicyCreate struct {
	PolicyID       string    `json:"policy_id,omitempty"`
	OrganizationID string    `json:"organization_id,omitempty"`
	Model          string    `json:"model"`
	Backends       []Backend `json:"backends"`
	Enabled        *bool     `json:"enabled,omitempty"`
}

// RoutingPolicy represents a routing policy response
type RoutingPolicy struct {
	PolicyID       string    `json:"policy_id"`
	OrganizationID string    `json:"organization_id"`
	Model          string    `json:"model"`
	Backends       []Backend `json:"backends"`
	Enabled        bool      `json:"enabled"`
}

// RoutingPolicyListResponse represents a paginated list of routing policies
type RoutingPolicyListResponse struct {
	Policies []RoutingPolicy `json:"policies"`
}

// CreateDeployment creates a new deployment record
func (c *Client) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) error {
	dto := createDeploymentRequestDTO{
		ModelName:    req.ModelName,
		ModelID:      req.ModelID,
		ExternalName: req.ExternalName,
		Environment:  req.Environment,
		Namespace:    req.Namespace,
		Replicas:     req.Replicas,
		ModelType:    req.ModelType,
	}
	return c.request(ctx, http.MethodPost, "/v1/deployments", dto, nil)
}

// UpdateDeploymentStatus updates a deployment's status
func (c *Client) UpdateDeploymentStatus(ctx context.Context, modelName, environment string, status DeploymentStatus) error {
	dto := deploymentStatusDTO{
		Status:               status.Status,
		ModelID:              status.ModelID,
		ExternalName:         status.ExternalName,
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

// ListRoutingPolicies retrieves routing policies for a specific model and organization
func (c *Client) ListRoutingPolicies(ctx context.Context, model, organizationID string) (*RoutingPolicyListResponse, error) {
	path := fmt.Sprintf("/v1/routing/policies?model=%s&organization_id=%s", model, organizationID)
	var result RoutingPolicyListResponse
	if err := c.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateRoutingPolicy creates a new routing policy
func (c *Client) CreateRoutingPolicy(ctx context.Context, policy RoutingPolicyCreate) (*RoutingPolicy, error) {
	var result RoutingPolicy
	if err := c.request(ctx, http.MethodPost, "/v1/routing/policies", policy, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// request performs an HTTP request to the Admin API with retry logic for rate limiting
func (c *Client) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	url := c.baseURL + path

	// Marshal body once (will be reused on retries)
	var bodyData []byte
	if body != nil {
		var err error
		bodyData, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create new request for each attempt (body reader must be fresh)
		var bodyReader io.Reader
		if bodyData != nil {
			bodyReader = bytes.NewReader(bodyData)
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

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		// Handle rate limiting (429)
		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited (429): %s", string(respBody))

			// Max retries exceeded
			if attempt >= maxRetries {
				return fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
			}

			// Calculate backoff duration
			backoff := calculateBackoff(attempt, initialBackoff, maxBackoff)

			// Check for Retry-After header (RFC 6585)
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					backoff = min(time.Duration(seconds)*time.Second, maxBackoff)
				}
			}

			// Wait before retry
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
			case <-time.After(backoff):
				// Continue to next attempt
				continue
			}
		}

		// Handle other errors
		if resp.StatusCode >= 400 {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		// Success - decode response if needed
		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}

		return nil
	}

	// Should not reach here, but safeguard
	return fmt.Errorf("unexpected retry loop exit: %w", lastErr)
}
