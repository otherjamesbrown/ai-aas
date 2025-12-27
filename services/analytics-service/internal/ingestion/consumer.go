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

	// Validate required fields
	if event.EventID == "" {
		return Event{}, fmt.Errorf("event_id is required")
	}
	if event.OrgID == "" {
		return Event{}, fmt.Errorf("org_id is required")
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
type Event struct {
	EventID      string                 `json:"event_id"`
	OrgID        string                 `json:"org_id"`
	ModelID      string                 `json:"model_id"`
	OccurredAt   time.Time              `json:"occurred_at"`
	InputTokens  int64                  `json:"input_tokens"`
	OutputTokens int64                  `json:"output_tokens"`
	LatencyMS    int                    `json:"latency_ms"`
	Status       string                 `json:"status"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	CostEstimate float64                `json:"cost_estimate"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
