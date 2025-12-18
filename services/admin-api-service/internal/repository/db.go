package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxPool defines the interface for database operations
type PgxPool interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Ping(ctx context.Context) error
	Close()
}

// DB wraps a PostgreSQL connection pool
type DB struct {
	pool PgxPool
}

// NewDB creates a new database connection pool
func NewDB(databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 5 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool returns the underlying connection pool interface
func (db *DB) Pool() PgxPool {
	return db.pool
}

// ConcretePool returns the underlying *pgxpool.Pool (for legacy services)
// Returns nil if the pool is not a *pgxpool.Pool (e.g., in tests with mocks)
func (db *DB) ConcretePool() *pgxpool.Pool {
	if pool, ok := db.pool.(*pgxpool.Pool); ok {
		return pool
	}
	return nil
}

// Ping checks database connectivity
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Stats returns connection pool statistics (only works with actual pgxpool.Pool)
func (db *DB) Stats() *pgxpool.Stat {
	if pool, ok := db.pool.(*pgxpool.Pool); ok {
		return pool.Stat()
	}
	return nil
}

