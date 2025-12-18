# Bead Todo (Work Dashboard)

Present actionable bead work with options to investigate or assign agents.

## Arguments: $ARGUMENTS

Options: `--label=X`, `--type=X`, `--priority=N`

## Instructions

### Step 1: Gather Bead Statistics

```bash
bd stats
bd list --status=open --json
```

Categorize open beads:
- Agent-ready (has `agent-ready` label)
- Needs investigation (has `needs-investigation` label)
- Needs spec (has `needs-spec` label)
- Blocked (has blockers)
- Uncategorized (no agent labels)

### Step 2: Show Dashboard

```
═══════════════════════════════════════════════════
 /jb-bd-todo - Work Dashboard
═══════════════════════════════════════════════════

 Open Beads: $TOTAL

 By Status:
   🟢 Agent-Ready:       $COUNT  (can assign agents now)
   🔍 Needs Investigation: $COUNT  (need more research)
   📝 Needs Spec:         $COUNT  (need specification)
   🚫 Blocked:            $COUNT  (waiting on dependencies)
   ⚪ Uncategorized:      $COUNT  (run /jb-bd-tidy first)

 By Priority:
   P0 Critical:  $COUNT
   P1 High:      $COUNT
   P2 Medium:    $COUNT
   P3 Low:       $COUNT
   P4 Backlog:   $COUNT

 By Type:
   🐛 Bugs:     $COUNT
   📋 Tasks:    $COUNT
   ✨ Features: $COUNT

═══════════════════════════════════════════════════

 What would you like to do?

 [1] 🤖 Assign agents to fix agent-ready beads
 [2] 🔍 Investigate beads needing research
 [3] 📝 Specify beads needing specs
 [4] 🧹 Run /jb-bd-tidy on uncategorized beads
 [5] 📊 Show detailed list
 [6] ❌ Exit

═══════════════════════════════════════════════════
```

### Step 3: Handle User Choice

#### Option 1: Assign Agents

List agent-ready beads and offer to spawn agents:

```bash
bd list --status=open --label=agent-ready
```

```
═══════════════════════════════════════════════════
 Agent-Ready Beads ($COUNT)
═══════════════════════════════════════════════════

 Quick Fixes ($QUICK_COUNT):
   $BEAD_ID: $TITLE [backend] [quick-fix]
   $BEAD_ID: $TITLE [cli] [quick-fix]

 Standard ($STANDARD_COUNT):
   $BEAD_ID: $TITLE [backend] [P1]
   $BEAD_ID: $TITLE [api] [P2]

 Complex ($COMPLEX_COUNT):
   $BEAD_ID: $TITLE [backend] [complex]

═══════════════════════════════════════════════════

 Options:
 [A] Assign agent to specific bead: $BEAD_ID
 [B] Assign agents to all quick-fixes (parallel)
 [C] Assign agent to highest priority
 [D] Back to dashboard

═══════════════════════════════════════════════════
```

When assigning an agent:
```bash
# Mark as in progress
bd update $BEAD_ID --status=in_progress

# Spawn agent with context
# Use Task tool to spawn implementation agent with:
# - Bead title and description
# - Investigation comments
# - Related files
# - Clear success criteria
```

#### Option 2: Investigate Beads

List beads needing investigation:

```bash
bd list --status=open --label=needs-investigation
```

For each, perform deep investigation (similar to /jb-bd-tidy Step 3):
- Search codebase
- Read related code
- Check git history
- Update bead with findings
- Re-categorize (agent-ready, needs-spec, or close if resolved)

#### Option 3: Specify Beads

List beads needing specification:

```bash
bd list --status=open --label=needs-spec
```

For each:
- Show current context
- Ask clarifying questions
- Write detailed specification as comment
- Update labels: remove `needs-spec`, add `agent-ready`

#### Option 4: Run Tidy

Invoke /jb-bd-tidy for uncategorized beads:
```
/jb-bd-tidy --uncategorized-only
```

#### Option 5: Detailed List

Show full list with filters:

```bash
bd list --status=open --priority=1  # High priority
bd list --status=open --type=bug    # All bugs
bd list --status=open --label=backend  # Backend work
```

### Step 4: Agent Assignment Template

When spawning an agent to fix a bead:

```markdown
## Task: Fix $BEAD_ID - $TITLE

### Context
$BEAD_DESCRIPTION

### Investigation Notes
$COMMENTS_FROM_TIDY

### Files to Modify
$FILE_LIST

### Success Criteria
- [ ] Issue is resolved
- [ ] Tests pass
- [ ] No regressions introduced

### When Complete
```bash
bd close $BEAD_ID --reason="Fixed in commit $SHA"
```
```

Use the Task tool with subagent_type=general-purpose to spawn the agent.

### Step 5: Track Agent Work

After assigning agents:
```bash
bd comments add $BEAD_ID "Assigned to agent at $(date)"
```

Monitor progress:
```bash
bd list --status=in_progress
```

## Notes

- Run `/jb-bd-tidy` first to categorize beads
- Quick-fixes are good candidates for parallel agent work
- Complex beads may need human review before agent assignment
- Always verify agent work before closing beads
