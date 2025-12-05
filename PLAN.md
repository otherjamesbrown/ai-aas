# Implementation Plan: Inference Engine Configuration System (AIAAS-042)

## Overview

Implement a multi-inference-engine support system allowing deployment of models using vLLM, Triton (with TensorRT-LLM backend), and TGI (text-generation-inference).

## Current State

- **Single runtime**: Only `vllm-runtime` ClusterServingRuntime exists
- **Hardcoded image**: `vllm/vllm-openai:v0.6.4` in runtime manifest
- **No version management**: Changing vLLM version requires manual ClusterServingRuntime creation
- **No engine selection**: All models use vLLM regardless of optimization needs

## Proposed Architecture

### Three-Level Configuration Model

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. INFERENCE ENGINES (what servers are available)              │
│    - vLLM, Triton, TGI                                         │
│    - Version management, container images                       │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 2. ENGINE CONFIGS (named resource profiles)                    │
│    - vllm/default, vllm/high-memory, triton/tensorrt-llm       │
│    - GPU count, memory, max-model-len, tensor-parallel-size    │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ 3. MODEL DEPLOYMENTS (model + config binding)                  │
│    - llama-7b deployed with vllm/default                       │
│    - gpt-oss-20b deployed with vllm/high-memory                │
└─────────────────────────────────────────────────────────────────┘
```

## Database Schema Changes

### New Table: `inference_engines`

```sql
CREATE TABLE inference_engines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,          -- e.g., "vllm", "triton", "tgi"
    display_name VARCHAR(255) NOT NULL,          -- e.g., "vLLM OpenAI Server"
    description TEXT,
    default_version VARCHAR(50) NOT NULL,        -- e.g., "0.6.4"
    protocol_versions TEXT[] DEFAULT '{v2}',     -- KServe protocols: v1, v2, grpc-v2
    supported_model_formats TEXT[],              -- e.g., '{vllm,pytorch,safetensors}'
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### New Table: `inference_engine_versions`

```sql
CREATE TABLE inference_engine_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    engine_id UUID NOT NULL REFERENCES inference_engines(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,                -- e.g., "0.6.4", "0.10.2", "0.12.0"
    container_image VARCHAR(500) NOT NULL,       -- e.g., "vllm/vllm-openai:v0.6.4"
    is_default BOOLEAN DEFAULT FALSE,
    is_deprecated BOOLEAN DEFAULT FALSE,
    min_gpu_memory_gb INTEGER,                   -- Minimum GPU memory required
    release_notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(engine_id, version)
);
```

### New Table: `inference_engine_configs`

```sql
CREATE TABLE inference_engine_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,           -- e.g., "vllm/default", "vllm/high-memory"
    engine_id UUID NOT NULL REFERENCES inference_engines(id),
    version_id UUID REFERENCES inference_engine_versions(id),  -- NULL = use default
    description TEXT,

    -- Resource configuration
    gpu_count INTEGER DEFAULT 1,
    gpu_type VARCHAR(50),                        -- e.g., "rtx-4090", "a100-40gb" (optional)
    memory_gb INTEGER DEFAULT 16,
    cpu_cores INTEGER DEFAULT 4,

    -- Engine-specific configuration (JSONB for flexibility)
    engine_args JSONB DEFAULT '{}',              -- e.g., {"max-model-len": 32768, "tensor-parallel-size": 2}

    -- Scaling configuration
    min_replicas INTEGER DEFAULT 1,
    max_replicas INTEGER DEFAULT 1,

    -- Health check configuration
    startup_timeout_seconds INTEGER DEFAULT 900,  -- 15 min for large models

    is_default BOOLEAN DEFAULT FALSE,            -- One default per engine
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Modify: `model_deployments`

```sql
ALTER TABLE model_deployments
    ADD COLUMN engine_config_id UUID REFERENCES inference_engine_configs(id);
```

## Seed Data

### Default Engines

| Name | Display Name | Default Version | Container Image |
|------|--------------|-----------------|-----------------|
| vllm | vLLM OpenAI Server | 0.6.4 | vllm/vllm-openai:v0.6.4 |
| triton | NVIDIA Triton | 24.04 | nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3 |
| tgi | Text Generation Inference | 2.4.1 | ghcr.io/huggingface/text-generation-inference:2.4.1 |

### Default Configs

| Name | Engine | GPU | Memory | Notes |
|------|--------|-----|--------|-------|
| vllm/default | vllm | 1 | 16GB | Standard 7B models |
| vllm/high-memory | vllm | 1 | 48GB | 20B+ models, max-model-len: 32768 |
| vllm/multi-gpu | vllm | 2 | 96GB | 70B models, tensor-parallel-size: 2 |
| triton/tensorrt-llm | triton | 1 | 24GB | TensorRT-LLM optimized |
| tgi/default | tgi | 1 | 16GB | HuggingFace TGI |

## CLI Commands

### Engine Management

```bash
# List available engines
ai-aas-cli engine list
# Output:
# NAME     VERSION   IMAGE                                    STATUS
# vllm     0.6.4     vllm/vllm-openai:v0.6.4                 active
# triton   24.04     nvcr.io/nvidia/tritonserver:24.04...    active
# tgi      2.4.1     ghcr.io/huggingface/text-generation...  active

# Add new engine version
ai-aas-cli engine version add vllm 0.12.0 --image vllm/vllm-openai:v0.12.0

# Set default version
ai-aas-cli engine version set-default vllm 0.12.0

# Show engine details
ai-aas-cli engine show vllm
```

### Config Management

```bash
# List configs
ai-aas-cli engine config list
# Output:
# NAME              ENGINE   GPU   MEMORY   DEFAULT
# vllm/default      vllm     1     16GB     yes
# vllm/high-memory  vllm     1     48GB     no
# triton/tensorrt   triton   1     24GB     yes

# Create config
ai-aas-cli engine config create vllm/large-context \
    --engine vllm \
    --gpu 1 \
    --memory 32 \
    --arg max-model-len=65536 \
    --arg gpu-memory-utilization=0.95

# Show config details
ai-aas-cli engine config show vllm/high-memory

# Delete config
ai-aas-cli engine config delete vllm/large-context
```

### Model Deployment with Config

```bash
# Deploy with specific engine config
ai-aas-cli model deploy create llama-7b \
    --engine-config vllm/default \
    -e development

# Deploy with engine config override
ai-aas-cli model hf-deploy mistral-7b-instruct \
    --engine-config vllm/high-memory \
    --arg max-model-len=16384 \
    -e development
```

## ClusterServingRuntime Generation

The system will dynamically generate ClusterServingRuntimes based on engine configs:

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ClusterServingRuntime
metadata:
  name: {{ .ConfigName }}-runtime   # e.g., "vllm-high-memory-runtime"
  labels:
    ai-aas.io/engine: {{ .EngineName }}
    ai-aas.io/config: {{ .ConfigName }}
spec:
  supportedModelFormats:
    - name: {{ .EngineName }}
      version: "1"
      autoSelect: true
  protocolVersions: {{ .ProtocolVersions }}
  containers:
    - name: kserve-container
      image: {{ .ContainerImage }}
      args: {{ .GeneratedArgs }}
      resources:
        requests:
          cpu: {{ .CPUCores }}
          memory: {{ .MemoryGB }}Gi
          nvidia.com/gpu: {{ .GPUCount }}
        limits:
          cpu: {{ .CPUCores * 2 }}
          memory: {{ .MemoryGB * 2 }}Gi
          nvidia.com/gpu: {{ .GPUCount }}
```

## Implementation Phases

### Phase 1: Database Schema & Admin API (ai-aas-ti8)
1. Create migration for new tables
2. Add seed data for default engines/configs
3. Implement CRUD endpoints in Admin API:
   - `GET/POST /engines`
   - `GET/POST/DELETE /engines/{name}/versions`
   - `GET/POST/PUT/DELETE /engine-configs`
4. Add engine_config_id to model deployments

### Phase 2: CLI Commands (ai-aas-04s)
1. Add `engine` command group
2. Add `engine config` subcommands
3. Update `model deploy create` with `--engine-config` flag
4. Update `model hf-deploy` with `--engine-config` flag

### Phase 3: Runtime Generation (ai-aas-fpg)
1. Implement ClusterServingRuntime template generation
2. Add GPU type node selector support
3. Integrate with existing InferenceService generation
4. Add ArgoCD sync for generated runtimes

### Phase 4: Testing & Documentation
1. Unit tests for new services
2. Integration tests for deployment flow
3. Update CLI help and documentation
4. Add runbook for engine management

## File Changes Summary

| Component | Files to Create/Modify |
|-----------|------------------------|
| **Database** | `db/migrations/20251205_001_inference_engines.sql` |
| **Admin API** | `services/admin-api-service/internal/services/engines/` (new) |
| **Admin API** | `services/admin-api-service/internal/handlers/engines/` (new) |
| **CLI** | `services/ai-aas-cli/internal/commands/engine/` (new) |
| **CLI** | `services/ai-aas-cli/internal/commands/model/deploy.go` (modify) |
| **K8s Templates** | `services/admin-api-service/internal/kubernetes/runtime.go` (new) |
| **Seed Data** | `db/seeds/inference_engines.sql` (new) |

## Open Questions

1. **Runtime lifecycle**: Should we auto-generate ClusterServingRuntimes on config creation, or on first deployment?
2. **Version compatibility**: How do we handle breaking changes between engine versions?
3. **GPU targeting**: Should GPU type be on the config level or deployment level?

## References

- [KServe Serving Runtimes](https://kserve.github.io/website/master/modelserving/servingruntimes/)
- [Triton Inference Server](https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/index.html)
- [TensorRT-LLM with KServe](https://www.alibabacloud.com/blog/building-a-large-language-model-inference-service-optimized-by-tensorrt-llm-based-on-kserve-on-asm_601556)
- [Text Generation Inference](https://huggingface.co/docs/text-generation-inference/en/index)
