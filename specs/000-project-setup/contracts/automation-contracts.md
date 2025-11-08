# Automation Contracts

**Branch**: `000-project-setup`  
**Date**: 2025-11-08  

## CLI Contracts

| Command | Inputs | Outputs | Notes |
|---------|--------|---------|-------|
| `make help` | none | Task list printed to stdout | Must include descriptions for `build`, `test`, `lint`, `check`, `ci-remote`, `ci-local`, `service-new` |
| `make build` | optional `SERVICE=<name>` | Build artifacts under `services/<name>/bin/` | Defaults to all services when SERVICE unset |
| `make test` | optional `SERVICE=<name>` | Test results to stdout, exit 0/1 | Should support `TEST_FLAGS` for advanced usage |
| `make check` | optional `SERVICE=<name>`, `METRICS=true|false` | Combined lint/format/security results | When `METRICS=true`, writes telemetry JSON via collector |
| `make ci-local` | optional `WORKFLOW=ci` | Simulated GitHub Actions run using `act` | Requires Docker; if unavailable, print remediation |
| `make ci-remote` | `SERVICE`, optional `REF`, `NOTES` | Triggers workflow_dispatch, echoes run URL and final status | Must exit with same status as remote workflow |
| `make service-new` | `NAME` (kebab-case) | Generates new service skeleton | Adds entry to `go.work`, copies `templates/service.mk` |

## GitHub Actions Workflows

### `ci.yml` (push/pull_request)
- **Jobs**:
  1. `setup`: checkout, cache Go modules, install tooling.
  2. `build`: matrix over services, runs `make build SERVICE=<name>`.
  3. `test`: matrix over services, runs `make test SERVICE=<name>`.
  4. `lint`: runs `make check`.
  5. `metrics-upload`: publishes JSON artifacts to S3 bucket.
- **Outputs**: status summary, metrics artifact URLs.
- **Failure Policy**: any job failure fails workflow; metrics upload still executes with `if: always()` to capture timings.

### `ci-remote.yml` (workflow_dispatch)
- **Inputs**:
  - `service` (string, optional, default `all`)
  - `ref` (string, optional, default `current`)
  - `notes` (string, optional)
- **Jobs**:
  1. `dispatch-info`: echo inputs for audit log.
  2. `pipeline`: reuses `ci.yml` composite to run same steps.
  3. `notify`: comments on associated PR (if any) with results.
- **Outputs**:
  - `run_url`: GitHub Actions run URL.
  - `status`: final status.

## Metrics Schema

```json
{
  "run_id": "<GitHub run id>",
  "service": "user-org-service",
  "command": "make check",
  "status": "success",
  "started_at": "2025-11-08T15:04:05Z",
  "finished_at": "2025-11-08T15:06:07Z",
  "duration_seconds": 122.3,
  "commit_sha": "abcdef1234567890",
  "actor": "dev@example.com",
  "environment": "github-actions",
  "collector_version": "1.0.0"
}
```

- Files stored at `s3://ai-aas-build-metrics/YYYY/MM/DD/<service>/<run-id>.json`.
- Upload job must set metadata headers (`Content-Type: application/json`, `x-run-status`).
- Retention enforced via lifecycle policy (≥30 days).

## Error Handling

- CLI commands return non-zero exit codes on failure with actionable messages.
- Remote workflow surfaces run URL even when failing.
- Metrics upload logs warnings but does not fail pipeline unless upload retries (3 attempts) fail.

