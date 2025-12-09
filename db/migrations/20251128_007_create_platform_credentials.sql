-- Migration: Create platform_credentials table
-- Feature: 020-model-management
-- Description: Encrypted storage for platform credentials (HF tokens, S3 keys)

-- +goose Up
CREATE TABLE IF NOT EXISTS platform_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_type VARCHAR(100) UNIQUE NOT NULL,   -- hf-token, s3-access, s3-secret, s3-endpoint, s3-bucket
    encrypted_value TEXT NOT NULL,                  -- AES-256 encrypted value
    metadata JSONB DEFAULT '{}',                    -- Additional metadata (e.g., endpoint URL)
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Index for lookups
CREATE INDEX idx_platform_credentials_type ON platform_credentials(credential_type);

-- Constraint for known credential types (idempotent)
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'chk_platform_credentials_type'
        AND table_name = 'platform_credentials'
    ) THEN
        ALTER TABLE platform_credentials ADD CONSTRAINT chk_platform_credentials_type
            CHECK (credential_type IN ('hf-token', 's3-access', 's3-secret', 's3-endpoint', 's3-bucket'));
    END IF;
END $$;
-- +goose StatementEnd

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_platform_credentials_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_platform_credentials_updated_at
    BEFORE UPDATE ON platform_credentials
    FOR EACH ROW
    EXECUTE FUNCTION update_platform_credentials_updated_at();

COMMENT ON TABLE platform_credentials IS 'Encrypted storage for platform credentials';
COMMENT ON COLUMN platform_credentials.encrypted_value IS 'AES-256 encrypted credential value - NEVER log or expose';
COMMENT ON COLUMN platform_credentials.credential_type IS 'Credential type: hf-token, s3-access, s3-secret, s3-endpoint, s3-bucket';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_platform_credentials_updated_at ON platform_credentials;
DROP FUNCTION IF EXISTS update_platform_credentials_updated_at();
ALTER TABLE platform_credentials DROP CONSTRAINT IF EXISTS chk_platform_credentials_type;
DROP INDEX IF EXISTS idx_platform_credentials_type;
DROP TABLE IF EXISTS platform_credentials;

