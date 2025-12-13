# AI-AAS Agent Rules

> **Last verified**: 2025-12-13 | **Commit**: 24c3e0ee

---

## CRITICAL RULES

**NEVER:**
- Start work without a bead
- Close a bead with just "Done" - include commit hash and summary
- Work outside your agent domain without creating a handoff bead
- Use `kubectl apply/edit/patch` for permanent changes - use GitOps
- Access database directly from CLI/UI - use APIs
- Commit to main directly - use develop → staging → main

**ALWAYS:**
- Create or find a bead BEFORE writing code
- Update bead status: `bd update <id> --status in_progress`
- Reference bead in commits: `fix(component): description [ai-aas-xxx]`
- Close bead with details: `bd close <id> --reason "IMPLEMENTED: commit <hash>, <summary>"`
- Create handoff beads for work outside your domain (with `agent:` label)

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
git commit -m "fix(component): description [ai-aas-xxx]"

# 2. Close bead with commit hash
bd close <id> --reason "IMPLEMENTED: commit abc1234, added retry logic"

# 3. Create handoff beads if needed
bd create "Handoff: <description>" --type task
bd update <new-id> --add-label "agent:target-agent"
```

---

## Agent Domains

| Agent | Owns | Hand Off To |
|-------|------|-------------|
| `cli-developer` | services/ai-aas-cli/ | go-services-developer (API issues) |
| `go-services-developer` | services/*-service/, shared/ | operator-developer (CRD), infra-ops-manager (Helm) |
| `operator-developer` | operators/, model-downloader/ | go-services-developer (API sync) |
| `infra-ops-manager` | infra/, gitops/, .github/, Helm charts | Developer agents (app code) |
| `web-portal-developer` | web/ | go-services-developer (API issues) |

**If file is outside your domain:** Create handoff bead, don't modify.

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
```

---

## Anti-patterns

```bash
# WRONG: No bead
git checkout -b fix-something
git commit -m "fix: something"

# WRONG: Closing without details
bd close ai-aas-xxx --reason "Done"

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
- [ ] Tests added/updated
- [ ] Commits reference bead ID
- [ ] Bead closed with commit hash
- [ ] Handoff beads created if needed

---

## Report Format

```markdown
**Bead**: ai-aas-xxx (closed)

**Summary**: What was accomplished

**Commits**: `abc1234`: description [ai-aas-xxx]

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
