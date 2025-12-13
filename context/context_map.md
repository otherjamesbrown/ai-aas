# Context Map

> This file is the INDEX for all agent context. Read this to understand how to navigate project documentation.

---

## RULES FOR CONTEXT DOCUMENTS

### Document Classification

| Type | Purpose | Max Lines | Format |
|------|---------|-----------|--------|
| **RULES** | Critical agent behavior | 200 | YAML/bullets, no prose |
| **REFERENCE** | Detailed lookups | 300 | YAML/tables, link to sources |
| **OPERATIONAL** | Step-by-step procedures | 500 | Numbered steps, can be prose |
| **SOURCE** | Actual code/config | N/A | Read directly from files |

### Format Requirements

```yaml
format_rules:
  structure:
    - Use YAML for: architecture, configs, hierarchies
    - Use tables for: lookups, mappings, comparisons
    - Use bullets for: rules, lists, requirements
    - Use code blocks for: commands, examples

  avoid:
    - ASCII diagrams (use YAML or link to external tool)
    - Prose paragraphs (use bullets)
    - Duplicating source files (link instead)
    - CORRECT examples in anti-patterns (agent knows the rules)

  required_metadata:
    - last_verified: YYYY-MM-DD
    - verified_against_commit: <hash>
    - inherits: path/to/parent.md (if applicable)
    - type: rules|reference|operational
```

### Content Principles

1. **Link, don't duplicate**
   - If content exists in a source file, link to it
   - Source files: code, Helm charts, CRDs, configs
   - Exception: Critical rules that agent MUST NOT miss

2. **Two-hop navigation**
   - Agent should find any info within 2 hops from this file
   - Level 1 → Level 2 → Level 3 or Source

3. **Stale content is dangerous**
   - Duplicated content becomes stale
   - Links stay current (agent reads live file)
   - Each doc must have `last_verified` date

4. **Agent-first writing**
   - Write for machine parsing, not human reading
   - Keywords: MUST, NEVER, ALWAYS (not "should", "consider")
   - Structured data > prose

### Maintenance Rules

```yaml
maintenance:
  verification:
    - Review all docs monthly
    - Update last_verified after review
    - Check links still resolve
    - Validate against live system

  when_to_update:
    - Source file changed → check if doc needs update
    - New feature added → add to relevant context
    - Bug caused by stale doc → update immediately

  ownership:
    - context/agents.md: all agents (core rules)
    - context/<agent>/agents.md: that agent
    - context_map.md: infra-ops-manager

  automation:
    agent: context-maintainer
    definition: .claude/agents/context-maintainer.md
    triggers:
      - Before PR creation
      - After bug fix (check if bug was caused by missing context)
      - Weekly scheduled review
    outputs:
      - Context Review Report
      - Required updates list
      - Beads for follow-up work
```

---

## CURRENT STATE AUDIT

### Summary

| Level | Files | Total Lines | Status |
|-------|-------|-------------|--------|
| 1 | 1 | 161 | ✅ OPTIMIZED |
| 2 | 8 | 996 | ✅ OPTIMIZED |
| 3 | 5+ | 1750+ | PENDING |

### Level 1: Core Rules

| File | Lines | Status | Notes |
|------|-------|--------|-------|
| `context/agents.md` | 161 | ✅ OPTIMIZED | Critical rules first, links to sources |

### Level 2: Agent Context

| File | Lines | Status | Notes |
|------|-------|--------|-------|
| `context/cli-developer/agents.md` | 117 | ✅ OPTIMIZED | YAML patterns, WRONG only |
| `context/go-services-developer/agents.md` | 135 | ✅ OPTIMIZED | YAML patterns, WRONG only |
| `context/operator-developer/agents.md` | 126 | ✅ OPTIMIZED | Links to CRD source |
| `context/infra-ops-manager/agents.md` | 143 | ✅ OPTIMIZED | Links to runbooks |
| `context/web-portal-developer/agents.md` | 149 | ✅ OPTIMIZED | YAML patterns |

### Level 2: Templates

| File | Lines | Status | Notes |
|------|-------|--------|-------|
| `context/templates/beads.md` | 125 | ✅ OK | Bead templates |
| `context/templates/argocd-app.md` | 113 | ✅ NEW | ArgoCD Application template |
| `context/templates/agent-context.md` | 88 | ✅ NEW | Template for agent context files |

### Level 3: Reference Docs

| File | Lines | Type | Status | Notes |
|------|-------|------|--------|-------|
| `ARCHITECTURE.md` | 247 | reference | ✅ OPTIMIZED | YAML format, no ASCII diagrams |
| `AI_ASSISTANT_GUIDE.md` | 165 | reference | ✅ OK | Onboarding guide, no overlap |
| `CLAUDE.md` | 430 | rules | ✅ UPDATED | Links to context/agents.md |
| `docs/platform/environment-access.md` | 337 | reference | ✅ UPDATED | Recently cleaned up |
| `docs/operators/ai-model-operator.md` | 338 | reference | ✅ UPDATED | Links to CRD source |

### Level 4: Sources (No changes needed - these ARE the source of truth)

```yaml
sources:
  code:
    - services/*/internal/
    - operators/ai-model-operator/
    - web/portal/src/
    - shared/

  config:
    - services/*/deployments/helm/
    - gitops/clusters/*/apps/
    - infra/k8s/aimodels/
    - .github/workflows/

  crd_spec:
    - operators/ai-model-operator/api/v1alpha1/aimodel_types.go

  secrets:
    - secrets/env/.env
    - secrets/kubeconfigs/
```

---

## NAVIGATION HIERARCHY

```yaml
hierarchy:
  level_1:
    context/agents.md:
      type: rules
      purpose: Critical rules all agents must follow
      read_when: Starting any work
      links_to:
        - level_2 (agent-specific context)
        - level_2 (templates)
        - level_3 (reference docs)

  level_2:
    agent_context:
      context/cli-developer/agents.md:
        type: rules
        purpose: CLI-specific patterns
        read_when: Working on services/ai-aas-cli/
        inherits: context/agents.md
        links_to:
          - services/ai-aas-cli/ (source)
          - docs/go-services/ (if exists)

      context/go-services-developer/agents.md:
        type: rules
        purpose: Go service patterns
        read_when: Working on services/*-service/
        inherits: context/agents.md
        links_to:
          - services/*/internal/ (source)
          - shared/ (source)

      context/operator-developer/agents.md:
        type: rules
        purpose: Operator patterns
        read_when: Working on operators/
        inherits: context/agents.md
        links_to:
          - operators/ai-model-operator/ (source)
          - docs/operators/ai-model-operator.md (reference)

      context/infra-ops-manager/agents.md:
        type: rules
        purpose: Infrastructure patterns
        read_when: Working on infra/, gitops/, .github/
        inherits: context/agents.md
        links_to:
          - gitops/clusters/ (source)
          - infra/ (source)
          - docs/runbooks/ (operational)

      context/web-portal-developer/agents.md:
        type: rules
        purpose: Frontend patterns
        read_when: Working on web/
        inherits: context/agents.md
        links_to:
          - web/portal/ (source)

    templates:
      context/templates/beads.md:
        type: reference
        purpose: Bead templates and labels
        read_when: Creating or updating beads
        final: true

      context/templates/argocd-app.md:
        type: reference
        purpose: ArgoCD Application template
        read_when: Creating new ArgoCD Applications
        final: true

      context/templates/agent-context.md:
        type: reference
        purpose: Template for new agent context files
        read_when: Creating new agent
        final: true

  level_3:
    reference:
      ARCHITECTURE.md:
        type: reference
        purpose: System architecture overview
        read_when: Understanding system design
        links_to:
          - docs/operators/ai-model-operator.md
          - services/*/README.md

      docs/platform/environment-access.md:
        type: reference
        purpose: Credentials and endpoints
        read_when: Accessing environments
        links_to:
          - secrets/env/.env (source)
          - secrets/kubeconfigs/ (source)

      docs/operators/ai-model-operator.md:
        type: reference
        purpose: AIModel CRD reference
        read_when: Working with AIModel resources
        links_to:
          - operators/ai-model-operator/api/v1alpha1/aimodel_types.go (source)
          - infra/k8s/aimodels/ (source)

    operational:
      docs/runbooks/:
        type: operational
        purpose: Step-by-step procedures
        read_when: Performing specific operations
        files:
          - ai-debugging-workflow.md (debugging)
          - deploy-to-environments.md (deployments)
          - argocd-bootstrap.md (ArgoCD setup)
          - migrations.md (database)
          - "... 18 total runbooks"

  level_4:
    sources:
      description: Actual source files - read directly
      categories:
        code: services/*, operators/*, web/*, shared/
        config: */deployments/helm/*, gitops/*, infra/k8s/*
        secrets: secrets/env/.env, secrets/kubeconfigs/
```

---

## OPTIMIZATION PLAN

### ✅ COMPLETED: Level 2 Agent Context Files

All agent context files now follow this structure (<150 lines each):
- Domain (5 lines)
- Key Patterns (YAML, 50 lines)
- Anti-patterns (WRONG only, 20 lines)
- Commands (15 lines)
- Sources (10 lines)
- Checklist (10 lines)

**Results**: 1311 → 670 lines (49% reduction)

### ✅ COMPLETED: Priority 1 - ARCHITECTURE.md

- Converted ASCII diagrams to YAML
- Removed prose, using structured YAML
- Links to source files and other docs
- **Result**: 481 → 247 lines (49% reduction)

### ✅ COMPLETED: Priority 2 - Resolve Overlap

| Files | Resolution |
|-------|------------|
| CLAUDE.md | Added "Related Documents" table linking to context/agents.md |
| AI_ASSISTANT_GUIDE.md | Kept as-is (onboarding guide, no duplication) |
| docs/operators/ai-model-operator.md | Added "Quick Links" table to CRD source |

---

## HOW TO USE THIS MAP

### For Agents

```yaml
navigation:
  starting_work:
    1: Read context/agents.md (critical rules)
    2: Read context/<your-agent>/agents.md (your domain)
    3: Check context_map.md if you need to find something

  finding_rules:
    - context/agents.md (core)
    - context/<agent>/agents.md (agent-specific)

  finding_how_to:
    - context/templates/beads.md (beads)
    - docs/runbooks/*.md (operations)

  finding_architecture:
    - ARCHITECTURE.md (overview)
    - docs/operators/ai-model-operator.md (operator detail)

  finding_code:
    - Go directly to source: services/*, operators/*, web/*
    - Don't read docs about code - read the code

  finding_config:
    - Helm: services/*/deployments/helm/
    - ArgoCD: gitops/clusters/*/apps/
    - AIModels: infra/k8s/aimodels/
```

### For Humans Maintaining Context

```yaml
maintenance_checklist:
  monthly:
    - [ ] Review all context/*.md files
    - [ ] Update last_verified dates
    - [ ] Check for stale content
    - [ ] Verify links resolve

  when_code_changes:
    - [ ] Check if related context needs update
    - [ ] If duplicated content exists, remove duplication
    - [ ] Update verified_against_commit

  when_adding_new_agent:
    - [ ] Create context/<agent>/agents.md
    - [ ] Add to context_map.md hierarchy
    - [ ] Ensure it inherits from context/agents.md
```
