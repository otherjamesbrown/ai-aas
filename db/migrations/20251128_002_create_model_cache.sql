-- Migration: Create model_cache table
-- Feature: 020-model-management
-- Description: Tracks cached model versions in object storage

-- +goose Up
CREATE TABLE IF NOT EXISTS model_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    version VARCHAR(255) NOT NULL,                  -- HF commit hash
    hf_revision VARCHAR(255),                       -- Branch/tag that was pulled
    object_storage_path VARCHAR(512) NOT NULL,      -- "s3://models/llama-3-8b/v1.0/"
    size_bytes BIGINT,                              -- Total size of cached files
    file_count INT,                                 -- Number of files
    checksum_sha256 VARCHAR(64),                    -- Manifest checksum
    status VARCHAR(50) DEFAULT 'downloading',       -- downloading, ready, failed, deleted
    cached_at TIMESTAMP DEFAULT NOW(),
    verified_at TIMESTAMP,
    UNIQUE(model_id, version)
);

-- Indexes for common queries
CREATE INDEX idx_model_cache_model_id ON model_cache(model_id);
CREATE INDEX idx_model_cache_status ON model_cache(status);
CREATE INDEX idx_model_cache_version ON model_cache(version);

-- Constraint for valid status values (idempotent)
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'chk_model_cache_status'
        AND table_name = 'model_cache'
    ) THEN
        ALTER TABLE model_cache ADD CONSTRAINT chk_model_cache_status
            CHECK (status IN ('downloading', 'ready', 'failed', 'deleted'));
    END IF;
END $$;
-- +goose StatementEnd

COMMENT ON TABLE model_cache IS 'Tracks cached model versions in object storage';
COMMENT ON COLUMN model_cache.version IS 'HuggingFace commit hash for this cached version';
COMMENT ON COLUMN model_cache.object_storage_path IS 'S3-compatible path where model files are stored';
COMMENT ON COLUMN model_cache.status IS 'Cache status: downloading, ready, failed, or deleted';

-- +goose Down
ALTER TABLE model_cache DROP CONSTRAINT IF EXISTS chk_model_cache_status;
DROP INDEX IF EXISTS idx_model_cache_version;
DROP INDEX IF EXISTS idx_model_cache_status;
DROP INDEX IF EXISTS idx_model_cache_model_id;
DROP TABLE IF EXISTS model_cache;

