// Package postgres provides usage data query methods.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UsagePoint represents a single data point in a usage series.
type UsagePoint struct {
	BucketStart       time.Time
	ModelID           *uuid.UUID
	Invocations       int64
	InputTokens       int64
	OutputTokens      int64
	CostEstimateCents float64
}

// UsageTotals represents aggregated totals for a time range.
type UsageTotals struct {
	Invocations       int64
	InputTokens       int64
	OutputTokens      int64
	CostEstimateCents float64
}

// GetUsageSeries retrieves usage data for an organization.
func (s *Store) GetUsageSeries(ctx context.Context, orgID uuid.UUID, start, end time.Time, granularity string, modelID *uuid.UUID) ([]UsagePoint, error) {
	var query string
	var bucketFormat string

	if granularity == "hour" {
		bucketFormat = "date_trunc('hour', bucket_start)"
		query = `
			SELECT 
				bucket_start,
				model_id,
				request_count AS invocations,
				tokens_total AS input_tokens,
				0 AS output_tokens,
				cost_total AS cost_estimate_cents
			FROM analytics_hourly_rollups
			WHERE organization_id = $1
				AND bucket_start >= $2
				AND bucket_start < $3
		`
	} else {
		bucketFormat = "date_trunc('day', bucket_start)"
		query = `
			SELECT 
				bucket_start,
				model_id,
				request_count AS invocations,
				tokens_total AS input_tokens,
				0 AS output_tokens,
				cost_total AS cost_estimate_cents
			FROM analytics_daily_rollups
			WHERE organization_id = $1
				AND bucket_start >= $2
				AND bucket_start < $3
		`
	}

	args := []interface{}{orgID, start, end}
	argIdx := 4

	if modelID != nil {
		query += fmt.Sprintf(" AND model_id = $%d", argIdx)
		args = append(args, *modelID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY bucket_start DESC, %s", bucketFormat)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage series: %w", err)
	}
	defer rows.Close()

	var points []UsagePoint
	for rows.Next() {
		var p UsagePoint
		var modelIDPtr *uuid.UUID
		err := rows.Scan(
			&p.BucketStart,
			&modelIDPtr,
			&p.Invocations,
			&p.InputTokens,
			&p.OutputTokens,
			&p.CostEstimateCents,
		)
		if err != nil {
			return nil, fmt.Errorf("scan usage point: %w", err)
		}
		p.ModelID = modelIDPtr
		points = append(points, p)
	}

	return points, rows.Err()
}

// GetUsageTotals calculates totals for a time range.
func (s *Store) GetUsageTotals(ctx context.Context, orgID uuid.UUID, start, end time.Time, modelID *uuid.UUID) (UsageTotals, error) {
	query := `
		SELECT 
			COALESCE(SUM(request_count), 0) AS invocations,
			COALESCE(SUM(tokens_total), 0) AS input_tokens,
			0 AS output_tokens,
			COALESCE(SUM(cost_total), 0) AS cost_estimate_cents
		FROM analytics_daily_rollups
		WHERE organization_id = $1
			AND bucket_start >= $2
			AND bucket_start < $3
	`

	args := []interface{}{orgID, start, end}
	if modelID != nil {
		query += " AND model_id = $4"
		args = append(args, *modelID)
	}

	var totals UsageTotals
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&totals.Invocations,
		&totals.InputTokens,
		&totals.OutputTokens,
		&totals.CostEstimateCents,
	)
	if err != nil {
		return UsageTotals{}, fmt.Errorf("query usage totals: %w", err)
	}

	return totals, nil
}

