-- +goose Up
-- Migration: Create benchmark_runs table
-- Feature: aas-kxw7
-- Description: Store benchmark run history and results

BEGIN;

CREATE TABLE IF NOT EXISTS benchmark_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id UUID NOT NULL REFERENCES benchmark_targets(id) ON DELETE CASCADE,
    scenario_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_seconds INT,
    results JSONB,
    error_message TEXT,
    triggered_by VARCHAR(50) DEFAULT 'schedule' CHECK (triggered_by IN ('schedule', 'manual', 'api', 'ci')),
    triggered_by_user VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_benchmark_runs_target_id ON benchmark_runs(target_id);
CREATE INDEX IF NOT EXISTS idx_benchmark_runs_status ON benchmark_runs(status);
CREATE INDEX IF NOT EXISTS idx_benchmark_runs_started_at ON benchmark_runs(started_at);
CREATE INDEX IF NOT EXISTS idx_benchmark_runs_scenario_name ON benchmark_runs(scenario_name);
CREATE INDEX IF NOT EXISTS idx_benchmark_runs_created_at ON benchmark_runs(created_at);

COMMENT ON TABLE benchmark_runs IS 'Benchmark run history and results';
COMMENT ON COLUMN benchmark_runs.target_id IS 'Reference to benchmark_targets.id';
COMMENT ON COLUMN benchmark_runs.scenario_name IS 'Scenario name at time of run (denormalized for history)';
COMMENT ON COLUMN benchmark_runs.status IS 'Run status: pending, running, completed, failed, cancelled';
COMMENT ON COLUMN benchmark_runs.started_at IS 'Timestamp when benchmark started';
COMMENT ON COLUMN benchmark_runs.completed_at IS 'Timestamp when benchmark completed';
COMMENT ON COLUMN benchmark_runs.duration_seconds IS 'Total run duration in seconds';
COMMENT ON COLUMN benchmark_runs.results IS 'Benchmark results as JSONB (metrics, percentiles, etc.)';
COMMENT ON COLUMN benchmark_runs.error_message IS 'Error message if run failed';
COMMENT ON COLUMN benchmark_runs.triggered_by IS 'Trigger source: schedule, manual, api, ci';
COMMENT ON COLUMN benchmark_runs.triggered_by_user IS 'User who triggered manual/api runs';

COMMIT;

-- +goose Down
-- Rollback: Drop benchmark_runs table and indexes

BEGIN;

DROP INDEX IF EXISTS idx_benchmark_runs_created_at;
DROP INDEX IF EXISTS idx_benchmark_runs_scenario_name;
DROP INDEX IF EXISTS idx_benchmark_runs_started_at;
DROP INDEX IF EXISTS idx_benchmark_runs_status;
DROP INDEX IF EXISTS idx_benchmark_runs_target_id;
DROP TABLE IF EXISTS benchmark_runs;

COMMIT;
