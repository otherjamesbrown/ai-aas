# Bead Templates

Use these templates when creating beads. Copy the relevant template and fill in the details.

## Bug Template

```markdown
## Problem
<What is broken? How does it manifest? Include error messages.>

## Evidence
<Logs, screenshots, commands that demonstrate the issue>

## Impact
<What functionality is affected? Who is impacted?>

## Environment
<Cluster, namespace, pod names, versions>

## Root Cause
<Why did this happen? Not just what is broken.>

## Proposed Fix
<What changes are needed?>

## Files to Modify
<List specific files>

## Prevention
<What should prevent this in future? Tests, docs, automation?>
```

## Epic Template

```markdown
## Objective
<What is this epic meant to achieve? Business value?>

## Current State
<What exists today?>

## Success Criteria
<How do we know this is complete?>

## Work Breakdown
| Bead ID | Task | Agent | Status |
|---------|------|-------|--------|
| <id> | <description> | <agent> | pending |

## Dependencies
<What must be done first?>

## Acceptance Criteria
- [ ] Criteria 1
- [ ] Criteria 2
```

## Task Template

```markdown
## Overview
<What needs to be done and why?>

## Current State
<What exists today?>

## Implementation Details
<Step-by-step approach>

## Files to Modify
<List specific files>

## Acceptance Criteria
- [ ] Criteria 1

## Testing
<How to verify this is complete?>
```

## Feature Template

```markdown
## User Story
As a <user type>, I want <functionality> so that <benefit>.

## Background
<Context and motivation>

## Requirements
- Requirement 1
- Requirement 2

## Technical Design
<High-level approach, components involved>

## API Changes (if applicable)
<New endpoints, modified schemas>

## Acceptance Criteria
- [ ] Criteria 1

## Out of Scope
<What is explicitly NOT included?>
```

## Close Reasons

When closing beads, use these prefixes:

| Prefix | When to Use | Example |
|--------|-------------|---------|
| `IMPLEMENTED` | Work complete | `IMPLEMENTED: commit abc1234, added retry logic` |
| `SUPERSEDED` | Replaced by new approach | `SUPERSEDED: replaced by AIModel CR in aas-yyy` |
| `OBSOLETE` | No longer needed | `OBSOLETE: feature removed in v2.0` |
| `DUPLICATE` | Same as another bead | `DUPLICATE: same as aas-yyy` |
| `WONT_FIX` | Decided not to fix | `WONT_FIX: edge case, cost > benefit` |

## Labels

| Label | Values | Purpose |
|-------|--------|---------|
| `agent:` | cli-developer, go-services-developer, operator-developer, infra-ops-manager, web-portal-developer | Ownership |
| `component:` | api-router, admin-api, user-org, analytics, operator, cli, web-portal, inference, observability | Which component |
| `env:` | development, staging, production | Which environment |
| `blocked` | (no value) | Cannot proceed |
