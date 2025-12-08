# Claude Code Agent Creation Guide

This guide documents how to create consistent, effective agents for the AI-AAS platform.

## Overview

Agents are specialized sub-agents that Claude Code spawns to handle specific domains of work. Each agent has:
- A defined scope of responsibilities
- Clear boundaries on what it does NOT handle
- A handoff protocol for out-of-scope issues
- Documentation ownership
- A structured final report format

## File Structure

```
.claude/
├── AGENT_GUIDE.md          # This file
├── settings.json           # Claude Code settings
├── settings.local.json     # Local overrides (not committed)
└── agents/
    ├── <agent-name>.md     # Agent definition files
    └── ...
```

Each agent also has a corresponding navigation document in `docs/`:
- `docs/platform/agent-<name>.md` for infrastructure agents
- `docs/go-services/agent-<name>.md` for Go service agents
- etc.

## Agent File Format

Agent files use YAML frontmatter followed by markdown content.

### Frontmatter (Required)

```yaml
---
name: agent-name
description: Use this agent when... [include examples in the description]
model: sonnet  # or opus, haiku
color: blue    # visual identifier (blue, red, green, yellow, etc.)
---
```

### Description Best Practices

The `description` field is critical - it tells Claude Code when to spawn this agent. Include:

1. **Clear trigger conditions**: "Use this agent when..."
2. **Specific examples**: Show 3-4 examples with context
3. **Anti-examples**: Show when NOT to use this agent (defer to another)

Example format in the description:
```
Use this agent when [primary use case]. This includes [list of tasks].

Examples:

<example>
Context: [situation]
user: "[user message]"
assistant: "[how assistant would respond and invoke agent]"
<commentary>
[Why this agent is appropriate]
</commentary>
</example>

<example>
Context: [situation where this agent should NOT be used]
user: "[user message]"
assistant: "[how assistant would invoke a DIFFERENT agent]"
<commentary>
[Why a different agent is more appropriate]
</commentary>
</example>
```

## Required Sections

Every agent MUST include these sections in this order:

### 1. Introduction
Brief role description (1-2 sentences).

### 2. Documentation - Your Primary Reference
```markdown
## Documentation - Your Primary Reference

**CRITICAL**: The `docs/<domain>/` directory is your source of truth.

### Step 1: Start Here
**Read `docs/<domain>/agent-<name>.md` first** - This is your navigation index.

### Step 2: Find the Right Document
| Task | Document |
|------|----------|
| ... | `docs/<domain>/<file>.md` |

### Documentation Maintenance Responsibility (MANDATORY)
**You are responsible for keeping `docs/<domain>/` accurate and current.**
[Include update rules and beads issue creation for gaps]
```

### 3. Core Responsibilities
List what this agent handles (5-8 bullet points).

### 4. Root Cause Analysis (MANDATORY for all agents)

**CRITICAL**: This section is REQUIRED for all agents. Fixing symptoms without understanding root causes leads to recurring problems.

```markdown
## Root Cause Analysis (MANDATORY)

**CRITICAL**: When fixing ANY issue, you MUST perform root cause analysis. Fixing the symptom is NOT sufficient - you must understand WHY it happened and prevent recurrence.

### Root Cause Analysis Protocol

After fixing an issue, you MUST answer these questions:

**1. Why did this happen?**
- Was this a one-time mistake or a systemic pattern?
- What assumptions or gaps led to this issue?

**2. Could this happen elsewhere?**
- Are there similar patterns that might have the same problem?
- Search for similar code/config patterns

**3. What should have prevented this?**
- Missing tests, documentation, automation, or tooling?
- Add whatever would have caught this

**4. What needs to be fixed upstream?**
- If documentation is missing/incorrect, update it
- If automation is missing, create beads issue
- If tests are missing, add them

### Required Actions After Fixing Any Issue

1. **Search for similar issues** in other locations
2. **Add prevention measures** (tests, docs, automation)
3. **Create beads for related issues** you can't fix immediately
4. **Update documentation** if it contributed to the problem
```

Include domain-specific examples of good vs. bad root cause analysis.

### 5. What You Do NOT Handle
List what this agent should NOT do. Be specific.

```markdown
## What You Do NOT Handle

- **Category 1**: Description. Hand off to <other-agent>
- **Category 2**: Description. Hand off to <other-agent>
- ...

### Handoff Protocol

When you identify issues outside your scope:
1. **Document your findings**: Capture relevant analysis
2. **Create a beads issue** with your findings:
   ```bash
   bd create "<issue> - requires <other-agent>" --type <type> --priority <n>
   bd comments add <issue-id> "Analysis from <this-agent>: <findings>"
   ```
3. **Report the handoff** clearly to the user with the beads issue ID
```

### 5. Domain-Specific Sections
Add sections specific to the agent's domain:
- For infra: GitOps workflow, ArgoCD templates, debugging workflow
- For Go services: Code patterns, testing requirements, optimization strategies
- etc.

### 6. Issue Tracking
```markdown
## Issue Tracking

Use beads for tracking work:
```bash
bd list --status open              # Check existing issues
bd create "Title" --type <type>    # Create issue
bd update <issue-id> --status in_progress
```

Always offer to create beads issues for:
- Discovered bugs during work
- Improvements identified
- Tasks to be done later
```

### 7. Communication Style
Brief guidelines on how the agent should communicate.

### 8. Task Completion Checklist (MANDATORY)
```markdown
## Task Completion Checklist (MANDATORY)

**Before reporting a task as complete, you MUST run through this checklist:**

### 1. Root Cause Analysis (for any bug/issue fix)
- [ ] Determined WHY the issue occurred (not just what was broken)
- [ ] Searched for similar patterns elsewhere
- [ ] Identified if this is a systemic problem
- [ ] Added prevention measures (tests, docs, automation)
- [ ] Created beads issues for related issues found

### 2. [Domain-specific checks]
- [ ] Check 1
- [ ] Check 2

### 3. Documentation Validation
- [ ] Read relevant docs - are they still accurate?
- [ ] Fix any outdated information found
- [ ] Check if missing documentation contributed to the issue

### 4. Documentation Updates
- [ ] Update docs if changes affect them
- [ ] Update `last_updated` field in any modified document
- [ ] Document any patterns discovered to prevent recurrence

### 5. Issue Tracking
- [ ] Create beads issues for problems discovered but not fixed
- [ ] Create beads issues for documentation gaps
- [ ] Create beads issues for similar problems in other locations
- [ ] Tag issues with appropriate agent (go-services-developer, infra-ops-manager)

### 6. Final Report (REQUIRED FORMAT)
Your completion report MUST include these sections:

**Summary**
- What was accomplished
- Any remaining issues or known limitations

**Root Cause Analysis** (REQUIRED for bug fixes)
- **Why it happened**: Underlying cause, not just the symptom
- **Similar patterns found**: Results of searching elsewhere
- **What should have prevented it**: Missing tests, docs, automation
- **Prevention measures added**: What you did to prevent recurrence
- If new feature: state "N/A - new feature implementation"

**Git Commits**
- `<hash>`: <message>

**[Domain-specific section]**
- Files changed, tests added, etc.

**Documentation Updates**
- List each file and what was changed
- Or: "No documentation updates required"

**Beads Issues**
- Issues created: `<id>`: <title> (tagged: <agent>)
- Issues updated: `<id>`: <update>
- Issues closed: `<id>`: <reason>
- **Systemic issues identified**: List beads for root cause fixes
- Or: "No beads issues created, updated, or closed"

**Handoffs to Other Agents**
- If issues require another agent: list beads issue IDs with agent tag
- Or: "No handoffs to other agents"

**Follow-up Items**
- Items requiring user attention
- Suggested next steps
- **Open beads for root causes**: Issues left open that prevent recurrence
```

## Creating the Navigation Document

Each agent needs a corresponding doc in `docs/<domain>/agent-<name>.md`:

```markdown
---
title: "<Agent Name> Navigation Index"
last_updated: YYYY-MM-DD
---

# <Agent Name> Navigation Index

Quick reference and document map for the <agent-name> agent.

## Document Index

| Document | Purpose |
|----------|---------|
| `<file>.md` | Description |

## Quick Reference

[Tables with common values: ports, endpoints, paths, etc.]

## Common Workflows

### Workflow 1: <Name>
1. Step
2. Step

## Services/Resources Inventory

| Name | Location | Notes |
|------|----------|-------|
```

## Agent Interaction Model

Agents interact through beads issues:

```
┌───────────────────┐   ┌───────────────────────┐   ┌─────────────────┐
│ infra-ops-manager │   │ go-services-developer │   │  cli-developer  │
│                   │   │                       │   │                 │
│ K8s, GitOps, CI/CD│   │ Backend APIs, DBs     │   │ CLI commands,UX │
│        │          │   │          │            │   │        │        │
│        ▼          │   │          ▼            │   │        ▼        │
│ Finds code bug ───┼───┼──► Creates beads      │   │ Needs API ──────┼───┐
│                   │   │                       │   │                 │   │
│ ◄─────────────────┼───┼── Finds infra issue   │   │ ◄───────────────┼───┤
│                   │   │                       │   │                 │   │
│ Needs deployment◄─┼───┼───────────────────────┼───┼── Creates beads │   │
└───────────────────┘   └───────────────────────┘   └─────────────────┘   │
          │                        │                         │            │
          └────────────────────────┴─────────────────────────┴────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────────────┐
                    │          Beads Issues           │
                    │ (Persistent handoff with context)│
                    └─────────────────────────────────┘
```

### Handoff Examples

| From | To | When |
|------|-----|------|
| infra-ops-manager | go-services-developer | Pod crashes due to code bug |
| infra-ops-manager | cli-developer | CLI binary build/release needed |
| go-services-developer | infra-ops-manager | Service needs deployment, env vars |
| go-services-developer | cli-developer | CLI should expose new API endpoint |
| cli-developer | go-services-developer | API endpoint missing or returning wrong data |
| cli-developer | infra-ops-manager | CLI binary deployment/release |

## Checklist for New Agents

Before deploying a new agent:

- [ ] Agent file created in `.claude/agents/<name>.md`
- [ ] Frontmatter includes name, description (with examples), model, color
- [ ] "What You Do NOT Handle" section with handoff protocol
- [ ] Documentation ownership defined
- [ ] Navigation document created in `docs/<domain>/agent-<name>.md`
- [ ] Task Completion Checklist includes all required sections
- [ ] Final Report format includes Handoffs section
- [ ] Anti-examples show when to use OTHER agents
- [ ] Tested by spawning and running a sample task

## Existing Agents

| Agent | Domain | Color | Hands off to |
|-------|--------|-------|--------------|
| infra-ops-manager | Infrastructure, K8s, GitOps, CI/CD | red | go-services-developer, cli-developer |
| go-services-developer | Backend Go services, APIs, databases | blue | infra-ops-manager, cli-developer |
| cli-developer | CLI tool (ai-aas-cli), commands, UX | green | go-services-developer (API changes), infra-ops-manager (deployment) |

## Model Selection Guidelines

| Model | Use When |
|-------|----------|
| `haiku` | Quick, simple tasks; low latency needed |
| `sonnet` | Default for most agents; good balance |
| `opus` | Complex reasoning; critical decisions |

## Tips for Effective Agents

1. **Be specific about scope**: Vague boundaries lead to confusion
2. **Include anti-examples**: Show when NOT to use the agent
3. **Mandate documentation updates**: Agents should maintain their docs
4. **Use beads for handoffs**: Creates audit trail and context preservation
5. **Require structured reports**: Makes it easy to verify completion
6. **Test with edge cases**: Try tasks at the boundary of scope
