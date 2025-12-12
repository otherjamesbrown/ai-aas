# Implementation Plan: Logging & Observability Improvements

**Branch**: `024-logging-observability-improvements` | **Date**: 2025-12-12 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/024-logging-observability-improvements/spec.md`

## Summary

Deploy a unified logging and observability infrastructure for the AI-AAS platform using Loki/Promtail/Grafana for log aggregation, Sentry for frontend error tracking, and enhanced middleware for request/response logging. Critical addition: inference backend (vLLM) log collection for debugging model deployments and GPU issues.

## Technical Context

**Language/Version**: Go 1.21+ (backend middleware), TypeScript 5.x (frontend logger)
**Primary Dependencies**: Loki 2.9+, Promtail 2.9+, Grafana 10+, Sentry SDK, Zap (logging), OpenTelemetry
**Storage**: Loki (log storage with retention policies), PersistentVolumes for Loki data
**Testing**: Go unit tests (middleware), integration tests (log pipeline), E2E (alert firing)
**Target Platform**: Kubernetes (LKE) with GPU nodes for inference
**Project Type**: Infrastructure deployment + shared library additions
**Performance Goals**: Dashboard load <5s, alerts fire within 2min, 95% requests have correlation IDs
**Constraints**: 14-day retention (dev), 30-day retention (prod), cardinality controls for labels
**Scale/Scope**: All platform services (4 Go services) + inference backends (vLLM pods)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Status | Evidence |
|------|--------|----------|
| API-First | PASS | Loki exposes REST API (`/loki/api/v1/query`), Grafana has HTTP API, all queryable via curl |
| Statelessness | PASS | Promtail is stateless DaemonSet; Loki uses external storage (PV); no in-memory state |
| Async Non-Critical | PASS | Logging is explicitly off critical path per spec; log shipping is fire-and-forget |
| Security | PASS | Sensitive headers/fields redacted in middleware; no secrets in logs; ingress with TLS |
| GitOps/Declarative | PASS | All components deployed via Helm charts + ArgoCD Applications |
| Observability | PASS | This IS the observability implementation; adds logs/metrics/traces/dashboards |
| Testing | PASS | Unit tests for middleware; integration tests for log pipeline (spec Task 2.1) |
| Performance | PASS | Dashboard load <5s, alert latency <2min defined in Success Criteria |

**All gates PASS. No violations requiring justification.**

## Project Structure

### Documentation (this feature)

```text
specs/024-logging-observability-improvements/
├── spec.md              # Feature specification
├── plan.md              # This file
├── architecture.md      # Technical architecture (exists)
├── tasks.md             # Implementation tasks (exists)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── checklists/          # Requirements quality checklists
```

### Source Code (repository root)

```text
# Infrastructure (Kubernetes manifests)
infra/k8s/monitoring/
├── loki/
│   ├── statefulset.yaml
│   ├── configmap.yaml
│   ├── service.yaml
│   └── ingress.yaml
├── promtail/
│   ├── daemonset.yaml
│   ├── configmap.yaml
│   ├── serviceaccount.yaml
│   └── clusterrole.yaml
├── grafana/
│   ├── ingress.yaml
│   └── dashboards/
│       ├── logs-overview.json
│       ├── service-logs.json
│       ├── request-tracing.json
│       └── inference-backends.json
└── alerts/
    └── logging-alerts.yaml

# GitOps (ArgoCD Applications)
gitops/clusters/development/apps/
├── loki.yaml
├── promtail.yaml
└── monitoring-dashboards.yaml

# Shared Libraries
shared/go/middleware/
├── request_logger.go
└── request_logger_test.go

# Frontend
web/portal/src/lib/
├── logger.ts
├── logger.test.ts
└── sentry.ts
```

**Structure Decision**: Infrastructure-first approach. Components deployed as Kubernetes resources managed by ArgoCD. Shared Go middleware added to existing `shared/go/` structure. Frontend logger added to existing `web/portal/src/lib/` structure.

## Complexity Tracking

> No constitution violations. Table not required.

## Implementation Phases

### Phase 1: Infrastructure Foundation (Critical)
- Deploy Loki StatefulSet with PersistentVolume
- Deploy Promtail DaemonSet with scrape configs
- Configure Grafana with Loki datasource
- **Add vLLM log collection** (ai-models, system, kserve namespaces)
- Configure OTEL Collector for Loki/Tempo export

### Phase 2: Backend Enhancements (High)
- Create request logger middleware with error body logging
- Integrate middleware into all Go services
- Add pod annotations for Promtail scraping
- Configure log sampling for verbose endpoints

### Phase 3: Frontend Enhancements (High, Parallel)
- Create structured frontend logger
- Integrate Sentry SDK
- Update ErrorBoundary with Sentry
- Replace console.log calls

### Phase 4: Alerting & Dashboards (Medium)
- Create PrometheusRule for error rate alerts
- Create service log dashboard
- Create request tracing dashboard
- Create inference backend dashboard

### Phase 5: Documentation (Medium-High)
- Create debugging runbook
- Create AI assistant debugging guide
- Update CLAUDE.md with observability section

## Dependencies

| Dependency | Type | Status | Notes |
|------------|------|--------|-------|
| Grafana | External | Exists | May need version upgrade |
| Prometheus | External | Exists | For alerting rules |
| ArgoCD | External | Exists | For GitOps deployment |
| NGINX Ingress | External | Exists | For external access |
| GPU Node Pool | External | Exists | For vLLM pods to monitor |

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Loki storage costs grow | Medium | Medium | Retention policies, log sampling |
| High cardinality labels | Medium | High | Label allowlist, drop rules in Promtail |
| vLLM log format changes | Low | Medium | Flexible regex patterns, version pinning |
| Performance impact from logging | Low | Medium | Async logging, sampling for verbose endpoints |
