-- Export jobs table for finance-friendly CSV exports
BEGIN;

-- Create ENUM types for export job status and granularity
DO $$ BEGIN
    CREATE TYPE analytics.export_job_status AS ENUM ('pending', 'running', 'succeeded', 'failed', 'expired');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE analytics.export_job_granularity AS ENUM ('hourly', 'daily', 'monthly');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- export_jobs: Track export job lifecycle for CSV generation and S3 delivery
CREATE TABLE IF NOT EXISTS analytics.export_jobs (
    job_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    requested_by UUID NOT NULL,
    time_range_start TIMESTAMPTZ NOT NULL,
    time_range_end TIMESTAMPTZ NOT NULL,
    granularity analytics.export_job_granularity NOT NULL DEFAULT 'daily',
    status analytics.export_job_status NOT NULL DEFAULT 'pending',
    output_uri TEXT,
    checksum TEXT,
    row_count BIGINT,
    initiated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    CONSTRAINT valid_time_range CHECK (time_range_end > time_range_start)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_export_jobs_org_initiated 
    ON analytics.export_jobs (org_id, initiated_at DESC);
CREATE INDEX IF NOT EXISTS idx_export_jobs_status_pending_running 
    ON analytics.export_jobs (status) 
    WHERE status IN ('pending', 'running');

COMMIT;

