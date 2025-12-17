# Tasks (Phase 3)

Create task breakdown and child beads. Wraps /speckit.tasks with bead integration.

## Instructions

### Step 1: Find Current Spec

Detect from working directory:
```bash
basename $(pwd)
```

### Step 2: Verify plan.md Exists

```bash
cat specs/$SPEC_FOLDER/plan.md
```

If not found, suggest running `/jb-3-plan` first.

### Step 3: Find Implementation Bead

Find the spec sub-bead (not epic):
```bash
bd list --label="spec$SPEC_NUMBER" --label="implementation"
```

### Step 4: Run Speckit Tasks

Invoke speckit.tasks workflow to create `tasks.md` with:
- Numbered tasks
- Dependencies between tasks
- Priority levels
- Estimated complexity

### Step 5: Create Child Beads for Each Task

For each task in tasks.md:

```bash
bd create --title="$TASK_TITLE" --type=task --priority=$PRIORITY --parent=$SPEC_BEAD_ID
bd update $TASK_BEAD_ID --label="spec$SPEC_NUMBER"
```

Add task bead IDs back to tasks.md as references.

### Step 6: Update Parent Bead

```bash
bd comments add $SPEC_BEAD_ID "Created $COUNT task beads from /jb-3-tasks"
bd update $SPEC_BEAD_ID --status=in_progress
```

### Step 7: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-3-tasks - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/
 File Created:     tasks.md

 Tasks Created:    $COUNT
 Beads Created:
   - $BEAD_1: $TASK_1
   - $BEAD_2: $TASK_2
   - ...

 Parent Bead:      $SPEC_BEAD_ID (in_progress)

 Next Steps:
   - Run /jb-3-implement to start implementation
   - Mark beads complete as you finish tasks
═══════════════════════════════════════════════════
```
