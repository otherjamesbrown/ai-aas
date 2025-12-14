# Specification Analysis Command

Validate consistency and quality across specification artifacts. This is a **read-only** analysis tool.

## Instructions

### Step 1: Load Artifacts
Read all three core artifacts for the specified feature:
- `specs/[feature]/spec.md` - Requirements
- `specs/[feature]/plan.md` - Implementation plan
- `specs/[feature]/tasks.md` - Task list

Also load:
- `memory/constitution.md` - Project principles (if exists)
- `CLAUDE.md` - Project guidelines

### Step 2: Detection Analysis
Scan for issues across these dimensions:

| Issue Type | What to Find |
|------------|--------------|
| Duplication | Same requirement stated differently |
| Ambiguity | Vague or unclear language |
| Underspecification | Missing details needed for implementation |
| Constitution Conflicts | Violations of project principles |
| Coverage Gaps | Requirements without tasks, tasks without specs |
| Terminology Drift | Inconsistent naming across documents |

### Step 3: Severity Classification

**CRITICAL** (must fix before implementation):
- Constitution violations
- Blocking coverage gaps
- Security/compliance issues

**HIGH**:
- Conflicts between artifacts
- Significant ambiguity

**MEDIUM**:
- Terminology inconsistencies
- Incomplete non-functional coverage

**LOW**:
- Wording improvements
- Documentation polish

### Step 4: Generate Report
Output a structured analysis report:

```markdown
# Specification Analysis: [Feature Name]

**Analyzed**: [date]
**Artifacts**: spec.md, plan.md, tasks.md

## Findings

| # | Severity | Type | Location | Description |
|---|----------|------|----------|-------------|
| 1 | CRITICAL | Constitution Conflict | plan.md:45 | Direct DB access violates API-first |
| 2 | HIGH | Coverage Gap | spec.md US-003 | No tasks generated for this story |
| 3 | MEDIUM | Terminology | spec.md, plan.md | "user" vs "customer" inconsistent |

## Coverage Summary

| Artifact | Completeness |
|----------|--------------|
| spec.md → plan.md | 95% (1 gap) |
| plan.md → tasks.md | 100% |
| spec.md → tasks.md | 90% (2 gaps) |

## Metrics
- Total findings: X
- Critical: X
- High: X
- Medium: X
- Low: X

## Recommended Actions
1. [Action for critical issue]
2. [Action for high issue]
...
```

### Step 5: Remediation Plan (Optional)
If requested, generate a remediation plan, but **do not modify files** without explicit user approval.

## Key Constraints
- **Read-only**: No file modifications without explicit approval
- Constitution violations are always CRITICAL
- Report must be actionable with specific file locations

## User Input
$ARGUMENTS
