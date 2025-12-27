// Package ingestion provides Kafka consumer for usage events.
//
// Purpose:
//
//	This package handles consuming usage events from Kafka, deduplicating
//	them, and persisting to TimescaleDB. It provides backpressure controls and
//	batch processing for efficient ingestion.
//
// Dependencies:
//   - Kafka for event streaming
//   - TimescaleDB for persistence
//
// Key Responsibilities:
//   - Connect to Kafka brokers
//   - Consume events in batches
//   - Deduplicate events by (event_id, org_id)
//   - Persist to usage_events table
//   - Track ingestion batches
//   - Handle backpressure and errors
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// Consumer handles Kafka consumption.
type Consumer struct {
	logger    *zap.Logger
	topic     string
	groupID   string
	batchSize int
	workers   int
	processor *Processor
	reader    *kafka.Reader
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// Config holds consumer configuration.
type Config struct {
	Brokers      []string
	Topic        string
	GroupID      string
	BatchSize    int
	Workers      int
	BatchTimeout time.Duration
	Logger       *zap.Logger
	Store        *postgres.Store
}

// NewConsumer creates a new ingestion consumer.
func NewConsumer(cfg Config) (*Consumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers are required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("group ID is required")
	}
	if cfg.BatchSize <= 0 {
		return nil, fmt.Errorf("batch size must be positive, got %d", cfg.BatchSize)
	}
	if cfg.Workers <= 0 {
		return nil, fmt.Errorf("workers must be positive, got %d", cfg.Workers)
	}
	if cfg.Store == nil {
		return nil, fmt.Errorf("store is required")
	}

	processor := NewProcessor(cfg.Store, cfg.Logger)

	// Create Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,       // Fetch at least 1 byte
		MaxBytes:       10e6,    // 10MB max per fetch
		CommitInterval: 1 * time.Second,
		StartOffset:    kafka.FirstOffset,
		MaxWait:        500 * time.Millisecond, // Max wait time for batch
	})

	return &Consumer{
		logger:    cfg.Logger,
		topic:     cfg.Topic,
		groupID:   cfg.GroupID,
		batchSize: cfg.BatchSize,
		workers:   cfg.Workers,
		processor: processor,
		reader:    reader,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start begins consuming events from Kafka.
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("starting ingestion consumer",
		zap.String("topic", c.topic),
		zap.String("group_id", c.groupID),
		zap.Int("batch_size", c.batchSize),
		zap.Int("workers", c.workers),
	)

	// Start worker goroutines
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}

	c.logger.Info("ingestion consumer started successfully")
	return nil
}

// Stop gracefully stops the consumer.
func (c *Consumer) Stop(ctx context.Context) error {
	c.logger.Info("stopping ingestion consumer")

	// Signal workers to stop
	close(c.stopCh)

	// Wait for workers to finish (with timeout)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("all workers stopped")
	case <-ctx.Done():
		c.logger.Warn("timeout waiting for workers to stop")
	case <-time.After(10 * time.Second):
		c.logger.Warn("timeout waiting for workers to stop")
	}

	// Close Kafka reader
	if c.reader != nil {
		if err := c.reader.Close(); err != nil {
			c.logger.Error("error closing Kafka reader", zap.Error(err))
		}
	}

	c.logger.Info("ingestion consumer stopped")
	return nil
}

// worker processes messages from Kafka.
func (c *Consumer) worker(ctx context.Context, id int) {
	defer c.wg.Done()

	c.logger.Info("worker started", zap.Int("worker_id", id))

	// Batch collection
	batch := make([]Event, 0, c.batchSize)
	messages := make([]kafka.Message, 0, c.batchSize)
	batchTimer := time.NewTimer(5 * time.Second)
	defer batchTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("worker stopping due to context cancellation", zap.Int("worker_id", id))
			// Process remaining batch
			if len(batch) > 0 {
				c.processBatch(ctx, batch, messages, id)
			}
			return

		case <-c.stopCh:
			c.logger.Info("worker stopping", zap.Int("worker_id", id))
			// Process remaining batch
			if len(batch) > 0 {
				c.processBatch(ctx, batch, messages, id)
			}
			return

		case <-batchTimer.C:
			// Process batch on timeout
			if len(batch) > 0 {
				c.processBatch(ctx, batch, messages, id)
				batch = batch[:0]
				messages = messages[:0]
			}
			batchTimer.Reset(5 * time.Second)

		default:
			// Try to read a message with timeout
			readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			msg, err := c.reader.FetchMessage(readCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					// No message available, check timer
					continue
				}
				c.logger.Error("failed to fetch message",
					zap.Int("worker_id", id),
					zap.Error(err),
				)
				// Brief backoff on error
				time.Sleep(1 * time.Second)
				continue
			}

			// Parse event
			event, err := c.parseMessage(msg)
			if err != nil {
				c.logger.Error("failed to parse message",
					zap.Int("worker_id", id),
					zap.Error(err),
				)
				// Commit the message even if parsing fails to avoid reprocessing
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					c.logger.Error("failed to commit invalid message",
						zap.Int("worker_id", id),
						zap.Error(err),
					)
				}
				continue
			}

			batch = append(batch, event)
			messages = append(messages, msg)

			// Process batch if it reaches batch size
			if len(batch) >= c.batchSize {
				c.processBatch(ctx, batch, messages, id)
				batch = batch[:0]
				messages = messages[:0]
				batchTimer.Reset(5 * time.Second)
			}
		}
	}
}

// parseMessage parses a Kafka message into an Event.
func (c *Consumer) parseMessage(msg kafka.Message) (Event, error) {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return Event{}, fmt.Errorf("unmarshal event: %w", err)
	}

	// Validate required fields (matching UsageRecord schema)
	if event.RecordID == "" {
		return Event{}, fmt.Errorf("record_id is required")
	}
	if event.OrganizationID == "" {
		return Event{}, fmt.Errorf("organization_id is required")
	}

	return event, nil
}

// processBatch processes a batch of events and commits offsets.
func (c *Consumer) processBatch(ctx context.Context, events []Event, messages []kafka.Message, workerID int) {
	if len(events) == 0 {
		return
	}

	// Use the processor to handle batch processing
	// For Kafka, we use the last message offset as the stream offset
	var streamOffset int64
	if len(messages) > 0 {
		streamOffset = messages[len(messages)-1].Offset
	}

	if err := c.processor.ProcessBatch(ctx, events, streamOffset); err != nil {
		c.logger.Error("failed to process batch",
			zap.Int("worker_id", workerID),
			zap.Int("event_count", len(events)),
			zap.Error(err),
		)
		// Don't commit offsets if processing fails
		return
	}

	// Commit offsets only after successful processing
	if len(messages) > 0 {
		if err := c.reader.CommitMessages(ctx, messages...); err != nil {
			c.logger.Error("failed to commit offsets",
				zap.Int("worker_id", workerID),
				zap.Int("message_count", len(messages)),
				zap.Error(err),
			)
		} else {
			c.logger.Debug("processed batch",
				zap.Int("worker_id", workerID),
				zap.Int("event_count", len(events)),
				zap.Int64("last_offset", streamOffset),
			)
		}
	}
}

// Event represents a usage event from Kafka.
// This struct MUST match the UsageRecord schema from api-router-service
// (see services/api-router-service/internal/usage/record.go).
type Event struct {
	// Core identification
	RecordID       string    `json:"record_id"`        // Maps to UsageRecord.RecordID
	RequestID      string    `json:"request_id"`       // Maps to UsageRecord.RequestID
	OrganizationID string    `json:"organization_id"`  // Maps to UsageRecord.OrganizationID
	APIKeyID       string    `json:"api_key_id"`       // Maps to UsageRecord.APIKeyID
	Timestamp      time.Time `json:"timestamp"`        // Maps to UsageRecord.Timestamp

	// Model and backend
	Model     string `json:"model"`      // Maps to UsageRecord.Model (user-facing model name)
	BackendID string `json:"backend_id"` // Maps to UsageRecord.BackendID

	// Token usage and cost
	TokensInput  int     `json:"tokens_input"`  // Maps to UsageRecord.TokensInput
	TokensOutput int     `json:"tokens_output"` // Maps to UsageRecord.TokensOutput
	CostUSD      float64 `json:"cost_usd"`      // Maps to UsageRecord.CostUSD

	// Performance and routing
	LatencyMS      int    `json:"latency_ms"`      // Maps to UsageRecord.LatencyMS
	LimitState     string `json:"limit_state"`     // Maps to UsageRecord.LimitState (WITHIN_LIMIT, RATE_LIMITED, BUDGET_EXCEEDED)
	DecisionReason string `json:"decision_reason"` // Maps to UsageRecord.DecisionReason (PRIMARY, FAILOVER, OVERRIDE, RATE_LIMIT)

	// Optional fields
	RetryCount int                    `json:"retry_count,omitempty"` // Maps to UsageRecord.RetryCount
	TraceID    string                 `json:"trace_id,omitempty"`    // Maps to UsageRecord.TraceID
	SpanID     string                 `json:"span_id,omitempty"`     // Maps to UsageRecord.SpanID
	Metadata   map[string]interface{} `json:"metadata,omitempty"`    // Maps to UsageRecord.Metadata
}
