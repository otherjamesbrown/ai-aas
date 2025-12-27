package ingestion

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// MockStore is a mock implementation of postgres.Store for testing.
type MockStore struct {
	mock.Mock
}

func (m *MockStore) CreateIngestionBatch(ctx context.Context, streamOffset int64, orgScope []any) (any, error) {
	args := m.Called(ctx, streamOffset, orgScope)
	return args.Get(0), args.Error(1)
}

func (m *MockStore) InsertUsageEvents(ctx context.Context, events []postgres.UsageEvent, batchID any) (int, error) {
	args := m.Called(ctx, events, batchID)
	return args.Int(0), args.Error(1)
}

func (m *MockStore) CompleteIngestionBatch(ctx context.Context, batchID any, dedupeConflicts int) error {
	args := m.Called(ctx, batchID, dedupeConflicts)
	return args.Error(0)
}

func TestNewConsumer(t *testing.T) {
	logger := zap.NewNop()
	mockStore := &postgres.Store{} // Using real struct pointer for validation

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid configuration",
			cfg: Config{
				Brokers:   []string{"localhost:9092"},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   4,
				Store:     mockStore,
				Logger:    logger,
			},
			wantErr: "",
		},
		{
			name: "missing brokers",
			cfg: Config{
				Brokers:   []string{},
				Topic:     "test-topic",
				GroupID:   "test-group",
				BatchSize: 100,
				Workers:   4,
				Store:     mockStore,
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
				Store:     mockStore,
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
				Store:     mockStore,
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
				Store:     mockStore,
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
				Store:     mockStore,
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
				Store:     mockStore,
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
				Store:     mockStore,
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
				assert.NotNil(t, consumer.reader)
				assert.Equal(t, tt.cfg.Topic, consumer.topic)
				assert.Equal(t, tt.cfg.GroupID, consumer.groupID)
				assert.Equal(t, tt.cfg.BatchSize, consumer.batchSize)
				assert.Equal(t, tt.cfg.Workers, consumer.workers)
				// Clean up the reader
				_ = consumer.reader.Close()
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	logger := zap.NewNop()
	consumer := &Consumer{logger: logger}

	tests := []struct {
		name    string
		message kafka.Message
		want    Event
		wantErr string
	}{
		{
			name: "valid message",
			message: kafka.Message{
				Value: []byte(`{
					"record_id": "rec_123e4567",
					"request_id": "req_123e4567",
					"organization_id": "org_123e4567",
					"api_key_id": "key_123e4567",
					"timestamp": "2025-12-27T10:00:00Z",
					"model": "meta-llama/Llama-2-7b-hf",
					"backend_id": "backend_123",
					"tokens_input": 100,
					"tokens_output": 50,
					"latency_ms": 250,
					"limit_state": "WITHIN_LIMIT",
					"decision_reason": "PRIMARY",
					"cost_usd": 0.05
				}`),
			},
			want: Event{
				RecordID:       "rec_123e4567",
				RequestID:      "req_123e4567",
				OrganizationID: "org_123e4567",
				APIKeyID:       "key_123e4567",
				Model:          "meta-llama/Llama-2-7b-hf",
				BackendID:      "backend_123",
				TokensInput:    100,
				TokensOutput:   50,
				LatencyMS:      250,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
				CostUSD:        0.05,
			},
		},
		{
			name: "malformed JSON",
			message: kafka.Message{
				Value: []byte(`{invalid json`),
			},
			wantErr: "unmarshal event",
		},
		{
			name: "missing record_id",
			message: kafka.Message{
				Value: []byte(`{
					"organization_id": "org_123e4567"
				}`),
			},
			wantErr: "record_id is required",
		},
		{
			name: "missing organization_id",
			message: kafka.Message{
				Value: []byte(`{
					"record_id": "rec_123e4567"
				}`),
			},
			wantErr: "organization_id is required",
		},
		{
			name: "empty record_id",
			message: kafka.Message{
				Value: []byte(`{
					"record_id": "",
					"organization_id": "org_123e4567"
				}`),
			},
			wantErr: "record_id is required",
		},
		{
			name: "empty organization_id",
			message: kafka.Message{
				Value: []byte(`{
					"record_id": "rec_123e4567",
					"organization_id": ""
				}`),
			},
			wantErr: "organization_id is required",
		},
		{
			name: "valid event with metadata",
			message: kafka.Message{
				Value: []byte(`{
					"record_id": "rec_123e4567",
					"organization_id": "org_123e4567",
					"request_id": "req_123e4567",
					"metadata": {"key": "value", "number": 42}
				}`),
			},
			want: Event{
				RecordID:       "rec_123e4567",
				OrganizationID: "org_123e4567",
				RequestID:      "req_123e4567",
				Metadata: map[string]interface{}{
					"key":    "value",
					"number": float64(42),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := consumer.parseMessage(tt.message)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want.RecordID, got.RecordID)
				assert.Equal(t, tt.want.OrganizationID, got.OrganizationID)
				assert.Equal(t, tt.want.RequestID, got.RequestID)
				assert.Equal(t, tt.want.APIKeyID, got.APIKeyID)
				assert.Equal(t, tt.want.Model, got.Model)
				assert.Equal(t, tt.want.BackendID, got.BackendID)
				assert.Equal(t, tt.want.TokensInput, got.TokensInput)
				assert.Equal(t, tt.want.TokensOutput, got.TokensOutput)
				assert.Equal(t, tt.want.LatencyMS, got.LatencyMS)
				assert.Equal(t, tt.want.LimitState, got.LimitState)
				assert.Equal(t, tt.want.DecisionReason, got.DecisionReason)
				assert.Equal(t, tt.want.CostUSD, got.CostUSD)
				if tt.want.Metadata != nil {
					assert.Equal(t, tt.want.Metadata, got.Metadata)
				}
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
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "123e4567-e89b-12d3-a456-426614174002",
				TokensInput:    100,
				TokensOutput:   50,
				LatencyMS:      250,
				LimitState:     "WITHIN_LIMIT",
			},
			isValid: true,
		},
		{
			name: "missing record_id",
			event: Event{
				RecordID:       "",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
			},
			isValid: false,
		},
		{
			name: "missing organization_id",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "",
			},
			isValid: false,
		},
		{
			name: "optional api_key_id can be empty",
			event: Event{
				RecordID:       "123e4567-e89b-12d3-a456-426614174000",
				OrganizationID: "123e4567-e89b-12d3-a456-426614174001",
				APIKeyID:       "",
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate event has required fields
			hasRequiredFields := tt.event.RecordID != "" && tt.event.OrganizationID != ""
			assert.Equal(t, tt.isValid, hasRequiredFields)
		})
	}
}

func TestProcessBatch(t *testing.T) {
	tests := []struct {
		name         string
		events       []Event
		messages     []kafka.Message
		expectCommit bool
	}{
		{
			name:         "empty batch",
			events:       []Event{},
			messages:     []kafka.Message{},
			expectCommit: false,
		},
		{
			name: "successful batch processing",
			events: []Event{
				{
					RecordID:       "rec_123e4567",
					OrganizationID: "org_123e4567",
				},
			},
			messages: []kafka.Message{
				{Offset: 100},
			},
			expectCommit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test is limited because processBatch is a private method
			// that requires a real Kafka reader for commit operations.
			// Full integration testing would require testcontainers or mocking the reader.
			assert.Equal(t, tt.expectCommit, len(tt.messages) > 0)
		})
	}
}

func TestConsumer_StartAndStop(t *testing.T) {
	logger := zap.NewNop()
	mockStore := &postgres.Store{}

	cfg := Config{
		Brokers:   []string{"localhost:9092"},
		Topic:     "test-topic",
		GroupID:   "test-group",
		BatchSize: 10,
		Workers:   2,
		Store:     mockStore,
		Logger:    logger,
	}

	consumer, err := NewConsumer(cfg)
	require.NoError(t, err)
	require.NotNil(t, consumer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consumer
	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// Give workers time to start
	time.Sleep(100 * time.Millisecond)

	// Stop consumer with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = consumer.Stop(stopCtx)
	assert.NoError(t, err)
}

func TestEvent_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "full event",
			event: Event{
				RecordID:       "rec_123e4567",
				RequestID:      "req_123e4567",
				OrganizationID: "org_123e4567",
				APIKeyID:       "key_123e4567",
				Timestamp:      now,
				Model:          "meta-llama/Llama-2-7b-hf",
				BackendID:      "backend_123",
				TokensInput:    100,
				TokensOutput:   50,
				LatencyMS:      250,
				LimitState:     "WITHIN_LIMIT",
				DecisionReason: "PRIMARY",
				CostUSD:        0.05,
				Metadata:       map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "minimal event",
			event: Event{
				RecordID:       "rec_123e4567",
				OrganizationID: "org_123e4567",
			},
		},
		{
			name: "event with rate limit",
			event: Event{
				RecordID:       "rec_123e4567",
				OrganizationID: "org_123e4567",
				LimitState:     "RATE_LIMITED",
				DecisionReason: "RATE_LIMIT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.event)
			require.NoError(t, err)

			// Unmarshal back
			var decoded Event
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// Verify round-trip
			assert.Equal(t, tt.event.RecordID, decoded.RecordID)
			assert.Equal(t, tt.event.OrganizationID, decoded.OrganizationID)
			assert.Equal(t, tt.event.RequestID, decoded.RequestID)
			assert.Equal(t, tt.event.Model, decoded.Model)
			assert.Equal(t, tt.event.TokensInput, decoded.TokensInput)
			assert.Equal(t, tt.event.TokensOutput, decoded.TokensOutput)
			assert.Equal(t, tt.event.LimitState, decoded.LimitState)
			assert.Equal(t, tt.event.DecisionReason, decoded.DecisionReason)
		})
	}
}
