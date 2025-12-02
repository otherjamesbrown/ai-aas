# Feature Specification: ArgoCD Improvements

**Feature Branch**: `021-argocd-improvements`
**Created**: 2025-11-29
**Status**: Completed
**Input**: Audit and improve ArgoCD setup for production readiness with RBAC, retry policies, and branch management.

## Summary

This spec documents improvements made to the ArgoCD configuration to address security, reliability, and operational concerns identified during a comprehensive review. The improvements focus on three areas:

1. **RBAC Project Policies**: Replace wildcard permissions with explicit namespace destinations and source repository whitelisting
2. **Retry Policies**: Add automatic retry with exponential backoff to all Applications for transient failure resilience
3. **Branch Targeting**: Fix Applications pointing to deleted branches and standardize branch conventions

## Scope

### In Scope

- AppProject RBAC policy improvements (namespace restrictions, source repo restrictions)
- Retry policy configuration for all ArgoCD Applications
- Branch targeting standardization (develop for dev, main for prod)
- Documentation updates (CLAUDE.md, constitution.md, CI/CD pipeline docs)

### Out of Scope

- ArgoCD RBAC for users (SSO integration)
- Multi-cluster deployment
- Progressive delivery (Argo Rollouts)
- Notifications (ArgoCD Notifications)
- Secret management (External Secrets Operator)

## Changes Implemented

### 1. RBAC Project Policies

**File**: `gitops/clusters/development/projects/platform-project.yaml`

**Before**: Wildcard permissions allowing any namespace and source
```yaml
spec:
  destinations:
    - namespace: '*'
      server: https://kubernetes.default.svc
  sourceRepos:
    - '*'
```

**After**: Explicit restrictions
```yaml
spec:
  destinations:
    - namespace: argocd
      server: https://kubernetes.default.svc
    - namespace: development
      server: https://kubernetes.default.svc
    - namespace: admin-api-service
      server: https://kubernetes.default.svc
    # ... all required namespaces explicitly listed
  sourceRepos:
    - https://github.com/otherjamesbrown/ai-aas
    - https://istio-release.storage.googleapis.com/charts
    - https://knative.github.io/operator
    - https://grafana.github.io/helm-charts
    - https://kserve.github.io/kserve
  clusterResourceWhitelist:
    - group: ''
      kind: Namespace
    - group: apiextensions.k8s.io
      kind: CustomResourceDefinition
    # ... other required cluster-scoped resources
```

### 2. Retry Policies

**Applied to**: All 15+ ArgoCD Applications

Standard retry policy block added to each Application:
```yaml
syncPolicy:
  automated:
    prune: true
    selfHeal: true
    allowEmpty: false
  syncOptions:
    - CreateNamespace=true
    - PrunePropagationPolicy=foreground
    - PruneLast=true
  retry:
    limit: 5
    backoff:
      duration: 5s
      factor: 2
      maxDuration: 3m
```

**Applications updated**:
- admin-api-service.yaml
- user-org-service.yaml
- web-portal.yaml
- api-router-service.yaml
- analytics-service.yaml
- grafana.yaml
- loki.yaml
- istio.yaml (3 sub-applications)
- promtail.yaml
- monitoring-dashboards.yaml
- knative-config.yaml
- knative-serving.yaml (3 sub-applications)
- kserve.yaml
- kserve-config.yaml
- infrastructure-appset.yaml (development and production)

### 3. Branch Targeting

**Before**: Some Applications pointed to deleted feature branches (`001-infrastructure`, `020-model-management`)

**After**: All Applications follow convention:
- Development: `targetRevision: develop`
- Production: `targetRevision: main`

**Fixed files**:
- infrastructure-appset.yaml (development): `001-infrastructure` → `develop`
- infrastructure-appset.yaml (production): `001-infrastructure` → `main`

## Future Improvements

The following improvements are recommended for future implementation:

### Phase 2: Operational Excellence
- **ArgoCD Notifications**: Slack/email alerts for sync failures
- **Health Checks**: Custom Lua health checks for CRDs (KServe, Knative)
- **Metrics**: Prometheus metrics export for sync duration, error rates

### Phase 3: Security Hardening
- **SSO Integration**: ArgoCD RBAC with OIDC for user authentication
- **External Secrets**: Integrate with External Secrets Operator for secret management
- **Audit Logging**: Enable ArgoCD audit logs to Loki

### Phase 4: Advanced Deployment
- **Progressive Delivery**: Argo Rollouts for canary/blue-green deployments
- **ApplicationSets**: Convert similar Applications to use generators
- **Multi-Cluster**: Add staging cluster with promotion workflow

## Testing

Changes validated by:
1. ArgoCD UI sync status verification
2. `kubectl get application -n argocd` shows Synced/Healthy status
3. Application retry behavior tested by simulating transient failures

## Documentation Updated

- `docs/platform/ci-cd-pipeline.md` - Added ArgoCD Application Standards section
- `CLAUDE.md` - Added ArgoCD Application Requirements section
- `memory/constitution.md` - Added ArgoCD Application Requirements (v1.5.0 → v1.6.0)

## Constitution Gates

- [x] **API-First**: N/A (infrastructure only)
- [x] **Statelessness**: N/A (infrastructure only)
- [x] **Async Non-Critical**: N/A (infrastructure only)
- [x] **Security**: RBAC project restrictions implemented
- [x] **GitOps/Declarative**: All changes via Git, ArgoCD auto-sync
- [x] **Deployment**: Helm charts and ArgoCD Applications configured
- [x] **Observability**: N/A (covered by existing monitoring)
- [x] **Testing**: Manual validation in development cluster
- [x] **Performance**: N/A (infrastructure only)
