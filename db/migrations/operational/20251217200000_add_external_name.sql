-- +goose Up
-- Migration: Add external_name to model_registry table
-- Purpose: Support configurable external names for OpenAI-compatible API
--
-- The external_name is exposed via /v1/models and used in /v1/chat/completions requests.
-- Internal systems always use the full modelID path (e.g., "unsloth/gpt-oss-20b").
-- The external_name allows operators to configure what name users see and use.
-- If not explicitly set, it's derived from modelID by taking the part after the last "/".

BEGIN;

-- Add external_name column to model_registry
ALTER TABLE model_registry ADD COLUMN IF NOT EXISTS external_name VARCHAR(255);

-- Create unique index for external_name (allows NULL but unique when set)
CREATE UNIQUE INDEX IF NOT EXISTS idx_model_registry_external_name
ON model_registry(external_name)
WHERE external_name IS NOT NULL;

-- Auto-migrate existing models: derive external_name from hf_model_id
-- If hf_model_id contains "/", take the part after the last "/"
-- Otherwise, use the existing name field
-- +goose StatementBegin
DO $$
BEGIN
    UPDATE model_registry
    SET external_name = CASE
        WHEN hf_model_id LIKE '%/%' THEN SUBSTRING(hf_model_id FROM POSITION('/' IN hf_model_id) + 1)
        ELSE name
    END
    WHERE external_name IS NULL;
END $$;
-- +goose StatementEnd

-- Add comment for documentation
COMMENT ON COLUMN model_registry.external_name IS
'The name exposed in OpenAI-compatible APIs (/v1/models). If not set explicitly, derived from hf_model_id. Must be unique per environment.';

COMMIT;

-- +goose Down
-- Rollback external_name from model_registry table

BEGIN;

DROP INDEX IF EXISTS idx_model_registry_external_name;
ALTER TABLE model_registry DROP COLUMN IF EXISTS external_name;

COMMIT;
