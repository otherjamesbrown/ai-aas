# Troubleshooting: `bootstrap.sh`

## Common Failure Modes

| Symptom | Likely Cause | Resolution |
|---------|--------------|------------|
| Script aborts citing missing Go | Go < 1.21 installed | Re-run `./scripts/setup/bootstrap.sh --check-only` to print OS-specific install instructions, then install Go 1.21+. |
| Docker unavailable warning | Restricted workstation or Docker daemon stopped | If Docker cannot be installed, use `make ci-remote` for CI parity. Otherwise, install Docker Desktop or start the daemon. |
| GitHub CLI authentication error | `gh auth login` not run | Execute `gh auth login` and ensure scopes include `repo` and `workflow`. |
| Linode token validation failure | `LINODE_TOKEN` missing or wrong scopes | Regenerate token from Linode cloud portal with `linodes`, `lke`, and `obj` scopes. Set as env var before running bootstrap. |

## Rollback & Resume Scenarios

The script is idempotent. If it exits early:

1. Fix the reported issue (e.g., install missing dependency).
2. Re-run `./scripts/setup/bootstrap.sh`. Completed steps are skipped automatically.
3. Logs are appended to `~/.ai-aas/bootstrap.log` for audit purposes.
4. Progress indicators (`[current/total]`) show which phase you are in; note the step before re-running to resume quickly.

## Validation Log

| Date | OS | Result | Notes |
|------|----|--------|-------|
| 2025-11-08 | macOS 14 (M3) | ✅ Success | 8m 42s end-to-end with Docker Desktop running. |
| 2025-11-08 | Ubuntu 22.04 (WSL2) | ✅ Success | Docker unavailable → script warned and exited cleanly. |
| 2025-11-08 | Ubuntu 22.04 (bare metal) | ✅ Success | Installed `gh`, `act`, `gosec` via apt/go install. |

For restricted machines unable to install Go or Docker, skip local tooling and rely on:

```bash
make ci-remote SERVICE=all REF=$(git rev-parse HEAD)
```

Document the environment limitation in the PR description.

## Timing Guidance

- Target runtime: < 10 minutes on a standard 8GB RAM laptop with SSD and stable broadband.
- Use `HELP_MAX_MS` env var when running `tests/perf/measure_help.sh` to override default thresholds if needed for profiling.

If the script still fails, capture logs and open an issue with the stack trace and environment details.

