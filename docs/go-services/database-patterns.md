# Database Patterns

---
last_updated: 2025-12-08
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

### File Structure

```
services/<name>/migrations/
├── 001_create_models_table.up.sql
├── 001_create_models_table.down.sql
├── 002_add_status_column.up.sql
└── 002_add_status_column.down.sql
```

### Migration Format

```sql
-- 001_create_models_table.up.sql
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    endpoint VARCHAR(500) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'inactive',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_models_status ON models(status);
```

```sql
-- 001_create_models_table.down.sql
DROP TABLE IF EXISTS models;
```

### Running Migrations

Migrations are typically run at service startup or via CLI:

```go
import "github.com/golang-migrate/migrate/v4"

func RunMigrations(dbURL, migrationsPath string) error {
    m, err := migrate.New(
        "file://"+migrationsPath,
        dbURL,
    )
    if err != nil {
        return fmt.Errorf("failed to create migrator: %w", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migration failed: %w", err)
    }

    return nil
}
```

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
