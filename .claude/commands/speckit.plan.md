# Implementation Planning Command

Create a technical implementation plan from an existing feature specification.

## Instructions

Follow this workflow to create an implementation plan:

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

### Step 3: Phase 0 - Research
For each unclear technical requirement:
1. Document the question
2. Research options and alternatives
3. Make a decision with rationale
4. Record in `research.md`

Create `specs/[feature]/research.md` documenting:
- Technical decisions made
- Alternatives considered
- Rationale for choices

### Step 4: Phase 1 - Design
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

### Step 5: Write Implementation Plan
Create `specs/[feature]/plan.md` with:

```markdown
# Implementation Plan: [Feature Name]

**Feature Branch**: `[NNN]-[feature-name]`
**Date**: [ISO date]
**Spec**: [link to spec.md]

## Summary
[Primary requirement + technical approach]

## Technical Context
- Language/Framework: [e.g., Go 1.22, React 18]
- Dependencies: [new packages needed]
- Storage: [database, cache requirements]
- Testing: [framework, approach]

## Constitution Compliance
[How this plan aligns with project principles]

## Project Structure
[Files to be created/modified]

## Phases
[Breakdown of implementation phases]
```

### Step 6: Validate
Ensure:
- All `[NEEDS CLARIFICATION]` from spec are resolved
- Plan aligns with project architecture
- No implementation details that contradict constitution

### Step 7: Report
Output:
- Generated artifacts list
- Branch information
- Next step: `/speckit.tasks`

## User Input
$ARGUMENTS
