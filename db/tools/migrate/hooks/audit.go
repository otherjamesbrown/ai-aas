package hooks

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/jackc/pgx/v5"
)

// AuditLogInput captures metadata needed to record an audit trail entry.
type AuditLogInput struct {
    Component     string
    Direction     string
    TargetVersion string
    DryRun        bool
    Duration      time.Duration
}

// RecordAuditLog writes a migration audit entry if the run was not a dry run.
func RecordAuditLog(ctx context.Context, input AuditLogInput) error {
    if input.DryRun {
        log.Printf("audit_log skipped (dry run) component=%s direction=%s version=%s", input.Component, input.Direction, input.TargetVersion)
        return nil
    }

    dsn, err := dsnForComponent(input.Component)
    if err != nil {
        return err
    }

    conn, err := pgx.Connect(ctx, dsn)
    if err != nil {
        return fmt.Errorf("audit connect: %w", err)
    }
    defer conn.Close(ctx)

    metadata := map[string]any{
        "duration_ms":    input.Duration.Milliseconds(),
        "component":      input.Component,
        "target_version": input.TargetVersion,
        "direction":      input.Direction,
    }
    if approved := os.Getenv("MIGRATION_APPROVED_BY"); approved != "" {
        metadata["approved_by"] = approved
    }
    payload, err := json.Marshal(metadata)
    if err != nil {
        return fmt.Errorf("audit marshal: %w", err)
    }

    actorID := defaultValue(os.Getenv("MIGRATION_ACTOR"), os.Getenv("USER"))
    if actorID == "" {
        actorID = "migration-cli"
    }

    action := fmt.Sprintf("migration_%s", input.Direction)
    target := fmt.Sprintf("%s/%s", input.Component, defaultValue(input.TargetVersion, "latest"))

    _, err = conn.Exec(ctx, `
INSERT INTO audit_log_entries (actor_type, actor_id, action, target, metadata)
VALUES ($1, $2, $3, $4, $5)
`,
        "migration",
        actorID,
        action,
        target,
        string(payload),
    )
    if err != nil {
        return fmt.Errorf("audit insert: %w", err)
    }

    log.Printf("audit_log recorded component=%s direction=%s version=%s actor=%s", input.Component, input.Direction, input.TargetVersion, actorID)
    return nil
}

func defaultValue(value, fallback string) string {
    if value == "" {
        return fallback
    }
    return value
}
