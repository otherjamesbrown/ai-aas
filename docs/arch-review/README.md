# Architecture Review Framework

Repeatable architecture review process for the AI-AAS platform. Run monthly to establish baselines and track drift.

## Quick Start

```bash
# Run a new architecture review
/arch-review

# Resume an existing review
/arch-review resume
```

## Review Themes

| # | Theme | Description |
|---|-------|-------------|
| 1 | Code Structure & Reuse | Codebase organization, shared code, function decomposition |
| 2 | Configuration Management | Centralized config strategy, environment handling |
| 3 | Data Storage | etcd vs postgres consistency, data access patterns |
| 4 | Logging & Observability | Log formats, levels, tracing, metrics |
| 5 | Error Handling | Error types, propagation, client responses |
| 6 | Security Practices | Input validation, auth patterns, secrets handling |
| 7 | API Design | REST conventions, versioning, error responses |
| 8 | Kubernetes Patterns | Helm charts, health probes, resource management |
| 9 | Testing Strategy | Test coverage, patterns, e2e approach |

## Scoring Rubric

Each component is scored 1-5 per theme:

| Score | Meaning | Action Required |
|-------|---------|-----------------|
| 5 | Excellent | Best practices followed, no changes needed |
| 4 | Good | Minor improvements possible |
| 3 | Adequate | Works but inconsistent with other components |
| 2 | Needs Work | Significant gaps, should be addressed |
| 1 | Critical | Major issues, prioritize remediation |

## Output Structure

```
docs/arch-review/
├── README.md                    # This file
├── methodology.md               # Detailed review process
├── templates/
│   ├── theme-template.md        # Template for each theme
│   └── summary-template.md      # Template for overall summary
└── reviews/
    └── YYYY-MM-DD/
        ├── summary.md           # Executive summary + scores
        ├── 01-code-structure.md
        ├── 02-configuration.md
        ├── ...
        └── remediation.md       # Prioritized action items
```

## Bead Integration

Each review creates:
- **Epic bead**: Parent for the entire review (labeled: `arch-review`, `YYYY-MM`)
- **Theme beads**: One per theme reviewed (labeled: `arch-review`, `theme-name`)
- **Remediation beads**: One per high-priority fix (labeled: `arch-review`, `remediation`)

Find all arch-review beads:
```bash
bd list --label=arch-review
```

## Comparing Reviews

Each summary.md includes a comparison table showing score changes from the previous review:
- ↑ Improved
- ↓ Regressed
- = No change
- NEW: First measurement

## Related Documents

- [methodology.md](./methodology.md) - Detailed review process
- [templates/](./templates/) - Review templates
