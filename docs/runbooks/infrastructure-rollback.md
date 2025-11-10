# Runbook: Infrastructure Rollback

**Feature**: `001-infrastructure`  
**Last Updated**: 2025-11-08  
**Audience**: Platform On-Call

---

## 1. When to Execute

- Terraform apply failed in production and cluster is unstable.
- Drift detection flagged `major` and automated remediation unsuccessful.
- Alert `EnvironmentDegraded` persists after remediation attempts.
- Security incident requires revoking recent infrastructure change.

## 2. Prerequisites

- Confirm incident ticket open (e.g., `INC-2025-1108-01`).
- Ensure `LINODE_TOKEN` and Terraform backend credentials are available (`LINODE_OBJECT_STORAGE_ACCESS_KEY` / `LINODE_OBJECT_STORAGE_SECRET_KEY` → exported as `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` before running Terraform CLI).
- Obtain latest successful state snapshot:
  ```bash
  make -C infra/terraform state-pull ENV=production > /tmp/prod-before-rollback.tfstate
  ```

## 3. Rollback Steps

1. **Freeze Deployments**
   - Pause ArgoCD sync for affected apps:
     ```bash
     argocd app suspend infrastructure-prod
     ```
   - Notify `#platform-infra` channel.

2. **Identify Change Set**
   - Pull PR or commit causing issue.
   - Inspect Terraform plan output stored in Object Storage (`linode-cli obj ls --cluster fr-par-1 ai-aas/terraform/plans/<env>/`).

3. **Execute Rollback**
   ```bash
   ./scripts/infra/rollback.sh \
     --env production \
     --change <PR_NUMBER> \
     --state /tmp/prod-before-rollback.tfstate
   ```
   - Script performs:
     - `terraform init` with snapshot state.
     - `terraform plan -refresh-only` to validate baseline.
     - `terraform apply` reverting to prior configuration.
     - ArgoCD sync of pinned manifests.

4. **Validate**
   ```bash
   ./scripts/infra/validate.sh --env production
   kubectl --context prod-platform get ns env-production
   ./tests/infra/healthcheck.sh --env production
   ```

5. **Resume GitOps**
   ```bash
   argocd app resume infrastructure-prod
   argocd app sync infrastructure-prod
   ```

6. **Communicate**
   - Post update in incident ticket with summary and next steps.
   - Share Grafana screenshot showing metrics stabilized.

## 4. Post-Rollback Tasks

- Run drift detection manually to confirm no lingering issues:
  ```bash
  ./scripts/infra/drift-detect.sh --env production --force
  ```
- Rotate any secrets touched during rollback.
- Capture a fresh state snapshot:
  ```bash
  ./scripts/infra/state-backup.sh --env production
  ```
- Schedule root-cause postmortem.
- Update `docs/runbooks/infrastructure-troubleshooting.md` if new failure mode discovered.
- Close incident after verification period (minimum 1 hour monitoring).
- Ensure automated drill workflow `.github/workflows/infra-rollback-drill.yml` remains scheduled; document the run outcome and timing in this runbook.

## 5. Rollback Checklist

- [ ] Deployments paused and communicated.  
- [ ] Terraform state snapshot captured.  
- [ ] Rollback script executed successfully.  
- [ ] Validation scripts passed.  
- [ ] ArgoCD sync resumed.  
- [ ] Stakeholders notified with impact summary.  
- [ ] Post-incident actions assigned.

Keep this runbook version-controlled; any change to steps must be tested in staging before updating production guidance.

