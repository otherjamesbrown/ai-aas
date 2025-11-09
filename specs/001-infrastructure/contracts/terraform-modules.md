# Contract: Terraform Modules

**Branch**: `001-infrastructure`  
**Date**: 2025-11-08  
**Spec**: `/specs/001-infrastructure/spec.md`

This document enumerates the Terraform modules that compose the infrastructure provisioning workflow. Each module publishes a semantic version and requires the inputs/outputs listed below. Modules live under `infra/terraform/modules/` and are consumed by environment stacks (`infra/terraform/environments/<env>/main.tf`).

## Module Catalog

### `lke-cluster`

| Category | Value |
|----------|-------|
| **Versioning** | Tag `modules/lke-cluster/v0.1.0` (bump minor for new optional features, major for breaking changes) |
| **Providers** | `linode`, `kubernetes` |
| **Inputs** | `cluster_label` (string, required); `region` (string, default `fr-par`); `k8s_version` (string, default `1.29`); `node_pools` (list(object{ type, count, max_count, labels, taints })) |
| **Outputs** | `cluster_id` (string); `kubeconfig` (sensitive string); `api_endpoints` (list of strings) |
| **Validation Rules** | At least one node pool with count ≥ 3; GPU pool allowed only when `enable_gpu` flag true |

### `network`

| Category | Value |
|----------|-------|
| **Providers** | `linode`, `kubernetes` |
| **Inputs** | `cluster_id` (string); `base_domain` (string); `allowed_egress_cidrs` (list(string)); `ingress_whitelist` (list(string)); `environment` (string) |
| **Outputs** | `vpc_id`; `firewall_id`; `ingress_hostname` |
| **Validation Rules** | Deny-all NetworkPolicy created by default; explicit allow list must include DNS/metrics endpoints |

### `secrets-bootstrap`

| Category | Value |
|----------|-------|
| **Providers** | `linode`, `kubernetes` |
| **Inputs** | `environment` (string); `secrets_map` (map(object{ value, rotation_days, owner })); `sealed_secrets_key` (string) |
| **Outputs** | `secret_names` (list(string)); `last_rotation_at` (timestamp) |
| **Validation Rules** | Fails apply if any secret lacks owner or rotation schedule; rotations executed via Kubernetes Job |

### `observability`

| Category | Value |
|----------|-------|
| **Providers** | `helm`, `kubernetes` |
| **Inputs** | `environment` (string); `grafana_admin_secret` (string); `retention_days` (number, default 30); `tempo_enabled` (bool, default true) |
| **Outputs** | `dashboard_urls` (map); `alertmanager_webhook_url` (string) |
| **Validation Rules** | Helm release must be healthy before module outputs succeed; retention ≥ 30 days |

### `argo-bootstrap`

| Category | Value |
|----------|-------|
| **Providers** | `helm`, `kubernetes`, `kubectl` |
| **Inputs** | `environment` (string); `repo_url` (string); `revision` (string); `project_name` (string, default `platform`) |
| **Outputs** | `argo_app_name`; `project_id` |
| **Validation Rules** | Sync policy defaults to `automated` with `selfHeal=true`; production apps require manual sync gate flagged via input `requires_approval=true` |

## Compliance Expectations

- All modules must execute `terraform fmt`, `terraform validate`, `tflint`, and `tfsec` in the GitHub Actions pipeline.  
- Sensitive outputs (kubeconfig, secrets) are marked `sensitive = true` and redacted from plan/apply logs.  
- Module READMEs document upgrade steps and backward compatibility notes.  
- Breaking changes require updating the Traceability Matrix and notifying downstream specs (`002-local-dev-environment`, `005-user-org-service`).  
- Consumers must pin module versions to avoid accidental drift (`source = "./modules/lke-cluster?ref=v0.1.0"`).

