# Staging Promotion (Phase 6)

Promote from develop to staging and validate.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec number from argument.

### Step 2: Verify Dev Validation Complete

Check that dev validation passed:
```bash
bd show $DEV_BEAD_ID
```

If open issues exist, warn user before proceeding.

### Step 3: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 4: Create Staging Sub-Bead

```bash
bd create --title="Spec $SPEC_NUMBER - Staging Validation" --type=task --priority=1 --parent=$EPIC_BEAD_ID
bd update $STG_BEAD_ID --label="spec$SPEC_NUMBER" --label="staging" --label="validation"
```

### Step 5: Create PR develop → staging

```bash
gh pr create \
  --base staging \
  --head develop \
  --title "Promote spec $SPEC_NUMBER to staging" \
  --body "Promotes changes from spec $SPEC_NUMBER after successful development validation.

Related beads:
- Epic: $EPIC_BEAD_ID
- Dev validation: $DEV_BEAD_ID
- Staging: $STG_BEAD_ID"
```

### Step 6: Update Bead with PR

```bash
bd comments add $STG_BEAD_ID "Created PR #$PR_NUMBER for develop → staging promotion"
```

### Step 7: After PR Merge - Run Validation

Similar to dev validation:
1. Wait for ArgoCD sync to staging
2. Load staging validation steps from spec.md
3. Run smoke tests against staging
4. Run feature-specific validation

### Step 8: Create Issue Beads for Failures

```bash
bd create --title="Staging: $ISSUE_DESCRIPTION" --type=bug --priority=1 --parent=$STG_BEAD_ID
bd update $ISSUE_BEAD_ID --label="spec$SPEC_NUMBER" --label="staging-issue"
```

### Step 9: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-6-staging $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Staging Bead:     $STG_BEAD_ID (child of $EPIC_BEAD_ID)
 PR Created:       #$PR_NUMBER (develop → staging)

 Validation Status:
   $TEST_RESULTS

 Issues Created:   $COUNT
   $ISSUE_LIST

 Next Steps:
   - Merge PR #$PR_NUMBER
   - Fix any staging issues
   - When stable: /jb-7-prod $ARGUMENTS
═══════════════════════════════════════════════════
```

## Notes

- Staging validation may find issues not caught in dev
- Track all fixes as child beads of $STG_BEAD_ID
- Production promotion requires clean staging validation
