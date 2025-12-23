# Specify (Phase 3.1)

Create a structured spec.md from the idea.md. Wraps `/speckit.specify` with workflow integration.

## Instructions

### Step 1: Find Current Spec

Detect the current spec from the working directory or branch name:

```bash
basename $(pwd)  # e.g., 031-ui-replacement
```

### Step 2: Load idea.md

```bash
cat specs/*/idea.md 2>/dev/null || cat idea.md 2>/dev/null
```

### Step 3: Find Epic Bead

```bash
bd list --label="spec$SPEC_NUMBER" --label="epic"
```

### Step 4: Run Speckit Specify

Invoke `/speckit.specify` with the idea content.

The speckit command will:
- Parse the idea and extract requirements
- Ask up to 3 clarifying questions
- Create spec.md with all required sections including Validation
- Create checklists/requirements.md

### Step 5: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "spec.md created with /jb-3.1-specify"
```

### Step 6: Sync Beads

```bash
bd sync
```

### Step 7: Status Summary

```
═══════════════════════════════════════════════════
 /jb-3.1-specify - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/

 Files Created:
   - spec.md
   - checklists/requirements.md

 Sections:
   - User Scenarios: [count]
   - Functional Requirements: [count]
   - Validation Steps: [count]

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review spec.md and refine if needed
   - If migration/refactor: /jb-3.2-impact
   - If greenfield: /jb-3.3-plan
═══════════════════════════════════════════════════
```
