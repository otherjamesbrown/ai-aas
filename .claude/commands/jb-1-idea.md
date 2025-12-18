# New Idea

Start a new feature idea by creating a spec folder and idea.md file.

## Idea Name: $ARGUMENTS

## Instructions

### Step 1: Auto-Detect Next Spec Number

Find the highest existing spec number and increment:

```bash
ls -d specs/*/ 2>/dev/null | grep -oE '[0-9]+' | sort -n | tail -1
```

If no specs exist, start with 001.
Add 1 to get the next spec number (e.g., 030 → 031).
Pad to 3 digits.

### Step 2: Create Spec Folder

Create the folder with format `NNN-name`:

```bash
mkdir -p specs/$NEXT_NUMBER-$IDEA_NAME/
```

Example: `specs/031-ui-replacement/`

### Step 3: Create idea.md

Create the idea file with template:

```markdown
# Idea: $IDEA_NAME

**Spec Number:** $NEXT_NUMBER
**Created:** $DATE
**Status:** Draft

## Problem

<!-- What issue triggered this? What pain point are we solving? -->

## Discussion

<!-- Captured conversation points - Claude will update this as we discuss -->

## Proposed Approach

<!-- High-level ideas for how to solve this -->

## Open Questions

<!-- Things that need clarification before we proceed -->
-

## Out of Scope

<!-- What we're explicitly NOT doing -->

## Notes

<!-- Any other relevant information -->
```

### Step 4: Initial Context

If there was discussion before this command was invoked, summarize it into the Discussion section.

### Step 5: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-1-idea $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Spec Number:      $NEXT_NUMBER (auto-detected)
 Folder Created:   specs/$NEXT_NUMBER-$IDEA_NAME/
 File Created:     specs/$NEXT_NUMBER-$IDEA_NAME/idea.md

 Next Steps:
   - Continue discussion, I'll update idea.md
   - When ready: /jb-2-workspace $NEXT_NUMBER-$IDEA_NAME
═══════════════════════════════════════════════════
```

### Step 6: Continue Discussion

After creating the file, continue discussing the idea with the user.
Update idea.md as the discussion progresses:
- Add points to Discussion section
- Add clarifications to Open Questions
- Refine the Problem statement
- Capture decisions in Notes

Remind user to commit idea.md to develop before creating workspace.
