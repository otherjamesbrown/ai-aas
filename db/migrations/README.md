# Database Migrations

## Directory Structure

**CRITICAL**: Place migrations in the correct subdirectory based on target database.

```
db/migrations/
├── operational/     ← Core platform tables (MOST COMMON)
├── analytics/       ← Usage metrics and reporting
└── README.md        ← This file
```

## Quick Reference

| What You're Creating | Directory |
|---------------------|-----------|
| Organizations, Users, API Keys | `operational/` |
| Models, Deployments, Routing | `operational/` |
| Inference Engines, Credentials | `operational/` |
| Usage events, rollups | `analytics/` |
| Export jobs, reports | `analytics/` |

## File Naming

Format: `YYYYMMDDHHMMSS_description.up.sql` and `.down.sql`

Example:
```
operational/20251215100000_add_status_column.up.sql
operational/20251215100000_add_status_column.down.sql
```

## Running Migrations

```bash
# Operational database
goose -dir db/migrations/operational postgres "$DATABASE_URL" up
goose -dir db/migrations/operational postgres "$DATABASE_URL" status

# Analytics database
goose -dir db/migrations/analytics postgres "$ANALYTICS_DATABASE_URL" up
```

## Full Documentation

See [docs/go-services/database-patterns.md](../docs/go-services/database-patterns.md) for complete migration patterns.
