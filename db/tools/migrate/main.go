package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/otherjamesbrown/ai-aas/db/tools/migrate/hooks"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.24.0"
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

	// Future work: invoke golang-migrate library with telemetry hooks.
	if opts.TargetVersion != "" && !strings.Contains(opts.TargetVersion, "_") {
		return errors.New("target version must include timestamp and slug (YYYYMMDDHHMM_slug)")
	}

	time.Sleep(100 * time.Millisecond) // Placeholder to simulate work without blocking
	span.AddEvent("migration_apply_complete")

	if err := hooks.RunPostChecks(ctx, hooks.PostCheckInput{
		Component:       opts.Component,
		Direction:       opts.Direction,
		TargetVersion:   opts.TargetVersion,
		DryRun:          opts.DryRun,
		Duration:        time.Since(start),
		PreCheckResults: preResult,
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "post-check failed")
		return err
	}

	duration := time.Since(start)
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
