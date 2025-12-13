# Observability Guide

---
last_updated: 2025-12-12
document_type: guide
spec: 024-logging-observability-improvements
---

## Stack Components

The AI-AAS platform uses a comprehensive observability stack deployed in the `system` namespace:

### Logging Stack (Loki + Promtail)

- **Loki** (StatefulSet):
  - Log aggregation and storage with LogQL query interface
  - 14-day retention (development), 30-day retention (production)
  - 50Gi PersistentVolume storage
  - Deployment: `infra/k8s/monitoring/loki/`
  - API: `http://loki.system.svc.cluster.local:3100`

- **Promtail** (DaemonSet):
  - Runs on every node to collect container logs
  - Scrapes from `/var/log/pods` and Kubernetes API
  - Parses JSON logs and extracts labels
  - Special handling for vLLM mixed-format logs (JSON + plain text)
  - Configuration: `infra/k8s/monitoring/promtail/configmap.yaml`

### Tracing Stack (Tempo + OTEL)

- **Tempo** (StatefulSet):
  - Distributed trace storage with 14-day retention
  - OTLP protocol for trace ingestion
  - 10Gi PersistentVolume storage
  - Deployment: `infra/k8s/monitoring/tempo/`
  - API: `http://tempo.system.svc.cluster.local:3100`

- **OTEL Collector** (Deployment):
  - Routes telemetry from services to backends
  - Receivers: OTLP gRPC (4317), OTLP HTTP (4318)
  - Processors: Service graph generation, batching
  - Exporters: Loki (logs), Tempo (traces), Prometheus (metrics)
  - Configuration: `infra/k8s/monitoring/otel-collector.yaml`

### Visualization & Metrics

- **Grafana** (Deployment):
  - Unified dashboard for logs, traces, and metrics
  - Datasources: Loki, Tempo, Prometheus
  - Pre-built dashboards: Inference Backends, Service Logs, Request Tracing
  - Trace-to-logs correlation with derived fields
  - Configuration: `infra/k8s/monitoring/grafana/`
  - Access: `https://grafana.dev.ai-aas.local`

- **Prometheus** (kube-prometheus-stack):
  - Scrapes cluster metrics, Kubernetes components, ingress, services
  - Retention: 15 days (disk provisioned per environment)
  - Service graph metrics from OTEL Collector (port 8889)

### Frontend Observability

- **Sentry** (External SaaS):
  - React error tracking with stack traces
  - Session replay (error-only, 100% sample rate)
  - Performance monitoring (10% sample rate in production)
  - Sensitive data scrubbing (Authorization, passwords, tokens)
  - Integration: `web/portal/src/lib/sentry.ts`
  - ErrorBoundary displays Sentry event IDs to users

- **Frontend Logger**:
  - Structured JSON logging in browser
  - Log levels: debug, info, warn, error
  - Trace correlation via sessionStorage
  - React hook: `useLogger(componentName)`
  - Implementation: `web/portal/src/lib/logger/`

### Alert Routing & Alerting Rules

- **Alertmanager**:
  - Routes alerts to Slack channels, PagerDuty (critical), and email (low priority)
  - Silence windows managed via `maintenance.ai-aas.dev/enabled` annotation or Alertmanager UI
  - Configuration: `infra/k8s/monitoring/alerts/alertmanager-config.yaml`

- **Log-Based Alerts** (PrometheusRule):
  - **Service Errors**: High error rate (>10/min), critical error bursts (fatal/panic)
  - **Loki Health**: Ingestion down, ingestion slowdown, Promtail failures
  - **Storage**: Disk space warnings (80%, 90% thresholds)
  - **vLLM Inference**: GPU errors, model loading failures, OOM events, timeouts
  - **API Issues**: Backend connection failures, database errors
  - **Security**: High authentication failure rates
  - **Crashes**: Service panics, crash loops
  - **Observability**: Low correlation ID coverage (<95%)
  - Alert latency target: <2 minutes from log event to alert state
  - Documentation: `infra/k8s/monitoring/alerts/README.md`

- **Alert Channels**:
  - `severity: critical` → PagerDuty + Slack #platform-critical
  - `category: inference` → Slack #ai-models
  - `category: infrastructure` → Slack #platform-infra
  - `category: security` → Slack #platform-infra

## Data Flow

### Backend Logs
```
[Go Service] → stdout (JSON) → [Promtail] → [Loki] → [Grafana]
```
- Services log structured JSON to stdout/stderr
- Promtail DaemonSet scrapes logs from all pods
- Loki indexes by labels and stores compressed chunks
- Grafana provides LogQL query interface

### Inference Backend Logs
```
[vLLM Pod] → stdout (mixed format) → [Promtail + parsing] → [Loki] → [Grafana Dashboard]
```
- vLLM logs mixed JSON and plain text
- Promtail extracts: model name, GPU errors, loading status
- Dedicated Grafana dashboard for model health monitoring

### Distributed Traces
```
[Service + OTEL SDK] → OTLP spans → [OTEL Collector] → [Tempo] → [Grafana]
```
- Services emit OTLP spans (gRPC port 4317)
- OTEL Collector processes and routes to Tempo
- Service graph processor generates topology metrics
- Grafana provides trace visualization and trace-to-logs linking

### Frontend Errors
```
[React Error] → [ErrorBoundary/captureError] → [Sentry SaaS] → [Alerts]
```
- ErrorBoundary catches React rendering errors
- Sentry SDK captures errors with context and stack traces
- Session replay captures user interactions leading to errors
- Email/Slack alerts on new issues

## Retention Policies

| Environment | Logs (Loki) | Traces (Tempo) | Metrics (Prometheus) | Frontend (Sentry) |
|-------------|-------------|----------------|---------------------|-------------------|
| Development | 14 days | 14 days | 15 days | 90 days (SaaS) |
| Staging | 14 days | 14 days | 15 days | 90 days (SaaS) |
| Production | 30 days (planned) | 30 days (planned) | 30 days | 90 days (SaaS) |

**Rationale**:
- Short retention in dev/staging reduces storage costs
- Production retention extended for incident investigation
- Sentry retention controlled by SaaS tier (free: 90 days)

**Storage Requirements**:
- Loki: ~10 GB/day (dev), ~100 GB/day (prod estimated)
- Tempo: ~2 GB/day (dev), ~20 GB/day (prod estimated)
- Total: 50Gi PV (Loki) + 10Gi PV (Tempo) = 60Gi per environment

## 1.1. Logging Standards

All Go services MUST use the `shared/go/logging` package for consistent structured logging:

### Unified Logging Package

- **Package**: `github.com/ai-aas/shared-go/logging`
- **Backend**: zap (uber-go/zap)
- **Format**: Structured JSON to stdout/stderr
- **Standardized Fields**: `service`, `environment`, `trace_id`, `span_id`, `request_id`, `user_id`, `org_id`

### Usage

```go
import "github.com/ai-aas/shared-go/logging"

// Create logger with config
cfg := logging.DefaultConfig().
    WithServiceName("my-service").
    WithEnvironment("development").
    WithLogLevel("info")

logger := logging.MustNew(cfg)

// Use logger
logger.Info("service started", zap.String("port", "8080"))

// With OpenTelemetry context
loggerWithCtx := logger.WithContext(ctx)
loggerWithCtx.Info("request processed")

// With request/user/org IDs
logger = logger.WithRequestID("req-123")
logger = logger.WithUserID("user-456")
logger = logger.WithOrgID("org-789")
```

### Log Levels

Controlled via `LOG_LEVEL` environment variable:
- `debug`: Verbose debugging information
- `info`: General informational messages (default)
- `warn`: Warning messages
- `error`: Error messages

### Log Redaction

Sensitive data is automatically redacted using patterns from `configs/log-redaction.yaml`:
- Passwords, tokens, API keys
- Connection strings with credentials
- Secrets in environment variables

Use `logging.RedactString()` or `logging.RedactFields()` for manual redaction when needed.

### Local Development

For local development and testing:
- **Loki**: Available at `http://localhost:3100` (via Docker Compose)
- **Promtail**: Collects logs from all containers
- **Access**: Use `make logs-view` or `make logs-tail SERVICE=<name>`

See `usage-guide/operations/log-analysis.md` for detailed log access patterns.

## Required Dashboards

| Dashboard UID | Name | Purpose | Owner |
|---------------|------|---------|-------|
| `env-dev-overview` | Dev Environment Overview | CPU/memory/pod counts, ingress latency, alerts summary | Platform Eng |
| `env-stg-overview` | Staging Overview | Same as above, includes canary error rate | Platform Eng |
| `env-prod-overview` | Production Overview | Business SLOs, infrastructure SLOs, pod disruption budget | SRE On-call |
| `env-sys-overview` | System Ops Overview | GPU utilization, queue lag, controller health | Systems Team |
| `infra-access` | Access & Secrets | Track secret rotations, access package issuance | Security |

Dashboards must include links to Loki queries (`Explore` view) and Tempo traces filtered by `environment`.

## Alert Policies

- **EnvironmentDegraded**: Fires when namespace health check fails (`status != Healthy` for 5 minutes). PagerDuty critical.
- **IngressLatencyHigh**: P95 latency > 300ms for 5 minutes. Slack warning.
- **SecretsRotationDue**: `last_rotation_age > rotation_goal - 3 days`. Slack reminder.
- **DriftDetectedMajor**: Drift detection reports `major`. PagerDuty critical + Slack.
- **NodePoolCapacityLow**: Autoscaler at max capacity for 10 minutes. Slack warning with remediation steps.

Alert definitions codified in `infra/helm/charts/observability-stack/templates/alerts/*.yaml`. Every change requires updating success criteria SC-006/SC-007 verification notes.

## Operational Runbooks

- `docs/runbooks/infrastructure-rollback.md`: Use after alert indicates failed deployment.
- `docs/runbooks/infrastructure-troubleshooting.md`: Map alerts to remediation.
- `docs/platform/access-control.md`: Reference for access-related alerts.

## Verification & Testing

1. Post-provisioning:
   ```bash
   kubectl --context dev-platform -n monitoring get pods
   ./tests/infra/observability_smoke.sh --env development
   ```
2. Synthetic alert test:
   ```bash
   kubectl --context dev-platform \
     -n env-development run latency-generator --image=ghcr.io/postmanlabs/httpbin:latest \
     -- bash -c 'while true; do curl -sSf https://sample.dev.ai-aas.dev/slow; done'
   ```
   - Validate `IngressLatencyHigh` triggers and clears after job deletion.

3. Availability probes:
   ```bash
   go test ./tests/infra/synthetics -run TestControlPlaneAvailability -env development
   ```
   - Same probe runs hourly via `.github/workflows/infra-availability.yml` with GitHub secrets `DEV_KUBECONFIG_B64`/`DEV_KUBE_CONTEXT` (development) and can be extended for production using `PROD_KUBECONFIG_B64`/`PROD_KUBE_CONTEXT`, raising Slack alerts if control plane availability drops below 99.5%.

4. Dashboard snapshots exported weekly via GitHub Actions job `observability-backup`.

## Onboarding Checklist

- [ ] Grafana user added to `platform-observers` team
- [ ] Access package includes dashboard URLs
- [ ] Alert routing documented in PagerDuty service directory
- [ ] Synthetic monitors configured (HTTP uptime per environment)

## Related Documentation

### Architecture & Design
- [Observability Architecture](../architecture/observability-architecture.md) - Detailed architecture, data flows, query patterns
- [Infrastructure Overview](infrastructure-overview.md) - Overall platform architecture

### Operations
- [Environment Access](environment-access.md) - Grafana/Loki/Tempo URLs and credentials
- [Endpoints and URLs](endpoints-and-urls.md) - All service endpoints
- [AI Debugging Workflow](../runbooks/ai-debugging-workflow.md) - Debug with logs and traces

### vLLM-Specific
- [vLLM Observability](vllm-observability.md) - Inference backend monitoring
- [Observability Stack Integration](../monitoring/observability-stack-integration.md) - vLLM metrics integration

### Specifications
- [Spec 024: Logging & Observability](../../specs/024-logging-observability-improvements/spec.md) - Feature specification
- [Spec 024: Architecture](../../specs/024-logging-observability-improvements/architecture.md) - Technical design document

