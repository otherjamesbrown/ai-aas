# Task Generation Command

Generate an actionable task list from the implementation plan, organized by user stories, and create beads to track implementation.

## Instructions

### Step 0: Load Project Configuration

Check for project-specific configuration:
```bash
cat specs/.speckit/config.yaml 2>/dev/null
```

If exists, load:
- `agents` - Agent definitions with file patterns and context files
- `components` - Component labels for categorization
- `beads_prefix` - Project prefix for bead IDs

Use config values for agent/component labeling. If no config, use defaults.

### Step 1: Load Context

Read the required design documents:
- `specs/[feature]/plan.md` - Implementation plan
- `specs/[feature]/spec.md` - Feature specification
- `specs/[feature]/data-model.md` - Data model (if exists)
- `specs/[feature]/contracts/` - API contracts (if exists)
- `specs/[feature]/impact.md` - Impact analysis (if exists)

### Step 2: Parse User Stories

Extract user stories from `spec.md`, noting their priorities (P1, P2, P3).

### Step 3: Generate Tasks

Create `specs/[feature]/tasks.md` with tasks organized by user story.

**Task Format** (mandatory):
```
- [ ] [TaskID] [P?] [Story?] Description with file path
```

Where:
- `TaskID`: Sequential (T001, T002, T003...)
- `[P]`: Marks parallelizable tasks (tasks with no shared file dependencies)
- `[Story]`: User story reference (US1, US2, etc.)
- Description: What to do, including exact file paths

### Step 4: Organize by Phase

```markdown
# Tasks: [Feature Name]

**Feature**: `[NNN]-[feature-name]`
**Generated**: YYYY-MM-DD
**Source**: plan.md, spec.md

## Phase 1: Setup & Prerequisites
- [ ] T001 [P] Create directory structure
- [ ] T002 [P] Add dependencies to go.mod/package.json

## Phase 2: Foundation (blocks all stories)
- [ ] T003 Define base types in `path/to/types.go`
- [ ] T004 Create database migrations in `path/to/migrations/`

## Phase 3: User Story 1 - [US1 Title] (P1)
### Tests
- [ ] T005 [US1] Write tests for [component] in `path/to/test.go`

### Implementation
- [ ] T006 [US1] Implement [component] in `path/to/file.go`
- [ ] T007 [P] [US1] Add API endpoint in `path/to/handler.go`

## Phase 4: User Story 2 - [US2 Title] (P2)
...

## Phase N: Polish & Integration
- [ ] T0XX [P] Add integration tests
- [ ] T0XX Update documentation
```

### Step 5: Identify Dependencies

Add a dependency section if tasks have prerequisites:
```markdown
## Dependencies
- T006 depends on T003, T004
- T007 depends on T006
```

### Step 6: Create Epic Bead

Create an epic bead to track the overall feature implementation:

```bash
bd create "[Feature Name] Implementation" --type epic --priority 2
```

Add metadata to the epic:
- Link to spec: `specs/[feature]/spec.md`
- Link to plan: `specs/[feature]/plan.md`
- Link to tasks: `specs/[feature]/tasks.md`

Note the epic ID (e.g., `aas-42`).

### Step 7: Create Task Beads

For each task in `tasks.md`, create a bead with:

```bash
bd create "[TaskID] [Description]" --type task --priority [1-3]
```

**Required Labels** (add with `bd label add <id> <label>`):

1. **Agent Label** - From config `agents` section, or use defaults:

   | Agent | Description |
   |-------|-------------|
   | `agent:go-services` | Go service code (handlers, business logic) |
   | `agent:infra-ops` | Kubernetes, Helm, ArgoCD, deployment |
   | `agent:cli` | CLI commands and client code |
   | `agent:operator` | Kubernetes operator code |
   | `agent:general` | Everything else (frontend, scripts, docs) |

2. **Component Label** - From config `components` section

**Agent Selection Rules** (from config or defaults):

Match file paths in task to agent patterns:

| File Path Pattern | Agent Label |
|-------------------|-------------|
| `services/*-service/**/*.go` | `agent:go-services` |
| `gitops/**`, `**/helm/**`, `**/k8s/**` | `agent:infra-ops` |
| `cmd/ai-aas-cli/**`, `internal/cli/**` | `agent:cli` |
| `operators/**` | `agent:operator` |
| `web/**`, `*.md`, `scripts/**` | `agent:general` |

### Step 8: Add Bead Dependencies

Map task dependencies to bead dependencies:

```bash
# If T006 depends on T003, T004:
bd dep add <T006-bead-id> <T003-bead-id>
bd dep add <T006-bead-id> <T004-bead-id>
```

### Step 9: Add Bead Context

For each task bead, add a comment with implementation context:

```bash
bd comments <bead-id> --add "## Context
**Spec Reference**: specs/[feature]/spec.md §[section]
**Files to Modify**:
- path/to/file1.go
- path/to/file2.go

**Acceptance Criteria**:
- [criteria from spec]

**Technical Notes**:
- [relevant notes from plan.md or research.md]
"
```

### Step 10: Link Tasks to Epic

Add all task beads as blockers of the epic (epic is blocked until all tasks done):

```bash
bd dep add <epic-id> <task-bead-id>
```

### Step 11: Report

Output summary:

```
═══════════════════════════════════════════════════
 speckit.tasks - COMPLETE
═══════════════════════════════════════════════════

 Feature:          [NNN]-[feature-name]
 Epic Bead:        [PREFIX]-XX - [Feature Name] Implementation

 Files Created:
   - tasks.md

 Tasks Created:
   | Bead ID | Task | Agent | Component | Dependencies |
   |---------|------|-------|-----------|--------------|
   | XX | T001 Setup directories | general | infra | - |
   | XX | T002 Add Go types | go-services | api-router | - |
   | XX | T003 Implement handler | go-services | api-router | T002 |

 Summary:
   - Total tasks: [count]
   - Phases: [count]
   - Parallel opportunities: [count] tasks can run concurrently

 Next Steps:
   - Review tasks.md for completeness
   - Run /speckit.analyze to validate consistency
   - Run /speckit.implement [epic-id] to start implementation
═══════════════════════════════════════════════════
```

## Key Constraints

- Every task must have exact file paths
- Tasks must be immediately actionable without additional context
- Tests come before implementation within each story phase
- Every task bead must have both agent and component labels
- Dependencies in tasks.md must be reflected in bead dependencies
- Use ISO date format: YYYY-MM-DD

## User Input
$ARGUMENTS
