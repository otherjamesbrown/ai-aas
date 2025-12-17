# Retrospective (Phase 8)

Run a retrospective on the completed spec to capture learnings.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec number from argument (e.g., `spec030`, `030`).

### Step 2: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 3: Create Retrospective Sub-Bead

```bash
bd create --title="Spec $SPEC_NUMBER - Retrospective" --type=task --priority=2 --parent=$EPIC_BEAD_ID
bd update $RETRO_BEAD_ID --label="spec$SPEC_NUMBER" --label="retrospective"
```

### Step 4: Gather Data

Collect information from all phases:

```bash
# List all beads for this spec
bd list --label="spec$SPEC_NUMBER"

# Get timeline from git
git log --oneline --since="2 weeks ago" --grep="spec$SPEC_NUMBER"

# Read the spec files
cat specs/*$SPEC_NUMBER*/spec.md
cat specs/*$SPEC_NUMBER*/plan.md
cat specs/*$SPEC_NUMBER*/tasks.md
```

### Step 5: Generate Retrospective

Create retrospective document at `specs/$SPEC_FOLDER/retrospective.md`:

```markdown
# Retrospective: Spec $SPEC_NUMBER

## Timeline
- Idea captured: $DATE
- Development started: $DATE
- PR merged: $DATE
- Production deployed: $DATE

## What Went Well
- $ITEM_1
- $ITEM_2

## What Could Be Improved
- $ITEM_1
- $ITEM_2

## Learnings
- $LEARNING_1
- $LEARNING_2

## Metrics
- Tasks planned: $COUNT
- Tasks completed: $COUNT
- Issues found in dev: $COUNT
- Issues found in staging: $COUNT
- Issues found in prod: $COUNT

## Recommendations for Future Specs
- $RECOMMENDATION_1
- $RECOMMENDATION_2
```

### Step 6: Ask User for Input

Prompt user:
- What went well?
- What could be improved?
- Any specific learnings to capture?

### Step 7: Update Bead with Summary

```bash
bd comments add $RETRO_BEAD_ID "Retrospective completed. Key learnings: $SUMMARY"
bd close $RETRO_BEAD_ID "Retrospective documented"
```

### Step 8: Close Epic Bead

If all work is complete:
```bash
bd close $EPIC_BEAD_ID "Spec $SPEC_NUMBER completed and retrospective done"
```

### Step 9: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-8-review $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Spec:             $SPEC_FOLDER
 Epic Bead:        $EPIC_BEAD_ID (CLOSED)
 Retro Bead:       $RETRO_BEAD_ID (CLOSED)

 Timeline:
   - Idea → Prod:  $DAYS days
   - Dev issues:   $COUNT
   - Staging issues: $COUNT
   - Prod issues:  $COUNT

 Retrospective saved to:
   specs/$SPEC_FOLDER/retrospective.md

 Next Steps:
   - Review retrospective with team
   - When ready to archive: /jb-9-archive $ARGUMENTS
═══════════════════════════════════════════════════
```

## Notes

- Retrospectives help improve future spec implementations
- Be honest about what didn't go well
- Focus on process improvements, not blame
- Archive spec after retrospective is complete
