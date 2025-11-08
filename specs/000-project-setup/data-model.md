# Data Model: Project Setup & Repository Structure

**Branch**: `000-project-setup`  
**Date**: 2025-11-08  
**Spec**: `/specs/000-project-setup/spec.md`

## Entities

### RepositoryStructure
- **description**: Canonical layout of directories, shared configs, and automation assets.
- **fields**:
  - `directories`: array (e.g., `services/`, `scripts/`, `.github/workflows/`, `configs/`)
  - `templates`: array of template file paths (e.g., `templates/service.mk`)
  - `docs`: array of documentation files (`README.md`, `CONTRIBUTING.md`)
- **relationships**:
  - Includes multiple **ServiceModule** entries.
  - References shared **AutomationTask** definitions.
- **rules**:
  - Directory names must be kebab-case.
  - Shared configs live under `configs/`.

### ServiceModule
- **description**: Represents an individual Go service adopting shared automation.
- **fields**:
  - `name`: string (e.g., `user-org-service`)
  - `path`: string (`services/<service-name>/`)
  - `makefile`: path to service Makefile including shared template.
  - `go_module`: Go module path (matches `go.work` entry).
- **relationships**:
  - Includes **AutomationTask** references (build, test, lint).
  - Linked to **MetricsRecord** entries produced by builds/tests.
- **rules**:
  - Must include `include ../../templates/service.mk`.
  - Must define service-specific variables via documented extension points.

### AutomationTask
- **description**: Standard tasks exposed via Make and GitHub Actions.
- **fields**:
  - `name`: string (`build`, `test`, `lint`, `check`, `ci-remote`)
  - `command`: string executed by Make or workflow step.
  - `description`: human-readable summary.
  - `scope`: enum (`root`, `service`, `ci`).
- **relationships**:
  - Executed by **ServiceModule** and **CIWorkflow**.
- **rules**:
  - Must be discoverable via `make help`.
  - Output must conform to logging conventions for metrics parsing.

### CIWorkflow
- **description**: GitHub Actions workflows orchestrating automation.
- **fields**:
  - `name`: string (`ci`, `ci-remote`)
  - `triggers`: array (`push`, `pull_request`, `workflow_dispatch`)
  - `jobs`: array of job definitions (build, test, lint, security, metrics).
  - `artifacts`: metrics JSON, logs, binaries.
- **relationships**:
  - Uses **AutomationTask** commands.
  - Produces **MetricsRecord**.
- **rules**:
  - `ci-remote` must be invocable via `make ci-remote`.
  - Workflows share composite action or reusable job definitions under `.github/workflows/reusable/`.

### MetricsRecord
- **description**: Structured telemetry emitted after builds/tests.
- **fields**:
  - `run_id`: UUID or GitHub run ID.
  - `service`: service name or `all`.
  - `command`: executed task (e.g., `make check`).
  - `status`: enum (`success`, `failure`, `cancelled`).
  - `started_at` / `finished_at`: timestamps (RFC3339).
  - `duration_seconds`: float.
  - `commit_sha`: string.
  - `collector_version`: semantic version.
- **relationships**:
  - Stored in **MetricsStorage** (S3 bucket).
  - Linked back to **CIWorkflow** for auditing.
- **rules**:
  - Retain at least 30 days of records.
  - Upload JSON path: `metrics/<YYYY>/<MM>/<DD>/<run_id>.json`.

### MetricsStorage
- **description**: Physical storage for metrics artifacts.
- **fields**:
  - `provider`: enum (`aws-s3`, `minio`).
  - `bucket`: string.
  - `credentials_secret`: reference to GitHub Actions secret.
  - `retention_days`: integer (>= 30).
- **relationships**:
  - Receives **MetricsRecord** uploads.
- **rules**:
  - Delete objects older than retention window via scheduled workflow.

### SetupScript
- **description**: CLI entry point that prepares local environment.
- **fields**:
  - `path`: `scripts/setup/bootstrap.sh`.
  - `checks`: array (Go version, Git, Docker, gh CLI).
  - `install_steps`: list of commands with OS detection.
  - `success_message`: string.
- **relationships**:
  - References **AutomationTask** to show next steps.
- **rules**:
  - Must be idempotent.
  - All failure paths return non-zero exit codes with actionable guidance.

### RemoteExecutionWorkflow
- **description**: CLI + workflow path enabling restricted machines to execute CI remotely.
- **fields**:
  - `cli_target`: `make ci-remote`.
  - `script`: `scripts/ci/trigger-remote.sh`.
  - `workflow_name`: `ci-remote`.
  - `response_time_sla`: 600 seconds.
- **relationships**:
  - Triggers **CIWorkflow** (`ci-remote`).
  - Produces **MetricsRecord** similar to local runs.
- **rules**:
  - Requires authenticated GitHub CLI or PAT.
  - Must surface run URL and status to contributor.

