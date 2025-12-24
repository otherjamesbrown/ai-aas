# Staging Promotion (Phase 6)

Promote from develop to staging and validate.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec number from argument.

### Step 2: Verify Dev Validation Complete

Check that dev validation passed:
```bash
bd show aas-spec$SPEC_NUMBER.dev
```

If open issues exist, warn user before proceeding.

### Step 3: Find Epic Bead

Epic bead ID follows convention: `aas-spec$SPEC_NUMBER`

```bash
bd show aas-spec$SPEC_NUMBER
```

### Step 4: Create Staging Sub-Bead

Create staging bead with period-based naming:

```bash
bd create --id="aas-spec$SPEC_NUMBER.stg" --title="Spec $SPEC_NUMBER - Staging Validation" --type=task --priority=1 --parent=aas-spec$SPEC_NUMBER
bd update aas-spec$SPEC_NUMBER.stg --label="spec$SPEC_NUMBER" --label="staging" --label="validation"
```

Staging bead ID: `aas-spec$SPEC_NUMBER.stg`

### Step 5: Create PR develop → staging

```bash
gh pr create \
  --base staging \
  --head develop \
  --title "Promote spec $SPEC_NUMBER to staging" \
  --body "Promotes changes from spec $SPEC_NUMBER after successful development validation.

Related beads:
- Epic: aas-spec$SPEC_NUMBER
- Dev validation: aas-spec$SPEC_NUMBER.dev
- Staging: aas-spec$SPEC_NUMBER.stg"
```

### Step 6: Update Bead with PR

```bash
bd comments add aas-spec$SPEC_NUMBER.stg "Created PR #$PR_NUMBER for develop → staging promotion"
```

### Step 7: After PR Merge - Run Validation

Similar to dev validation:
1. Wait for ArgoCD sync to staging
2. Load staging validation steps from spec.md
3. Run smoke tests against staging
4. Run feature-specific validation

### Step 8: Create Issue Beads for Failures

```bash
bd create --title="Staging: $ISSUE_DESCRIPTION" --type=bug --priority=1 --parent=aas-spec$SPEC_NUMBER.stg
bd update $ISSUE_BEAD_ID --label="spec$SPEC_NUMBER" --label="staging-issue"
```

### Step 9: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-6-staging $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Staging Bead:     aas-spec$SPEC_NUMBER.stg
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
- Track all fixes as child beads of aas-spec$SPEC_NUMBER.stg
- Production promotion requires clean staging validation
