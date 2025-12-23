# Checklist Generation Command

Create requirement quality checklists - "unit tests for English" that validate whether specifications are complete, clear, consistent, and measurable.

## Important Distinction

Checklists assess **requirement writing quality**, not implementation correctness.

Wrong: "Verify the button clicks correctly"
Right: "Are interaction states consistently defined across all buttons?"

## Instructions

### Step 1: Load Context
Read the relevant specification files:
- `specs/[feature]/spec.md` - Requirements
- `specs/[feature]/plan.md` - Implementation plan (if exists)
- `specs/[feature]/tasks.md` - Task list (if exists)

### Step 2: Clarify Intent
Based on user input and spec signals, ask up to 3 clarifying questions:
- What aspect to focus on? (API, UX, data, security, etc.)
- What depth of coverage?
- Any specific concerns?

### Step 3: Generate Checklist
Create a checklist file in `specs/[feature]/checklists/`:
- Use descriptive names: `ux.md`, `api.md`, `security.md`, `data.md`
- Default to `requirements.md` for general coverage

### Step 4: Checklist Structure

```markdown
# [Aspect] Quality Checklist: [Feature Name]

**Generated**: YYYY-MM-DD
**Source**: spec.md, plan.md

## Completeness
- [ ] Are all user scenarios defined with Given/When/Then? [Spec §User Scenarios]
- [ ] Are success criteria measurable and specific? [Spec §Success Criteria]
- [ ] Are edge cases documented? [Spec §Edge Cases]

## Clarity
- [ ] Is each requirement unambiguous (single interpretation)? [Spec §Requirements]
- [ ] Are technical terms defined or referenced? [Terminology]
- [ ] Are acceptance criteria testable? [Spec §Acceptance]

## Consistency
- [ ] Is terminology consistent across all sections? [Cross-reference]
- [ ] Do user stories align with functional requirements? [Spec §US vs §FR]
- [ ] Does the plan match spec priorities? [Plan §Phases vs Spec §Priorities]

## Coverage
- [ ] Is every functional requirement traced to a user story? [Spec §FR → §US]
- [ ] Does every user story have acceptance criteria? [Spec §US]
- [ ] Are non-functional requirements specified? [Gap] or [Spec §NFR]

## Edge Cases
- [ ] Are error scenarios defined for each user story? [Spec §US Exception]
- [ ] Are boundary conditions documented? [Spec §Edge Cases]
- [ ] Are recovery scenarios specified? [Spec §US Recovery]

## Measurability
- [ ] Are performance targets quantified? [Spec §Success Criteria]
- [ ] Are SLAs/SLOs defined where applicable? [Gap] or [Spec §NFR]
```

### Step 5: Quality Markers
Each checklist item should include:
- Quality dimension in brackets: `[Completeness]`, `[Clarity]`, `[Consistency]`, `[Coverage]`, `[Edge Cases]`, `[Measurability]`
- Reference: `[Spec §X.Y]` or gap markers: `[Gap]`, `[Ambiguity]`, `[Conflict]`, `[Assumption]`

### Step 6: Report
Output:
- File path created
- Item count
- Coverage summary

## Checklist Item Pattern
```
Are [requirement aspect] defined/specified for [scenario]? [Quality Dimension, Reference]
```

## Key Constraints

- Use ISO date format: YYYY-MM-DD

## User Input
$ARGUMENTS
