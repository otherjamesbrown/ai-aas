# Claude Rules for AI-AAS Platform

This document provides a set of specific, critical rules for interacting with the AI-AAS Platform repository.

For a general overview of the architecture, development workflow, and key documents, please first read the main guide:

➡️ **[AI_ASSISTANT_GUIDE.md](./AI_ASSISTANT_GUIDE.md)**

After reading the main guide, adhere to the following critical rules below.

## Core Principles

In addition to the principles outlined in the main guide, always adhere to:

2.  **GitOps-First Deployment**: ALWAYS use GitOps for infrastructure and deployment changes. Never make direct changes to Kubernetes clusters. All changes must go through: edit → commit → push → ArgoCD sync.

## Environment Access & Credentials

**CRITICAL**: Before searching for credentials or environment access information, ALWAYS check this document first:

📖 **[docs/platform/environment-access.md](docs/platform/environment-access.md)** - Complete environment access guide

This document contains:
- Kubernetes cluster access (kubeconfigs, contexts)
- ArgoCD URLs and credentials
- Database connection strings
- API endpoints and ingress IPs
- API keys and authentication tokens
- Admin CLI configuration
- SSH keys and infrastructure tokens
- Port-forwarding commands
- Troubleshooting common access issues

**Quick Access Examples:**
- Kubernetes: `kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml`
- Database: Connection string in `secrets/env/.env` as `DATABASE_URL`
- API Router: `https://api.172.232.58.222.nip.io` or `https://api.dev.ai-aas.local`
- Master Admin API Key: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY`
- ArgoCD: `https://argocd.dev.ai-aas.local` (password retrieved from k8s secret)

## Endpoint URL Management Strategy

**CRITICAL**: All Kubernetes service endpoints must be managed consistently across the platform.

### Internal Cluster Services (vLLM, Redis, etcd, etc.)

**DO commit to source control** - These are infrastructure definitions that should be versioned.

**Best Practices:**
1. **Use Kubernetes DNS patterns**: `service-name.namespace.svc.cluster.local`
2. **Define in ONE place**: Use Helm chart `values-<environment>.yaml` files only
3. **Avoid inline values in ArgoCD Applications**: Remove `values:` section from ArgoCD Application manifests
4. **Reference via valueFiles**: Let ArgoCD load from `valueFiles: [values-development.yaml]`

**Example - Correct approach:**
```yaml
# services/api-router-service/deployments/helm/api-router-service/values-development.yaml
backends:
  - name: vllm-gpt-oss-20b
    serviceName: gpt-oss-20b-vllm-deployment  # Matches actual Kubernetes Service name
    namespace: system
    port: 8000
    path: /v1/chat/completions
```

**Example - Avoid this:**
```yaml
# gitops/clusters/development/apps/api-router-service.yaml
spec:
  source:
    helm:
      valueFiles:
        - values-development.yaml
      values: |  # ❌ Avoid inline values - creates duplication
        backends:
          endpoints: "..."
```

### External Endpoints (LoadBalancer IPs, Third-party APIs)

**DO NOT commit to source control** - These change on redeployment or provider updates.

**Recommended approaches:**
1. **ConfigMaps created imperatively:**
   ```bash
   kubectl create configmap external-endpoints \
     --from-literal=loadbalancer-ip=172.232.58.222
   ```
2. **External secret management**: Use tools like Sealed Secrets, External Secrets Operator, or Vault
3. **Reference in Helm charts:**
   ```yaml
   envFrom:
     - configMapRef:
         name: external-endpoints
   ```

### Verification Commands

```bash
# Find actual Kubernetes service names
kubectl get svc -A | grep <service-pattern>

# Verify deployed configuration
kubectl get deployment <name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="BACKEND_ENDPOINTS")].value}'
```

## GitOps Deployment Workflow

**CRITICAL**: All infrastructure and Kubernetes resource changes MUST follow this workflow. Never use `kubectl apply`, `kubectl edit`, or `kubectl patch` for permanent changes.

### Correct Workflow:

1.  **Make changes locally**: Edit Helm charts, Kubernetes manifests, or ArgoCD applications in the git repository
2.  **Test locally** (optional): Validate with `helm template`, `kubectl diff`, or `make check`
3.  **Commit changes**: `git add . && git commit -m "description"`
4.  **Push to repository**: `git push origin main` (or feature branch)
5.  **ArgoCD syncs automatically** (development) or **manually sync** (production): `argocd app sync <app-name>`
6.  **Verify deployment**: Check application status with `kubectl get pods` or `argocd app get <app-name>`

### Reference Documentation:

- `docs/runbooks/deploy-to-environments.md`: Complete deployment runbook
- ArgoCD endpoints: `argocd.dev.ai-aas.local`, `argocd.prod.ai-aas.local`
