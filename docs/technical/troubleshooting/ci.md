# Troubleshooting: CI Parity & Remote Execution

## Local `make check` vs Remote `ci`

- Ensure `configs/tool-versions.mk` aligns with local tool versions; run `make version` to print expected versions.
- When `make check` fails locally, use `make fmt` or follow linter hints. Remote CI will mirror the same failure output.
- To reproduce remote failures locally, run:
  ```bash
  make ci-local WORKFLOW=ci
  ```
  This uses `act` and Docker; if unavailable, rely on remote workflows (see below).

## Remote Execution (`make ci-remote`)

- Authenticate with GitHub CLI: `gh auth status` must report logged-in with `workflow` scope.
- Provide optional metadata:
  ```bash
  make ci-remote SERVICE=user-org-service REF=$(git rev-parse HEAD) NOTES="Testing from restricted laptop"
  ```
- The command prints the Actions run URL. Monitor progress there; expected turnaround < 10 minutes.

## Common Issues

| Symptom | Resolution |
|---------|------------|
| `gh` command not found | Install GitHub CLI from https://cli.github.com and run `gh auth login`. |
| Workflow dispatch failed (403) | Ensure token scopes include `repo` and `workflow`; verify branch permissions. |
| Metrics upload failed | Check `METRICS_BUCKET` secret and IAM permissions; see `docs/metrics/policy.md`. |
| `act` image missing | Pull the required container: `act pull ghcr.io/catthehacker/ubuntu:act-latest`. |

For persistent issues, capture the `make` command output and the GitHub run URL, then open a troubleshooting ticket.

