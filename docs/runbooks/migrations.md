# Runbook: Database Migrations

## Purpose
Provide a repeatable, auditable procedure for applying and rolling back schema changes across environments while capturing telemetry and enforcing guardrails.

## Prerequisites
- Repository cloned and up to date with the desired migration version.
- `go`, `golang-migrate`, and `psql` installed (see `specs/003-database-schemas/quickstart.md`).
- Environment file (`migrate.env`) populated with `DB_URL`, `ANALYTICS_URL`, OTEL headers, and component selector.
- Appropriate credentials to access the target database.
- Approval from a second reviewer for production changes (see Dual Approval below).

## Workflow
1. **Review Pending Changes**
   ```bash
   scripts/db/apply.sh --status --component operational --env migrate.env
   scripts/db/apply.sh --status --component analytics --env migrate.env
   ```
   Confirm the migrations to be applied and verify no drift in the target environment.

2. **Dry Run (optional but recommended)**
   ```bash
   scripts/db/apply.sh --component operational --env migrate.env --dry-run --yes
   ```
   Inspect logs for validation errors. Repeat for the analytics component as needed.

3. **Production Approval Gate**
   - Ensure a second engineer signs off on the `migrations.md` diff and migration plan.
- Set `MIGRATION_APPROVED_BY=<reviewer>` when running `scripts/db/apply.sh` (or export it in your shell).
- Record the approval in the deployment ticket or PR comments.

4. **Apply Migrations**
   ```bash
   scripts/db/apply.sh --component operational --env migrate.env --yes
   scripts/db/apply.sh --component analytics --env migrate.env --yes
   ```
   The CLI emits structured logs (`migration_start`, `migration_finish`) and OTEL spans for dashboards. Run `make db-migrate-status` to verify applied versions.

5. **Post Checks**
   - Review OTEL dashboards for duration, error rate, and row counts.
   - Execute service health checks.
   - Run `scripts/db/validate-schema.sh` to confirm docs remain in sync.

6. **Rollback (if required)**
   ```bash
   scripts/db/rollback.sh --component operational --env migrate.env --version <YYYYMMDDHHMM_slug> --yes
   scripts/db/rollback.sh --component analytics --env migrate.env --version <YYYYMMDDHHMM_slug> --yes
   ```
   Follow with smoke verification (`scripts/db/smoke-test.sh`) to confirm recoverability.

## Dual Approval Policy
- All production-bound migrations require **two approvals** (author + reviewer) prior to execution.
- Approval evidence must include:
  - Link to the reviewed migration scripts (`db/migrations/**`).
  - Reviewer’s confirmation that dry-run results and telemetry checks passed.
- Automated enforcement is handled by CI guardrails (`.github/workflows/db-guardrails.yml`).

## Telemetry & Observability
- Migrate CLI publishes OTEL spans under service `db-migrate-cli` with attributes:
  - `migration.component`
  - `migration.direction`
  - `migration.version`
  - `migration.duration_ms`
  - `migration.dry_run`
- Logs emit structured messages `migration_start`, `migration_pre_checks`, `migration_finish`, etc., which feed into log aggregation.
- `scripts/db/status.sh` wraps the CLI `--status` invocation; `scripts/db/measure-window.sh` captures apply/rollback timings and enforces the 10-minute window.

## Troubleshooting
| Symptom | Action |
|---------|--------|
| CLI exits with `pre-check failed` | Inspect pre-check logs (connectivity, pending lock detection). Resolve and retry. |
| Migration exceeds duration guardrail | Review DB load, indexes, or large table changes. Consider breaking changes into smaller batches. |
| Telemetry missing | Confirm OTEL endpoint/headers in `migrate.env`. Check network egress policies. |
| Rollback required | Execute rollback command with the exact version. Follow with smoke test to ensure system stability. |
| Row count mismatch | Inspect the row-count deltas printed by post-checks; confirm expected data movement or revert the migration. |

## References
- `configs/data-classification.yml` for encryption/retention requirements.
- `specs/003-database-schemas/plan.md` for project structure and guardrail rationale.
- CI guardrail workflow (`.github/workflows/db-guardrails.yml`) for automated enforcement.
