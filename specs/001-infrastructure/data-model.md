# Data Model: Infrastructure Provisioning

**Branch**: `001-infrastructure`  
**Date**: 2025-11-08  
**Spec**: `/specs/001-infrastructure/spec.md`

## Entities

### Environment
- **description**: Logical partition (development, staging, production, system operations) provisioned within the shared LKE cluster.
- **fields**:
  - `name`: enum (`development`, `staging`, `production`, `system`)
  - `namespace`: Kubernetes namespace identifier (`env-<name>`)
  - `labels`: key/value map used by ArgoCD, Prometheus, and policies
  - `quota`: CPU/memory/pod limits configured via `ResourceQuota`
  - `status`: enum (`provisioning`, `ready`, `degraded`, `recovering`, `retired`)
- **relationships**:
  - Owns one or more **NodePool** entries.
  - Associated with an **AccessPolicy** and **SecretsBundle**.
  - Observed by a dedicated **EnvironmentDashboard**.
- **rules**:
  - Must transition to `ready` before accepting application deployments.
  - `status` changes published as events to audit log.

### NodePool
- **description**: Worker node configuration per environment, managed by Terraform LKE resources.
- **fields**:
  - `id`: Linode node pool ID
  - `instance_type`: string (e.g., `g6-standard-8`, `g1-gpu-rtx6000`)
  - `count`: integer (min nodes)
  - `max_count`: integer (autoscaling upper bound)
  - `taints`: optional array of Kubernetes taints
  - `labels`: map of scheduling labels
- **relationships**:
  - Belongs to an **Environment**.
  - Referenced by **DeploymentPolicy** for autoscaling rules.
- **rules**:
  - Production node pools require multi-AZ when available.
  - GPU pools only attached to `system` environment initially.

### AccessPolicy
- **description**: RBAC definitions and credential packages granting human or automation access.
- **fields**:
  - `role`: enum (`platform-engineer`, `app-team`, `read-only`, `break-glass`)
  - `kubeconfig_secret`: reference to secret location (Linode secrets manager ARN)
  - `permissions`: list of Kubernetes API groups/verbs
  - `expiry`: ISO-8601 timestamp for credential rotation
- **relationships**:
  - Linked to **Environment** (one policy per role per environment).
  - Consumed by **ServiceHandoffContract** for application teams.
- **rules**:
  - Break-glass credentials expire within 24 hours and require incident ticket.
  - Policies must be reviewed quarterly and recorded in audit log.

### SecretsBundle
- **description**: Collection of environment-scoped secrets synced to Sealed Secrets.
- **fields**:
  - `bundle_id`: UUID
  - `items`: array of `{name, purpose, rotation_frequency_days}`
  - `source`: enum (`linode-secret-manager`, `ext-vault`)
  - `synced_at`: timestamp of latest sync
- **relationships**:
  - Belongs to an **Environment**.
  - Mirrored into Kubernetes as `SealedSecret` + `Secret`.
- **rules**:
  - Must contain at least API token and metrics credentials per environment.
  - Rotation must update source and cluster; missing sync triggers alert.

### ObservabilityStack
- **description**: Deployments providing metrics, logs, and traces.
- **fields**:
  - `chart_version`: Helm chart version (e.g., `kube-prometheus-stack-56.3.0`)
  - `dashboards`: list of Grafana dashboard IDs/paths
  - `alert_routes`: mapping of alert rule → contact channel
  - `otel_collectors`: number of OpenTelemetry collectors deployed
- **relationships**:
  - One stack per cluster, with Environment-specific `values` overlays.
  - Feeds **EnvironmentDashboard** entities.
- **rules**:
  - Must expose `/metrics`, `/healthz`, `/readyz` endpoints.
  - Alert routes require acknowledgement within 5 minutes (per SC-006).

### EnvironmentDashboard
- **description**: Grafana dashboard summarizing environment health.
- **fields**:
  - `dashboard_uid`: Grafana UID
  - `panels`: array (CPU, memory, ingress latency, pod restarts, secret freshness)
  - `alerts`: array of bound alert rules
- **relationships**:
  - Tied to an **Environment** and fed by **ObservabilityStack**.
- **rules**:
  - Must include drill-down links to Loki and Tempo queries.
  - Creation/update recorded in `docs/platform/observability-guide.md`.

### TerraformModule
- **description**: Reusable Terraform component encapsulating infrastructure logic.
- **fields**:
  - `name`: string (`lke-cluster`, `network`, `observability`, `secrets-bootstrap`)
  - `inputs`: map of variables with types and defaults
  - `outputs`: map of exported values (endpoints, IDs)
  - `version`: semantic version tracked via git tag
- **relationships**:
  - Invoked by environment-specific Terraform stacks.
  - Documented in `contracts/terraform-modules.md`.
- **rules**:
  - Must include `README.md` with usage example and input/output descriptions.
  - Terraform validations enforce required inputs before apply.

### ArgoApplication
- **description**: ArgoCD Application or ApplicationSet driving Helm deployments.
- **fields**:
  - `name`: string (`baseline-observability`, `ingress`, `sample-service`)
  - `source_repo`: git URL/path
  - `destination_namespace`: Kubernetes namespace
  - `sync_policy`: settings (automated, self-heal, prune)
  - `sync_wave`: integer controlling deployment order
- **relationships**:
  - Deploys Helm charts defined under `infra/helm`.
  - Depends on outputs from **TerraformModule** (e.g., DNS zones).
- **rules**:
  - Production applications require manual wave gating (approval hook).
  - Self-heal enabled for baseline components; sample service uses manual sync.

### DriftReport
- **description**: Result of scheduled `terraform plan` with read-only credentials.
- **fields**:
  - `report_id`: UUID
  - `environment`: reference to **Environment**
  - `detected_at`: timestamp
  - `drift_summary`: enum (`none`, `minor`, `major`)
  - `details_path`: object storage path to full plan output
- **relationships**:
  - Generated by drift detection workflow under `scripts/infra/drift-detect.sh`.
  - Trigger notifications to platform Slack channel.
- **rules**:
  - `major` drift must open incident ticket automatically.
  - Reports retained for 90 days.

### RollbackPlan
- **description**: Documented procedure and automation artifacts for reverting infrastructure changes.
- **fields**:
  - `change_id`: Git commit or PR reference
  - `trigger_condition`: list (failed deploy, alert, manual decision)
  - `steps`: ordered commands leveraging `scripts/infra/rollback.sh`
  - `validation`: checks to confirm successful rollback (cluster health, metrics)
- **relationships**:
  - Linked to **DriftReport** and change approvals.
  - Referenced in `docs/runbooks/infrastructure-rollback.md`.
- **rules**:
  - Must be generated for every production change (SC-007).
  - Includes verification of secrets and ArgoCD sync status post-rollback.

### ServiceHandoffContract
- **description**: Document/API describing how application teams consume environments.
- **fields**:
  - `environment`: reference to **Environment**
  - `kubeconfig_url`: signed URL to retrieve kubeconfig
  - `ingress_base_domain`: string
  - `observability_links`: list of dashboard URLs
  - `deployment_steps`: ordered list of tasks for sample service
- **relationships**:
  - Backed by OpenAPI definition in `contracts/service-handoff-openapi.yaml`.
- **rules**:
  - Must be updated automatically on environment recreation.
  - Access requires `platform-engineer` approval for production.

## State Transitions

### Environment Lifecycle

| Current State | Event | Next State | Notes |
|---------------|-------|------------|-------|
| `provisioning` | Terraform/Helm apply succeeds | `ready` | Terratest and smoke tests pass; ArgoCD sync health `Healthy` |
| `provisioning` | Critical failure detected | `recovering` | Rollback triggered using latest state snapshot |
| `ready` | Health check failure (alerts triggered) | `degraded` | Investigate via runbook; traffic may be impacted |
| `degraded` | Remediation complete | `recovering` | Apply fixes, run validation checks |
| `recovering` | Validation passes | `ready` | Service restored; incident resolved |
| `ready` | Scheduled retirement | `retired` | Environment archived; namespaces deleted after backup |

State changes emit audit events stored in Loki and written to `docs/runbooks/infrastructure-rollback.md`.

