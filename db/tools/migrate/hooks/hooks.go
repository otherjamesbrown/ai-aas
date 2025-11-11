package hooks

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	connectionTimeout      = 10 * time.Second
	blockingQueryThreshold = 5 * time.Minute
)

// PreCheckInput captures the metadata supplied to pre-migration checks.
type PreCheckInput struct {
	Component     string
	Direction     string
	TargetVersion string
	DryRun        bool
}

// PreCheckResult captures information gathered during pre-checks.
type PreCheckResult struct {
	RowCounts map[string]int64
}

// PostCheckInput captures metadata for post-migration assertions.
type PostCheckInput struct {
	Component       string
	Direction       string
	TargetVersion   string
	DryRun          bool
	Duration        time.Duration
	PreCheckResults PreCheckResult
}

// RunPreChecks executes guardrails prior to applying migrations.
func RunPreChecks(ctx context.Context, input PreCheckInput) (PreCheckResult, error) {
	log.Printf("migration_pre_checks component=%s direction=%s version=%s dry_run=%t", input.Component, input.Direction, input.TargetVersion, input.DryRun)

	dsn, err := dsnForComponent(input.Component)
	if err != nil {
		return PreCheckResult{}, err
	}

	connCtx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return PreCheckResult{}, fmt.Errorf("pre-check connect: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(connCtx); err != nil {
		return PreCheckResult{}, fmt.Errorf("pre-check ping: %w", err)
	}

	rows, err := db.QueryContext(ctx, `
SELECT pid, state, query,
       EXTRACT(EPOCH FROM (now() - query_start))::bigint AS age_seconds
FROM pg_stat_activity
WHERE state <> 'idle'
  AND application_name <> 'db-migrate-cli'
ORDER BY query_start`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid int32
			var state, query string
			var ageSeconds int64
			if err := rows.Scan(&pid, &state, &query, &ageSeconds); err != nil {
				continue
			}
			age := time.Duration(ageSeconds) * time.Second
			if age > blockingQueryThreshold {
				log.Printf("[WARN] long-running query detected: pid=%d state=%s age=%s query=%s", pid, state, age, trimQuery(query))
			}
		}
	}

	result := PreCheckResult{
		RowCounts: map[string]int64{},
	}
	for _, table := range rowCountTables(input.Component) {
		var count int64
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
			log.Printf("[WARN] unable to count rows for %s: %v", table, err)
			continue
		}
		result.RowCounts[table] = count
		log.Printf("pre_check_rowcount table=%s count=%d", table, count)
	}

	return result, nil
}

// RunPostChecks enforces safety checks after migrations finish.
func RunPostChecks(ctx context.Context, input PostCheckInput) error {
	log.Printf("migration_post_checks component=%s direction=%s version=%s dry_run=%t duration_ms=%d", input.Component, input.Direction, input.TargetVersion, input.DryRun, input.Duration.Milliseconds())
	if !input.DryRun && input.Duration > 10*time.Minute {
		return fmt.Errorf("migration exceeded allowed duration: %s", input.Duration)
	}

	dsn, err := dsnForComponent(input.Component)
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("post-check connect: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("post-check ping: %w", err)
	}

	for _, table := range rowCountTables(input.Component) {
		var count int64
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
			log.Printf("[WARN] unable to count rows for %s after migration: %v", table, err)
			continue
		}
		prev := input.PreCheckResults.RowCounts[table]
		log.Printf("post_check_rowcount table=%s count=%d delta=%d", table, count, count-prev)
		if count < 0 {
			return fmt.Errorf("row count negative for table %s", table)
		}
	}

	return nil
}

// DSNForComponent returns the connection string for the supplied component.
func DSNForComponent(component string) (string, error) {
	return dsnForComponent(component)
}

func dsnForComponent(component string) (string, error) {
	switch component {
	case "operational":
		if dsn := strings.TrimSpace(os.Getenv("DB_URL")); dsn != "" {
			return dsn, nil
		}
	case "analytics":
		if dsn := strings.TrimSpace(os.Getenv("ANALYTICS_URL")); dsn != "" {
			return dsn, nil
		}
	}
	return "", fmt.Errorf("dsn not configured for component %q", component)
}

func trimQuery(query string) string {
	query = strings.TrimSpace(query)
	if len(query) <= 120 {
		return query
	}
	return query[:117] + "..."
}

func rowCountTables(component string) []string {
	if env := strings.TrimSpace(os.Getenv("MIGRATION_ROWCOUNT_TABLES")); env != "" {
		parts := strings.Split(env, ",")
		var tables []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				tables = append(tables, part)
			}
		}
		if len(tables) > 0 {
			return tables
		}
	}

	if component == "analytics" {
		return []string{"usage_events"}
	}
	return []string{"organizations", "users", "api_keys", "model_registry_entries", "usage_events", "audit_log_entries"}
}
