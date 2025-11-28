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
//
package analytics

import (
	"context"
	"net/http"
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

// QueryUsage retrieves usage data for an organization.
// This is a stub implementation - backend endpoint not yet available.
func (c *Client) QueryUsage(ctx context.Context, params UsageQueryParams) (*UsageDataResponse, error) {
	// TODO: Implement when analytics-service usage endpoint is available
	// For now, return empty response indicating no data
	return &UsageDataResponse{
		OrgID:       params.OrgID,
		Start:       params.Start,
		End:         params.End,
		Granularity: params.Granularity,
		Series:      []UsageSeriesPoint{},
		Totals:      UsageTotals{},
		Freshness:   DataFreshness{Status: "stub", LagSeconds: 0},
	}, nil
}

