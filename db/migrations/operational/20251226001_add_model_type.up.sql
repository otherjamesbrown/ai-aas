-- Migration: Add model_type to model_registry
-- Feature: aas-c6ya
-- Description: Add model_type field to distinguish between text, vision-language, embedding, and audio models

-- +goose Up
ALTER TABLE model_registry
ADD COLUMN IF NOT EXISTS model_type VARCHAR(50) DEFAULT 'text';

COMMENT ON COLUMN model_registry.model_type IS 'Model type: text, vision-language, embedding, audio';
