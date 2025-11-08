# CI Remote CLI Usage

`make ci-remote` wraps GitHub Actions workflow_dispatch to run the full automation pipeline from restricted machines.

## Prerequisites

1. Install GitHub CLI (`gh`) and authenticate:
   ```bash
   gh auth login --scopes repo,workflow
   ```
2. Ensure `GH_HOST` and default repo are set if using enterprise instances:
   ```bash
   gh repo set-default otherjamesbrown/ai-aas
   ```

## Command Reference

```bash
make ci-remote SERVICE=user-org-service REF=$(git rev-parse HEAD) NOTES="Smoke test"
```

- `SERVICE`: Optional; defaults to `all`. Pass a specific service to scope build/test matrices.
- `REF`: Git ref to run (defaults to current HEAD). Useful when testing PR branches.
- `NOTES`: Appended to workflow run summary for auditing.

The command prints:

1. Workflow dispatch confirmation
2. Actions run URL
3. Final status summary when the workflow completes

## Exit Codes

- `0`: Workflow finished successfully.
- `1`: Dispatch failed (invalid auth, missing workflow).
- `2`: Workflow completed but reported failure (inspect the Actions log).

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `CI_REMOTE_WORKFLOW` | Override workflow filename (default `ci-remote.yml`). |
| `GH_API_TOKEN` | Use a specific PAT instead of the authenticated CLI session. |
| `CI_REMOTE_WAIT` | Set to `false` to dispatch and exit immediately without waiting. |

## Troubleshooting

If `workflow_dispatch` doesn't create runs:
1. Ensure the workflow exists on the `main` branch
2. Check for workflow syntax errors in GitHub Actions UI
3. See `docs/troubleshooting/ci-remote-dispatch.md` for detailed resolution
4. Review `docs/platform/github-actions-guide.md` for common pitfalls

For other CI issues, see `docs/troubleshooting/ci.md`.

