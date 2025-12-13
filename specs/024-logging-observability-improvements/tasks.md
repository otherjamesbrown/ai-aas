# Tasks: Logging & Observability Improvements

## Phase 1: Infrastructure Foundation

### Task 1.1: Deploy Loki Stack
**Priority:** Critical
**Estimate:** Medium complexity

- [ ] Create `infra/k8s/monitoring/loki/` directory structure
- [ ] Write Loki StatefulSet manifest with persistent storage
- [ ] Write Loki ConfigMap with retention policies
- [ ] Write Loki Service for internal cluster access
- [ ] Write Loki Ingress for external access via nip.io
- [ ] Create ArgoCD Application for Loki
- [ ] Verify Loki is receiving logs via API
- [ ] Verify Loki is accessible via ingress endpoint

**Files to create:**
- `infra/k8s/monitoring/loki/statefulset.yaml`
- `infra/k8s/monitoring/loki/configmap.yaml`
- `infra/k8s/monitoring/loki/service.yaml`
- `infra/k8s/monitoring/loki/ingress.yaml`
- `gitops/clusters/development/apps/loki.yaml`

**Ingress Endpoints:**
- `http://loki.172.232.58.222.nip.io` (primary - no DNS setup needed)
- `http://loki.dev.ai-aas.local` (alternative - requires local DNS)

**Acceptance Criteria:**
- Loki pod is running and healthy
- Can query Loki API at `http://loki.172.232.58.222.nip.io/ready`
- Can query logs via `curl http://loki.172.232.58.222.nip.io/loki/api/v1/query_range?query={level="error"}`
- PersistentVolume is bound and storing data

---

### Task 1.2: Deploy Promtail DaemonSet
**Priority:** Critical
**Estimate:** Medium complexity

- [ ] Create `infra/k8s/monitoring/promtail/` directory structure
- [ ] Write Promtail DaemonSet manifest
- [ ] Write Promtail ConfigMap with scrape configs
- [ ] Configure JSON parsing pipeline for our log format
- [ ] Add label extraction for service, level, trace_id
- [ ] Configure skip paths for health endpoints
- [ ] Create ArgoCD Application for Promtail
- [ ] Verify logs flowing from pods to Loki

**Files to create:**
- `infra/k8s/monitoring/promtail/daemonset.yaml`
- `infra/k8s/monitoring/promtail/configmap.yaml`
- `infra/k8s/monitoring/promtail/serviceaccount.yaml`
- `infra/k8s/monitoring/promtail/clusterrole.yaml`
- `gitops/clusters/development/apps/promtail.yaml`

**Acceptance Criteria:**
- Promtail pods running on all nodes
- Logs visible in Loki from all services
- Labels correctly extracted (service, level, namespace)

---

### Task 1.3: Configure Grafana for Loki
**Priority:** Critical
**Estimate:** Low complexity

- [ ] Add Loki as datasource in Grafana
- [ ] Create Grafana Ingress for external access via nip.io
- [ ] Create basic log exploration dashboard
- [ ] Create service-specific log panels
- [ ] Create error rate visualization
- [ ] Test log queries work end-to-end via ingress

**Files to modify:**
- `infra/k8s/monitoring/grafana/` (datasource config)

**Files to create:**
- `infra/k8s/monitoring/grafana/ingress.yaml`
- `infra/k8s/monitoring/grafana/dashboards/logs-overview.json`

**Ingress Endpoints:**
- `http://grafana.172.232.58.222.nip.io` (primary - no DNS setup needed)
- `http://grafana.dev.ai-aas.local` (alternative - requires local DNS)

**Acceptance Criteria:**
- Can access Grafana at `http://grafana.172.232.58.222.nip.io`
- Loki datasource configured and working
- Can query logs in Grafana Explore
- Dashboard shows log volume by service
- Can filter by log level and service

---

### Task 1.4: Configure vLLM/Inference Backend Log Collection
**Priority:** Critical
**Estimate:** Medium complexity

- [ ] Add Promtail scrape config for `ai-models` namespace
- [ ] Add Promtail scrape config for `system` namespace (vLLM deployments)
- [ ] Add Promtail scrape config for `kserve` namespace
- [ ] Configure pipeline to parse vLLM mixed-format logs (JSON + text)
- [ ] Extract model name from pod labels
- [ ] Extract GPU error patterns (CUDA, OOM)
- [ ] Extract model loading status
- [ ] Create Grafana dashboard for inference backends
- [ ] Test log collection with deployed vLLM model

**Files to modify:**
- `infra/k8s/monitoring/promtail/configmap.yaml`

**Files to create:**
- `infra/k8s/monitoring/grafana/dashboards/inference-backends.json`

**Promtail Labels to Extract:**
- `namespace`: ai-models, system, kserve
- `model`: From pod label `serving.kserve.io/inferenceservice`
- `pod`: Pod name
- `container`: vllm, kserve-container
- `gpu_error`: CUDA/OOM patterns
- `model_status`: Loading/Loaded/Failed

**LogQL Queries for Testing:**
```bash
# All inference backend logs
{namespace=~"ai-models|system|kserve", container=~"vllm|kserve-container"}

# Model loading events
{container=~"vllm|kserve-container"} |~ "(?i)loading model|model loaded|failed to load"

# GPU/CUDA errors
{container=~"vllm|kserve-container"} |~ "(?i)cuda|out.?of.?memory|oom|gpu"

# Specific model logs
{model="gpt-oss-20b"}

# Inference errors
{container=~"vllm|kserve-container", level="error"}
```

**Acceptance Criteria:**
- vLLM logs visible in Loki with correct labels
- Can filter by model name
- GPU errors are labeled and easily queryable
- Model loading status is extractable
- Dashboard shows model health and error rates

---

## Phase 2: Backend Enhancements

### Task 2.1: Create Request Logger Middleware
**Priority:** High
**Estimate:** Medium complexity

- [ ] Create `shared/go/middleware/` package
- [ ] Implement `RequestLogger` middleware
- [ ] Implement `responseWriter` wrapper for status capture
- [ ] Add configurable skip paths (health checks)
- [ ] Add sensitive header filtering
- [ ] Add request duration tracking
- [ ] Write unit tests

**Files to create:**
- `shared/go/middleware/request_logger.go`
- `shared/go/middleware/request_logger_test.go`

**Acceptance Criteria:**
- Middleware compiles and passes tests
- Logs include method, path, status, duration
- Sensitive headers are not logged
- Health check paths are skipped

---

### Task 2.2: Integrate Request Logger into Services
**Priority:** High
**Estimate:** Low complexity per service

- [ ] Add middleware to `api-router-service`
- [ ] Add middleware to `user-org-service`
- [ ] Add middleware to `admin-api-service`
- [ ] Add middleware to `analytics-service`
- [ ] Update go.mod dependencies
- [ ] Test in development environment

**Files to modify:**
- `services/api-router-service/cmd/router/main.go`
- `services/user-org-service/cmd/server/main.go`
- `services/admin-api-service/cmd/server/main.go`
- `services/analytics-service/cmd/server/main.go`

**Acceptance Criteria:**
- All services log requests consistently
- Request IDs correlate across services
- No performance regression (< 1ms overhead)

---

### Task 2.3: Add Pod Annotations for Promtail
**Priority:** High
**Estimate:** Low complexity

- [ ] Add `logging.enabled: "true"` annotation to all service deployments
- [ ] Verify Promtail picks up logs from annotated pods

**Files to modify:**
- `services/api-router-service/deployments/helm/*/values*.yaml`
- `services/user-org-service/deployments/helm/*/values*.yaml`
- `services/admin-api-service/deployments/helm/*/values*.yaml`
- `services/analytics-service/deployments/helm/*/values*.yaml`

**Acceptance Criteria:**
- All service pods have annotation
- Promtail successfully scrapes all services

---

### Task 2.4: Add Log Sampling for Verbose Endpoints
**Priority:** Low
**Estimate:** Low complexity

- [ ] Configure zap sampling in shared logging package
- [ ] Sample 1-in-100 for debug level
- [ ] Sample 1-in-10 for info level on high-traffic paths
- [ ] Never sample warn/error

**Files to modify:**
- `shared/go/logging/logger.go`
- `shared/go/logging/config.go`

**Acceptance Criteria:**
- Log volume reduced for health checks
- All errors still logged
- Sampling configurable per environment

---

## Phase 3: Frontend Enhancements

### Task 3.1: Create Frontend Logger Library
**Priority:** High
**Estimate:** Medium complexity

- [ ] Create `web/portal/src/lib/logger.ts`
- [ ] Implement log levels (debug, info, warn, error)
- [ ] Add structured JSON output
- [ ] Add trace ID correlation
- [ ] Create `useLogger` React hook
- [ ] Add environment-based configuration
- [ ] Write tests

**Files to create:**
- `web/portal/src/lib/logger.ts`
- `web/portal/src/lib/logger.test.ts`

**Acceptance Criteria:**
- Logger respects VITE_LOG_LEVEL
- JSON output in production mode
- Pretty output in development mode
- Trace ID included when available

---

### Task 3.2: Replace Console.log Calls
**Priority:** Medium
**Estimate:** Medium complexity

- [ ] Search for all `console.log` calls
- [ ] Replace with appropriate logger method
- [ ] Search for all `console.error` calls
- [ ] Replace with `logger.error`
- [ ] Search for all `console.warn` calls
- [ ] Replace with `logger.warn`
- [ ] Test in development

**Files to modify:**
- `web/portal/src/providers/TelemetryProvider.tsx`
- `web/portal/src/providers/AuthProvider.tsx`
- `web/portal/src/providers/ToastProvider.tsx`
- `web/portal/src/providers/FeatureFlagProvider.tsx`
- (and any other files with console.* calls)

**Acceptance Criteria:**
- No direct console.* calls except in logger
- All logs include component context
- Log levels appropriate to message type

---

### Task 3.3: Integrate Sentry SDK
**Priority:** High
**Estimate:** Medium complexity

- [ ] Install `@sentry/react` and `@sentry/tracing`
- [ ] Create `web/portal/src/lib/sentry.ts`
- [ ] Initialize Sentry in `main.tsx`
- [ ] Configure environment and DSN from env vars
- [ ] Add sensitive data scrubbing
- [ ] Configure error filtering
- [ ] Enable session replay for errors

**Files to create:**
- `web/portal/src/lib/sentry.ts`

**Files to modify:**
- `web/portal/src/main.tsx`
- `web/portal/.env.example`

**Acceptance Criteria:**
- Sentry initializes without errors
- Test error appears in Sentry dashboard
- Sensitive headers are scrubbed
- Session replay works for errors

---

### Task 3.4: Update ErrorBoundary with Sentry
**Priority:** High
**Estimate:** Low complexity

- [ ] Import Sentry in ErrorBoundary
- [ ] Call `Sentry.captureException` in componentDidCatch
- [ ] Display error ID to users
- [ ] Add component stack context

**Files to modify:**
- `web/portal/src/components/ErrorBoundary.tsx`

**Acceptance Criteria:**
- Errors captured in Sentry with component stack
- Users see error ID for support reference
- Integration tested with forced error

---

### Task 3.5: Add Environment Variables
**Priority:** High
**Estimate:** Low complexity

- [ ] Add VITE_SENTRY_DSN to environment configs
- [ ] Add VITE_LOG_LEVEL to environment configs
- [ ] Update .env.example documentation
- [ ] Configure in Kubernetes ConfigMaps

**Files to modify:**
- `web/portal/.env.example`
- `web/portal/.env.development`
- `services/web-portal/deployments/helm/*/values*.yaml`

**Acceptance Criteria:**
- Environment variables documented
- Development and production configs differ appropriately

---

## Phase 4: Alerting & Dashboards

### Task 4.1: Create Error Alerting Rules
**Priority:** High
**Estimate:** Low complexity

- [ ] Create PrometheusRule for error rate threshold
- [ ] Configure alert routing to notification channel
- [ ] Test alert fires on simulated errors
- [ ] Document alert response procedure

**Files to create:**
- `infra/k8s/monitoring/alerts/logging-alerts.yaml`

**Acceptance Criteria:**
- Alert fires when error rate > 10/min
- Notification received within 5 minutes
- Alert includes service and sample errors

---

### Task 4.2: Create Service Log Dashboard
**Priority:** Medium
**Estimate:** Medium complexity

- [ ] Create Grafana dashboard JSON
- [ ] Add panels: log volume, error rate, latency
- [ ] Add service selector variable
- [ ] Add time range controls
- [ ] Add drill-down links to traces

**Files to create:**
- `infra/k8s/monitoring/grafana/dashboards/service-logs.json`

**Acceptance Criteria:**
- Dashboard loads without errors
- Can filter by service
- Shows meaningful visualizations

---

### Task 4.3: Create Request Tracing Dashboard
**Priority:** Medium
**Estimate:** Medium complexity

- [ ] Create dashboard for request flow visualization
- [ ] Show trace_id correlation across services
- [ ] Add latency breakdown by service
- [ ] Link to Loki logs by trace_id

**Files to create:**
- `infra/k8s/monitoring/grafana/dashboards/request-tracing.json`

**Acceptance Criteria:**
- Can trace request across services
- Latency visible per service hop
- Links to detailed logs work

---

## Phase 5: Documentation & Runbooks

### Task 5.1: Create Debugging Runbook
**Priority:** Medium
**Estimate:** Low complexity

- [ ] Document how to search logs in Grafana
- [ ] Document common log queries
- [ ] Document how to trace a request
- [ ] Document how to find errors by trace_id
- [ ] Document Sentry workflow

**Files to create:**
- `docs/runbooks/debugging-with-logs.md`

**Acceptance Criteria:**
- New team members can follow guide
- Common scenarios covered
- Screenshots included

---

### Task 5.2: Update Architecture Documentation
**Priority:** Low
**Estimate:** Low complexity

- [ ] Update system architecture diagram
- [ ] Document logging data flow
- [ ] Document retention policies
- [ ] Add to developer onboarding docs

**Files to modify:**
- `docs/architecture/` (relevant files)
- `AI_ASSISTANT_GUIDE.md`

**Acceptance Criteria:**
- Documentation accurate and current
- Diagrams reflect actual infrastructure

---

### Task 5.3: Update Technical Documentation
**Priority:** Medium
**Estimate:** Medium complexity

- [ ] Audit existing `/docs` structure for relevant files
- [ ] Update `docs/platform/` with observability stack details
- [ ] Add Loki/Promtail/Grafana to infrastructure documentation
- [ ] Document log format specification and field definitions
- [ ] Add Sentry configuration to frontend documentation
- [ ] Update `docs/platform/environment-access.md` with:
  - Loki API endpoint
  - Grafana dashboard URLs
  - Sentry project access
- [ ] Create `docs/platform/observability.md` with full stack overview
- [ ] Update `CLAUDE.md` with logging-related instructions

**Files to create:**
- `docs/platform/observability.md`

**Files to modify:**
- `docs/platform/environment-access.md`
- `docs/architecture/system-overview.md` (if exists)
- `CLAUDE.md`
- `AI_ASSISTANT_GUIDE.md`

**Acceptance Criteria:**
- All observability components documented
- Access credentials and URLs documented
- Log format and fields defined
- Cross-references between docs are accurate

---

### Task 5.4: Create AI Coding Assistant Debugging Guide
**Priority:** High
**Estimate:** Medium complexity

Create comprehensive documentation for AI coding assistants (Claude Code, Cursor, etc.) to effectively use the logging infrastructure for debugging.

- [ ] Create `docs/ai-assistant/debugging-with-logs.md`
- [ ] Document the deploy → test → debug workflow
- [ ] Provide LogCLI commands for common queries
- [ ] Provide kubectl commands for log access
- [ ] Document how to correlate errors across services
- [ ] Add example debugging sessions with sample output
- [ ] Update `CLAUDE.md` with debugging instructions
- [ ] Update `AI_ASSISTANT_GUIDE.md` with observability section

**Files to create:**
- `docs/ai-assistant/debugging-with-logs.md`

**Files to modify:**
- `CLAUDE.md`
- `AI_ASSISTANT_GUIDE.md`

**Guide Content Structure:**

```markdown
# AI Assistant Debugging Guide

## Environment Setup

All commands use the remote development cluster via kubeconfig:

# Set alias for convenience (optional)
alias k='kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml'

# Or use full path each time
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml

## Monitoring Endpoints (No Port-Forwarding Required)

| Service | Endpoint | Purpose |
|---------|----------|---------|
| Loki | http://loki.172.232.58.222.nip.io | Log aggregation API |
| Grafana | http://grafana.172.232.58.222.nip.io | Dashboard & log exploration |

## Quick Reference Commands

### Check Pod Logs (Real-time via kubectl)
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs -f deployment/<service> -n default --tail=100

# Examples for specific services:
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs -f deployment/api-router-service -n default --tail=100

kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs -f deployment/user-org-service -n default --tail=100

### Query Loki via HTTP Endpoint (Recommended)

# Find all errors in last 15 minutes
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={level="error"}' \
  --data-urlencode 'limit=50' \
  --data-urlencode 'since=15m' | jq '.data.result'

# Find logs for specific service
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service"}' \
  --data-urlencode 'limit=100' | jq '.data.result'

# Find logs containing request_id
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service"} |= "<request-id>"' \
  --data-urlencode 'limit=50' | jq '.data.result'

# Find errors with stack traces
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={level="error"} | json | line_format "{{.msg}} {{.error}}"' \
  --data-urlencode 'limit=20' | jq -r '.data.result[].values[][1]'

### Time-Based Queries (Specific Time Windows)

# Query a specific time window (ISO8601 format, nanoseconds)
# Use: date -d "2024-01-15T10:30:00Z" +%s to convert to unix timestamp
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={level="error"}' \
  --data-urlencode 'start=1705314600000000000' \
  --data-urlencode 'end=1705315500000000000' \
  --data-urlencode 'limit=100' | jq '.data.result'

# Last N minutes/hours/days
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service"}' \
  --data-urlencode 'since=1h' \       # 1 hour
  --data-urlencode 'limit=200' | jq '.data.result'

# Helper: Convert ISO8601 to nanoseconds for Loki
# date -d "2024-01-15T10:30:00Z" +%s%N

### Aggregation Queries (Metrics from Logs)

# Count errors by service in last hour
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query' \
  --data-urlencode 'query=sum by(service) (count_over_time({level="error"}[1h]))' \
  | jq '.data.result'

# Error rate over time (for graphing)
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query=sum(rate({level="error"}[5m]))' \
  --data-urlencode 'since=1h' \
  --data-urlencode 'step=60' | jq '.data.result'

# Top error messages
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query' \
  --data-urlencode 'query=topk(10, sum by(msg) (count_over_time({level="error"} | json [1h])))' \
  | jq '.data.result'

# Log volume by service
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query' \
  --data-urlencode 'query=sum by(service) (count_over_time({}[1h]))' \
  | jq '.data.result'

### Inference Backend / vLLM Log Queries

# All vLLM logs
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"}' \
  --data-urlencode 'limit=100' | jq '.data.result'

# Model loading events
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)loading model|model loaded|failed to load"' \
  --data-urlencode 'limit=50' | jq '.data.result'

# GPU/CUDA errors (CRITICAL)
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)cuda|out.?of.?memory|oom|gpu"' \
  --data-urlencode 'limit=50' | jq '.data.result'

# Specific model logs
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={model="gpt-oss-20b"}' \
  --data-urlencode 'limit=100' | jq '.data.result'

# vLLM errors only
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)error|exception|failed|traceback"' \
  --data-urlencode 'limit=50' | jq '.data.result'

# AsyncEngineDeadError (vLLM crash)
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "AsyncEngineDeadError"' \
  --data-urlencode 'limit=20' | jq '.data.result'

### Quick method - tail recent pod logs and grep (fallback)
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs deployment/api-router-service -n default --tail=500 | grep -i error

# vLLM pod logs directly (when Loki unavailable)
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs -l app=vllm -n system --tail=200 | grep -i "error\|cuda\|oom"

## Debug Workflow

### 1. Deploy Code Change
git add . && git commit -m "fix: description" && git push
# ArgoCD syncs automatically for development
# Or check sync status:
# Open https://argocd.dev.ai-aas.local or use CLI

### 2. Run Tests
# Integration tests
make test-integration

# Or specific service tests
cd services/<service> && go test ./...

### 3. On Test Failure - Query Logs

#### Find errors in the last 15 minutes (all services)
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={level="error"}' \
  --data-urlencode 'limit=100' \
  --data-urlencode 'since=15m' | jq '.data.result'

#### Find logs for specific service with errors
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service", level=~"error|warn"}' \
  --data-urlencode 'limit=100' | jq '.data.result'

#### Search logs by request_id
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={} |= "<request-id>"' \
  --data-urlencode 'limit=50' | jq '.data.result'

### 4. Analyze and Fix
- Look for error messages and stack traces in JSON logs
- Parse JSON with jq: `... | jq -r '.data.result[].values[][1]'`
- Check request_id correlation across services
- Review the sequence of events leading to failure
- Check for panic/fatal logs

## Common Debugging Scenarios

### API Returns 500
1. Get request_id from response headers
2. Query Loki for that request_id across all services:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={} |= "<request_id>"' \
     --data-urlencode 'limit=50' | jq '.data.result'
3. Check downstream services with same request_id

### Test Timeout
1. Query service logs during test window:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={service="<service>"}' \
     --data-urlencode 'since=5m' \
     --data-urlencode 'limit=200' | jq '.data.result'
2. Look for slow operations in duration fields
3. Check for connection errors or retries

### Authentication Failure
1. Check user-org-service logs:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={service="user-org-service"} |= "auth"' \
     --data-urlencode 'limit=100' | jq '.data.result'
2. Check for token validation errors
3. Verify user/org context in logs

### Pod Crash/Restart
1. Check pod status:
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     get pods -n default
2. Check previous container logs:
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     logs deployment/<service> -n default --previous
3. Check events:
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     describe pod <pod-name> -n default

### Model Loading Failure (vLLM/Inference)
1. Check model pod status:
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     get pods -n system -l app=vllm
2. Query Loki for model loading events:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)loading|failed|error"' \
     --data-urlencode 'since=30m' | jq '.data.result'
3. Check for GPU/CUDA issues:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)cuda|oom|gpu|memory"' \
     --data-urlencode 'since=30m' | jq '.data.result'
4. Check pod events for resource issues:
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     describe pod -n system -l app=vllm | grep -A5 Events

### Inference Returns Error / Slow Response
1. Get request_id from API response
2. Trace through api-router to vLLM:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={} |= "<request_id>"' \
     --data-urlencode 'limit=50' | jq '.data.result'
3. Check vLLM queue/inference timing:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={container=~"vllm|kserve-container"} | json | duration_ms > 5000' \
     --data-urlencode 'since=15m' | jq '.data.result'
4. Check for OOM during inference:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "OutOfMemory"' \
     --data-urlencode 'since=15m' | jq '.data.result'

### GPU Memory Exhaustion
1. Check current GPU memory:
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     exec -n system deployment/vllm -- nvidia-smi
2. Query for OOM events:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)out.?of.?memory|oom|cuda.*memory"' \
     --data-urlencode 'since=1h' | jq '.data.result'
3. Check for KV cache warnings:
   curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
     --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "KV cache"' \
     --data-urlencode 'since=1h' | jq '.data.result'

## Grafana Dashboard Access (Visual Log Exploration)
# Open directly in browser - no port-forward needed:
http://grafana.172.232.58.222.nip.io

# Navigate to Explore > Select Loki datasource
# Use LogQL queries like: {service="api-router-service", level="error"}
```

**Acceptance Criteria:**
- AI assistant can follow guide to debug without human help
- All common scenarios covered with exact commands
- Commands work with current infrastructure
- Examples include sample output for context
- Guide is referenced in CLAUDE.md for automatic discovery

---

### Task 5.5: Add Observability Section to CLAUDE.md
**Priority:** High
**Estimate:** Low complexity

Update `CLAUDE.md` to include a dedicated section for observability and debugging that AI assistants will automatically read.

- [ ] Add "Debugging & Observability" section to CLAUDE.md
- [ ] Include quick reference commands
- [ ] Reference the full debugging guide
- [ ] Add common LogQL queries
- [ ] Document log format expectations

**Files to modify:**
- `CLAUDE.md`

**Content to add:**

```markdown
## Debugging & Observability

When debugging issues or investigating test failures, use the logging infrastructure on the remote development cluster.

### Monitoring Endpoints (No Port-Forwarding Required)

| Service | Endpoint | Purpose |
|---------|----------|---------|
| Loki | http://loki.172.232.58.222.nip.io | Log aggregation API |
| Grafana | http://grafana.172.232.58.222.nip.io | Dashboard & log exploration |

### Quick Log Access

# Real-time pod logs for a service
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs -f deployment/api-router-service -n default --tail=100

# Query Loki for errors (recommended)
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={level="error"}' \
  --data-urlencode 'limit=50' | jq '.data.result'

# Query specific service
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service"}' \
  --data-urlencode 'limit=100' | jq '.data.result'

# Search by request_id
curl -s 'http://loki.172.232.58.222.nip.io/loki/api/v1/query_range' \
  --data-urlencode 'query={} |= "<request-id>"' \
  --data-urlencode 'limit=50' | jq '.data.result'

### Log Format
All services output JSON logs with these fields:
- `level`: debug|info|warn|error
- `service`: service name
- `trace_id`: distributed trace correlation
- `request_id`: per-request correlation
- `msg`: log message
- `error`: error details (when applicable)

### Debug Workflow
1. Deploy: `git push` → ArgoCD syncs automatically (dev)
2. Test: `make test-integration` or service-specific tests
3. On failure: Query Loki via curl or open Grafana
4. Parse JSON output with jq
5. Correlate across services using request_id
6. Fix and repeat

📖 Full guide: `docs/ai-assistant/debugging-with-logs.md`
```

**Acceptance Criteria:**
- CLAUDE.md contains actionable debugging commands
- AI assistants automatically have access to debugging workflow
- Commands use correct kubeconfig paths from environment-access.md

---

## Summary

| Phase | Tasks | Priority | Dependencies |
|-------|-------|----------|--------------|
| Phase 1 | 1.1, 1.2, 1.3, 1.4 | Critical | None |
| Phase 2 | 2.1, 2.2, 2.3, 2.4 | High | Phase 1 |
| Phase 3 | 3.1, 3.2, 3.3, 3.4, 3.5 | High | None (parallel) |
| Phase 4 | 4.1, 4.2, 4.3 | Medium | Phase 1, 2 |
| Phase 5 | 5.1, 5.2, 5.3, 5.4, 5.5 | Medium-High | Phase 1-4 |

### Task Count: 21 tasks across 5 phases

**Recommended Order:**
1. Phase 1 (Infrastructure) - unblocks everything
2. Phase 3 (Frontend) - can run parallel with Phase 2
3. Phase 2 (Backend) - depends on Phase 1
4. Phase 4 (Alerting) - depends on Phase 1, 2
5. Phase 5 (Docs) - after implementation complete
   - **Task 5.4 and 5.5 are HIGH priority** - critical for AI assistant debugging workflow

**AI Assistant Debugging Workflow (enabled by Tasks 5.4 & 5.5):**
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Edit Code  │ ──► │ Git Push    │ ──► │ Run Tests   │ ──► │ Query Logs  │
└─────────────┘     └─────────────┘     └─────────────┘     └──────┬──────┘
                                                                   │
                                              ┌────────────────────┘
                                              ▼
                                        ┌─────────────┐
                                        │ Analyze &   │
                                        │ Fix Issue   │
                                        └─────────────┘
```
