# Implement (Phase 3.6)

Execute implementation of tasks. Wraps /speckit.implement with bead tracking.

## Instructions

### Step 1: Find Current Spec

Detect from working directory:
```bash
basename $(pwd)
```

### Step 2: Load Context

Read all spec files:
```bash
cat specs/$SPEC_FOLDER/spec.md
cat specs/$SPEC_FOLDER/impact.md 2>/dev/null  # May not exist for greenfield
cat specs/$SPEC_FOLDER/plan.md
cat specs/$SPEC_FOLDER/tasks.md
```

### Step 3: Find Task Beads

```bash
bd list --label="spec$SPEC_NUMBER" --status=open
```

### Step 4: Run Speckit Implement

Invoke speckit.implement workflow to:
- Work through tasks in order
- Implement each task
- Update bead status as tasks complete

### Step 5: Update Beads During Implementation

As each task is completed:
```bash
bd update $TASK_BEAD_ID --status=in_progress  # When starting
bd close $TASK_BEAD_ID "Implemented in commit $SHA"  # When done
```

### Step 6: Track Progress

After implementation session:
```bash
bd comments add $SPEC_BEAD_ID "Implementation progress: $COMPLETED/$TOTAL tasks complete"
```

### Step 7: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-3.6-implement - SESSION COMPLETE
═══════════════════════════════════════════════════

 Spec:             $SPEC_FOLDER

 Progress:
   - Tasks completed this session: $COUNT
   - Total tasks complete: $COMPLETED/$TOTAL
   - Remaining: $REMAINING

 Beads Updated:
   - $BEAD_1: CLOSED
   - $BEAD_2: CLOSED
   - $BEAD_3: in_progress

 Next Steps:
   - Continue with /jb-3.6-implement for remaining tasks
   - When all tasks done: /jb-4-pr to create PR
═══════════════════════════════════════════════════
```

## Notes

- Implementation may span multiple sessions
- Use `/jb-0-resume` to reload context if needed
- Close beads as tasks complete, don't batch them
