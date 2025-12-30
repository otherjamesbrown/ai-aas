# Compliance Reviewer Sub-Agent Specification

## Overview

The **compliance-reviewer** is a specialized sub-agent that detects drift from use cases and architecture, producing actionable recommendations for worker agents to address.

## Purpose

Unlike worker agents that implement features, the compliance-reviewer is a **read-only auditor** that:

1. Analyzes code changes against use case definitions
2. Verifies architectural principles are followed
3. Checks test coverage for acceptance criteria
4. Produces structured recommendations (not fixes)
5. Creates beads for tracking issues

## When to Use

| Trigger | Description |
|---------|-------------|
| **Post-implementation** | Automatically via Stop hook after `jb-3.6-implement` |
| **PR review** | Before merging to catch violations |
| **On-demand** | Via `/review-compliance` skill |
| **Periodic audit** | As part of `/arch-review` Theme 10 |
| **After refactoring** | Verify no regressions |

## Agent Definition

```yaml
name: compliance-reviewer
type: specialized
description: |
  Detects drift from use cases and architecture. Produces structured
  recommendations that worker agents can act on. Read-only - does not
  modify code, only analyzes and reports.

capabilities:
  - Analyze code against use case acceptance criteria
  - Verify scope boundaries (in_scope, out_of_scope, must_not)
  - Check architectural compliance (API-first, GitOps, thin clients)
  - Verify test coverage for acceptance criteria
  - Detect bug patterns by use case
  - Generate structured recommendations
  - Create tracking beads for issues

tools_allowed:
  - Read          # Read files
  - Glob          # Find files by pattern
  - Grep          # Search file contents
  - Bash          # Run read-only commands (git, scripts)

tools_denied:
  - Edit          # Cannot modify files
  - Write         # Cannot create files
  - NotebookEdit  # Cannot modify notebooks

inputs:
  - Use case YAML files (usecases/*.yaml)
  - Architecture documents (ARCHITECTURE.md, constitution)
  - Agent rules (context/agents.md)
  - Source code (services/*, internal/*)
  - Test files (*_test.go, *.test.ts)
  - Git history (recent changes)

outputs:
  - Compliance report (structured markdown)
  - Recommendations list (actionable items)
  - Beads for tracking (optional, requires confirmation)
  - Coverage metrics
```

## Analysis Workflow

### Phase 1: Scope Detection

Determine what to analyze:

```
┌─────────────────────────────────────────┐
│ Input: What triggered the review?       │
├─────────────────────────────────────────┤
│ • Specific UC ID (UC-BM-001)            │
│ • Recent git changes (HEAD~5)           │
│ • Entire feature (branch scope)         │
│ • Full codebase (periodic audit)        │
└─────────────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────┐
│ Output: Files and UCs to analyze        │
├─────────────────────────────────────────┤
│ • Changed files list                    │
│ • Related use cases                     │
│ • Affected components                   │
└─────────────────────────────────────────┘
```

### Phase 2: Use Case Compliance

For each file-to-UC mapping:

```
┌─────────────────────────────────────────┐
│ Check 1: Scope Boundary Violations      │
├─────────────────────────────────────────┤
│ • Does code implement out_of_scope?     │
│ • Does code violate must_not?           │
│ • Is functionality in correct UC?       │
└─────────────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────┐
│ Check 2: Acceptance Criteria Coverage   │
├─────────────────────────────────────────┤
│ • Does each AC have a test?             │
│ • Do tests verify the 'then' outcomes?  │
│ • Are error paths tested?               │
└─────────────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────┐
│ Check 3: Unmapped Code Detection        │
├─────────────────────────────────────────┤
│ • Is there code not covered by any AC?  │
│ • Is this legitimate or drift?          │
│ • Should a new AC be added?             │
└─────────────────────────────────────────┘
```

### Phase 3: Architecture Compliance

Check against architectural principles:

```
┌─────────────────────────────────────────┐
│ Principle 1: API-First                  │
├─────────────────────────────────────────┤
│ • Business logic in API layer?          │
│ • CLI/UI are thin clients?              │
│ • No direct DB access from CLI?         │
└─────────────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────┐
│ Principle 2: GitOps                     │
├─────────────────────────────────────────┤
│ • Infrastructure in git?                │
│ • No imperative kubectl commands?       │
│ • Helm charts for deployments?          │
└─────────────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────┐
│ Principle 3: Constitution Rules         │
├─────────────────────────────────────────┤
│ • NEVER rules not violated?             │
│ • ALWAYS rules followed?                │
│ • Domain boundaries respected?          │
└─────────────────────────────────────────┘
```

### Phase 4: Report Generation

Produce structured output:

```
┌─────────────────────────────────────────┐
│ Report Structure                        │
├─────────────────────────────────────────┤
│ 1. Executive Summary                    │
│    - Total issues by severity           │
│    - Overall compliance score           │
│                                         │
│ 2. Use Case Violations                  │
│    - Scope boundary violations          │
│    - Missing AC coverage                │
│    - Unmapped code                      │
│                                         │
│ 3. Architecture Violations              │
│    - API-first violations               │
│    - GitOps violations                  │
│    - Constitution violations            │
│                                         │
│ 4. Recommendations                      │
│    - Prioritized action items           │
│    - Specific file:line references      │
│    - Suggested fixes                    │
│                                         │
│ 5. Bead Suggestions                     │
│    - Issues to track                    │
│    - Labels to apply                    │
└─────────────────────────────────────────┘
```

## Recommendation Format

Each recommendation follows this structure:

```yaml
- id: REC-001
  severity: high | medium | low
  category: uc-drift | arch-violation | test-coverage

  # What was found
  finding:
    description: "CLI directly queries database for scenario validation"
    file: "internal/cmd/benchmark.go"
    line: 89
    evidence: |
      db.Query("SELECT * FROM scenarios WHERE name = ?", name)

  # What rule was violated
  violation:
    type: architecture
    principle: API-First
    reference: "ARCHITECTURE.md#api-first"
    constraint: "CLI must not contain business logic or direct DB access"

  # How to fix it
  recommendation:
    action: "Move scenario validation to Admin API"
    steps:
      - "Add /v1/scenarios/validate endpoint to admin-api-service"
      - "Update CLI to call validation API"
      - "Remove database import from CLI"
    effort: medium

  # Tracking
  bead_suggestion:
    title: "Move scenario validation to API layer"
    type: bug
    priority: 1
    labels:
      - arch-violation
      - uc:UC-BM-001
```

## Integration with Worker Agents

The compliance-reviewer produces output that worker agents consume:

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│                  │     │                  │     │                  │
│  Implementation  │────▶│   Compliance     │────▶│   Worker Agent   │
│  (jb-3.6)        │     │   Reviewer       │     │   (fix issues)   │
│                  │     │                  │     │                  │
└──────────────────┘     └──────────────────┘     └──────────────────┘
        │                        │                        │
        │                        ▼                        │
        │               ┌──────────────────┐              │
        │               │                  │              │
        └──────────────▶│   Beads Tracker  │◀─────────────┘
                        │                  │
                        └──────────────────┘
```

### Handoff Document

When handing off to worker agents:

```markdown
## Compliance Issues for Worker Agent

**Generated by**: compliance-reviewer
**Scope**: UC-BM-001, UC-BM-002
**Date**: 2025-12-30

### High Priority (Fix Before PR)

#### Issue 1: Architecture Violation
**Bead**: aas-xyz
**File**: internal/cmd/benchmark.go:89
**Problem**: Direct database access in CLI
**Fix Required**:
1. Add validation endpoint to admin-api-service
2. Call endpoint from CLI instead of direct query
3. Remove `database/sql` import

### Medium Priority (Fix in Same Sprint)

#### Issue 2: Missing Test Coverage
**Bead**: aas-abc
**UC**: UC-BM-001/AC-03
**Problem**: No test for invalid scenario handling
**Fix Required**:
```go
t.Run("UC-BM-001/AC-03: reject invalid scenario", func(t *testing.T) {
    // Implement test
})
```

### Verification

After fixes, run:
```bash
/review-compliance UC-BM-001
```
```

## Configuration

### Enable/Disable

In `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/compliance-review.sh"
          }
        ]
      }
    ]
  },
  "compliance_reviewer": {
    "enabled": true,
    "auto_create_beads": false,
    "severity_threshold": "medium",
    "block_on_high_severity": false
  }
}
```

### Customization

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | true | Enable/disable the reviewer |
| `auto_create_beads` | false | Auto-create beads for issues |
| `severity_threshold` | medium | Minimum severity to report |
| `block_on_high_severity` | false | Block completion on high severity |

## Metrics & Tracking

The compliance-reviewer tracks:

```yaml
metrics:
  # Per-session metrics
  session:
    files_analyzed: 15
    uc_violations: 2
    arch_violations: 1
    missing_tests: 3

  # Per-UC metrics (aggregated over time)
  usecases:
    UC-BM-001:
      total_bugs: 5
      ac_gap_bugs: 2  # Bugs that revealed missing ACs
      current_coverage: 75%

  # Trends
  trends:
    compliance_score: [85, 88, 82, 90]  # Last 4 reviews
    regression_count: 1  # Score decreased from previous
```

## Related Documents

- [CLAUDE.md](../CLAUDE.md) - Agent instructions including UC workflow
- [usecases/SCHEMA.md](../usecases/SCHEMA.md) - Use case YAML schema
- [review-compliance.md](../.claude/commands/review-compliance.md) - Skill invocation
- [arch-review.md](../.claude/commands/arch-review.md) - Full architecture review
