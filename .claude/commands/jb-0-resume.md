# Resume Spec Work

Resume work on an existing spec by loading context from beads and spec files.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract the spec identifier from the argument. Accept formats:
- `spec030` → spec number 030
- `030` → spec number 030
- `030-ui-replacement` → spec folder name

### Step 2: Find Spec Folder

```bash
ls -d specs/*$SPEC_NUMBER*/
```

### Step 3: Load Spec Files

Read all existing spec files to understand context:

```bash
# Read in order of importance
cat specs/$SPEC_FOLDER/idea.md 2>/dev/null
cat specs/$SPEC_FOLDER/spec.md 2>/dev/null
cat specs/$SPEC_FOLDER/plan.md 2>/dev/null
cat specs/$SPEC_FOLDER/tasks.md 2>/dev/null
```

### Step 4: Find Related Beads

Look for the epic bead and all sub-beads:

```bash
# Find epic bead
bd list --json | jq '.[] | select(.id | contains("spec$SPEC_NUMBER"))'

# Or search by title
bd search "spec $SPEC_NUMBER"
```

Show the bead hierarchy:
- `aas-spec$NNN` (epic)
- `aas-spec$NNNspec` (implementation)
- `aas-spec$NNNpr` (PR review)
- `aas-spec$NNNdev` (dev cluster)
- `aas-spec$NNNstg` (staging)
- `aas-spec$NNNprod` (production)
- `aas-spec$NNNretro` (retrospective)

### Step 5: Determine Current Phase

Based on which beads and files exist, determine current phase:

| Files/Beads Present | Current Phase |
|---------------------|---------------|
| Only idea.md | Phase 1: Idea capture |
| spec.md exists, no workspace | Phase 2: Ready for workspace |
| spec bead exists, tasks.md | Phase 3: Implementation |
| pr bead exists | Phase 4: PR review |
| dev bead exists | Phase 5: Dev validation |
| stg bead exists | Phase 6: Staging validation |
| prod bead exists | Phase 7: Production |
| retro bead exists | Phase 8: Retrospective |

### Step 6: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-0-resume $ARGUMENTS - LOADED
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/
 Current Phase:    $PHASE_NUMBER - $PHASE_NAME

 Files Found:
   $FILE_LIST

 Beads Found:
   $BEAD_LIST

 Open Issues:      $COUNT issues in progress

 Context Loaded:   $SUMMARY_OF_SPEC

 Next Steps:
   - $SUGGESTED_NEXT_ACTION
   - Next command: /jb-$NEXT_PHASE-$COMMAND
═══════════════════════════════════════════════════
```

### Step 7: Offer Next Actions

Based on current phase, suggest what to do next:
- If in Phase 1: "Ready for /jb-2-workspace"
- If in Phase 3: "Continue with /jb-3-4-implement or /jb-4-pr when done"
- If in Phase 5: "Fix issues or proceed to /jb-6-staging"
- etc.
