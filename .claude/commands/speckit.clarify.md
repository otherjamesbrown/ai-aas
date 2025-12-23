# Specification Clarification Command

Identify and resolve ambiguities in a feature specification before proceeding to technical planning.

## Instructions

### Step 1: Load Specification
Find and read the target specification:
```bash
ls -d specs/*/spec.md
```

If not specified, ask which feature to clarify.

### Step 2: Ambiguity Scanning
Systematically scan the spec across these categories:

| Category | What to Check |
|----------|---------------|
| Functional Scope | Are all features clearly bounded? |
| Data Modeling | Are entities and relationships defined? |
| UX Flows | Are user journeys complete? |
| Non-Functional | Performance, security, scalability requirements? |
| Integrations | External system dependencies clear? |
| Edge Cases | Error handling, boundary conditions? |
| Constraints | Technical or business limitations? |
| Terminology | Are terms used consistently? |
| Completion Signals | How do we know when it's done? |

Rate each: **Clear**, **Partial**, or **Missing**

### Step 3: Interactive Clarification
Ask up to 5 questions maximum, one at a time:

- For multiple-choice: Include a recommended option with reasoning
- For short answers: Suggest a default response
- User can respond with answer or say "done"/"stop" to end early

**Question Priority** (ask highest impact first):
1. Scope-defining questions
2. Security/compliance questions
3. UX-critical questions
4. Technical detail questions

### Step 4: Update Specification
After each answer, update `spec.md`:

1. Add to `## Clarifications` section with dated header:
```markdown
### Session YYYY-MM-DD

- **Q:** [Question asked]
  **A:** [User's answer]
```

2. Apply the answer to the relevant spec section (requirements, data model, etc.)

### Step 5: Validation Report
Generate a coverage summary:

```markdown
## Clarification Summary

| Category | Status |
|----------|--------|
| Functional Scope | Clear |
| Data Modeling | Clear |
| UX Flows | Partial |
| ... | ... |

**Outstanding Items**: [list any remaining gaps]
**Recommended Next Step**: `/speckit.plan`
```

## Key Constraints
- Maximum 5 questions per session
- Questions must materially impact architecture, testing, UX, or compliance
- No speculative tech-stack questions unless they block functional clarity
- User can end early with "done" or "stop"

## Key Constraints

- Use ISO date format: YYYY-MM-DD

## User Input
$ARGUMENTS
