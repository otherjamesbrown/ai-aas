# AI-AAS Development Workflow

> A guide to the spec-driven, use-case-verified, agent-assisted development workflow.

## Overview

This workflow ensures:
- **Clarity**: Requirements are captured before code
- **Traceability**: Every feature maps to specs and tests
- **Quality**: Drift is detected and prevented
- **Context preservation**: Knowledge persists across sessions

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         DEVELOPMENT LIFECYCLE                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   DEFINE                    IMPLEMENT                    MAINTAIN       │
│   ──────                    ─────────                    ────────       │
│                                                                         │
│   ┌──────────┐             ┌──────────┐                ┌──────────┐    │
│   │   Spec   │────────────▶│  Code +  │───────────────▶│Use Cases │    │
│   │  (temp)  │             │  Tests   │                │(living)  │    │
│   └──────────┘             └──────────┘                └──────────┘    │
│        │                        │                           │          │
│        │                        │                           │          │
│        ▼                        ▼                           ▼          │
│   ┌──────────┐             ┌──────────┐                ┌──────────┐    │
│   │   Use    │             │  Beads   │                │Compliance│    │
│   │  Cases   │             │ Tracker  │                │  Review  │    │
│   └──────────┘             └──────────┘                └──────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## The Lifecycle

### 1. Specs Are Temporary

Specs (`specs/NNN-feature/spec.md`) capture:
- **What** we're building and **why**
- Requirements, constraints, decisions
- Architectural approach

After implementation, specs are **archived** to `.archive-specs/`. They served their purpose - the requirements are now embodied in code and use cases.

### 2. Use Cases Are Living Documentation

Use cases (`usecases/*.yaml`) define:
- **Acceptance criteria** - testable requirements
- **Scope boundaries** - what's in/out of scope
- **Anti-requirements** - what we must NOT do

Use cases **persist forever** and:
- Drive test implementation (one test per AC)
- Prevent context drift during implementation
- Attribute bugs to specific requirements

### 3. Tests Are the Source of Truth

Tests (`tests/usecases/*_test.go`) are:
- Named after use cases (`TestUC_BM_001_CreateBenchmarkTarget`)
- Organized by acceptance criteria (`t.Run("AC-01: ...")`)
- The definitive proof that requirements are met

---

## Workflow Phases

```
jb-1-idea → jb-2-workspace → jb-3.1-specify → jb-3.1b-usecases →
jb-3.2-impact → jb-3.3-plan → jb-3.4-tasks → jb-3.5-analyze →
jb-3.6-implement → jb-4-pr → jb-5-validate → jb-6-staging →
jb-7-prod → jb-8-review → jb-9-archive
```

### Definition Phase (3.1 - 3.5)

| Phase | Command | Output | Purpose |
|-------|---------|--------|---------|
| 3.1 | `/jb-3.1-specify` | `spec.md` | Define WHAT and WHY |
| 3.1b | `/jb-3.1b-usecases` | `usecases/*.yaml` | Define acceptance criteria |
| 3.2 | `/jb-3.2-impact` | `impact.md` | Analyze migration/refactor impact |
| 3.3 | `/jb-3.3-plan` | `plan.md` | Technical approach |
| 3.4 | `/jb-3.4-tasks` | `tasks.md` + beads | Break into trackable units |
| 3.5 | `/jb-3.5-analyze` | validation | Quality gate before implementation |

### Implementation Phase (3.6)

```bash
# Each task follows this pattern:
1. Read use case YAML for context
2. Output implementation context block
3. Write failing tests FIRST (one per AC)
4. Implement until tests pass
5. STOP - do not add anything beyond acceptance criteria
6. Close task bead with commit reference
```

### Completion Phase (4-9)

| Phase | Command | Purpose |
|-------|---------|---------|
| 4 | `/jb-4-pr` | Create pull request |
| 5 | `/jb-5-validate` | Dev cluster validation |
| 6 | `/jb-6-staging` | Promote to staging |
| 7 | `/jb-7-prod` | Promote to production |
| 8 | `/jb-8-review` | Retrospective |
| 9 | `/jb-9-archive` | Archive spec (use cases remain) |

---

## Agent Ecosystem

### Agent Specialization Model

Agents are specialized by **domain** and **mode**:

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AGENT ECOSYSTEM                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  READ-ONLY AGENTS                    WRITE AGENTS                    │
│  ────────────────                    ────────────                    │
│                                                                      │
│  ┌─────────────────┐                ┌─────────────────┐             │
│  │    debugger     │                │ cli-developer   │             │
│  │  (investigate)  │                │ (ai-aas-cli)    │             │
│  └─────────────────┘                └─────────────────┘             │
│                                                                      │
│  ┌─────────────────┐                ┌─────────────────┐             │
│  │   compliance-   │                │ go-services-    │             │
│  │   reviewer      │                │ developer       │             │
│  └─────────────────┘                └─────────────────┘             │
│                                                                      │
│                                     ┌─────────────────┐             │
│                                     │ operator-       │             │
│                                     │ developer       │             │
│                                     └─────────────────┘             │
│                                                                      │
│                                     ┌─────────────────┐             │
│                                     │ infra-ops-      │             │
│                                     │ manager         │             │
│                                     └─────────────────┘             │
│                                                                      │
│                                     ┌─────────────────┐             │
│                                     │ web-portal-     │             │
│                                     │ developer       │             │
│                                     └─────────────────┘             │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Agent Responsibilities

| Agent | Domain | Mode | Purpose |
|-------|--------|------|---------|
| **debugger** | Any | Read-only | Root cause analysis, produces investigation reports |
| **compliance-reviewer** | Any | Read-only | Detects UC drift, architecture violations |
| **cli-developer** | `services/ai-aas-cli/` | Write | CLI commands and client code |
| **go-services-developer** | `services/*-service/` | Write | API services, business logic |
| **operator-developer** | `operators/` | Write | Kubernetes operators, CRDs |
| **infra-ops-manager** | `infra/`, `gitops/` | Write | Infrastructure, deployments |
| **web-portal-developer** | `web/` | Write | Frontend, React components |

### Handoff Model

Agents don't work outside their domain. They create **handoff beads**:

```bash
# Example: CLI developer finds API bug
bd create --title="API returns wrong status code for invalid model" --type=bug
bd update $NEW_BEAD --add-label "agent:go-services-developer"
bd update $NEW_BEAD --add-label "component:admin-api"
```

### When to Use Each Agent

| Scenario | Agent |
|----------|-------|
| "Why is this failing?" | `debugger` |
| "Verify implementation matches spec" | `compliance-reviewer` |
| "Add a new CLI command" | `cli-developer` |
| "Add an API endpoint" | `go-services-developer` |
| "AIModel CR not behaving correctly" | `operator-developer` |
| "Deploy new service" | `infra-ops-manager` |
| "Fix frontend bug" | `web-portal-developer` |
| Complex bug, >30 min investigation | `debugger` first, then worker agent |

---

## Context Structure

### Hierarchy Overview

```
Level 1: Core Rules (all agents read)
├── context/agents.md                    # Critical rules, NEVER/ALWAYS

Level 2: Domain-Specific (agent reads own domain)
├── context/cli-developer/agents.md
├── context/go-services-developer/agents.md
├── context/operator-developer/agents.md
├── context/infra-ops-manager/agents.md
├── context/web-portal-developer/agents.md
├── context/debugger/agents.md
├── context/e2e-testing/agents.md
├── context/compliance-reviewer-agent.md

Level 2: Use Cases & Templates
├── usecases/SCHEMA.md                   # UC YAML format
├── usecases/*.yaml                      # Acceptance criteria
├── context/templates/beads.md           # Bead templates
├── context/templates/argocd-app.md      # ArgoCD template

Level 3: Reference Documentation
├── ARCHITECTURE.md                      # System overview
├── docs/platform/environment-access.md  # Credentials, endpoints
├── docs/operators/*.md                  # Operator reference
├── docs/runbooks/*.md                   # Step-by-step procedures

Level 4: Source Code (always current)
├── services/*/internal/                 # Service code
├── operators/*/                         # Operator code
├── web/*/                               # Frontend code
├── tests/*/                             # Test code
```

### Context Loading Order

When an agent starts work:

```
1. Read context/agents.md (critical rules)
2. Read context/<agent>/agents.md (domain-specific)
3. Read relevant use cases (if implementing)
4. Read source files directly (not docs about code)
```

### Design Principles

| Principle | Rationale |
|-----------|-----------|
| **Link, don't duplicate** | Duplicated content becomes stale |
| **Two-hop navigation** | Any info within 2 hops from context_map.md |
| **Agent-first writing** | MUST/NEVER keywords, structured data |
| **Source is truth** | Read code, not docs about code |

### Context Maintenance

```bash
# Before PR
/context-maintainer          # Check if context docs need updates

# After fixing a bug
bd update $BUG_ID --add-label "context-gap"   # If caused by missing context
bd close $BUG_ID --reason "CONTEXT-GAP: No anti-pattern for X in agents.md"
```

---

## Key Integration Points

### Beads Integration

Everything is tracked in beads:

```yaml
workflow_beads:
  epic:
    - Created by: jb-2-workspace
    - Label: spec$NUMBER, epic
    - Contains: All child task beads

  use_case:
    - Created by: jb-3.1b-usecases
    - Label: uc:UC-XXX-NNN
    - Purpose: Track implementation of one UC

  task:
    - Created by: jb-3.4-tasks
    - Label: spec$NUMBER
    - Purpose: Individual implementation unit

  bug:
    - Created by: Any agent
    - Label: uc:UC-XXX-NNN (attribute to UC)
    - Purpose: Track bug to its requirement
```

### Test Integration

Tests mirror use cases:

```
usecases/benchmarks.yaml           →    tests/usecases/benchmarks_test.go
├── UC-BM-001                      →    ├── TestUC_BM_001_CreateBenchmarkTarget
│   ├── AC-01                      →    │   ├── t.Run("AC-01: ...")
│   ├── AC-02                      →    │   ├── t.Run("AC-02: ...")
│   └── must_not                   →    │   └── TestUC_BM_001_..._MustNot
└── UC-BM-002                      →    └── TestUC_BM_002_TriggerBenchmarkRun
```

### Compliance Hook

Automatic compliance review on session end:

```json
// .claude/settings.json
{
  "hooks": {
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/compliance-review.sh"
      }]
    }]
  }
}
```

---

## Quick Reference

### Starting New Feature

```bash
/jb-1-idea feature-name           # Create idea.md
# Discuss and refine idea
git add specs/ && git commit      # Commit to develop
/jb-2-workspace NNN-name          # Create workspace + epic bead
/jb-3.1-specify                   # Create spec.md
/jb-3.1b-usecases                 # Create use cases + beads
/jb-3.3-plan                      # Create plan.md
/jb-3.4-tasks                     # Create task beads
/jb-3.5-analyze                   # Quality gate
/jb-3.6-implement                 # Build it
```

### Before Committing

```bash
# Run relevant tests
go test ./path/to/package/...

# Check compliance (automatic on Stop, or manual)
/review-compliance UC-XXX-NNN
```

### Completing Feature

```bash
/jb-4-pr                          # Create PR
/jb-5-validate                    # Dev cluster testing
/jb-6-staging                     # Promote to staging
/jb-7-prod                        # Promote to production
/jb-8-review                      # Retrospective
/jb-9-archive                     # Archive spec (use cases remain)
```

---

## Summary

| Artifact | Lifecycle | Purpose |
|----------|-----------|---------|
| **Spec** | Temporary → Archived | Define requirements |
| **Use Case** | Permanent | Living acceptance criteria |
| **Test** | Permanent | Prove requirements met |
| **Bead** | Open → Closed | Track work units |
| **Context Doc** | Maintained | Agent knowledge |
| **Source Code** | Permanent | Implementation |

The workflow ensures that:
1. **Requirements are clear** before implementation begins
2. **Tests verify** what was requested, not what was built
3. **Drift is detected** by compliance review
4. **Knowledge persists** through context structure
5. **Work is tracked** through beads
