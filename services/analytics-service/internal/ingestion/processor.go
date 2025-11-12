// Package ingestion provides event processing and persistence.
//
// Purpose:
//   This package processes batches of events from RabbitMQ, deduplicates them,
//   and persists them to TimescaleDB. It tracks ingestion batches and handles
//   errors gracefully.
//
package ingestion

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// Processor handles event processing and persistence.
type Processor struct {
	store  *postgres.Store
	logger *zap.Logger
}

// NewProcessor creates a new event processor.
func NewProcessor(store *postgres.Store, logger *zap.Logger) *Processor {
	return &Processor{
		store:  store,
		logger: logger,
	}
}

// ProcessBatch processes a batch of events with deduplication.
func (p *Processor) ProcessBatch(ctx context.Context, events []Event, streamOffset int64) error {
	if len(events) == 0 {
		return nil
	}

	// Extract unique org IDs for batch tracking
	orgSet := make(map[uuid.UUID]struct{})
	for _, e := range events {
		orgID, err := uuid.Parse(e.OrgID)
		if err != nil {
			p.logger.Warn("invalid org_id in event", zap.String("event_id", e.EventID), zap.Error(err))
			continue
		}
		orgSet[orgID] = struct{}{}
	}

	orgScope := make([]uuid.UUID, 0, len(orgSet))
	for orgID := range orgSet {
		orgScope = append(orgScope, orgID)
	}

	// Create ingestion batch
	batchID, err := p.store.CreateIngestionBatch(ctx, streamOffset, orgScope)
	if err != nil {
		return fmt.Errorf("create ingestion batch: %w", err)
	}

	// Convert events to database format
	dbEvents := make([]postgres.UsageEvent, 0, len(events))
	for _, e := range events {
		dbEvent, err := p.convertEvent(e)
		if err != nil {
			p.logger.Warn("skipping invalid event", zap.String("event_id", e.EventID), zap.Error(err))
			continue
		}
		dbEvents = append(dbEvents, dbEvent)
	}

	// Insert events (deduplication handled by database constraint)
	inserted, err := p.store.InsertUsageEvents(ctx, dbEvents, batchID)
	if err != nil {
		return fmt.Errorf("insert usage events: %w", err)
	}

	dedupeConflicts := len(dbEvents) - inserted

	// Mark batch as completed
	if err := p.store.CompleteIngestionBatch(ctx, batchID, dedupeConflicts); err != nil {
		return fmt.Errorf("complete ingestion batch: %w", err)
	}

	p.logger.Info("processed batch",
		zap.String("batch_id", batchID.String()),
		zap.Int("total_events", len(events)),
		zap.Int("inserted", inserted),
		zap.Int("duplicates", dedupeConflicts),
	)

	return nil
}

// convertEvent converts an Event to a postgres.UsageEvent.
func (p *Processor) convertEvent(e Event) (postgres.UsageEvent, error) {
	eventID, err := uuid.Parse(e.EventID)
	if err != nil {
		return postgres.UsageEvent{}, fmt.Errorf("invalid event_id: %w", err)
	}

	orgID, err := uuid.Parse(e.OrgID)
	if err != nil {
		return postgres.UsageEvent{}, fmt.Errorf("invalid org_id: %w", err)
	}

	var modelID uuid.UUID
	if e.ModelID != "" {
		modelID, err = uuid.Parse(e.ModelID)
		if err != nil {
			return postgres.UsageEvent{}, fmt.Errorf("invalid model_id: %w", err)
		}
	}

	var actorID uuid.UUID
	// ActorID is optional, leave as Nil if not provided

	now := time.Now()
	return postgres.UsageEvent{
		EventID:           eventID,
		OrgID:             orgID,
		OccurredAt:        e.OccurredAt,
		ReceivedAt:        now,
		ModelID:           modelID,
		ActorID:           actorID,
		InputTokens:       e.InputTokens,
		OutputTokens:      e.OutputTokens,
		LatencyMS:         e.LatencyMS,
		Status:            e.Status,
		ErrorCode:         e.ErrorCode,
		CostEstimateCents: e.CostEstimate,
		Metadata:          e.Metadata,
	}, nil
}

