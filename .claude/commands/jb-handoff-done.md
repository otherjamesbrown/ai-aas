# Complete Handoff

Close a handoff bead and optionally close related beads.

## Arguments: $ARGUMENTS

Optional: Handoff bead ID. If not provided, finds the in-progress handoff for current branch.

## Instructions

### Step 1: Find Handoff Bead

If `$ARGUMENTS` provided and looks like a bead ID:
```bash
bd show $ARGUMENTS
```

Otherwise, find the in-progress handoff for current branch:
```bash
BRANCH=$(git rev-parse --abbrev-ref HEAD)
bd list --label handoff --label $BRANCH --status in_progress
```

If no in-progress handoff found, check for open ones:
```bash
bd list --label handoff --label $BRANCH --status open
```

If still nothing found:
```
No active handoff found for branch '$BRANCH'.

To find all handoffs: bd list --label handoff
```

### Step 2: Show Handoff Summary

```bash
bd show $HANDOFF_BEAD_ID
```

Display:
```
───────────────────────────────────────────────────────────────
 CLOSING HANDOFF: $HANDOFF_BEAD_ID
───────────────────────────────────────────────────────────────
 Title:    $TITLE
 Branch:   $BRANCH
 Created:  $TIMESTAMP

 Related Beads:
   $RELATED_BEAD_LIST
───────────────────────────────────────────────────────────────
```

### Step 3: Check Related Beads

Extract related beads from the handoff description. For each one:

```bash
bd show $RELATED_BEAD_ID --json | jq -r '.status'
```

Categorize them:
- **Completed but open**: Should probably be closed
- **Still in progress**: May need attention
- **Already closed**: No action needed

### Step 4: Prompt for Related Bead Actions

If there are related beads that are still open:

```
Related beads still open:

  ai-aas-xxxx (in_progress) - "Fix routing issue"
  ai-aas-yyyy (open)        - "Update documentation"

Close these beads as well? [y/n/select]
  y      - Close all related beads
  n      - Keep them open
  select - Choose which to close
```

Use AskUserQuestion tool if needed.

### Step 5: Close Beads

Close the handoff bead:
```bash
bd close $HANDOFF_BEAD_ID --reason "Handoff work completed"
```

If user chose to close related beads:
```bash
bd close $RELATED_BEAD_1 $RELATED_BEAD_2 ...
```

### Step 6: Output Confirmation

```
═══════════════════════════════════════════════════════════════
 HANDOFF COMPLETE
═══════════════════════════════════════════════════════════════

 Closed:
   ✓ $HANDOFF_BEAD_ID - Handoff bead
   ✓ $RELATED_BEAD_1  - $TITLE
   ✓ $RELATED_BEAD_2  - $TITLE

 Still open (kept):
   • $OTHER_BEAD - $TITLE

═══════════════════════════════════════════════════════════════
```

## Edge Cases

- **No handoff found**: Suggest `bd list --label handoff` to find manually
- **Multiple in-progress handoffs**: List them and ask which to close
- **Handoff already closed**: Inform user, no action needed
