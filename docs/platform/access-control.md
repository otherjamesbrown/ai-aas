# Infrastructure Access Control Guide

**Feature**: `001-infrastructure`  
**Last Updated**: 2025-11-08  
**Owner**: Platform Engineering

## 1. Roles & Responsibilities

| Role | Scope | Capabilities | Approval Required |
|------|-------|--------------|-------------------|
| `platform-engineer` | Cluster-wide | Terraform applies, ArgoCD admin, secrets rotation, node scaling | Yes (peer review) |
| `app-team` | Namespaced (per environment) | Deploy workloads, read metrics/logs, manage namespace-scoped secrets | No (auto on ticket) |
| `read-only` | Namespaced | `kubectl get` access, Grafana read-only dashboard view | No (manager approval) |
| `break-glass` | Cluster-wide | Temporary admin access (expires ≤8h) for incidents | Yes (director + incident commander) |

## 2. Access Package Issuance

1. Requester submits ticket in `Jira: INFRA-Access` with environment, role, duration.
2. Platform engineer reviews and runs:
   ```bash
   ./scripts/infra/access-package.sh \
     --env staging \
     --role app-team \
     --expires-in 8h \
     --ticket INFRA-1234
   ```
3. Script generates package and uploads artifacts to Linode Object Storage (`access-packages/<env>/<role>/<timestamp>/`).
4. Signed URLs sent to requester via secure channel (1Password, Slack DM with ephemeral retention).
5. Audit event stored in Loki label `event=access_package_issued`.

## 3. Credential Rotation

- Default rotation: every 30 days for app-team, every 90 days for read-only, every 7 days for platform-engineer.
- Execute:
  ```bash
  ./scripts/infra/secrets/rotate.sh --env production --role app-team
  ```
- Script revokes existing kubeconfig, regenerates certificates, updates Secret Manager, and re-seals Kubernetes secrets.
- Notify impacted teams using template in `docs/templates/notifications/access-rotation.md` (to be added).

## 4. Break-Glass Procedure

1. Incident commander requests access in `#platform-incident` with ticket reference.
2. Two-factor confirmation by Director of Platform.
3. Platform engineer issues package with `--role break-glass --expires-in 4h`.
4. Access monitored via Kubernetes audit logs and Loki queries (`event=break_glass_action`).
5. Access automatically revoked at expiration; manual revocation using:
   ```bash
   ./scripts/infra/access-package.sh --env production --revoke <package-id>
   ```
6. Post-incident review ensures no lingering permissions.

## 5. Observability & Alerting for Access

- Alert rule `AccessPackageExpiry` warns 1 hour before expiration (Slack DM to requester + `#platform-infra`).
- Loki query for monitoring:
  ```logql
  {app="access-api"} | json | packageStatus="issued"
  ```
- Grafana dashboard `access-overview` tracks active packages, expirations, and rotation status.

## 6. Compliance Checklist

- [x] Requests tied to ticket numbers.  
- [x] All production access requires peer approval.  
- [x] Break-glass access limited to ≤8 hours.  
- [x] Rotation schedule documented and automated.  
- [x] Revocation tested quarterly (SC-007 alignment).

Update this guide whenever roles, durations, or tooling change to keep downstream specs aligned.

