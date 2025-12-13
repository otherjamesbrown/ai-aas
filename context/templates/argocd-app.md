# ArgoCD Application Template

Use this template when creating new ArgoCD Applications.

---

## Standard Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: <service>-<env>
  namespace: argocd
  labels:
    environment: <env>
    app: <service>
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: platform-<env>
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: <branch>  # develop|staging|main
    path: services/<name>/deployments/helm/<name>
    helm:
      valueFiles:
        - values-<env>.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: <namespace>
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

---

## Branch Targeting

| Environment | Branch | targetRevision |
|-------------|--------|----------------|
| development | develop | `develop` |
| staging | staging | `staging` |
| production | main | `main` |

**NEVER use feature branches in targetRevision.**

---

## Required Elements

```yaml
required:
  metadata:
    - name: "<service>-<env>" format
    - namespace: argocd
    - finalizers: resources-finalizer.argocd.argoproj.io

  spec:
    - project: "platform-<env>"
    - targetRevision: "develop|staging|main (not feature branches)"
    - helm.valueFiles: "values-<env>.yaml (not inline values)"

  syncPolicy:
    - automated.prune: true
    - automated.selfHeal: true
    - retry with backoff
```

---

## Anti-patterns

```yaml
# WRONG: Inline values (creates duplication)
spec:
  source:
    helm:
      values: |
        replicas: 3

# WRONG: Feature branch
spec:
  source:
    targetRevision: feature/my-branch

# WRONG: Missing finalizers (orphaned resources)
metadata:
  name: my-app
  # No finalizers!
```

---

## Examples

See existing applications:
- `gitops/clusters/development/apps/admin-api-service.yaml`
- `gitops/clusters/development/apps/api-router-service.yaml`
- `gitops/clusters/development/apps/ai-model-operator.yaml`
