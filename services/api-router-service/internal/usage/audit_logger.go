// Package usage provides usage tracking and audit logging for API requests.
//
// Purpose:
//   This package implements audit event emission for request denials and usage tracking.
//
// Dependencies:
//   - Kafka (optional, falls back to logger)
//
// Key Responsibilities:
//   - Emit audit events for budget/rate limit denials
//   - Include request context (org, key, model, tokens)
//   - Structured event format
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-002 (Enforce budgets and safe usage)
//
package usage

import (
	"time"

	"go.uber.org/zap"
)

// AuditLogger emits audit events for request denials and usage.
type AuditLogger struct {
	logger *zap.Logger
	// TODO: Add Kafka producer when available
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(logger *zap.Logger) *AuditLogger {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AuditLogger{
		logger: logger,
	}
}

// AuditEvent represents an audit event.
type AuditEvent struct {
	RequestID      string
	OrganizationID string
	APIKeyID       string
	Model          string
	Action         string // "REQUEST_DENIED", "REQUEST_ALLOWED"
	DecisionReason string // "BUDGET_EXCEEDED", "RATE_LIMIT_EXCEEDED", "QUOTA_EXCEEDED"
	LimitState     string
	Timestamp      time.Time
}

// LogDenial logs a request denial event.
func (a *AuditLogger) LogDenial(event AuditEvent) {
	event.Timestamp = time.Now()
	a.logger.Info("request denied",
		zap.String("request_id", event.RequestID),
		zap.String("organization_id", event.OrganizationID),
		zap.String("api_key_id", event.APIKeyID),
		zap.String("model", event.Model),
		zap.String("action", event.Action),
		zap.String("decision_reason", event.DecisionReason),
		zap.String("limit_state", event.LimitState),
		zap.Time("timestamp", event.Timestamp),
	)
	
	// TODO: Emit to Kafka when available
}

// LogAllowed logs a request allowed event (for usage tracking).
func (a *AuditLogger) LogAllowed(event AuditEvent) {
	event.Timestamp = time.Now()
	a.logger.Debug("request allowed",
		zap.String("request_id", event.RequestID),
		zap.String("organization_id", event.OrganizationID),
		zap.String("api_key_id", event.APIKeyID),
		zap.String("model", event.Model),
		zap.String("action", event.Action),
		zap.Time("timestamp", event.Timestamp),
	)
	
	// TODO: Emit to Kafka when available
}

