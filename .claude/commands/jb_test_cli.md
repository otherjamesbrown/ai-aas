# CLI Smoke Tests

Run CLI status checks and smoke tests against Development and Staging environments.

## Instructions

**CRITICAL: You MUST use the `AskUserQuestion` tool before running any commands.**

**Step 1: Use AskUserQuestion tool with these two questions:**

Question 1 - Environment (header: "Environment"):
- "Both (Recommended)" - Run tests against Development and Staging in parallel
- "Dev Only" - Run tests against Development environment only
- "Staging Only" - Run tests against Staging environment only

Question 2 - Output Level (header: "Output"):
- "Verbose" - Show models table, inference details, and Q&A responses
- "Normal" - Show inference details only (faster output)

**DO NOT proceed to Step 2 until user answers both questions.**

**Step 2: Run the appropriate command:**

| Environment | Output | Command |
|-------------|--------|---------|
| Dev Only + Verbose | `--dev-only --verbose` |
| Dev Only + Normal | `--dev-only` |
| Staging Only + Verbose | `--staging-only --verbose` |
| Staging Only + Normal | `--staging-only` |
| Both + Verbose | `--verbose` |
| Both + Normal | (no flags) |

```bash
/home/dev/worktrees/develop/scripts/cli-smoke-test.sh [flags]
```

**Step 3: Present results using this template:**

---

## CLI Smoke Test Results

### Summary
| Environment | Tests | Status |
|-------------|-------|--------|
| Development | X/10  | PASS/FAIL |
| Staging     | X/10  | PASS/FAIL |

### Test Breakdown
| Test | Development | Staging |
|------|-------------|---------|
| create_org | PASS/FAIL | PASS/FAIL |
| create_user | PASS/FAIL | PASS/FAIL |
| grant_model_access | PASS/FAIL | PASS/FAIL |
| activate_user | PASS/FAIL | PASS/FAIL |
| create_apikey | PASS/FAIL | PASS/FAIL |
| list_models | PASS/FAIL | PASS/FAIL |
| inference | PASS/FAIL | PASS/FAIL |
| inference_health | PASS/FAIL | PASS/FAIL |
| usage_query | PASS/FAIL | PASS/FAIL |
| cleanup | PASS/FAIL | PASS/FAIL |

### Inference Details (All Models)
| Environment | Model | Status | Prompt | Completion | Time (ms) |
|-------------|-------|--------|--------|------------|-----------|
| Development | model-name | PASS/FAIL | X | X | X |
| ... | ... | ... | ... | ... | ... |

### Published Models (verbose only)
| Environment | Model ID | Owner |
|-------------|----------|-------|
| Development | model-id | owner |
| ... | ... | ... |

### Q&A Details (verbose only)

**Development:**
- Model: model-name (PASS/FAIL)
  - Q: What is the capital of France?
  - A: [response text or error message]

**Staging:**
- Model: model-name (PASS/FAIL)
  - Q: What is the capital of France?
  - A: [response text or error message]

---

## Handling Failures

If tests fail, check for existing issues:
```bash
bd list --status open --label cli-smoke-test
```

## Git-Crypt Note

If you get an error about `MASTER_ADMIN_API_KEY not set`:
```bash
git crypt unlock ~/.config/git-crypt/ai-aas-key
```
