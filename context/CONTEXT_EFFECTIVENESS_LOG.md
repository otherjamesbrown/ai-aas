# Context Effectiveness Log

> Track context gaps, fixes, and outcomes to measure improvement over time.

---

## Log Format

Each entry records a context gap that was identified and fixed:

```yaml
- date: YYYY-MM-DD
  bug_bead: ai-aas-xxx
  gap_type: missing_pattern | missing_antipattern | stale_content | missing_rule
  context_file: context/<agent>/agents.md
  what_was_missing: "Brief description"
  fix_applied: "What was added/changed"
  fix_bead: ai-aas-yyy (if separate task created)
  prevented_bugs: []  # Updated later when similar bugs DON'T happen
```

---

## 2025-12

<!-- Add entries below this line, newest first -->

### Template Entry

```yaml
- date: 2025-12-15
  bug_bead: ai-aas-9s4z
  gap_type: missing_antipattern
  context_file: context/operator-developer/agents.md
  what_was_missing: "No documentation about KServe admission webhook overriding container probe configuration"
  fix_applied: "Added 'KServe Admission Webhook Probe Override' section with WRONG examples and verification steps"
  fix_bead: ai-aas-tb3j
  prevented_bugs: []
```

- date: 2025-12-13
  bug_bead: ai-aas-xxx
  gap_type: missing_antipattern
  context_file: context/go-services-developer/agents.md
  what_was_missing: "No example showing N+1 query anti-pattern"
  fix_applied: "Added WRONG example for N+1 queries in Anti-patterns section"
  fix_bead: ai-aas-yyy
  prevented_bugs: []
```

---

## Metrics

### Gap Types (update monthly)

| Type | Count | % |
|------|-------|---|
| missing_pattern | 0 | 0% |
| missing_antipattern | 1 | 100% |
| stale_content | 0 | 0% |
| missing_rule | 0 | 0% |

### By Context File (update monthly)

| File | Gaps Found | Gaps Fixed |
|------|------------|------------|
| context/agents.md | 0 | 0 |
| context/cli-developer/agents.md | 0 | 0 |
| context/go-services-developer/agents.md | 0 | 0 |
| context/operator-developer/agents.md | 1 | 1 |
| context/infra-ops-manager/agents.md | 0 | 0 |
| context/web-portal-developer/agents.md | 0 | 0 |

### Effectiveness (update quarterly)

| Quarter | Bugs with context-gap | Bugs prevented by prior fixes | Effectiveness |
|---------|----------------------|-------------------------------|---------------|
| 2025-Q4 | 1 | 0 | N/A |

---

## How to Use This Log

### When Closing a context-gap Bug

1. Add entry to this log with gap details
2. Update the Metrics tables
3. Commit with the context fix

### When a Bug is Prevented

If you notice a bug was avoided because an anti-pattern or rule existed:

1. Find the original log entry that added that context
2. Add the prevented bug bead to `prevented_bugs` array
3. Update Effectiveness metrics

### Monthly Review

Run `bd list --label "context-gap" --status closed` and ensure all are logged here.

### Quarterly Review

1. Count total bugs with `context-gap` label
2. Count entries where `prevented_bugs` is non-empty
3. Calculate effectiveness: `prevented / (prevented + new_gaps)`
