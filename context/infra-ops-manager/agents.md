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

  kserve_access:
    rawdeployment: "Internal only - external returns 404, route via API Router"
    serverless: "Gets external VirtualService via Istio"

  environment_access:
    reference: docs/platform/environment-access.md
    kubeconfig: "~/kubeconfigs/kubeconfig-development.yaml"

  cert_manager_webhooks:
    rule: Use ignoreDifferences for cert-manager caBundle injection
    why: ArgoCD v3 detects cert-manager's runtime injection as drift
    pattern: |
      ignoreDifferences:
        - group: admissionregistration.k8s.io
          kind: ValidatingWebhookConfiguration
          jqPathExpressions:
            - .webhooks[]?.clientConfig.caBundle
        - group: admissionregistration.k8s.io
          kind: MutatingWebhookConfiguration
          jqPathExpressions:
            - .webhooks[]?.clientConfig.caBundle
    applies_to:
      - Istio (istiod-istio-system webhook)
      - KServe (inferenceservice.serving.kserve.io webhook)
      - GPU Operator (nvidia.com webhooks)
      - Any operator using cert-manager for webhook certificates

  argocd_sync_waves:
    rule: Use sync waves for CRD-dependent resources
    why: Prevents race conditions where resources deploy before CRDs are ready
    pattern: |
      metadata:
        annotations:
          argocd.argoproj.io/sync-wave: "0"  # CRDs and operators
          argocd.argoproj.io/sync-wave: "1"  # Custom resources using those CRDs
    critical_for:
      - Istio: Base CRDs (wave 0) → Istio control plane (wave 1) → Gateway/VirtualService (wave 2)
      - KServe: KServe operator (wave 0) → InferenceService CRs (wave 1)
      - GPU Operator: Operator (wave 0) → GPU configurations (wave 1)
    symptom_without: "unable to recognize... no matches for kind"
    note: Lower wave numbers deploy first, default is 0
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

# WRONG: Not ignoring cert-manager caBundle injection
# ArgoCD v3 detects cert-manager's caBundle injection as drift
# This causes constant OutOfSync status for webhooks
# Must use ignoreDifferences to ignore the injected field

# WRONG: Duplicate environment variables in Helm charts
# Defining same env var in BOTH values file AND deployment template
# values.yaml:
env:
  - name: LOG_LEVEL
    value: info
# templates/deployment.yaml (also defines LOG_LEVEL):
env:
  - name: LOG_LEVEL
    valueFrom:
      configMapKeyRef:
        name: config
        key: log-level
# This creates duplicate env vars - ArgoCD validation rejects this!
# FIX: Define in ONE place only (values OR template, never both)
# Related bug: aas-exfp

# WRONG: Kustomize commonLabels affecting immutable selectors
# Kustomize commonLabels applies to ALL label fields including Deployment selectors
kustomization.yaml:
  commonLabels:
    app.kubernetes.io/part-of: observability  # BREAKS: modifies immutable selectors!
# FIX: Use commonAnnotations instead (annotations are mutable):
  commonAnnotations:
    app.kubernetes.io/part-of: observability
# OR: Use patches to target pod template labels only
# Related bug: aas-7yh3

# WRONG: TGI VLM models without PREFIX_CACHING=0
# TGI v2.0+ enables prefix caching by default, but VLMs don't support it
# Symptom: Pod crashes with "NotImplementedError: Vlm do not work with prefix caching yet"
# AIModel CR missing runtimeEnv:
spec:
  runtime: tgi
  modelName: Qwen/Qwen2-VL-7B-Instruct
  # Missing: runtimeEnv with PREFIX_CACHING=0
# FIX: Add runtimeEnv to disable prefix caching for VLMs:
spec:
  runtime: tgi
  runtimeEnv:
    - name: PREFIX_CACHING
      value: "0"
# Applies to: Qwen2-VL, Gemma-3-Vision, LLaVA, any TGI VLM
# Related bug: aas-t8gr
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
  jq -r '.items[] | .metadata.name as $name | .status.conditions[]? | select(.message | test("part of applications"; "i")) | "\($name): \(.message)"'

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
