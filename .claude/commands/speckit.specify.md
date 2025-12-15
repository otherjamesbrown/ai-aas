# Feature Specification Command

Create a new feature specification following the spec-driven development methodology.

## Instructions

Follow this 7-step workflow to create a feature specification:

### Step 1: Generate Branch Naming
Extract a 2-4 word identifier from the feature description (e.g., "user-auth", "oauth2-api-integration").

### Step 2: Check Existing Specs
List existing spec directories to determine the next available feature number:
```bash
ls -d specs/*/
```

### Step 3: Load Template
Reference the spec template structure from `templates/spec-template.md` if it exists, otherwise use the standard structure.

### Step 4: Parse & Analyze
Extract from the user's description:
- Actors (who uses this)
- Actions (what they do)
- Data (what information is involved)
- Constraints (limitations, requirements)

Ask a maximum of 3 critical clarifying questions before proceeding.

### Step 5: Write Specification
Create the spec directory and files:
```
specs/[NNN]-[feature-name]/
├── spec.md           # Requirements document
├── checklists/
│   └── requirements.md
```

The `spec.md` must include:
- **Metadata**: Feature branch, created date, status, input description
- **Clarifications**: Q&A sessions documenting decisions
- **Assumptions**: What we're taking as given
- **User Scenarios**: Given/When/Then acceptance criteria with priorities (P1-P3)
- **Functional Requirements**: FR-001, FR-002, etc.
- **Success Criteria**: Measurable outcomes
- **Edge Cases**: Boundary conditions

### Step 6: Validate Quality
Create `checklists/requirements.md` validating:
- Content quality (no implementation details)
- Requirement completeness (testable/unambiguous)
- Feature readiness (bounded scope, clear acceptance criteria)

### Step 7: Report Completion
Provide:
- Branch name suggestion
- Spec path
- Readiness status for next phase (`/speckit.plan`)

## Key Constraints
- Maximum 3 `[NEEDS CLARIFICATION]` markers per spec
- Focus on WHAT and WHY, not HOW (no implementation details)
- Every requirement must be testable and measurable
- Success criteria must be technology-agnostic

## User Input
$ARGUMENTS
