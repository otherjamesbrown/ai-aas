# Review Compliance

Review implementation for drift from use cases and architecture. Produces actionable recommendations that worker agents can address.

**Agent Type**: compliance-reviewer (read-only auditor)
**Specification**: [context/compliance-reviewer-agent.md](../../context/compliance-reviewer-agent.md)

## Purpose

This reviewer detects:
1. **Use Case Drift** - Code that doesn't match acceptance criteria or violates scope boundaries
2. **Architecture Drift** - Violations of architectural principles from ARCHITECTURE.md and constitution
3. **Missing Coverage** - Acceptance criteria without corresponding tests

## When to Run

- After implementing a use case (before PR)
- During PR review
- On-demand when refactoring
- As a periodic health check
- **Automatically**: Via Stop hook after implementation sessions

## Arguments: $ARGUMENTS

- No arguments: Review recent changes (HEAD~5)
- `UC-XXX-NNN`: Review specific use case
- `--full`: Full codebase review
- `--create-beads`: Auto-create beads for issues found

## Instructions

### Step 1: Identify Scope

Determine what to review:

```bash
# Option A: Review specific UC
# If user provided UC-XXX-NNN, review that UC

# Option B: Review recent changes
git diff --name-only HEAD~5 | grep -E '\.(go|ts|tsx)$'

# Option C: Review entire feature
# Based on current branch/worktree
```

### Step 2: Load Reference Documents

```bash
# Load use cases
cat usecases/*.yaml

# Load architecture
cat ARCHITECTURE.md

# Load constitution (if exists)
cat memory/constitution.md 2>/dev/null || cat context/constitution.md 2>/dev/null || true

# Load agents context
cat context/agents.md
```

### Step 3: Analyze Code Against Use Cases

For each changed file:

1. **Map to Use Cases**: Which UCs does this file relate to?
2. **Check Scope Boundaries**:
   - Is any code doing something in `out_of_scope`?
   - Is any code violating `must_not`?
3. **Verify Acceptance Criteria**:
   - Does the code satisfy the AC `then` conditions?
   - Are there behaviors not covered by any AC?

### Step 4: Analyze Code Against Architecture

Check for violations of:

1. **API-First Principle**: Is business logic in CLI/UI instead of API?
2. **GitOps Principle**: Are there direct kubectl commands for persistent changes?
3. **Thin Client Principle**: Is the CLI doing more than calling APIs?
4. **Constitution Rules**: Any explicit NEVER/ALWAYS violations?

### Step 5: Check Test Coverage

```bash
./scripts/uc-coverage.sh
```

Identify:
- ACs without tests
- Tests that don't map to any AC (orphan tests)

### Step 6: Generate Report

Output a structured report:

```markdown
# Compliance Review Report

**Scope**: UC-BM-001, UC-BM-002
**Files Reviewed**: 12
**Date**: YYYY-MM-DD

## Summary

| Category | Issues | Severity |
|----------|--------|----------|
| Use Case Drift | 2 | Medium |
| Architecture Violations | 1 | High |
| Missing Test Coverage | 3 | Medium |
| Total | 6 | - |

## Use Case Drift

### UC-BM-001: Create Benchmark Target

#### Issue 1: Out of Scope Violation
**Severity**: Medium
**File**: `internal/cmd/benchmark.go:145`
**Description**: Code auto-starts benchmark after target creation
**Violated Constraint**: `out_of_scope: Starting benchmark execution (UC-BM-002)`
**Recommendation**: Remove auto-start logic. Target creation should only create and return the target.
**Action**:
```
Remove lines 145-160 in internal/cmd/benchmark.go
These lines call StartBenchmarkRun() which violates the out_of_scope constraint
```

#### Issue 2: Missing AC Coverage
**Severity**: Medium
**File**: N/A
**Description**: No test for AC-03 (reject invalid scenario)
**Recommendation**: Add test case for invalid scenario handling
**Action**:
```go
// Add to internal/cmd/benchmark_test.go
t.Run("UC-BM-001/AC-03: reject invalid scenario", func(t *testing.T) {
    // Test implementation
})
```

## Architecture Violations

### Issue 3: API-First Violation
**Severity**: High
**File**: `internal/cmd/benchmark.go:89`
**Description**: CLI directly queries database for scenario validation
**Violated Principle**: CLI must not contain business logic or direct DB access
**Recommendation**: Move scenario validation to Admin API, call from CLI
**Action**:
1. Add `/v1/scenarios/validate` endpoint to admin-api-service
2. Update CLI to call validation API instead of direct DB query
3. Remove database import from CLI command file

## Missing Test Coverage

| UC | AC | Status | Recommendation |
|----|-----|--------|----------------|
| UC-BM-001 | AC-03 | Missing | Add test for invalid scenario |
| UC-BM-002 | AC-02 | Missing | Add test for non-existent target |
| UC-BM-002 | AC-03 | Missing | Add test for unavailable model |

## Recommendations Summary

### High Priority (Fix Before PR)
1. [ARCH-001] Move scenario validation to API layer

### Medium Priority (Fix in Same Sprint)
2. [UC-001-SCOPE] Remove auto-start logic from target creation
3. [UC-001-TEST] Add test for AC-03
4. [UC-002-TEST] Add tests for AC-02, AC-03

### Low Priority (Backlog)
None

## Beads to Create

If issues should be tracked as beads:

```bash
# High priority architecture violation
bd create --title="Move scenario validation to API layer" --type=bug --priority=1
bd label add <id> arch-violation
bd label add <id> uc:UC-BM-001

# Missing tests
bd create --title="Add missing tests for UC-BM-001/AC-03, UC-BM-002/AC-02,AC-03" --type=task --priority=2
bd label add <id> test-coverage
bd label add <id> uc:UC-BM-001
bd label add <id> uc:UC-BM-002
```
```

### Step 7: Offer Actions

After presenting the report, offer:

1. **Create beads for issues?** - Track issues in beads
2. **Generate fix list?** - Detailed steps for worker agent
3. **Re-run after fixes?** - Verify compliance after changes

## Output Format

The review should produce:

1. **Summary table** - Quick overview of issue counts
2. **Detailed findings** - Each issue with file, line, description, recommendation
3. **Action items** - Specific code changes needed
4. **Bead suggestions** - Issues to track

## Quality Checklist

Before completing review:

- [ ] All changed files analyzed
- [ ] Cross-referenced with relevant UCs
- [ ] Checked against architecture principles
- [ ] Test coverage verified
- [ ] Each issue has actionable recommendation
- [ ] Severity assigned to all issues

## Agent Behavior Rules

As a compliance-reviewer agent, you MUST:

1. **READ ONLY** - Never modify files, only analyze and report
2. **Be specific** - Include file:line references for all issues
3. **Be actionable** - Every finding must have a clear recommendation
4. **Prioritize correctly**:
   - High: Security, data loss, API-first violations
   - Medium: Scope violations, missing tests
   - Low: Style, minor improvements
5. **Track patterns** - Note if same issue appears multiple times
6. **Suggest beads** - Recommend tracking for significant issues

## Handoff to Worker Agents

After review, if issues found, generate a handoff document:

```markdown
## Compliance Handoff for Worker Agent

**Review Date**: YYYY-MM-DD
**Scope**: [UCs reviewed]
**Issues Found**: [count by severity]

### Fix Order (by priority)

1. **[HIGH] Issue Title** (Bead: aas-xxx)
   - File: path/to/file.go:123
   - Problem: Description
   - Fix: Specific steps

2. **[MEDIUM] Issue Title** (Bead: aas-yyy)
   ...

### Verification

After fixes, worker agent should run:
```bash
/review-compliance [scope]
```
```

## Integration Points

- **Hook**: `.claude/hooks/compliance-review.sh` runs on Stop event
- **Arch Review**: Theme 10 in `/arch-review` uses this reviewer
- **PR Review**: Can be called from `/jb-4-pr` for compliance check
- **Beads**: Issues labeled with `uc:UC-XXX-NNN` and `compliance`
