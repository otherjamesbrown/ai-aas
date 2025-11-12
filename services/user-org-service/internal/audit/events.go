// Package audit provides audit event emission for the user-org service.
//
// Purpose:
//   This package defines the audit event structure and provides an interface
//   for emitting audit events to Kafka. It includes a logger-based stub
//   implementation for development and testing, with a clear path to replace
//   with Kafka producer in production.
//
// Dependencies:
//   - github.com/google/uuid: UUID generation for event IDs
//   - github.com/rs/zerolog: Structured logging for stub implementation
//   - github.com/segmentio/kafka-go: Kafka producer for audit event streaming
//
// Key Responsibilities:
//   - Event struct defines audit event schema matching data model
//   - Emitter interface abstracts Kafka vs logger implementations
//   - LoggerEmitter provides development-friendly stub (logs events)
//   - KafkaEmitter produces to audit.identity topic when Kafka is configured
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-004 (Audit & Compliance)
//   - specs/005-user-org-service/data-model.md (Audit Event entity)
//   - specs/005-user-org-service/spec.md#FR-012 (Audit Logging)
//
// Debugging Notes:
//   - LoggerEmitter logs events as JSON for development visibility
//   - Events include org_id, actor_id, action, target_id for traceability
//   - Hash and signature fields reserved for future tamper-evident features
//   - Metadata field allows extensible event context
//
// Thread Safety:
//   - Emitter implementations must be safe for concurrent use
//   - LoggerEmitter uses zerolog (thread-safe)
//
// Error Handling:
//   - Emit methods return errors for production monitoring
//   - LoggerEmitter never fails (best-effort logging)
//   - KafkaEmitter handles retries via kafka-go library and returns errors for monitoring
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
)

// Event represents an audit event matching the data model schema.
// All state-mutating operations should emit audit events for compliance.
type Event struct {
	EventID    uuid.UUID              `json:"event_id"`
	OrgID      uuid.UUID              `json:"org_id"`
	ActorID    uuid.UUID              `json:"actor_id"`
	ActorType  string                 `json:"actor_type"` // "user", "service_account", "system"
	TargetID   *uuid.UUID             `json:"target_id,omitempty"`
	TargetType string                 `json:"target_type,omitempty"` // "org", "user", "api_key", etc.
	Action     string                 `json:"action"`                 // "org.create", "user.invite", "user.suspend", etc.
	Resource   string                 `json:"resource,omitempty"`     // Resource path or identifier
	PolicyID   *uuid.UUID             `json:"policy_id,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Metadata   map[string]any          `json:"metadata,omitempty"`
	Hash       string                  `json:"hash"`       // SHA256 of event payload (for tamper detection)
	Signature  string                  `json:"signature"` // Reserved for Ed25519 signature (future)
	DeliveredAt *time.Time             `json:"delivered_at,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// Emitter defines the interface for audit event emission.
// Implementations can use Kafka, logger, or other backends.
type Emitter interface {
	// Emit sends an audit event asynchronously.
	// Returns an error if emission fails (for monitoring/alerting).
	Emit(ctx context.Context, event Event) error
}

// LoggerEmitter is a development stub that logs audit events as JSON.
// Useful for local development and testing. In production, replace with
// KafkaEmitter for proper event streaming.
type LoggerEmitter struct {
	logger zerolog.Logger
}

// NewLoggerEmitter creates a logger-based audit emitter.
func NewLoggerEmitter(logger zerolog.Logger) *LoggerEmitter {
	return &LoggerEmitter{logger: logger.With().Str("component", "audit").Logger()}
}

// Emit logs the audit event as structured JSON.
// Never fails (best-effort logging for development).
func (e *LoggerEmitter) Emit(ctx context.Context, event Event) error {
	e.logger.Info().
		Str("event_id", event.EventID.String()).
		Str("org_id", event.OrgID.String()).
		Str("actor_id", event.ActorID.String()).
		Str("actor_type", event.ActorType).
		Str("action", event.Action).
		Str("target_type", event.TargetType).
		Interface("metadata", event.Metadata).
		Msg("audit event")
	return nil
}

// NoopEmitter is a no-op implementation that discards all events.
// Useful for testing or when audit is disabled.
type NoopEmitter struct{}

// NewNoopEmitter creates a no-op audit emitter.
func NewNoopEmitter() *NoopEmitter {
	return &NoopEmitter{}
}

// Emit discards the event (no-op).
func (e *NoopEmitter) Emit(ctx context.Context, event Event) error {
	return nil
}

// KafkaEmitter produces audit events to a Kafka topic.
// Thread-safe and handles connection lifecycle automatically.
type KafkaEmitter struct {
	writer *kafka.Writer
	logger zerolog.Logger
	mu     sync.RWMutex
}

// NewKafkaEmitter creates a Kafka-based audit emitter.
// The writer is configured for at-least-once delivery with automatic retries.
func NewKafkaEmitter(brokers []string, topic, clientID string, logger zerolog.Logger) *KafkaEmitter {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne, // Wait for leader acknowledgment
		Async:        false,             // Synchronous writes for reliability
		BatchSize:    1,                 // Send immediately (audit events are critical)
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	return &KafkaEmitter{
		writer: writer,
		logger: logger.With().Str("component", "audit-kafka").Logger(),
	}
}

// Emit serializes the audit event to JSON and writes it to Kafka.
// Returns an error if the write fails (for monitoring/alerting).
// The event is written synchronously to ensure delivery before returning.
func (e *KafkaEmitter) Emit(ctx context.Context, event Event) error {
	e.mu.RLock()
	writer := e.writer
	e.mu.RUnlock()

	if writer == nil {
		return fmt.Errorf("kafka writer is closed")
	}

	// Serialize event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		e.logger.Error().Err(err).Str("event_id", event.EventID.String()).Msg("failed to serialize audit event")
		return fmt.Errorf("serialize audit event: %w", err)
	}

	// Set delivered_at timestamp
	now := time.Now().UTC()
	event.DeliveredAt = &now

	// Write to Kafka with event ID as key for partitioning
	message := kafka.Message{
		Key:   []byte(event.EventID.String()),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(event.EventID.String())},
			{Key: "org_id", Value: []byte(event.OrgID.String())},
			{Key: "action", Value: []byte(event.Action)},
			{Key: "target_type", Value: []byte(event.TargetType)},
		},
		Time: event.CreatedAt,
	}

	if err := writer.WriteMessages(ctx, message); err != nil {
		e.logger.Error().
			Err(err).
			Str("event_id", event.EventID.String()).
			Str("org_id", event.OrgID.String()).
			Str("action", event.Action).
			Msg("failed to write audit event to Kafka")
		return fmt.Errorf("write audit event to Kafka: %w", err)
	}

	e.logger.Debug().
		Str("event_id", event.EventID.String()).
		Str("org_id", event.OrgID.String()).
		Str("action", event.Action).
		Msg("audit event emitted to Kafka")

	return nil
}

// Close closes the Kafka writer connection.
// Safe to call multiple times.
func (e *KafkaEmitter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.writer == nil {
		return nil
	}

	err := e.writer.Close()
	e.writer = nil
	return err
}

// NewKafkaEmitterFromConfig creates a KafkaEmitter from configuration.
// Returns nil if brokers are not configured (use LoggerEmitter instead).
func NewKafkaEmitterFromConfig(brokers, topic, clientID string, logger zerolog.Logger) (*KafkaEmitter, error) {
	if brokers == "" {
		return nil, nil // Not configured, caller should use LoggerEmitter
	}

	brokerList := strings.Split(brokers, ",")
	// Trim whitespace from each broker address
	for i := range brokerList {
		brokerList[i] = strings.TrimSpace(brokerList[i])
	}

	if topic == "" {
		topic = "audit.identity" // Default topic
	}

	return NewKafkaEmitter(brokerList, topic, clientID, logger), nil
}

// BuildEvent constructs an audit event from common parameters.
// Automatically generates event ID, hash, and timestamps.
func BuildEvent(orgID, actorID uuid.UUID, actorType, action, targetType string, targetID *uuid.UUID) Event {
	eventID := uuid.New()
	now := time.Now().UTC()

	event := Event{
		EventID:   eventID,
		OrgID:     orgID,
		ActorID:   actorID,
		ActorType: actorType,
		Action:    action,
		TargetType: targetType,
		TargetID:  targetID,
		CreatedAt: now,
	}

	// Compute hash of event payload (excluding hash/signature fields for consistency)
	hash := computeEventHash(event)
	event.Hash = hash

	return event
}

// BuildEventFromRequest enriches an audit event with HTTP request metadata.
func BuildEventFromRequest(event Event, r *http.Request) Event {
	event.IPAddress = getClientIP(r)
	event.UserAgent = r.Header.Get("User-Agent")
	if event.Resource == "" {
		event.Resource = r.Method + " " + r.URL.Path
	}
	return event
}

// computeEventHash computes SHA256 hash of event payload (excluding hash/signature).
func computeEventHash(event Event) string {
	// Create a copy without hash/signature for hashing
	eventCopy := event
	eventCopy.Hash = ""
	eventCopy.Signature = ""
	eventCopy.DeliveredAt = nil

	payload, err := json.Marshal(eventCopy)
	if err != nil {
		// Fallback: hash the string representation
		payload = []byte(fmt.Sprintf("%+v", eventCopy))
	}

	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

// getClientIP extracts the client IP from the request, handling proxies.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (from load balancer/proxy)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	// Fallback to RemoteAddr
	return r.RemoteAddr
}

// Common action constants for consistency.
const (
	ActionOrgCreate   = "org.create"
	ActionOrgUpdate   = "org.update"
	ActionOrgSuspend  = "org.suspend"
	ActionUserInvite  = "user.invite"
	ActionUserCreate  = "user.create"
	ActionUserUpdate  = "user.update"
	ActionUserSuspend = "user.suspend"
	ActionUserActivate = "user.activate"
	ActionUserDelete   = "user.delete"
	ActionRoleAssign   = "role.assign"
	ActionRoleRevoke   = "role.revoke"
	ActionAPIKeyIssue  = "api_key.issue"
	ActionAPIKeyRevoke = "api_key.revoke"
	ActionAccountLockout = "account.lockout"
	ActionRecoveryInitiate = "recovery.initiate"
	ActionRecoveryApprove = "recovery.approve"
	ActionRecoveryReject = "recovery.reject"
	ActionRecoveryComplete = "recovery.complete"
)

// Common target type constants.
const (
	TargetTypeOrg     = "org"
	TargetTypeUser    = "user"
	TargetTypeRole    = "role"
	TargetTypeAPIKey  = "api_key"
)

// Common actor type constants.
const (
	ActorTypeUser          = "user"
	ActorTypeServiceAccount = "service_account"
	ActorTypeSystem        = "system"
)

