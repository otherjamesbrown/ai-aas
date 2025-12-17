# Bead Tidy (Prune & Categorize)

AI-powered review of stale beads to prune resolved issues and prepare remaining work for agents.

## Arguments: $ARGUMENTS

Options: `--age=N` (default 7 days), `--type=bug|task|feature|all`, `--dry-run`

## Instructions

### Step 1: Parse Arguments

- `--age=N` - Review beads older than N days (default: 7)
- `--type=X` - Filter by type (default: all)
- `--dry-run` - Show what would happen without making changes

### Step 2: Find Stale Beads

```bash
bd list --status=open --json | jq '[.[] | select(.created_at < (now - (7 * 24 * 3600)))]'
```

Or manually filter beads older than the threshold.

### Step 3: For Each Bead - Investigate

For each stale bead, perform deep investigation:

#### 3a. Parse the Bead
- Extract keywords, file references, function names from title
- Read any comments for additional context

#### 3b. Search Codebase
```bash
# Search for related files
grep -r "keyword" --include="*.go" services/
glob "**/*relevant*"
```

#### 3c. Read Related Code
- Read the files identified
- Look for the specific issue/feature mentioned

#### 3d. Check Git History
```bash
git log --oneline --since="14 days ago" -- <related-files>
git log --oneline --grep="<bead-id or keywords>"
```

#### 3e. Make Determination

Based on investigation, classify as:

| Verdict | Meaning | Action |
|---------|---------|--------|
| `RESOLVED` | Code shows issue is fixed | Close bead |
| `SUPERSEDED` | Code refactored/removed, bead N/A | Close bead |
| `STILL_OPEN` | Issue still exists in code | Keep & categorize |
| `NEEDS_REVIEW` | Can't determine automatically | Flag for human |

### Step 4: Present Findings

For each bead, show:

```
═══════════════════════════════════════════════════
 Investigating: $BEAD_ID "$TITLE"
 Age: $DAYS days | Type: $TYPE | Priority: $PRIORITY
═══════════════════════════════════════════════════

 Search: "$KEYWORDS" in codebase...
 Found: $FILE_PATH

 Git history (last 14d):
   $COMMIT_LIST

 Code analysis:
   $RELEVANT_CODE_SNIPPET
   $ANALYSIS

 Verdict: $VERDICT
   $REASONING

 [Close] [Keep & Categorize] [Skip]
═══════════════════════════════════════════════════
```

### Step 5: Close Resolved Beads

For RESOLVED or SUPERSEDED:
```bash
bd close $BEAD_ID --reason="$VERDICT: $REASONING"
```

### Step 6: Categorize Kept Beads

For STILL_OPEN beads, ensure proper categorization for agents:

#### 6a. Verify/Update Type
- `bug` - Something broken that needs fixing
- `task` - Work item, improvement, or chore
- `feature` - New functionality

```bash
bd update $BEAD_ID --type=$CORRECT_TYPE
```

#### 6b. Set Priority
- `0` - Critical (P0) - Production issue
- `1` - High (P1) - Important, do soon
- `2` - Medium (P2) - Normal priority
- `3` - Low (P3) - Nice to have
- `4` - Backlog (P4) - Someday/maybe

```bash
bd update $BEAD_ID --priority=$PRIORITY
```

#### 6c. Add Agent Labels

| Label | When to Use |
|-------|-------------|
| `agent-ready` | Has enough context for agent to work |
| `needs-spec` | Needs more specification first |
| `needs-investigation` | Requires more research |
| `quick-fix` | Simple, isolated change (<30 min) |
| `complex` | Multi-file or architectural change |
| `backend` | Backend/service code |
| `cli` | CLI tool changes |
| `api` | API changes |
| `infra` | Infrastructure/GitOps |
| `docs` | Documentation |

```bash
bd label add $BEAD_ID agent-ready
bd label add $BEAD_ID backend
bd label add $BEAD_ID quick-fix
```

#### 6d. Add Context Comment

Add investigation findings as comment so agents have context:
```bash
bd comments add $BEAD_ID "Investigation findings:
- File: $FILE_PATH
- Issue: $DESCRIPTION
- Suggested approach: $SUGGESTION"
```

### Step 7: Show Summary

```
═══════════════════════════════════════════════════
 /jb-bd-tidy - COMPLETE
═══════════════════════════════════════════════════

 Beads Reviewed:    $TOTAL

 Closed:            $CLOSED_COUNT
   $CLOSED_LIST

 Kept & Categorized: $KEPT_COUNT
   $KEPT_LIST

 Skipped:           $SKIPPED_COUNT

 Agent-Ready Beads: $AGENT_READY_COUNT
   Use /jb-bd-todo to assign agents

═══════════════════════════════════════════════════
```

## Notes

- Run regularly (weekly) to keep backlog clean
- Investigation uses Claude's code analysis - trust but verify
- Always add context comments for agents
- After tidy, use `/jb-bd-todo` to work on remaining items
