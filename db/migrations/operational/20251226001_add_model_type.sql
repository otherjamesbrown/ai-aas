-- +goose Up
-- Migration: Add model_type to model_registry
-- Feature: aas-c6ya
-- Description: Add model_type field to distinguish between text, vision-language, embedding, and audio models

-- +goose Up
ALTER TABLE model_registry
ADD COLUMN IF NOT EXISTS model_type VARCHAR(50) DEFAULT 'text';

COMMENT ON COLUMN model_registry.model_type IS 'Model type: text, vision-language, embedding, audio';

-- +goose Down
-- Rollback: Remove model_type from model_registry

ALTER TABLE model_registry
DROP COLUMN IF EXISTS model_type;
