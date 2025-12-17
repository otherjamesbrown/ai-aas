# Development Cluster Validation (Phase 5)

Validate deployment to development cluster after PR merge.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec number from argument (e.g., `spec030`, `030`).

### Step 2: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 3: Create Dev Sub-Bead

```bash
bd create --title="Spec $SPEC_NUMBER - Development Validation" --type=task --priority=1 --parent=$EPIC_BEAD_ID
bd update $DEV_BEAD_ID --label="spec$SPEC_NUMBER" --label="dev" --label="validation"
```

### Step 4: Check CI/CD Status

Verify deployment to development cluster:
```bash
# Check ArgoCD sync status
argocd app list | grep development

# Or check recent deployments
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml get pods -n $NAMESPACE
```

Wait for deployment to complete before proceeding.

### Step 5: Load Validation Steps

Read validation steps from spec.md:
```bash
grep -A 20 "### Development Cluster" specs/*$SPEC_NUMBER*/spec.md
```

### Step 6: Run Smoke Tests

Execute standard smoke tests:
```bash
./scripts/cli_smoke_test.sh
```

### Step 7: Run Feature-Specific Validation

For each validation step in spec.md:
1. Execute the check
2. Record result (PASS/FAIL)
3. If FAIL, create child bead for the issue

### Step 8: Create Issue Beads for Failures

For each failure:
```bash
bd create --title="Dev: $ISSUE_DESCRIPTION" --type=bug --priority=1 --parent=$DEV_BEAD_ID
bd update $ISSUE_BEAD_ID --label="spec$SPEC_NUMBER" --label="dev-issue"
```

### Step 9: Update Dev Bead

```bash
bd comments add $DEV_BEAD_ID "Validation results:
- Smoke tests: $PASS_FAIL
- Feature validation: $PASS_FAIL
- Issues found: $COUNT"
```

### Step 10: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-5-validate $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Bead Created:     $DEV_BEAD_ID (child of $EPIC_BEAD_ID)
 Validation Steps: $COUNT defined in spec.md

 Tests Run:
   $TEST_RESULTS_WITH_CHECKMARKS

 Issues Created:   $COUNT
   $ISSUE_LIST

 Next Steps:
   - Fix issues tracked in $DEV_BEAD_ID
   - Re-run /jb-5-validate $ARGUMENTS when ready
   - When all pass: /jb-6-staging $ARGUMENTS
═══════════════════════════════════════════════════
```

## Notes

- Validation may need multiple runs as issues are fixed
- Close issue beads as they're resolved
- Only proceed to staging when all validation passes
