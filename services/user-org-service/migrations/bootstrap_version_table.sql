-- Bootstrap script for goose_db_version_user_org table
-- This is a ONE-TIME script to migrate from shared goose_db_version to dedicated table
-- Run this manually BEFORE deploying the updated Helm chart

-- Create the new version table for user-org-service
CREATE TABLE IF NOT EXISTS goose_db_version_user_org (
    id serial NOT NULL,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp without time zone DEFAULT now(),
    CONSTRAINT goose_db_version_user_org_pkey PRIMARY KEY (id)
);

-- Mark migrations 000001-000006 as already applied
-- These migrations created the orgs, users, api_keys, etc. tables that already exist
INSERT INTO goose_db_version_user_org (version_id, is_applied, tstamp)
VALUES
    (1, true, now()),
    (2, true, now()),
    (3, true, now()),
    (4, true, now()),
    (5, true, now()),
    (6, true, now())
ON CONFLICT DO NOTHING;

-- Migration 000007 (audit_logs) will be applied by goose on next pod startup
