// Package analytics provides types for analytics-service API client.
//
// Purpose:
//
//	Define request and response types for analytics-service API operations.
//
// Requirements Reference:
//   - specs/009-admin-cli/plan.md#client/analytics/types
//
package analytics

import "time"

// ExportUsageRequest is a placeholder type for Phase 5 implementation.
type ExportUsageRequest struct {
	// TODO: Implement in Phase 5
}

// UsageQueryParams represents parameters for usage queries.
type UsageQueryParams struct {
	OrgID       string
	Start       time.Time
	End         time.Time
	Granularity string // "hour" or "day"
	ModelID     string // Optional filter
}

// UsageSeriesPoint represents a single usage data point in a time series.
type UsageSeriesPoint struct {
	BucketStart       string `json:"bucketStart"`
	Invocations       int64  `json:"invocations"`
	InputTokens       int64  `json:"inputTokens"`
	OutputTokens      int64  `json:"outputTokens"`
	CostEstimateCents int64  `json:"costEstimateCents"`
}

// UsageTotals represents aggregated usage totals.
type UsageTotals struct {
	Invocations       int64 `json:"invocations"`
	InputTokens       int64 `json:"inputTokens"`
	OutputTokens      int64 `json:"outputTokens"`
	CostEstimateCents int64 `json:"costEstimateCents"`
}

// DataFreshness represents data freshness/lag information.
type DataFreshness struct {
	Status     string `json:"status"`     // "fresh", "stale", "unknown"
	LagSeconds int64  `json:"lagSeconds"` // How many seconds behind real-time
}

// UsageDataResponse represents usage query response.
type UsageDataResponse struct {
	OrgID       string             `json:"orgId"`
	Start       time.Time          `json:"start"`
	End         time.Time          `json:"end"`
	Granularity string             `json:"granularity"`
	Series      []UsageSeriesPoint `json:"series"`
	Totals      UsageTotals        `json:"totals"`
	Freshness   DataFreshness      `json:"freshness"`
}

