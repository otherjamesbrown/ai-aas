# Claude Rules for AI-AAS Platform

This document provides a set of specific, critical rules for interacting with the AI-AAS Platform repository.

For a general overview of the architecture, development workflow, and key documents, please first read the main guide:

➡️ **[AI_ASSISTANT_GUIDE.md](./AI_ASSISTANT_GUIDE.md)**

After reading the main guide, adhere to the following critical rules below.

## Backlog & Task Tracking

**IMPORTANT**: We maintain a project backlog to track ideas, improvements, and tasks.

📋 **[specs/BACKLOG.md](specs/BACKLOG.md)** - Platform backlog and scratch list

**When ending a session or completing work:**
- Ask the user: "Would you like to add anything to the backlog before we finish?"
- If the user mentions something to do "later" or "eventually", offer to add it to the backlog
- When discovering issues or improvement opportunities, suggest adding them to the backlog

**When starting a session:**
- If relevant to the task, check the backlog for related items
- Offer to pick up items from the backlog if the user is looking for things to work on

## CLI-First Operations

**IMPORTANT**: When performing platform operations, ALWAYS prefer the CLI over direct API calls or kubectl commands.

### Why CLI-First?

- **Consistency**: CLI provides validated, tested workflows
- **Documentation**: CLI `--help` shows proper usage and next steps
- **Auditing**: CLI operations are logged consistently
- **Error handling**: CLI provides actionable error messages

### Required Approach

1. **Check if CLI supports the operation** before using kubectl or API calls:
   ```bash
   ai-aas-cli --help                    # See all commands
   ai-aas-cli model --help              # See model commands
   ai-aas-cli model deploy create --help # See specific command
   ```

2. **Use CLI for these operations**:
   - Model management: `ai-aas-cli model registry/cache/deploy/troubleshoot`
   - Organizations: `ai-aas-cli org list/create/update/delete`
   - Users: `ai-aas-cli user list/create/update/delete`
   - API Keys: `ai-aas-cli apikey list/create/delete`
   - Credentials: `ai-aas-cli credentials set/list/test`
   - Status checks: `ai-aas-cli status`

3. **If CLI doesn't support an operation**:
   - Note this as a gap: "CLI doesn't support X - consider adding to backlog"
   - Use the Admin API as fallback
   - Suggest the command that should exist

### Example Workflow

```bash
# CORRECT: Use CLI for model deployment
ai-aas-cli model registry add meta-llama/Llama-2-7b-hf --name llama-7b
ai-aas-cli model cache pull llama-7b
ai-aas-cli model deploy create llama-7b -e development

# AVOID: Direct kubectl for routine operations
# kubectl apply -f deployment.yaml  # Only for infrastructure changes via GitOps
```

## Core Principles

In addition to the principles outlined in the main guide, always adhere to:

1.  **API-First Interfaces (from Constitution)**: All functionality MUST be exposed via REST APIs first.
    - **CLI and Web UI are thin clients** - they MUST NOT contain business logic
    - **No direct database access** from CLI or UI - always go through the Admin API
    - When implementing CLI commands, use the existing `internal/api` and `internal/registry` clients
    - Example pattern:
      ```go
      // CORRECT: Use API client
      apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey, opts...)
      regClient := registry.NewClient(apiClient)
      model, err := regClient.Get(ctx, modelName)

      // WRONG: Direct database access
      db, err := sql.Open("postgres", cfg.DatabaseURL)
      rows, err := db.Query("SELECT * FROM models")
      ```

2.  **GitOps-First Deployment**: ALWAYS use GitOps for infrastructure and deployment changes. Never make direct changes to Kubernetes clusters. All changes must go through: edit → commit → push → ArgoCD sync.

3.  **Reuse Existing Components**: Before implementing new functionality:
    - Check for existing clients in `internal/api`, `internal/registry`, `internal/kubernetes`
    - Check if the Admin API already has the endpoint you need
    - If an API endpoint is missing, add it to Admin API first, then use it from CLI

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

## Service Creation Requirements

**CRITICAL**: When creating a new deployable service, you MUST include the following:

### Required Components

1. **Helm Chart** at `services/<service-name>/deployments/helm/<service-name>/`:
   - `Chart.yaml` - Chart metadata
   - `values.yaml` - Default values
   - `values-development.yaml` - Development environment values (optional)
   - `templates/deployment.yaml` - Deployment with health probes
   - `templates/service.yaml` - Service definition
   - `templates/serviceaccount.yaml` - Service account

2. **ArgoCD Application** at `gitops/clusters/<env>/apps/<service-name>.yaml`:
   - References the Helm chart path
   - Configures sync policy and destination namespace
   - See existing applications (e.g., `api-router-service.yaml`) as templates

3. **Health Probes** (mandatory in Helm chart):
   ```yaml
   livenessProbe:
     httpGet:
       path: /health
       port: http
     initialDelaySeconds: 10
     periodSeconds: 10
   readinessProbe:
     httpGet:
       path: /ready
       port: http
     initialDelaySeconds: 5
     periodSeconds: 5
   ```

### Not Sufficient for Production

- Raw Kubernetes manifests in `services/<service-name>/k8s/` are **NOT** sufficient
- Services without ArgoCD Applications will NOT be deployed or persistent
- Refer to Constitution v1.5.0 for full requirements

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

## ArgoCD Application Requirements

**CRITICAL**: All ArgoCD Applications MUST follow these standards for consistency and reliability.

### Required Sync Policy

Every new or modified Application MUST include these sync options:

```yaml
syncPolicy:
  automated:
    prune: true        # Remove resources not in Git
    selfHeal: true     # Revert manual changes
    allowEmpty: false  # Prevent accidental deletion
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

### Branch Targeting Rules

📖 **[docs/development/branching-workflow.md](docs/development/branching-workflow.md)** - Complete branching workflow guide

The platform uses a three-branch promotion workflow:

```
develop → staging → main
```

| Branch | ArgoCD Target | Environment |
|--------|---------------|-------------|
| `develop` | development | Fast iteration |
| `staging` | staging | Code review & testing |
| `main` | production | Production-ready |

**Key Rules:**
- **Development apps**: Target `develop` branch (`targetRevision: develop`)
- **Staging apps**: Target `staging` branch (`targetRevision: staging`)
- **Production apps**: Target `main` branch (`targetRevision: main`)
- PRs to `staging` must come from `develop`
- PRs to `main` must come from `staging`
- **NEVER** reference feature branches in Applications - they may be deleted

### RBAC Project Requirements

Applications MUST be assigned to an AppProject with restrictive policies:

- **Explicit destinations**: List specific namespaces, never use wildcard `*`
- **Explicit sourceRepos**: Only allow specific repository URLs
- **clusterResourceWhitelist**: Only allow required cluster-scoped resources
- See `gitops/clusters/development/projects/platform-project.yaml` as reference

### Application Template

Use this template for new Applications:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: <service-name>-<environment>
  namespace: argocd
  labels:
    environment: <environment>
    app: <service-name>
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: platform-<environment>
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: develop  # or main for production
    path: services/<service-name>/deployments/helm/<service-name>
    helm:
      valueFiles:
        - values-<environment>.yaml
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
