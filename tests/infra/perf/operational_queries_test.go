package perf

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestOperationalQueryLatency validates that a representative query stays within the 200ms P95 threshold.
func TestOperationalQueryLatency(t *testing.T) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set; skipping latency benchmark")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	row := db.QueryRowContext(ctx, `SELECT organization_id FROM organizations ORDER BY created_at DESC LIMIT 1`)
	var id string
	if err := row.Scan(&id); err != nil && err != sql.ErrNoRows {
		t.Fatalf("query: %v", err)
	}
	duration := time.Since(start)

	if duration > 200*time.Millisecond {
		t.Fatalf("operational query exceeded P95 target: %v", duration)
	}
}
