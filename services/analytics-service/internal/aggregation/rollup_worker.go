// Package aggregation provides rollup workers for TimescaleDB continuous aggregates.
//
// Purpose:
//   This package orchestrates periodic rollup jobs that aggregate usage_events into
//   hourly and daily rollups, and updates freshness_status for monitoring.
//
package aggregation

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// Worker orchestrates rollup jobs.
type Worker struct {
	store      *postgres.Store
	logger     *zap.Logger
	interval   time.Duration
	workers    int
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// Config holds worker configuration.
type Config struct {
	Store    *postgres.Store
	Logger   *zap.Logger
	Interval time.Duration
	Workers  int
}

// NewWorker creates a new rollup worker.
func NewWorker(cfg Config) *Worker {
	return &Worker{
		store:    cfg.Store,
		logger:   cfg.Logger,
		interval: cfg.Interval,
		workers:  cfg.Workers,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the rollup worker loop.
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("starting rollup worker",
		zap.Duration("interval", w.interval),
		zap.Int("workers", w.workers),
	)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run initial rollup immediately
	if err := w.runRollups(ctx); err != nil {
		w.logger.Error("initial rollup failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("rollup worker stopping due to context cancellation")
			close(w.doneCh)
			return nil

		case <-w.stopCh:
			w.logger.Info("rollup worker stopping")
			close(w.doneCh)
			return nil

		case <-ticker.C:
			if err := w.runRollups(ctx); err != nil {
				w.logger.Error("rollup failed", zap.Error(err))
				// Continue running despite errors
			}
		}
	}
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	close(w.stopCh)
	<-w.doneCh
}

// runRollups executes hourly and daily rollups.
func (w *Worker) runRollups(ctx context.Context) error {
	now := time.Now().UTC()

	// Calculate time windows
	hourEnd := now.Truncate(time.Hour)
	hourStart := hourEnd.Add(-1 * time.Hour)

	dayEnd := now.Truncate(24 * time.Hour)
	dayStart := dayEnd.Add(-24 * time.Hour)

	w.logger.Info("running rollups",
		zap.Time("hour_start", hourStart),
		zap.Time("hour_end", hourEnd),
		zap.Time("day_start", dayStart),
		zap.Time("day_end", dayEnd),
	)

	// Run hourly rollup
	if err := w.runHourlyRollup(ctx, hourStart, hourEnd); err != nil {
		return fmt.Errorf("hourly rollup failed: %w", err)
	}

	// Run daily rollup
	if err := w.runDailyRollup(ctx, dayStart, dayEnd); err != nil {
		return fmt.Errorf("daily rollup failed: %w", err)
	}

	// Update freshness status
	if err := w.updateFreshnessStatus(ctx); err != nil {
		w.logger.Warn("failed to update freshness status", zap.Error(err))
		// Don't fail the rollup if freshness update fails
	}

	return nil
}

// runHourlyRollup executes the hourly rollup transform.
func (w *Worker) runHourlyRollup(ctx context.Context, start, end time.Time) error {
	query := `
		INSERT INTO analytics_hourly_rollups (
			bucket_start,
			organization_id,
			model_id,
			request_count,
			tokens_total,
			error_count,
			cost_total,
			updated_at
		)
		SELECT
			date_trunc('hour', occurred_at) AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`

	_, err := w.store.Pool().Exec(ctx, query, start, end)
	if err != nil {
		return fmt.Errorf("execute hourly rollup: %w", err)
	}

	w.logger.Debug("hourly rollup completed",
		zap.Time("start", start),
		zap.Time("end", end),
	)

	return nil
}

// runDailyRollup executes the daily rollup transform.
func (w *Worker) runDailyRollup(ctx context.Context, start, end time.Time) error {
	query := `
		INSERT INTO analytics_daily_rollups (
			bucket_start,
			organization_id,
			model_id,
			request_count,
			tokens_total,
			error_count,
			cost_total,
			updated_at
		)
		SELECT
			date_trunc('day', occurred_at)::date AS bucket_start,
			org_id AS organization_id,
			model_id,
			COUNT(*) AS request_count,
			SUM(input_tokens + output_tokens) AS tokens_total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
			SUM(cost_estimate_cents / 100.0) AS cost_total,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1, 2, 3
		ON CONFLICT (bucket_start, organization_id, model_id)
		DO UPDATE SET
			request_count = EXCLUDED.request_count,
			tokens_total  = EXCLUDED.tokens_total,
			error_count   = EXCLUDED.error_count,
			cost_total    = EXCLUDED.cost_total,
			updated_at    = NOW()
	`

	_, err := w.store.Pool().Exec(ctx, query, start, end)
	if err != nil {
		return fmt.Errorf("execute daily rollup: %w", err)
	}

	w.logger.Debug("daily rollup completed",
		zap.Time("start", start),
		zap.Time("end", end),
	)

	return nil
}

// updateFreshnessStatus updates the freshness_status table.
func (w *Worker) updateFreshnessStatus(ctx context.Context) error {
	query := `
		INSERT INTO analytics.freshness_status (
			org_id, model_id, last_event_at, last_rollup_at, lag_seconds, status, updated_at
		)
		SELECT
			org_id,
			model_id,
			MAX(occurred_at) AS last_event_at,
			NOW() AS last_rollup_at,
			EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER AS lag_seconds,
			CASE
				WHEN EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER < 300 THEN 'fresh'
				WHEN EXTRACT(EPOCH FROM (NOW() - MAX(occurred_at)))::INTEGER < 600 THEN 'stale'
				ELSE 'delayed'
			END AS status,
			NOW() AS updated_at
		FROM analytics.usage_events
		WHERE occurred_at >= NOW() - INTERVAL '24 hours'
		GROUP BY org_id, model_id
		ON CONFLICT (org_id, model_id)
		DO UPDATE SET
			last_event_at = EXCLUDED.last_event_at,
			last_rollup_at = EXCLUDED.last_rollup_at,
			lag_seconds = EXCLUDED.lag_seconds,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`

	_, err := w.store.Pool().Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("update freshness status: %w", err)
	}

	return nil
}

