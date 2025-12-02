-- Migration: Create pull_jobs table
-- Feature: 020-model-management
-- Description: Tracks model pull/download operations

-- +goose Up
CREATE TABLE IF NOT EXISTS pull_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    revision VARCHAR(255) NOT NULL DEFAULT 'main',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    progress DECIMAL(5,2) DEFAULT 0,
    bytes_total BIGINT,
    bytes_completed BIGINT DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP,
    created_by VARCHAR(255),
    -- Partial unique index: Ensures only one active pull job can exist per model/revision.
    -- This prevents race conditions and redundant downloads by constraining uniqueness
    -- only for in-progress statuses (pending, downloading, uploading).
    -- Completed/failed/cancelled jobs are not constrained, allowing historical records.
    UNIQUE(model_id, revision, status) WHERE status IN ('pending', 'downloading', 'uploading')
);

-- Indexes for common queries
CREATE INDEX idx_pull_jobs_model_id ON pull_jobs(model_id);
CREATE INDEX idx_pull_jobs_status ON pull_jobs(status);
CREATE INDEX idx_pull_jobs_started_at ON pull_jobs(started_at DESC);

-- Constraint for valid status values
ALTER TABLE pull_jobs ADD CONSTRAINT chk_pull_jobs_status
    CHECK (status IN ('pending', 'downloading', 'uploading', 'complete', 'failed', 'cancelled'));

COMMENT ON TABLE pull_jobs IS 'Tracks model pull/download operations from HuggingFace to object storage';
COMMENT ON COLUMN pull_jobs.revision IS 'HuggingFace branch/tag/commit being pulled';
COMMENT ON COLUMN pull_jobs.progress IS 'Download progress percentage (0-100)';
COMMENT ON COLUMN pull_jobs.status IS 'Job status: pending, downloading, uploading, complete, failed, cancelled';

-- +goose Down
ALTER TABLE pull_jobs DROP CONSTRAINT IF EXISTS chk_pull_jobs_status;
DROP INDEX IF EXISTS idx_pull_jobs_started_at;
DROP INDEX IF EXISTS idx_pull_jobs_status;
DROP INDEX IF EXISTS idx_pull_jobs_model_id;
DROP TABLE IF EXISTS pull_jobs;
