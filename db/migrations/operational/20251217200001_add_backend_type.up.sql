-- +goose Up
-- Migration: Add backend_type and tokenizer columns to routing_policies
-- Feature: 032-triton-api-support
-- Description: Enable Triton backend support via translation layer

ALTER TABLE routing_policies
    ADD COLUMN IF NOT EXISTS backend_type VARCHAR(32) DEFAULT 'openai',
    ADD COLUMN IF NOT EXISTS tokenizer VARCHAR(64);

-- Add check constraint for valid backend types
ALTER TABLE routing_policies
    ADD CONSTRAINT check_backend_type CHECK (backend_type IN ('openai', 'triton'));

COMMENT ON COLUMN routing_policies.backend_type IS 'Backend protocol: openai (default) for vLLM, triton for Triton Inference Server';
COMMENT ON COLUMN routing_policies.tokenizer IS 'Tiktoken encoding name for token counting (e.g., cl100k_base, llama3)';
