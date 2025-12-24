# Plan (Phase 3.3)

Create an implementation plan from the spec. Wraps /speckit.plan with workflow integration.

## Instructions

### Step 1: Find Current Spec

Detect from working directory or branch:
```bash
basename $(pwd)
```

### Step 2: Verify spec.md Exists

```bash
cat specs/$SPEC_FOLDER/spec.md
```

If not found, suggest running `/jb-3.1-specify` first.

### Step 3: Check for impact.md

```bash
cat specs/$SPEC_FOLDER/impact.md 2>/dev/null
```

If impact.md exists:
- Load it alongside spec.md
- Plan phases should align with migration order
- Include REMOVE/DEPRECATE tasks
- Reference file paths and risk levels from impact.md

If impact.md doesn't exist and spec contains migration signals, suggest running `/jb-3.2-impact` first.

### Step 4: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 5: Run Speckit Plan

Invoke speckit.plan workflow to create `plan.md` with:
- Technical architecture
- Component breakdown
- Dependencies
- Implementation order
- Risk assessment

If impact.md exists, incorporate:
- Migration phases as plan phases
- Risk levels from impact analysis
- Rollback strategies

### Step 6: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "plan.md created with /jb-3-3-plan"
```

### Step 7: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-3-3-plan - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/
 File Created:     plan.md

 Components:       $COUNT
 Dependencies:     $COUNT
 Estimated Tasks:  $COUNT

 Impact Analysis:  $INCORPORATED_OR_NA

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review plan.md
   - Run /jb-3.4-tasks to create task breakdown with beads
═══════════════════════════════════════════════════
```
