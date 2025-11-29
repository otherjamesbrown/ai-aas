# Feature Specification: Model Management

**Feature Branch**: `020-model-management`
**Created**: 2025-11-28
**Status**: Draft
**Input**: User description: "Provide comprehensive model lifecycle management through ai-aas-cli: registry management, HF token handling, object storage caching, deployment orchestration, validation/audit capabilities, and update workflows. The outcome is reliable, auditable model operations with clear visibility into model state across all layers (registry, cache, deployment, endpoint)."

## Clarifications

### Session 2025-11-28

- Q: Should models be downloaded directly from HuggingFace at pod startup or cached in object storage? → A: Object storage cache is preferred. Direct HF download causes 30+ minute cold starts, rate limit risks, and external dependency at deploy time. Cache provides faster startup (~5 min), version pinning, and air-gap capability.
- Q: How should HF tokens be managed? → A: Tokens stored encrypted via ai-aas-cli credentials command. Tokens used only during `model pull` operations, not at deployment runtime.
- Q: What validation checks are needed? → A: Full-stack validation: registry (model exists, HF accessible), cache (files complete, checksums valid), deployment (probes configured, resources adequate), endpoint (health passing, inference working), router (registered, routing active).
- Q: Should we support model aliases (multiple names for same model)? → A: Yes, simple aliases. Enables version transitions (`llama-3-latest` → current version), environment abstraction (`default-model` → different per env), and A/B testing. Simple `alias_name` → `model_id` mapping, no complex resolution chains.
- Q: Should cache cleanup be automatic or always manual? → A: Manual with guardrails. Safety over convenience—accidental deletion during deploy is catastrophic. Storage costs are predictable; surprise deletions aren't. Guardrails: `--dry-run` preview, alerts on threshold, `--keep-versions` protection, never auto-delete if deployment references that version.
- Q: Should validation run automatically on deploy, or be a separate step? → A: Both—validate automatically by default, allow bypass with `--skip-validation` for power users. Post-deploy validation (health checks, inference test) always runs with no skip option.
- Q: How to handle models requiring HF license agreements? → A: Explicit acknowledgment at registration. At `model add`, check if gated, display license info and acceptance URL, prompt for confirmation. Store `license_accepted_at` and `license_accepted_by` in registry. `model pull` fails with clear message if not accepted. Add `--accept-license` flag for CI/CD (implies human accepted externally).

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Model Lifecycle                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐    ┌─────────────────┐    ┌──────────────┐    ┌─────────┐ │
│  │  HuggingFace │───▶│  Object Storage │───▶│   vLLM Pod   │───▶│ Ready   │ │
│  │     Hub      │    │   (S3/Linode)   │    │  (KServe)    │    │         │ │
│  └──────────────┘    └─────────────────┘    └──────────────┘    └─────────┘ │
│         │                    │                     │                         │
│         │  ai-aas-cli         │  ai-aas-cli          │  ai-aas-cli              │
│         │  model pull        │  model deploy       │  model validate         │
│         │                    │                     │                         │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                     Model Registry (PostgreSQL)                       │   │
│  │  - Model metadata, HF IDs, versions, cache locations, deployments    │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## User Scenarios & Testing *(mandatory)*

### User Story 0 (US-000) - CLI Initialization (Priority: P1)

As a platform admin setting up the CLI for the first time, I can run an initialization wizard that configures all required settings so I can start managing models immediately.

**Why this priority**: Foundation for all CLI operations. Without initialization, no other commands can authenticate or connect to the platform.

**Independent Test**: Can be tested by running init on a fresh install and verifying all subsequent commands work.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a fresh CLI installation, **When** I run `ai-aas-cli --init`, **Then** CLI first checks if it's in the user's PATH and provides shell-specific instructions if not, then launches an interactive wizard that prompts for: Admin API key, API endpoint URL, default environment, and optional HuggingFace token.

2. **[Primary]** **Given** the CLI is not in PATH, **When** init detects this, **Then** CLI displays instructions appropriate to the user's shell:
   - **bash/zsh**: `echo 'export PATH="$PATH:/path/to/ai-aas-cli"' >> ~/.bashrc && source ~/.bashrc`
   - **fish**: `set -Ua fish_user_paths /path/to/ai-aas-cli`
   - **PowerShell**: Instructions to add to `$PROFILE`
   - **Windows CMD**: Instructions to update System Environment Variables

3. **[Primary]** **Given** the init wizard is running, **When** I provide the Admin API key, **Then** CLI validates the key against the API endpoint and confirms authentication success with user/role info.

4. **[Primary]** **Given** init completes successfully, **When** I run any model command, **Then** CLI uses the stored configuration without prompting again.

5. **[Alternate]** **Given** I want non-interactive setup (CI/CD), **When** I run `ai-aas-cli --init --api-key <key> --endpoint <url> --environment <env>`, **Then** CLI configures without prompts (skips PATH check in non-interactive mode).

6. **[Primary]** **Given** existing configuration exists, **When** I run `ai-aas-cli --init`, **Then** CLI warns about existing config and asks to overwrite or update specific values.

7. **[Primary]** **Given** I want to view current configuration, **When** I run `ai-aas-cli config show`, **Then** CLI displays current settings with sensitive values masked, plus PATH status.

8. **[Alternate]** **Given** I need to update a single setting, **When** I run `ai-aas-cli config set api-key <new-key>`, **Then** only that setting is updated without full re-initialization.

9. **[Exception]** **Given** invalid API key or unreachable endpoint, **When** init validation fails, **Then** CLI shows clear error and allows retry without restarting wizard.

10. **[Primary]** **Given** configuration is complete, **When** I run `ai-aas-cli config test`, **Then** CLI validates all connections (API, object storage if configured) and reports status.

11. **[Alternate]** **Given** I want to check PATH status only, **When** I run `ai-aas-cli config check-path`, **Then** CLI reports whether it's in PATH and provides instructions if not.

---

### User Story 1 (US-001) - Model Registry & Discovery (Priority: P1)

As a platform admin, I can register models in the platform, view all known models, and understand their current state across all layers.

**Why this priority**: Foundation for all model operations. Without a registry, cannot track what models exist, their versions, or their deployment state.

**Independent Test**: Can be tested by adding models to registry and querying their status. Delivers immediate value: single source of truth for model inventory.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a HuggingFace model ID (e.g., `meta-llama/Llama-3-8B-Instruct`), **When** I run `ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b`, **Then** model is registered with metadata (HF ID, name, requires-auth flag), and registry entry is created.

2. **[Primary]** **Given** models exist in registry, **When** I run `ai-aas-cli model list`, **Then** CLI displays table with columns: Name, HF Model ID, Cached, Deployed, Status, and summary counts.

3. **[Primary]** **Given** a registered model, **When** I run `ai-aas-cli model info llama-3-8b`, **Then** CLI displays full details: registry info, cache status, deployment status, endpoint health, last validation timestamp.

4. **[Alternate]** **Given** I want to see only deployed models, **When** I run `ai-aas-cli model list --deployed`, **Then** only models with active deployments are shown.

5. **[Alternate]** **Given** I want to find orphaned cache entries, **When** I run `ai-aas-cli model list --orphaned`, **Then** models cached but not registered are displayed for cleanup.

6. **[Exception]** **Given** an invalid HF model ID, **When** I attempt to add it, **Then** CLI validates against HF API (if token available) and returns clear error if model doesn't exist.

7. **[Primary]** **Given** a gated HF model requiring license acceptance, **When** I run `ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b`, **Then** CLI detects gated status, displays license type and acceptance URL, and prompts for confirmation that license was accepted on HuggingFace.

8. **[Alternate]** **Given** CI/CD automation needs, **When** I run `ai-aas-cli model add meta-llama/Llama-3-8B-Instruct --name llama-3-8b --accept-license`, **Then** CLI skips interactive prompt (implies human accepted externally) and records acceptance.

9. **[Exception]** **Given** a gated model where license not accepted, **When** I run `ai-aas-cli model pull llama-3-8b`, **Then** CLI fails with clear error message indicating license must be accepted on HuggingFace first, with link to acceptance page.

---

### User Story 2 (US-002) - Credentials Management (Priority: P1)

As a platform admin, I can securely store and manage HuggingFace tokens and object storage credentials for model operations.

**Why this priority**: Required for accessing gated models and object storage. Without credentials, cannot pull most production models.

**Independent Test**: Can be tested by setting credentials and verifying they work against HF API. Delivers value immediately: enables gated model access.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a HuggingFace token, **When** I run `ai-aas-cli credentials set hf-token <token>`, **Then** token is stored encrypted in secure storage (Kubernetes secret or Vault), and confirmation is displayed.

2. **[Primary]** **Given** stored credentials, **When** I run `ai-aas-cli credentials list`, **Then** CLI shows configured credential types with masked values (e.g., `hf-token: hf_***abc123`).

3. **[Primary]** **Given** a stored HF token, **When** I run `ai-aas-cli credentials test hf-token`, **Then** CLI validates token against HF API, shows account info (username, rate limits), and confirms access level.

4. **[Primary]** **Given** object storage credentials, **When** I run `ai-aas-cli credentials set s3 --access-key <key> --secret-key <secret> --endpoint <url>`, **Then** credentials are stored and bucket access is verified.

5. **[Exception]** **Given** an invalid or expired token, **When** I run `credentials test`, **Then** CLI returns clear error indicating the issue (invalid, expired, insufficient permissions).

6. **[Alternate]** **Given** I need to rotate credentials, **When** I run `ai-aas-cli credentials set hf-token <new-token>`, **Then** old credential is replaced and audit log records the rotation.

---

### User Story 3 (US-003) - Model Caching (Priority: P1)

As a platform admin, I can download models from HuggingFace to object storage for fast, reliable deployments.

**Why this priority**: Eliminates 30+ minute cold starts and HF dependency at deploy time. Critical for production reliability.

**Independent Test**: Can be tested by pulling a model and verifying files in object storage. Delivers value: deployment time reduced from 30+ min to ~5 min.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a registered model with HF token configured, **When** I run `ai-aas-cli model pull llama-3-8b`, **Then** CLI downloads model from HF, uploads to object storage, records cache entry in registry, and shows progress (bytes downloaded, ETA).

2. **[Primary]** **Given** a model pull in progress, **When** download completes, **Then** CLI verifies file checksums against HF manifest, reports completion with size and location (`s3://models/llama-3-8b/v1.0.0-abc123/`).

3. **[Primary]** **Given** cached models exist, **When** I run `ai-aas-cli model cache list`, **Then** CLI displays cached models with version, size, cache date, and storage path.

4. **[Alternate]** **Given** I want a specific version, **When** I run `ai-aas-cli model pull llama-3-8b --revision abc123`, **Then** that specific HF commit is downloaded and cached with version tag.

5. **[Alternate]** **Given** I want to preview download, **When** I run `ai-aas-cli model pull llama-3-8b --dry-run`, **Then** CLI shows files to be downloaded, total size, and estimated time without downloading.

6. **[Primary]** **Given** a cached model, **When** I run `ai-aas-cli model cache verify llama-3-8b`, **Then** CLI verifies all files exist with correct checksums and reports integrity status.

7. **[Exception]** **Given** download fails midway (network error), **When** I re-run pull, **Then** CLI resumes from last checkpoint rather than restarting.

8. **[Alternate]** **Given** old cache versions, **When** I run `ai-aas-cli model cache gc --keep-versions 2`, **Then** versions older than the 2 most recent are deleted and storage is reclaimed.

---

### User Story 4 (US-004) - Model Deployment (Priority: P2)

As a platform admin, I can deploy cached models to Kubernetes with proper configuration for probes, resources, and scaling.

**Why this priority**: Enables model serving. Depends on caching (US-003) being available.

**Independent Test**: Can be tested by deploying a cached model and verifying pod runs with inference working.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a cached model, **When** I run `ai-aas-cli model deploy llama-3-8b --environment development`, **Then** CLI runs pre-deploy validation (cache integrity, resource availability), generates InferenceService manifest, applies to cluster, and waits for pod ready.

2. **[Primary]** **Given** deployment in progress, **When** pod starts, **Then** CLI shows deployment progress (pod scheduled, image pulled, model loading, health check passing), followed by post-deploy validation (health checks, inference test).

3. **[Alternate]** **Given** a known-good model needing quick redeploy, **When** I run `ai-aas-cli model deploy llama-3-8b --environment development --skip-validation`, **Then** CLI skips pre-deploy validation (post-deploy validation still runs).

4. **[Primary]** **Given** deployment request, **When** I run with `--dry-run`, **Then** CLI outputs the Kubernetes YAML that would be applied without applying it.

5. **[Alternate]** **Given** custom resource requirements, **When** I run `ai-aas-cli model deploy llama-3-8b --gpu-count 2 --memory 56Gi --replicas 1-3`, **Then** deployment uses specified resources and autoscaling bounds.

6. **[Primary]** **Given** a deployed model, **When** I run `ai-aas-cli model undeploy llama-3-8b --environment development`, **Then** InferenceService is deleted with graceful drain period.

7. **[Alternate]** **Given** a running deployment, **When** I run `ai-aas-cli model restart llama-3-8b`, **Then** rolling restart is performed with zero-downtime.

8. **[Alternate]** **Given** a running deployment, **When** I run `ai-aas-cli model scale llama-3-8b --replicas 3`, **Then** replica count is updated.

9. **[Exception]** **Given** deployment fails (insufficient GPU, image pull error), **When** failure occurs, **Then** CLI shows clear error with pod events and suggests remediation.

10. **[Exception]** **Given** pre-deploy validation fails (cache corrupted, insufficient resources), **When** deploy is attempted, **Then** CLI fails fast with clear error and remediation before any cluster changes.

---

### User Story 5 (US-005) - Model Validation & Audit (Priority: P2)

As a platform admin, I can validate that models are correctly configured across all layers and diagnose issues.

**Why this priority**: Essential for troubleshooting and ensuring production readiness. Provides confidence before go-live.

**Independent Test**: Can be tested by running validation on a model and verifying all checks pass or fail appropriately.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a deployed model, **When** I run `ai-aas-cli model validate llama-3-8b --environment development`, **Then** CLI runs all validation checks and displays results categorized by layer (registry, cache, deployment, endpoint, router).

2. **[Primary]** **Given** validation checks, **Then** the following checks are performed:
   - **Registry**: Model registered, HF token configured (if needed), HF model accessible
   - **Cache**: Model cached, files complete, checksums valid, version current
   - **Deployment**: InferenceService exists, pod running, startup/readiness/liveness probes configured, GPU allocated, memory sufficient
   - **Endpoint**: Service exists, health check passes, model loaded, inference test works
   - **Router**: Registered in API router, routing policy exists, rate limits configured

3. **[Primary]** **Given** validation failures, **When** checks fail, **Then** CLI provides clear remediation suggestions for each failure.

4. **[Alternate]** **Given** auto-fixable issues, **When** I run `ai-aas-cli model validate llama-3-8b --fix`, **Then** CLI automatically fixes issues where safe (e.g., probe timeout too short) and reports what was fixed.

5. **[Primary]** **Given** all models need checking, **When** I run `ai-aas-cli model validate --environment development`, **Then** CLI validates all registered models and shows summary table.

6. **[Alternate]** **Given** CI/CD integration needs, **When** I run `ai-aas-cli model validate --json`, **Then** output is machine-readable JSON with check results and exit code reflects pass/fail.

7. **[Primary]** **Given** quick status needed, **When** I run `ai-aas-cli model status`, **Then** CLI shows condensed status table without full validation details.

---

### User Story 6 (US-006) - Model Updates (Priority: P3)

As a platform admin, I can check for and apply model updates from HuggingFace with controlled rollout.

**Why this priority**: Enables keeping models current. Lower priority as most deployments use pinned versions.

**Independent Test**: Can be tested by checking for updates and applying one with rollback capability.

**Acceptance Scenarios**:

1. **[Primary]** **Given** cached models, **When** I run `ai-aas-cli model check-updates`, **Then** CLI queries HF for each model's latest version and compares to cached version.

2. **[Primary]** **Given** update available, **When** I run `ai-aas-cli model update llama-3-8b`, **Then** CLI pulls new version to cache, then performs rolling update of deployment.

3. **[Alternate]** **Given** cautious rollout needed, **When** I run `ai-aas-cli model update llama-3-8b --canary 10%`, **Then** only 10% of traffic routes to new version until manually promoted.

4. **[Exception]** **Given** update fails health checks, **When** new version doesn't pass validation, **Then** CLI automatically rolls back and reports failure reason.

5. **[Alternate]** **Given** I want to pin a specific version, **When** I run `ai-aas-cli model pin llama-3-8b --version v1.0.0`, **Then** model is marked as pinned and `check-updates` skips it.

---

### User Story 7 (US-007) - Troubleshooting (Priority: P3)

As a platform admin, I can diagnose model issues using integrated troubleshooting commands.

**Why this priority**: Aids debugging. Can use kubectl directly as fallback, so lower priority.

**Independent Test**: Can be tested by viewing logs and events for a problematic deployment.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a deployed model, **When** I run `ai-aas-cli model logs llama-3-8b`, **Then** CLI streams pod logs with appropriate formatting.

2. **[Primary]** **Given** a deployed model, **When** I run `ai-aas-cli model events llama-3-8b`, **Then** CLI shows recent Kubernetes events for the deployment.

3. **[Primary]** **Given** a deployed model, **When** I run `ai-aas-cli model describe llama-3-8b`, **Then** CLI shows full deployment details (pod spec, status, resource usage).

4. **[Primary]** **Given** endpoint issues, **When** I run `ai-aas-cli model test llama-3-8b`, **Then** CLI runs inference test and reports latency, success, and any errors.

---

### User Story 8 (US-008) - Model Enable/Disable (Library Management) (Priority: P2)

As a platform admin, I can enable and disable models from a library of configured and cached models, allowing quick capacity management without losing configuration or cached data.

**Why this priority**: Critical for operations—allows managing GPU capacity by swapping models in/out without re-caching (which takes 30+ minutes). Enables cost-effective scaling.

**Independent Test**: Can be tested by disabling a deployed model, verifying it's undeployed but config/cache remain, then re-enabling and confirming fast startup.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a deployed model, **When** I run `ai-aas-cli model disable mistral-7b --environment production`, **Then** model is undeployed (InferenceService removed), but registry entry and cached files remain intact.

2. **[Primary]** **Given** a disabled model with cached data, **When** I run `ai-aas-cli model enable mistral-7b --environment production`, **Then** model deploys quickly (~5 min) using cached data, without re-downloading from HuggingFace.

3. **[Primary]** **Given** models in various states, **When** I run `ai-aas-cli model list --environment production`, **Then** CLI shows status column with: `enabled` (deployed & running), `disabled` (configured but not deployed), `cached` (cached but never deployed to this env).

4. **[Primary]** **Given** I want to see library overview, **When** I run `ai-aas-cli model library`, **Then** CLI shows all models with columns: Name, Cached, Enabled (per environment), Last Enabled, Last Disabled.

5. **[Alternate]** **Given** I need to swap models for capacity, **When** I run `ai-aas-cli model swap mistral-7b llama-3-8b --environment production`, **Then** CLI disables first model, waits for graceful drain, then enables second model (atomic swap).

6. **[Alternate]** **Given** I want to enable multiple models, **When** I run `ai-aas-cli model enable mistral-7b qwen-7b --environment production`, **Then** both models are deployed in parallel.

7. **[Exception]** **Given** a model is disabled, **When** API router receives request for that model, **Then** router returns 503 with message indicating model is disabled (not 404), suggesting enabled alternatives.

8. **[Alternate]** **Given** I want to schedule capacity changes, **When** I run `ai-aas-cli model disable mistral-7b --environment production --at "2025-01-01T00:00:00Z"`, **Then** CLI schedules the disable operation for the specified time. (Format: ISO 8601, e.g., `2025-01-01T00:00:00Z` or `2025-01-01T00:00:00-05:00`)

9. **[Primary]** **Given** I want to understand enable/disable history, **When** I run `ai-aas-cli model history mistral-7b`, **Then** CLI shows audit log of enable/disable events with timestamps and operators.

---

### User Story 9 (US-009) - Model Aliases (Priority: P3)

As a platform admin, I can create aliases for models to enable version transitions, environment abstraction, and A/B testing without client changes.

**Why this priority**: Convenience feature that enables smoother operations. Core functionality works without aliases.

**Independent Test**: Can be tested by creating an alias and verifying it resolves to the correct model.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a registered model, **When** I run `ai-aas-cli model alias create llama-latest --target llama-3-8b`, **Then** alias is created and resolves to the target model in all commands.

2. **[Primary]** **Given** aliases exist, **When** I run `ai-aas-cli model alias list`, **Then** CLI displays all aliases with their target models.

3. **[Primary]** **Given** an alias exists, **When** I run `ai-aas-cli model info llama-latest`, **Then** CLI resolves the alias and shows the target model's information.

4. **[Alternate]** **Given** I need to update an alias target, **When** I run `ai-aas-cli model alias update llama-latest --target llama-3-70b`, **Then** alias is updated to point to new model.

5. **[Primary]** **Given** an alias exists, **When** I run `ai-aas-cli model alias delete llama-latest`, **Then** alias is removed (target model unaffected).

6. **[Exception]** **Given** an alias pointing to a deployed model, **When** target model is removed, **Then** CLI warns about orphaned alias and requires `--force` to proceed.

---

## Data Model

### Database Schema

```sql
-- Model registry: what models we know about
CREATE TABLE model_registry (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,           -- Internal name: "llama-3-8b"
    hf_model_id VARCHAR(512) NOT NULL,           -- "meta-llama/Llama-3-8B-Instruct"
    hf_revision VARCHAR(255) DEFAULT 'main',     -- HF branch/tag/commit
    requires_auth BOOLEAN DEFAULT false,         -- Needs HF token
    is_gated BOOLEAN DEFAULT false,              -- Requires HF license acceptance
    license_type VARCHAR(100),                   -- "llama3", "apache-2.0", etc.
    license_url VARCHAR(512),                    -- URL to accept license on HF
    license_accepted_at TIMESTAMP,               -- When license was acknowledged
    license_accepted_by VARCHAR(255),            -- Who acknowledged (for audit)
    recommended_gpu_memory_gb INT,               -- Minimum GPU memory
    recommended_cpu_memory_gb INT,               -- Minimum system memory
    pinned_version VARCHAR(255),                 -- If set, skip update checks
    metadata JSONB DEFAULT '{}',                 -- Additional metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Model aliases: alternative names for models
CREATE TABLE model_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias_name VARCHAR(255) UNIQUE NOT NULL,     -- "llama-latest", "default-model"
    model_id UUID REFERENCES model_registry(id) ON DELETE RESTRICT,
    description VARCHAR(512),                    -- Optional description of alias purpose
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Cached versions in object storage
CREATE TABLE model_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID REFERENCES model_registry(id) ON DELETE CASCADE,
    version VARCHAR(255) NOT NULL,               -- HF commit hash
    hf_revision VARCHAR(255),                    -- Branch/tag that was pulled
    object_storage_path VARCHAR(512) NOT NULL,   -- "s3://models/llama-3-8b/v1.0/"
    size_bytes BIGINT,
    file_count INT,
    checksum_sha256 VARCHAR(64),                 -- Manifest checksum
    status VARCHAR(50) DEFAULT 'downloading',    -- downloading, ready, failed, deleted
    cached_at TIMESTAMP DEFAULT NOW(),
    verified_at TIMESTAMP,
    UNIQUE(model_id, version)
);

-- Deployments: what's running where
CREATE TABLE model_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID REFERENCES model_registry(id) ON DELETE CASCADE,
    cache_id UUID REFERENCES model_cache(id),
    environment VARCHAR(50) NOT NULL,            -- development, staging, production
    namespace VARCHAR(100) NOT NULL,
    inferenceservice_name VARCHAR(255),
    endpoint VARCHAR(512),
    enabled BOOLEAN DEFAULT true,                -- false = disabled (config kept, not deployed)
    status VARCHAR(50) DEFAULT 'pending',        -- pending, deploying, ready, failed, disabled, terminated
    replicas_desired INT DEFAULT 1,
    replicas_ready INT DEFAULT 0,
    gpu_count INT DEFAULT 1,
    memory_gb INT,
    last_health_check_at TIMESTAMP,
    last_health_status VARCHAR(50),
    last_enabled_at TIMESTAMP,                   -- When model was last enabled
    last_enabled_by VARCHAR(255),                -- Who enabled it
    last_disabled_at TIMESTAMP,                  -- When model was last disabled
    last_disabled_by VARCHAR(255),               -- Who disabled it
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(model_id, environment)
);

-- Enable/disable history for audit
CREATE TABLE model_state_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID REFERENCES model_deployments(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL,                 -- enabled, disabled, swapped_out, swapped_in
    performed_by VARCHAR(255),
    reason TEXT,                                 -- Optional reason for change
    scheduled_at TIMESTAMP,                      -- If scheduled operation
    executed_at TIMESTAMP DEFAULT NOW()
);

-- Validation results
CREATE TABLE model_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID REFERENCES model_registry(id) ON DELETE CASCADE,
    environment VARCHAR(50),
    validation_type VARCHAR(100),                -- registry, cache, deployment, endpoint, router
    check_name VARCHAR(100),
    status VARCHAR(20),                          -- pass, warn, fail, skip
    message TEXT,
    remediation TEXT,
    validated_at TIMESTAMP DEFAULT NOW()
);

-- Credentials (encrypted)
CREATE TABLE platform_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_type VARCHAR(100) UNIQUE NOT NULL, -- hf-token, s3-access, s3-secret
    encrypted_value TEXT NOT NULL,               -- Encrypted with platform key
    metadata JSONB DEFAULT '{}',                 -- e.g., {"endpoint": "..."}
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Object Storage Structure

```
s3://ai-aas-models/
├── llama-3-8b/
│   ├── v1.0.0-abc123/
│   │   ├── config.json
│   │   ├── tokenizer.json
│   │   ├── tokenizer_config.json
│   │   ├── model-00001-of-00004.safetensors
│   │   ├── model-00002-of-00004.safetensors
│   │   ├── model-00003-of-00004.safetensors
│   │   ├── model-00004-of-00004.safetensors
│   │   └── .manifest.json          # File list with checksums
│   └── v1.1.0-def789/
│       └── ...
├── qwen2-7b/
│   └── v1.0.0-xyz789/
│       └── ...
└── .index.json                      # Global index of all cached models
```

---

## CLI Command Reference

### Initialization & Configuration

```bash
ai-aas-cli --init [--api-key <key>] [--endpoint <url>] [--environment <env>] [--hf-token <token>]
ai-aas-cli config show
ai-aas-cli config set <key> <value>
ai-aas-cli config test
ai-aas-cli config check-path
ai-aas-cli --version
ai-aas-cli --help
```

### Discovery & Status

```bash
ai-aas-cli model list [--cached] [--deployed] [--orphaned] [--format table|json|csv]
ai-aas-cli model info <model-name>
ai-aas-cli model status [model-name] [--environment <env>]
```

### Registry Management

```bash
ai-aas-cli model add <hf-model-id> --name <internal-name> [--requires-auth] [--license <type>] [--accept-license]
ai-aas-cli model remove <model-name> [--force]
ai-aas-cli model update-metadata <model-name> [--recommended-gpu-memory <GB>] [--license <type>]
```

### Aliases

```bash
ai-aas-cli model alias create <alias-name> --target <model-name> [--description <text>]
ai-aas-cli model alias list
ai-aas-cli model alias update <alias-name> --target <model-name>
ai-aas-cli model alias delete <alias-name>
```

### Credentials

```bash
ai-aas-cli credentials set hf-token <token>
ai-aas-cli credentials set s3 --access-key <key> --secret-key <secret> --endpoint <url> --bucket <name>
ai-aas-cli credentials list
ai-aas-cli credentials test <credential-type>
ai-aas-cli credentials delete <credential-type>
```

### Caching

```bash
ai-aas-cli model pull <model-name> [--revision <tag|commit>] [--dry-run]
ai-aas-cli model cache list [--model <name>]
ai-aas-cli model cache verify <model-name> [--version <version>]
ai-aas-cli model cache delete <model-name> [--version <version>] [--all-versions]
ai-aas-cli model cache gc [--keep-versions <n>] [--older-than <duration>]
```

### Deployment

```bash
ai-aas-cli model deploy <model-name> --environment <env> [--gpu-count <n>] [--memory <GB>] [--replicas <min>-<max>] [--dry-run] [--skip-validation]
ai-aas-cli model undeploy <model-name> --environment <env> [--force]
ai-aas-cli model restart <model-name> --environment <env>
ai-aas-cli model scale <model-name> --environment <env> --replicas <n>
```

### Enable/Disable (Library Management)

```bash
ai-aas-cli model library [--environment <env>]
ai-aas-cli model enable <model-name> [model-name...] --environment <env>
ai-aas-cli model disable <model-name> --environment <env> [--at <ISO8601-datetime>] [--reason <text>]
ai-aas-cli model swap <disable-model> <enable-model> --environment <env>
ai-aas-cli model history <model-name> [--environment <env>] [--limit <n>]
```

### Validation

```bash
ai-aas-cli model validate [model-name] [--environment <env>] [--fix] [--json]
```

### Updates

```bash
ai-aas-cli model check-updates [model-name]
ai-aas-cli model update <model-name> [--environment <env>] [--canary <percent>]
ai-aas-cli model pin <model-name> --version <version>
ai-aas-cli model unpin <model-name>
```

### Troubleshooting

```bash
ai-aas-cli model logs <model-name> [--environment <env>] [--follow] [--tail <n>]
ai-aas-cli model events <model-name> [--environment <env>]
ai-aas-cli model describe <model-name> [--environment <env>]
ai-aas-cli model test <model-name> [--environment <env>] [--prompt <text>]
```

---

## Non-Functional Requirements

### Performance

- `model list` should return in <1 second for up to 100 models
- `model pull` should show progress updates every 5 seconds
- `model validate` should complete all checks in <30 seconds per model
- Object storage uploads should use multipart upload for files >100MB

### Security

- HF tokens stored encrypted at rest (AES-256)
- Credentials never logged or displayed in full
- Audit log for all credential and deployment operations
- Object storage access via IAM roles where possible

### Reliability

- `model pull` supports resume on failure
- `model deploy` has rollback on health check failure
- `model validate` continues on individual check failures
- All destructive operations require `--confirm` or `--force`

---

## Dependencies

- **admin-api-service** (017): Backend API for CLI operations
- **user-org-service** (005): Audit logging integration
- **KServe** (016): InferenceService deployment target
- **Object Storage**: Linode Object Storage or S3-compatible

**Note**: This spec defines `ai-aas-cli` as the new unified CLI tool, replacing `admin-cli` (009).

---

## Out of Scope

- Model fine-tuning or training
- Model quantization (future enhancement)
- Multi-cluster deployment orchestration
- Cost optimization recommendations
- Model performance benchmarking

