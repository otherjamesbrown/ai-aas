-- +goose Up
-- Migration: Create benchmark_scenarios table
-- Feature: aas-kxw7
-- Description: Store benchmark scenario configurations synced from config repo

BEGIN;

CREATE TABLE IF NOT EXISTS benchmark_scenarios (
    name VARCHAR(255) PRIMARY KEY,
    description TEXT,
    version VARCHAR(50),
    config JSONB NOT NULL DEFAULT '{}'::JSONB,
    synced_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE benchmark_scenarios IS 'Benchmark scenario configurations synced from ai-aas-config repo';
COMMENT ON COLUMN benchmark_scenarios.name IS 'Unique scenario identifier (e.g., latency-p99, throughput-max)';
COMMENT ON COLUMN benchmark_scenarios.description IS 'Human-readable description of the benchmark scenario';
COMMENT ON COLUMN benchmark_scenarios.version IS 'Scenario version from config repo';
COMMENT ON COLUMN benchmark_scenarios.config IS 'Full scenario configuration as JSONB (rate, duration, prompts, etc.)';
COMMENT ON COLUMN benchmark_scenarios.synced_at IS 'Last sync timestamp from config repo';

COMMIT;

-- +goose Down
-- Rollback: Drop benchmark_scenarios table

BEGIN;

DROP TABLE IF EXISTS benchmark_scenarios;

COMMIT;
