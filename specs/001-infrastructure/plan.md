# Implementation Plan: Infrastructure Provisioning

**Branch**: `001-infrastructure` | **Date**: 2025-11-08 | **Spec**: `/specs/001-infrastructure/spec.md`  
**Input**: Feature specification from `/specs/001-infrastructure/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Provision four isolated Kubernetes environments (development, staging, production, system operations) on Akamai Linode using fully declarative Terraform and Helm pipelines. Deliver baseline networking, secrets, and observability scaffolding so application teams can deploy services with secure defaults, documented access, and GitOps-driven change management. Automation runs through GitHub Actions with validated rollback procedures, drift detection, and environment dashboards.

## Technical Context

**Language/Version**: Terraform 1.6.x, Helm 3.14+, Bash 5.2, Go 1.21 for Terratest harnesses  
**Primary Dependencies**: `linode/linode` Terraform provider, `hashicorp/kubernetes` provider, Helm charts (Ingress Nginx, cert-manager, Prometheus stack), Sealed Secrets controller, ArgoCD CLI  
**Storage**: Kubernetes etcd (managed by LKE), Linode Object Storage for state backups and Terraform remote state (`s3` backend), PostgreSQL/Redis endpoints provisioned separately but referenced for network policies  
**Testing**: Terratest suites invoking Terraform modules, `kubectl`/`helm` smoke tests via GitHub Actions, integration checks for sample service deployment using KinD plus Linode API stubs, policy checks via `tflint`, `tfsec`, `conftest`, and Kubernetes conformance tests (`sonobuoy` targeted)  
**Target Platform**: Akamai Linode Kubernetes Engine (LKE) clusters in `us-east`, GitHub Actions runners (ubuntu-latest) for automation, local developer workstations (macOS/Linux) with Linode CLI for spot testing  
**Project Type**: Multi-environment infrastructure-as-code stack with supporting runbooks and automation scripts  
**Performance Goals**: Full four-environment provisioning ≤45 minutes; incremental apply ≤10 minutes; sample service deployment ≤15 minutes post-provisioning; alert propagation ≤5 minutes; rollback ≤15 minutes  
**Constraints**: GitOps-only changes to production; no plaintext secrets in Terraform state; zero-trust network policies enforced by default; observability stack must publish metrics/logs/traces tagged per environment; automation must be idempotent and auditable  
**Scale/Scope**: Support 30 microservices per environment with autoscaling node pools, dual-availability-zone worker nodes where available, and baseline quotas for pods, CPU, memory, and persistent volumes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **API-First Interfaces**: Infrastructure exposed via Terraform modules and documented GitHub Actions/CLI workflows; sample OpenAPI contract provided for environment access metadata service consumed by application teams.  
- **Stateless Microservices & Async Non-Critical Paths**: Cluster design enforces stateless workloads (persistent state lives in managed Postgres/Redis). Background provisioning tasks (metrics export, telemetry uploads) run asynchronously through ArgoCD hooks and GitHub Actions dispatchers.  
- **Security by Default**: Secrets sourced from Linode secret manager + Sealed Secrets; RBAC roles and NetworkPolicies deliver zero-trust posture; CI includes `gitleaks`, `tfsec`, `tflint`, `trivy`, `hadolint`.  
- **Declarative Infrastructure & GitOps**: Terraform manages cloud primitives; Helm + ArgoCD reconcile cluster add-ons; no manual `kubectl` in production—automation emits change plans, requires approvals, and records drift.  
- **Observability**: Prometheus/Grafana stack, Loki, and OpenTelemetry Collector deployed as part of baseline; dashboards and alert routes defined per environment.  
- **Testing**: Terratest ensures Terraform modules converge; `sonobuoy` conformance verifies cluster health; integration tests validate sample service deploys and is observable; QA includes negative tests for network isolation.  
- **Performance**: Node pool sizing and autoscaling policies meet performance SLOs; plan defines load test hooks to confirm pod scheduling and ingress latency targets.  

No constitution waivers required; all gates satisfied with automation + documentation deliverables.

## Project Structure

### Documentation (this feature)

```text
specs/001-infrastructure/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── environment-access.md
│   ├── terraform-modules.md
│   └── service-handoff-openapi.yaml
└── tasks.md                 # generated via /speckit.tasks
```

### Source Code (repository root)

```text
infra/
├── terraform/
│   ├── Makefile                 # entry points for plan/apply/destroy/drift
│   ├── backend.hcl              # remote state (Linode object storage)
│   ├── environments/
│   │   ├── development/
│   │   │   └── main.tf
│   │   ├── staging/
│   │   │   └── main.tf
│   │   ├── production/
│   │   │   └── main.tf
│   │   └── system/
│   │       └── main.tf
│   ├── modules/
│   │   ├── lke-cluster/
│   │   ├── network/
│   │   ├── secrets-bootstrap/
│   │   ├── observability/
│   │   └── data-services/
│   └── pipelines/
│       └── github-actions/terraform-ci.yml
├── helm/
│   ├── charts/
│   │   ├── ingress-nginx/
│   │   ├── cert-manager/
│   │   ├── observability-stack/
│   │   └── sample-service/
│   └── values/
│       ├── development.yaml
│       ├── staging.yaml
│       ├── production.yaml
│       └── system.yaml
├── argo/
│   ├── applications/
│   │   ├── infrastructure.yaml   # ArgoCD ApplicationSet for baseline charts
│   │   └── sample-service.yaml
│   └── projects/platform.yaml
├── scripts/
│   ├── infra/
│   │   ├── plan.sh               # wraps terraform plan + security checks
│   │   ├── apply.sh              # orchestrates approved applies
│   │   ├── rollback.sh           # leverages terraform state snapshots
│   │   └── drift-detect.sh       # scheduled detection with Slack webhook
│   └── secrets/
│       └── sync.sh               # syncs Linode secrets into Sealed Secrets
├── .github/
│   └── workflows/
│       ├── infra-terraform.yml
│       ├── infra-availability.yml
│       ├── sample-service-smoke.yml
│       └── infra-rollback-drill.yml
├── docs/
│   ├── platform/
│   │   ├── infrastructure-overview.md
│   │   ├── access-control.md
│   │   └── observability-guide.md
│   └── runbooks/
│       ├── infrastructure-rollback.md
│       └── infrastructure-troubleshooting.md
└── tests/
    └── infra/
        ├── terratest/
        │   ├── environments_test.go
        │   └── network_policies_test.go
        ├── synthetics/
        │   └── availability_probe.go
        ├── perf/
        │   └── capacity_test.go
        └── integration/
            └── sample_service_deploy_test.go
```

**Structure Decision**: Establish an `infra/` top-level directory to separate infrastructure-as-code assets from application code. Terraform environments and modules reside under `infra/terraform`, with reusable pipelines under `pipelines/github-actions`. Helm and ArgoCD manifests live under `infra/helm` and `infra/argo` to enable GitOps reconciliation. Supporting scripts and runbooks are colocated under `scripts/infra` and `docs/platform` to keep operational workflows documented and executable. Tests for infrastructure (Terratest, integration smoke tests) live under `tests/infra` to align with existing repository testing conventions.

## Complexity Tracking

No constitution violations introduced. GitOps-first approach keeps automation declarative; additional directories (`infra/`, `tests/infra/`) are required to encapsulate infrastructure assets and maintain separation of concerns.
