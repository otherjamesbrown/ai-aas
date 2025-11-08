# Runbook: Remote CI Execution

## Purpose

Allow contributors on restricted machines to run the CI pipeline via GitHub Actions workflow_dispatch.

## Steps

1. Ensure prerequisites:
   - GitHub CLI authenticated (`gh auth status`)
   - `LINODE_TOKEN` configured if workflows interact with Linode resources
2. Trigger workflow:
   ```bash
   make ci-remote SERVICE=all REF=$(git rev-parse HEAD) NOTES="Smoke test"
   ```
3. Monitor run via URL printed by the command.
4. On success, metrics artifacts appear in `s3://ai-aas-build-metrics`.
5. On failure:
   - Inspect Actions logs
   - Re-run locally with `make ci-local` if Docker available
   - Capture logs and open issue if infrastructure-related

## SLA

- Expected completion time: < 10 minutes.
- If exceeded, investigate queue length and resource contention.

## Validation Log

| Date | Service | Ref | Result | Notes |
|------|---------|-----|--------|-------|
| 2025-11-08 | all | feature/000-project-setup | ✅ Success | Full pipeline parity with CI workflow. |
| 2025-11-08 | user-org-service | refs/heads/main | ✅ Success | Metrics artifact uploaded (dry-run). |

