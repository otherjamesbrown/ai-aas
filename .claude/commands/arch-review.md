# Architecture Review

Run a comprehensive architecture review of the AI-AAS platform. Creates dated output folder, beads for tracking, and reviews each theme with checkpoints.

## Arguments: $ARGUMENTS

- No arguments: Start new review
- `resume`: Resume an in-progress review

## Instructions

### Step 0: Determine Mode

If `$ARGUMENTS` is "resume":
- Go to Step 0a (Resume Mode)

Otherwise:
- Go to Step 1 (New Review)

### Step 0a: Resume Mode

Find existing in-progress review:

```bash
bd list --label=arch-review --status=in_progress
```

Find the most recent review folder:

```bash
ls -d docs/arch-review/reviews/*/ 2>/dev/null | sort -r | head -1
```

Read the summary.md to determine which themes are complete.
Continue from the next incomplete theme at Step 4.

---

## NEW REVIEW WORKFLOW

### Step 1: Create Review Folder

Get today's date and create the folder:

```bash
DATE=$(date +%Y-%m-%d)
mkdir -p docs/arch-review/reviews/$DATE
```

### Step 2: Create Epic Bead

Create the parent epic for this review:

```bash
bd create --title="Architecture Review $DATE" --type=epic --priority=2
```

Capture the auto-generated bead ID (e.g., `ai-aas-xxxx`).

Add labels to the epic:

```bash
bd label add $EPIC_BEAD_ID arch-review
bd label add $EPIC_BEAD_ID review-$(date +%Y-%m)
```

Update status to in_progress:

```bash
bd update $EPIC_BEAD_ID --status=in_progress
```

### Step 3: Initialize Summary

Create initial summary.md from template:

```bash
cp docs/arch-review/templates/summary-template.md docs/arch-review/reviews/$DATE/summary.md
```

Update the header with:
- Review Date: $DATE
- Epic Bead: $EPIC_BEAD_ID
- Previous Review: Check for most recent folder in `docs/arch-review/reviews/`

Create empty remediation.md:

```markdown
# Remediation Backlog

**Review Date:** $DATE
**Epic Bead:** $EPIC_BEAD_ID

## High Priority

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|

## Medium Priority

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|

## Low Priority

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|
```

### Step 4: Review Themes (Loop)

For each theme (1-9), execute the following:

#### Step 4a: Create Theme Bead

```bash
bd create --title="Arch Review: $THEME_NAME" --type=task --priority=3
bd label add $THEME_BEAD_ID arch-review
bd label add $THEME_BEAD_ID theme-$THEME_SLUG
bd dep add $THEME_BEAD_ID $EPIC_BEAD_ID
bd update $THEME_BEAD_ID --status=in_progress
```

Theme slugs: `code-structure`, `configuration`, `data-storage`, `logging`, `error-handling`, `security`, `api-design`, `kubernetes`, `testing`

#### Step 4b: Run Theme Analysis

Use the Task tool with subagent_type=Explore to analyze the codebase for this theme:

**Theme Criteria** (from methodology.md):

| Theme | Key Files to Check |
|-------|-------------------|
| 1. Code Structure | `services/*/internal/`, `internal/`, function sizes |
| 2. Configuration | `*/config/`, env vars, Helm values*.yaml |
| 3. Data Storage | DB clients, repositories, etcd usage, migrations |
| 4. Logging | Logger init, log statements, metrics |
| 5. Error Handling | Error types, handlers, responses |
| 6. Security | Input validation, queries, auth middleware |
| 7. API Design | Routes, handlers, response structures |
| 8. Kubernetes | Helm templates, ArgoCD apps |
| 9. Testing | `*_test.go`, test utilities, CI config |

For each component, score 1-5 and document findings.

#### Step 4c: Write Theme Report

Create the theme report file:

```bash
# Copy template
cp docs/arch-review/templates/theme-template.md docs/arch-review/reviews/$DATE/0$N-$THEME_SLUG.md
```

Fill in:
- Scores per component
- Detailed findings
- Remediation items with priorities

#### Step 4d: Create Remediation Beads

For each HIGH priority remediation item found:

```bash
bd create --title="Fix: $ISSUE_DESCRIPTION" --type=task --priority=1
bd label add $REMEDIATION_BEAD_ID arch-review
bd label add $REMEDIATION_BEAD_ID remediation
bd label add $REMEDIATION_BEAD_ID theme-$THEME_SLUG
bd dep add $REMEDIATION_BEAD_ID $THEME_BEAD_ID
```

Add bead IDs to the theme report and remediation.md.

#### Step 4e: Theme Checkpoint

Present findings to user:

```
═══════════════════════════════════════════════════
 Theme $N: $THEME_NAME - Review Complete
═══════════════════════════════════════════════════

 Epic Bead:        $EPIC_BEAD_ID
 Theme Bead:       $THEME_BEAD_ID

 Components Reviewed: 7

 Scores:
   admin-api-service:    X/5
   api-router-service:   X/5
   analytics-service:    X/5
   user-org-service:     X/5
   ai-model-operator:    X/5
   ai-aas-cli:           X/5
   internal:             X/5

 Average: X.X/5

 Key Findings:
   - Finding 1
   - Finding 2

 Remediation Items Created: X beads

 Progress: $N/9 themes complete

═══════════════════════════════════════════════════
```

**MANDATORY**: Ask the user before continuing:

```
Continue to Theme $NEXT? [Y/n/skip/stop]
- Y: Continue to next theme
- n: Re-review this theme
- skip: Skip to a specific theme number
- stop: Save progress and exit (resume later with /arch-review resume)
```

#### Step 4f: Close Theme Bead

```bash
bd close $THEME_BEAD_ID --reason="Review complete, score X.X/5"
```

### Step 5: Generate Final Summary

After all themes complete:

1. Update summary.md with all scores
2. Calculate averages by theme and component
3. Identify top 3 strengths and top 3 improvement areas
4. If previous review exists, calculate deltas and trends
5. Update remediation.md with all items sorted by priority

### Step 6: Compare to Previous Review (if exists)

Find previous review:

```bash
ls -d docs/arch-review/reviews/*/ | sort -r | head -2 | tail -1
```

If found, read its summary.md and:
- Extract previous scores
- Calculate deltas for each component/theme
- Flag regressions (score decreased)
- Highlight improvements (score increased)
- Add comparison section to new summary.md

### Step 7: Close Epic Bead

```bash
bd close $EPIC_BEAD_ID --reason="Architecture review complete. Overall score: X.X/5"
```

### Step 8: Final Summary

```
═══════════════════════════════════════════════════
 /arch-review - COMPLETE
═══════════════════════════════════════════════════

 Review Date:      $DATE
 Epic Bead:        $EPIC_BEAD_ID (closed)

 Output Folder:    docs/arch-review/reviews/$DATE/

 Files Created:
   - summary.md
   - 01-code-structure.md
   - 02-configuration.md
   - 03-data-storage.md
   - 04-logging.md
   - 05-error-handling.md
   - 06-security.md
   - 07-api-design.md
   - 08-kubernetes.md
   - 09-testing.md
   - remediation.md

 Beads Created:
   - 1 epic (arch-review)
   - 9 theme beads
   - X remediation beads

 Overall Scores:
   By Theme:
     1. Code Structure:    X.X/5
     2. Configuration:     X.X/5
     ...

   By Component:
     admin-api-service:    X.X/5
     api-router-service:   X.X/5
     ...

   Overall Average:        X.X/5

 Comparison to Previous:   ↑ improved / ↓ regressed / = same

 High Priority Remediations: X items

 View remediation backlog:
   bd list --label=arch-review --label=remediation

 Next Steps:
   1. Review remediation.md for prioritized action items
   2. Start with high-priority, low-effort items
   3. Run regression tests after each batch of changes
   4. Schedule next review: $(date -d "+1 month" +%Y-%m-%d)

═══════════════════════════════════════════════════
```

## Important Notes

- **Checkpoints are mandatory**: Always pause and ask user before proceeding to next theme
- **Use auto-generated bead IDs**: Do not create custom bead names
- **Label consistently**: All beads get `arch-review` label for easy filtering
- **Reference beads in docs**: Link bead IDs in all markdown files
- **Preserve context**: On resume, read existing files to understand progress
