// Package analytics provides a client for the analytics service.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is a client for the analytics service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new analytics client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UsagePoint represents a single usage data point.
type UsagePoint struct {
	BucketStart       time.Time `json:"bucketStart"`
	ModelID           *string   `json:"modelId,omitempty"`
	Invocations       int64     `json:"invocations"`
	InputTokens       int64     `json:"inputTokens"`
	OutputTokens      int64     `json:"outputTokens"`
	CostEstimateCents int64     `json:"costEstimateCents"`
}

// UsageTotals represents aggregated usage totals.
type UsageTotals struct {
	Invocations       int64 `json:"invocations"`
	InputTokens       int64 `json:"inputTokens"`
	OutputTokens      int64 `json:"outputTokens"`
	CostEstimateCents int64 `json:"costEstimateCents"`
}

// FreshnessIndicator represents freshness status.
type FreshnessIndicator struct {
	Status       string    `json:"status"`
	LagSeconds   int       `json:"lagSeconds"`
	LastEventAt  time.Time `json:"lastEventAt"`
	LastRollupAt time.Time `json:"lastRollupAt"`
}

// UsageResponse represents the response from the analytics service.
type UsageResponse struct {
	OrgID       string               `json:"orgId"`
	Granularity string               `json:"granularity"`
	Series      []UsagePoint         `json:"series"`
	Totals      UsageTotals          `json:"totals"`
	Freshness   FreshnessIndicator   `json:"freshness"`
}

// GetUsage retrieves usage data for an organization.
func (c *Client) GetUsage(ctx context.Context, orgID string, start, end time.Time, granularity string, apiKey string) (*UsageResponse, error) {
	// Build URL with query parameters
	u, err := url.Parse(fmt.Sprintf("%s/analytics/v1/orgs/%s/usage", c.baseURL, url.PathEscape(orgID)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("start", start.Format(time.RFC3339))
	q.Set("end", end.Format(time.RFC3339))
	q.Set("granularity", granularity)
	u.RawQuery = q.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("analytics service returned status %d", resp.StatusCode)
	}

	// Parse response
	var result UsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
