-- Migration: 000009_rename_audit_logs
-- Description: Rename audit_logs table to user_org_audit_logs to avoid naming collision with admin-api-service
-- Resolves: aas-334l0, aas-nkxdy

-- +goose Up
-- +goose StatementBegin
-- Rename table
ALTER TABLE audit_logs RENAME TO user_org_audit_logs;

-- Rename indexes to match new table name
ALTER INDEX audit_logs_org_created_idx RENAME TO user_org_audit_logs_org_created_idx;
ALTER INDEX audit_logs_org_action_idx RENAME TO user_org_audit_logs_org_action_idx;
ALTER INDEX audit_logs_org_actor_idx RENAME TO user_org_audit_logs_org_actor_idx;
ALTER INDEX audit_logs_org_target_type_idx RENAME TO user_org_audit_logs_org_target_type_idx;
ALTER INDEX audit_logs_org_filters_idx RENAME TO user_org_audit_logs_org_filters_idx;

-- Rename constraint to match new table name
ALTER TABLE user_org_audit_logs RENAME CONSTRAINT audit_logs_actor_type_check TO user_org_audit_logs_actor_type_check;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert constraint rename
ALTER TABLE user_org_audit_logs RENAME CONSTRAINT user_org_audit_logs_actor_type_check TO audit_logs_actor_type_check;

-- Revert index renames
ALTER INDEX user_org_audit_logs_org_filters_idx RENAME TO audit_logs_org_filters_idx;
ALTER INDEX user_org_audit_logs_org_target_type_idx RENAME TO audit_logs_org_target_type_idx;
ALTER INDEX user_org_audit_logs_org_actor_idx RENAME TO audit_logs_org_actor_idx;
ALTER INDEX user_org_audit_logs_org_action_idx RENAME TO audit_logs_org_action_idx;
ALTER INDEX user_org_audit_logs_org_created_idx RENAME TO audit_logs_org_created_idx;

-- Revert table rename
ALTER TABLE user_org_audit_logs RENAME TO audit_logs;
-- +goose StatementEnd
