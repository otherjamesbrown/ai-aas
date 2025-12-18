# Task Generation Command

Generate an actionable task list from the implementation plan, organized by user stories, and create beads to track implementation.

## Instructions

### Step 1: Load Context
Read the required design documents:
- `specs/[feature]/plan.md` - Implementation plan
- `specs/[feature]/spec.md` - Feature specification
- `specs/[feature]/impact.md` - Impact analysis (**if exists** - for migrations)
- `specs/[feature]/data-model.md` - Data model (if exists)
- `specs/[feature]/contracts/` - API contracts (if exists)

### Step 2: Parse User Stories
Extract user stories from `spec.md`, noting their priorities (P1, P2, P3).

### Step 3: Generate Tasks
Create `specs/[feature]/tasks.md` with tasks organized by user story.

**Task Format** (mandatory):
```
- [ ] [TaskID] [P?] [Story?] [Action?] Description with file path
```

Where:
- `TaskID`: Sequential (T001, T002, T003...)
- `[P]`: Marks parallelizable tasks
- `[Story]`: User story reference (US1, US2, etc.)
- `[Action]`: **For migrations only** - one of:
  - `[ADD]` - Create new file/code
  - `[MODIFY]` - Change existing file/code
  - `[REMOVE]` - Delete file/code
  - `[DEPRECATE]` - Mark for future removal
  - `[UPDATE]` - Docs/tests sync
- Description: What to do, including exact file paths

### Step 4: Organize by Phase

**For standard features:**

```markdown
# Tasks: [Feature Name]

**Feature**: `[NNN]-[feature-name]`
**Generated**: [date]
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

## Phase N-1: Documentation

### Human Documentation (`/docs/`)
- [ ] T0XX [UPDATE] Update `docs/operators/ai-model-operator.md`
- [ ] T0XX [UPDATE] Add runbook `docs/runbooks/[feature].md`

### Agent Context (`/context/`) - CRITICAL
- [ ] T0XX [UPDATE] Update `context/[agent]/agents.md` with new patterns

## Phase N: Polish & Integration
- [ ] T0XX [P] Add integration tests
```

**For migrations/refactors** (when `impact.md` exists):

```markdown
# Tasks: [Feature Name]

**Feature**: `[NNN]-[feature-name]`
**Generated**: [date]
**Source**: plan.md, spec.md, impact.md
**Type**: Migration

## Phase 1: Prepare (Backward Compatible)
- [ ] T001 [ADD] Add DeploymentMode field to `path/to/types.go`
- [ ] T002 [ADD] Add new builder method in `path/to/builder.go`
- [ ] T003 [P] [UPDATE] Regenerate CRDs

## Phase 2: Implement
- [ ] T004 [MODIFY] Update controller logic in `path/to/controller.go`
- [ ] T005 [ADD] Add HPA support in `path/to/hpa.go`
- [ ] T006 [UPDATE] Add tests for new behavior

## Phase 3: Migrate
- [ ] T007 [MODIFY] Update existing recipes to use explicit mode
- [ ] T008 [ADD] Run migration script
- [ ] T009 [UPDATE] Validate existing deployments

## Phase 4: Documentation

### Human Documentation (`/docs/`)
- [ ] T010 [UPDATE] Update `docs/operators/[component].md`
- [ ] T011 [UPDATE] Add/update runbook `docs/runbooks/[feature].md`
- [ ] T012 [UPDATE] Update platform guides `docs/platform/[topic].md`

### Agent Context (`/context/`) - CRITICAL
- [ ] T013 [UPDATE] Update `context/[agent]/agents.md` with new patterns/anti-patterns

## Phase 5: Cleanup
- [ ] T014 [REMOVE] Delete implicit nodeSelector logic from `path/to/old.go`
- [ ] T015 [REMOVE] Remove stale Knative revisions
- [ ] T016 [DEPRECATE] Mark old config for removal in `path/to/config.yaml`

## Rollback Checkpoints
<!-- From impact.md -->
- After Phase 1: Remove new CRD fields
- After Phase 2: Revert controller code
- After Phase 3: Set deploymentMode: Serverless
- After Phase 4: N/A (cleanup only)
```

### Step 5: Identify Dependencies
Add a dependency section if tasks have prerequisites:
```markdown
## Dependencies
- T006 depends on T003, T004
- T007 depends on T006
```

### Step 6: Create Epic Bead
Create an epic bead to track the overall feature implementation.

**Epic ID Format**: `ai-aas-spec[NNN]` where `[NNN]` is the spec number (e.g., `ai-aas-spec030` for `specs/030-gpu-deployment-mode/`).

```bash
bd create "[Feature Name] Implementation" --id ai-aas-spec[NNN] --type epic --priority 2
```

**Example** for spec 030:
```bash
bd create "GPU Deployment Mode Migration" --id ai-aas-spec030 --type epic --priority 2
```

Add metadata to the epic:
- Link to spec: `specs/[feature]/spec.md`
- Link to plan: `specs/[feature]/plan.md`
- Link to tasks: `specs/[feature]/tasks.md`

The epic ID will be `ai-aas-spec[NNN]` (e.g., `ai-aas-spec030`).

### Step 7: Create Task Beads
For each task in `tasks.md`, create a bead with:

```bash
bd create "[TaskID] [Description]" --type task --priority [1-3]
```

**Required Labels** (add with `bd label add <id> <label>`):

1. **Agent Label** - Who should work on this:
   - `agent:go-services` - Go service code (handlers, business logic)
   - `agent:infra-ops` - Kubernetes, Helm, ArgoCD, deployment
   - `agent:cli` - CLI commands and client code
   - `agent:operator` - Kubernetes operator code
   - `agent:general` - Everything else (frontend, scripts, docs)

2. **Component Label** - What it touches:
   - `component:api-router` - API Router Service
   - `component:admin-api` - Admin API Service
   - `component:user-org` - User/Org Service
   - `component:analytics` - Analytics Service
   - `component:web-portal` - Web Portal (React)
   - `component:cli` - CLI tool
   - `component:gitops` - GitOps/ArgoCD configs
   - `component:shared` - Shared libraries
   - `component:infra` - Infrastructure/Helm charts

3. **Change Type Label** (for migrations only):
   - `change:add` - Creating new code/config
   - `change:modify` - Changing existing code/config
   - `change:remove` - Deleting code/config
   - `change:deprecate` - Marking for future removal

**Agent Selection Rules:**
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

```markdown
## Task Generation Complete

**Feature**: [NNN]-[feature-name]
**Epic Bead**: ai-aas-spec[NNN] - [Feature Name] Implementation
**Type**: Feature | Migration

### Tasks Created
| Bead ID | Task | Agent | Component | Change | Dependencies |
|---------|------|-------|-----------|--------|--------------|
| AIAAS-XX | T001 Setup directories | general | infra | add | - |
| AIAAS-XX | T002 Add Go types | go-services | api-router | add | - |
| AIAAS-XX | T003 Implement handler | go-services | api-router | modify | T002 |

**Total**: X tasks across Y phases
**Parallel opportunities**: Z tasks can run concurrently

<!-- For migrations only -->
### Change Summary
| Change Type | Count |
|-------------|-------|
| ADD | X |
| MODIFY | Y |
| REMOVE | Z |
| DEPRECATE | W |

### Migration Phases
| Phase | Tasks | Risk |
|-------|-------|------|
| 1. Prepare | T001-T003 | LOW |
| 2. Implement | T004-T006 | MEDIUM |
| 3. Migrate | T007-T009 | HIGH |
| 4. Cleanup | T010-T013 | LOW |

**Next Step**: `/speckit.implement ai-aas-spec[NNN]` (epic bead ID)
```

## Key Constraints
- Every task must have exact file paths
- Tasks must be immediately actionable without additional context
- Tests come before implementation within each story phase
- Every task bead must have both agent and component labels
- Dependencies in tasks.md must be reflected in bead dependencies

**For migrations:**
- Every task must have a change type label (add/modify/remove/deprecate)
- REMOVE tasks must come AFTER replacement is implemented
- Phase order must match impact.md migration_order
- Rollback checkpoints must be documented

## Documentation Task Requirements

### `/docs/` - Human Documentation
Standard markdown documentation for human readers:
- Clear explanations with context
- Examples and use cases
- Troubleshooting sections
- Can use prose paragraphs

### `/context/` - Agent Context (CRITICAL)

**Purpose**: Loaded into AI agent context window. NOT for human reading.

**Format Requirements** (from `context/context_map.md`):

```yaml
structure:
  - YAML for: architecture, configs, hierarchies
  - Tables for: lookups, mappings, comparisons
  - Bullets for: rules, lists, requirements
  - Code blocks for: commands, examples

writing_style:
  keywords: MUST, NEVER, ALWAYS  # Not "should", "consider"
  format: Structured data > prose
  diagrams: No ASCII (use YAML instead)
  duplication: Link to source files, don't copy

constraints:
  rules_type: max 200 lines
  reference_type: max 300 lines
  required_metadata:
    - "last_verified: YYYY-MM-DD"
    - "type: rules|reference|operational"

anti_patterns:
  - Show WRONG examples only
  - No CORRECT examples (agent knows the rules)
  - No prose explanations
```

**Example Context Update** (for `context/operator-developer/agents.md`):

```yaml
# Add to patterns section:
deployment_mode:
  rule: Use explicit deploymentMode, not implicit nodeSelector inference
  options:
    - Serverless: Knative, scale-to-zero (CPU workloads)
    - RawDeployment: Standard K8s Deployment (GPU workloads)
  default_by_runtime:
    tensorrt-llm: RawDeployment
    triton: RawDeployment
    vllm_with_gpu: RawDeployment
    vllm_cpu: Serverless
    tgi: Serverless
```

```go
// Add to anti-patterns section:
// WRONG: Implicit deployment mode from nodeSelector
deploymentMode := "Serverless"
if len(b.nodeSelector) > 0 {
    deploymentMode = "RawDeployment"
}
```

**Create Documentation Bead**: Always create a bead for documentation tasks:

```bash
bd create "Update context/[agent]/agents.md for [feature]" \
  --type task \
  --priority 2 \
  --labels "agent:general,component:docs,change:update"
```

## User Input
$ARGUMENTS
