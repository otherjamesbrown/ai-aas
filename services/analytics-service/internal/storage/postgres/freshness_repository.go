// Package postgres provides freshness status repository methods.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/freshness"
)

// GetAllFreshnessStatus retrieves all freshness status entries.
func (s *Store) GetAllFreshnessStatus(ctx context.Context) ([]*freshness.Indicator, error) {
	query := `
		SELECT org_id, model_id, last_event_at, last_rollup_at, lag_seconds, status
		FROM analytics.freshness_status
		ORDER BY org_id, model_id
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query freshness status: %w", err)
	}
	defer rows.Close()

	var indicators []*freshness.Indicator
	for rows.Next() {
		var ind freshness.Indicator
		var modelIDPtr *uuid.UUID

		err := rows.Scan(
			&ind.OrgID,
			&modelIDPtr,
			&ind.LastEventAt,
			&ind.LastRollupAt,
			&ind.LagSeconds,
			&ind.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("scan freshness indicator: %w", err)
		}

		ind.ModelID = modelIDPtr
		indicators = append(indicators, &ind)
	}

	return indicators, rows.Err()
}

// GetFreshnessStatus retrieves freshness status for a specific org/model.
func (s *Store) GetFreshnessStatus(ctx context.Context, orgID uuid.UUID, modelID *uuid.UUID) (*freshness.Indicator, error) {
	query := `
		SELECT org_id, model_id, last_event_at, last_rollup_at, lag_seconds, status
		FROM analytics.freshness_status
		WHERE org_id = $1 AND (model_id = $2 OR ($2 IS NULL AND model_id IS NULL))
	`

	var ind freshness.Indicator
	var modelIDPtr *uuid.UUID

	err := s.pool.QueryRow(ctx, query, orgID, modelID).Scan(
		&ind.OrgID,
		&modelIDPtr,
		&ind.LastEventAt,
		&ind.LastRollupAt,
		&ind.LagSeconds,
		&ind.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("get freshness status: %w", err)
	}

	ind.ModelID = modelIDPtr
	return &ind, nil
}

