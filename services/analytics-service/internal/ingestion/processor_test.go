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
				EventID:      "123e4567-e89b-12d3-a456-426614174000",
				OrgID:        "123e4567-e89b-12d3-a456-426614174001",
				ModelID:      "123e4567-e89b-12d3-a456-426614174002",
				OccurredAt:   now,
				InputTokens:  100,
				OutputTokens: 50,
				LatencyMS:    250,
				Status:       "success",
				CostEstimate: 0.0015,
				Metadata:     map[string]interface{}{"key": "value"},
			},
			wantErr: "",
		},
		{
			name: "valid event without model_id",
			event: Event{
				EventID:      "123e4567-e89b-12d3-a456-426614174000",
				OrgID:        "123e4567-e89b-12d3-a456-426614174001",
				ModelID:      "",
				OccurredAt:   now,
				InputTokens:  100,
				OutputTokens: 50,
				Status:       "success",
			},
			wantErr: "",
		},
		{
			name: "invalid event_id",
			event: Event{
				EventID: "not-a-uuid",
				OrgID:   "123e4567-e89b-12d3-a456-426614174001",
			},
			wantErr: "invalid event_id",
		},
		{
			name: "invalid org_id",
			event: Event{
				EventID: "123e4567-e89b-12d3-a456-426614174000",
				OrgID:   "not-a-uuid",
			},
			wantErr: "invalid org_id",
		},
		{
			name: "invalid model_id",
			event: Event{
				EventID: "123e4567-e89b-12d3-a456-426614174000",
				OrgID:   "123e4567-e89b-12d3-a456-426614174001",
				ModelID: "not-a-uuid",
			},
			wantErr: "invalid model_id",
		},
		{
			name: "event with error status",
			event: Event{
				EventID:      "123e4567-e89b-12d3-a456-426614174000",
				OrgID:        "123e4567-e89b-12d3-a456-426614174001",
				OccurredAt:   now,
				Status:       "error",
				ErrorCode:    "RATE_LIMIT_EXCEEDED",
				InputTokens:  0,
				OutputTokens: 0,
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
				assert.Equal(t, tt.event.InputTokens, dbEvent.InputTokens)
				assert.Equal(t, tt.event.OutputTokens, dbEvent.OutputTokens)
				assert.Equal(t, tt.event.Status, dbEvent.Status)
				assert.Equal(t, tt.event.ErrorCode, dbEvent.ErrorCode)
				assert.Equal(t, tt.event.LatencyMS, dbEvent.LatencyMS)
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
		EventID:      "123e4567-e89b-12d3-a456-426614174000",
		OrgID:        "123e4567-e89b-12d3-a456-426614174001",
		ModelID:      "123e4567-e89b-12d3-a456-426614174002",
		OccurredAt:   now,
		InputTokens:  150,
		OutputTokens: 75,
		LatencyMS:    300,
		Status:       "success",
		ErrorCode:    "",
		CostEstimate: 0.025,
		Metadata:     map[string]interface{}{"request_id": "req-123"},
	}

	dbEvent, err := p.convertEvent(event)
	require.NoError(t, err)

	// Verify field mapping
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", dbEvent.EventID.String())
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174001", dbEvent.OrgID.String())
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174002", dbEvent.ModelID.String())
	assert.Equal(t, now, dbEvent.OccurredAt)
	assert.Equal(t, int64(150), dbEvent.InputTokens)
	assert.Equal(t, int64(75), dbEvent.OutputTokens)
	assert.Equal(t, 300, dbEvent.LatencyMS)
	assert.Equal(t, "success", dbEvent.Status)
	assert.Equal(t, 0.025, dbEvent.CostEstimateCents)

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
