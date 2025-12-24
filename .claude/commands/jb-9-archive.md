# Archive Spec (Phase 9)

Archive a completed spec by moving it out of active specs directory.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec number from argument (e.g., `spec030`, `030`).

### Step 2: Verify Spec is Complete

Check that all beads are closed:
```bash
bd list --label="spec$SPEC_NUMBER" --status=open
```

If any beads are still open, warn user and list them.

### Step 3: Find Spec Folder

```bash
ls -d specs/*$SPEC_NUMBER*
```

### Step 4: Create Archive Directory

```bash
mkdir -p .archive-specs
```

### Step 5: Move Spec to Archive

```bash
mv specs/$SPEC_FOLDER .archive-specs/
```

### Step 6: Update Git

```bash
git add specs/ .archive-specs/
git commit -m "chore: archive spec $SPEC_NUMBER

Spec completed and retrospective done.
Moving to .archive-specs for historical reference."
```

### Step 7: Compact Beads (Optional)

If beads are getting large, compact closed issues:
```bash
bd compact --label="spec$SPEC_NUMBER"
```

### Step 8: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-9-archive $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Spec:             $SPEC_FOLDER

 Actions Taken:
   ✓ Verified all beads closed
   ✓ Moved to .archive-specs/$SPEC_FOLDER
   ✓ Committed archive change

 Files Archived:
   - spec.md
   - plan.md
   - tasks.md
   - retrospective.md

 Beads:
   - Total beads for spec: $COUNT
   - All closed: ✓

 The spec is now archived and out of active development.

 To reference archived specs:
   ls .archive-specs/
   cat .archive-specs/$SPEC_FOLDER/spec.md
═══════════════════════════════════════════════════
```

## Notes

- Only archive after retrospective is complete
- Archived specs remain in git history
- Beads remain searchable even after archiving
- Use `.archive-specs/` to keep root clean but specs accessible

## Restoring an Archived Spec

If you need to reopen work on an archived spec:
```bash
mv .archive-specs/$SPEC_FOLDER specs/
bd reopen $EPIC_BEAD_ID
```
