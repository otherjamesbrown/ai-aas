# Runbook: Infrastructure Troubleshooting

**Feature**: `001-infrastructure`  
**Last Updated**: 2025-11-08  
**Audience**: Platform Engineers, SRE

---

## Quick Reference Table

| Alert / Symptom | Likely Cause | Diagnostic Commands | Resolution | Escalation |
|-----------------|-------------|---------------------|------------|------------|
| `EnvironmentDegraded` | Failed ArgoCD sync or unhealthy workloads | `argocd app get infrastructure-<env>`; `kubectl get pods -n env-<env>` | Fix failing deployment, re-run sync, ensure health | Escalate to app owner if workload-specific |
| `IngressLatencyHigh` | Load balancer saturation, pod scaling issues | `kubectl top pods -n ingress-nginx`; `kubectl describe hpa sample-service -n env-<env>` | Scale node pool (`make -C infra/terraform apply ENV=<env> VAR=pool_size++`), check autoscaler logs | Platform on-call |
| `SecretsRotationDue` | Missed scheduled rotation | `./scripts/infra/secrets/status.sh --env <env>` | Run `rotate.sh`, verify sync, update ticket | Security if rotation fails |
| `DriftDetectedMajor` | Manual change or failed apply | `./scripts/infra/drift-detect.sh --env <env> --details` | Execute rollback runbook, reapply desired state | Immediate PagerDuty |
| `NodePoolCapacityLow` | Autoscaler pinned at max nodes | `linode-cli lke pools-list <cluster>`; `kubectl get nodes` | Increase node pool max, plan scaling in Terraform, notify capacity planning | Platform manager |
| Pod CrashLoop | Misconfigured secrets or image pull errors | `kubectl describe pod <pod>`; `kubectl logs <pod>` | Fix secret/config, redeploy; ensure registry access | App owner if service-specific |
| Terraform lock persists | Previous run interrupted | `terraform force-unlock <lock-id>` or `make -C infra/terraform force-unlock` | Clear lock, rerun plan/apply | N/A |
| ArgoCD authentication failure | Token expired or RBAC drift | `argocd account get-user-info`; `kubectl describe sa argocd-server` | Refresh token, reapply ArgoCD RBAC manifest | Platform security |

## Diagnostic Playbooks

### 1. Investigating Pod Failures

```bash
kubectl --context <env>-platform get pods -n env-<env>
kubectl --context <env>-platform describe pod/<pod-name> -n env-<env>
kubectl --context <env>-platform logs pod/<pod-name> -n env-<env> --previous
```
- Verify secrets mounted (`kubectl get secret <name> -n env-<env> -o yaml`).
- Check NetworkPolicies blocking dependencies.

### 2. Terraform Apply Errors

```bash
make -C infra/terraform plan ENV=<env>
terraform -chdir=infra/terraform/environments/<env> show plan.tfplan
```
- Ensure backend credentials valid.
- Confirm Linode quotas available via `linode-cli lke cluster-type-list`.

### 3. Drift Detection Follow-Up

```bash
./scripts/infra/drift-detect.sh --env <env> --details
terraform -chdir=infra/terraform/environments/<env> plan -detailed-exitcode
```
- If exit code 2, collect plan artifact and proceed with rollback runbook.

## Communication Templates

- **Slack (initial alert)**:
  > `:rotating_light: [ENV] Infrastructure alert triggered: <AlertName>. Investigating. Incident ticket: <ID>.`

- **Resolution update**:
  > `:white_check_mark: [ENV] <AlertName> cleared after <action>. Monitoring for 30 minutes before closing.`  

Templates stored in `docs/templates/notifications/` (to be created).

## Escalation Matrix

| Severity | Trigger | Escalate To | Response SLA |
|----------|---------|-------------|--------------|
| Sev 1 | Production outage, data loss, security breach | Platform Director + On-call SRE | 5 minutes |
| Sev 2 | Partial production impact, prolonged degradation | Platform On-call + Application Owner | 15 minutes |
| Sev 3 | Staging/system issues, tooling failures | Platform Engineer | 1 hour |
| Sev 4 | Non-blocking tasks | Backlog | Next business day |

## Post-Incident Tasks

- Update incident ticket with timeline, impact, root cause.
- Capture Grafana snapshots and attach to ticket.
- Review Terraform state and ArgoCD history for anomalies.
- Schedule postmortem within 48 hours for Sev 1/2 incidents.
- Update documentation or automation to prevent recurrence.

Keep this runbook synchronized with alert policies and tooling changes. Link new troubleshooting scenarios back to this file to maintain a single source of truth.

