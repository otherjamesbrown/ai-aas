# Migrations

Database migrations for the User & Organization Service live in this directory and are executed with [`goose`](https://github.com/pressly/goose).

## Usage

Set `USER_ORG_DATABASE_URL` to a valid Postgres connection string (matching the operational database for this service) and run:

```sh
make migrate          # apply all pending migrations
make rollback         # roll back the most recent migration
make schema-drift     # show migration status to detect drift
```

`sqlc generate` uses the SQL definitions in `sql/` to produce typed data accessors under `internal/storage/sqlc/`.


