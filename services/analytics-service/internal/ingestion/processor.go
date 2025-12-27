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
		orgID, err := uuid.Parse(e.OrganizationID)
		if err != nil {
			p.logger.Warn("invalid organization_id in event", zap.String("record_id", e.RecordID), zap.Error(err))
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
			p.logger.Warn("skipping invalid event", zap.String("record_id", e.RecordID), zap.Error(err))
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
	// Parse record_id (maps to event_id in database)
	recordID, err := uuid.Parse(e.RecordID)
	if err != nil {
		return postgres.UsageEvent{}, fmt.Errorf("invalid record_id: %w", err)
	}

	// Parse organization_id
	orgID, err := uuid.Parse(e.OrganizationID)
	if err != nil {
		return postgres.UsageEvent{}, fmt.Errorf("invalid organization_id: %w", err)
	}

	// Parse api_key_id (maps to actor_id in database - represents the actor making the request)
	var actorID uuid.UUID
	if e.APIKeyID != "" {
		actorID, err = uuid.Parse(e.APIKeyID)
		if err != nil {
			return postgres.UsageEvent{}, fmt.Errorf("invalid api_key_id: %w", err)
		}
	}

	// Model is a string (user-facing model name, not UUID)
	// We'll store this in metadata for now, and use Nil for ModelID
	// TODO: Consider adding a model_name field to the database schema
	var modelID uuid.UUID
	// modelID remains Nil - we don't have a model registry ID mapping yet

	// Convert token counts to int64 for database
	inputTokens := int64(e.TokensInput)
	outputTokens := int64(e.TokensOutput)

	// Map limit_state to status field
	// WITHIN_LIMIT → "success", RATE_LIMITED/BUDGET_EXCEEDED → "limited"
	status := "success"
	if e.LimitState == "RATE_LIMITED" || e.LimitState == "BUDGET_EXCEEDED" {
		status = "limited"
	}

	// Store routing metadata in metadata field
	metadata := e.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["model"] = e.Model
	metadata["backend_id"] = e.BackendID
	metadata["limit_state"] = e.LimitState
	metadata["decision_reason"] = e.DecisionReason
	if e.RequestID != "" {
		metadata["request_id"] = e.RequestID
	}
	if e.RetryCount > 0 {
		metadata["retry_count"] = e.RetryCount
	}
	if e.TraceID != "" {
		metadata["trace_id"] = e.TraceID
	}
	if e.SpanID != "" {
		metadata["span_id"] = e.SpanID
	}

	now := time.Now()
	return postgres.UsageEvent{
		EventID:           recordID, // record_id maps to event_id
		OrgID:             orgID,
		OccurredAt:        e.Timestamp, // timestamp maps to occurred_at
		ReceivedAt:        now,
		ModelID:           modelID, // Nil for now - no model registry mapping
		ActorID:           actorID, // api_key_id maps to actor_id
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		LatencyMS:         e.LatencyMS,
		Status:            status,
		ErrorCode:         "", // No error code in UsageRecord schema
		CostEstimateCents: e.CostUSD,
		Metadata:          metadata,
	}, nil
}

