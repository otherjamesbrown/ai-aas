# PR Review

Comprehensive review and action workflow for a GitHub Pull Request with integrated issue tracking.

## PR URL: $ARGUMENTS

## Instructions

### Phase 1: PR Summary

1. **Parse the PR URL** - Extract owner, repo, and PR number

2. **Gather PR information**:
   ```bash
   gh pr view <PR_NUMBER> --repo <OWNER>/<REPO> --json title,body,additions,deletions,changedFiles,commits,state,statusCheckRollup,headRefName,baseRefName
   ```

3. **Get file changes**:
   ```bash
   gh pr diff <PR_NUMBER> --repo <OWNER>/<REPO> --stat
   ```

4. **Check CI status**:
   ```bash
   gh pr checks <PR_NUMBER> --repo <OWNER>/<REPO>
   ```

5. **Display Summary** including:
   - PR title and number
   - Branch info (head -> base)
   - Stats: lines added/removed, files changed, commits
   - CI/Check status (pass/fail for each check)
   - Key files changed grouped by area
   - Any failing checks with failure details

### Phase 1.5: Create PR Tracking Bead

After displaying the summary, create a bead to track this PR review work:

1. **Create the tracking bead** with explicit ID `ai-aas-pr<PR_NUMBER>`:
   ```bash
   bd create "PR#<PR_NUMBER>: <PR_TITLE>" --id ai-aas-pr<PR_NUMBER> --type task --priority 1 --force
   ```

   **Note**: The `--force` flag is required because the custom ID format triggers a prefix mismatch warning.

   Example: For PR #55, the bead ID will be `ai-aas-pr55`.

2. **Update the bead with PR details**:
   ```bash
   bd update <BEAD_ID> --label pr-review --label "pr-<PR_NUMBER>" --label "<BASE_BRANCH>-to-<HEAD_BRANCH>"
   ```

3. **Add a comment with the PR summary**:
   ```bash
   bd comments add <BEAD_ID> "PR Summary:
   - Branch: <HEAD_BRANCH> → <BASE_BRANCH>
   - Files changed: <COUNT>
   - Additions: +<ADDITIONS>, Deletions: -<DELETIONS>
   - CI Status: <PASS/FAIL>
   - URL: <PR_URL>"
   ```

4. **Inform the user**: Display the created bead ID so they can reference it:
   ```
   📋 Created tracking bead: ai-aas-pr<PR_NUMBER>
   Use `bd show ai-aas-pr<PR_NUMBER>` to view progress.
   ```

---

### Phase 2: Ask User What to Address

After displaying the summary and creating the tracking bead, ask the user:

**"What would you like me to address?"**

Options:
- **Review comments** - Process and respond to reviewer comments on the PR
- **Failing tests/CI** - Investigate and fix failing checks
- **Both** - Address comments first, then fix failing tests
- **Environment issues** - Track and debug issues after merging to a new environment
- **Neither** - Just wanted the summary

Wait for user response before proceeding.

---

### Phase 3A: Address Review Comments (if selected)

**CRITICAL: This phase is AUTOMATIC. Do NOT ask the user which comments to address - process and fix ALL review comments without further prompting.**

**IMPORTANT: Understand the difference between Reviews and Review Comments**

- **Reviews** (`/pulls/<PR>/reviews`) - Top-level review summaries (e.g., "This PR looks good overall...")
  - URL format: `#pullrequestreview-3542658823`
  - These are NOT actionable - they're just summaries
  - Do NOT respond to these

- **Review Comments** (`/pulls/<PR>/comments`) - Inline comments on specific lines of code
  - URL format: `#discussion_r2591027213`
  - These contain the ACTUAL actionable feedback
  - You MUST address these

1. **Checkout the PR branch**:
   ```bash
   gh pr checkout <PR_NUMBER> --repo <OWNER>/<REPO>
   ```

2. **Fetch all review comments** (inline comments on specific code lines):
   ```bash
   gh api repos/<OWNER>/<REPO>/pulls/<PR_NUMBER>/comments
   ```

   This returns comments with:
   - `id` - The numeric comment ID (use this for replies)
   - `body` - The actual feedback/issue raised
   - `path` - The file being commented on
   - `line` - The line number
   - `diff_hunk` - The code context

3. **For each review comment** (process ALL of them automatically - do not ask user to select):
   - **Read the `body` field** - This contains the actual issue/feedback
   - **Understand the code context** from `path`, `line`, and `diff_hunk`
   - **Decide** - Determine if it requires a code change
   - **Fix** - Make the code changes if needed
   - **Respond** - **REQUIRED**: Reply directly to that comment using its `id`:
     ```bash
     gh api repos/<OWNER>/<REPO>/pulls/<PR_NUMBER>/comments/<COMMENT_ID>/replies \
       -X POST \
       -f body="<RESPONSE>"
     ```

   **You MUST reply to each comment on GitHub after addressing it. Do not skip this step.**

4. **Example workflow**:
   ```bash
   # Fetch comments
   gh api repos/owner/repo/pulls/55/comments

   # Response includes:
   # {
   #   "id": 2591027213,
   #   "body": "The GetPullJob handler should also use the model name parameter...",
   #   "path": "services/admin-api-service/internal/handlers/models/handler.go",
   #   "line": 190
   # }

   # Read the body - this is the actual issue to address!
   # Fix the code at the specified path/line
   # Then reply:
   gh api repos/owner/repo/pulls/55/comments/2591027213/replies \
     -X POST \
     -f body="Fixed - updated GetPullJob to accept and validate the model name parameter"
   ```

5. **Response guidelines** (MUST reply to every comment):
   - If fixed: Describe what you changed
   - If not fixing: Explain why (e.g., "Won't fix - this is intentional because...")
   - Keep responses concise
   - Reply to the specific comment ID, NOT to the review summary
   - **Never skip the reply step** - reviewers need to know their feedback was addressed

6. **Commit changes** after addressing comments:
   ```bash
   git add -A && git commit -m "Address PR review comments"
   git push
   ```

7. **Update tracking bead**:
   ```bash
   bd comments add <BEAD_ID> "Addressed review comments:
   - <SUMMARY_OF_CHANGES>
   Commit: <COMMIT_SHA>"
   ```

8. **Return to menu** - After completing, go back to Phase 2 and ask the user what else they want to address.

**Phase 3A Summary Checklist:**
- [ ] Checked out PR branch
- [ ] Fetched ALL review comments via API
- [ ] For EACH comment: understood → fixed → replied on GitHub
- [ ] Committed all fixes
- [ ] Pushed to remote
- [ ] Updated tracking bead

---

### Phase 3B: Fix Failing Tests/CI (if selected)

1. **Identify the failing checks** from the CI status

2. **Get failure details**:
   ```bash
   gh run view <RUN_ID> --repo <OWNER>/<REPO> --log-failed
   ```

3. **Analyze failures**:
   - Test failures: Identify which tests failed and why
   - Coverage failures: Identify which packages need tests
   - Lint/build failures: Identify the errors

4. **Fix the issues**:
   - For test failures: Fix the failing tests or the code causing them
   - For coverage gaps: Write tests for uncovered code
   - For lint/build: Fix the reported errors

5. **Verify locally** (if possible):
   ```bash
   # Run relevant test commands based on the project
   ```

6. **Commit and push**:
   ```bash
   git add -A && git commit -m "Fix CI failures"
   git push
   ```

7. **Update tracking bead**:
   ```bash
   bd comments add <BEAD_ID> "Fixed CI failures:
   - <SUMMARY_OF_FIXES>
   Commit: <COMMIT_SHA>"
   ```

8. **Return to menu** - After completing, go back to Phase 2 and ask the user what else they want to address.

---

### Phase 4: Environment Issue Tracking (if selected)

Use this phase when the PR has been merged and deployed to a new environment (e.g., develop → staging → main) and issues arise.

#### 4.1: Gather Environment Context

1. **Ask the user about the environment**:
   - Which environment was the PR merged into? (development/staging/production)
   - What issues are being observed?
   - Any error messages, logs, or symptoms?

2. **Update the tracking bead**:
   ```bash
   bd comments add <BEAD_ID> "Merged to <ENVIRONMENT> - investigating issues"
   bd update <BEAD_ID> --label "env-<ENVIRONMENT>"
   ```

#### 4.2: Debug Using Debugger Subagent

1. **Launch the debugger subagent** to investigate the issue:
   - Use the Task tool with `subagent_type='debugger'`
   - Provide context: environment, symptoms, error messages, relevant logs
   - The debugger will produce a root cause analysis

2. **Example debugger invocation**:
   ```
   Task(subagent_type='debugger', prompt='''
   Investigate issue in <ENVIRONMENT> environment after PR #<PR_NUMBER> merge.

   Symptoms:
   - <SYMPTOM_1>
   - <SYMPTOM_2>

   Error messages:
   <ERROR_MESSAGES>

   Relevant services: <SERVICE_LIST>
   ''')
   ```

3. **Capture findings**: Document the root cause analysis in the tracking bead:
   ```bash
   bd comments add <BEAD_ID> "Root Cause Analysis:
   <SUMMARY_FROM_DEBUGGER>"
   ```

#### 4.3: Create Sub-Beads for Fixes

For each issue identified that requires a fix:

1. **Create a sub-bead for the fix**:
   ```bash
   bd create "Fix: <ISSUE_DESCRIPTION>" --type bug --priority 1
   ```

2. **Link to parent PR bead**:
   ```bash
   bd update <SUB_BEAD_ID> --label "parent-<PARENT_BEAD_ID>" --label "pr-<PR_NUMBER>" --label "env-<ENVIRONMENT>"
   bd dep add <SUB_BEAD_ID> <PARENT_BEAD_ID>
   ```

3. **Add context to the sub-bead**:
   ```bash
   bd comments add <SUB_BEAD_ID> "Issue discovered after PR #<PR_NUMBER> merged to <ENVIRONMENT>.

   Root cause: <ROOT_CAUSE>

   Suggested fix: <SUGGESTED_FIX>

   Parent tracking bead: <PARENT_BEAD_ID>"
   ```

4. **Update parent bead with sub-bead reference**:
   ```bash
   bd comments add <PARENT_BEAD_ID> "Created sub-bead <SUB_BEAD_ID> to track fix for: <ISSUE_DESCRIPTION>"
   ```

#### 4.4: Work on Fixes

For each sub-bead:

1. **Update status to in_progress**:
   ```bash
   bd update <SUB_BEAD_ID> --status in_progress
   ```

2. **Implement the fix** (may involve additional debugging, code changes)

3. **Create a new PR for the fix** if needed

4. **Close the sub-bead when fixed**:
   ```bash
   bd close <SUB_BEAD_ID> "Fixed in PR #<FIX_PR_NUMBER> / commit <COMMIT_SHA>"
   ```

#### 4.5: Track Promotion Success

When all issues are resolved and the code is stable in the target environment:

1. **Update the tracking bead**:
   ```bash
   bd comments add <BEAD_ID> "✅ Successfully deployed to <ENVIRONMENT>. All issues resolved."
   ```

2. **If ready to promote to next environment**, add a note:
   ```bash
   bd comments add <BEAD_ID> "Ready for promotion to <NEXT_ENVIRONMENT>"
   ```

3. **Close the tracking bead** when the PR journey is complete:
   ```bash
   bd close <BEAD_ID> "PR successfully deployed through all target environments"
   ```

---

## Notes

### GitHub API Notes
- **Reviews vs Comments**:
  - `#pullrequestreview-XXX` = Review summary (ignore these)
  - `#discussion_rXXX` = Inline comment with actual feedback (address these)
- Comment IDs: The `r` prefix in URL fragments is removed for API calls (`r2591027213` → `2591027213`)
- Always use `/pulls/<PR>/comments` endpoint to get actionable feedback
- If both comments and tests are selected, address comments first, then tests

### Automatic Processing Reminder
- **Phase 3A is AUTOMATIC**: When user selects "Review comments", process ALL comments without asking again
- **Do NOT present comments in a table and ask which to fix** - just fix them all
- **Reply to EVERY comment on GitHub** - this closes the feedback loop with reviewers
- The only user interaction in Phase 3A should be the initial selection in Phase 2

### Bead Tracking Notes
- **Tracking bead**: Created in Phase 1.5 with explicit ID `ai-aas-pr<PR_NUMBER>` (e.g., `ai-aas-pr55`). Requires `--force` flag due to prefix parsing.
- **Labels used**:
  - `pr-review` - Identifies this as a PR review tracking bead
  - `pr-<PR_NUMBER>` - Links to the specific PR number
  - `<BASE>-to-<HEAD>` - Shows branch flow (e.g., `develop-to-staging`)
  - `env-<ENVIRONMENT>` - Added when tracking environment issues
  - `parent-<BEAD_ID>` - Used on sub-beads to link to parent
- **Sub-beads**: Created in Phase 4 to track individual fixes needed after environment promotion
- **Dependencies**: Use `bd dep add <child> <parent>` to establish relationships
- **Finding PR beads**: `bd show ai-aas-pr<NUMBER>` (direct lookup) or `bd list --label pr-review`
- **Workflow**:
  1. Bead created at PR review start
  2. Comments added as work progresses
  3. Sub-beads created for environment issues
  4. Parent bead closed when PR deployed to all target environments
