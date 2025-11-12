// Package ingestion provides RabbitMQ stream consumer for usage events.
//
// Purpose:
//
//	This package handles consuming usage events from RabbitMQ streams, deduplicating
//	them, and persisting to TimescaleDB. It provides backpressure controls and
//	batch processing for efficient ingestion.
//
// Dependencies:
//   - RabbitMQ Streams plugin
//   - TimescaleDB for persistence
//
// Key Responsibilities:
//   - Connect to RabbitMQ stream
//   - Consume events in batches
//   - Deduplicate events by (event_id, org_id)
//   - Persist to usage_events table
//   - Track ingestion batches
//   - Handle backpressure and errors
package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// Consumer handles RabbitMQ stream consumption.
type Consumer struct {
	logger         *zap.Logger
	streamName     string
	consumer       string
	batchSize      int
	workers        int
	processor      *Processor
	env            *stream.Environment
	consumerHandle *stream.Consumer
	stopCh         chan struct{}
	wg             sync.WaitGroup
	config         Config // Store config for access in Start
}

// Config holds consumer configuration.
type Config struct {
	StreamURL     string
	Stream        string
	Consumer      string
	BatchSize     int
	Workers       int
	BatchTimeout  time.Duration
	Logger        *zap.Logger
	Store         *postgres.Store
	RabbitMQHost  string
	RabbitMQPort  int
	RabbitMQUser  string
	RabbitMQPass  string
}

// NewConsumer creates a new ingestion consumer.
func NewConsumer(cfg Config) (*Consumer, error) {
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

	return &Consumer{
		logger:     cfg.Logger,
		streamName: cfg.Stream,
		consumer:   cfg.Consumer,
		batchSize:  cfg.BatchSize,
		workers:    cfg.Workers,
		processor:  processor,
		stopCh:     make(chan struct{}),
		config:     cfg, // Store config
	}, nil
}

// Start begins consuming events from the stream.
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("starting ingestion consumer",
		zap.String("stream", c.streamName),
		zap.String("consumer", c.consumer),
		zap.Int("batch_size", c.batchSize),
		zap.Int("workers", c.workers),
	)

	// Parse connection details from URL or use provided values
	host := c.config.RabbitMQHost
	port := c.config.RabbitMQPort
	user := c.config.RabbitMQUser
	password := c.config.RabbitMQPass

	// Parse URL if provided and values not set
	if c.config.StreamURL != "" && (host == "" || port == 0 || user == "" || password == "") {
		parsedHost, parsedPort, parsedUser, parsedPass, err := parseRabbitMQURL(c.config.StreamURL)
		if err == nil {
			if host == "" {
				host = parsedHost
			}
			if port == 0 {
				port = parsedPort
			}
			if user == "" {
				user = parsedUser
			}
			if password == "" {
				password = parsedPass
			}
		}
	}

	// Defaults
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 5552 // RabbitMQ Streams default port
	}
	if user == "" {
		user = "guest"
	}
	if password == "" {
		password = "guest"
	}

	// Create RabbitMQ Stream environment
	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(host).
			SetPort(port).
			SetUser(user).
			SetPassword(password),
	)
	if err != nil {
		return fmt.Errorf("create stream environment: %w", err)
	}
	c.env = env

	// Declare stream if it doesn't exist
	err = env.DeclareStream(c.streamName,
		stream.NewStreamOptions().
			SetMaxLengthBytes(stream.ByteCapacity{}.GB(50)),
	)
	if err != nil && !errors.Is(err, stream.StreamAlreadyExists) {
		return fmt.Errorf("declare stream: %w", err)
	}

	// Create message channel for workers
	messageCh := make(chan *amqp.Message, c.batchSize*c.workers)

	// Create consumer with callback that sends messages to channel
	consumer, err := env.NewConsumer(
		c.streamName,
		func(consumerContext stream.ConsumerContext, message *amqp.Message) {
			select {
			case messageCh <- message:
				// Message queued successfully
			case <-c.stopCh:
				// Consumer stopping, drop message
				c.logger.Debug("dropping message during shutdown")
			default:
				// Channel full - backpressure
				c.logger.Warn("message channel full, dropping message")
			}
		},
		stream.NewConsumerOptions().
			SetConsumerName(c.consumer).
			SetOffset(stream.OffsetSpecification{}.First()),
	)
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}
	c.consumerHandle = consumer

	// Start worker goroutines
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i, messageCh)
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

	// Close consumer
	if c.consumerHandle != nil {
		if err := c.consumerHandle.Close(); err != nil {
			c.logger.Error("error closing consumer", zap.Error(err))
		}
	}

	// Close environment
	if c.env != nil {
		if err := c.env.Close(); err != nil {
			c.logger.Error("error closing stream environment", zap.Error(err))
		}
	}

	c.logger.Info("ingestion consumer stopped")
	return nil
}

// worker processes messages from the stream.
func (c *Consumer) worker(ctx context.Context, id int, messageCh <-chan *amqp.Message) {
	defer c.wg.Done()

	c.logger.Info("worker started", zap.Int("worker_id", id))

	// Batch collection
	batch := make([]Event, 0, c.batchSize)
	batchTimer := time.NewTimer(5 * time.Second)
	defer batchTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("worker stopping due to context cancellation", zap.Int("worker_id", id))
			// Process remaining batch
			if len(batch) > 0 {
				c.processBatch(ctx, batch, id)
			}
			return

		case <-c.stopCh:
			c.logger.Info("worker stopping", zap.Int("worker_id", id))
			// Process remaining batch
			if len(batch) > 0 {
				c.processBatch(ctx, batch, id)
			}
			return

		case msg, ok := <-messageCh:
			if !ok {
				// Channel closed, process remaining batch
				if len(batch) > 0 {
					c.processBatch(ctx, batch, id)
				}
				return
			}

			// Parse event
			event, err := c.parseMessage(msg)
			if err != nil {
				c.logger.Error("failed to parse message",
					zap.Int("worker_id", id),
					zap.Error(err),
				)
				continue
			}

			batch = append(batch, event)

			// Process batch if it reaches batch size
			if len(batch) >= c.batchSize {
				c.processBatch(ctx, batch, id)
				batch = batch[:0]
				batchTimer.Reset(5 * time.Second)
			}

		case <-batchTimer.C:
			// Process batch on timeout
			if len(batch) > 0 {
				c.processBatch(ctx, batch, id)
				batch = batch[:0]
			}
			batchTimer.Reset(5 * time.Second)
		}
	}
}
// parseRabbitMQURL parses a RabbitMQ URL to extract connection details.
// Supports formats: amqp://user:pass@host:port or stream://host:port
func parseRabbitMQURL(rawURL string) (host string, port int, user string, password string, err error) {
	// Defaults
	host = "localhost"
	port = 5552 // RabbitMQ Streams default port
	user = "guest"
	password = "guest"

	if rawURL == "" {
		return host, port, user, password, nil
	}

	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return host, port, user, password, fmt.Errorf("parse URL: %w", err)
	}

	// Extract host
	if u.Hostname() != "" {
		host = u.Hostname()
	}

	// Extract port
	if u.Port() != "" {
		parsedPort, err := strconv.Atoi(u.Port())
		if err == nil {
			port = parsedPort
		}
	} else {
		// Default port based on scheme
		if strings.HasPrefix(rawURL, "amqp://") {
			port = 5672 // AMQP port
		} else if strings.HasPrefix(rawURL, "stream://") {
			port = 5552 // Stream port
		}
	}

	// Extract user and password
	if u.User != nil {
		user = u.User.Username()
		if p, ok := u.User.Password(); ok {
			password = p
		}
	}

	return host, port, user, password, nil
}

// parseMessage parses a RabbitMQ stream message into an Event.
func (c *Consumer) parseMessage(msg *amqp.Message) (Event, error) {
	var event Event
	// Get message data (may be in multiple parts, concatenate them)
	data := msg.GetData()
	if len(data) == 0 && len(msg.Data) > 0 {
		// Fallback: concatenate all data parts
		var totalLen int
		for _, part := range msg.Data {
			totalLen += len(part)
		}
		data = make([]byte, 0, totalLen)
		for _, part := range msg.Data {
			data = append(data, part...)
		}
	}
	if err := json.Unmarshal(data, &event); err != nil {
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

// processBatch processes a batch of events.
func (c *Consumer) processBatch(ctx context.Context, events []Event, workerID int) {
	if len(events) == 0 {
		return
	}

	// Use the processor to handle batch processing
	// Note: streamOffset would be tracked per message in production
	streamOffset := int64(0) // Simplified - should track actual offset
	if err := c.processor.ProcessBatch(ctx, events, streamOffset); err != nil {
		c.logger.Error("failed to process batch",
			zap.Int("worker_id", workerID),
			zap.Int("event_count", len(events)),
			zap.Error(err),
		)
	} else {
		c.logger.Debug("processed batch",
			zap.Int("worker_id", workerID),
			zap.Int("event_count", len(events)),
		)
	}
}

// Event represents a usage event from RabbitMQ.
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
