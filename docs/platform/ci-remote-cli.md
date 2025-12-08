# CI Remote CLI Usage

---
last_updated: 2025-12-08
document_type: guide
---

`make ci-remote` wraps GitHub Actions workflow_dispatch to run the full automation pipeline from restricted machines.

## Prerequisites

1. Install GitHub CLI (`gh`) and authenticate:
   ```bash
   gh auth login --scopes repo,workflow
   ```
2. Set default repo if using enterprise instances:
   ```bash
   gh repo set-default otherjamesbrown/ai-aas
   ```

## Command Reference

```bash
make ci-remote SERVICE=user-org-service REF=$(git rev-parse HEAD) NOTES="Smoke test"
```

| Parameter | Description | Default |
|-----------|-------------|---------|
| `SERVICE` | Service to build/test | `all` |
| `REF` | Git ref to run | Current HEAD |
| `NOTES` | Run summary for auditing | (empty) |

## Output

The command prints:
1. Workflow dispatch confirmation
2. Actions run URL
3. Final status summary when workflow completes

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Workflow finished successfully |
| `1` | Dispatch failed (auth, missing workflow) |
| `2` | Workflow completed but reported failure |

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `CI_REMOTE_WORKFLOW` | Override workflow filename | `remote-ci.yml` |
| `GH_API_TOKEN` | Use specific PAT instead of CLI session | (CLI auth) |
| `CI_REMOTE_WAIT` | Set to `false` to exit without waiting | `true` |

## Examples

```bash
# Run all services on current branch
make ci-remote

# Run specific service
make ci-remote SERVICE=api-router-service

# Run on specific ref
make ci-remote SERVICE=user-org-service REF=feature-branch

# With audit note
make ci-remote SERVICE=web-portal NOTES="Pre-release validation"
```

## Troubleshooting

If `workflow_dispatch` doesn't create runs:

1. Ensure `remote-ci.yml` exists on `main` branch
2. Check for workflow syntax errors in GitHub Actions UI
3. Verify authentication: `gh auth status`
4. Review [GitHub Actions Guide](github-actions-guide.md) for common pitfalls

## Related Documentation

- [GitHub Actions Guide](github-actions-guide.md) - Workflow patterns and troubleshooting
- [CI/CD Pipeline](ci-cd-pipeline.md) - Overall pipeline architecture
