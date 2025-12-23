# Impact Analysis (Phase 3.2)

Analyze how the specification impacts the existing codebase. Wraps `/speckit.impact` with workflow integration.

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

Scan spec.md for migration signals:
- "migrate from X to Y"
- "replace X with Y"
- "remove X"
- "change behavior of X"
- "no longer use X"

If NO signals found:
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

Invoke `/speckit.impact` for the current spec folder.

The speckit command will:
- Load architecture context
- Search codebase for each impact signal
- Map dependencies
- Categorize changes (REMOVE, MODIFY, ADD, DEPRECATE, UPDATE)
- Assess risk levels
- Determine migration order with rollback strategies
- Generate impact.md

### Step 6: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "impact.md created - $CHANGE_COUNT changes identified ($HIGH_RISK HIGH risk)"
```

### Step 7: Sync Beads

```bash
bd sync
```

### Step 8: Status Summary

```
═══════════════════════════════════════════════════
 /jb-3.2-impact - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/

 File Created:
   - impact.md

 Impact Summary:
   - REMOVE:     [count] items
   - MODIFY:     [count] items
   - ADD:        [count] items
   - DEPRECATE:  [count] items
   - UPDATE:     [count] items

 Risk Profile:
   - HIGH:   [count] changes
   - MEDIUM: [count] changes
   - LOW:    [count] changes

 Migration Phases: [count]

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review impact.md for completeness
   - Run /jb-3.3-plan (plan will incorporate migration phases)
═══════════════════════════════════════════════════
```
