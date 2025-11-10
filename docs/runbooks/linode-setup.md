# Runbook: Linode Environment Setup

## Goal

Provision baseline infrastructure components required by project automation using Akamai Linode APIs.

## Prerequisites

- Personal access token with scopes `linodes`, `lke`, `object-storage`.
- `LINODE_TOKEN` and `LINODE_DEFAULT_REGION` exported in shell.
- Terraform/Helm configurations stored in infrastructure repository (outside scope here).

## Steps

1. **Create LKE Cluster (manual API example)**  
   ```bash
   curl -H "Authorization: Bearer $LINODE_TOKEN" \
     -H "Content-Type: application/json" \
  -d '{"label":"ai-aas-dev","region":"fr-par","k8s_version":"1.32"}' \
     https://api.linode.com/v4/lke/clusters
   ```
2. **Provision Supporting Linodes** (if needed for bastion/utility nodes). See [Linode Instance API](https://techdocs.akamai.com/linode-api/reference/post-linode-instance).
3. **Create Object Storage Bucket**  
   ```bash
   curl -H "Authorization: Bearer $LINODE_TOKEN" \
     -H "Content-Type: application/json" \
  -d '{"label":"ai-aas-build-metrics","cluster":"fr-par-1"}' \
     https://api.linode.com/v4/object-storage/buckets
   ```
4. Configure bucket lifecycle policy per `docs/metrics/policy.md`.
5. Apply Terraform/Helm manifests to install ingress, cert-manager, ArgoCD, etc. (Refer to infrastructure repo.)

## Verification

- `kubectl get nodes` shows expected pool.
- Object storage bucket accessible via `aws s3 ls`.
- CI secrets (`LINODE_TOKEN`) injected into GitHub Actions.

