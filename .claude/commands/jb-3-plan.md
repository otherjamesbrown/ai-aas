# Plan (Phase 3)

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

If not found, suggest running `/jb-3-specify` first.

### Step 3: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 4: Run Speckit Plan

Invoke speckit.plan workflow to create `plan.md` with:
- Technical architecture
- Component breakdown
- Dependencies
- Implementation order
- Risk assessment

### Step 5: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "plan.md created with /jb-3-plan"
```

### Step 6: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-3-plan - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/
 File Created:     plan.md

 Components:       $COUNT
 Dependencies:     $COUNT
 Estimated Tasks:  $COUNT

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review plan.md
   - Run /jb-3-tasks to create task breakdown with beads
═══════════════════════════════════════════════════
```
