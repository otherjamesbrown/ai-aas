package perf

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestAuditLogEntryExists(t *testing.T) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not available; skipping audit verification")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	var count int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log_entries WHERE occurred_at > NOW() - INTERVAL '15 minutes'`).Scan(&count); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected at least one audit log entry in last 15 minutes")
	}
}
