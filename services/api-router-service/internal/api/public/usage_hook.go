// Package public provides usage emission hooks for the inference flow.
//
// Purpose:
//   This package implements hooks for emitting usage records during the inference
//   request flow, with retry logic and buffering for reliability.
//
// Key Responsibilities:
//   - Emit usage records after successful inference
//   - Buffer records when Kafka is unavailable
//   - Retry failed publishes
//   - Integrate with routing decision tracking
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-004 (Accurate, timely usage accounting)
//
package public

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

// UsageHook handles usage record emission with retry and buffering.
type UsageHook struct {
	publisher   *usage.Publisher
	bufferStore *usage.BufferStore
	builder     *usage.RecordBuilder
	logger      *zap.Logger
	retryDelay  time.Duration
	maxRetries  int
	mu          sync.Mutex
	retryTicker *time.Ticker
	retryCtx    context.Context
	retryCancel context.CancelFunc
}

// UsageHookConfig configures the usage hook.
type UsageHookConfig struct {
	Publisher   *usage.Publisher
	BufferStore *usage.BufferStore
	Builder     *usage.RecordBuilder
	Logger      *zap.Logger
	RetryDelay  time.Duration
	MaxRetries  int
}

// NewUsageHook creates a new usage hook.
func NewUsageHook(cfg UsageHookConfig) *UsageHook {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 5 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	hook := &UsageHook{
		publisher:   cfg.Publisher,
		bufferStore: cfg.BufferStore,
		builder:     cfg.Builder,
		logger:      cfg.Logger.With(zap.String("component", "usage-hook")),
		retryDelay:  cfg.RetryDelay,
		maxRetries:  cfg.MaxRetries,
		retryCtx:    ctx,
		retryCancel: cancel,
	}

	// Start retry worker
	hook.startRetryWorker()

	return hook
}

// EmitUsage emits a usage record after successful inference.
func (h *UsageHook) EmitUsage(
	ctx context.Context,
	authCtx *auth.AuthenticatedContext,
	requestID string,
	model string,
	backendID string,
	decisionReason string,
	tokensInput int,
	tokensOutput int,
	latencyMS int,
	limitState string,
	spanContext trace.SpanContext,
	retryCount int,
) error {
	// Build usage record context
	recordCtx := usage.NewRecordContext(
		requestID,
		authCtx.OrganizationID,
		authCtx.APIKeyID,
		model,
		backendID,
		tokensInput,
		tokensOutput,
		latencyMS,
		limitState,
		decisionReason,
	).
		WithTraceContext(spanContext).
		WithRetryCount(retryCount)

	// Build record
	record := h.builder.BuildRecord(recordCtx)

	// Try to publish immediately
	if err := h.publisher.Publish(ctx, record); err != nil {
		h.logger.Warn("failed to publish usage record, buffering",
			zap.String("record_id", record.RecordID),
			zap.String("request_id", requestID),
			zap.Error(err),
		)

		// Buffer for retry
		if h.bufferStore != nil {
			if bufferErr := h.bufferStore.Store(record); bufferErr != nil {
				h.logger.Error("failed to buffer usage record",
					zap.String("record_id", record.RecordID),
					zap.String("request_id", requestID),
					zap.Error(bufferErr),
				)
				return fmt.Errorf("buffer usage record: %w", bufferErr)
			}
		}

		return err
	}

	h.logger.Debug("usage record emitted",
		zap.String("record_id", record.RecordID),
		zap.String("request_id", requestID),
		zap.String("organization_id", authCtx.OrganizationID),
	)

	return nil
}

// startRetryWorker starts a background worker to retry buffered records.
func (h *UsageHook) startRetryWorker() {
	h.retryTicker = time.NewTicker(h.retryDelay)

	go func() {
		defer h.retryTicker.Stop()

		for {
			select {
			case <-h.retryCtx.Done():
				return
			case <-h.retryTicker.C:
				h.retryBufferedRecords()
			}
		}
	}()
}

// retryBufferedRecords attempts to republish buffered records.
func (h *UsageHook) retryBufferedRecords() {
	if h.bufferStore == nil || h.publisher == nil {
		return
	}

	// Load buffered records
	records, err := h.bufferStore.Load()
	if err != nil {
		h.logger.Warn("failed to load buffered records", zap.Error(err))
		return
	}

	if len(records) == 0 {
		return
	}

	h.logger.Debug("retrying buffered usage records",
		zap.Int("count", len(records)),
	)

	// Try to publish each record
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, record := range records {
		if err := h.publisher.Publish(ctx, record); err != nil {
			h.logger.Warn("failed to retry buffered usage record",
				zap.String("record_id", record.RecordID),
				zap.Error(err),
			)
			// Keep in buffer for next retry
			continue
		}

		// Successfully published, remove from buffer
		if err := h.bufferStore.Remove(record.RecordID); err != nil {
			h.logger.Warn("failed to remove buffered record after successful publish",
				zap.String("record_id", record.RecordID),
				zap.Error(err),
			)
		} else {
			h.logger.Debug("successfully retried buffered usage record",
				zap.String("record_id", record.RecordID),
			)
		}
	}
}

// Stop stops the retry worker.
func (h *UsageHook) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.retryCancel != nil {
		h.retryCancel()
	}
	if h.retryTicker != nil {
		h.retryTicker.Stop()
	}
}

// LoadBufferedRecords loads buffered records (for testing/admin).
func (h *UsageHook) LoadBufferedRecords() ([]*usage.UsageRecord, error) {
	if h.bufferStore == nil {
		return nil, fmt.Errorf("buffer store not configured")
	}
	return h.bufferStore.Load()
}

