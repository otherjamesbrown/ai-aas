# Research: Infrastructure Provisioning

**Branch**: `001-infrastructure`  
**Date**: 2025-11-08  
**Spec**: `/specs/001-infrastructure/spec.md`

## Research Questions & Answers

| Topic | Decision | Rationale | Alternatives Considered |
|-------|----------|-----------|--------------------------|
| Cloud provider surface | Standardize on Akamai Linode Kubernetes Engine (LKE) with multi-node pools in `fr-par` (configurable) | Aligns with target EU region while remaining configurable per environment, integrates with current automation, meets cost/SLA targets | AWS EKS (higher cost, larger blast radius), GKE (locks us into GCP billing) |
| Terraform state backend | Use Linode Object Storage bucket `infra-state` with Dynamo-like locking via `terraform-plugin-lock` wrapper | Keeps state off local machines, supports versioned backups, avoids external dependencies | Local state (unsafe), AWS S3 + Dynamo (introduces AWS dependency), Consul (ops overhead) |
| Secrets management & sync | Authoritative store in Linode Secret Manager; sync to Kubernetes via Sealed Secrets and CSI driver | Centralized auditing, encrypted at rest, integrates with Kubernetes and GitOps | HashiCorp Vault (heavier ops), SOPS with Git-committed secrets (risk of plaintext mishaps) |
| Observability baseline | Deploy kube-prometheus-stack + Loki + Tempo via Helm, aggregated through Grafana per environment | Satisfies constitution observability gate, provides metrics/logs/traces, widely adopted charts | Self-managed Prometheus deployment (more toil), Datadog (additional licensing) |
| Network isolation | Enforce Calico NetworkPolicies with default deny; expose ingress via NGINX + cert-manager | Implements zero-trust baseline, integrates with TLS automation, allows granular policies | Cilium (nice but more ops; not default on LKE), simple SecurityGroups (insufficient for K8s pods) |
| Deployment workflow | GitHub Actions plans Terraform + Helm, ArgoCD auto-syncs approved manifests; production changes require manual approval step | Maintains GitOps discipline, auditability, and rollback support | Direct Terraform applies from developer machines (untracked), FluxCD (less familiar to team) |
| Sample service validation | Package `sample-service` Helm chart with smoke tests and Terratest scenario to verify DNS, ingress, metrics | Demonstrates end-to-end readiness for app teams, documents handoff contract | Rely on manual kubectl testing (error-prone), skip sample (no validation) |
| Rollback & drift detection | Hourly drift detection using Terraform plan in read-only mode; rollback via captured state snapshots and ArgoCD sync waves | Ensures recoverability and compliance with declarative ops gate | Manual drift checks (unreliable), third-party drift tools (extra cost) |
| Compliance logging | Forward audit logs (Terraform, ArgoCD, Kubernetes events) to Loki with retention 30 days and export friendly labels | Provides traceability for security/compliance requirements | CloudTrail-style external service (not available on Linode), leaving logs only in stdout (hard to audit) |

## Outstanding Follow-ups

- Confirm Linode production quotas for GPU node pools to satisfy future vLLM workloads (handoff to spec `010-vllm-deployment`).  
- Finalize SLA for infrastructure incident response and integrate with on-call schedule (with Platform Ops).  
- Evaluate budget for dedicated staging cluster vs. shared cluster namespaces; current plan assumes single cluster with isolated namespaces.  
- Align ArgoCD project RBAC with organization-wide GitHub teams once identity provider decisions land in security roadmap.

