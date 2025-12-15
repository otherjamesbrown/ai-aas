# Agent Context Template

Use this template when creating or updating agent-specific context files.

**Target: <150 lines**

---

```markdown
# <Agent Name> Context

> **Inherits**: context/agents.md | **Verified**: YYYY-MM-DD | **Commit**: <hash>

---

## Domain

You own:
- `path/to/owned/code/`
- `path/to/other/owned/`

You do NOT own (hand off to):
- <area> → <agent>

---

## Key Patterns

```yaml
patterns:
  pattern_name:
    rule: One line rule
    do:
      - Action 1
      - Action 2
    why: Brief explanation

  another_pattern:
    rule: One line rule
    do:
      - Action 1
    avoid:
      - Bad practice
```

---

## Anti-patterns

```bash
# WRONG: Description
<bad code example>

# WRONG: Description
<bad code example>
```

---

## Commands

```bash
# Common commands for this domain
command1  # description
command2  # description
```

---

## Sources

| What | Where |
|------|-------|
| Code | `path/to/code/` |
| Config | `path/to/config/` |
| Docs | `path/to/docs/` |
```

---

## Guidelines

1. **<150 lines total**
2. **YAML for patterns** - structured, parseable
3. **No ASCII diagrams** - use YAML hierarchies
4. **No prose** - use bullets
5. **Link to sources** - don't duplicate code/config
6. **WRONG examples only** - agent knows the rules from context/agents.md
