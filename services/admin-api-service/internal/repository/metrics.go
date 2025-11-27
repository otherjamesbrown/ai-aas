package repository

import (
	"context"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/api/handlers"
)

// WithMetrics wraps a repository operation with duration metrics
func WithMetrics(operation, table string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start).Seconds()

	handlers.DBQueryDuration.WithLabelValues(operation, table).Observe(duration)

	return err
}

// UpdateConnectionMetrics updates database connection pool metrics
func (db *DB) UpdateConnectionMetrics() {
	if db.pool == nil {
		return
	}

	stats := db.pool.Stat()
	handlers.DBConnectionsActive.Set(float64(stats.AcquiredConns()))
}

// StartMetricsUpdater starts a goroutine that periodically updates connection metrics
func (db *DB) StartMetricsUpdater(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				db.UpdateConnectionMetrics()
			}
		}
	}()
}

