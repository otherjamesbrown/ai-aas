# Implementation Planning Command

Create a technical implementation plan from an existing feature specification.

## Instructions

Follow this workflow to create an implementation plan:

### Step 0: Load Project Configuration

Check for project-specific configuration:
```bash
cat specs/.speckit/config.yaml 2>/dev/null
```

If exists, load:
- Path conventions for context and docs
- Architecture file location
- Agent definitions (for later task assignment)

### Step 1: Setup

Identify the feature specification to plan. If not specified, list available specs:
```bash
ls -d specs/*/spec.md
```

### Step 2: Load Context

Read the required files:
- `specs/[feature]/spec.md` - The feature specification
- `memory/constitution.md` - Project principles (if exists)
- `CLAUDE.md` or `ARCHITECTURE.md` - Project architecture

**Context-aware loading** (from config paths):
- `context/agents.md` - Agent routing rules
- `context/context_map.md` - Context structure
- Relevant agent-specific context based on spec scope

### Step 3: Check for Impact Analysis

```bash
cat specs/[feature]/impact.md 2>/dev/null
```

If `impact.md` exists:
- Load migration phases from impact analysis
- Plan phases should align with migration order
- Include REMOVE/DEPRECATE tasks from impact
- Reference file paths and risk levels from impact.md

If `impact.md` doesn't exist and spec contains migration signals ("migrate", "replace", "remove", "deprecate"), suggest running `/speckit.impact` first.

### Step 4: Phase 0 - Research

For each unclear technical requirement:
1. Document the question
2. Research options and alternatives
3. Make a decision with rationale
4. Record in `research.md`

Create `specs/[feature]/research.md` documenting:
- Technical decisions made
- Alternatives considered
- Rationale for choices

### Step 5: Phase 1 - Design

Generate concrete design outputs:

**Data Model** (`data-model.md`):
- Entity definitions with fields and types
- Validation rules
- Relationships

**API Contracts** (`contracts/` directory):
- Request/response schemas
- Endpoint definitions
- Error responses

**Quickstart** (`quickstart.md`):
- How to use/test the feature
- Example commands or API calls

### Step 6: Write Implementation Plan

Create `specs/[feature]/plan.md` with:

```markdown
# Implementation Plan: [Feature Name]

**Feature Branch**: `[NNN]-[feature-name]`
**Date**: YYYY-MM-DD
**Spec**: [link to spec.md]
**Impact Analysis**: [link to impact.md or "N/A - greenfield"]

## Summary
[Primary requirement + technical approach]

## Technical Context
- Language/Framework: [e.g., Go 1.22, React 18]
- Dependencies: [new packages needed]
- Storage: [database, cache requirements]
- Testing: [framework, approach]

## Architecture Fit
[How this fits with existing systems from /context/]

## Constitution Compliance
[How this plan aligns with project principles]

## Project Structure
[Files to be created/modified]

## Phases

### Phase 1: [Name]
- Description: [what this phase accomplishes]
- Tasks: [high-level task list]
- Risk: [LOW/MEDIUM/HIGH]
- Rollback: [how to undo if needed]

### Phase 2: [Name]
...
```

If `impact.md` exists, align phases with migration order from impact analysis.

### Step 7: Validate

Ensure:
- All `[NEEDS CLARIFICATION]` from spec are resolved
- Plan aligns with project architecture
- No implementation details that contradict constitution
- Context updates are planned for `/context/` if needed
- Documentation updates are planned for `/docs/` if needed

### Step 8: Report

Output completion summary:

```
═══════════════════════════════════════════════════
 speckit.plan - COMPLETE
═══════════════════════════════════════════════════

 Spec:             [NNN]-[feature-name]

 Files Created:
   - plan.md
   - research.md
   - data-model.md (if applicable)
   - contracts/ (if applicable)

 Plan Summary:
   - Phases: [count]
   - Components affected: [list]
   - Risk profile: [HIGH/MEDIUM/LOW]

 Impact Analysis:  [Incorporated / N/A]

 Next Steps:
   - Review plan.md
   - Run /speckit.tasks to create task breakdown
═══════════════════════════════════════════════════
```

## Key Constraints

- Use ISO date format: YYYY-MM-DD
- Plan must align with existing architecture in `/context/`
- If impact.md exists, phases must align with migration order

## User Input
$ARGUMENTS
