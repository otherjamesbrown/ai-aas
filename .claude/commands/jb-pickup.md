# Pick Up Handoff

Resume work from a handoff bead created in a previous session.

## Arguments: $ARGUMENTS

Optional: Specific bead ID or branch name to filter by

## Instructions

### Step 1: Get Current Branch

```bash
git rev-parse --abbrev-ref HEAD
```

Store as `$CURRENT_BRANCH`.

### Step 2: Find Handoff Beads

Search for handoff beads, prioritizing current branch:

```bash
# First, try current branch
bd list --label handoff --label $CURRENT_BRANCH --status open

# If no results, show all handoffs
bd list --label handoff --status open
```

If `$ARGUMENTS` provided:
- If it looks like a bead ID (e.g., `ai-aas-xxxx`), use that directly
- If it's a branch name, filter: `bd list --label handoff --label $ARGUMENTS`

### Step 3: Select Handoff

If multiple handoffs found, present options sorted by most recent:

```
Found handoff beads:

  1. ai-aas-xxxx (2h ago)  - Branch: feature-xyz - "Handoff: API routing fix"
  2. ai-aas-yyyy (1d ago)  - Branch: develop     - "Handoff: Database migration"

Which handoff to resume? (enter number or bead ID)
```

If only one found, use it automatically.

### Step 4: Load Handoff Context

```bash
bd show $SELECTED_HANDOFF_ID
```

Parse the description to extract:
- Branch name
- Session goal
- Completed work
- Remaining work
- Key findings
- Related beads
- Files changed

### Step 5: Load Related Beads

For each related bead mentioned in the handoff:

```bash
bd show $RELATED_BEAD_ID
```

### Step 6: Verify Branch

Check if we're on the right branch:

```bash
git rev-parse --abbrev-ref HEAD
```

If not on the handoff branch, warn:
```
⚠️  Handoff was from branch '$HANDOFF_BRANCH' but you're on '$CURRENT_BRANCH'
    Consider: git checkout $HANDOFF_BRANCH
```

### Step 7: Show Files Changed

If files were mentioned in the handoff, check their current state:

```bash
git status
git diff --stat
```

### Step 8: Output Summary

```
═══════════════════════════════════════════════════════════════
 HANDOFF LOADED: $HANDOFF_BEAD_ID
═══════════════════════════════════════════════════════════════

 Branch:          $BRANCH_NAME
 Created:         $TIMESTAMP
 Handoff Reason:  $REASON

───────────────────────────────────────────────────────────────
 SESSION GOAL
───────────────────────────────────────────────────────────────
 $SESSION_GOAL

───────────────────────────────────────────────────────────────
 COMPLETED WORK
───────────────────────────────────────────────────────────────
 $COMPLETED_WORK

───────────────────────────────────────────────────────────────
 REMAINING WORK
───────────────────────────────────────────────────────────────
 $REMAINING_WORK

───────────────────────────────────────────────────────────────
 KEY FINDINGS
───────────────────────────────────────────────────────────────
 $KEY_FINDINGS

───────────────────────────────────────────────────────────────
 RELATED BEADS
───────────────────────────────────────────────────────────────
 $RELATED_BEADS_WITH_STATUS

───────────────────────────────────────────────────────────────
 NEXT STEPS
───────────────────────────────────────────────────────────────
 1. $FIRST_ACTION
 2. $SECOND_ACTION
 ...

═══════════════════════════════════════════════════════════════

Ready to continue. What would you like to work on first?
```

### Step 9: Update Handoff Status

Mark the handoff as in progress:

```bash
bd update $HANDOFF_BEAD_ID --status in_progress
```

### Step 10: Offer Actions

Based on remaining work, suggest:
- If there are related beads to close: "Close completed beads: bd close $BEAD_IDS"
- If there are blockers: "Address blocker: $BLOCKER"
- If clear next step: "Continue with: $NEXT_STEP"

## When Handoff Complete

Once the handoff work is done, close the handoff bead:

```bash
bd close $HANDOFF_BEAD_ID
```
