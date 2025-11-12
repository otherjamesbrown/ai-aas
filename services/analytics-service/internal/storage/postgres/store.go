// Package postgres provides TimescaleDB-backed persistence for the analytics service.
//
// Purpose:
//
//	This package provides data access methods for usage events, ingestion batches,
//	and freshness status. It uses pgxpool for connection pooling and supports
//	TimescaleDB hypertables.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides Postgres-backed persistence for analytics data.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a store using the provided connection string.
func NewStore(ctx context.Context, connString string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close closes the underlying pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Pool exposes the underlying pgx pool.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// InsertUsageEvents inserts usage events in a batch with deduplication.
func (s *Store) InsertUsageEvents(ctx context.Context, events []UsageEvent, batchID uuid.UUID) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	// Use COPY for efficient batch insert, then handle deduplication
	// For now, use individual inserts with ON CONFLICT for simplicity
	// TODO: Optimize with COPY + CTE for better performance
	query := `
		INSERT INTO analytics.usage_events (
			event_id, org_id, occurred_at, received_at, model_id, actor_id,
			input_tokens, output_tokens, latency_ms, status, error_code,
			cost_estimate_cents, metadata, batch_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (event_id, org_id) DO NOTHING
	`

	inserted := 0
	for _, e := range events {
		var modelID, actorID *uuid.UUID
		if e.ModelID != uuid.Nil {
			modelID = &e.ModelID
		}
		if e.ActorID != uuid.Nil {
			actorID = &e.ActorID
		}

		var errorCode *string
		if e.ErrorCode != "" {
			errorCode = &e.ErrorCode
		}

		metadataJSON, err := json.Marshal(e.Metadata)
		if err != nil {
			metadataJSON = []byte("{}")
		}

		ct, err := s.pool.Exec(ctx, query,
			e.EventID, e.OrgID, e.OccurredAt, e.ReceivedAt,
			modelID, actorID,
			e.InputTokens, e.OutputTokens, e.LatencyMS,
			e.Status, errorCode,
			e.CostEstimateCents, string(metadataJSON), batchID,
		)
		if err != nil {
			return inserted, fmt.Errorf("insert usage event: %w", err)
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}

	return inserted, nil
}

// UsageEvent represents a usage event for insertion.
type UsageEvent struct {
	EventID           uuid.UUID
	OrgID             uuid.UUID
	OccurredAt        time.Time
	ReceivedAt        time.Time
	ModelID           uuid.UUID
	ActorID           uuid.UUID
	InputTokens       int64
	OutputTokens      int64
	LatencyMS         int
	Status            string
	ErrorCode         string
	CostEstimateCents float64
	Metadata          map[string]interface{}
}

// CreateIngestionBatch creates a new ingestion batch record.
func (s *Store) CreateIngestionBatch(ctx context.Context, offset int64, orgScope []uuid.UUID) (uuid.UUID, error) {
	batchID := uuid.New()
	query := `
		INSERT INTO analytics.ingestion_batches (
			batch_id, stream_offset, org_scope, started_at
		) VALUES ($1, $2, $3, NOW())
		RETURNING batch_id
	`
	err := s.pool.QueryRow(ctx, query, batchID, offset, orgScope).Scan(&batchID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create ingestion batch: %w", err)
	}
	return batchID, nil
}

// CompleteIngestionBatch marks a batch as completed with dedupe stats.
func (s *Store) CompleteIngestionBatch(ctx context.Context, batchID uuid.UUID, dedupeConflicts int) error {
	query := `
		UPDATE analytics.ingestion_batches
		SET completed_at = NOW(), dedupe_conflicts = $2
		WHERE batch_id = $1
	`
	_, err := s.pool.Exec(ctx, query, batchID, dedupeConflicts)
	return err
}
