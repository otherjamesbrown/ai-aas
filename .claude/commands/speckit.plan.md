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
- `specs/[feature]/impact.md` - Impact analysis (**if exists** - for migrations/refactors)
- `memory/constitution.md` - Project principles (if exists)
- `CLAUDE.md` or `ARCHITECTURE.md` - Project architecture

**If `impact.md` exists**, this is a migration/refactoring spec:
- Phases must align with migration order from impact.md
- Include REMOVE/DEPRECATE tasks in plan
- Reference affected files from impact analysis

**If `impact.md` does NOT exist** and spec involves:
- Removing existing functionality
- Migrating between approaches
- Refactoring existing patterns

Then **STOP** and recommend running `/speckit.impact` first.

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
**Impact Analysis**: [link to impact.md or "N/A - greenfield feature"]
**Type**: Feature | Migration | Refactor

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

### Files to Add
[New files to create]

### Files to Modify
[Existing files to change - reference impact.md if available]

### Files to Remove
[Files to delete - only for migrations/refactors, from impact.md]

## Phases
[Breakdown of implementation phases]

<!-- For migrations/refactors, align with impact.md migration_order -->
```

**For Migration/Refactor specs** (when `impact.md` exists), add:

```markdown
## Migration Strategy

### Phase 1: Prepare (Backward Compatible)
[Changes that don't break existing behavior]
- Add new fields/types
- Feature flags if needed

### Phase 2: Implement
[New functionality]
- New code paths
- Updated tests

### Phase 3: Migrate
[Switch to new behavior]
- Update existing resources
- Data migrations

### Phase 4: Cleanup
[Remove old code]
- Delete deprecated code
- Remove feature flags
- Update documentation

## Rollback Plan
[From impact.md - how to revert each phase]

## Risk Mitigation
[HIGH risk items from impact.md and how to address them]
```

### Step 6: Validate
Ensure:
- All `[NEEDS CLARIFICATION]` from spec are resolved
- Plan aligns with project architecture
- No implementation details that contradict constitution

**For Migration/Refactor specs**, also validate:
- All REMOVE items from `impact.md` are addressed in plan
- Migration phases are safe to execute sequentially
- Rollback plan exists for each phase
- HIGH risk items have mitigation strategies

### Step 7: Report
Output:
- Generated artifacts list
- Branch information
- **For migrations**: Summary of REMOVE/MODIFY/ADD counts
- Next step: `/speckit.tasks`

## User Input
$ARGUMENTS
