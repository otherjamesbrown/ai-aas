# Impact Analysis Command

Analyze how a specification impacts the existing codebase. Run this **before** `/speckit.plan` for migration, refactoring, or deprecation specs.

## When to Use

Run `/speckit.impact` when the spec involves:
- Removing or deprecating existing functionality
- Migrating from one approach to another
- Refactoring existing code patterns
- Changing behavior of existing features

Skip for pure greenfield features with no existing code impact.

## Instructions

### Step 1: Load Context

Read the specification and architecture context:
```bash
# Required
specs/[feature]/spec.md

# Architecture context
context/agents.md
context/context_map.md
ARCHITECTURE.md

# Agent-specific context (based on spec scope)
context/operator-developer/agents.md    # If touching operators/
context/go-services-developer/agents.md # If touching services/
context/infra-ops-manager/agents.md     # If touching gitops/, infra/
context/cli-developer/agents.md         # If touching CLI
```

### Step 2: Extract Impact Signals

From `spec.md`, identify:

| Signal | Indicates |
|--------|-----------|
| "migrate from X to Y" | Migration - find all X usage |
| "replace X with Y" | Replacement - find X, plan removal |
| "remove X" | Deprecation - find X and dependents |
| "change behavior of X" | Modification - find X call sites |
| "no longer use X" | Removal - find X integration points |

Document these signals for search queries.

### Step 3: Codebase Search

For each impact signal, search the codebase:

```bash
# Find code references
grep -r "pattern" --include="*.go" --include="*.yaml"

# Find configuration
grep -r "pattern" gitops/ infra/ services/*/deployments/

# Find documentation references
grep -r "pattern" docs/ *.md
```

**Search Categories:**

| Category | Where to Search | What to Find |
|----------|-----------------|--------------|
| **Code** | `services/`, `operators/`, `shared/` | Functions, types, imports |
| **Config** | `gitops/`, `infra/`, `*/helm/` | YAML manifests, Helm values |
| **CRDs** | `operators/*/api/` | Type definitions, validation |
| **Tests** | `**/*_test.go`, `**/tests/` | Test coverage of affected code |
| **Docs** | `docs/`, `*.md` | Documentation to update |
| **Scripts** | `scripts/`, `.github/` | Automation that may break |

### Step 4: Dependency Mapping

For each affected file, trace dependencies:

```yaml
affected_file: operators/ai-model-operator/internal/kserve/inferenceservice.go
depends_on:
  - operators/ai-model-operator/api/v1alpha1/aimodel_types.go
  - k8s.io/api/core/v1
depended_by:
  - operators/ai-model-operator/controllers/aimodel_controller.go
  - operators/ai-model-operator/internal/kserve/inferenceservice_test.go
```

### Step 5: Categorize Changes

Classify each finding into:

| Category | Definition | Example |
|----------|------------|---------|
| **REMOVE** | Delete entirely, no replacement | Old config files, dead code |
| **MODIFY** | Change existing code/config | Add field, change logic |
| **ADD** | Create net new | New files, new functions |
| **DEPRECATE** | Keep but mark for removal | Add deprecation warning, maintain temporarily |
| **UPDATE** | Docs/tests that need sync | README, test assertions |

### Step 6: Risk Assessment

For each change, assess:

| Risk Level | Criteria |
|------------|----------|
| **HIGH** | Affects production traffic, data integrity, or security |
| **MEDIUM** | Affects developer workflow, CI/CD, or observability |
| **LOW** | Documentation, comments, unused code paths |

### Step 7: Migration Order

Determine safe order of operations:

```yaml
migration_order:
  phase_1_prepare:
    - Add new fields/types (backward compatible)
    - Add feature flags if needed

  phase_2_implement:
    - Implement new behavior
    - Update tests for new behavior

  phase_3_migrate:
    - Switch to new behavior
    - Migrate existing resources

  phase_4_cleanup:
    - Remove old code paths
    - Remove deprecated config
    - Update documentation
```

### Step 8: Generate Impact Report

Create `specs/[feature]/impact.md`:

```markdown
# Impact Analysis: [Feature Name]

**Spec**: [link to spec.md]
**Analyzed**: YYYY-MM-DD
**Type**: Migration | Refactor | Deprecation

## Summary

[1-2 sentence overview of impact scope]

## Impact Signals

| Signal from Spec | Search Pattern | Findings |
|------------------|----------------|----------|
| "migrate from Knative to RawDeployment" | `Serverless\|Knative\|deploymentMode` | 12 files |

## Affected Components

```yaml
components:
  operators/ai-model-operator/:
    files: 8
    risk: HIGH
    changes: [MODIFY, ADD]

  gitops/clusters/:
    files: 3
    risk: MEDIUM
    changes: [MODIFY]

  docs/:
    files: 5
    risk: LOW
    changes: [UPDATE]
```

## Detailed Findings

### REMOVE

| File | Lines | What | Risk | Notes |
|------|-------|------|------|-------|
| `path/to/file.go` | 45-67 | Implicit nodeSelector logic | MEDIUM | Replace with explicit mode |

### MODIFY

| File | Lines | What | Risk | Notes |
|------|-------|------|------|-------|
| `path/to/types.go` | 23 | Add DeploymentMode field | HIGH | CRD change, needs migration |

### ADD

| File | What | Risk | Notes |
|------|------|------|-------|
| `path/to/hpa.go` | HPA builder for RawDeployment | MEDIUM | New capability |

### DEPRECATE

| File | What | Removal Target | Notes |
|------|------|----------------|-------|
| `infra/k8s/knative/` | Knative config for GPU | Phase 4 | Keep for CPU workloads |

### UPDATE (Tests)

| File | What |
|------|------|
| `*_test.go` | Update test assertions |

### UPDATE (Documentation)

**IMPORTANT**: Documentation updates are split into two categories with different purposes:

#### `/docs/` - Human Documentation
For users, operators, and developers reading documentation.

| File | What | Audience |
|------|------|----------|
| `docs/operators/ai-model-operator.md` | Add deploymentMode documentation | Operators |
| `docs/runbooks/*.md` | Add/update runbooks | On-call engineers |
| `docs/platform/*.md` | Platform guides | Platform users |

#### `/context/` - Agent Context (CRITICAL)
For AI agents loaded into context. **NOT for human consumption.**

| File | What | Agent |
|------|------|-------|
| `context/operator-developer/agents.md` | Add deployment mode patterns | operator-developer |
| `context/agents.md` | Core rules if broadly applicable | all agents |

**Context Document Rules** (from `context/context_map.md`):
```yaml
format_rules:
  structure:
    - Use YAML for: architecture, configs, hierarchies
    - Use tables for: lookups, mappings, comparisons
    - Use bullets for: rules, lists, requirements
    - Use code blocks for: commands, examples

  writing_style:
    - Keywords: MUST, NEVER, ALWAYS (not "should", "consider")
    - Structured data > prose
    - No ASCII diagrams (use YAML)
    - Link to source files, don't duplicate

  constraints:
    - RULES type: max 200 lines
    - REFERENCE type: max 300 lines
    - Include: last_verified date, type metadata

  anti_patterns_format:
    - Show WRONG examples only
    - Agent already knows the rules
    - No need for CORRECT examples alongside
```

## Migration Order

```yaml
phase_1_prepare:
  description: "Add new types, backward compatible"
  tasks:
    - Add DeploymentMode to ModelRecipeSpec
    - Add DeploymentMode to AIModelSpec
    - Regenerate CRDs
  risk: LOW
  rollback: "Remove new fields"

phase_2_implement:
  description: "Implement new behavior"
  tasks:
    - Add determineDeploymentMode() function
    - Update InferenceServiceBuilder
    - Add HPA support
  risk: MEDIUM
  rollback: "Revert to implicit logic"

phase_3_migrate:
  description: "Switch existing deployments"
  tasks:
    - Update existing GPU recipes
    - Configure Knative GC
    - Run cleanup script
  risk: HIGH
  rollback: "Recipes still work with old logic"

phase_4_cleanup:
  description: "Remove deprecated code"
  tasks:
    - Remove implicit nodeSelector logic
    - Clean up stale Knative revisions
    - Update documentation
  risk: LOW
  rollback: "N/A - old code already unused"
```

## Dependencies

```yaml
external_dependencies:
  - KServe (existing)
  - Knative (existing, partially removing)

internal_dependencies:
  - AIModel CRD (MODIFY)
  - ModelRecipe CRD (MODIFY)
  - ai-model-operator controller (MODIFY)
```

## Test Coverage

| Component | Current Coverage | Needs Update |
|-----------|------------------|--------------|
| `aimodel_controller_test.go` | 78% | Yes - add mode selection tests |
| `inferenceservice_test.go` | 65% | Yes - add RawDeployment tests |

## Rollback Plan

| Phase | Rollback Strategy |
|-------|-------------------|
| Phase 1 | Remove new CRD fields, redeploy operator |
| Phase 2 | Revert operator code, existing deployments unaffected |
| Phase 3 | Set `deploymentMode: Serverless` on recipes |
| Phase 4 | No rollback needed (cleanup only) |

## Open Questions

1. [Any questions discovered during analysis]

## Next Step

`/speckit.plan [feature]` - Plan will incorporate this impact analysis
```

### Step 9: Handoff to Plan

After generating `impact.md`, inform user:

```markdown
## Impact Analysis Complete

**Output**: `specs/[feature]/impact.md`

### Summary
- **REMOVE**: X items
- **MODIFY**: Y items
- **ADD**: Z items
- **UPDATE**: W items

### Risk Profile
- HIGH: X changes
- MEDIUM: Y changes
- LOW: Z changes

### Recommended Next Step
`/speckit.plan [feature]` - The plan will incorporate migration phases from this analysis.

### Review Checklist
- [ ] All affected files identified?
- [ ] Migration order makes sense?
- [ ] Rollback strategies viable?
- [ ] Test coverage gaps identified?
```

## Key Constraints

- **Read-only**: Analysis only, no code modifications
- **Thorough search**: Better to find too much than miss affected code
- **Risk-aware**: Flag anything touching production paths as HIGH
- **Order matters**: Migration phases must be safe to execute sequentially
- **Rollback-first**: Every phase needs a rollback strategy

## Integration with speckit.plan

When `/speckit.plan` runs after `/speckit.impact`:
1. Load `impact.md` alongside `spec.md`
2. Plan phases should align with migration order
3. Include REMOVE/DEPRECATE tasks in the plan
4. Reference impact.md for file paths

## Key Constraints

- Use ISO date format: YYYY-MM-DD

## User Input
$ARGUMENTS
