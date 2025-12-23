# Impact Analysis (Phase 3.2)

Analyze how the specification impacts the existing codebase. Run this **before** planning for migrations, refactors, or deprecations.

## When to Use

Run this phase when the spec involves:
- Removing or deprecating existing functionality
- Migrating from one approach to another
- Refactoring existing code patterns
- Changing behavior of existing features

**Skip for pure greenfield features** - proceed directly to `/jb-3.3-plan`.

## Instructions

### Step 1: Find Current Spec

Detect from working directory:
```bash
basename $(pwd)
```

### Step 2: Verify spec.md Exists

```bash
cat specs/$SPEC_FOLDER/spec.md
```

If not found, suggest running `/jb-3.1-specify` first.

### Step 3: Check for Impact Signals

Scan spec.md for these signals:

| Signal | Indicates |
|--------|-----------|
| "migrate from X to Y" | Migration - find all X usage |
| "replace X with Y" | Replacement - find X, plan removal |
| "remove X" | Deprecation - find X and dependents |
| "change behavior of X" | Modification - find X call sites |
| "no longer use X" | Removal - find X integration points |

If NO signals found, output:
```
═══════════════════════════════════════════════════
 /jb-3.2-impact - SKIPPED (Greenfield Feature)
═══════════════════════════════════════════════════

 No migration/refactor signals detected in spec.md.
 This appears to be a greenfield feature.

 Next Step: /jb-3.3-plan
═══════════════════════════════════════════════════
```

### Step 4: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 5: Run Speckit Impact

Invoke `/speckit.impact` workflow to:
1. Load architecture context (`context/agents.md`, `ARCHITECTURE.md`)
2. Search codebase for each impact signal
3. Map dependencies (what depends on affected code)
4. Categorize changes (REMOVE, MODIFY, ADD, DEPRECATE, UPDATE)
5. Assess risk levels (HIGH, MEDIUM, LOW)
6. Determine migration order with rollback strategies

### Step 6: Generate impact.md

Create `specs/$SPEC_FOLDER/impact.md` with:

```markdown
# Impact Analysis: [Feature Name]

**Spec**: [link to spec.md]
**Analyzed**: [date]
**Type**: Migration | Refactor | Deprecation

## Summary
[1-2 sentence overview]

## Impact Signals
| Signal from Spec | Search Pattern | Findings |
|------------------|----------------|----------|

## Affected Components
```yaml
components:
  path/to/component/:
    files: N
    risk: HIGH|MEDIUM|LOW
    changes: [REMOVE, MODIFY, ADD, DEPRECATE, UPDATE]
```

## Detailed Findings
### REMOVE / MODIFY / ADD / DEPRECATE / UPDATE
[Tables of affected files with risk and notes]

## Migration Order
```yaml
phase_1_prepare:
  description: "..."
  tasks: [...]
  risk: LOW
  rollback: "..."
```

## Rollback Plan
[Per-phase rollback strategies]

## Open Questions
[Discovered during analysis]
```

### Step 7: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "impact.md created with /jb-3.2-impact - $CHANGE_COUNT changes identified ($HIGH_RISK HIGH risk)"
```

### Step 8: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-3.2-impact - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/
 File Created:     impact.md

 Impact Summary:
   - REMOVE:     $COUNT items
   - MODIFY:     $COUNT items
   - ADD:        $COUNT items
   - DEPRECATE:  $COUNT items
   - UPDATE:     $COUNT items

 Risk Profile:
   - HIGH:   $COUNT changes
   - MEDIUM: $COUNT changes
   - LOW:    $COUNT changes

 Migration Phases: $COUNT

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review impact.md for completeness
   - Run /jb-3.3-plan to create implementation plan
     (Plan will incorporate migration phases)
═══════════════════════════════════════════════════
```

## Key Constraints

- **Read-only**: Analysis only, no code modifications
- **Thorough search**: Better to find too much than miss affected code
- **Risk-aware**: Flag anything touching production paths as HIGH
- **Order matters**: Migration phases must be safe to execute sequentially
- **Rollback-first**: Every phase needs a rollback strategy

## Integration with jb-3-3-plan

When `/jb-3-3-plan` runs after this phase:
1. Load `impact.md` alongside `spec.md`
2. Plan phases should align with migration order from impact.md
3. Include REMOVE/DEPRECATE tasks in the plan
4. Reference impact.md for file paths and risk levels
