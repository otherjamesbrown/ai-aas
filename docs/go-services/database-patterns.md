# Database Patterns

---
last_updated: 2025-12-27
document_type: guide
---

## Overview

This document defines database patterns for Go services in the AI-AAS platform.

## Connection Management

### Connection String

Database URLs are provided via environment variables:

```go
// Read from environment
dbURL := os.Getenv("DATABASE_URL")
// Format: postgres://user:password@host:port/database?sslmode=require
```

### Connection Pool

Configure connection pools appropriately:

```go
import (
    "database/sql"
    _ "github.com/lib/pq"
)

func NewDB(connStr string) (*sql.DB, error) {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Connection pool settings
    db.SetMaxOpenConns(50)      // Maximum open connections
    db.SetMaxIdleConns(10)      // Maximum idle connections
    db.SetConnMaxLifetime(time.Hour) // Connection max lifetime

    // Verify connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return db, nil
}
```

## Query Patterns

### Context with Timeout

Always use context with timeout for database operations:

```go
func (r *Repository) GetModel(ctx context.Context, name string) (*Model, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    var model Model
    err := r.db.QueryRowContext(ctx,
        `SELECT id, name, endpoint, status FROM models WHERE name = $1`,
        name,
    ).Scan(&model.ID, &model.Name, &model.Endpoint, &model.Status)

    if err == sql.ErrNoRows {
        return nil, &NotFoundError{Resource: "model", ID: name}
    }
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }

    return &model, nil
}
```

### Transactions

Use transactions for multi-step operations:

```go
func (r *Repository) CreateOrganizationWithAdmin(ctx context.Context, org *Organization, admin *User) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback() // Rollback if not committed

    // Insert organization
    _, err = tx.ExecContext(ctx,
        `INSERT INTO organizations (id, name) VALUES ($1, $2)`,
        org.ID, org.Name,
    )
    if err != nil {
        return fmt.Errorf("failed to insert organization: %w", err)
    }

    // Insert admin user
    _, err = tx.ExecContext(ctx,
        `INSERT INTO users (id, org_id, email, role) VALUES ($1, $2, $3, $4)`,
        admin.ID, org.ID, admin.Email, "admin",
    )
    if err != nil {
        return fmt.Errorf("failed to insert admin user: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}
```

### Batch Operations

Use batch inserts for efficiency:

```go
func (r *Repository) BulkInsertUsageRecords(ctx context.Context, records []*UsageRecord) error {
    if len(records) == 0 {
        return nil
    }

    // Build bulk insert query
    valueStrings := make([]string, 0, len(records))
    valueArgs := make([]interface{}, 0, len(records)*4)

    for i, r := range records {
        valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)", i*4+1, i*4+2, i*4+3, i*4+4))
        valueArgs = append(valueArgs, r.ID, r.OrgID, r.Tokens, r.Timestamp)
    }

    query := fmt.Sprintf(
        `INSERT INTO usage_records (id, org_id, tokens, timestamp) VALUES %s`,
        strings.Join(valueStrings, ","),
    )

    _, err := r.db.ExecContext(ctx, query, valueArgs...)
    if err != nil {
        return fmt.Errorf("bulk insert failed: %w", err)
    }

    return nil
}
```

## Migrations

We use [goose](https://github.com/pressly/goose) v3 for database migrations.

### File Structure

```
db/migrations/operational/
├── 20251115001_core_entities.sql
├── 20251115002_usage_events.sql
└── 20251126001_routing_policies.sql
```

### Migration Format (Goose v3)

Goose v3 uses a single file with `-- +goose Up` and `-- +goose Down` annotations:

```sql
-- +goose Up
-- Create models table
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    endpoint VARCHAR(500) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'inactive',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_models_status ON models(status);

-- +goose Down
-- Drop models table
DROP TABLE IF EXISTS models;
```

### Migration File Naming

Migration files follow the format: `<timestamp>_<description>.sql`

Example: `20251115001_core_entities.sql`

### Running Migrations

Migrations are run automatically via init container in Kubernetes deployments:

```bash
goose -dir /app/migrations/sql postgres "$DATABASE_URL" up
```

For local development:

```bash
goose -dir db/migrations/operational postgres "$DATABASE_URL" up
goose -dir db/migrations/operational postgres "$DATABASE_URL" down  # Rollback last
goose -dir db/migrations/operational postgres "$DATABASE_URL" status
```

### Migration Configuration

Migrations are configured in Helm values:

```yaml
# values-development.yaml
migrations:
  enabled: true  # Run migrations via init container
```

The init container runs before the main application container starts, ensuring the database schema is up to date.

## Using sqlc

We use [sqlc](https://sqlc.dev/) for type-safe SQL:

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "migrations/"
    gen:
      go:
        package: "db"
        out: "internal/db"
```

```sql
-- queries/models.sql

-- name: GetModel :one
SELECT * FROM models WHERE name = $1;

-- name: ListModels :many
SELECT * FROM models ORDER BY name LIMIT $1 OFFSET $2;

-- name: CreateModel :one
INSERT INTO models (name, endpoint, status)
VALUES ($1, $2, $3)
RETURNING *;
```

Generate with:
```bash
sqlc generate
```

## Upserts and Deduplication

### ON CONFLICT Requirements

**CRITICAL**: `ON CONFLICT` clauses MUST match an existing unique constraint exactly.

PostgreSQL requires that the columns in `ON CONFLICT (...)` match a `UNIQUE` constraint, `UNIQUE INDEX`, or `PRIMARY KEY` exactly. Mismatches will cause runtime errors.

```go
// CORRECT: Matches unique constraint
// Table: UNIQUE(event_id)
INSERT INTO events (...) VALUES (...)
ON CONFLICT (event_id) DO NOTHING

// CORRECT: Matches composite constraint
// Table: UNIQUE(org_id, user_id)
INSERT INTO user_access (...) VALUES (...)
ON CONFLICT (org_id, user_id) DO UPDATE SET ...

// WRONG: No matching constraint
// Table: PRIMARY KEY (event_id, occurred_at)
INSERT INTO events (...) VALUES (...)
ON CONFLICT (event_id) DO NOTHING  // ERROR: constraint doesn't exist

// FIX: Add unique index or use matching columns
CREATE UNIQUE INDEX idx_events_event_id ON events (event_id);
-- Now ON CONFLICT (event_id) works
```

### TimescaleDB Composite Keys

TimescaleDB hypertables require the partitioning column in the primary key. This creates a composite key that may not match your deduplication needs:

```sql
-- Table design for TimescaleDB
CREATE TABLE usage_events (
    event_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    -- Other columns...
    PRIMARY KEY (event_id, occurred_at)  -- Required for hypertable
);

-- Add unique index for deduplication
CREATE UNIQUE INDEX idx_usage_events_event_id
    ON usage_events (event_id);

-- Now ON CONFLICT works
INSERT INTO usage_events (...) VALUES (...)
ON CONFLICT (event_id) DO NOTHING;
```

**Pattern**: When using TimescaleDB or other partitioned tables:
1. Use composite PRIMARY KEY for partitioning requirements
2. Add separate UNIQUE INDEX on deduplication column(s)
3. Reference the UNIQUE INDEX in ON CONFLICT clause

## Error Handling

Map database errors to domain errors:

```go
import "github.com/lib/pq"

func (r *Repository) Create(ctx context.Context, model *Model) error {
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO models (name, endpoint) VALUES ($1, $2)`,
        model.Name, model.Endpoint,
    )
    if err != nil {
        // Check for unique constraint violation
        if pqErr, ok := err.(*pq.Error); ok {
            if pqErr.Code == "23505" { // unique_violation
                return &ConflictError{
                    Resource: "model",
                    ID:       model.Name,
                    Message:  "model already exists",
                }
            }
        }
        return fmt.Errorf("insert failed: %w", err)
    }
    return nil
}
```

## Related Documents

- [error-handling.md](error-handling.md) - Error handling patterns
- Service DEPLOYMENT.md files for database configuration requirements
