# Development Cluster Validation (Phase 5)

Validate deployment to development cluster after PR merge.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Verify Branch (REQUIRED)

**CRITICAL**: This command must only be run from the develop branch.

```bash
git branch --show-current
```

If NOT on `develop`:
```
⚠️  ERROR: Must be on develop branch to run validation
Current branch: $CURRENT_BRANCH

Run: git checkout develop
```
**STOP HERE** if not on develop branch.

### Step 2: Sync with Remote (REQUIRED)

**CRITICAL**: Always pull latest from develop remote before validating.

```bash
git fetch origin develop
git status
```

Check if behind remote:
```bash
git rev-list HEAD..origin/develop --count
```

If behind (count > 0):
```
⚠️  WARNING: Local develop is behind remote by $COUNT commits

Pulling latest...
```

```bash
git pull origin develop
```

### Step 3: Parse Spec Reference

Extract spec number from argument (e.g., `spec030`, `030`).

### Step 4: Find Epic Bead

Epic bead ID follows convention: `ai-aas-spec$SPEC_NUMBER`

```bash
bd show ai-aas-spec$SPEC_NUMBER
```

### Step 5: Find or Create Dev Sub-Bead (REQUIRED)

**CRITICAL**: Always ensure dev sub-bead exists to track deployment issues.

Dev bead ID: `ai-aas-spec$SPEC_NUMBER.dev`

Check if dev bead already exists:
```bash
bd show ai-aas-spec$SPEC_NUMBER.dev
```

If exists:
- Set status to in_progress
```bash
bd update ai-aas-spec$SPEC_NUMBER.dev --status=in_progress
```

If NOT exists, create it with period-based naming:

```bash
bd create --id="ai-aas-spec$SPEC_NUMBER.dev" --title="Spec $SPEC_NUMBER - Development Validation" --type=task --priority=1 --parent=ai-aas-spec$SPEC_NUMBER
bd update ai-aas-spec$SPEC_NUMBER.dev --label="spec$SPEC_NUMBER" --label="dev" --label="validation"
```

Show bead status:
```bash
bd show ai-aas-spec$SPEC_NUMBER.dev
```

### Step 6: Check CI/CD Status

Verify deployment to development cluster:
```bash
# Check ArgoCD sync status
argocd app list | grep development

# Or check recent deployments
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml get pods -n $NAMESPACE
```

Wait for deployment to complete before proceeding.

### Step 7: Load Validation Steps

Read validation steps from spec.md:
```bash
grep -A 20 "### Development Cluster" specs/*$SPEC_NUMBER*/spec.md
```

### Step 8: Run Smoke Tests

Execute standard smoke tests:
```bash
./scripts/cli_smoke_test.sh
```

### Step 9: Run Feature-Specific Validation

For each validation step in spec.md:
1. Execute the check
2. Record result (PASS/FAIL)
3. If FAIL, create child bead for the issue

### Step 10: Create Issue Beads for Failures

For each failure:
```bash
bd create --title="Dev: $ISSUE_DESCRIPTION" --type=bug --priority=1 --parent=ai-aas-spec$SPEC_NUMBER.dev
bd update $ISSUE_BEAD_ID --label="spec$SPEC_NUMBER" --label="dev-issue"
```

### Step 11: Update Dev Bead

```bash
bd comments add ai-aas-spec$SPEC_NUMBER.dev "Validation results:
- Smoke tests: $PASS_FAIL
- Feature validation: $PASS_FAIL
- Issues found: $COUNT"
```

### Step 12: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-5-validate $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Branch:           develop ✓
 Remote Sync:      Up to date ✓

 Dev Bead:         ai-aas-spec$SPEC_NUMBER.dev
 Validation Steps: $COUNT defined in spec.md

 Tests Run:
   $TEST_RESULTS_WITH_CHECKMARKS

 Issues Created:   $COUNT
   $ISSUE_LIST

 Next Steps:
   - Fix issues tracked in ai-aas-spec$SPEC_NUMBER.dev
   - Re-run /jb-5-validate $ARGUMENTS when ready
   - When all pass: /jb-6-staging $ARGUMENTS
═══════════════════════════════════════════════════
```

## Notes

- Validation may need multiple runs as issues are fixed
- Close issue beads as they're resolved
- Only proceed to staging when all validation passes
