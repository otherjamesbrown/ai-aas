// Package analytics provides the API client for analytics-service.
//
// Purpose:
//
//	REST client implementation for consuming analytics-service APIs. Handles
//	authentication, request/response formatting, and error handling with retry logic.
//
// Dependencies:
//   - net/http: HTTP client
//   - internal/client/retry: Retry logic with exponential backoff
//   - internal/client/analytics/types: Request/response types
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#FR-008 (consume existing service APIs)
//   - specs/009-admin-cli/plan.md#client/analytics
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client provides access to analytics-service APIs.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new analytics-service API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryUsage retrieves usage data for an organization or API key.
// If params.APIKeyID is provided, queries per-API-key usage.
func (c *Client) QueryUsage(ctx context.Context, params UsageQueryParams) (*UsageDataResponse, error) {
	// Build endpoint URL - use API key endpoint if APIKeyID is provided
	var endpoint string
	if params.APIKeyID != "" {
		endpoint = fmt.Sprintf("%s/analytics/v1/orgs/%s/apikeys/%s/usage", c.baseURL, params.OrgID, params.APIKeyID)
	} else {
		endpoint = fmt.Sprintf("%s/analytics/v1/orgs/%s/usage", c.baseURL, params.OrgID)
	}

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("start", params.Start.Format(time.RFC3339))
	queryParams.Set("end", params.End.Format(time.RFC3339))
	queryParams.Set("granularity", params.Granularity)
	if params.ModelID != "" {
		queryParams.Set("modelId", params.ModelID)
	}

	fullURL := fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add authentication header
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response UsageDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &response, nil
}
