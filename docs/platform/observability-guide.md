# Infrastructure Observability Guide

**Feature**: `001-infrastructure`  
**Last Updated**: 2025-11-08  
**Owner**: Platform Engineering

## 1. Stack Components

- **Prometheus** (kube-prometheus-stack):
  - Scrapes cluster metrics, Kubernetes components, ingress, sample service.
  - Retention: 15 days (disk provisioned per environment).
- **Grafana**:
  - Dashboards stored as code under `infra/helm/charts/observability-stack/dashboards/`.
  - Authentication delegated to GitHub OAuth (org SSO).
- **Alertmanager**:
  - Routes alerts to Slack (`#platform-infra`), PagerDuty (critical), and email (low priority).
  - Silence windows managed via `maintenance.ai-aas.dev/enabled` annotation or Alertmanager UI.
- **Loki**:
  - Collects container logs (JSON structured) with environment label.
  - Retention: 14 days; shipped to object storage for archival (30 days).
- **Tempo**:
  - Ingests OpenTelemetry traces; sample service emits traces for handshake pipeline.

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

## 2. Required Dashboards

| Dashboard UID | Name | Purpose | Owner |
|---------------|------|---------|-------|
| `env-dev-overview` | Dev Environment Overview | CPU/memory/pod counts, ingress latency, alerts summary | Platform Eng |
| `env-stg-overview` | Staging Overview | Same as above, includes canary error rate | Platform Eng |
| `env-prod-overview` | Production Overview | Business SLOs, infrastructure SLOs, pod disruption budget | SRE On-call |
| `env-sys-overview` | System Ops Overview | GPU utilization, queue lag, controller health | Systems Team |
| `infra-access` | Access & Secrets | Track secret rotations, access package issuance | Security |

Dashboards must include links to Loki queries (`Explore` view) and Tempo traces filtered by `environment`.

## 3. Alert Policies

- **EnvironmentDegraded**: Fires when namespace health check fails (`status != Healthy` for 5 minutes). PagerDuty critical.
- **IngressLatencyHigh**: P95 latency > 300ms for 5 minutes. Slack warning.
- **SecretsRotationDue**: `last_rotation_age > rotation_goal - 3 days`. Slack reminder.
- **DriftDetectedMajor**: Drift detection reports `major`. PagerDuty critical + Slack.
- **NodePoolCapacityLow**: Autoscaler at max capacity for 10 minutes. Slack warning with remediation steps.

Alert definitions codified in `infra/helm/charts/observability-stack/templates/alerts/*.yaml`. Every change requires updating success criteria SC-006/SC-007 verification notes.

## 4. Operational Runbooks

- `docs/runbooks/infrastructure-rollback.md`: Use after alert indicates failed deployment.
- `docs/runbooks/infrastructure-troubleshooting.md`: Map alerts to remediation.
- `docs/platform/access-control.md`: Reference for access-related alerts.

## 5. Verification & Testing

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

## 6. Onboarding Checklist

- [ ] Grafana user added to `platform-observers` team.  
- [ ] Access package includes dashboard URLs (`observability-links.json`).  
- [ ] Alert routing documented in PagerDuty service directory.  
- [ ] Synthetic monitors configured in Checkly (HTTP uptime per environment).  
- [ ] Observability section in `specs/001-infrastructure/quickstart.md` reviewed.

Update this guide whenever alert thresholds, dashboards, or tooling change.

