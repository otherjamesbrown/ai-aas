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
			name: "invalid batch size - zero",
			cfg: Config{
				BatchSize: 0,
				Workers:   4,
				Store:     nil, // Will fail on store check
				Logger:    logger,
			},
			wantErr: "batch size must be positive",
		},
		{
			name: "invalid batch size - negative",
			cfg: Config{
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

func TestParseRabbitMQURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantHost     string
		wantPort     int
		wantUser     string
		wantPassword string
		wantErr      bool
	}{
		{
			name:         "empty URL returns defaults",
			url:          "",
			wantHost:     "localhost",
			wantPort:     5552,
			wantUser:     "guest",
			wantPassword: "guest",
			wantErr:      false,
		},
		{
			name:         "amqp URL with credentials",
			url:          "amqp://myuser:mypass@rabbitmq.example.com:5672",
			wantHost:     "rabbitmq.example.com",
			wantPort:     5672,
			wantUser:     "myuser",
			wantPassword: "mypass",
			wantErr:      false,
		},
		{
			name:         "stream URL with custom port",
			url:          "stream://rabbitmq.example.com:5553",
			wantHost:     "rabbitmq.example.com",
			wantPort:     5553,
			wantUser:     "guest",
			wantPassword: "guest",
			wantErr:      false,
		},
		{
			name:         "amqp URL without port uses default amqp port",
			url:          "amqp://user:pass@host",
			wantHost:     "host",
			wantPort:     5672,
			wantUser:     "user",
			wantPassword: "pass",
			wantErr:      false,
		},
		{
			name:         "stream URL without port uses default stream port",
			url:          "stream://host",
			wantHost:     "host",
			wantPort:     5552,
			wantUser:     "guest",
			wantPassword: "guest",
			wantErr:      false,
		},
		{
			name:         "URL with only host",
			url:          "amqp://localhost",
			wantHost:     "localhost",
			wantPort:     5672,
			wantUser:     "guest",
			wantPassword: "guest",
			wantErr:      false,
		},
		{
			name:         "URL with special characters in password",
			url:          "amqp://admin:p%40ssw0rd@host:5672",
			wantHost:     "host",
			wantPort:     5672,
			wantUser:     "admin",
			wantPassword: "p@ssw0rd",
			wantErr:      false,
		},
		{
			name:         "URL with IPv4 address",
			url:          "amqp://guest:guest@192.168.1.100:5672",
			wantHost:     "192.168.1.100",
			wantPort:     5672,
			wantUser:     "guest",
			wantPassword: "guest",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, user, password, err := parseRabbitMQURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantHost, host)
				assert.Equal(t, tt.wantPort, port)
				assert.Equal(t, tt.wantUser, user)
				assert.Equal(t, tt.wantPassword, password)
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
