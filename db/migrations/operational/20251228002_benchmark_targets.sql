-- +goose Up
-- Migration: Create benchmark_targets table
-- Feature: aas-kxw7
-- Description: Store benchmark target configurations (model + scenario + environment combinations)

BEGIN;

CREATE TABLE IF NOT EXISTS benchmark_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    model_name VARCHAR(255) NOT NULL,
    scenario_name VARCHAR(255) NOT NULL,
    environment VARCHAR(50) NOT NULL CHECK (environment IN ('development', 'staging', 'production')),
    endpoint_url VARCHAR(512),
    org_id UUID,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'paused', 'disabled')),
    interval_seconds INT,
    schedule_enabled BOOLEAN DEFAULT true,
    config_overrides JSONB DEFAULT '{}'::JSONB,
    last_run_at TIMESTAMPTZ,
    last_run_status VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT benchmark_targets_unique_name_env UNIQUE (name, environment)
);

CREATE INDEX IF NOT EXISTS idx_benchmark_targets_environment ON benchmark_targets(environment);
CREATE INDEX IF NOT EXISTS idx_benchmark_targets_status ON benchmark_targets(status);
CREATE INDEX IF NOT EXISTS idx_benchmark_targets_org_id ON benchmark_targets(org_id);
CREATE INDEX IF NOT EXISTS idx_benchmark_targets_model_name ON benchmark_targets(model_name);
CREATE INDEX IF NOT EXISTS idx_benchmark_targets_scenario_name ON benchmark_targets(scenario_name);

COMMENT ON TABLE benchmark_targets IS 'Benchmark targets: model + scenario + environment combinations to benchmark';
COMMENT ON COLUMN benchmark_targets.name IS 'Unique target identifier within environment';
COMMENT ON COLUMN benchmark_targets.display_name IS 'Human-readable display name';
COMMENT ON COLUMN benchmark_targets.model_name IS 'Model name to benchmark';
COMMENT ON COLUMN benchmark_targets.scenario_name IS 'Scenario name (references benchmark_scenarios.name)';
COMMENT ON COLUMN benchmark_targets.environment IS 'Target environment: development, staging, production';
COMMENT ON COLUMN benchmark_targets.endpoint_url IS 'Override endpoint URL (defaults to environment API router)';
COMMENT ON COLUMN benchmark_targets.org_id IS 'Organization ID for multi-tenant benchmarks';
COMMENT ON COLUMN benchmark_targets.status IS 'Target status: active, paused, disabled';
COMMENT ON COLUMN benchmark_targets.interval_seconds IS 'Benchmark schedule interval in seconds';
COMMENT ON COLUMN benchmark_targets.schedule_enabled IS 'Whether scheduled benchmarks are enabled';
COMMENT ON COLUMN benchmark_targets.config_overrides IS 'Scenario config overrides as JSONB';
COMMENT ON COLUMN benchmark_targets.last_run_at IS 'Timestamp of last benchmark run';
COMMENT ON COLUMN benchmark_targets.last_run_status IS 'Status of last benchmark run';

COMMIT;

-- +goose Down
-- Rollback: Drop benchmark_targets table and indexes

BEGIN;

DROP INDEX IF EXISTS idx_benchmark_targets_scenario_name;
DROP INDEX IF EXISTS idx_benchmark_targets_model_name;
DROP INDEX IF EXISTS idx_benchmark_targets_org_id;
DROP INDEX IF EXISTS idx_benchmark_targets_status;
DROP INDEX IF EXISTS idx_benchmark_targets_environment;
DROP TABLE IF EXISTS benchmark_targets;

COMMIT;
