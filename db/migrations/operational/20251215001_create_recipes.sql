-- Migration: Create recipes table
-- Feature: 025-model-recipes
-- Description: Recipes table for storing pre-configured deployment templates
-- Task: T013 (ai-aas-6pz1)

-- +goose Up
CREATE TABLE IF NOT EXISTS recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,              -- Unique recipe name: "llama-7b-vllm"
    display_name VARCHAR(255),                      -- Human-readable name: "Llama 7B with vLLM"
    description TEXT,                               -- Description of the recipe
    model_id VARCHAR(255) NOT NULL,                 -- HuggingFace model ID: "meta-llama/Llama-2-7b-hf"
    runtime VARCHAR(50) NOT NULL DEFAULT 'vllm',    -- Runtime: vllm, triton, tgi
    spec JSONB NOT NULL,                            -- Full recipe specification as JSON
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_recipes_runtime ON recipes(runtime);
CREATE INDEX IF NOT EXISTS idx_recipes_model_id ON recipes(model_id);

-- Trigger to update updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_recipes_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_recipes_updated_at
    BEFORE UPDATE ON recipes
    FOR EACH ROW
    EXECUTE FUNCTION update_recipes_updated_at();

COMMENT ON TABLE recipes IS 'Pre-configured deployment templates for model recipes';
COMMENT ON COLUMN recipes.name IS 'Unique recipe name used in API and CLI';
COMMENT ON COLUMN recipes.model_id IS 'HuggingFace model ID (org/repo format)';
COMMENT ON COLUMN recipes.runtime IS 'Inference runtime: vllm, triton, tgi';
COMMENT ON COLUMN recipes.spec IS 'Full recipe specification including resources, runtime args, and scheduling';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_recipes_updated_at ON recipes;
DROP FUNCTION IF EXISTS update_recipes_updated_at();
DROP INDEX IF EXISTS idx_recipes_model_id;
DROP INDEX IF EXISTS idx_recipes_runtime;
DROP TABLE IF EXISTS recipes;
