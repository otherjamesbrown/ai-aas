// Package postgres provides reliability data query methods.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReliabilityPoint represents a single data point in a reliability series.
type ReliabilityPoint struct {
	BucketStart time.Time
	ModelID     *uuid.UUID
	ErrorRate   float64
	LatencyP50  int
	LatencyP95  int
	LatencyP99  int
}

// GetReliabilitySeries retrieves reliability data (error rates and latency percentiles) for an organization.
func (s *Store) GetReliabilitySeries(ctx context.Context, orgID uuid.UUID, start, end time.Time, granularity string, modelID *uuid.UUID) ([]ReliabilityPoint, error) {
	var bucketExpr string
	if granularity == "hour" {
		bucketExpr = "date_trunc('hour', occurred_at)"
	} else {
		bucketExpr = "date_trunc('day', occurred_at)"
	}

	query := fmt.Sprintf(`
		SELECT 
			%s AS bucket_start,
			model_id,
			CASE 
				WHEN COUNT(*) = 0 THEN 0.0
				ELSE CAST(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*)
			END AS error_rate,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY latency_ms)::INTEGER AS latency_p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms)::INTEGER AS latency_p95,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms)::INTEGER AS latency_p99
		FROM analytics.usage_events
		WHERE org_id = $1
			AND occurred_at >= $2
			AND occurred_at < $3
	`, bucketExpr)

	args := []interface{}{orgID, start, end}
	argIdx := 4

	if modelID != nil {
		query += fmt.Sprintf(" AND model_id = $%d", argIdx)
		args = append(args, *modelID)
		argIdx++
	}

	query += fmt.Sprintf(" GROUP BY %s, model_id ORDER BY bucket_start DESC", bucketExpr)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query reliability series: %w", err)
	}
	defer rows.Close()

	var points []ReliabilityPoint
	for rows.Next() {
		var p ReliabilityPoint
		var modelIDPtr *uuid.UUID
		err := rows.Scan(
			&p.BucketStart,
			&modelIDPtr,
			&p.ErrorRate,
			&p.LatencyP50,
			&p.LatencyP95,
			&p.LatencyP99,
		)
		if err != nil {
			return nil, fmt.Errorf("scan reliability point: %w", err)
		}
		p.ModelID = modelIDPtr
		points = append(points, p)
	}

	return points, rows.Err()
}

