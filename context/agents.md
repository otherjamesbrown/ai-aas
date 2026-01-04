# AI-AAS Agent Rules

> **Last verified**: 2026-01-04 | **Commit**: 0c0f761b

---

## Beads Prefix

**Issue prefix is `aas-`** (e.g., `aas-1rk`, `aas-c1e`)

> **Note**: If you see `ai-aas-` prefix in old documentation or commits, the correct prefix is now `aas-`. The prefix was shortened on 2025-12-24 to fix a beads import bug.

---

## CRITICAL RULES

**NEVER:**
- Start work without a bead
- Close a bead with just "Done" - include commit hash and summary
- Work outside your agent domain without creating a handoff bead
- Use `kubectl apply/edit/patch` for permanent changes - use GitOps
- Access database directly from CLI/UI - use APIs
- Commit to main directly - use develop → staging → main
- Use libraries that download data at runtime (K8s pods have no internet)
- Use Knative/Serverless mode for GPU workloads - use RawDeployment (see Architecture below)
- Nest AIModel fields incorrectly - `deploymentMode` is `spec.deploymentMode` NOT `spec.deployment.deploymentMode`
- **Ship new code without unit tests** - all new functionality requires tests
- **Close a bead without running tests** - verify tests pass before marking complete

**ALWAYS:**
- Create or find a bead BEFORE writing code
- Update bead status: `bd update <id> --status in_progress`
- Reference bead in commits: `fix(component): description [aas-xxx]`
- Close bead with details: `bd close <id> --reason "IMPLEMENTED: commit <hash>, <summary>"`
- Create handoff beads for work outside your domain (with `agent:` label)
- **Write unit tests** for any new functionality you create
- **Run relevant unit tests** after completing coding work (use `go test ./...` in the affected package)

---

## Before Starting Work

```bash
# 1. Find existing bead or create new one
bd list --status open
bd ready                    # Find unblocked tasks
bd create "Title" --type bug|task|feature|epic

# 2. Verify bead has sufficient detail (see templates)
bd show <id>

# 3. Claim the work
bd update <id> --status in_progress
```

If bead lacks detail, add it using templates: `context/templates/beads.md`

---

## While Working

1. **Stay in your domain** - see Agent Domains below
2. **Document progress** - add comments to bead
3. **Follow principles:**
   - API-first: CLI/UI are thin clients, no business logic
   - GitOps-first: All infra changes via Git → ArgoCD
   - Bead-first: All work tracked in beads

---

## When Done

```bash
# 1. Commit with bead reference
git commit -m "fix(component): description [aas-xxx]"

# 2. Close bead with commit hash
bd close <id> --reason "IMPLEMENTED: commit abc1234, added retry logic"

# 3. Create handoff beads if needed
bd create "Handoff: <description>" --type task
bd update <new-id> --add-label "agent:target-agent"
```

---

## Tracking Context Gaps

When fixing a bug, ask: **"Was this caused by missing or stale context?"**

### Indicators of Context Gap

- Agent didn't know a rule existed
- Context doc said X but code does Y
- Pattern not documented, agent guessed wrong
- Anti-pattern not shown, agent made the mistake

### Tagging Context Gaps

```bash
# Add label to bug bead
bd update <id> --add-label "context-gap"

# In close reason, specify what was missing
bd close <id> --reason "IMPLEMENTED: commit abc123. CONTEXT-GAP: No anti-pattern for N+1 queries in go-services-developer/agents.md"
```

### Required Close Fields for Context Gaps

When closing a bead with `context-gap` label:

```
IMPLEMENTED: <commit hash>
CONTEXT-GAP: <what was missing>
CONTEXT-FILE: <which file needs update>
CONTEXT-FIX: <what to add - pattern, anti-pattern, rule>
```

### Finding Context Gaps

```bash
# Find all bugs caused by missing context
bd list --label "context-gap"

# Review for patterns - what's commonly missing?
bd list --label "context-gap" --status closed
```

### Closing the Loop

After fixing a context gap bug:
1. Add `context-gap` label to the bead
2. Close with CONTEXT-GAP details
3. Create follow-up task to update context doc:
   ```bash
   bd create "Update context: add N+1 anti-pattern" --type task
   bd update <new-id> --add-label "agent:go-services-developer"
   bd update <new-id> --add-label "context-update"
   ```

---

## Agent Domains

| Agent | Owns | Hand Off To |
|-------|------|-------------|
| `cli-developer` | services/ai-aas-cli/, services/ai-aas-org/ | go-services-developer (API issues), test-developer (UC tests) |
| `go-services-developer` | services/*-service/, shared/ | operator-developer (CRD), infra-ops-manager (Helm), test-developer (UC tests) |
| `operator-developer` | operators/, model-downloader/ | go-services-developer (API sync) |
| `infra-ops-manager` | infra/, gitops/, .github/, Helm charts | Developer agents (app code) |
| `web-portal-developer` | web/ | go-services-developer (API issues) |
| `test-developer` | tests/usecases/, tests/integration/ | go-services-developer (API bugs), cli-developer (CLI bugs) |
| `debugger` | Bug investigation (read-only) | Domain agents (fixes), context-maintainer (gaps) |
| `compliance-reviewer` | Drift detection (read-only) | Domain agents (fixes) |

**If file is outside your domain:** Create handoff bead, don't modify.

### Compliance Reviewer Agent

The `compliance-reviewer` is a specialized **read-only auditor** that:

- **Detects use case drift** - Code violating scope boundaries or missing AC coverage
- **Detects architecture drift** - Violations of API-first, GitOps principles
- **Produces recommendations** - Actionable handoffs for worker agents
- **Creates tracking beads** - For significant issues

**Invoke via**: `/review-compliance` or automatically after implementation (Stop hook)

**Specification**: `context/compliance-reviewer-agent.md`

**Key Rules for compliance-reviewer**:
- NEVER modify files - only analyze and report
- ALWAYS include file:line references for issues
- ALWAYS produce actionable recommendations
- Create beads for tracking (with user confirmation)

### Spawn Triggers

Create handoff bead and spawn agent when:

| Trigger | Spawn |
|---------|-------|
| API returns wrong data or missing endpoint | `go-services-developer` |
| Deployment failing, Helm/ArgoCD issue | `infra-ops-manager` |
| AIModel CR behavior wrong, CRD needs change | `operator-developer` |
| CLI needs new command or fix | `cli-developer` |
| Frontend needs API integration fix | `web-portal-developer` |
| CI/CD pipeline failing | `infra-ops-manager` |
| Database migration needed | `go-services-developer` |
| UC acceptance test needed or failing | `test-developer` |
| Contract test (CLI-to-API) needed | `test-developer` |
| Test fixture or harness work | `test-developer` |
| User asks "why did this happen?" | `debugger` |
| Bug is complex, recurring, or >30 min unresolved | `debugger` |
| Need root cause analysis before fix | `debugger` |
| Implementation complete, need compliance check | `compliance-reviewer` |
| Before PR merge, verify no drift | `compliance-reviewer` |
| Periodic architecture/UC audit | `compliance-reviewer` |

---

## Architecture: Model Deployment

### Deployment Modes

| Mode | Use For | Creates | Networking |
|------|---------|---------|------------|
| **RawDeployment** | GPU workloads (vLLM, Triton, TensorRT-LLM) | Deployment + ClusterIP Service | Direct HTTP |
| **Serverless** | CPU-only workloads (future, if ever) | Knative Service → Istio | Through Istio gateway |

**GPU workloads MUST use RawDeployment** because:
- Knative rejects `nodeSelector` (needed for GPU node scheduling)
- Knative has single-port restriction (Triton needs multiple)
- Scale-to-zero is counterproductive (5-10 min model load times)
- Istio/Knative routing adds complexity and failure modes

### AIModel CRD Fields

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
spec:
  deploymentMode: RawDeployment  # CORRECT - direct field
  # NOT spec.deployment.deploymentMode (wrong - will be ignored)
```

### What This Means for Routing

| Deployment Mode | Service Type | API Router Endpoint |
|-----------------|--------------|---------------------|
| RawDeployment | ClusterIP | `http://<model>-predictor.<ns>.svc.cluster.local:80` |
| Serverless | ExternalName → Istio | Times out (don't use) |


---

## Config Drift Anti-patterns

**Config drift** occurs when database/runtime configuration overrides GitOps-managed configuration, causing unexpected behavior.

### Source of Truth Hierarchy

| Priority | Source | Example |
|----------|--------|---------|
| 1 (highest) | GitOps Helm values | `values-development.yaml` |
| 2 | Environment variables | `BACKEND_ENDPOINTS` |
| 3 (lowest) | Database runtime config | Routing policies in Admin API |

**Problem**: When services sync configuration from the database, stale entries can override correct GitOps values.

### Symptoms

- Tests pass locally but fail in cluster
- "Connection refused" errors to `localhost` from cluster pods
- GitOps shows correct config, but runtime behavior differs
- Works after pod restart, fails after policy sync

### Real Incident (aas-88yt0)

**What happened**: UC tests failed with `dial tcp [::1]:8001: connect: connection refused`

**Root cause**: Routing policies in Admin API database contained `localhost:8001` URLs. The api-router-service synced these policies, overriding the correct cluster DNS names from Helm chart.

**Fix**: Updated routing policies to use cluster DNS (`<service>.<namespace>.svc.cluster.local`)

### Prevention Rules

**NEVER:**
- Store `localhost` URLs in database for cluster environments
- Store raw IP addresses in routing policies
- Allow database config to override GitOps without validation

**ALWAYS:**
- Use Kubernetes DNS pattern: `<service>.<namespace>.svc.cluster.local`
- Validate URLs at API layer (reject localhost for non-local environments)
- Prefer GitOps-only configuration over database-synced config
- Add startup validation that backend endpoints are reachable

### URL Validation Pattern

```go
// WRONG: Accept any URL
func (s *PolicyService) Create(ctx context.Context, p *Policy) error {
    return s.db.Create(p)
}

// CORRECT: Validate backend URLs
func (s *PolicyService) Create(ctx context.Context, p *Policy) error {
    if err := validateBackendURL(p.BackendURL, s.environment); err != nil {
        return err
    }
    return s.db.Create(p)
}

func validateBackendURL(url, env string) error {
    if env != "local" && strings.Contains(url, "localhost") {
        return fmt.Errorf("localhost URLs not allowed in %s environment", env)
    }
    // Warn on IP addresses, suggest DNS
    return nil
}
```

### Debugging Config Drift

```bash
# 1. Check GitOps config (Helm values)
cat services/<service>/deployments/helm/<service>/values-development.yaml

# 2. Check deployed config (env vars)
kubectl get deployment <name> -n <ns> -o jsonpath='{.spec.template.spec.containers[0].env}'

# 3. Check runtime config (database)
# Query Admin API for routing policies, compare to GitOps

# 4. If mismatch found: database is drifted, update via API
```
---

## Quick Commands

```bash
# Beads
bd list --status open          # Open issues
bd ready                       # Unblocked tasks
bd show <id>                   # Issue details
bd update <id> --status in_progress
bd close <id> --reason "IMPLEMENTED: commit <hash>, <summary>"

# Labels
bd update <id> --add-label "agent:go-services-developer"
bd update <id> --add-label "component:api-router"
bd update <id> --add-label "env:development"
bd update <id> --add-label "context-gap"      # Bug caused by missing context
bd update <id> --add-label "context-update"   # Task to fix context docs
```

---

## Anti-patterns

```bash
# WRONG: No bead
git checkout -b fix-something
git commit -m "fix: something"

# WRONG: Closing without details
bd close aas-xxx --reason "Done"

# WRONG: kubectl for permanent changes
kubectl apply -f deployment.yaml

# WRONG: Direct to main
git push origin main
```

---

## Completion Checklist

Before reporting complete:
- [ ] Bead exists
- [ ] Root cause analysis done (bugs)
- [ ] **Unit tests written** for new functionality (required)
- [ ] **Unit tests executed** and passing (`go test ./...` in affected packages)
- [ ] Commits reference bead ID
- [ ] Bead closed with commit hash
- [ ] Handoff beads created if needed

### Testing Requirements

**New functionality MUST have tests:**
```bash
# After writing code, create corresponding *_test.go file
# Use table-driven tests for multiple cases
# Test both success and error paths

# Run tests in the affected package
go test -v ./path/to/package/...

# Run tests with coverage report
go test -cover ./path/to/package/...
```

**Do NOT close a bead if:**
- New code has no corresponding tests
- Tests are failing
- Tests weren't executed

---

## Report Format

```markdown
**Bead**: aas-xxx (closed)

**Summary**: What was accomplished

**Commits**: `abc1234`: description [aas-xxx]

**Files Changed**: path/to/file.go

**Handoffs**: Beads created or "None"
```

---

## Reference Documents

| What | Where |
|------|-------|
| Architecture | `ARCHITECTURE.md` |
| Full agent context | `context/<agent>/agents.md` |
| Bead templates | `context/templates/beads.md` |
| Environment access | `docs/platform/environment-access.md` |
| Debugging | `docs/runbooks/ai-debugging-workflow.md` |
| Deployment | `docs/runbooks/deploy-to-environments.md` |
| All runbooks | `docs/runbooks/` (18 available) |
| Use case schema | `usecases/SCHEMA.md` |
| Use case workflow | `CLAUDE.md#use-case-driven-development` |
| Compliance reviewer | `context/compliance-reviewer-agent.md` |
| E2E testing | `context/e2e-testing/agents.md` |
| Test developer | `context/test-developer/agents.md` |
