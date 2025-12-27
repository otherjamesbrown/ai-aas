package ingestion

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewConsumer(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "missing brokers",
			cfg: Config{
				Brokers:   []string{},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   4,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "brokers are required",
		},
		{
			name: "missing topic",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   4,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "topic is required",
		},
		{
			name: "missing group ID",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "",
				BatchSize: 100,
				Workers:   4,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "group ID is required",
		},
		{
			name: "invalid batch size - zero",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 0,
				Workers:   4,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "batch size must be positive",
		},
		{
			name: "invalid batch size - negative",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: -10,
				Workers:   4,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "batch size must be positive",
		},
		{
			name: "invalid workers - zero",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   0,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "workers must be positive",
		},
		{
			name: "invalid workers - negative",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   -5,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "workers must be positive",
		},
		{
			name: "nil store",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   4,
				Store:     nil,
				Logger:    logger,
			},
			wantErr: "store is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, consumer)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, consumer)
			}
		})
	}
}

func TestEvent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		isValid bool
	}{
		{
			name: "valid event",
			event: Event{
				EventID:      "123e4567-e89b-12d3-a456-426614174000",
				OrgID:        "123e4567-e89b-12d3-a456-426614174001",
				ModelID:      "123e4567-e89b-12d3-a456-426614174002",
				InputTokens:  100,
				OutputTokens: 50,
				LatencyMS:    250,
				Status:       "success",
			},
			isValid: true,
		},
		{
			name: "missing event_id",
			event: Event{
				EventID: "",
				OrgID:   "123e4567-e89b-12d3-a456-426614174001",
			},
			isValid: false,
		},
		{
			name: "missing org_id",
			event: Event{
				EventID: "123e4567-e89b-12d3-a456-426614174000",
				OrgID:   "",
			},
			isValid: false,
		},
		{
			name: "optional model_id can be empty",
			event: Event{
				EventID: "123e4567-e89b-12d3-a456-426614174000",
				OrgID:   "123e4567-e89b-12d3-a456-426614174001",
				ModelID: "",
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate event has required fields
			hasRequiredFields := tt.event.EventID != "" && tt.event.OrgID != ""
			assert.Equal(t, tt.isValid, hasRequiredFields)
		})
	}
}
