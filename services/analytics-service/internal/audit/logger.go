// Package audit provides audit logging for the analytics service.
//
// Purpose:
//   This package integrates with shared/go/auth to provide structured audit logging
//   for all authorization decisions made by the RBAC middleware.
//
// Dependencies:
//   - github.com/otherjamesbrown/ai-aas/shared/go/auth: Shared authorization audit events
//   - go.uber.org/zap: Structured logging
package audit

import (
	"time"

	"github.com/otherjamesbrown/ai-aas/shared/go/auth"
	"go.uber.org/zap"
)

// Logger provides audit logging functionality.
type Logger struct {
	logger *zap.Logger
}

// NewLogger creates a new audit logger.
func NewLogger(logger *zap.Logger) *Logger {
	return &Logger{
		logger: logger,
	}
}

// Setup configures the shared auth package to use this logger for audit events.
func (l *Logger) Setup() {
	auth.SetAuditRecorder(l.record)
}

// record logs an audit event in structured format.
func (l *Logger) record(event auth.AuditEvent) {
	fields := []zap.Field{
		zap.String("audit.action", event.Action),
		zap.String("audit.subject", event.Subject),
		zap.Strings("audit.roles", event.Roles),
		zap.Bool("audit.allowed", event.Allowed),
		zap.Time("audit.timestamp", time.Now()),
	}

	if event.Allowed {
		l.logger.Info("authorization allowed", fields...)
	} else {
		l.logger.Warn("authorization denied", fields...)
	}
}

