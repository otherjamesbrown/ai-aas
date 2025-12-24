# Observability Architecture

---
last_updated: 2025-12-12
document_type: architecture
spec: 024-logging-observability-improvements
---

## Overview

This document describes the unified observability architecture for the AI-AAS platform, including logging, tracing, error tracking, and visualization components deployed as part of spec 024.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          Kubernetes Cluster                                     │
│                                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  api-router  │  │  user-org    │  │  analytics   │  │  admin-api   │       │
│  │   service    │  │   service    │  │   service    │  │   service    │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                 │                 │                 │               │
│         └─────────────────┴─────────────────┴─────────────────┘               │
│                           │                                                    │
│                           │ stdout/stderr (JSON logs)                          │
│                           │                                                    │
│         ┌─────────────────┴────────────────────────────────────┐               │
│         │                                                      │               │
│         ▼                                                      ▼               │
│  ┌────────────────────────────┐                    ┌────────────────────────┐  │
│  │  Promtail (DaemonSet)      │                    │   OTEL Collector       │  │
│  │  - Tails container logs    │                    │   - OTLP receiver      │  │
│  │  - Parses JSON             │                    │   - Service graph gen  │  │
│  │  - Extracts labels         │                    │                        │  │
│  └─────────┬──────────────────┘                    └───────┬────────────────┘  │
│            │                                               │                   │
│            │                                               │                   │
│            ▼                                               ▼                   │
│  ┌────────────────────────────┐                    ┌────────────────────────┐  │
│  │     Loki (StatefulSet)     │                    │   Tempo (StatefulSet)  │  │
│  │  - Log aggregation         │                    │   - Trace storage      │  │
│  │  - 14-day retention        │                    │   - 14-day retention   │  │
│  │  - PersistentVolume: 50Gi  │                    │   - PersistentVolume   │  │
│  │  - LogQL query interface   │                    │   - Trace search       │  │
│  └─────────┬──────────────────┘                    └───────┬────────────────┘  │
│            │                                               │                   │
│            └───────────────────┬───────────────────────────┘                   │
│                                │                                               │
│                                ▼                                               │
│                    ┌────────────────────────────┐                              │
│                    │    Grafana (Deployment)    │                              │
│                    │  - Loki datasource         │                              │
│                    │  - Tempo datasource        │                              │
│                    │  - Trace-to-logs linking   │                              │
│                    │  - Dashboards:             │                              │
│                    │    * Inference Backends    │                              │
│                    │    * Service Logs          │                              │
│                    │    * Request Tracing       │                              │
│                    └────────────────────────────┘                              │
│                                                                                 │
│  ┌──────────────────────────────────────────────────────────────────────────┐  │
│  │                    Inference Backend Pods (ai-models, system, kserve)    │  │
│  │  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐            │  │
│  │  │  vLLM GPT-OSS  │  │  vLLM Llama-2  │  │  vLLM Mistral  │            │  │
│  │  │  - Mixed logs  │  │  - JSON logs   │  │  - Text logs   │            │  │
│  │  │  - GPU errors  │  │  - Loading     │  │  - CUDA info   │            │  │
│  │  └────────┬───────┘  └────────┬───────┘  └────────┬───────┘            │  │
│  │           │                   │                   │                      │  │
│  │           └───────────────────┴───────────────────┘                      │  │
│  │                               │                                          │  │
│  │                               │ (scraped by Promtail)                    │  │
│  │                               ▼                                          │  │
│  │                      [Logs flow to Loki]                                 │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      │ (frontend errors)
                                      │
                    ┌─────────────────▼────────────────┐
                    │         Web Portal (Browser)     │
                    │  ┌────────────────────────────┐  │
                    │  │  Logger Library            │  │
                    │  │  - Structured JSON         │  │
                    │  │  - Trace correlation       │  │
                    │  │  - Log levels (debug/info) │  │
                    │  └────────────────────────────┘  │
                    │  ┌────────────────────────────┐  │
                    │  │  Sentry Integration        │  │
                    │  │  - Error capture           │  │
                    │  │  - Session replay          │  │
                    │  │  - Breadcrumb tracking     │  │
                    │  │  - Sensitive data scrub    │  │
                    │  └────────────────────────────┘  │
                    └─────────────────┬────────────────┘
                                      │
                                      ▼
                              ┌─────────────────┐
                              │  Sentry (SaaS)  │
                              │  - External     │
                              │  - Error alerts │
                              └─────────────────┘
```

## Component Architecture

### 1. Log Collection Layer

#### Promtail (DaemonSet)
- **Purpose**: Collects logs from all pods and forwards to Loki
- **Deployment**: DaemonSet (one pod per node)
- **Configuration**: `/home/dev/ai-aas-024-observability/infra/k8s/monitoring/promtail/configmap.yaml`
- **Scrape Jobs**:
  - `kubernetes-pods`: General service logs (api-router, user-org, analytics, admin-api)
  - `kubernetes-inference-backends`: vLLM/inference backend logs (ai-models, system, kserve namespaces)
- **Features**:
  - JSON log parsing with automatic field extraction
  - Mixed-format log handling (JSON + plain text for vLLM)
  - Label extraction from Kubernetes metadata
  - GPU error detection and labeling
  - Model loading status extraction

#### Log Sources
- **Go Services**: Structured JSON logs via Zap logger
  - Fields: `timestamp`, `level`, `service`, `trace_id`, `span_id`, `request_id`, `msg`
  - Middleware: Request/response logging with error body capture
  - Sensitive data: Automatically redacted

- **vLLM Pods**: Mixed-format logs
  - JSON logs: `{"timestamp": "...", "level": "info", "message": "..."}`
  - Plain text: Model loading, GPU info, CUDA errors
  - Extracted fields: `model_name`, `gpu_id`, `loading_status`, `gpu_error`

### 2. Log Storage Layer

#### Loki (StatefulSet)
- **Purpose**: Scalable log aggregation and storage
- **Configuration**: `/home/dev/ai-aas-024-observability/infra/k8s/monitoring/loki/statefulset.yaml`
- **Storage**: 50Gi PersistentVolume per replica
- **Retention**: 14 days (development), 30 days (production)
- **Query Interface**: LogQL (Prometheus-like query language)
- **Ingress**: `loki.dev.otherjamesbrown.com` (internal access)
- **API Endpoint**: `/loki/api/v1/query`, `/loki/api/v1/query_range`

**Index Strategy**:
- Label-based indexing (not full-text)
- Indexed labels: `namespace`, `pod`, `container`, `service`, `level`, `model`, `environment`
- Cardinality control: Limited label set to avoid index explosion

**Storage Layout**:
```
/loki/
├── chunks/          # Compressed log chunks
├── index/           # Index for label queries
└── wal/            # Write-ahead log
```

### 3. Distributed Tracing Layer

#### Tempo (StatefulSet)
- **Purpose**: Distributed trace storage and querying
- **Configuration**: `/home/dev/ai-aas-024-observability/infra/k8s/monitoring/tempo/statefulset.yaml`
- **Storage**: 10Gi PersistentVolume
- **Retention**: 14 days
- **Protocol**: OTLP (OpenTelemetry Protocol)
- **Query Interface**: Tempo Search API

#### OTEL Collector (Deployment)
- **Purpose**: Telemetry routing and processing
- **Configuration**: `/home/dev/ai-aas-024-observability/infra/k8s/monitoring/otel-collector.yaml`
- **Receivers**:
  - OTLP (gRPC: 4317, HTTP: 4318) - for traces and logs
- **Processors**:
  - Service graph: Generates topology metrics from traces
  - Batch: Batches telemetry for efficiency
- **Exporters**:
  - Loki: OTLP logs → Loki
  - Tempo: OTLP traces → Tempo
  - Prometheus: Service graph metrics (port 8889)

### 4. Visualization Layer

#### Grafana (kube-prometheus-stack)
- **Purpose**: Unified visualization and exploration
- **Deployment**: Managed by kube-prometheus-stack Helm chart in `monitoring` namespace
- **Dashboards**: Deployed via ConfigMaps with `grafana_dashboard: "1"` label (sidecar auto-discovery)
  - Dashboard definitions: `infra/k8s/monitoring/dashboards/`
  - ArgoCD app: `monitoring-dashboards-development`
- **Datasources**:
  - **Loki**: Log queries via LogQL
  - **Tempo**: Trace exploration
  - **Prometheus**: Metrics (existing)
- **Pre-built Dashboards**:
  - `gpu-fleet.json`: GPU inventory, utilization, health
  - `kubernetes-resources.json`: Node/pod status, CPU/memory
  - `api-performance-v2.json`: Request rate, latency, errors by org
  - `inference-performance.json`: Model latency, throughput, KV cache
  - `inference-engine.json`: vLLM engine metrics
  - `org-usage.json`: Organization token consumption
  - `cost-efficiency.json`: Cost tracking, efficiency metrics
  - `platform-overview.json`: Health score, API success
- **Features**:
  - Trace-to-logs correlation: Click trace → view related logs
  - Derived fields: Extract `trace_id` from logs → link to Tempo
  - Node graph: Service topology from OTEL service graph

### 5. Frontend Observability

#### Logger Library
- **Location**: `/home/dev/ai-aas-024-observability/web/portal/src/lib/logger/index.ts`
- **Features**:
  - Log levels: `debug`, `info`, `warn`, `error`
  - Structured JSON output in production
  - Trace correlation: Reads `trace_id` from `sessionStorage`
  - Context management: Component name, user ID, org ID
  - Environment-based configuration
  - Remote logging: Buffers logs, flushes on error or interval
- **React Hook**: `useLogger(componentName)` for automatic context

#### Sentry Integration
- **Location**: `/home/dev/ai-aas-024-observability/web/portal/src/lib/sentry.ts`
- **SDK Version**: @sentry/react 7.99.0
- **Features**:
  - Error capture with stack traces
  - Session replay (error-only, 100% sample rate)
  - Performance monitoring (10% sample rate in production)
  - Sensitive data scrubbing (Authorization, passwords, tokens)
  - Request header/body sanitization
  - User context tracking
- **ErrorBoundary Integration**: Displays Sentry event ID to users
- **Configuration**: `VITE_SENTRY_DSN` environment variable

## Data Flow

### Backend Log Flow
```
[Go Service]
  → stdout/stderr (JSON structured logs)
  → [Promtail] (DaemonSet scrapes logs)
  → [Loki] (indexes by labels, stores chunks)
  → [Grafana] (LogQL queries for visualization)
```

### Inference Backend Log Flow
```
[vLLM Pod]
  → stdout/stderr (mixed JSON + plain text)
  → [Promtail] (regex parsing, label extraction)
  → [Loki] (labels: model, gpu_error, loading_status)
  → [Grafana] (Inference Backends dashboard)
```

### Trace Flow
```
[Go Service with OTEL SDK]
  → OTLP spans (gRPC to OTEL Collector:4317)
  → [OTEL Collector] (service graph processor)
  → [Tempo] (trace storage)
  → [Grafana] (trace visualization, trace-to-logs)
```

### Frontend Error Flow
```
[React App Error]
  → [ErrorBoundary] (catches React errors)
  → [Sentry.captureError()] (with context + stack)
  → [Sentry SaaS] (external error tracking)
  → [Email/Slack alerts] (on new issues)
```

## Storage and Retention

| Component | Storage Type | Size (Dev) | Retention | Backup |
|-----------|-------------|------------|-----------|--------|
| Loki | PersistentVolume | 50Gi | 14 days | None (ephemeral) |
| Tempo | PersistentVolume | 10Gi | 14 days | None (ephemeral) |
| Grafana | PersistentVolume | 10Gi | N/A (dashboards) | GitOps (ConfigMap) |
| Sentry | External SaaS | N/A | 90 days (free tier) | N/A |

**Retention Policies**:
- Development: 14 days (optimize for cost)
- Staging: 14 days
- Production: 30 days (planned, not yet deployed)

**Cardinality Limits**:
- Loki label cardinality: < 10,000 unique label combinations
- Promtail drops high-cardinality labels (e.g., request IDs in labels)
- Log volume limit: 10MB/s per service

## Query Patterns

### LogQL Examples

**Find errors across all services:**
```logql
{namespace="system"} | json | level="error"
```

**Search by trace ID:**
```logql
{namespace="system"} | json | trace_id="abc123"
```

**vLLM GPU errors:**
```logql
{namespace=~"ai-models|system|kserve"} | json | gpu_error="true"
```

**Model loading events:**
```logql
{namespace=~"ai-models|system|kserve"} | json | loading_status=~"loading|ready|failed"
```

**Log volume by service (rate):**
```logql
sum(rate({namespace="system"}[5m])) by (service)
```

### Trace Queries

**Find traces with errors:**
- Grafana Explore → Tempo → Search → Status: `error`

**Trace by ID:**
- Grafana Explore → Tempo → Query: `<trace_id>`

**Trace to logs:**
- Click on span → "Logs for this span" → Opens Loki with `trace_id` filter

## Access and Endpoints

### Internal Cluster Access

| Service | Endpoint | Port | Protocol |
|---------|----------|------|----------|
| Loki | `loki.system.svc.cluster.local` | 3100 | HTTP |
| Tempo | `tempo.system.svc.cluster.local` | 3100 | HTTP |
| OTEL Collector | `otel-collector.system.svc.cluster.local` | 4317 | gRPC |
| OTEL Collector | `otel-collector.system.svc.cluster.local` | 4318 | HTTP |
| Grafana | `kube-prometheus-stack-grafana.monitoring.svc.cluster.local` | 80 | HTTP |

### External Access (Development)

| Service | Ingress URL | Authentication |
|---------|-------------|----------------|
| Grafana | `https://grafana.dev.otherjamesbrown.com` | Admin credentials |
| Loki | `https://loki.dev.otherjamesbrown.com` | None (internal only) |

See [Environment Access](../platform/environment-access.md) for credentials.

## Performance Considerations

### Log Volume Management

**Strategies**:
1. **Log Sampling**: Health check endpoints logged at 1% rate
2. **Redaction**: Sensitive data automatically removed
3. **Retention**: 14-day limit to control storage costs
4. **Cardinality**: Limited label set (no high-cardinality labels like request IDs)

**Expected Volume**:
- Development: ~10 GB/day
- Production: ~100 GB/day (estimated)

### Query Performance

**Optimizations**:
- Loki queries filtered by time range and namespace
- Indexed labels used for filtering (avoid full-text search)
- Grafana dashboards use query caching
- Dashboard load time: < 5 seconds (success criteria)

## Monitoring and Alerting

### Key Metrics

**Loki Health**:
- `loki_ingester_streams`: Number of active streams
- `loki_distributor_bytes_received_total`: Incoming log volume
- `loki_query_duration_seconds`: Query performance

**Tempo Health**:
- `tempo_ingester_traces_created_total`: Traces ingested
- `tempo_query_frontend_queries_total`: Query volume

**OTEL Collector**:
- `otelcol_receiver_accepted_spans`: Spans received
- `otelcol_exporter_sent_spans`: Spans exported

### Alert Rules (Planned - Phase 4)

| Alert | Condition | Severity | Destination |
|-------|-----------|----------|-------------|
| HighErrorRate | Error logs > 10/min | Warning | Slack |
| LogIngestionDown | No logs for 5 min | Critical | PagerDuty |
| DiskSpaceAlert | Loki PV > 80% | Warning | Slack |
| vLLMGPUError | GPU error detected | Critical | Slack + Email |

## Security Considerations

### Sensitive Data Protection

**Backend Logs**:
- Automatic redaction via `shared/go/logging` package
- Patterns: passwords, tokens, API keys, connection strings
- Regex-based filtering in Promtail (additional layer)

**Frontend Logs**:
- Sentry scrubs: `Authorization` headers, API keys, passwords, tokens
- Request body sanitization: password fields removed
- Session replay: Text input masking enabled

### Access Control

- **Grafana**: Admin-only access (no anonymous)
- **Loki API**: Internal cluster access only (no ingress)
- **Tempo API**: Internal cluster access only
- **OTEL Collector**: Internal receivers only (not exposed externally)

## GitOps Deployment

### ArgoCD Applications

| Application | Path | Namespace | Branch |
|-------------|------|-----------|--------|
| loki | `infra/k8s/monitoring/loki` | system | develop |
| promtail | `infra/k8s/monitoring/promtail` | system | develop |
| tempo | `infra/k8s/monitoring/tempo` | system | develop |
| otel-collector | `infra/k8s/monitoring/otel-collector.yaml` | system | develop |
| monitoring-dashboards | `infra/k8s/monitoring/dashboards` | monitoring | develop |
| monitoring-ingresses | `infra/k8s/monitoring/ingresses` | monitoring | develop |

**Note**: Grafana is deployed as part of `kube-prometheus-stack` Helm chart, not as a standalone application. Dashboards are automatically loaded via ConfigMap sidecar.

### Deployment Workflow

1. Edit Kubernetes manifests in `infra/k8s/monitoring/`
2. Commit and push to `develop` branch
3. ArgoCD auto-syncs to development cluster
4. Verify deployment: `kubectl get pods -n system`
5. Promote to `staging` branch via PR
6. Promote to `main` (production) via PR

See [GitOps Deployment Workflow](../runbooks/deploy-to-environments.md).

## Developer Onboarding

### For Backend Developers

**Required Knowledge**:
1. **Logging**: Use `shared/go/logging` package for all log statements
2. **Trace Correlation**: Ensure OpenTelemetry context propagation
3. **Query Logs**: Learn basic LogQL for debugging
4. **Dashboards**: Access Grafana for service-specific dashboards

**Quick Start**:
```bash
# View logs for your service
kubectl logs -n system -l app=my-service -f

# Access Grafana
open https://grafana.dev.otherjamesbrown.com

# Query logs in Grafana
{namespace="system", service="my-service"} | json | level="error"
```

### For Frontend Developers

**Required Knowledge**:
1. **Logger**: Import and use `@/lib/logger` instead of `console.log`
2. **Sentry**: Errors automatically captured, include context
3. **Trace IDs**: Automatically correlated via sessionStorage

**Quick Start**:
```typescript
import { useLogger } from '@/lib/logger';

function MyComponent() {
  const logger = useLogger('MyComponent');

  logger.info('Component mounted', { foo: 'bar' });
  logger.error('API call failed', { error: err });
}
```

### For AI Assistants (Claude)

See [AI Debugging Workflow](../runbooks/ai-debugging-workflow.md) for:
- LogQL commands for common scenarios
- kubectl commands for log access
- Debug workflow (find logs → identify trace → correlate services)
- Common troubleshooting patterns

## Migration Notes

### From Console Logs

**Before (Deprecated)**:
```typescript
console.log('User logged in:', userId);
```

**After (Preferred)**:
```typescript
logger.info('User logged in', { userId });
```

### From Direct kubectl Logs

**Before**:
```bash
kubectl logs -n system deployment/api-router --tail=100
```

**After**:
- Use Grafana for persistent log search
- Use LogQL for filtering and aggregation
- kubectl logs still available for real-time tailing

## Related Documentation

### Platform Docs
- [Infrastructure Overview](../platform/infrastructure-overview.md) - Overall architecture
- [Observability Guide](../platform/observability-guide.md) - Monitoring and alerting
- [Environment Access](../platform/environment-access.md) - Credentials and endpoints
- [vLLM Observability](../platform/vllm-observability.md) - Inference backend monitoring

### Runbooks
- [AI Debugging Workflow](../runbooks/ai-debugging-workflow.md) - Debug with logs
- [Deploy to Environments](../runbooks/deploy-to-environments.md) - GitOps workflow
- [Infrastructure Troubleshooting](../runbooks/infrastructure-troubleshooting.md) - Issue resolution

### Specs
- [Spec 024](../../specs/024-logging-observability-improvements/spec.md) - Feature specification
- [Architecture](../../specs/024-logging-observability-improvements/architecture.md) - Technical design
- [Tasks](../../specs/024-logging-observability-improvements/tasks.md) - Implementation tasks

## Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2025-12-12 | 1.0 | Initial architecture documentation for spec 024 |
