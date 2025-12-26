-- +goose Up
-- Migration: Add triton-grpc to valid backend types
-- Feature: TRT-LLM gRPC routing support
-- Description: Enable gRPC-based Triton backend for TRT-LLM decoupled models

-- Drop and recreate the constraint with the new value
ALTER TABLE routing_policies DROP CONSTRAINT IF EXISTS check_backend_type;
ALTER TABLE routing_policies
    ADD CONSTRAINT check_backend_type CHECK (backend_type IN ('openai', 'triton', 'triton-grpc'));

COMMENT ON COLUMN routing_policies.backend_type IS 'Backend protocol: openai (default) for vLLM, triton for Triton HTTP, triton-grpc for Triton gRPC (TRT-LLM)';
