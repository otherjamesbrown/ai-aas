-- +goose Up
-- Model Management Tables for Spec 020
-- Creates tables for model registry, cache, deployments, and credentials

-- 1. Model Registry - Central registry of all known models
CREATE TABLE IF NOT EXISTS model_registry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    hf_model_id VARCHAR(512) NOT NULL,
    hf_revision VARCHAR(255) DEFAULT 'main',
    requires_auth BOOLEAN DEFAULT false,
    is_gated BOOLEAN DEFAULT false,
    license_type VARCHAR(100),
    license_url VARCHAR(512),
    license_accepted_at TIMESTAMP WITH TIME ZONE,
    license_accepted_by VARCHAR(255),
    recommended_gpu_memory_gb INT,
    recommended_cpu_memory_gb INT,
    pinned_version VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Model Aliases - Alternative names for models
CREATE TABLE IF NOT EXISTS model_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias_name VARCHAR(255) NOT NULL UNIQUE,
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE RESTRICT,
    description VARCHAR(512),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. Model Cache - Tracks cached model versions in object storage
CREATE TABLE IF NOT EXISTS model_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    version VARCHAR(255) NOT NULL,
    hf_revision VARCHAR(255),
    object_storage_path VARCHAR(512) NOT NULL,
    size_bytes BIGINT,
    file_count INT,
    checksum_sha256 VARCHAR(64),
    status VARCHAR(50) DEFAULT 'downloading',
    cached_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    verified_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(model_id, version)
);

-- 4. Model Deployments - Tracks deployed model instances per environment
CREATE TABLE IF NOT EXISTS model_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    cache_id UUID REFERENCES model_cache(id),
    environment VARCHAR(50) NOT NULL,
    namespace VARCHAR(100) NOT NULL,
    inferenceservice_name VARCHAR(255),
    endpoint VARCHAR(512),
    enabled BOOLEAN DEFAULT true,
    status VARCHAR(50) DEFAULT 'pending',
    replicas_desired INT DEFAULT 1,
    replicas_ready INT DEFAULT 0,
    gpu_count INT DEFAULT 1,
    memory_gb INT,
    last_health_check_at TIMESTAMP WITH TIME ZONE,
    last_health_status VARCHAR(50),
    last_enabled_at TIMESTAMP WITH TIME ZONE,
    last_enabled_by VARCHAR(255),
    last_disabled_at TIMESTAMP WITH TIME ZONE,
    last_disabled_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(model_id, environment)
);

-- 5. Model State History - Audit trail for enable/disable operations
CREATE TABLE IF NOT EXISTS model_state_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES model_deployments(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL,
    performed_by VARCHAR(255),
    reason TEXT,
    scheduled_at TIMESTAMP WITH TIME ZONE,
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 6. Model Validations - Records validation check results
CREATE TABLE IF NOT EXISTS model_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    environment VARCHAR(50),
    validation_type VARCHAR(100),
    check_name VARCHAR(100),
    status VARCHAR(20),
    message TEXT,
    remediation TEXT,
    validated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 7. Platform Credentials - Encrypted storage for platform credentials
CREATE TABLE IF NOT EXISTS platform_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_type VARCHAR(100) NOT NULL UNIQUE,
    encrypted_value TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_model_cache_model_id ON model_cache(model_id);
CREATE INDEX IF NOT EXISTS idx_model_cache_status ON model_cache(status);
CREATE INDEX IF NOT EXISTS idx_model_deployments_model_id ON model_deployments(model_id);
CREATE INDEX IF NOT EXISTS idx_model_deployments_environment ON model_deployments(environment);
CREATE INDEX IF NOT EXISTS idx_model_deployments_status ON model_deployments(status);
CREATE INDEX IF NOT EXISTS idx_model_state_history_deployment_id ON model_state_history(deployment_id);
CREATE INDEX IF NOT EXISTS idx_model_validations_model_id ON model_validations(model_id);
CREATE INDEX IF NOT EXISTS idx_model_aliases_model_id ON model_aliases(model_id);

-- Triggers for updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';
-- +goose StatementEnd

CREATE TRIGGER update_model_registry_updated_at
    BEFORE UPDATE ON model_registry
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_model_aliases_updated_at
    BEFORE UPDATE ON model_aliases
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_model_deployments_updated_at
    BEFORE UPDATE ON model_deployments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_platform_credentials_updated_at
    BEFORE UPDATE ON platform_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
