# Quickstart: Infrastructure Provisioning

**Branch**: `001-infrastructure`  
**Date**: 2025-11-08  
**Audience**: Platform engineers standing up core environments

---

## 1. Prerequisites

1. **Linode access & tokens**
   - Follow `docs/platform/linode-access.md` to create a Personal Access Token with `linodes:read_write`, `lke:read_write`, and `object-storage:read_write`.
   - Export variables (consider `direnv`):
     ```bash
     export LINODE_TOKEN=lnsk_...
   export LINODE_DEFAULT_REGION=fr-par
     ```
2. **Tooling**
   ```bash
   brew install terraform helm kubernetes-cli kustomize \
     && pipx install terratest-suite==0.3.0 \
     && npm install -g @redocly/cli
   ```
   - Alternatively, run `make infra-tools` (to be added in implementation) to install pinned versions from `configs/tool-versions.mk`.
   - Deployment defaults to the Linode Paris region (`fr-par`). Override per environment by editing `infra/terraform/environments/_shared/variables.tf` (`default_region`/`region_overrides`).
3. **GitHub Actions secrets**
   - Store `LINODE_TOKEN`, `LINODE_OBJECT_STORAGE_ACCESS_KEY`, `LINODE_OBJECT_STORAGE_SECRET_KEY`, and `SLACK_WEBHOOK_URL` in repository secrets.
   - Configure environment protection rules for `production` requiring manual approval.
4. **State backend bucket**
   ```bash
   linode-cli obj mb infra-state
   linode-cli obj policy set infra-state --file infra/terraform/state-policy.json
   ```
   - Bucket name and policy documented in `infra/terraform/backend.hcl`.

## 2. Bootstrap the Infrastructure Stack

```bash
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas
SPECIFY_FEATURE=001-infrastructure \
  terraform -chdir=infra/terraform/environments/development init
make -C infra/terraform plan ENV=development
```

- The Make target wraps `terraform fmt`, `validate`, `tflint`, `tfsec`, and outputs a signed plan artifact.
- Repeat for `staging`, `production`, and `system` after validating quotas.
- Apply changes post-approval:
  ```bash
  make -C infra/terraform apply ENV=development
  ```
- Applying `production` requires `--approve` flag from a GitHub Actions deployment job.

## 3. Provision Baseline Add-ons

1. **ArgoCD bootstrap**
   ```bash
   ./scripts/infra/apply.sh argo-bootstrap --env development
   ```
   - Script applies the `argo-bootstrap` Terraform module and syncs `infra/argo/applications/infrastructure.yaml`.
2. **Observability stack**
   ```bash
   ./scripts/infra/apply.sh observability --env development
   ```
   - Helm values reference `values/development.yaml` overlays.
3. **Ingress & cert-manager**
   ```bash
   ./scripts/infra/apply.sh ingress --env development
   ./scripts/infra/apply.sh cert-manager --env development
   ```
   - DNS entries for `*.dev.ai-aas.dev` should resolve to Linode Load Balancer (documented in `docs/platform/infrastructure-overview.md`).

## 4. Sync Secrets

```bash
./scripts/infra/secrets/sync.sh --env development \
  --source linode \
  --bundle secrets/bundles/development.yaml
```

- Script retrieves secrets from Linode secret manager, encrypts with Sealed Secrets key, and applies via ArgoCD.
- Verify rotation schedules with:
  ```bash
  ./scripts/infra/secrets/rotate.sh --env development --dry-run
  ```

## 5. Validate with Sample Service

```bash
make -C infra/tests/infra terratest ENV=development
kubectl --context dev-platform -n env-development get pods
```

- Terratest deploys `infra/helm/charts/sample-service` via ArgoCD ApplicationSet and runs health checks.
- Access the sample endpoint:
  ```bash
  curl https://sample.dev.ai-aas.dev/healthz
  ```
- Confirm metrics ingest:
  ```bash
  kubectl --context dev-platform -n monitoring port-forward svc/kube-prometheus-stack-prometheus 9090:9090
  open http://localhost:9090/graph
  ```
- Review generated artifacts under `infra/terraform/environments/<env>/.generated/` (network policies, secrets manifests, observability values, ApplicationSet manifests) and commit or promote via GitOps workflows.

## 6. Observability & Alerts

- Grafana dashboards: `https://grafana.dev.ai-aas.dev/d/<environment>-overview`.
- Alertmanager routes to `#platform-infra` Slack channel; acknowledge within 5 minutes per SC-006.
- Loki queries preconfigured under `observability-links.json`.

## 7. Rollback Procedure

1. Fetch latest state snapshot:
   ```bash
   ./scripts/infra/state-backup.sh --env development
   ```
2. Execute rollback script:
   ```bash
   ./scripts/infra/rollback.sh --env development --change <CHANGE_ID>
   ```
3. Validate:
   ```bash
   ./scripts/infra/validate.sh --env development
   kubectl --context dev-platform get ns env-development
   ```
4. Record outcome in `docs/runbooks/infrastructure-rollback.md`.

## 8. Troubleshooting

| Scenario | Resolution |
|----------|------------|
| `terraform plan` stalls waiting for lock | Ensure no concurrent applies; run `make -C infra/terraform force-unlock ENV=<env> LOCK_ID=<id>` |
| LKE cluster reports insufficient resources | Check Linode quotas; open support ticket; temporarily scale down staging to free nodes |
| ArgoCD application fails to sync | Inspect `argocd app get <name>`; check for pending manual approval in production |
| Sealed Secrets failing to decrypt | Rotate key pair (`./scripts/infra/secrets/rotate.sh --env <env> --regenerate-key`) and reapply bundles |
| Alert flood during maintenance | Set maintenance window annotation `maintenance.ai-aas.dev/enabled=true` on environment namespace; silences alerts automatically |

---

**Next Steps**: Run `/speckit.tasks` to review detailed implementation tasks, ensure `docs/platform/infrastructure-overview.md` is updated with architecture diagrams, and coordinate with `002-local-dev-environment` for developer kubeconfig distribution.

