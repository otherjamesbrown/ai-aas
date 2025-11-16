# Infrastructure Architecture Overview

**Feature**: `001-infrastructure`  
**Last Updated**: 2025-11-08  
**Owner**: Platform Engineering

## High-Level Architecture

- **Provider**: Akamai Linode Kubernetes Engine (LKE) in `fr-par` (configurable via `default_region` / `region_overrides` in Terraform).
- **Environments**: `development`, `staging`, `production`, `system` share a single control plane with dedicated namespaces and resource quotas.
- **Node Pools**:
  - Baseline: `g6-standard-8` (3–6 nodes per environment, autoscaling to 10).
  - GPU (system): `g1-gpu-rtx6000` (2 nodes) reserved for vLLM workloads.
- **Networking**:
  - Calico NetworkPolicies implement default-deny stance.
  - Ingress via NGINX Ingress Controller + cert-manager (Let's Encrypt for production).
  - Self-signed certificates for local development (see `infra/secrets/certs/README.md`).
  - External DNS records served through Linode DNS (production) or local hosts file (development).
- **GitOps**:
  - Terraform provisions clusters, networking, secrets scaffolding.
  - ArgoCD reconciles Helm charts (`infra/helm/charts/*`), including observability, ingress, and sample service.

## Change Management Flow

1. **Plan**: Contributor opens PR modifying `infra/terraform` or `infra/helm`.
2. **Validation**: GitHub Actions runs `terraform fmt/validate/plan`, `tflint`, `tfsec`, Terratest, and Helm lint.
3. **Approval**: Production changes require two approvals plus environment protection.
4. **Apply**:
   - Terraform: `make -C infra/terraform apply ENV=<env>` executed by GitHub Actions with OIDC.
   - ArgoCD: Syncs Git changes automatically (production gated via manual `argocd app sync`).
5. **Verification**: Automated smoke tests deploy sample service and confirm metrics/alerts.
6. **Audit**: Every change emits audit events to Loki (`infra_change_applied`) and posts summary to `#platform-infra`.

## Secrets & Access

- Linode Secret Manager is source-of-truth. Secret bundles defined under `infra/secrets/bundles/*.yaml`.
- Sealed Secrets controller encrypts bundle payloads; ArgoCD applies manifests per environment.
- Access packages described in `specs/001-infrastructure/contracts/environment-access.md`. Packages expire within 24 hours and require Slack request logged in `#platform-access`.

## Observability Stack

- **Prometheus/Grafana**: `kube-prometheus-stack` with environment-tagged dashboards.
- **Logs**: Loki with tenant per environment; Tempo enabled for tracing (optional by environment).
- **Alert Routing**: Alertmanager routes to Slack (`#platform-infra` primary, `#platform-pager` on-call).
- **Dashboards**: Overview dashboards named `<env>-overview`, stored under Grafana UID `env-<env>`.

## Sample Service Validation

- Helm chart: `infra/helm/charts/sample-service`.
- Validates ingress, secrets mount, metrics emission, and alert pipeline.
- Terratest pipeline asserts:
  - Deployment becomes `Available` within 5 minutes.
  - `/healthz` endpoint returns 200.
  - Prometheus scrape discovers `sample_service_up` metric.

## Compliance & Auditing

- Terraform state stored in Linode Object Storage bucket `ai-aas` (path `terraform/environments/<env>/terraform.tfstate`). Export `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` from the `.env` entries (`LINODE_OBJECT_STORAGE_ACCESS_KEY`, `LINODE_OBJECT_STORAGE_SECRET_KEY`) before running Terraform so the backend is accessible.
- Drift detection job (`scripts/infra/drift-detect.sh`) runs hourly; severity `major` opens incident automatically.
- ArgoCD application events forward to Loki (label `argo_app`).
- Quarterly controls: secrets rotation check, rollback drill (SC-007), alert simulation.

## TLS/SSL Certificates

### Production
- **Let's Encrypt**: Automatic certificate issuance and renewal via cert-manager
- **DNS**: Public DNS records required for Let's Encrypt validation
- **Ingress**: NGINX Ingress Controller with TLS termination

### Local Development
- **Self-Signed Certificates**: CA and TLS certificates in `infra/secrets/certs/`
- **Local DNS**: Hosts file entries for `.ai-aas.local` domains
- **Firewall-Restricted**: Access limited to authorized machines (no VPN required)
- **Setup**: See `infra/secrets/certs/README.md` for certificate generation and trust instructions

## Dependencies

- `docs/platform/linode-access.md`: Token creation and CLI setup.
- `docs/platform/access-control.md`: Detailed RBAC and package issuance procedures.
- `docs/platform/observability-guide.md`: Dashboard conventions, alert tuning.
- `docs/runbooks/infrastructure-rollback.md`: Step-by-step rollback instructions.
- `docs/runbooks/infrastructure-troubleshooting.md`: Issue resolution catalog.
- `infra/secrets/certs/README.md`: Self-signed certificate setup and management.

Keep this document updated alongside infrastructure changes to ensure context for downstream specs (`005-user-org-service`, `010-vllm-deployment`, `011-observability`).

## Capacity Validation

- Terraform module `infra/terraform/modules/lke-cluster/quotas.tf` defines per-environment quotas supporting 30 services.  
- Performance harness `tests/infra/perf/capacity_test.go` deploys placeholder workloads to verify scheduling and HPA thresholds.  
- Record results and tuning notes in this guide after each significant scaling event.

## Cluster Inventory

- **Development**: LKE cluster `531921`, kubeconfig context `lke531921-ctx`. GitHub secrets: `DEV_KUBECONFIG_B64`, `DEV_KUBE_CONTEXT`.
- **Production**: LKE cluster `531922`, kubeconfig context `lke531922-ctx`. GitHub secrets: `PROD_KUBECONFIG_B64`, `PROD_KUBE_CONTEXT`.
- **Staging/System**: Defined in Terraform for future rollout; keep configuration in sync but defer `terraform apply` until post-launch objectives require additional environments.
- Kubeconfigs are stored in 1Password and replicated into GitHub Actions secrets for automation (availability probes, scripted applies).

## Generated Artifacts

- Terraform renders manifests into `infra/generated/<environment>/` for GitOps promotion.
- Running Terraform per environment writes manifests and values into `infra/terraform/environments/<env>/.generated/`:
  - Network policies and firewall specs (`network/`).
  - Sealed secrets bootstrap manifests (`secrets/`).
  - Observability values overlays (`observability/`).
  - ArgoCD ApplicationSet manifests (`argo/`).
- Copy the contents into `infra/generated/<environment>/` (already committed) and promote those files into the GitOps repository that ArgoCD watches.



