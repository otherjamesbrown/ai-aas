// Package telemetry provides token usage metrics for observability dashboards.
//
// Purpose:
//
//	This package implements metrics collection for token usage across requests,
//	enabling cost tracking and usage analytics in Grafana dashboards.
//
// Key Responsibilities:
//   - Track total tokens processed (counter)
//   - Track token distribution per request (histogram)
//   - Provide metrics labeled by org_id, model, and request_type
//
// Requirements Reference:
//   - ai-aas-rio0: Token Metrics Instrumentation (P1)
//   - Supports Cost Efficiency Dashboard (ai-aas-6eq7)
//   - Supports Org Usage Dashboard (ai-aas-s0pj)
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// TokenMetrics tracks token usage metrics.
type TokenMetrics struct {
	logger *zap.Logger

	// Metrics
	tokensProcessedTotal metric.Int64Counter
	tokensPerRequest     metric.Float64Histogram
}

// NewTokenMetrics creates a new token metrics collector.
func NewTokenMetrics(logger *zap.Logger) (*TokenMetrics, error) {
	meter := otel.Meter("api-router-service")

	tokensProcessedTotal, err := meter.Int64Counter(
		"tokens_processed_total",
		metric.WithDescription("Total number of tokens processed, labeled by token_type (prompt/completion)"),
	)
	if err != nil {
		return nil, err
	}

	tokensPerRequest, err := meter.Float64Histogram(
		"tokens_per_request",
		metric.WithDescription("Distribution of tokens per request"),
		metric.WithUnit("tokens"),
	)
	if err != nil {
		return nil, err
	}

	return &TokenMetrics{
		logger:               logger,
		tokensProcessedTotal: tokensProcessedTotal,
		tokensPerRequest:     tokensPerRequest,
	}, nil
}

// RecordTokens records token usage for a request.
//
// Parameters:
//   - orgID: Organization ID
//   - model: Model name/alias
//   - requestType: Type of request (chat, completion)
//   - promptTokens: Number of prompt tokens
//   - completionTokens: Number of completion tokens
func (m *TokenMetrics) RecordTokens(
	ctx context.Context,
	orgID string,
	model string,
	requestType string,
	promptTokens int,
	completionTokens int,
) {
	totalTokens := promptTokens + completionTokens

	baseAttrs := []attribute.KeyValue{
		attribute.String("org_id", orgID),
		attribute.String("model", model),
		attribute.String("request_type", requestType),
	}

	// Record prompt tokens (counter with token_type=prompt)
	promptAttrs := append([]attribute.KeyValue{}, baseAttrs...)
	promptAttrs = append(promptAttrs, attribute.String("token_type", "prompt"))
	m.tokensProcessedTotal.Add(ctx, int64(promptTokens), metric.WithAttributes(promptAttrs...))

	// Record completion tokens (counter with token_type=completion)
	completionAttrs := append([]attribute.KeyValue{}, baseAttrs...)
	completionAttrs = append(completionAttrs, attribute.String("token_type", "completion"))
	m.tokensProcessedTotal.Add(ctx, int64(completionTokens), metric.WithAttributes(completionAttrs...))

	// Record tokens per request distribution (histogram for total tokens)
	m.tokensPerRequest.Record(ctx, float64(totalTokens), metric.WithAttributes(baseAttrs...))

	// Log debug information for high token usage
	if totalTokens > 10000 {
		m.logger.Debug("high token usage recorded",
			zap.String("org_id", orgID),
			zap.String("model", model),
			zap.String("request_type", requestType),
			zap.Int("prompt_tokens", promptTokens),
			zap.Int("completion_tokens", completionTokens),
			zap.Int("total_tokens", totalTokens),
		)
	}
}
