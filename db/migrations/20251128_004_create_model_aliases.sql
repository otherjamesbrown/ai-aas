-- Migration: Create model_aliases table
-- Feature: 020-model-management
-- Description: Alternative names for models enabling version abstraction

-- +goose Up
CREATE TABLE IF NOT EXISTS model_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias_name VARCHAR(255) UNIQUE NOT NULL,        -- "llama-latest", "default-model"
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE RESTRICT,
    description VARCHAR(512),                       -- Optional description of alias purpose
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_model_aliases_model_id ON model_aliases(model_id);
CREATE INDEX idx_model_aliases_alias_name ON model_aliases(alias_name);

-- Trigger to update updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_model_aliases_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_model_aliases_updated_at
    BEFORE UPDATE ON model_aliases
    FOR EACH ROW
    EXECUTE FUNCTION update_model_aliases_updated_at();

-- Prevent creating alias with same name as existing model
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_alias_not_model_name()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM model_registry WHERE name = NEW.alias_name) THEN
        RAISE EXCEPTION 'Cannot create alias with same name as existing model: %', NEW.alias_name;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_check_alias_not_model_name
    BEFORE INSERT OR UPDATE ON model_aliases
    FOR EACH ROW
    EXECUTE FUNCTION check_alias_not_model_name();

COMMENT ON TABLE model_aliases IS 'Alternative names for models enabling version abstraction';
COMMENT ON COLUMN model_aliases.alias_name IS 'Alias name that resolves to a model (e.g., llama-latest)';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_check_alias_not_model_name ON model_aliases;
DROP FUNCTION IF EXISTS check_alias_not_model_name();
DROP TRIGGER IF EXISTS trigger_model_aliases_updated_at ON model_aliases;
DROP FUNCTION IF EXISTS update_model_aliases_updated_at();
DROP INDEX IF EXISTS idx_model_aliases_alias_name;
DROP INDEX IF EXISTS idx_model_aliases_model_id;
DROP TABLE IF EXISTS model_aliases;

