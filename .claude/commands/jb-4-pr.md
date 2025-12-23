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

Epic bead ID follows convention: `ai-aas-spec$SPEC_NUMBER`

```bash
bd show ai-aas-spec$SPEC_NUMBER
```

### Step 4: Create PR Sub-Bead

Create a sub-bead under the epic for PR tracking with period-based naming:

```bash
bd create --id="ai-aas-spec$SPEC_NUMBER.pr" --title="PR#$PR_NUMBER: $PR_TITLE" --type=task --priority=1 --parent=ai-aas-spec$SPEC_NUMBER
bd update ai-aas-spec$SPEC_NUMBER.pr --label="spec$SPEC_NUMBER" --label="pr" --label="pr-$PR_NUMBER"
```

PR bead ID: `ai-aas-spec$SPEC_NUMBER.pr`

### Step 5: Run PR Review

Execute the full PR review workflow (from jb_pr_review):
1. Gather PR information
2. Display summary
3. Ask what to address (comments, CI, both)
4. Process as needed

### Step 6: Track Work in PR Bead

As work progresses:
```bash
bd comments add ai-aas-spec$SPEC_NUMBER.pr "Addressed review comments..."
bd comments add ai-aas-spec$SPEC_NUMBER.pr "Fixed CI failures..."
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
   - Epic: ai-aas-spec$SPEC_NUMBER
   - PR: ai-aas-spec$SPEC_NUMBER.pr

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
