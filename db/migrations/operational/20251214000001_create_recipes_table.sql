-- +goose Up
-- Model Recipes Table for Spec 025
-- Stores pre-configured templates for deploying AI models with specific settings

CREATE TABLE IF NOT EXISTS model_recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    description TEXT,
    model_id VARCHAR(512) NOT NULL,

    -- Resource requirements
    gpu_count INT DEFAULT 1,
    memory_limit VARCHAR(50) DEFAULT '24Gi',
    cpu_limit VARCHAR(50) DEFAULT '4',

    -- Replica configuration
    replicas INT DEFAULT 1,
    min_replicas INT,
    max_replicas INT,

    -- Runtime configuration (stored as JSONB for flexibility)
    runtime_env JSONB DEFAULT '{}',
    extra_args JSONB DEFAULT '{}',

    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_recipes_name ON model_recipes(name);
CREATE INDEX IF NOT EXISTS idx_recipes_model_id ON model_recipes(model_id);
CREATE INDEX IF NOT EXISTS idx_recipes_deleted_at ON model_recipes(deleted_at);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_model_recipes_updated_at ON model_recipes;
CREATE TRIGGER update_model_recipes_updated_at
    BEFORE UPDATE ON model_recipes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
-- Rollback Model Recipes Table

-- Drop trigger first
DROP TRIGGER IF EXISTS update_model_recipes_updated_at ON model_recipes;

-- Drop indexes
DROP INDEX IF EXISTS idx_recipes_deleted_at;
DROP INDEX IF EXISTS idx_recipes_model_id;
DROP INDEX IF EXISTS idx_recipes_name;

-- Drop table
DROP TABLE IF EXISTS model_recipes;
