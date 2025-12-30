# Use Cases (Phase 3.1b)

Create use cases with acceptance criteria from spec.md. This phase extracts testable scenarios that will drive implementation and testing.

## Purpose

- Convert spec requirements into verifiable acceptance criteria
- Define clear scope boundaries (in_scope, out_of_scope, must_not)
- Create parent beads for each use case
- Enable test-driven implementation

## Instructions

### Step 1: Find Current Spec

Detect the current spec from the working directory or branch name:

```bash
SPEC_FOLDER=$(basename $(pwd))  # e.g., 031-ui-replacement
SPEC_NUMBER=$(echo $SPEC_FOLDER | grep -oE '^[0-9]+')
```

### Step 2: Load spec.md

```bash
cat specs/*/spec.md 2>/dev/null || cat spec.md 2>/dev/null
```

If no spec.md exists, abort and direct to `/jb-3.1-specify` first.

### Step 3: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 4: Determine Feature Prefix

Based on the spec content, determine the appropriate feature prefix:

| Feature Area | Prefix |
|--------------|--------|
| Authentication/authorization | `UC-AUTH-` |
| User management | `UC-USR-` |
| API key management | `UC-KEY-` |
| Model access | `UC-MDL-` |
| Benchmarks | `UC-BM-` |
| Audit logs | `UC-AUD-` |
| Usage/billing | `UC-USG-` |
| Organization management | `UC-ORG-` |

If the feature doesn't fit existing prefixes, propose a new 2-4 letter prefix.

### Step 5: Extract Use Cases from Spec

For each user scenario or functional requirement in the spec:

1. **Identify the actor** (who performs this action)
2. **Define the goal** (what they want to achieve)
3. **Write acceptance criteria** using Given/When/Then format
4. **Define scope boundaries**:
   - `in_scope`: What this UC covers
   - `out_of_scope`: What it explicitly excludes
   - `must_not`: Anti-requirements (things we must NOT do)

### Step 6: Create/Update Use Case YAML

Create or update the feature YAML file:

```bash
# Check if file exists
ls usecases/*.yaml 2>/dev/null
```

Write use cases following the schema in `usecases/SCHEMA.md`.

**CRITICAL**: Each use case MUST include:
- Unique ID following prefix pattern (UC-XXX-NNN)
- Clear description (1-3 paragraphs)
- At least 2 acceptance criteria
- All three scope sections (in_scope, out_of_scope, must_not)

### Step 7: Create Parent Beads

For each use case, create a parent bead:

```bash
# Create bead for each UC
bd create --title="UC-XXX-001: [Title]" --type=feature --priority=2

# Add dependency to epic
bd dep add $UC_BEAD_ID $EPIC_BEAD_ID

# Add labels
bd label add $UC_BEAD_ID uc:UC-XXX-001
bd label add $UC_BEAD_ID spec$SPEC_NUMBER
```

### Step 8: Update UC YAML with Bead IDs

Add the bead ID to each use case in the YAML:

```yaml
- id: UC-XXX-001
  title: "..."
  bead: aas-xxxxx  # <-- Add this
```

### Step 9: Update Spec with UC Reference

Add a reference to the use cases file in spec.md:

```markdown
## Acceptance Criteria

See [usecases/<feature>.yaml](../../usecases/<feature>.yaml) for acceptance criteria:
- UC-XXX-001: [Title]
- UC-XXX-002: [Title]
- UC-XXX-003: [Title]
```

### Step 10: Validate Use Cases

Run the linter to check for issues:

```bash
./scripts/lint-usecases.sh
```

### Step 11: Sync Beads

```bash
bd sync
```

### Step 12: Status Summary

```
═══════════════════════════════════════════════════
 /jb-3.1b-usecases - COMPLETE
═══════════════════════════════════════════════════

 Spec:             specs/$SPEC_FOLDER/spec.md
 Use Cases File:   usecases/<feature>.yaml

 Use Cases Created:
   - UC-XXX-001: [Title] (bead: aas-xxxxx)
   - UC-XXX-002: [Title] (bead: aas-yyyyy)
   - UC-XXX-003: [Title] (bead: aas-zzzzz)

 Acceptance Criteria: [total count]

 Beads Created: [count] (linked to epic: $EPIC_BEAD_ID)

 Validation:
   - Linter: [PASS/FAIL]
   - All UCs have in_scope: [YES/NO]
   - All UCs have out_of_scope: [YES/NO]
   - All UCs have must_not: [YES/NO]

 Next Steps:
   - Review use cases for completeness
   - If migration/refactor: /jb-3.2-impact
   - If greenfield: /jb-3.3-plan
═══════════════════════════════════════════════════
```

## Example Output

Given a spec for "Benchmark Target Management", this phase would create:

```yaml
# usecases/benchmarks.yaml
feature: Benchmarks
description: |
  Benchmark testing allows organization admins to validate model performance
  before production deployment.

dependencies:
  - usecases/authentication.yaml
  - usecases/apikeys.yaml

usecases:
  - id: UC-BM-001
    title: Create Benchmark Target
    interface: cli
    status: active
    bead: aas-ucbm001
    description: |
      An organization admin wants to create a benchmark target configuration...
    # ... full UC definition
```

## Quality Checklist

Before completing this phase, verify:

- [ ] Each spec requirement maps to at least one UC
- [ ] Each UC has at least 2 acceptance criteria
- [ ] All UCs have explicit in_scope, out_of_scope, must_not
- [ ] Parent beads created and linked to epic
- [ ] Bead IDs added to UC YAML
- [ ] Spec.md updated to reference use cases file
- [ ] Linter passes with no errors
