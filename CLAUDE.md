# Claude Rules for AI-AAS Platform

This document provides Claude-specific configuration for the AI-AAS Platform repository.

## Related Documents

| Document | Purpose |
|----------|---------|
| **[context/agents.md](./context/agents.md)** | Core agent rules (NEVER/ALWAYS, beads workflow, domains) |
| **[context/context_map.md](./context/context_map.md)** | Navigation index for all context docs |
| **[AI_ASSISTANT_GUIDE.md](./AI_ASSISTANT_GUIDE.md)** | Onboarding guide (repo structure, CLI commands) |
| **[ARCHITECTURE.md](./ARCHITECTURE.md)** | System architecture (YAML format) |
| **[usecases/SCHEMA.md](./usecases/SCHEMA.md)** | Use case YAML schema and examples |

> **Note**: Rules in `context/agents.md` apply to ALL AI agents. This file contains Claude-specific extensions.

## Issue Tracking with Beads

**IMPORTANT**: We use beads for issue tracking. All tasks, bugs, and features are tracked as beads issues.

**Common beads commands:**
```bash
bd list --status open              # List open issues
bd list --priority 1               # List high priority issues
bd show <issue-id>                 # Show issue details
bd create "Title" --type feature   # Create new issue
bd update <issue-id> --status in_progress  # Update status
bd close <issue-id>                # Close an issue
```

**Bead ID shorthand:**
- Prefix `aas-` can be omitted when referencing beads
- "spec030" → aas-spec030
- "pr93" → aas-pr93

**When ending a session or completing work:**
- Ask the user: "Would you like to create any beads issues before we finish?"
- If the user mentions something to do "later" or "eventually", offer to create a beads issue

**When starting a session:**
- Use `bd list --status open` to check for relevant issues
- Use `bd ready` to find tasks with no blockers

### Tracking Discovered Work

**CRITICAL**: When working on a bead and you discover a follow-on bug or issue:

1. **Create a new bead immediately** - Don't just fix it inline
2. **Link to the parent bead** using `bd dep add <new-bead> <parent-bead>`
3. **Use descriptive titles** that reference the discovery context

```bash
# Example: Discovered tensor shape issue while fixing aas-ih6c
bd create --title="TRT-LLM tensor shape: add batch dimension" --type=bug --priority=1
bd dep add <new-bead-id> aas-ih6c  # New bead depends on (was discovered from) parent
```

**Why this matters:**
- Creates audit trail for debugging decisions
- Helps future developers understand why changes were made
- Enables proper root cause analysis across related issues
- Prevents "mystery fixes" that lack context

## Use Case Driven Development

**CRITICAL**: All new features and modifications to existing features MUST be driven by use cases.

📖 **Schema Reference**: [usecases/SCHEMA.md](usecases/SCHEMA.md) - Complete YAML schema

### Why Use Cases?

- **Prevent context drift** - Clear scope boundaries prevent agents from adding/removing features accidentally
- **Testable contracts** - Each acceptance criterion becomes a test
- **Traceability** - Bugs map to use cases, revealing weak acceptance criteria

### Workflow Integration

Use cases fit into the JB workflow after spec:

```
jb-3.1-specify → jb-3.1b-usecases → jb-3.2-impact → jb-3.3-plan → jb-3.4-tasks
```

- Spec defines WHAT and WHY (requirements)
- Use cases define HOW TO VERIFY (acceptance criteria)
- Specs reference use cases instead of containing acceptance criteria

### Before Implementation: Context Loading

**REQUIRED**: Before implementing any UC, you MUST:

1. **Read the use case file**:
   ```bash
   cat usecases/<feature>.yaml
   ```

2. **Output the structured context block**:
   ```markdown
   ## Implementation Context
   **Use Case**: UC-BM-001 - Create Benchmark Target
   **Acceptance Criteria**: AC-01, AC-02, AC-03
   **In Scope**: Creating target, validation, returning ID
   **Out of Scope**: Starting execution, modifying targets, creating scenarios
   **Must NOT**: Auto-start benchmark, modify existing targets, expose internal metrics
   ```

3. **Write failing tests FIRST** (one subtest per AC)

4. **Implement until tests pass**

5. **STOP** - Do not add anything not in acceptance criteria

### Test Naming Convention

Tests must reference UC and AC IDs:

```go
func TestUC_BM_001_CreateBenchmarkTarget(t *testing.T) {
    t.Run("AC-01: create target with required fields", func(t *testing.T) {
        // Given: authenticated org admin
        client := setupAuthenticatedClient(t)

        // When: create benchmark target
        target, err := client.CreateBenchmarkTarget(ctx, &CreateTargetRequest{
            Model:    "llama-7b",
            Scenario: "standard",
        })

        // Then: target is created
        require.NoError(t, err)
        assert.NotEmpty(t, target.ID)
    })

    t.Run("AC-02: reject unauthorized model", func(t *testing.T) {
        // ...
    })
}
```

### After Implementation: Drift Review

**REQUIRED**: Before marking a UC as complete, you MUST verify no drift occurred:

```markdown
## Drift Review for UC-BM-001

### Changes Made
1. Added `CreateBenchmarkTarget` function in `internal/cmd/benchmark.go`
   → Maps to AC-01
2. Added model access validation in `internal/api/benchmark.go`
   → Maps to AC-02
3. Added scenario validation in `internal/api/benchmark.go`
   → Maps to AC-03

### Verification
- [ ] All code changes map to an acceptance criterion
- [ ] No out_of_scope work was done
- [ ] No must_not violations occurred
- [ ] All AC subtests pass

### Unmapped Code (potential drift)
None
```

If any code doesn't map to an AC, either:
- Remove it (if it's drift)
- Add a new AC to the UC (if it's a legitimate discovery - requires UC update in same PR)

### Partial Implementation

If a UC spans multiple sessions:

1. Document progress in bead comment:
   ```bash
   bd comments add aas-ucbm001 "Partial: AC-01, AC-02 complete. Remaining: AC-03, AC-04"
   ```

2. Create handoff note in PR or bead:
   ```markdown
   ## Handoff: UC-BM-001
   Completed: AC-01, AC-02 (tests passing)
   Remaining: AC-03 (error handling), AC-04 (edge cases)
   Next steps: Implement scenario validation
   ```

### Bug Attribution to Use Cases

**CRITICAL**: When creating bug beads, ALWAYS attribute them to the relevant use case(s).

**Workflow:**
1. Create the bug bead
2. Add use case label: `bd label add <bug-id> uc:UC-BM-001`
3. If the bug reveals an AC gap, note it in comments

**Example:**
```bash
# Bug discovered: benchmark target accepts invalid scenarios
bd create --title="Benchmark target accepts non-existent scenario" --type=bug --priority=2
bd label add aas-xyz uc:UC-BM-001
bd comments add aas-xyz "AC Gap: UC-BM-001/AC-03 should verify scenario exists before creation"
```

**Querying bugs by use case:**
```bash
bd list --label=uc:UC-BM-001 --type=bug        # All bugs for UC-BM-001
bd list --label=uc:UC-BM-001 --status=open     # Open bugs for UC-BM-001
```

### Backfill Strategy

- **New features**: UC required before implementation
- **Modifying existing features**: Create UC first (backfill on touch)
- **Bug fixes**: Create UC for the affected feature if none exists

### UC Dependency Order

- Dependencies between UCs are tracked in the YAML
- You may implement UCs in any order during development
- BUT: Dependency UC tests must pass before marking a UC complete
- Run `./scripts/uc-deps.sh UC-BM-002` to see dependency tree

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
   - Note this as a gap: "CLI doesn't support X - consider creating a beads issue"
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

### Building CLI Binaries

To rebuild both CLI binaries (ai-aas-cli and ai-aas-org):

```bash
# Build only (binaries stay in source directories)
./scripts/build-clis.sh

# Build and install to ~/.local/bin
./scripts/build-clis.sh --install

# After installing, refresh shell cache
hash -r
```

**Binary locations:**
- Source: `services/ai-aas-cli/ai-aas-cli`, `services/ai-aas-org/ai-aas-org`
- Installed: `~/.local/bin/ai-aas-cli`, `~/.local/bin/ai-aas-org`

**When to rebuild:**
- After pulling new changes that modify CLI code
- After switching branches/worktrees
- If CLI commands are missing or behaving unexpectedly

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

## Workspace & Git Worktree Setup

The development machine uses a **workspace** bash function (defined in `~/.bashrc`) to manage git worktrees for parallel development:

```bash
workspace <name> [branch]    # Create/switch to a worktree
workspace                    # List current worktrees
workspace-remove <name>      # Clean up a worktree
```

**Key details:**
- Main repository: `~/ai-aas`
- Worktrees created in: `~/worktrees/<name>`
- Permanent worktrees: `develop`, `staging`
- Temporary worktrees: feature branches, PR reviews

### Git-Crypt

The repository uses **git-crypt** to encrypt sensitive files (`.env`, secrets).

- **Key location**: `~/.config/git-crypt/ai-aas-key`
- The workspace function auto-unlocks git-crypt when creating new worktrees
- If you encounter git-crypt errors, manually unlock:
  ```bash
  cd ~/ai-aas && git-crypt unlock ~/.config/git-crypt/ai-aas-key
  ```

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
- API Router: `https://api.dev.otherjamesbrown.com`
- Master Admin API Key: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY`
- ArgoCD: `https://argocd.dev.otherjamesbrown.com` (password retrieved from k8s secret)

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
- ArgoCD endpoints: `argocd.dev.otherjamesbrown.com`, `argocd.prod.otherjamesbrown.com`

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

📖 **[docs/platform/branching-workflow.md](docs/platform/branching-workflow.md)** - Complete branching workflow guide

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

## Debugging & Observability

**CRITICAL**: When debugging issues, use the observability stack. Don't guess - look at the logs and traces.

📖 **Full Guide**: [docs/runbooks/ai-debugging-workflow.md](docs/runbooks/ai-debugging-workflow.md)

### Quick Reference

| What | Command/URL |
|------|-------------|
| **Grafana** | https://grafana.dev.otherjamesbrown.com |
| **Loki API** | https://loki.dev.otherjamesbrown.com |
| **Service Logs Dashboard** | Grafana → Dashboards → Service Logs |
| **Request Tracing Dashboard** | Grafana → Dashboards → Request Tracing |

### Common Debug Commands

```bash
# View recent errors for a service
kubectl logs -n <namespace> -l app=<service> --tail=100 | grep -i error

# Query Loki directly for errors (last hour)
curl -G https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range \
  --data-urlencode 'query={service="api-router-service",level="error"}' \
  --data-urlencode 'limit=50'

# Find logs by trace ID
curl -G https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range \
  --data-urlencode 'query={trace_id="<TRACE_ID>"}'

# View vLLM/inference backend logs
kubectl logs -n system -l serving.kserve.io/inferenceservice=<model> --tail=100

# Check for GPU/CUDA errors
curl -G https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range \
  --data-urlencode 'query={namespace="system"} |~ "(?i)cuda|oom|gpu"'
```

### Log Format

All Go services output structured JSON logs:
```json
{
  "level": "error",
  "ts": "2025-12-12T10:30:00Z",
  "msg": "request failed",
  "service": "api-router-service",
  "trace_id": "abc123",
  "request_id": "req-456",
  "error": "connection refused"
}
```

**Key fields for filtering**:
- `service` - Service name (api-router-service, admin-api-service, etc.)
- `level` - Log level (debug, info, warn, error)
- `trace_id` - Distributed trace ID (correlates requests across services)
- `request_id` - Unique request identifier
- `error` - Error message (when level=error)

### Debug Workflow

1. **Identify the error** - Get the error message or trace_id from the user/logs
2. **Check service logs** - Use Grafana or kubectl to view recent logs
3. **Correlate with trace_id** - Find all logs for a specific request across services
4. **Check dashboards** - Look at error rates, latency spikes
5. **Check alerts** - See if any alerts fired around the time of the issue

### Frontend Errors

Frontend errors are captured by Sentry. Check:
- Sentry dashboard for React errors with stack traces
- Session replay to see user actions leading to errors
- Error ID displayed to users correlates to Sentry event
