-- Migration: Create inference engine tables
-- Feature: AIAAS-042 Multi-inference-engine support
-- Description: Tables for managing inference engines (vLLM, Triton, TGI), versions, and configs

-- +goose Up

-- Table: inference_engines
-- Stores available inference engine types (vLLM, Triton, TGI)
CREATE TABLE IF NOT EXISTS inference_engines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,              -- e.g., "vllm", "triton", "tgi"
    display_name VARCHAR(255) NOT NULL,             -- e.g., "vLLM OpenAI Server"
    description TEXT,
    default_version VARCHAR(50) NOT NULL,           -- e.g., "0.6.4"
    protocol_versions TEXT[] DEFAULT '{v2}',        -- KServe protocols: v1, v2, grpc-v2
    supported_model_formats TEXT[],                 -- e.g., '{vllm,pytorch,safetensors}'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_inference_engines_name ON inference_engines(name);

-- Trigger to update updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_inference_engines_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_inference_engines_updated_at
    BEFORE UPDATE ON inference_engines
    FOR EACH ROW
    EXECUTE FUNCTION update_inference_engines_updated_at();

COMMENT ON TABLE inference_engines IS 'Available inference engine types (vLLM, Triton, TGI)';
COMMENT ON COLUMN inference_engines.name IS 'Unique identifier for the engine, e.g., vllm, triton, tgi';
COMMENT ON COLUMN inference_engines.protocol_versions IS 'KServe protocol versions supported: v1, v2, grpc-v2';


-- Table: inference_engine_versions
-- Stores version history for each engine with container images
CREATE TABLE IF NOT EXISTS inference_engine_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engine_id UUID NOT NULL REFERENCES inference_engines(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,                   -- e.g., "0.6.4", "0.10.2", "0.12.0"
    container_image VARCHAR(500) NOT NULL,          -- e.g., "vllm/vllm-openai:v0.6.4"
    is_default BOOLEAN DEFAULT FALSE,
    is_deprecated BOOLEAN DEFAULT FALSE,
    min_gpu_memory_gb INTEGER,                      -- Minimum GPU memory required
    release_notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(engine_id, version)
);

CREATE INDEX idx_inference_engine_versions_engine_id ON inference_engine_versions(engine_id);
CREATE INDEX idx_inference_engine_versions_is_default ON inference_engine_versions(is_default) WHERE is_default = true;

COMMENT ON TABLE inference_engine_versions IS 'Version history for inference engines with container images';
COMMENT ON COLUMN inference_engine_versions.is_default IS 'Only one version per engine should be default';
COMMENT ON COLUMN inference_engine_versions.is_deprecated IS 'Deprecated versions can still be used but are not recommended';


-- Table: inference_engine_configs
-- Named resource profiles combining engine, version, and resource settings
CREATE TABLE IF NOT EXISTS inference_engine_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,              -- e.g., "vllm/default", "vllm/high-memory"
    engine_id UUID NOT NULL REFERENCES inference_engines(id),
    version_id UUID REFERENCES inference_engine_versions(id),  -- NULL = use engine default
    description TEXT,

    -- Resource configuration
    gpu_count INTEGER DEFAULT 1,
    gpu_type VARCHAR(50),                           -- e.g., "rtx-4090", "a100-40gb" (optional)
    memory_gb INTEGER DEFAULT 16,
    cpu_cores INTEGER DEFAULT 4,

    -- Engine-specific configuration (JSONB for flexibility)
    engine_args JSONB DEFAULT '{}',                 -- e.g., {"max-model-len": 32768, "tensor-parallel-size": 2}

    -- Scaling configuration
    min_replicas INTEGER DEFAULT 1,
    max_replicas INTEGER DEFAULT 1,

    -- Health check configuration
    startup_timeout_seconds INTEGER DEFAULT 900,    -- 15 min for large models

    is_default BOOLEAN DEFAULT FALSE,               -- One default per engine
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_inference_engine_configs_engine_id ON inference_engine_configs(engine_id);
CREATE INDEX idx_inference_engine_configs_name ON inference_engine_configs(name);
CREATE INDEX idx_inference_engine_configs_is_default ON inference_engine_configs(is_default) WHERE is_default = true;

-- Trigger to update updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_inference_engine_configs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_inference_engine_configs_updated_at
    BEFORE UPDATE ON inference_engine_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_inference_engine_configs_updated_at();

COMMENT ON TABLE inference_engine_configs IS 'Named resource profiles combining engine, version, and resource settings';
COMMENT ON COLUMN inference_engine_configs.name IS 'Format: engine/profile, e.g., vllm/default, vllm/high-memory';
COMMENT ON COLUMN inference_engine_configs.engine_args IS 'Engine-specific args like max-model-len, tensor-parallel-size';
COMMENT ON COLUMN inference_engine_configs.is_default IS 'One config per engine should be marked as default';


-- Add engine_config_id to model_deployments
ALTER TABLE model_deployments
    ADD COLUMN engine_config_id UUID REFERENCES inference_engine_configs(id);

CREATE INDEX idx_model_deployments_engine_config_id ON model_deployments(engine_config_id);

COMMENT ON COLUMN model_deployments.engine_config_id IS 'Reference to the inference engine config used for this deployment';


-- +goose StatementBegin
-- Seed default inference engines
DO $$
DECLARE
    vllm_engine_id UUID;
    triton_engine_id UUID;
    tgi_engine_id UUID;
    vllm_version_id UUID;
    triton_version_id UUID;
    tgi_version_id UUID;
BEGIN
    -- Insert vLLM engine
    INSERT INTO inference_engines (name, display_name, description, default_version, protocol_versions, supported_model_formats)
    VALUES (
        'vllm',
        'vLLM OpenAI Server',
        'High-throughput LLM serving with PagedAttention. Supports OpenAI-compatible API.',
        '0.6.4',
        ARRAY['v2'],
        ARRAY['vllm', 'pytorch', 'safetensors']
    )
    ON CONFLICT (name) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        description = EXCLUDED.description,
        default_version = EXCLUDED.default_version
    RETURNING id INTO vllm_engine_id;

    -- Insert Triton engine
    INSERT INTO inference_engines (name, display_name, description, default_version, protocol_versions, supported_model_formats)
    VALUES (
        'triton',
        'NVIDIA Triton Inference Server',
        'NVIDIA Triton with TensorRT-LLM backend for optimized inference.',
        '24.04',
        ARRAY['v2', 'grpc-v2'],
        ARRAY['tensorrt-llm', 'pytorch', 'onnx']
    )
    ON CONFLICT (name) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        description = EXCLUDED.description,
        default_version = EXCLUDED.default_version
    RETURNING id INTO triton_engine_id;

    -- Insert TGI engine
    INSERT INTO inference_engines (name, display_name, description, default_version, protocol_versions, supported_model_formats)
    VALUES (
        'tgi',
        'Text Generation Inference',
        'HuggingFace Text Generation Inference server with continuous batching.',
        '2.4.1',
        ARRAY['v2'],
        ARRAY['pytorch', 'safetensors']
    )
    ON CONFLICT (name) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        description = EXCLUDED.description,
        default_version = EXCLUDED.default_version
    RETURNING id INTO tgi_engine_id;

    -- Insert vLLM versions
    INSERT INTO inference_engine_versions (engine_id, version, container_image, is_default, min_gpu_memory_gb, release_notes)
    VALUES
        (vllm_engine_id, '0.6.4', 'vllm/vllm-openai:v0.6.4', TRUE, 16, 'Stable release with OpenAI-compatible API'),
        (vllm_engine_id, '0.6.6', 'vllm/vllm-openai:v0.6.6', FALSE, 16, 'Performance improvements and bug fixes')
    ON CONFLICT (engine_id, version) DO UPDATE SET
        container_image = EXCLUDED.container_image,
        is_default = EXCLUDED.is_default;

    -- Get vLLM default version ID
    SELECT id INTO vllm_version_id FROM inference_engine_versions
    WHERE engine_id = vllm_engine_id AND is_default = TRUE;

    -- Insert Triton versions
    INSERT INTO inference_engine_versions (engine_id, version, container_image, is_default, min_gpu_memory_gb, release_notes)
    VALUES
        (triton_engine_id, '24.04', 'nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3', TRUE, 24, 'TensorRT-LLM backend with Python support')
    ON CONFLICT (engine_id, version) DO UPDATE SET
        container_image = EXCLUDED.container_image,
        is_default = EXCLUDED.is_default;

    -- Get Triton default version ID
    SELECT id INTO triton_version_id FROM inference_engine_versions
    WHERE engine_id = triton_engine_id AND is_default = TRUE;

    -- Insert TGI versions
    INSERT INTO inference_engine_versions (engine_id, version, container_image, is_default, min_gpu_memory_gb, release_notes)
    VALUES
        (tgi_engine_id, '2.4.1', 'ghcr.io/huggingface/text-generation-inference:2.4.1', TRUE, 16, 'Stable release with continuous batching')
    ON CONFLICT (engine_id, version) DO UPDATE SET
        container_image = EXCLUDED.container_image,
        is_default = EXCLUDED.is_default;

    -- Get TGI default version ID
    SELECT id INTO tgi_version_id FROM inference_engine_versions
    WHERE engine_id = tgi_engine_id AND is_default = TRUE;

    -- Insert vLLM configs
    INSERT INTO inference_engine_configs (name, engine_id, version_id, description, gpu_count, memory_gb, cpu_cores, engine_args, is_default)
    VALUES
        ('vllm/default', vllm_engine_id, vllm_version_id, 'Standard config for 7B parameter models', 1, 16, 4, '{}', TRUE),
        ('vllm/high-memory', vllm_engine_id, vllm_version_id, 'High memory config for 20B+ models with extended context', 1, 48, 8, '{"max-model-len": 32768}', FALSE),
        ('vllm/multi-gpu', vllm_engine_id, vllm_version_id, 'Multi-GPU config for 70B models with tensor parallelism', 2, 96, 16, '{"tensor-parallel-size": 2, "max-model-len": 32768}', FALSE)
    ON CONFLICT (name) DO UPDATE SET
        description = EXCLUDED.description,
        gpu_count = EXCLUDED.gpu_count,
        memory_gb = EXCLUDED.memory_gb,
        engine_args = EXCLUDED.engine_args;

    -- Insert Triton configs
    INSERT INTO inference_engine_configs (name, engine_id, version_id, description, gpu_count, memory_gb, cpu_cores, engine_args, is_default)
    VALUES
        ('triton/tensorrt-llm', triton_engine_id, triton_version_id, 'TensorRT-LLM optimized inference', 1, 24, 8, '{}', TRUE)
    ON CONFLICT (name) DO UPDATE SET
        description = EXCLUDED.description,
        gpu_count = EXCLUDED.gpu_count,
        memory_gb = EXCLUDED.memory_gb;

    -- Insert TGI configs
    INSERT INTO inference_engine_configs (name, engine_id, version_id, description, gpu_count, memory_gb, cpu_cores, engine_args, is_default)
    VALUES
        ('tgi/default', tgi_engine_id, tgi_version_id, 'Standard TGI config with continuous batching', 1, 16, 4, '{}', TRUE)
    ON CONFLICT (name) DO UPDATE SET
        description = EXCLUDED.description,
        gpu_count = EXCLUDED.gpu_count,
        memory_gb = EXCLUDED.memory_gb;

    RAISE NOTICE 'Seeded inference engines: vllm=%, triton=%, tgi=%', vllm_engine_id, triton_engine_id, tgi_engine_id;
END;
$$;
-- +goose StatementEnd


-- +goose Down
-- Remove engine_config_id from model_deployments
DROP INDEX IF EXISTS idx_model_deployments_engine_config_id;
ALTER TABLE model_deployments DROP COLUMN IF EXISTS engine_config_id;

-- Drop inference_engine_configs
DROP TRIGGER IF EXISTS trigger_inference_engine_configs_updated_at ON inference_engine_configs;
DROP FUNCTION IF EXISTS update_inference_engine_configs_updated_at();
DROP INDEX IF EXISTS idx_inference_engine_configs_is_default;
DROP INDEX IF EXISTS idx_inference_engine_configs_name;
DROP INDEX IF EXISTS idx_inference_engine_configs_engine_id;
DROP TABLE IF EXISTS inference_engine_configs;

-- Drop inference_engine_versions
DROP INDEX IF EXISTS idx_inference_engine_versions_is_default;
DROP INDEX IF EXISTS idx_inference_engine_versions_engine_id;
DROP TABLE IF EXISTS inference_engine_versions;

-- Drop inference_engines
DROP TRIGGER IF EXISTS trigger_inference_engines_updated_at ON inference_engines;
DROP FUNCTION IF EXISTS update_inference_engines_updated_at();
DROP INDEX IF EXISTS idx_inference_engines_name;
DROP TABLE IF EXISTS inference_engines;
