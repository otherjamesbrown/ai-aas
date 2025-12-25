-- +goose Up
-- Migration: Create audit_logs table for admin-api-service
-- Feature: 017-admin-api-service

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    actor VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    outcome VARCHAR(20) NOT NULL,
    changes JSONB,
    error_detail TEXT,
    client_ip INET,
    user_agent VARCHAR(512),
    request_id UUID
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);

-- +goose Down
-- Rollback: Drop audit_logs table
DROP TABLE IF EXISTS audit_logs;
