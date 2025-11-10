# Quickstart: Database Schemas & Migrations

**Branch**: `003-database-schemas-clarifications`  
**Date**: 2025-11-10  
**Audience**: Platform engineers, service developers, analytics stakeholders

## Prerequisites
- macOS, Linux, or Windows (WSL2) with Docker Desktop or Podman
- Go 1.21+ installed locally (`go version`)
- `golang-migrate` CLI (`migrate -version`)
- Access to managed PostgreSQL instance (staging or sandbox) and analytics warehouse credentials
- Platform observability credentials (OpenTelemetry exporter endpoint + API token)
- `make`, `jq`, `psql`, `dbt` CLI (>=1.7) or equivalent transforms runner

> **Tip**: Run `make tools-check` after cloning to verify all required binaries are available. The command prints install instructions if anything is missing.

## 1. Bootstrap Local Databases
1. Clone the repository and checkout the feature branch:
   ```bash
   git clone git@github.com:otherjamesbrown/ai-aas.git
   cd ai-aas
   git checkout 003-database-schemas-clarifications
   ```
2. Start containerized services:
   ```bash
   make dev-db-up             # launches Postgres + analytics warehouse emulator
   ```
3. Verify connectivity:
   ```bash
   make dev-db-health         # runs readiness probes against both stores
   ```

## 2. Apply Baseline Migrations
1. Export required environment variables (sample config lives in `configs/migrate.example.env`):
   ```bash
   export DB_URL=postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable
   export ANALYTICS_URL=postgres://analytics:analytics@localhost:6432/ai_aas_warehouse?sslmode=disable
   export OTEL_EXPORTER_OTLP_ENDPOINT=https://otel.dev.ai-aas.internal
   export OTEL_EXPORTER_OTLP_HEADERS="x-api-key=$(cat ~/.config/ai-aas/otel.token)"
   ```
2. Run migrations including telemetry (use `--yes` to skip confirmation prompts in non-production environments):
   ```bash
  scripts/db/apply.sh --env local --component operational --yes
  scripts/db/apply.sh --env local --component analytics --yes
   ```
3. Inspect the migration status table:
   ```bash
   make db-migrate-status
   scripts/db/status.sh --env migrate.env --component operational
   ```
- Measure timing (optional, enforces the 10-minute window):
  ```bash
  scripts/db/measure-window.sh --env migrate.env --component operational --version 20251115002_usage_events --rollback
  ```

## 3. Seed Deterministic Fixtures
1. Populate operational data (organizations, users, API keys):
   ```bash
   scripts/db/seed.sh --package local-dev --component operational
   ```
2. Seed analytics usage events for dashboards:
   ```bash
   scripts/db/seed.sh --package local-dev --component analytics
   ```
3. Run smoke tests to validate end-to-end flows:
   ```bash
   scripts/db/smoke-test.sh --env local
   ```

## 4. Explore Schema Documentation
- Generated ERD and entity dictionary are published under `db/docs/`.
- To regenerate after editing migrations:
  ```bash
  make db-docs-generate
  ```
- Verify documentation consistency:
  ```bash
  make db-docs-validate
  ```

## 5. Run Analytics Rollups
1. Execute hourly rollup transform locally:
   ```bash
   make analytics-rollup-run GRANULARITY=hour
   ```
2. Confirm reconciliation results:
   ```bash
   make analytics-verify
   ```
3. Choose adapter (local vs warehouse):
   ```bash
   export ANALYTICS_ADAPTER=default   # or warehouse
   cat analytics/transforms/config.yml
   ```

## 6. Perform Apply → Rollback → Reapply
1. Choose a migration version (e.g., `202511101530_add_audit_index`), then:
   ```bash
   scripts/db/apply.sh --version 202511101530_add_audit_index --component operational
   scripts/db/rollback.sh --version 202511101530_add_audit_index --component operational
   scripts/db/apply.sh --version 202511101530_add_audit_index --component operational
   ```
2. Observe telemetry in Grafana dashboard `DB Migration Runs`; ensure metrics show three runs (apply, rollback, apply).

## 7. Prepare for CI/Staging
- Upload seed artifacts to object storage:
  ```bash
  make db-seed-package ENV=ci
  ```
- Trigger CI validation:
  ```bash
  make db-validate-ci
  ```
- For staging, set `ENV=staging` and provide managed DB credentials. CI pipeline enforces the same smoke test workflow prior to deployment.

## Troubleshooting
- `migrate` failures: Inspect structured logs (`logs/db-migrate/*.json`) and rerun with `--dry-run` to diagnose.
- Telemetry not visible: Ensure OTEL endpoint/token exports and network egress are allowed.
- Seed idempotency issues: Run `make db-clean` to reset local fixtures before retrying.
- Analytics rollups slow: Check warehouse resource allocation; adjust concurrency via `configs/analytics/rollups.yml`.

## Next Steps
- Review `data-model.md` and `contracts/` for entity definitions and service boundaries.
- Coordinate with security for data classification updates before introducing new sensitive fields.
- Open follow-up tasks for infrastructure provisioning (managed DB sizing, analytics warehouse quotas) as needed.

