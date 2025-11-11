# DB Guardrails Troubleshooting

## Context
- Workflow: `DB Guardrails` on PR #5 (“Finalize database guardrails and analytics rollups”).
- Target environment: remote Postgres instance (`defaultdb` on Akamai-managed cluster) accessed via `migrate.env`.
- Goal: run migrations, seeds, and smoke tests through `scripts/db/*` automation.
- Representative failure run: [guardrails job](https://github.com/otherjamesbrown/ai-aas/actions/runs/19258963128/job/55071055748?pr=5) on 2025‑11‑11.

## Symptoms Observed
- Guardrails job failed at “Seed databases” because migrations never created the expected tables.
- Local reruns of `scripts/db/apply.sh` emitted:

```
migration command failed: migrations dir .../db/tools/migrations/operational: stat .../db/tools/migrations/operational: no such file or directory
```

- Operational seed (`scripts/db/seed.sh --component operational`) aborted with:

```
email hash key: MIGRATION_EMAIL_HASH_KEY (or EMAIL_HASH_KEY) must be set
```

- Smoke test (`scripts/db/smoke-test.sh`) stopped early:

```
[ERROR] MIGRATION_APPROVED_BY must be set (dual approval required)
```

## Root Causes
1. **Incorrect migration directory lookup**  
   The custom migrate CLI resolved paths relative to `db/tools/migrate`, producing `db/tools/migrations/...`, but the actual SQL lives under `db/migrations/...`. As a result, migrations were never applied and tables were missing for seeding.

2. **Missing deterministic email hash key**  
   The operational seed now requires `MIGRATION_EMAIL_HASH_KEY` (or `EMAIL_HASH_KEY`) to HMAC email addresses. Without the key, the seed process exits immediately.

3. **Dual-approval enforcement in automation scripts**  
   `scripts/db/apply.sh` checks `MIGRATION_APPROVED_BY` unless `--dry-run` is used. The smoke test wraps `apply.sh`, so the variable must be present even when running locally.

## Fixes Implemented
- Updated `db/tools/migrate/main.go` so `migrationsDir` climbs two directories and targets `db/migrations/<component>`. After this change:
  - `apply-operational.log` shows `migration_applied` entries for both operational migrations.
  - `apply-analytics.log` confirms the analytics rollup migration applied.
- Hardened `scripts/db/apply.sh` and `scripts/db/seed.sh` with preflight guards:
  - `apply.sh` now checks for the presence of `db/migrations/<component>` and passes the resolved path via `MIGRATIONS_ROOT`.
  - `seed.sh` fails fast when `MIGRATION_EMAIL_HASH_KEY` (or `EMAIL_HASH_KEY`) is missing.
- Updated the `DB Guardrails` workflow so both operational and analytics runs point to the same Postgres database.  
  The analytics seed queries operational tables (`organizations`, `api_keys`, etc.), so using a single DSN avoids `relation ... does not exist` errors on CI (`/.github/workflows/db-guardrails.yml`).
- Reran seeds with the necessary environment:

```
MIGRATION_APPROVED_BY=ci-local \
MIGRATION_EMAIL_HASH_KEY=ci-seed-email-hmac \
  scripts/db/seed.sh --env migrate.env --component operational

scripts/db/seed.sh --env migrate.env --component analytics
```

  Operational seed completed with organization/user/api-key/model IDs logged, and analytics seed succeeded (empty log on success).
- Executed the smoke test with both variables set; the script completed (no rollback path because no explicit `--version` was provided).

## Verification
- `tmp/apply-operational.log` and `tmp/apply-analytics.log` show successful migrations with post-check row counts and audit-log inserts.
- `tmp/seed-operational.log` prints the “Seed completed” summary.
- `tmp/smoke-test.log` captures the full run of `apply.sh --component operational --env migrate.env --yes` followed by the status check, ending cleanly.

## Residual Risks & Follow-Up
- Re-run the GitHub `DB Guardrails` workflow now that the migrate CLI is fixed and seed prerequisites are documented. Confirm the “Seed databases” step no longer exits with `relation ... does not exist` in future runs.
- Ensure CI runners export `MIGRATION_EMAIL_HASH_KEY` and `MIGRATION_APPROVED_BY` via secrets or workflow env before invoking seeds/smoke-test to prevent `email hash key` or approval errors.
- The guardrails run surfaced a cache warning (“Restore cache failed: Dependencies file is not found … go.sum”); evaluate if the workflow should snapshot module sums or disable caching for tools modules to avoid repeated warnings.
- Consider adding preflight validation (e.g., check required environment variables, verify migration directories) to the scripts to fail fast with actionable messages when inputs are missing.

