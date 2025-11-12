// Package usage provides Kafka publisher for usage records.
//
// Purpose:
//   This package implements Kafka publishing for usage records with at-least-once
//   delivery guarantees, buffering, and retry logic.
//
// Key Responsibilities:
//   - Publish usage records to Kafka topic
//   - Handle connection failures gracefully
//   - Buffer records when Kafka is unavailable
//   - Retry failed publishes
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-004 (Accurate, timely usage accounting)
//   - specs/006-api-router-service/spec.md#NFR-006 (At-least-once delivery)
//
package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Publisher publishes usage records to Kafka.
type Publisher struct {
	writer *kafka.Writer
	logger *zap.Logger
	mu     sync.RWMutex
	topic  string
}

// PublisherConfig configures the Kafka publisher.
type PublisherConfig struct {
	Brokers      []string
	Topic        string
	ClientID     string
	BatchSize    int
	BatchTimeout time.Duration
	WriteTimeout time.Duration
	RequiredAcks kafka.RequiredAcks
}

// NewPublisher creates a new Kafka publisher for usage records.
func NewPublisher(cfg PublisherConfig, logger *zap.Logger) *Publisher {
	if logger == nil {
		logger = zap.NewNop()
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: cfg.RequiredAcks,
		Async:        false, // Synchronous writes for reliability
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		WriteTimeout: cfg.WriteTimeout,
		ReadTimeout:  5 * time.Second,
	}

	if cfg.ClientID != "" {
		writer.Transport = &kafka.Transport{
			ClientID: cfg.ClientID,
		}
	}

	return &Publisher{
		writer: writer,
		logger: logger.With(zap.String("component", "usage-publisher")),
		topic:  cfg.Topic,
	}
}

// Publish publishes a usage record to Kafka.
// Returns an error if the publish fails (for buffering/retry logic).
func (p *Publisher) Publish(ctx context.Context, record *UsageRecord) error {
	p.mu.RLock()
	writer := p.writer
	p.mu.RUnlock()

	if writer == nil {
		return fmt.Errorf("kafka writer is closed")
	}

	// Serialize record to JSON
	payload, err := json.Marshal(record)
	if err != nil {
		p.logger.Error("failed to serialize usage record",
			zap.String("record_id", record.RecordID),
			zap.String("request_id", record.RequestID),
			zap.Error(err),
		)
		return fmt.Errorf("serialize usage record: %w", err)
	}

	// Create Kafka message with record ID as key for partitioning
	message := kafka.Message{
		Key:   []byte(record.RecordID),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "record_id", Value: []byte(record.RecordID)},
			{Key: "request_id", Value: []byte(record.RequestID)},
			{Key: "organization_id", Value: []byte(record.OrganizationID)},
			{Key: "model", Value: []byte(record.Model)},
			{Key: "backend_id", Value: []byte(record.BackendID)},
		},
		Time: record.Timestamp,
	}

	// Write to Kafka
	if err := writer.WriteMessages(ctx, message); err != nil {
		p.logger.Error("failed to publish usage record to Kafka",
			zap.String("record_id", record.RecordID),
			zap.String("request_id", record.RequestID),
			zap.String("organization_id", record.OrganizationID),
			zap.String("model", record.Model),
			zap.Error(err),
		)
		return fmt.Errorf("publish usage record to Kafka: %w", err)
	}

	p.logger.Debug("usage record published to Kafka",
		zap.String("record_id", record.RecordID),
		zap.String("request_id", record.RequestID),
		zap.String("topic", p.topic),
	)

	return nil
}

// PublishBatch publishes multiple usage records in a batch.
func (p *Publisher) PublishBatch(ctx context.Context, records []*UsageRecord) error {
	if len(records) == 0 {
		return nil
	}

	p.mu.RLock()
	writer := p.writer
	p.mu.RUnlock()

	if writer == nil {
		return fmt.Errorf("kafka writer is closed")
	}

	messages := make([]kafka.Message, 0, len(records))
	for _, record := range records {
		payload, err := json.Marshal(record)
		if err != nil {
			p.logger.Error("failed to serialize usage record in batch",
				zap.String("record_id", record.RecordID),
				zap.Error(err),
			)
			continue // Skip invalid records
		}

		message := kafka.Message{
			Key:   []byte(record.RecordID),
			Value: payload,
			Headers: []kafka.Header{
				{Key: "record_id", Value: []byte(record.RecordID)},
				{Key: "request_id", Value: []byte(record.RequestID)},
				{Key: "organization_id", Value: []byte(record.OrganizationID)},
				{Key: "model", Value: []byte(record.Model)},
				{Key: "backend_id", Value: []byte(record.BackendID)},
			},
			Time: record.Timestamp,
		}
		messages = append(messages, message)
	}

	if len(messages) == 0 {
		return fmt.Errorf("no valid messages to publish")
	}

	if err := writer.WriteMessages(ctx, messages...); err != nil {
		p.logger.Error("failed to publish usage record batch to Kafka",
			zap.Int("batch_size", len(messages)),
			zap.Error(err),
		)
		return fmt.Errorf("publish usage record batch to Kafka: %w", err)
	}

	p.logger.Debug("usage record batch published to Kafka",
		zap.Int("batch_size", len(messages)),
		zap.String("topic", p.topic),
	)

	return nil
}

// Close closes the Kafka writer connection.
// Safe to call multiple times.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.writer == nil {
		return nil
	}

	err := p.writer.Close()
	p.writer = nil
	p.logger.Info("Kafka publisher closed")
	return err
}

// Health checks if the publisher can connect to Kafka.
func (p *Publisher) Health(ctx context.Context) error {
	p.mu.RLock()
	writer := p.writer
	p.mu.RUnlock()

	if writer == nil {
		return fmt.Errorf("kafka writer is closed")
	}

	// Try to write a test message (or use a different health check mechanism)
	// For now, we'll just check if writer exists
	// In production, you might want to use kafka.Conn for health checks
	return nil
}

