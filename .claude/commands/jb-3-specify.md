# Specify (Phase 3)

Create a structured spec.md from the idea.md. Wraps /speckit.specify with workflow integration.

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

Invoke the speckit.specify workflow, but ensure it:
1. Reads from `specs/$SPEC_FOLDER/idea.md`
2. Writes to `specs/$SPEC_FOLDER/spec.md`
3. Includes a **## Validation** section with environment-specific checks

The spec.md should include:
- Metadata
- Clarifications from idea.md
- User Scenarios
- Functional Requirements
- **Validation Steps** (for post-deployment testing)
- Success Criteria

### Step 5: Add Validation Section

Ensure spec.md includes:

```markdown
## Validation

### Development Cluster
- [ ] $VALIDATION_CHECK_1
- [ ] $VALIDATION_CHECK_2

### Staging
- [ ] $STAGING_CHECK_1

### Production
- [ ] $PROD_CHECK_1
```

### Step 6: Update Bead

```bash
bd comments add $EPIC_BEAD_ID "spec.md created with /jb-3-specify"
```

### Step 7: Show Status Summary

```
═══════════════════════════════════════════════════
 /jb-3-specify - COMPLETE
═══════════════════════════════════════════════════

 Spec Folder:      specs/$SPEC_FOLDER/
 File Created:     spec.md

 Sections:
   - Metadata
   - User Scenarios: $COUNT
   - Requirements: $COUNT
   - Validation Steps: $COUNT

 Epic Bead:        $EPIC_BEAD_ID (updated)

 Next Steps:
   - Review spec.md and refine if needed
   - Run /jb-3-plan to create implementation plan
═══════════════════════════════════════════════════
```
