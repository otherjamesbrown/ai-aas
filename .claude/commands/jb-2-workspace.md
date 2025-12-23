# Create Workspace

Create a new workspace for implementing a spec. Commits changes to develop, creates worktree and branch, and sets up beads.

## Spec: $ARGUMENTS

## Instructions

### Step 1: Parse Spec Reference

Extract spec info from argument:
- `031-ui-replacement` → folder name and number
- `spec031` → find folder by number
- `031` → find folder by number

```bash
ls -d specs/*$SPEC_REF*/
```

### Step 2: Verify idea.md Exists

```bash
cat specs/$SPEC_FOLDER/idea.md
```

If no idea.md, warn user and suggest `/jb-1-idea` first.

### Step 3: Check for Uncommitted Changes

```bash
git status --porcelain
```

If there are uncommitted changes:
1. Show what's changed
2. Ask user if they want to commit these changes to develop first
3. If yes, commit with message: "feat(spec): Add idea for $SPEC_NAME"

### Step 4: Commit and Push to Develop

```bash
git add specs/$SPEC_FOLDER/
git commit -m "feat(spec): Add idea for $SPEC_NAME"
git push origin develop
```

### Step 5: Create Workspace

Use the workspace function to create worktree and branch:

```bash
workspace new $SPEC_FOLDER develop
```

This will:
- Create branch `$SPEC_FOLDER` from develop
- Create worktree at `~/worktrees/$SPEC_FOLDER`
- Unlock git-crypt
- Start tmux session

**Note:** This command will switch to the new tmux session. The remaining steps happen in the new workspace.

### Step 6: Create Epic Bead

In the new workspace, create the top-level epic bead with explicit ID:

```bash
bd create --id="ai-aas-spec$SPEC_NUMBER" --title="Spec $SPEC_NUMBER: $SPEC_NAME" --type=epic --priority=1 --description="Epic for spec $SPEC_FOLDER. See specs/$SPEC_FOLDER/ for details."
```

Epic bead ID: `ai-aas-spec$SPEC_NUMBER`

### Step 7: Create Implementation Sub-Bead

Create the implementation tracking sub-bead with period-based naming:

```bash
bd create --id="ai-aas-spec$SPEC_NUMBER.impl" --title="$SPEC_NAME - Implementation" --type=task --priority=1 --parent=ai-aas-spec$SPEC_NUMBER
```

Implementation bead ID: `ai-aas-spec$SPEC_NUMBER.impl`

### Step 8: Update Beads with Labels

```bash
bd update ai-aas-spec$SPEC_NUMBER --label="spec$SPEC_NUMBER" --label="epic"
bd update ai-aas-spec$SPEC_NUMBER.impl --label="spec$SPEC_NUMBER" --label="implementation"
```

### Step 9: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-2-workspace $ARGUMENTS - COMPLETE
═══════════════════════════════════════════════════

 Committed:        specs/$SPEC_FOLDER/idea.md to develop
 Branch Created:   $SPEC_FOLDER (from develop)
 Worktree:         ~/worktrees/$SPEC_FOLDER
 Tmux Session:     $SPEC_FOLDER

 Beads Created:
   - ai-aas-spec$SPEC_NUMBER (epic): Spec $SPEC_NUMBER: $SPEC_NAME
   - ai-aas-spec$SPEC_NUMBER.impl (implementation): $SPEC_NAME - Implementation

 Next Steps:
   - Run /jb-3-1-specify to create spec.md from idea.md
   - Then /jb-3-2-plan to create implementation plan
   - Then /jb-3-3-tasks to create task breakdown
═══════════════════════════════════════════════════
```

## Important Notes

- This command switches to a new tmux session
- Make sure idea.md is committed before running
- The epic bead ID should be noted for future reference
- All subsequent work happens in the new workspace
