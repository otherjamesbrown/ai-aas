# PR Review (Phase 4)

Review a PR and create tracking beads. Extends /jb_pr_review with spec workflow integration.

## PR URL: $ARGUMENTS

## Instructions

### Step 1: Parse PR URL

Extract owner, repo, PR number from the URL.

### Step 2: Detect Related Spec

From the PR branch name, detect the spec:
```bash
gh pr view $PR_NUMBER --json headRefName --jq '.headRefName'
```

If branch matches pattern `NNN-*` or `spec-NNN-*`, extract spec number.

### Step 3: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 4: Create PR Sub-Bead

Create a sub-bead under the epic for PR tracking:

```bash
bd create --title="PR#$PR_NUMBER: $PR_TITLE" --type=task --priority=1 --parent=$EPIC_BEAD_ID
bd update $PR_BEAD_ID --label="spec$SPEC_NUMBER" --label="pr" --label="pr-$PR_NUMBER"
```

### Step 5: Run PR Review

Execute the full PR review workflow (from jb_pr_review):
1. Gather PR information
2. Display summary
3. Ask what to address (comments, CI, both)
4. Process as needed

### Step 6: Track Work in PR Bead

As work progresses:
```bash
bd comments add $PR_BEAD_ID "Addressed review comments..."
bd comments add $PR_BEAD_ID "Fixed CI failures..."
```

### Step 7: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-4-pr $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 PR:               #$PR_NUMBER - $PR_TITLE
 Branch:           $HEAD → $BASE
 Related Spec:     $SPEC_NUMBER

 Beads:
   - Epic: $EPIC_BEAD_ID
   - PR: $PR_BEAD_ID (created)

 PR Status:
   - Files changed: $COUNT
   - CI Status: $STATUS
   - Review comments: $COUNT

 Next Steps:
   - Address any review comments or CI failures
   - After merge: /jb-5-validate spec$SPEC_NUMBER
═══════════════════════════════════════════════════
```

## Notes

- If no spec detected from branch, ask user which spec this relates to
- PR bead becomes child of epic bead for full traceability
- After PR merges, proceed to /jb-5-validate
