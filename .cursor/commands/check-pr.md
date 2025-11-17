# Check and Address PR Review Issues

This command helps you check open Pull Requests, address issues raised by Gemini Code Assist, and fix failing tests.

## Quick Start

```bash
# Check all open PRs and their review status
gh pr list --state open

# Check a specific PR's review comments
gh pr view <PR_NUMBER> --comments

# Check CI/test status for a PR
gh pr checks <PR_NUMBER>
```

## What This Does

1. **Lists open PRs** - Shows all open pull requests in the repository
2. **Checks review comments** - Identifies issues raised by Gemini Code Assist
3. **Addresses issues** - Fixes code issues based on review feedback
4. **Comments on issues** - Leaves comments explaining how issues were addressed
5. **Checks tests** - Identifies and fixes any failing tests
6. **Updates PR** - Commits fixes and pushes updates

## Prerequisites

- Git repository initialized
- GitHub CLI (`gh`) authenticated: `gh auth login`
- Current branch should be the PR branch (or you'll be prompted to switch)

**Required tools:**
- `gh` - GitHub CLI for PR operations
- `git` - For committing fixes
- Access to the repository

## Workflow Overview

```
Check Open PRs
  ↓
Identify Review Issues (Gemini Code Assist)
  ↓
Address Each Issue
  ↓
Comment on Issue Thread
  ↓
Check Test Status
  ↓
Fix Failing Tests
  ↓
Commit & Push Fixes
  ↓
Verify PR Status
```

## Step-by-Step Process

### 1. Check Open Pull Requests

```bash
# List all open PRs
gh pr list --state open

# View details of a specific PR
gh pr view <PR_NUMBER>

# View PR with comments/reviews
gh pr view <PR_NUMBER> --comments
```

### 2. Check Review Comments

Gemini Code Assist leaves review comments on PRs. To see them:

```bash
# View all review comments for a PR
gh pr view <PR_NUMBER> --comments | grep -A 10 "gemini-code-assist"

# View specific review comment thread
gh api repos/:owner/:repo/pulls/<PR_NUMBER>/comments/<COMMENT_ID>
```

**Common review comment patterns:**
- Code quality issues (naming, structure, best practices)
- Security concerns
- Performance suggestions
- Documentation gaps
- Test coverage issues

### 3. Address Review Issues

For each issue raised:

1. **Understand the issue** - Read the review comment carefully
2. **Locate the code** - Find the file and line mentioned
3. **Make the fix** - Implement the suggested change or equivalent improvement
4. **Test locally** - Verify the fix doesn't break anything
5. **Comment on the issue** - Explain how you addressed it

### 4. Comment on Review Issues

After addressing an issue, leave a comment on the review thread:

```bash
# Reply to a specific review comment
gh pr comment <PR_NUMBER> --reply-to <COMMENT_ID> --body "✅ Fixed: [explanation]"

# Or add a general comment
gh pr comment <PR_NUMBER> --body "✅ **Issue addressed**: [description]"
```

**Comment format:**
```markdown
✅ **Issue: [Brief description]** - RESOLVED

I've addressed this issue by:

1. **Change made**: [What was changed]
2. **Files modified**: [List of files]
3. **Rationale**: [Why this approach was chosen]

**Verification:**
- [ ] Code updated
- [ ] Tests pass
- [ ] No breaking changes
```

### 5. Check Test Status

```bash
# Check CI/test status for a PR
gh pr checks <PR_NUMBER>

# View detailed test results
gh run list --workflow ci.yml --limit 5
gh run view <RUN_ID> --log-failed
```

**Common test failure types:**
- Unit test failures
- Integration test failures
- Linting errors
- Build failures
- Security scan issues

### 6. Fix Failing Tests

For each failing test:

1. **Identify the failure** - Check the test logs
2. **Reproduce locally** - Run the test locally to understand the issue
3. **Fix the code** - Address the root cause
4. **Verify fix** - Run tests locally before pushing
5. **Update PR** - Commit and push the fix

```bash
# Run tests locally
make test SERVICE=<service-name>

# Run specific test
go test ./path/to/test -v

# Run linting
make lint SERVICE=<service-name>

# Run all checks
make check SERVICE=<service-name>
```

### 7. Commit and Push Fixes

```bash
# Stage changes
git add <files>

# Commit with descriptive message
git commit -m "fix: Address PR review feedback

- Fixed [issue 1]: [description]
- Fixed [issue 2]: [description]
- Fixed failing test: [test name]

Addresses:
- https://github.com/owner/repo/pull/<PR_NUMBER>#discussion_r<COMMENT_ID>"

# Push to PR branch
git push origin <branch-name>
```

## Automated Script (Recommended)

Create a script to automate this process:

```bash
#!/bin/bash
# scripts/dev/check-pr.sh

PR_NUMBER="${1:-}"
if [ -z "$PR_NUMBER" ]; then
  echo "Usage: $0 <PR_NUMBER>"
  echo "Or run without args to check all open PRs"
  exit 1
fi

echo "🔍 Checking PR #${PR_NUMBER}..."

# Check PR status
gh pr view $PR_NUMBER

# Check for review comments
echo ""
echo "📝 Review Comments:"
gh pr view $PR_NUMBER --comments | grep -A 5 "gemini-code-assist" || echo "No Gemini Code Assist comments found"

# Check test status
echo ""
echo "🧪 Test Status:"
gh pr checks $PR_NUMBER

# Check for failed tests
FAILED_CHECKS=$(gh pr checks $PR_NUMBER | grep -i "fail" | wc -l)
if [ "$FAILED_CHECKS" -gt 0 ]; then
  echo ""
  echo "⚠️  Found $FAILED_CHECKS failing checks"
  echo "Run 'gh pr checks $PR_NUMBER' for details"
fi
```

## Manual Steps

### View Review Comments in Detail

```bash
# Get all review comments as JSON
gh api repos/:owner/:repo/pulls/<PR_NUMBER>/comments | jq '.[] | select(.user.login == "gemini-code-assist")'

# View specific comment thread
gh api repos/:owner/:repo/pulls/<PR_NUMBER>/comments/<COMMENT_ID>
```

### Address Specific Issue

1. **Get comment details:**
   ```bash
   gh pr view <PR_NUMBER> --comments | grep -B 5 -A 20 "<issue description>"
   ```

2. **Navigate to the file:**
   ```bash
   # Comment usually includes file path and line number
   # Example: web/portal/src/providers/AuthProvider.tsx:243
   ```

3. **Make the fix** - Edit the file according to the review feedback

4. **Test the change:**
   ```bash
   # Run relevant tests
   make test SERVICE=<service>
   
   # Or run specific test file
   npm test -- <test-file>
   go test ./path/to/test
   ```

5. **Comment on the issue:**
   ```bash
   gh pr comment <PR_NUMBER> --reply-to <COMMENT_ID> --body "✅ Fixed: [explanation]"
   ```

### Fix Failing Tests

1. **Identify failing test:**
   ```bash
   gh run view <RUN_ID> --log-failed | grep -A 20 "FAIL\|Error"
   ```

2. **Reproduce locally:**
   ```bash
   # Run the same test command locally
   make test SERVICE=<service>
   ```

3. **Fix and verify:**
   ```bash
   # Make changes
   # Run tests again
   make test SERVICE=<service>
   ```

4. **Commit fix:**
   ```bash
   git add <files>
   git commit -m "fix: Fix failing test [test name]"
   git push origin <branch>
   ```

## Example: Addressing a Review Issue

**Review Comment:**
> "The variable name `VITE_USER_ORG_SERVICE_URL` is semantically confusing since it now points to the API router, not the user-org-service directly."

**Steps to address:**

1. **Understand the issue**: Variable name doesn't match its purpose
2. **Check current usage:**
   ```bash
   grep -r "VITE_USER_ORG_SERVICE_URL" web/portal/src
   ```
3. **Make the fix**: Consolidate into `VITE_API_BASE_URL` or rename appropriately
4. **Update all references**: Files, configs, docs
5. **Test:**
   ```bash
   cd web/portal && npm run build
   ```
6. **Comment on the issue:**
   ```bash
   gh pr comment 14 --reply-to <COMMENT_ID> --body "✅ **Variable naming** - RESOLVED

   I've consolidated \`VITE_USER_ORG_SERVICE_URL\` into \`VITE_API_BASE_URL\`:
   - Removed redundant variable
   - Updated all references
   - Simplified configuration
   
   Files changed:
   - web/portal/src/providers/AuthProvider.tsx
   - web/portal/src/features/admin/api/apiKeys.ts
   - web/portal/deployments/helm/web-portal/values*.yaml"
   ```
7. **Commit and push:**
   ```bash
   git add web/portal/
   git commit -m "fix: Consolidate VITE_USER_ORG_SERVICE_URL into VITE_API_BASE_URL"
   git push origin feature/my-branch
   ```

## Example: Fixing a Failing Test

**Test Failure:**
```
FAIL: TestUserLogin (0.01s)
    auth_test.go:45: Expected status 200, got 401
```

**Steps to fix:**

1. **View test details:**
   ```bash
   gh run view <RUN_ID> --log-failed | grep -A 30 "TestUserLogin"
   ```

2. **Run test locally:**
   ```bash
   cd services/user-org-service
   go test ./internal/httpapi/auth -v -run TestUserLogin
   ```

3. **Debug the issue:**
   - Check test setup
   - Verify test data
   - Check authentication logic

4. **Fix the code:**
   - Update the failing code
   - Ensure test data is correct

5. **Verify fix:**
   ```bash
   go test ./internal/httpapi/auth -v -run TestUserLogin
   ```

6. **Commit:**
   ```bash
   git add services/user-org-service/
   git commit -m "fix: Fix TestUserLogin - correct authentication setup"
   git push origin feature/my-branch
   ```

## Quick Reference

### Common Commands

```bash
# List open PRs
gh pr list --state open

# View PR details
gh pr view <PR_NUMBER>

# View review comments
gh pr view <PR_NUMBER> --comments

# Check test status
gh pr checks <PR_NUMBER>

# View failed test logs
gh run view <RUN_ID> --log-failed

# Comment on PR
gh pr comment <PR_NUMBER> --body "Message"

# Reply to specific comment
gh pr comment <PR_NUMBER> --reply-to <COMMENT_ID> --body "Reply"

# Run tests locally
make test SERVICE=<service-name>

# Run all checks
make check SERVICE=<service-name>
```

### Review Comment Patterns

**Gemini Code Assist typically raises:**
- 🔴 **Critical**: Security vulnerabilities, breaking changes
- 🟡 **High**: Code quality, maintainability, performance
- 🟢 **Low**: Style, documentation, minor improvements

**Common issue types:**
- Variable naming confusion
- Missing error handling
- Security concerns
- Performance optimizations
- Documentation gaps
- Test coverage
- Code duplication

### Test Failure Patterns

**Common test failures:**
- Unit test assertions failing
- Integration test timeouts
- Linting errors (golint, eslint, etc.)
- Build failures
- Type errors
- Missing dependencies

## Best Practices

✅ **Do:**
- Address all review comments systematically
- Test fixes locally before pushing
- Leave clear comments explaining fixes
- Reference the original issue in commit messages
- Verify tests pass after fixes

❌ **Don't:**
- Ignore review comments
- Push untested fixes
- Leave vague comments
- Fix multiple unrelated issues in one commit
- Skip verification steps

## Troubleshooting

### Can't find review comments

```bash
# Check if Gemini Code Assist has reviewed
gh pr view <PR_NUMBER> --json reviews | jq '.[] | select(.author.login == "gemini-code-assist")'

# View all comments (including inline)
gh api repos/:owner/:repo/pulls/<PR_NUMBER>/comments
```

### Tests pass locally but fail in CI

- Check environment differences
- Verify dependencies are locked
- Check for race conditions
- Review CI configuration

### Can't reproduce test failure

```bash
# Run tests in CI-like environment
docker run --rm -v $(pwd):/work -w /work <test-image> make test

# Or check CI logs for environment details
gh run view <RUN_ID> --log | grep -i "env\|version"
```

## Related Documentation

- `docs/runbooks/code-review-process.md` - Code review guidelines
- `CONTRIBUTING.md` - Contribution guidelines
- `.github/PULL_REQUEST_TEMPLATE.md` - PR template
- `docs/platform/ci-cd-pipeline.md` - CI/CD pipeline overview

