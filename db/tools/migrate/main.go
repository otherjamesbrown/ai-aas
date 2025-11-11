package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/otherjamesbrown/ai-aas/db/tools/migrate/hooks"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type migrateOptions struct {
	Component     string
	Direction     string
	TargetVersion string
	StatusOnly    bool
	DryRun        bool
}

const applicationName = "db-migrate-cli"

func main() {
	opts := parseFlags()

	ctx := context.Background()
	shutdown := initTelemetry(ctx)
	defer shutdown()

	var err error
	switch {
	case opts.StatusOnly:
		err = runStatus(ctx, opts)
	case opts.Direction == "up" || opts.Direction == "down":
		err = runMigrations(ctx, opts)
	default:
		err = fmt.Errorf("unsupported direction %q (expected up or down)", opts.Direction)
	}

	if err != nil {
		log.Fatalf("migration command failed: %v", err)
	}
}

func parseFlags() migrateOptions {
	defaultComponent := getEnvOrDefault("MIGRATION_COMPONENT", "operational")

	var opts migrateOptions
	flag.StringVar(&opts.Component, "component", defaultComponent, "Component to operate on (operational|analytics)")
	flag.StringVar(&opts.Direction, "direction", "up", "Migration direction (up|down)")
	flag.StringVar(&opts.TargetVersion, "version", "", "Optional target version (YYYYMMDDHHMM_slug)")
	flag.BoolVar(&opts.StatusOnly, "status", false, "Report current migration status and exit")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Execute migrations in dry-run mode (no apply/commit)")
	flag.Parse()

	opts.Component = strings.ToLower(strings.TrimSpace(opts.Component))
	opts.Direction = strings.ToLower(strings.TrimSpace(opts.Direction))
	opts.TargetVersion = strings.TrimSpace(opts.TargetVersion)

	return opts
}

func initTelemetry(ctx context.Context) func() {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")

	if endpoint == "" {
		log.Println("telemetry disabled: OTEL_EXPORTER_OTLP_ENDPOINT not set")
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func() {}
	}

	clientOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	if hdrs := parseOTLPHeaders(headers); len(hdrs) > 0 {
		clientOpts = append(clientOpts, otlptracegrpc.WithHeaders(hdrs))
	}

	exporter, err := otlptracegrpc.New(ctx, clientOpts...)
	if err != nil {
		log.Printf("telemetry disabled: failed to initialise exporter: %v", err)
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func() {}
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("db-migrate-cli"),
			attribute.String("service.component", "migrate"),
		),
	)
	if err != nil {
		log.Printf("telemetry resource merge failed: %v", err)
		res = resource.Default()
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("telemetry configured: endpoint=%s headers=%q", endpoint, headers)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			log.Printf("telemetry shutdown error: %v", err)
		} else {
			log.Println("telemetry shutdown complete")
		}
	}
}

func runStatus(ctx context.Context, opts migrateOptions) error {
	tracer := otel.Tracer("github.com/otherjamesbrown/ai-aas/db/tools/migrate")
	ctx, span := tracer.Start(ctx, "migration.status",
		trace.WithAttributes(
			attribute.String("migration.component", opts.Component),
		))
	defer span.End()

	log.Printf("migration_status component=%s", opts.Component)
	return nil // Placeholder: integrate with migrate CLI status command.
}

func runMigrations(ctx context.Context, opts migrateOptions) error {
	if opts.Component != "operational" && opts.Component != "analytics" {
		return fmt.Errorf("unknown component %q", opts.Component)
	}

	if opts.TargetVersion != "" && !strings.Contains(opts.TargetVersion, "_") {
		return errors.New("target version must include timestamp and slug (YYYYMMDDHHMM_slug)")
	}

	tracer := otel.Tracer("github.com/otherjamesbrown/ai-aas/db/tools/migrate")
	ctx, span := tracer.Start(ctx, "migration.run",
		trace.WithAttributes(
			attribute.String("migration.component", opts.Component),
			attribute.String("migration.direction", opts.Direction),
			attribute.String("migration.version", opts.TargetVersion),
			attribute.Bool("migration.dry_run", opts.DryRun),
		))
	defer span.End()

	start := time.Now()
	log.Printf("migration_start component=%s direction=%s version=%s dry_run=%t", opts.Component, opts.Direction, opts.TargetVersion, opts.DryRun)

	preResult, err := hooks.RunPreChecks(ctx, hooks.PreCheckInput{
		Component:     opts.Component,
		Direction:     opts.Direction,
		TargetVersion: opts.TargetVersion,
		DryRun:        opts.DryRun,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "pre-check failed")
		return err
	}

	span.AddEvent("migration_pre_checks_complete")

	if opts.DryRun {
		log.Printf("[INFO] dry-run requested; skipping database migration component=%s", opts.Component)
		span.AddEvent("migration_dry_run_skipped")
	} else {
		if err := applyMigrations(ctx, opts); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "apply failed")
			return err
		}
		span.AddEvent("migration_apply_complete")
	}

	duration := time.Since(start)

	if err := hooks.RunPostChecks(ctx, hooks.PostCheckInput{
		Component:       opts.Component,
		Direction:       opts.Direction,
		TargetVersion:   opts.TargetVersion,
		DryRun:          opts.DryRun,
		Duration:        duration,
		PreCheckResults: preResult,
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "post-check failed")
		return err
	}

	span.SetAttributes(attribute.Int64("migration.duration_ms", duration.Milliseconds()))
	span.SetStatus(codes.Ok, "migration completed")

	if err := hooks.RecordAuditLog(ctx, hooks.AuditLogInput{
		Component:     opts.Component,
		Direction:     opts.Direction,
		TargetVersion: opts.TargetVersion,
		DryRun:        opts.DryRun,
		Duration:      duration,
	}); err != nil {
		span.RecordError(err)
		log.Printf("[WARN] audit log recording failed: %v", err)
	}

	log.Printf("migration_finish component=%s direction=%s version=%s dry_run=%t status=success duration_ms=%d",
		opts.Component, opts.Direction, opts.TargetVersion, opts.DryRun, duration.Milliseconds())
	return nil
}

func applyMigrations(ctx context.Context, opts migrateOptions) error {
	dsn, err := hooks.DSNForComponent(opts.Component)
	if err != nil {
		return err
	}

	dsnWithApp, err := ensureApplicationName(dsn)
	if err != nil {
		return err
	}

	migrationsPath, err := migrationsDir(opts.Component)
	if err != nil {
		return err
	}

	migrations, err := discoverMigrations(migrationsPath)
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		log.Printf("[INFO] no migration files discovered for component=%s", opts.Component)
		return nil
	}

	db, err := sql.Open("pgx", dsnWithApp)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[WARN] closing database connection: %v", err)
		}
	}()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if err := ensureMigrationsTable(ctx, db, opts.Component); err != nil {
		return err
	}

	appliedMap, appliedOrdered, err := loadAppliedMigrations(ctx, db, opts.Component)
	if err != nil {
		return err
	}

	switch opts.Direction {
	case "up":
		return applyUpMigrations(ctx, db, opts, migrations, appliedMap)
	case "down":
		return applyDownMigrations(ctx, db, opts, migrations, appliedMap, appliedOrdered)
	default:
		return fmt.Errorf("unsupported direction %q", opts.Direction)
	}
}

func migrationsDir(component string) (string, error) {
	if component == "" {
		return "", fmt.Errorf("component is required")
	}

	if root := strings.TrimSpace(os.Getenv("MIGRATIONS_ROOT")); root != "" {
		return ensureMigrationsDir(filepath.Join(root, component))
	}

	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locate migrations dir: unable to determine caller")
	}

	candidate := filepath.Join(filepath.Dir(callerFile), "..", "..", "migrations", component)
	return ensureMigrationsDir(candidate)
}

func ensureMigrationsDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("locate migrations dir: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("migrations dir %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("migrations path %s is not a directory", abs)
	}
	return abs, nil
}

func migrationsTableName(component string) string {
	switch component {
	case "analytics":
		return "schema_migrations_analytics"
	default:
		return "schema_migrations_operational"
	}
}

type migrationFile struct {
	Version  uint64
	Slug     string
	UpPath   string
	DownPath string
}

func discoverMigrations(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var migrations []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version, slug, err := parseMigrationFilename(name)
		if err != nil {
			return nil, fmt.Errorf("parse migration filename %s: %w", name, err)
		}
		upPath := filepath.Join(dir, name)
		downCandidate := strings.TrimSuffix(name, ".up.sql") + ".down.sql"
		downPath := filepath.Join(dir, downCandidate)
		if _, err := os.Stat(downPath); err != nil {
			downPath = ""
		}
		migrations = append(migrations, migrationFile{
			Version:  version,
			Slug:     slug,
			UpPath:   upPath,
			DownPath: downPath,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].Version == migrations[j].Version {
			return migrations[i].Slug < migrations[j].Slug
		}
		return migrations[i].Version < migrations[j].Version
	})

	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version == migrations[i-1].Version {
			return nil, fmt.Errorf("duplicate migration version %d (%s and %s)",
				migrations[i].Version, migrations[i-1].UpPath, migrations[i].UpPath)
		}
	}

	return migrations, nil
}

func parseMigrationFilename(filename string) (uint64, string, error) {
	base := filepath.Base(filename)
	if !strings.HasSuffix(base, ".up.sql") {
		return 0, "", fmt.Errorf("expected .up.sql suffix")
	}
	trimmed := strings.TrimSuffix(base, ".up.sql")
	parts := strings.SplitN(trimmed, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("expected format <version>_<slug>.up.sql")
	}
	version, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse version %q: %w", parts[0], err)
	}
	return version, parts[1], nil
}

type appliedMigration struct {
	Slug string
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB, component string) error {
	table := migrationsTableName(component)
	stmt := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	version     BIGINT PRIMARY KEY,
	slug        TEXT NOT NULL,
	applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`, table)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB, component string) (map[uint64]appliedMigration, []uint64, error) {
	table := migrationsTableName(component)
	query := fmt.Sprintf(`SELECT version, slug FROM %s ORDER BY version ASC`, table)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[uint64]appliedMigration)
	var ordered []uint64
	for rows.Next() {
		var version int64
		var slug string
		if err := rows.Scan(&version, &slug); err != nil {
			return nil, nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[uint64(version)] = appliedMigration{Slug: slug}
		ordered = append(ordered, uint64(version))
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}
	return applied, ordered, nil
}

func applyUpMigrations(ctx context.Context, db *sql.DB, opts migrateOptions, migrations []migrationFile, applied map[uint64]appliedMigration) error {
	targetVersion := uint64(math.MaxUint64)
	if opts.TargetVersion != "" {
		version, err := parseMigrationVersion(opts.TargetVersion)
		if err != nil {
			return err
		}
		targetVersion = version
	}

	table := migrationsTableName(opts.Component)
	for _, mig := range migrations {
		if mig.Version > targetVersion {
			break
		}
		if _, already := applied[mig.Version]; already {
			continue
		}
		if err := executeMigration(ctx, db, mig.UpPath, "up", mig); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO %s (version, slug) VALUES ($1, $2) ON CONFLICT (version) DO NOTHING`, table),
			int64(mig.Version), mig.Slug); err != nil {
			return fmt.Errorf("record migration %d: %w", mig.Version, err)
		}
		log.Printf("migration_applied component=%s version=%d slug=%s", opts.Component, mig.Version, mig.Slug)
	}
	return nil
}

func applyDownMigrations(ctx context.Context, db *sql.DB, opts migrateOptions, migrations []migrationFile, applied map[uint64]appliedMigration, appliedOrdered []uint64) error {
	if len(appliedOrdered) == 0 {
		log.Printf("[INFO] no applied migrations to roll back for component=%s", opts.Component)
		return nil
	}

	targetVersion := uint64(0)
	if opts.TargetVersion != "" {
		version, err := parseMigrationVersion(opts.TargetVersion)
		if err != nil {
			return err
		}
		targetVersion = version
	}

	migrationIndex := make(map[uint64]migrationFile, len(migrations))
	for _, mig := range migrations {
		migrationIndex[mig.Version] = mig
	}

	var toRollback []uint64
	if opts.TargetVersion == "" {
		toRollback = append(toRollback, appliedOrdered[len(appliedOrdered)-1])
	} else {
		for i := len(appliedOrdered) - 1; i >= 0; i-- {
			version := appliedOrdered[i]
			if version >= targetVersion {
				toRollback = append(toRollback, version)
			}
		}
		if len(toRollback) == 0 {
			log.Printf("[INFO] no migrations >= %d to roll back for component=%s", targetVersion, opts.Component)
			return nil
		}
	}

	table := migrationsTableName(opts.Component)
	for _, version := range toRollback {
		mig, ok := migrationIndex[version]
		if !ok {
			return fmt.Errorf("down migration not found for version %d", version)
		}
		if mig.DownPath == "" {
			return fmt.Errorf("down migration file missing for version %d (%s)", version, mig.Slug)
		}
		if err := executeMigration(ctx, db, mig.DownPath, "down", mig); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE version = $1`, table),
			int64(version)); err != nil {
			return fmt.Errorf("remove migration %d: %w", version, err)
		}
		log.Printf("migration_rolled_back component=%s version=%d slug=%s", opts.Component, version, mig.Slug)
	}
	return nil
}

func executeMigration(ctx context.Context, db *sql.DB, path, direction string, mig migrationFile) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}
	sqlText := strings.TrimSpace(string(sqlBytes))
	if sqlText == "" {
		log.Printf("[INFO] migration %s (%s) is empty; skipping", path, direction)
		return nil
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction for %s: %w", path, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("execute migration %s: %w", path, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", path, err)
	}
	return nil
}

func ensureApplicationName(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	q := u.Query()
	if q.Get("application_name") == "" {
		q.Set("application_name", applicationName)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func parseMigrationVersion(raw string) (uint64, error) {
	parts := strings.SplitN(raw, "_", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid migration version %q", raw)
	}
	value, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse migration version %q: %w", raw, err)
	}
	return value, nil
}

func getEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseOTLPHeaders(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	headers := map[string]string{}
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return headers
}
