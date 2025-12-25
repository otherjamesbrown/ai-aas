-- +goose Up
-- Migration: Create policy_sync_log table for admin-api-service
-- Feature: 017-admin-api-service

CREATE TABLE IF NOT EXISTS policy_sync_log (
    id BIGSERIAL PRIMARY KEY,
    router_instance_id VARCHAR(255) NOT NULL,
    policy_id VARCHAR(255) NOT NULL,
    policy_version INTEGER NOT NULL,
    synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sync_duration_ms INTEGER,
    environment VARCHAR(50)
);

CREATE INDEX IF NOT EXISTS idx_sync_log_instance ON policy_sync_log(router_instance_id, synced_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_log_policy ON policy_sync_log(policy_id, synced_at DESC);
