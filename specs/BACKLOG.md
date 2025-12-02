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

- [x] **Fix API Router to KServe networking** (2024-12-02)
  - **Problem:** API Router cannot connect to KServe InferenceServices - requests timeout
  - **Root cause:** Port 80 on ClusterIP services routes through Istio which causes timeouts
  - **Solution:** Use the `-private` service on port 8012 (queue-proxy port)
    - This bypasses Istio routing and connects directly to the queue-proxy sidecar
    - Example: `http://model-predictor-00001-private.namespace.svc.cluster.local:8012`
  - **Key learning:** KServe exposes multiple service endpoints:
    - Port 80 on regular service: Routes through Istio gateway (needs Host header)
    - Port 8012 on -private service: Direct queue-proxy access (recommended for internal services)

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

- [ ] **Server-side model caching (background job worker)**
  - Current `ai-aas model cache pull` downloads client-side (requires local disk space + bandwidth)
  - For large models (27GB+ like Mistral-7B), this is impractical from dev machines
  - **Proposed solution**: Background job worker to process `PullJob` records
    - Admin-api already has `CreatePullJob` which inserts `pending` status records
    - Need a worker process to pick up pending jobs and execute them server-side
    - Options: Kubernetes Job, dedicated worker pod, or serverless function
  - Worker would have access to S3 credentials and HuggingFace token
  - CLI would just create the job and poll for status

### Pipeline & CI/CD
- [ ] Enable GitHub branch protection on `main` (require PR review, status checks)
- [ ] Add Slack/PagerDuty notifications for workflow failures
- [ ] Add dependency vulnerability scanning (Dependabot/Snyk)

---

## Medium Priority

### Testing & Quality

- [ ] **Full integration test suite for user-model-access-control**
  - **Context**: E2E CLI-based tests exist at `tests/e2e/scripts/test-user-model-access.sh`
  - **Missing tests**:
    - Handler integration tests (Admin API endpoints with real database)
    - API Router integration tests (model access enforcement at inference time)
    - Repository unit tests for edge cases
  - **Proposed structure**:
    ```
    tests/
    ├── e2e/scripts/test-user-model-access.sh     # ✅ Created
    ├── integration/
    │   ├── admin-api/user-model-access_test.go   # Handler tests
    │   └── api-router/access-enforcement_test.go # Inference-time enforcement
    └── unit/
        └── repository/user-model-access_test.go  # Edge cases
    ```
  - **Why it matters**: Ensures the access control system works correctly across all layers (API, database, inference)

### Web Portal / UI

- [ ] **UI Resilience & Defensive Coding** (Added 2024-12-02)
  - **Problem:** UI crashes when API returns incomplete data (e.g., missing `billing_contact`, `address` on organization profile)
  - **Root cause:** Components access nested properties without null checks (e.g., `profile.billing_contact.name`)
  - **Current workaround:** Added defensive defaults in OrganizationSettingsPage
  - **Recommended improvements:**
    1. Add Zod/TypeScript validation for all API responses at the service layer
    2. Create utility functions for safe nested property access
    3. Add error boundaries around each major component section
    4. Implement graceful fallback UI for missing data (e.g., "Not configured" placeholders)
  - **Files needing review:**
    - All admin pages accessing nested API response objects
    - `src/features/admin/org/OrganizationSettingsPage.tsx` (partially fixed)
    - `src/features/access/hooks/usePermissionGuard.ts` (partially fixed for empty roles)

- [ ] **API Contract Alignment**
  - **Problem:** Frontend expects fields that backend doesn't return
  - **Examples found:**
    - `/organizations/me` doesn't return `billing_contact` or `address` objects
    - `/auth/userinfo` returns empty `roles: []` array
  - **Options:**
    1. Update user-org-service to return complete objects (including empty nested objects)
    2. Update frontend types to mark all nested objects as optional
    3. Create API schema documentation (OpenAPI) and validate both sides
  - **Recommendation:** Option 3 - proper API contracts prevent future drift

- [ ] **Circuit Breaker UX Improvements**
  - **Problem:** When api-router circuit breaker opens, login hangs indefinitely with no feedback
  - **Current behavior:** Loading spinner spins forever, user doesn't know what's wrong
  - **Recommended improvements:**
    1. Add request timeout on frontend (e.g., 10s) with user-friendly error message
    2. Show "Service temporarily unavailable" instead of infinite loading
    3. Add retry button with exponential backoff
    4. Consider adding health status indicator in UI header

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

- [x] **Fix KServe webhook certificate bootstrap issue** (2024-11-30)
  - **Problem:** KServe CRDs failed to sync due to invalid caBundle (`Cg==`) placeholder
  - **Root cause:** ArgoCD applied CRDs before cert-manager created the Certificate resource
  - **Solution:**
    - Added ArgoCD sync-wave annotations (-2 for Issuer, -1 for Certificate, 1 for CRDs, 2 for webhooks)
    - Removed invalid `caBundle: Cg==` placeholders from kserve.yaml
    - cert-manager injects CA automatically via `cert-manager.io/inject-ca-from` annotation
  - **Documentation:**
    - Created `docs/platform/certificate-architecture.md`
    - Created `docs/troubleshooting/kserve-certificate-issues.md`
    - Updated `docs/runbooks/kserve-migration-deployment.md`
    - Updated `infra/k8s/kserve/base/README.md`
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
- **Certificate architecture has two types:**
  - **External TLS** (Let's Encrypt) - for public HTTPS endpoints
  - **Internal webhooks** (self-signed) - for Kubernetes internal communication (KServe, etc.)
- All external TLS certificates managed by cert-manager with Let's Encrypt (auto-renewal)
- KServe internal certificates use self-signed issuer with cert-manager CA injection
