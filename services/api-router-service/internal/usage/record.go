// Package usage provides usage record building and tracking.
//
// Purpose:
//   This package implements usage record construction capturing routing metadata,
//   token usage, costs, and decision context for billing and analytics.
//
// Key Responsibilities:
//   - Build usage records from request/response context
//   - Capture routing decision metadata
//   - Calculate costs and token usage
//   - Include trace and span IDs for observability
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-004 (Accurate, timely usage accounting)
//   - specs/006-api-router-service/contracts/usage-record.schema.yaml
//
package usage

import (
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// UsageRecord represents a usage record for billing and analytics.
// Matches the schema defined in usage-record.schema.yaml
type UsageRecord struct {
	RecordID       string                 `json:"record_id"`
	RequestID      string                 `json:"request_id"`
	OrganizationID string                 `json:"organization_id"`
	APIKeyID       string                 `json:"api_key_id"`
	Model          string                 `json:"model"`
	BackendID      string                 `json:"backend_id"`
	TokensInput    int                    `json:"tokens_input"`
	TokensOutput   int                    `json:"tokens_output"`
	LatencyMS      int                    `json:"latency_ms"`
	CostUSD        float64                `json:"cost_usd"`
	LimitState     string                 `json:"limit_state"`
	DecisionReason string                 `json:"decision_reason"`
	BudgetSnapshot *BudgetSnapshot        `json:"budget_snapshot,omitempty"`
	RetryCount     int                    `json:"retry_count,omitempty"`
	TraceID        string                 `json:"trace_id,omitempty"`
	SpanID         string                 `json:"span_id,omitempty"`
	Metadata       map[string]string      `json:"metadata,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// BudgetSnapshot represents budget state at the time of the request.
type BudgetSnapshot struct {
	Period            string  `json:"period"` // "DAILY" or "MONTHLY"
	TokensRemaining   int     `json:"tokens_remaining"`
	CurrencyRemaining float64 `json:"currency_remaining"`
}

// RecordBuilder builds usage records from request context.
type RecordBuilder struct {
	// Cost calculation function (can be overridden for testing)
	costCalculator func(tokensInput, tokensOutput int, model string) float64
}

// NewRecordBuilder creates a new usage record builder.
func NewRecordBuilder() *RecordBuilder {
	return &RecordBuilder{
		costCalculator: defaultCostCalculator,
	}
}

// SetCostCalculator sets a custom cost calculation function.
func (b *RecordBuilder) SetCostCalculator(calc func(tokensInput, tokensOutput int, model string) float64) {
	b.costCalculator = calc
}

// BuildRecord builds a usage record from request context and routing metadata.
func (b *RecordBuilder) BuildRecord(ctx *RecordContext) *UsageRecord {
	recordID := uuid.New().String()

	// Calculate cost
	cost := b.costCalculator(ctx.TokensInput, ctx.TokensOutput, ctx.Model)

	record := &UsageRecord{
		RecordID:       recordID,
		RequestID:      ctx.RequestID,
		OrganizationID: ctx.OrganizationID,
		APIKeyID:       ctx.APIKeyID,
		Model:          ctx.Model,
		BackendID:      ctx.BackendID,
		TokensInput:    ctx.TokensInput,
		TokensOutput:   ctx.TokensOutput,
		LatencyMS:      ctx.LatencyMS,
		CostUSD:        cost,
		LimitState:     ctx.LimitState,
		DecisionReason: ctx.DecisionReason,
		RetryCount:     ctx.RetryCount,
		Timestamp:      time.Now().UTC(),
	}

	// Add trace/span IDs if available
	if ctx.TraceID != "" {
		record.TraceID = ctx.TraceID
	}
	if ctx.SpanID != "" {
		record.SpanID = ctx.SpanID
	}

	// Add budget snapshot if available
	if ctx.BudgetSnapshot != nil {
		record.BudgetSnapshot = ctx.BudgetSnapshot
	}

	// Add metadata if available
	if len(ctx.Metadata) > 0 {
		record.Metadata = ctx.Metadata
	}

	return record
}

// RecordContext contains all context needed to build a usage record.
type RecordContext struct {
	RequestID      string
	OrganizationID string
	APIKeyID       string
	Model          string
	BackendID      string
	TokensInput    int
	TokensOutput   int
	LatencyMS      int
	LimitState     string // "WITHIN_LIMIT", "RATE_LIMITED", "BUDGET_EXCEEDED"
	DecisionReason string // "PRIMARY", "FAILOVER", "OVERRIDE", "RATE_LIMIT"
	BudgetSnapshot *BudgetSnapshot
	RetryCount     int
	TraceID        string
	SpanID         string
	Metadata       map[string]string
}

// NewRecordContext creates a new record context from basic fields.
func NewRecordContext(
	requestID, organizationID, apiKeyID, model, backendID string,
	tokensInput, tokensOutput, latencyMS int,
	limitState, decisionReason string,
) *RecordContext {
	return &RecordContext{
		RequestID:      requestID,
		OrganizationID: organizationID,
		APIKeyID:       apiKeyID,
		Model:          model,
		BackendID:      backendID,
		TokensInput:    tokensInput,
		TokensOutput:   tokensOutput,
		LatencyMS:      latencyMS,
		LimitState:     limitState,
		DecisionReason: decisionReason,
		Metadata:       make(map[string]string),
	}
}

// WithTraceContext adds trace and span IDs from OpenTelemetry span context.
func (c *RecordContext) WithTraceContext(spanContext trace.SpanContext) *RecordContext {
	c.TraceID = spanContext.TraceID().String()
	c.SpanID = spanContext.SpanID().String()
	return c
}

// WithBudgetSnapshot adds budget snapshot information.
func (c *RecordContext) WithBudgetSnapshot(snapshot *BudgetSnapshot) *RecordContext {
	c.BudgetSnapshot = snapshot
	return c
}

// WithRetryCount sets the retry count.
func (c *RecordContext) WithRetryCount(count int) *RecordContext {
	c.RetryCount = count
	return c
}

// WithMetadata adds metadata key-value pairs.
func (c *RecordContext) WithMetadata(key, value string) *RecordContext {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	c.Metadata[key] = value
	return c
}

// WithMetadataMap adds multiple metadata entries.
func (c *RecordContext) WithMetadataMap(metadata map[string]string) *RecordContext {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	for k, v := range metadata {
		c.Metadata[k] = v
	}
	return c
}

// defaultCostCalculator calculates cost based on token usage and model.
// This is a simplified cost model - in production, this would query
// a pricing service or use a more sophisticated model.
func defaultCostCalculator(tokensInput, tokensOutput int, model string) float64 {
	// Simplified cost model (per 1K tokens)
	// These are example rates - should be configurable or fetched from pricing service
	var inputRate, outputRate float64

	switch model {
	case "gpt-4o", "gpt-4":
		inputRate = 0.005   // $0.005 per 1K input tokens
		outputRate = 0.015  // $0.015 per 1K output tokens
	case "gpt-3.5-turbo":
		inputRate = 0.0005  // $0.0005 per 1K input tokens
		outputRate = 0.0015 // $0.0015 per 1K output tokens
	default:
		// Default rates
		inputRate = 0.001
		outputRate = 0.002
	}

	inputCost := (float64(tokensInput) / 1000.0) * inputRate
	outputCost := (float64(tokensOutput) / 1000.0) * outputRate

	return inputCost + outputCost
}

