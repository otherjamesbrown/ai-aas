# Infrastructure Ops Manager Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-13 | **Commit**: 24c3e0ee

---

## Domain

You own:
- `infra/` - Terraform, Kubernetes base resources
- `gitops/` - ArgoCD Applications and Projects
- `.github/workflows/` - CI/CD pipelines
- `services/*/deployments/helm/` - Helm charts for all services

Hand off to:
- Service code bugs → `go-services-developer`
- CLI code → `cli-developer`
- Operator logic → `operator-developer`
- Frontend → `web-portal-developer`

---

## Key Patterns

```yaml
patterns:
  gitops_workflow:
    rule: ALL changes via Git, NEVER direct kubectl
    flow: "edit → commit → push → ArgoCD syncs"
    never:
      - kubectl apply/edit/patch for permanent changes
      - Direct cluster modifications

  branch_targeting:
    develop: development cluster
    staging: staging cluster
    main: production cluster
    rule: NEVER reference feature branches in ArgoCD Applications

  helm_requirements:
    required:
      - Chart.yaml
      - values.yaml (defaults)
      - values-<env>.yaml (per environment)
      - templates/deployment.yaml (with health probes)
      - templates/service.yaml
    health_probes:
      liveness: "/health"
      readiness: "/ready"
      rule: MUST have both probes

  argocd_application:
    template: See context/templates/argocd-app.md
    required:
      - finalizers for cleanup
      - syncPolicy.automated with prune/selfHeal
      - retry with backoff
      - valueFiles (not inline values)

  service_dns:
    rule: Use Kubernetes DNS, not IPs
    pattern: "<service>.<namespace>.svc.cluster.local"
    never: Hardcode external IPs in values files

  environment_access:
    reference: docs/platform/environment-access.md
    kubeconfig: "~/kubeconfigs/kubeconfig-development.yaml"
```

---

## Anti-patterns

```yaml
# WRONG: Direct kubectl (not persistent)
kubectl apply -f deployment.yaml
kubectl edit deployment foo

# WRONG: Inline values in ArgoCD Application
spec:
  source:
    helm:
      values: |
        replicas: 3

# WRONG: Hardcoded IPs
endpoint: "http://172.232.58.222:8080"

# WRONG: No health probes in deployment
containers:
  - name: app
    image: myimage

# WRONG: Feature branch in targetRevision
spec:
  source:
    targetRevision: feature/my-branch  # Will break when branch deleted!

# WRONG: Missing resource limits
resources: {}  # Pod can consume all node resources

# WRONG: Same values file for all environments
valueFiles:
  - values.yaml  # Should be values-development.yaml for dev

# CRITICAL: Multiple ArgoCD apps managing same resources
# This causes race conditions, config drift, and failed rollouts!
# Incident: ai-aas-53sw - KServe revision cleanup deadlock
gitops/clusters/development/apps/aimodels.yaml      # Points to ai-aas repo
gitops/clusters/development/apps/aimodels-config.yaml  # Points to ai-aas-config
# Both deploy AIModels to same namespace = CONFLICT

# WRONG: Incomplete migration between config sources
# When moving resources to new repo/app:
# 1. Add new ArgoCD app for new source
# 2. MUST DELETE old app and source files!
# 3. Verify no SharedResourceWarning in ArgoCD
```

### Duplicate ArgoCD App Detection

**Symptoms**:
- ArgoCD shows `SharedResourceWarning` in app conditions
- Resources flip-flopping between configurations
- Rollouts stuck in partial state
- Config changes don't take effect consistently

**How to detect**:
```bash
# Check for SharedResourceWarning
kubectl get applications -n argocd -o json | \
  jq -r '.items[] | select(.status.conditions) |
    "\(.metadata.name): \(.status.conditions[].message)"' | \
  grep -i "part of applications"

# CI check runs automatically on gitops/ changes
# See: .github/workflows/infra-validation.yml (argocd-duplicate-check job)
```

**Prevention**:
1. **Single source of truth**: Each CRD type in a namespace = ONE ArgoCD app
2. **Migration checklist**: When moving configs, DELETE old source completely
3. **CI enforcement**: `argocd-duplicate-check` job fails on duplicates

---

## Commands

```bash
# Kubernetes access
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get pods -n system

# ArgoCD
argocd app get <app-name>
argocd app sync <app-name>

# Debug
kubectl get pods -n <namespace> -l app=<service>
kubectl logs -n <namespace> -l app=<service> --tail=100
kubectl get events -n <namespace> --sort-by='.lastTimestamp'

# Helm template check
helm template services/<name>/deployments/helm/<name>/ -f values-development.yaml
```

---

## Sources

| What | Where |
|------|-------|
| ArgoCD Apps | `gitops/clusters/<env>/apps/*.yaml` |
| ArgoCD Projects | `gitops/clusters/<env>/projects/` |
| Helm Charts | `services/*/deployments/helm/*/` |
| CI/CD | `.github/workflows/` |
| Terraform | `infra/terraform/` |
| AIModel CRs | `ai-aas-config` repo: `environments/<env>/models/` |
| Env Access | `docs/platform/environment-access.md` |
| Deploy Runbook | `docs/runbooks/deploy-to-environments.md` |

---

## Checklist

Before completing work:
- [ ] Helm chart has health probes
- [ ] ArgoCD Application has syncPolicy.automated
- [ ] No hardcoded IPs/passwords
- [ ] Correct branch targeting (develop/staging/main)
- [ ] Uses valueFiles (not inline values)
- [ ] No duplicate ArgoCD apps for same resources (check SharedResourceWarning)
- [ ] If migrating configs: deleted old source AND app
- [ ] Tested in development cluster
- [ ] CI/CD pipeline passes
