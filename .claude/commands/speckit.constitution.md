# Constitution Command

Create or update the project constitution (`memory/constitution.md`) - the governing principles that all specifications must comply with.

## Instructions

### Step 1: Check Existing Constitution
Look for existing constitution:
```bash
ls memory/constitution.md 2>/dev/null || echo "No constitution found"
```

Also check `CLAUDE.md` and `ARCHITECTURE.md` for existing principles.

### Step 2: Load Template
If creating new, use this structure:

```markdown
# Project Constitution

**Project**: [PROJECT_NAME]
**Version**: 1.0.0
**Last Updated**: YYYY-MM-DD

## Preamble
[Brief description of what this project is and its core mission]

## Article I - [Principle Name]
[Declarative, testable principle]

**Rationale**: [Why this principle exists]
**Compliance**: [How to verify compliance]

## Article II - [Principle Name]
...

## Amendment Process
[How principles can be changed]
```

### Step 3: Collect Values
For each placeholder, gather from:
1. User input (ask if needed)
2. Repository context (README, existing docs)
3. Inference from codebase patterns

### Step 4: Draft Content
Ensure principles are:
- **Declarative**: State what MUST or SHOULD happen
- **Testable**: Can verify compliance objectively
- **Justified**: Include rationale for each principle

Common principles to consider:
- API-first architecture
- Test-first development
- GitOps deployment
- Library-first design
- CLI mandate
- Simplicity over abstraction

### Step 5: Propagate Consistency
After updating constitution, check alignment with:
- `CLAUDE.md` - Should reference constitution
- `specs/*/plan.md` - Plans should have constitution compliance section
- Templates - Should enforce constitution checks

### Step 6: Version Appropriately
- **MAJOR** (X.0.0): Incompatible changes, principle removals
- **MINOR** (0.X.0): New principles, expanded guidance
- **PATCH** (0.0.X): Clarifications, wording refinements

### Step 7: Write Constitution
Create/update `memory/constitution.md` with:
- Sync impact report as HTML comment at top
- All placeholders replaced with concrete values
- ISO-formatted dates (YYYY-MM-DD)
- MUST/SHOULD language with explicit rationale

### Step 8: Report
Output:
- File path
- Version change (if update)
- Sections modified
- Suggested commit message

## Key Constraints
- Never create a new file if one exists (always update)
- Principles must be testable (can objectively verify compliance)
- Use MUST for requirements, SHOULD for recommendations
- Include rationale for every principle

## Existing Project Principles
Check `CLAUDE.md` for current principles:
- API-First Interfaces
- GitOps-First Deployment
- CLI-First Operations
- Reuse Existing Components

## User Input
$ARGUMENTS
