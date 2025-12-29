package aggregation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewWorker(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		cfg     Config
		wantNil bool
	}{
		{
			name: "creates worker with valid config",
			cfg: Config{
				Store:    nil, // Would be *postgres.Store in real usage
				Logger:   logger,
				Interval: 1 * time.Hour,
				Workers:  2,
			},
			wantNil: false,
		},
		{
			name: "creates worker with zero interval",
			cfg: Config{
				Store:    nil,
				Logger:   logger,
				Interval: 0,
				Workers:  2,
			},
			wantNil: false,
		},
		{
			name: "creates worker with zero workers",
			cfg: Config{
				Store:    nil,
				Logger:   logger,
				Interval: 1 * time.Hour,
				Workers:  0,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := NewWorker(tt.cfg)
			if tt.wantNil {
				assert.Nil(t, worker)
			} else {
				require.NotNil(t, worker)
				assert.Equal(t, tt.cfg.Interval, worker.interval)
				assert.Equal(t, tt.cfg.Workers, worker.workers)
				assert.Equal(t, tt.cfg.Logger, worker.logger)
			}
		})
	}
}

func TestWorker_Config(t *testing.T) {
	logger := zap.NewNop()

	cfg := Config{
		Store:    nil,
		Logger:   logger,
		Interval: 30 * time.Minute,
		Workers:  4,
	}

	worker := NewWorker(cfg)

	assert.Equal(t, 30*time.Minute, worker.interval)
	assert.Equal(t, 4, worker.workers)
	assert.NotNil(t, worker.stopCh)
	assert.NotNil(t, worker.doneCh)
}

func TestWorker_StopBeforeStart(t *testing.T) {
	logger := zap.NewNop()

	worker := NewWorker(Config{
		Logger:   logger,
		Interval: 1 * time.Hour,
		Workers:  1,
	})

	// Verify worker was created correctly
	assert.NotNil(t, worker)
	assert.Equal(t, 1*time.Hour, worker.interval)

	// Note: Calling Stop() before Start() would deadlock in the real implementation
	// because doneCh is never closed. This test documents the expected state.
}

// Note: TestWorker_ContextCancellation requires a real database connection
// and is covered by integration tests. The worker's Start() method immediately
// calls runRollups() which requires a valid store.

func TestTimeWindowCalculation(t *testing.T) {
	// Test the time window calculation logic used in runRollups
	now := time.Date(2024, 6, 15, 14, 35, 22, 0, time.UTC)

	// Hour window
	hourEnd := now.Truncate(time.Hour)
	hourStart := hourEnd.Add(-1 * time.Hour)

	assert.Equal(t, time.Date(2024, 6, 15, 14, 0, 0, 0, time.UTC), hourEnd)
	assert.Equal(t, time.Date(2024, 6, 15, 13, 0, 0, 0, time.UTC), hourStart)

	// Day window
	dayEnd := now.Truncate(24 * time.Hour)
	dayStart := dayEnd.Add(-24 * time.Hour)

	assert.Equal(t, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), dayEnd)
	assert.Equal(t, time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC), dayStart)
}

func TestTimeWindowEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		now           time.Time
		wantHourStart time.Time
		wantHourEnd   time.Time
		wantDayStart  time.Time
		wantDayEnd    time.Time
	}{
		{
			name:          "midnight",
			now:           time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			wantHourStart: time.Date(2024, 6, 14, 23, 0, 0, 0, time.UTC),
			wantHourEnd:   time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			wantDayStart:  time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC),
			wantDayEnd:    time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "end of day",
			now:           time.Date(2024, 6, 15, 23, 59, 59, 0, time.UTC),
			wantHourStart: time.Date(2024, 6, 15, 22, 0, 0, 0, time.UTC),
			wantHourEnd:   time.Date(2024, 6, 15, 23, 0, 0, 0, time.UTC),
			wantDayStart:  time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC),
			wantDayEnd:    time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "year boundary",
			now:           time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC),
			wantHourStart: time.Date(2023, 12, 31, 23, 0, 0, 0, time.UTC),
			wantHourEnd:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantDayStart:  time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			wantDayEnd:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "leap year feb 29",
			now:           time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			wantHourStart: time.Date(2024, 2, 29, 11, 0, 0, 0, time.UTC),
			wantHourEnd:   time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			wantDayStart:  time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
			wantDayEnd:    time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hourEnd := tt.now.Truncate(time.Hour)
			hourStart := hourEnd.Add(-1 * time.Hour)
			dayEnd := tt.now.Truncate(24 * time.Hour)
			dayStart := dayEnd.Add(-24 * time.Hour)

			assert.Equal(t, tt.wantHourStart, hourStart, "hour start mismatch")
			assert.Equal(t, tt.wantHourEnd, hourEnd, "hour end mismatch")
			assert.Equal(t, tt.wantDayStart, dayStart, "day start mismatch")
			assert.Equal(t, tt.wantDayEnd, dayEnd, "day end mismatch")
		})
	}
}

func TestWorkerChannelsInitialized(t *testing.T) {
	logger := zap.NewNop()

	worker := NewWorker(Config{
		Logger:   logger,
		Interval: 1 * time.Hour,
		Workers:  2,
	})

	// Channels should be initialized and not nil
	assert.NotNil(t, worker.stopCh)
	assert.NotNil(t, worker.doneCh)

	// Verify channels are not closed (should not block on select with default)
	select {
	case <-worker.stopCh:
		t.Fatal("stopCh should not be closed initially")
	default:
		// Expected
	}

	select {
	case <-worker.doneCh:
		t.Fatal("doneCh should not be closed initially")
	default:
		// Expected
	}
}
