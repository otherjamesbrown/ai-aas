// Package exports provides CSV export functionality for analytics data.
//
// Purpose:
//   This package generates CSV exports of usage events for incident analysis,
//   allowing engineers to quickly share scoped datasets during incidents.
//
package exports

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// IncidentExporter generates CSV exports of usage events for incident analysis.
type IncidentExporter struct {
	store  *postgres.Store
	logger *zap.Logger
}

// NewIncidentExporter creates a new incident exporter.
func NewIncidentExporter(store *postgres.Store, logger *zap.Logger) *IncidentExporter {
	return &IncidentExporter{
		store:  store,
		logger: logger,
	}
}

// ExportRequest specifies the parameters for an incident export.
type ExportRequest struct {
	OrgID    uuid.UUID
	ModelID  *uuid.UUID
	Start    time.Time
	End      time.Time
	MaxRows  int // Limit rows to prevent huge exports (default: 10000)
}

// Export generates a CSV export of usage events for the specified time range.
func (e *IncidentExporter) Export(ctx context.Context, req ExportRequest) ([]byte, error) {
	if req.MaxRows == 0 {
		req.MaxRows = 10000 // Default limit
	}

	// Query usage events
	query := `
		SELECT 
			event_id,
			org_id,
			occurred_at,
			received_at,
			model_id,
			actor_id,
			input_tokens,
			output_tokens,
			latency_ms,
			status,
			error_code,
			cost_estimate_cents,
			metadata
		FROM analytics.usage_events
		WHERE org_id = $1
			AND occurred_at >= $2
			AND occurred_at < $3
	`

	args := []interface{}{req.OrgID, req.Start, req.End}
	argIdx := 4

	if req.ModelID != nil {
		query += fmt.Sprintf(" AND model_id = $%d", argIdx)
		args = append(args, *req.ModelID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT $%d", argIdx)
	args = append(args, req.MaxRows)

	rows, err := e.store.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage events: %w", err)
	}
	defer rows.Close()

	// Generate CSV
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"event_id",
		"org_id",
		"occurred_at",
		"received_at",
		"model_id",
		"actor_id",
		"input_tokens",
		"output_tokens",
		"latency_ms",
		"status",
		"error_code",
		"cost_estimate_cents",
		"metadata",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header: %w", err)
	}

	// Write rows
	rowCount := 0
	for rows.Next() {
		var eventID, orgID uuid.UUID
		var occurredAt, receivedAt time.Time
		var modelID, actorID *uuid.UUID
		var inputTokens, outputTokens int64
		var latencyMS int
		var status string
		var errorCode *string
		var costEstimateCents float64
		var metadataJSON string

		err := rows.Scan(
			&eventID,
			&orgID,
			&occurredAt,
			&receivedAt,
			&modelID,
			&actorID,
			&inputTokens,
			&outputTokens,
			&latencyMS,
			&status,
			&errorCode,
			&costEstimateCents,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan usage event: %w", err)
		}

		// Format row
		row := []string{
			eventID.String(),
			orgID.String(),
			occurredAt.Format(time.RFC3339),
			receivedAt.Format(time.RFC3339),
			formatUUID(modelID),
			formatUUID(actorID),
			fmt.Sprintf("%d", inputTokens),
			fmt.Sprintf("%d", outputTokens),
			fmt.Sprintf("%d", latencyMS),
			status,
			formatString(errorCode),
			fmt.Sprintf("%.4f", costEstimateCents),
			metadataJSON,
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row: %w", err)
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV: %w", err)
	}

	e.logger.Info("generated incident export",
		zap.String("org_id", req.OrgID.String()),
		zap.Int("row_count", rowCount),
		zap.Time("start", req.Start),
		zap.Time("end", req.End),
	)

	return buf.Bytes(), nil
}

func formatUUID(u *uuid.UUID) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func formatString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

