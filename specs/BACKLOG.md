# Platform Backlog

A scratch list of tasks, ideas, and improvements to tackle. Add items here so we don't forget!

---

## High Priority

### Infrastructure - DNS & TLS Setup

**Domain:** `otherjamesbrown.com` (GoDaddy)

- [x] **Set up DNS for development cluster** (2024-11-29)
  - Domain: `*.dev.otherjamesbrown.com` → `172.232.58.222`
  - Created wildcard A record in GoDaddy DNS
  - Services available at: `api.dev.otherjamesbrown.com`, `portal.dev.otherjamesbrown.com`, etc.

- [ ] **Set up DNS for production cluster** (when ready)
  - Domain: `*.otherjamesbrown.com` or `*.prod.otherjamesbrown.com`
  - Point to production LoadBalancer IP

- [x] **Configure valid TLS certificates** (2024-11-29)
  - Implemented Let's Encrypt with cert-manager (free, auto-renewal)
  - Created letsencrypt-prod and letsencrypt-staging ClusterIssuers
  - All ingresses configured for automatic certificate provisioning

- [x] **Update all ingress hostnames** (2024-11-29)
  - Replaced `*.172.232.58.222.nip.io` with `*.dev.otherjamesbrown.com`
  - Updated CLI default config
  - Updated docs/platform/environment-access.md

- [x] **Centralize domain configuration** (2024-11-29 - PR #47)
  - Created `configs/environments/development.yaml` and `production.yaml` as canonical domain source
  - Updated all Helm charts to use `global.domain.base` templating
  - Updated ingress templates to auto-compute hosts from global domain
  - CLI uses `AI_AAS_BASE_DOMAIN` environment variable
  - GitHub workflows use repository variables

### Model Deployment
- [x] **Deploy vLLM model to development cluster** (2024-11-30)
  - Registered `mistral-7b-instruct` (Mistral-7B-Instruct-v0.3) in model registry
  - Created vLLM ClusterServingRuntime for KServe
  - Deployed via CLI: `ai-aas model deploy mistral-7b-instruct`
  - Verified inference via OpenAI-compatible API (`/v1/chat/completions`)

- [ ] **Multi-inference-engine support**
  - Currently only supports vLLM runtime
  - Add support for NVIDIA Triton Inference Server
  - Add support for other engines (TensorRT-LLM, text-generation-inference)
  - Create pluggable ClusterServingRuntime abstraction
  - Allow model registry to specify preferred inference engine per model

- [ ] **Per-model inference engine parameters**
  - Models may require different runtime settings (context length, batch size, etc.)
  - Add `inference_config` JSONB field to model_registry for engine-specific params
  - Examples: `max_model_len`, `tensor_parallel_size`, `dtype`, `quantization`
  - CLI should support: `ai-aas model add --inference-config '{"max_model_len": 32768}'`
  - InferenceService should inject these as container args/env vars

- [ ] **GPU type targeting**
  - Add `gpu_type` field to model_registry (e.g., `a100`, `h100`, `rtx4000-ada`)
  - Deploy command should set Kubernetes `nodeSelector` based on GPU type
  - Support scheduling to specific GPU classes based on model requirements

- [ ] **Pre-cache models to Object Storage**
  - Integrate `model pull` with `model deploy` (use S3 cache instead of HF download)
  - Update `model pull` to create `model_cache` DB entry after upload
  - Reduces vLLM cold start time from ~10min to ~2min for large models

### Pipeline & CI/CD
- [ ] Enable GitHub branch protection on `main` (require PR review, status checks)
- [ ] Add Slack/PagerDuty notifications for workflow failures
- [ ] Add dependency vulnerability scanning (Dependabot/Snyk)

---

## Medium Priority

### Web Portal / UI
- [ ] **Add service health dashboard to Web Portal**
  - Display platform health status similar to CLI `ai-aas-cli status` output
  - Show all services: Admin API, API Router, User-Org, Analytics, Grafana, ArgoCD
  - Real-time status indicators (healthy/slow/unhealthy)
  - Latency display for each service

### CLI Improvements
- [ ] Consider raising "slow" threshold from 1s to 2s for external health checks
- [ ] Add `ai-aas-cli config show` command to display current configuration
- [ ] **Fix hardcoded 'main' revision in model library commands** (`library_cmd.go:130`)
  - **Problem:** The `ai-aas-cli model library update` and related commands hardcode `"main"` as the HuggingFace revision, preventing version pinning for reproducible deployments.
  - **Why it matters:**
    - Production environments need exact version control for model artifacts
    - "main" branch on HuggingFace can change without notice
    - No way to update to a specific model version or rollback
  - **Proposed solution:**
    1. Add `--revision` / `-r` flag to library update/sync commands
    2. Store selected revision in model registry metadata
    3. Consider auto-detecting latest SHA from HuggingFace API when `main` is specified
    4. Add `ai-aas-cli model library versions <model>` to list available HuggingFace revisions
- [x] **Refactor model commands to use nested subcommands** (2024-11-30)
  - Refactored 27 flat commands into 6 parent command groups
  - New structure: `model registry/cache/deploy/troubleshoot/version/library`
  - Added world-class `--help` output with examples, workflows, and next steps
  - Updated org/user/apikey commands with improved help text
  - Added CLI-first guidance to CLAUDE.md and AI_ASSISTANT_GUIDE.md

### Observability
- [ ] Add post-sync health validation hooks in ArgoCD
- [ ] Implement SLO monitoring (error rate, latency percentiles)
- [ ] Add code coverage enforcement to CI

### Services
- [ ] Complete production deployment for all services (api-router, analytics, admin-api missing)
- [ ] Add rollback testing for service deployments

---

## Low Priority / Future Ideas

### Platform
- [ ] Implement app-of-apps pattern for production (like development)
- [ ] Add changelog generation for service releases
- [ ] Consider external secret management (Vault, External Secrets Operator)

### Developer Experience
- [ ] Add local development with Tilt or Skaffold
- [ ] Create developer onboarding documentation

---

## Completed

- [x] Fix CLI extractBaseDomain to handle multiple service prefixes (2024-11-29)
- [x] Sync develop branch with main (2024-11-29)
- [x] Fix production RBAC - replace wildcards with explicit whitelist (2024-11-29)
- [x] Add post-deployment health check workflow (2024-11-29)
- [x] Add branch sync check workflow (2024-11-29)
- [x] Add circuit breakers and parallel health checks to API Router
- [x] Add self-healing (HPA, replicas, startup probes) to API Router

---

## Notes

- Using real DNS now (`*.dev.otherjamesbrown.com`) - much faster than nip.io
- Internal API Router responds in ~34 microseconds, external latency depends on network path
- Production RBAC was too permissive with wildcards - now fixed
- All TLS certificates managed by cert-manager with Let's Encrypt (auto-renewal)
