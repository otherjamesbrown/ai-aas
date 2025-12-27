package ingestion

import (
	"testing"
	"time"

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
