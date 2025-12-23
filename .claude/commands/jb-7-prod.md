# Production Promotion (Phase 7)

Promote from staging to production (main) and validate.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec number from argument.

### Step 2: Verify Staging Validation Complete

Check that staging validation passed:
```bash
bd show ai-aas-spec$SPEC_NUMBER.stg
```

If open issues exist, **DO NOT PROCEED**. Warn user.

### Step 3: Find Epic Bead

Epic bead ID follows convention: `ai-aas-spec$SPEC_NUMBER`

```bash
bd show ai-aas-spec$SPEC_NUMBER
```

### Step 4: Create Production Sub-Bead

Create production bead with period-based naming:

```bash
bd create --id="ai-aas-spec$SPEC_NUMBER.prod" --title="Spec $SPEC_NUMBER - Production Deployment" --type=task --priority=1 --parent=ai-aas-spec$SPEC_NUMBER
bd update ai-aas-spec$SPEC_NUMBER.prod --label="spec$SPEC_NUMBER" --label="production" --label="validation"
```

Production bead ID: `ai-aas-spec$SPEC_NUMBER.prod`

### Step 5: Create PR staging → main

```bash
gh pr create \
  --base main \
  --head staging \
  --title "Release spec $SPEC_NUMBER to production" \
  --body "Releases changes from spec $SPEC_NUMBER after successful staging validation.

Related beads:
- Epic: ai-aas-spec$SPEC_NUMBER
- Dev validation: ai-aas-spec$SPEC_NUMBER.dev
- Staging validation: ai-aas-spec$SPEC_NUMBER.stg
- Production: ai-aas-spec$SPEC_NUMBER.prod

## Checklist
- [ ] Staging validation passed
- [ ] No open issues in staging
- [ ] Ready for production traffic"
```

### Step 6: Update Bead with PR

```bash
bd comments add ai-aas-spec$SPEC_NUMBER.prod "Created PR #$PR_NUMBER for staging → main promotion"
```

### Step 7: After PR Merge - Run Validation

1. Wait for ArgoCD sync to production
2. Load production validation steps from spec.md
3. Run smoke tests against production
4. Run feature-specific validation
5. Monitor for any issues

### Step 8: Create Issue Beads for Failures

```bash
bd create --title="PROD: $ISSUE_DESCRIPTION" --type=bug --priority=0 --parent=ai-aas-spec$SPEC_NUMBER.prod
bd update $ISSUE_BEAD_ID --label="spec$SPEC_NUMBER" --label="prod-issue" --label="urgent"
```

**Note:** Production issues are P0 priority.

### Step 9: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-7-prod $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Production Bead:  ai-aas-spec$SPEC_NUMBER.prod
 PR Created:       #$PR_NUMBER (staging → main)

 Validation Status:
   $TEST_RESULTS

 Issues Created:   $COUNT
   $ISSUE_LIST

 Next Steps:
   - Merge PR #$PR_NUMBER (requires approval)
   - Monitor production after deployment
   - When stable: /jb-8-review $ARGUMENTS
═══════════════════════════════════════════════════
```

## Important

- Production deployments require extra care
- All issues are P0 priority
- Have rollback plan ready
- Monitor logs and metrics after deployment
