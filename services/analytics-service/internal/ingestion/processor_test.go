package ingestion

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

func TestProcessor_convertEvent(t *testing.T) {
	logger := zap.NewNop()
	// Processor with nil store for unit testing conversion logic
	p := &Processor{
		store:  nil,
		logger: logger,
	}

	now := time.Now().UTC()

	tests := []struct {
		name    string
		event   Event
		wantErr string
	}{
		{
			name: "valid event with all fields",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				RequestID:      "req-123",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "123e4567-e89b-12d3-a456-426614174002",
				Timestamp:      now,
				Model:          "gpt-4o",
				BackendID:      "backend-1",
				TokensInput:    100,
				TokensOutput:   50,
				CostUSD:        0.0015,
				LatencyMS:      250,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
				Metadata:       map[string]interface{}{"key": "value"},
			},
			wantErr: "",
		},
		{
			name: "valid event without api_key_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "",
				Timestamp:      now,
				Model:          "gpt-3.5-turbo",
				TokensInput:    100,
				TokensOutput:   50,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
			},
			wantErr: "",
		},
		{
			name: "invalid record_id",
			event: Event{
				RecordID:       "not-a-uuid",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
			},
			wantErr: "invalid record_id",
		},
		{
			name: "invalid organization_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "not-a-uuid",
			},
			wantErr: "invalid organization_id",
		},
		{
			name: "invalid api_key_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "not-a-uuid",
			},
			wantErr: "invalid api_key_id",
		},
		{
			name: "event with rate limited status",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
				Timestamp:      now,
				Model:          "gpt-4o",
				LimitState:     "RATE_LIMITED",
				DecisionReason: "RATE_LIMIT",
				TokensInput:    0,
				TokensOutput:   0,
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbEvent, err := p.convertEvent(tt.event)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, int64(tt.event.TokensInput), dbEvent.InputTokens)
				assert.Equal(t, int64(tt.event.TokensOutput), dbEvent.OutputTokens)
				assert.Equal(t, tt.event.LatencyMS, dbEvent.LatencyMS)
				// Verify metadata contains routing fields
				if tt.event.Model != "" {
					assert.Equal(t, tt.event.Model, dbEvent.Metadata["model"])
				}
				if tt.event.BackendID != "" {
					assert.Equal(t, tt.event.BackendID, dbEvent.Metadata["backend_id"])
				}
			}
		})
	}
}

func TestProcessor_convertEvent_FieldMapping(t *testing.T) {
	logger := zap.NewNop()
	p := &Processor{
		store:  nil,
		logger: logger,
	}

	now := time.Now().UTC()
	event := Event{
		RecordID:       "123e4567-e89b-12d3-a456-426614174000",
		RequestID:      "req-123",
		OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
		APIKeyID:       "123e4567-e89b-12d3-a456-426614174002",
		Timestamp:      now,
		Model:          "gpt-4o",
		BackendID:      "backend-vllm-1",
		TokensInput:    150,
		TokensOutput:   75,
		CostUSD:        0.025,
		LatencyMS:      300,
		LimitState:     "WITHIN_LIMIT",
		DecisionReason: "PRIMARY",
		TraceID:        "trace-abc",
		SpanID:         "span-123",
		Metadata:       map[string]interface{}{"custom_key": "custom_value"},
	}

	dbEvent, err := p.convertEvent(event)
	require.NoError(t, err)

	// Verify field mapping
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", dbEvent.EventID.String())
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174001", dbEvent.OrgID.String())
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174002", dbEvent.ActorID.String()) // api_key_id → actor_id
	assert.Equal(t, now, dbEvent.OccurredAt)                                          // timestamp → occurred_at
	assert.Equal(t, int64(150), dbEvent.InputTokens)
	assert.Equal(t, int64(75), dbEvent.OutputTokens)
	assert.Equal(t, 300, dbEvent.LatencyMS)
	assert.Equal(t, "success", dbEvent.Status)
	assert.Equal(t, 0.025, dbEvent.CostEstimateCents)

	// Verify routing metadata is stored
	assert.Equal(t, "gpt-4o", dbEvent.Metadata["model"])
	assert.Equal(t, "backend-vllm-1", dbEvent.Metadata["backend_id"])
	assert.Equal(t, "WITHIN_LIMIT", dbEvent.Metadata["limit_state"])
	assert.Equal(t, "PRIMARY", dbEvent.Metadata["decision_reason"])
	assert.Equal(t, "req-123", dbEvent.Metadata["request_id"])
	assert.Equal(t, "trace-abc", dbEvent.Metadata["trace_id"])
	assert.Equal(t, "span-123", dbEvent.Metadata["span_id"])
	assert.Equal(t, "custom_value", dbEvent.Metadata["custom_key"]) // Original metadata preserved

	// ReceivedAt should be set to current time (approximately)
	assert.WithinDuration(t, time.Now(), dbEvent.ReceivedAt, 5*time.Second)
}

func TestNewProcessor(t *testing.T) {
	logger := zap.NewNop()

	t.Run("creates processor with valid store and logger", func(t *testing.T) {
		// We'd need a mock store here, but for basic test we can verify nil handling
		p := NewProcessor(nil, logger)
		assert.NotNil(t, p)
		assert.Equal(t, logger, p.logger)
	})
}

func TestProcessor_ProcessBatch_EmptyBatch(t *testing.T) {
	logger := zap.NewNop()
	p := &Processor{
		store:  nil,
		logger: logger,
	}

	// Empty batch should return nil without error
	err := p.ProcessBatch(nil, []Event{}, 0)
	assert.NoError(t, err)
}

// UsageEvent is used for testing field mapping
func TestUsageEventFields(t *testing.T) {
	// Verify UsageEvent struct has expected fields
	event := postgres.UsageEvent{}

	// These should compile - verifies struct field names
	_ = event.EventID
	_ = event.OrgID
	_ = event.OccurredAt
	_ = event.ReceivedAt
	_ = event.ModelID
	_ = event.ActorID
	_ = event.InputTokens
	_ = event.OutputTokens
	_ = event.LatencyMS
	_ = event.Status
	_ = event.ErrorCode
	_ = event.CostEstimateCents
	_ = event.Metadata
}

// TestProcessor_convertEvent_SchemaMapping tests the complete field mapping
// between UsageRecord (api-router) and UsageEvent (analytics-service)
func TestProcessor_convertEvent_SchemaMapping(t *testing.T) {
	logger := zap.NewNop()
	p := &Processor{
		store:  nil,
		logger: logger,
	}

	tests := []struct {
		name    string
		event   Event
		want    func(*testing.T, postgres.UsageEvent)
		wantErr string
	}{
		{
			name: "complete field mapping - all fields present",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				RequestID:      "req-abc123",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "323e4567-e89b-12d3-a456-426614174002",
				Timestamp:      time.Date(2025, 12, 27, 10, 30, 0, 0, time.UTC),
				Model:          "meta-llama/Llama-3.1-8B-Instruct",
				BackendID:      "backend-vllm-1",
				TokensInput:    256,
				TokensOutput:   128,
				CostUSD:        0.0034,
				LatencyMS:      450,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
				RetryCount:     0,
				TraceID:        "trace-xyz789",
				SpanID:         "span-456",
				Metadata:       map[string]interface{}{"custom": "data"},
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				// Verify core ID mappings
				assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", got.EventID.String(), "record_id should map to EventID")
				assert.Equal(t, "223e4567-e89b-12d3-a456-426614174001", got.OrgID.String(), "organization_id should map to OrgID")
				assert.Equal(t, "323e4567-e89b-12d3-a456-426614174002", got.ActorID.String(), "api_key_id should map to ActorID")

				// Verify timestamp mapping
				expectedTime := time.Date(2025, 12, 27, 10, 30, 0, 0, time.UTC)
				assert.Equal(t, expectedTime, got.OccurredAt, "timestamp should map to OccurredAt")
				assert.WithinDuration(t, time.Now(), got.ReceivedAt, 5*time.Second, "ReceivedAt should be set to current time")

				// Verify token counts (int → int64 conversion)
				assert.Equal(t, int64(256), got.InputTokens, "tokens_input should map to InputTokens as int64")
				assert.Equal(t, int64(128), got.OutputTokens, "tokens_output should map to OutputTokens as int64")

				// Verify cost and latency
				assert.Equal(t, 0.0034, got.CostEstimateCents, "cost_usd should map to CostEstimateCents")
				assert.Equal(t, 450, got.LatencyMS, "latency_ms should map to LatencyMS")

				// Verify status mapping (WITHIN_LIMIT → success)
				assert.Equal(t, "success", got.Status, "WITHIN_LIMIT should map to status=success")

				// Verify metadata contains routing fields
				assert.Equal(t, "meta-llama/Llama-3.1-8B-Instruct", got.Metadata["model"], "model should be stored in metadata")
				assert.Equal(t, "backend-vllm-1", got.Metadata["backend_id"], "backend_id should be stored in metadata")
				assert.Equal(t, "WITHIN_LIMIT", got.Metadata["limit_state"], "limit_state should be stored in metadata")
				assert.Equal(t, "PRIMARY", got.Metadata["decision_reason"], "decision_reason should be stored in metadata")
				assert.Equal(t, "req-abc123", got.Metadata["request_id"], "request_id should be stored in metadata")
				assert.Equal(t, "trace-xyz789", got.Metadata["trace_id"], "trace_id should be stored in metadata")
				assert.Equal(t, "span-456", got.Metadata["span_id"], "span_id should be stored in metadata")

				// Verify original metadata is preserved
				assert.Equal(t, "data", got.Metadata["custom"], "original metadata should be preserved")

				// Verify ModelID is Nil (no mapping yet)
				assert.Equal(t, uuid.Nil, got.ModelID, "ModelID should be Nil (no registry mapping)")
			},
		},
		{
			name: "optional fields - empty api_key_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "", // Empty api_key_id
				Timestamp:      time.Date(2025, 12, 27, 12, 0, 0, 0, time.UTC),
				Model:          "gpt-4o",
				TokensInput:    100,
				TokensOutput:   50,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.Equal(t, uuid.Nil, got.ActorID, "empty api_key_id should map to Nil ActorID")
				assert.Equal(t, "gpt-4o", got.Metadata["model"], "model should still be in metadata")
			},
		},
		{
			name: "optional fields - empty backend_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "323e4567-e89b-12d3-a456-426614174002",
				Timestamp:      time.Date(2025, 12, 27, 12, 0, 0, 0, time.UTC),
				Model:          "gpt-4o",
				BackendID:      "", // Empty backend_id
				TokensInput:    100,
				TokensOutput:   50,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "FAILOVER",
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.Equal(t, "", got.Metadata["backend_id"], "empty backend_id should be stored as empty string")
				assert.Equal(t, "FAILOVER", got.Metadata["decision_reason"], "decision_reason should be stored")
			},
		},
		{
			name: "status mapping - RATE_LIMITED",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				LimitState:     "RATE_LIMITED", // Should map to status=limited
				DecisionReason: "RATE_LIMIT",
				TokensInput:    0,
				TokensOutput:   0,
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.Equal(t, "limited", got.Status, "RATE_LIMITED should map to status=limited")
				assert.Equal(t, "RATE_LIMITED", got.Metadata["limit_state"], "limit_state should be preserved in metadata")
				assert.Equal(t, "RATE_LIMIT", got.Metadata["decision_reason"], "decision_reason should be preserved in metadata")
			},
		},
		{
			name: "status mapping - BUDGET_EXCEEDED",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				LimitState:     "BUDGET_EXCEEDED", // Should also map to status=limited
				DecisionReason: "BUDGET_LIMIT",
				TokensInput:    0,
				TokensOutput:   0,
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.Equal(t, "limited", got.Status, "BUDGET_EXCEEDED should map to status=limited")
				assert.Equal(t, "BUDGET_EXCEEDED", got.Metadata["limit_state"])
			},
		},
		{
			name: "zero values - tokens and cost",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				TokensInput:    0,
				TokensOutput:   0,
				CostUSD:        0.0,
				LatencyMS:      0,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.Equal(t, int64(0), got.InputTokens, "zero tokens_input should be preserved")
				assert.Equal(t, int64(0), got.OutputTokens, "zero tokens_output should be preserved")
				assert.Equal(t, 0.0, got.CostEstimateCents, "zero cost_usd should be preserved")
				assert.Equal(t, 0, got.LatencyMS, "zero latency_ms should be preserved")
			},
		},
		{
			name: "metadata with nil map",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				BackendID:      "backend-1",
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
				Metadata:       nil, // Nil metadata map
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.NotNil(t, got.Metadata, "metadata should be initialized if nil")
				assert.Equal(t, "gpt-4o", got.Metadata["model"], "model should be added to initialized metadata")
				assert.Equal(t, "backend-1", got.Metadata["backend_id"], "backend_id should be added")
			},
		},
		{
			name: "retry count handling",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				RetryCount:     3, // Retries occurred
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "FAILOVER",
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				assert.Equal(t, 3, got.Metadata["retry_count"], "retry_count should be stored in metadata")
			},
		},
		{
			name: "retry count zero - should not be in metadata",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				RetryCount:     0, // No retries
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
			},
			want: func(t *testing.T, got postgres.UsageEvent) {
				_, hasRetryCount := got.Metadata["retry_count"]
				assert.False(t, hasRetryCount, "retry_count=0 should not be stored in metadata")
			},
		},
		{
			name: "invalid record_id",
			event: Event{
				RecordID:       "not-a-uuid",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
			},
			wantErr: "invalid record_id",
		},
		{
			name: "invalid organization_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "not-a-uuid",
			},
			wantErr: "invalid organization_id",
		},
		{
			name: "invalid api_key_id - non-empty invalid UUID",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "invalid-uuid-string",
			},
			wantErr: "invalid api_key_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.convertEvent(tt.event)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				tt.want(t, got)
			}
		})
	}
}

// TestProcessor_convertEvent_CostAccuracy tests cost calculation accuracy
func TestProcessor_convertEvent_CostAccuracy(t *testing.T) {
	logger := zap.NewNop()
	p := &Processor{
		store:  nil,
		logger: logger,
	}

	tests := []struct {
		name     string
		costUSD  float64
		wantCost float64
	}{
		{
			name:     "zero cost",
			costUSD:  0.0,
			wantCost: 0.0,
		},
		{
			name:     "small cost - 0.001 USD",
			costUSD:  0.001,
			wantCost: 0.001,
		},
		{
			name:     "typical cost - 0.05 USD",
			costUSD:  0.05,
			wantCost: 0.05,
		},
		{
			name:     "high precision - 0.123456789 USD",
			costUSD:  0.123456789,
			wantCost: 0.123456789,
		},
		{
			name:     "large cost - 10.50 USD",
			costUSD:  10.50,
			wantCost: 10.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      time.Now().UTC(),
				Model:          "gpt-4o",
				CostUSD:        tt.costUSD,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
			}

			got, err := p.convertEvent(event)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCost, got.CostEstimateCents, "cost_usd should be preserved with full precision")
		})
	}
}

// TestProcessor_convertEvent_TimestampHandling tests timestamp parsing and formatting
func TestProcessor_convertEvent_TimestampHandling(t *testing.T) {
	logger := zap.NewNop()
	p := &Processor{
		store:  nil,
		logger: logger,
	}

	tests := []struct {
		name      string
		timestamp time.Time
	}{
		{
			name:      "UTC timestamp",
			timestamp: time.Date(2025, 12, 27, 10, 30, 45, 123456789, time.UTC),
		},
		{
			name:      "timestamp with different timezone",
			timestamp: time.Date(2025, 12, 27, 10, 30, 45, 0, time.FixedZone("EST", -5*3600)),
		},
		{
			name:      "zero timestamp",
			timestamp: time.Time{},
		},
		{
			name:      "current time",
			timestamp: time.Now(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "223e4567-e89b-12d3-a456-426614174001",
				Timestamp:      tt.timestamp,
				Model:          "gpt-4o",
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
			}

			got, err := p.convertEvent(event)
			require.NoError(t, err)
			assert.Equal(t, tt.timestamp, got.OccurredAt, "timestamp should be preserved exactly")
			assert.WithinDuration(t, time.Now(), got.ReceivedAt, 5*time.Second, "ReceivedAt should be current time")
		})
	}
}
