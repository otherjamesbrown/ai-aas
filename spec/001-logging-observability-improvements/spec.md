# Spec 001: Logging & Observability Improvements

## Overview

This specification proposes improvements to the AI-AAS platform's logging and observability infrastructure to enable effective debugging, error capture, and production incident investigation.

## Problem Statement

### Current State

The platform has foundational logging in place:
- Go services use Uber Zap with structured JSON output
- Shared logging package provides consistency and sensitive data redaction
- OpenTelemetry instrumentation exists for distributed tracing
- ErrorBoundary catches React rendering errors

### Critical Gaps

| Gap | Impact | Severity |
|-----|--------|----------|
| No log aggregation backend | Cannot search historical logs or debug past issues | **Critical** |
| Logs not persisted | Lose all context when pods restart | **Critical** |
| No error tracking/alerting | Issues go unnoticed until user reports | **High** |
| Inconsistent frontend logging | Difficult to filter/debug browser issues | **Medium** |
| No request/response logging | Cannot replay or analyze failed API calls | **Medium** |
| No log sampling | Health check noise floods log output | **Low** |

### Impact

Without these capabilities, debugging production issues requires:
1. Hoping the relevant pod hasn't restarted
2. Manually tailing logs across multiple services
3. Guessing at request flows without correlation
4. No historical data for pattern analysis

## Proposed Solution

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Kubernetes Cluster                            │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │
│  │ api-router   │  │ user-org     │  │ analytics    │                  │
│  │   service    │  │   service    │  │   service    │                  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                  │
│         │                 │                 │                          │
│         │    stdout/stderr (JSON logs)      │                          │
│         ▼                 ▼                 ▼                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Promtail (DaemonSet)                         │   │
│  │         Collects logs from all pods via container runtime       │   │
│  └─────────────────────────────┬───────────────────────────────────┘   │
│                                │                                       │
│                                ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                         Loki                                    │   │
│  │              Log aggregation and storage                        │   │
│  └─────────────────────────────┬───────────────────────────────────┘   │
│                                │                                       │
│                                ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                       Grafana                                   │   │
│  │         Dashboards, log exploration, alerting                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────┐                                                      │
│  │  Web Portal  │ ──────► Sentry (External SaaS)                       │
│  │  (Browser)   │         Error tracking & session replay              │
│  └──────────────┘                                                      │
└─────────────────────────────────────────────────────────────────────────┘
```

### Component Breakdown

#### 1. Log Aggregation Stack (Loki + Promtail + Grafana)

**Why Loki over ELK?**
- Native Prometheus ecosystem integration
- Lower resource footprint
- Label-based indexing matches our structured logging
- Already have Grafana patterns in the codebase

**Components:**
- **Promtail**: DaemonSet that tails container logs and pushes to Loki
- **Loki**: Horizontally-scalable log aggregation system
- **Grafana**: Visualization and alerting (may already exist for Prometheus)

#### 2. Frontend Logging Library

**Proposal: Implement structured browser logger**

Replace scattered `console.log` calls with a dedicated logger that:
- Supports log levels (debug, info, warn, error)
- Outputs structured JSON in production
- Integrates with OpenTelemetry for trace correlation
- Can be silenced in production for non-errors

#### 3. Request/Response Logging Middleware

**For Go services:**
- Log request metadata (method, path, headers minus sensitive ones)
- Log response status and duration
- Correlate with request_id and trace_id
- Configurable body logging for debugging

#### 4. Error Tracking (Sentry)

**Why Sentry?**
- ErrorBoundary already has Sentry integration scaffolded
- Industry standard for frontend error tracking
- Session replay helps reproduce issues
- Source map support for stack traces

**Scope:**
- Frontend: Unhandled exceptions, ErrorBoundary catches, API errors
- Backend: Optional integration for panic recovery

#### 5. Enhanced OTEL Collector Configuration

Update the existing collector to:
- Export traces to Jaeger or Tempo
- Export logs to Loki (via OTLP)
- Add service graph generation

## Design Decisions

### Decision 1: Loki vs ELK vs CloudWatch

| Option | Pros | Cons |
|--------|------|------|
| **Loki** | Low resource usage, Prometheus native, simple | Less powerful full-text search |
| ELK | Powerful search, mature | Heavy resource requirements |
| CloudWatch | Managed, no ops | Vendor lock-in, cost at scale |

**Decision: Loki** - Matches our Prometheus ecosystem, sufficient for our scale.

### Decision 2: Self-hosted vs SaaS for Error Tracking

| Option | Pros | Cons |
|--------|------|------|
| **Sentry SaaS** | No ops, session replay, mature | Monthly cost |
| Self-hosted Sentry | Free, data control | Complex to operate |
| GlitchTip | Simpler self-hosted | Fewer features |

**Decision: Sentry SaaS** - Free tier sufficient for development, worth the cost for production reliability.

### Decision 3: Log Retention Policy

| Environment | Debug/Info | Warn/Error | Audit |
|-------------|------------|------------|-------|
| Development | 7 days | 14 days | 30 days |
| Production | 14 days | 30 days | 90 days |

## Success Criteria

1. **Log Search**: Can search logs across all services for the past 14 days
2. **Trace Correlation**: Can follow a request from frontend through all backend services
3. **Error Alerting**: Production errors trigger notifications within 5 minutes
4. **Debug Workflow**: Can reproduce and debug an issue reported by a user within 1 hour
5. **No Log Loss**: Logs persist through pod restarts and deployments

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Loki storage costs grow | Medium | Medium | Implement retention policies, log sampling |
| Sentry quota exceeded | Low | Low | Configure rate limiting, filter noise |
| Performance impact from logging | Low | Medium | Async logging, sampling for verbose endpoints |
| Sensitive data in logs | Medium | High | Extend redaction patterns, audit log output |

## Out of Scope

- Application Performance Monitoring (APM) - future enhancement
- Log-based anomaly detection - future enhancement
- Custom dashboards for business metrics - separate initiative
- Multi-region log replication - not needed at current scale

## References

- [Grafana Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Sentry Documentation](https://docs.sentry.io/)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- Current logging implementation: `shared/go/logging/`
- Current OTEL config: `infra/k8s/monitoring/otel-collector.yaml`
