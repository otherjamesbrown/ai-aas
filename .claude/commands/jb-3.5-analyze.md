# Analyze (Phase 3.5)

Validate consistency and quality across specification artifacts before implementation. Wraps /speckit.analyze with bead integration.

## Purpose

Quality gate to catch issues before coding begins:
- Constitution violations
- Coverage gaps between spec → plan → tasks
- Terminology inconsistencies
- Ambiguous requirements

## Instructions

### Step 1: Find Current Spec

Detect from working directory:
```bash
basename $(pwd)
```

### Step 2: Verify All Artifacts Exist

```bash
cat specs/$SPEC_FOLDER/spec.md
cat specs/$SPEC_FOLDER/plan.md
cat specs/$SPEC_FOLDER/tasks.md
```

If any are missing, suggest running the appropriate prior step first.

### Step 3: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 4: Run Speckit Analyze

Invoke `/speckit.analyze` workflow to check:

| Issue Type | What to Find |
|------------|--------------|
| Constitution Conflicts | Violations of project principles |
| Coverage Gaps | Requirements without tasks, tasks without specs |
| Terminology Drift | Inconsistent naming across documents |
| Ambiguity | Vague or unclear language |
| Duplication | Same requirement stated differently |

### Step 5: Generate Analysis Report

Create or display analysis with severity levels:

- **CRITICAL**: Must fix before implementation (constitution violations, security issues)
- **HIGH**: Conflicts between artifacts, significant ambiguity
- **MEDIUM**: Terminology inconsistencies, incomplete coverage
- **LOW**: Wording improvements, documentation polish

### Step 6: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "Analysis complete: $CRITICAL critical, $HIGH high, $MEDIUM medium issues"
```

### Step 7: Gate Decision

**If CRITICAL issues found:**
```
═══════════════════════════════════════════════════
 /jb-3.5-analyze - BLOCKED
═══════════════════════════════════════════════════

 CRITICAL issues must be resolved before implementation.

 Issues Found:
   - CRITICAL: $COUNT
   - HIGH:     $COUNT
   - MEDIUM:   $COUNT
   - LOW:      $COUNT

 Action Required:
   - Fix critical issues in spec.md/plan.md/tasks.md
   - Re-run /jb-3.5-analyze

 DO NOT proceed to /jb-3.6-implement until resolved.
═══════════════════════════════════════════════════
```

**If no CRITICAL issues:**
```
═══════════════════════════════════════════════════
 /jb-3.5-analyze - PASSED
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/

 Analysis Summary:
   - CRITICAL: 0
   - HIGH:     $COUNT
   - MEDIUM:   $COUNT
   - LOW:      $COUNT

 Coverage:
   - spec.md → plan.md: $PERCENT%
   - plan.md → tasks.md: $PERCENT%
   - spec.md → tasks.md: $PERCENT%

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review HIGH/MEDIUM issues (optional but recommended)
   - Run /jb-3.6-implement to start implementation
═══════════════════════════════════════════════════
```

## Key Constraints

- **Read-only**: Analysis only, no file modifications without approval
- **Constitution violations are always CRITICAL**
- **CRITICAL issues block implementation**
- Report must be actionable with specific file locations
