# Session Handoff

Create a handoff bead to preserve context for resuming work in a new session.

## Arguments: $ARGUMENTS

Optional: Brief description of why you're handing off (e.g., "end of day", "context full")

## Instructions

### Step 1: Check for Existing Handoffs

First, check if there's already an open handoff for this branch:

```bash
BRANCH=$(git rev-parse --abbrev-ref HEAD)
bd list --label handoff --label $BRANCH --status open
bd list --label handoff --label $BRANCH --status in_progress
```

If existing handoff(s) found, ask the user:

```
Found existing handoff(s) for branch '$BRANCH':

  ai-aas-xxxx (in_progress) - "Handoff: Previous work description"
  ai-aas-yyyy (open)        - "Handoff: Earlier session"

These should probably be closed before creating a new handoff.

Options:
  1. Close existing and create new handoff
  2. Update existing handoff instead (add new context)
  3. Keep existing and create another (not recommended)
  4. Cancel - don't create new handoff
```

Use AskUserQuestion tool to get user choice.

**If user chooses option 1 (Close existing):**
```bash
bd close $EXISTING_HANDOFF_ID --reason "Superseded by new handoff"
```
Then continue to create new handoff.

**If user chooses option 2 (Update existing):**
Add new context as a comment instead:
```bash
bd comments add $EXISTING_HANDOFF_ID "## Update $(date +%Y-%m-%d)

$NEW_CONTEXT"
```
Then skip to Step 5 (output summary) with the existing bead ID.

**If user chooses option 3 (Keep both):**
Continue to create new handoff (warn this creates duplicates).

**If user chooses option 4 (Cancel):**
Exit without creating handoff.

### Step 2: Gather Context

Collect all relevant information from the current session:

1. **What was the goal?** - Original task/problem being worked on
2. **What was done?** - Completed steps, commits made, files changed
3. **What's blocking/remaining?** - Unfinished work, blockers, next steps
4. **Key findings** - Root causes discovered, important decisions made
5. **Related beads** - Any beads created or worked on this session

### Step 2: Get Branch Info

```bash
git rev-parse --abbrev-ref HEAD
```

Store this as `$BRANCH_NAME`.

### Step 3: Create Handoff Bead

Create a bead with comprehensive context:

```bash
bd create \
  --title "Handoff: $BRIEF_SUMMARY" \
  --type task \
  --priority 1 \
  --labels "handoff,$BRANCH_NAME" \
  --description "$(cat <<'EOF'
## Branch
$BRANCH_NAME

## Session Goal
$WHAT_WAS_THE_GOAL

## Completed Work
$WHAT_WAS_DONE

## Remaining Work
$WHAT_IS_LEFT_TO_DO

## Key Findings
$IMPORTANT_DISCOVERIES

## Related Beads
$LIST_OF_RELATED_BEAD_IDS

## Files Changed
$KEY_FILES_MODIFIED

## Handoff Reason
$ARGUMENTS or "Session ended"
EOF
)"
```

### Step 4: Link Related Beads (if any)

If there are related beads that should be linked:

```bash
bd dep add $HANDOFF_BEAD_ID $RELATED_BEAD_ID
```

### Step 5: Output Summary

```
═══════════════════════════════════════════════════════════════
 HANDOFF COMPLETE
═══════════════════════════════════════════════════════════════

 Handoff Bead:    $HANDOFF_BEAD_ID
 Branch:          $BRANCH_NAME
 Label:           handoff

 To resume in new session:
 ┌─────────────────────────────────────────────────────────────┐
 │  /jb-pickup                                                 │
 │                                                             │
 │  Or manually:                                               │
 │  bd list --label handoff --label $BRANCH_NAME               │
 │  bd show $HANDOFF_BEAD_ID                                   │
 └─────────────────────────────────────────────────────────────┘

 You can now safely clear context or end session.

═══════════════════════════════════════════════════════════════
```

## Quality Checklist

Before creating the handoff bead, ensure:

- [ ] Description is detailed enough to resume without prior context
- [ ] All relevant file paths are included
- [ ] Related bead IDs are referenced
- [ ] Next steps are actionable and clear
- [ ] Any error messages or stack traces are captured if relevant
