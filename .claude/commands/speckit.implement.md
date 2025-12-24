# Implementation Command

Execute tasks by working through beads and spawning specialized sub-agents.

## Instructions

### Step 0: Load Project Configuration

Check for project-specific configuration:
```bash
cat specs/.speckit/config.yaml 2>/dev/null
```

If exists, load:
- `agents` - Agent definitions with context files to load
- `commands` - Test/lint/build commands

### Step 1: Identify Epic

Find the epic bead to implement. Either:
- Use the epic ID passed as argument: `$ARGUMENTS`
- Or list available epics and ask user to select:
  ```bash
  bd list --type epic --status open
  ```

### Step 2: Load Context

Read the spec artifacts linked from the epic:
- `specs/[feature]/tasks.md` - Task list
- `specs/[feature]/plan.md` - Implementation plan
- `specs/[feature]/spec.md` - Feature specification
- `specs/[feature]/data-model.md` - Data model (if exists)
- `specs/[feature]/contracts/` - API contracts (if exists)
- `specs/[feature]/research.md` - Technical decisions (if exists)
- `specs/[feature]/impact.md` - Impact analysis (if exists)

### Step 3: Checklist Validation

If `specs/[feature]/checklists/` exists, scan completion status:
- Count total items, completed items, incomplete items
- If incomplete items exist, ask whether to proceed

### Step 4: Get Ready Tasks

Find tasks with no unmet dependencies:

```bash
bd ready
```

Filter to tasks belonging to this epic (check if they block the epic).

### Step 5: Execute Ready Tasks

For each ready task bead:

#### 5a. Read Bead Details
```bash
bd show <bead-id>
```

Extract:
- Task description
- Agent label (`agent:go-services`, `agent:infra-ops`, etc.)
- Component label
- Context from comments

#### 5b. Load Agent-Specific Context

From project config, find the agent's context file:

| Agent Label | Context File (from config) |
|-------------|---------------------------|
| `agent:go-services` | `context/go-services-developer/agents.md` |
| `agent:infra-ops` | `context/infra-ops-manager/agents.md` |
| `agent:cli` | `context/cli-developer/agents.md` |
| `agent:operator` | `context/operator-developer/agents.md` |
| `agent:general` | None |

Load the relevant context file before spawning the sub-agent.

#### 5c. Map Agent Label to Sub-Agent Type

| Agent Label | Sub-Agent Type | Description |
|-------------|----------------|-------------|
| `agent:go-services` | `go-services-developer` | Go service code in services/*-service/ |
| `agent:infra-ops` | `infra-ops-manager` | Kubernetes, Helm, ArgoCD, GitOps |
| `agent:cli` | `cli-developer` | CLI commands and API client code |
| `agent:operator` | `operator-developer` | Kubernetes operator code |
| `agent:general` | `general-purpose` | Frontend, docs, scripts, other |

#### 5d. Build Agent Prompt

Construct a detailed prompt for the sub-agent:

```
You are implementing a task from a feature specification.

## Task
**Bead ID**: <bead-id>
**Description**: <task description>
**Component**: <component label>

## Context
<paste bead comments/context here>

## Agent Context
<paste from agent-specific context file>

## Spec Reference
<relevant section from spec.md>

## Technical Guidance
<relevant section from plan.md>

## Files to Modify
<list from bead context>

## Acceptance Criteria
<from spec.md>

## Instructions
1. Read the files listed above to understand current state
2. Implement the task following the technical guidance
3. Run tests to verify your changes: `go test ./...` (or appropriate command)
4. Report what you changed and any issues encountered

Do NOT create a PR or commit. Just implement the changes.
```

#### 5e. Spawn Sub-Agent

Use the Task tool to spawn the appropriate agent:

```
Task(
  subagent_type: "<agent-type>",
  prompt: "<constructed prompt>",
  description: "Implement <task-id>"
)
```

**Parallel Execution Rules:**
- Tasks marked `[P]` with no shared files CAN run in parallel
- Tasks modifying the same files MUST run sequentially
- When in doubt, run sequentially
- Maximum 3 parallel agents at once

#### 5f. Process Agent Result

When the agent completes:

1. **On Success**:
   - Mark bead as completed: `bd close <bead-id> "Implemented by <agent-type>"`
   - Update `tasks.md`: Change `- [ ]` to `- [x]`
   - Report progress to user

2. **On Failure**:
   - Add failure details to bead: `bd comments <bead-id> --add "Failed: <error>"`
   - Ask user whether to:
     - Retry with same agent
     - Try with different agent (e.g., general-purpose)
     - Skip and continue
     - Stop implementation

### Step 6: Iterate

After completing ready tasks:
1. Check for newly unblocked tasks: `bd ready`
2. Repeat Step 5 for each new ready task
3. Continue until no tasks remain or user stops

### Step 7: Validation

After all tasks complete:
- Run full test suite (from config or default): `go test ./...`
- Verify features match specification acceptance criteria
- Check epic bead should now have no blockers

### Step 8: Close Epic

If all tasks completed successfully:
```bash
bd close <epic-id> "All tasks implemented"
```

### Step 9: Report Completion

Output final summary:

```
═══════════════════════════════════════════════════
 speckit.implement - COMPLETE
═══════════════════════════════════════════════════

 Epic:             [epic-id] - [epic title]
 Feature:          [NNN]-[feature-name]

 Task Summary:
   | Bead ID | Task | Agent Used | Status |
   |---------|------|------------|--------|
   | XX | T001 Setup | general-purpose | Done |
   | XX | T002 Types | go-services-developer | Done |
   | XX | T003 Handler | go-services-developer | Done |

 Completed:        [X]/[Y] tasks
 Test Results:     [pass/fail summary]

 Files Changed:
   - path/to/file1.go (created)
   - path/to/file2.go (modified)

 Next Steps:
   - Review changes: `git diff`
   - Run full tests: `go test ./...`
   - Create PR when ready
═══════════════════════════════════════════════════
```

## Error Recovery

If implementation is interrupted:
- Beads track exactly which tasks are done
- Resume with: `/speckit.implement <epic-id>`
- Only incomplete tasks will be executed

## Key Constraints

- Always use sub-agents for implementation work
- Load agent-specific context before spawning sub-agents
- Respect bead dependencies - never skip ahead
- Mark beads complete immediately after agent succeeds
- Never run more than 3 agents in parallel
- Stop on critical failures (compilation errors, test failures)
- Agent failures are recoverable - user decides next action

## Agent Label Fallback

If a bead has no agent label or an unrecognized label:
1. Infer from file paths in the task
2. Default to `general-purpose` if unclear
3. Add inferred label to bead for future reference

## User Input
$ARGUMENTS
