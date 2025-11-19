// Package routing provides backend selection and request forwarding logic.
//
// Purpose:
//   This package implements the routing engine that selects backends based on
//   configured policies, health status, and organization-level rules. It also
//   provides client wrappers for forwarding requests to backend services.
//
// Dependencies:
//   - internal/config: Routing policy configuration
//
// Key Responsibilities:
//   - Select backend based on routing policies
//   - Forward requests to backend services
//   - Handle timeouts and retries
//   - Track backend health and latency
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#FR-003 (Routing engine)
//
package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

)

// BackendEndpoint represents a backend model service endpoint.
type BackendEndpoint struct {
	ID        string
	URI       string
	ModelVariant string
	Timeout   time.Duration
}

// BackendClient wraps HTTP client for backend communication.
type BackendClient struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewBackendClient creates a new backend client.
func NewBackendClient(logger *zap.Logger, timeout time.Duration) *BackendClient {
	return &BackendClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// BackendRequest represents a request to a backend model service.
type BackendRequest struct {
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// BackendResponse represents a response from a backend model service.
type BackendResponse struct {
	Text       string                 `json:"text"`
	TokensUsed int                    `json:"tokens_used"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ForwardRequest forwards an inference request to a backend and returns the response.
func (c *BackendClient) ForwardRequest(ctx context.Context, backend *BackendEndpoint, req *BackendRequest) (*BackendResponse, error) {
	startTime := time.Now()

	// Prepare request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", backend.URI, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("backend request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(startTime)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var backendResp struct {
		Text       string                 `json:"text"`
		TokensUsed int                    `json:"tokens_used"`
		Metadata   map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := json.Unmarshal(body, &backendResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.logger.Info("backend request completed",
		zap.String("backend_id", backend.ID),
		zap.Duration("latency", latency),
		zap.Int("tokens_used", backendResp.TokensUsed),
	)

	return &BackendResponse{
		Text:       backendResp.Text,
		TokensUsed: backendResp.TokensUsed,
		Metadata:   backendResp.Metadata,
	}, nil
}

// HealthCheck checks the health of a backend endpoint.
func (c *BackendClient) HealthCheck(ctx context.Context, backend *BackendEndpoint) error {
	healthURL := backend.URI
	// Try /health endpoint if URI doesn't end with it
	if len(healthURL) > 0 && healthURL[len(healthURL)-1] != '/' {
		healthURL += "/health"
	} else {
		healthURL += "health"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

