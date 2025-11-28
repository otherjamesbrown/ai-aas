-- Migration: Create model_registry table
-- Feature: 020-model-management
-- Description: Central registry of all known models with HuggingFace metadata

-- +goose Up
CREATE TABLE IF NOT EXISTS model_registry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,              -- Internal name: "llama-3-8b"
    hf_model_id VARCHAR(512) NOT NULL,              -- "meta-llama/Llama-3-8B-Instruct"
    hf_revision VARCHAR(255) DEFAULT 'main',        -- HF branch/tag/commit
    requires_auth BOOLEAN DEFAULT false,            -- Needs HF token
    is_gated BOOLEAN DEFAULT false,                 -- Requires HF license acceptance
    license_type VARCHAR(100),                      -- "llama3", "apache-2.0", etc.
    license_url VARCHAR(512),                       -- URL to accept license on HF
    license_accepted_at TIMESTAMP,                  -- When license was acknowledged
    license_accepted_by VARCHAR(255),               -- Who acknowledged (for audit)
    recommended_gpu_memory_gb INT,                  -- Minimum GPU memory
    recommended_cpu_memory_gb INT,                  -- Minimum system memory
    pinned_version VARCHAR(255),                    -- If set, skip update checks
    metadata JSONB DEFAULT '{}',                    -- Additional metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_model_registry_name ON model_registry(name);
CREATE INDEX idx_model_registry_hf_model_id ON model_registry(hf_model_id);

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_model_registry_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_model_registry_updated_at
    BEFORE UPDATE ON model_registry
    FOR EACH ROW
    EXECUTE FUNCTION update_model_registry_updated_at();

COMMENT ON TABLE model_registry IS 'Central registry of all known models with HuggingFace metadata';
COMMENT ON COLUMN model_registry.name IS 'Internal model name used in CLI commands';
COMMENT ON COLUMN model_registry.hf_model_id IS 'HuggingFace repository ID (org/repo format)';
COMMENT ON COLUMN model_registry.is_gated IS 'True if model requires license acceptance on HuggingFace';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_model_registry_updated_at ON model_registry;
DROP FUNCTION IF EXISTS update_model_registry_updated_at();
DROP INDEX IF EXISTS idx_model_registry_hf_model_id;
DROP INDEX IF EXISTS idx_model_registry_name;
DROP TABLE IF EXISTS model_registry;

