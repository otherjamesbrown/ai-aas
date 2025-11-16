# Shared Libraries Overview

This directory contains the polyglot building blocks that power service bootstrapping across the platform.

- `shared/go` – Go modules for configuration loading, observability, data access helpers, standardized error handling, and authorization middleware.
- `shared/ts` – TypeScript packages providing the equivalent functionality for Node.js services.

## Quick Start

Refer to the quickstart (`specs/004-shared-libraries/quickstart.md`) for environment setup. In short:

```go
// Go example
cfg := config.MustLoad(ctx)
obs := observability.MustInit(ctx, observability.Config{ServiceName: cfg.Service.Name, Endpoint: cfg.Telemetry.Endpoint})
defer obs.Shutdown(ctx)

policyEngine, _ := auth.LoadPolicyFromFile("policies/service-template/policy.json")
router := chi.NewRouter()
router.Use(observability.RequestContextMiddleware)
router.With(auth.Middleware(policyEngine, auth.HeaderExtractor)).Get("/secure/data", handler)
```

```ts
// TypeScript example
const config = loadConfig();
const telemetry = await startTelemetry({ ...config.telemetry, serviceName: config.service.name });
const policy = await PolicyEngine.fromFile('policies/service-template/policy.json');
const app = Fastify();
app.addHook('onRequest', createRequestContextHook());
app.addHook('preHandler', createAuthMiddleware(policy));
```

## API Reference

### Go Libraries (`shared/go`)

#### Configuration (`github.com/ai-aas/shared-go/config`)

Load service configuration from environment variables with validation.

**Environment Variables:**
- `SERVICE_NAME` - Service identifier (required)
- `SERVICE_ADDRESS` - Bind address (default: `:8080`)
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OpenTelemetry endpoint (default: `localhost:4317`)
- `OTEL_EXPORTER_OTLP_PROTOCOL` - Protocol: `grpc` or `http` (default: `grpc`)
- `OTEL_EXPORTER_OTLP_HEADERS` - Comma-separated `key=value` headers
- `OTEL_EXPORTER_OTLP_INSECURE` - Disable TLS (default: `false`)
- `DATABASE_DSN` - Database connection string
- `DATABASE_MAX_IDLE_CONNS` - Connection pool idle size (default: `2`)
- `DATABASE_MAX_OPEN_CONNS` - Connection pool max size (default: `10`)
- `DATABASE_CONN_MAX_LIFETIME` - Connection lifetime (default: `5m`)

**Functions:**
- `Load(ctx context.Context) (Config, error)` - Load configuration with validation
- `MustLoad(ctx context.Context) Config` - Load configuration, panics on error

#### Error Handling (`github.com/ai-aas/shared-go/errors`)

Standardized error types conforming to platform error schema.

**Types:**
- `Error` - Structured error with code, message, detail, request/trace IDs, actor, and timestamp
- `Actor` - Authenticated subject metadata (subject, roles)

**Functions:**
- `New(code, message string, opts ...Option) *Error` - Create a new error
- `From(err error) *Error` - Coerce any error to a shared Error
- `Marshal(err error) ([]byte, error)` - Serialize error to JSON
- Options: `WithDetail()`, `WithRequestID()`, `WithTraceID()`, `WithActor()`, `WithTimestamp()`

**Error Codes:**
- Standard codes: `INTERNAL`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `VALIDATION_ERROR`

#### Observability (`github.com/ai-aas/shared-go/observability`)

OpenTelemetry integration with automatic fallback and graceful degradation.

**Functions:**
- `Init(ctx context.Context, cfg Config) (*Provider, error)` - Initialize OpenTelemetry
- `MustInit(ctx context.Context, cfg Config) *Provider` - Initialize, panics on error
- `RequestContextMiddleware` - HTTP middleware for request/trace ID injection
- `Provider.Shutdown(ctx context.Context) error` - Flush and shutdown
- `Provider.Fallback() bool` - Check if operating in degraded mode

**Features:**
- Automatic gRPC → HTTP fallback on connection failure
- Degraded mode when all exporters fail (no-op tracer)
- Request ID and trace ID propagation via middleware
- Exporter failure metrics (`shared_telemetry_export_failures_total`)

#### Authorization (`github.com/ai-aas/shared-go/auth`)

OPA-based authorization middleware with policy evaluation.

**Functions:**
- `LoadPolicyFromFile(path string) (*Engine, error)` - Load OPA policy bundle
- `Middleware(engine *Engine, extractor Extractor) func(http.Handler) http.Handler` - HTTP authorization middleware
- `HeaderExtractor(r *http.Request) Actor` - Extract actor from `X-Actor-*` headers
- `ActorFromContext(ctx context.Context) (Actor, bool)` - Extract actor from request context

**Policy Format:**
```rego
package authz

default allow = false

allow {
    input.action == "GET:/api/data"
    input.roles[_] == "reader"
}
```

#### Data Access (`github.com/ai-aas/shared-go/dataaccess`)

Database connection helpers with health probes.

**Functions:**
- `NewPool(cfg DatabaseConfig) (*sql.DB, error)` - Create connection pool
- `HealthProbe(db *sql.DB) Probe` - Create health check function
- `Probe(ctx context.Context) (Status, error)` - Execute health check

### TypeScript Libraries (`shared/ts`)

Install via workspace or local path:
```bash
npm install @ai-aas/shared
# or
pnpm add @ai-aas/shared
```

#### Configuration (`@ai-aas/shared/config`)

Load service configuration from environment variables.

**Environment Variables:** Same as Go configuration (see above).

**Functions:**
- `loadConfig(): SharedConfig` - Load configuration with validation
- `mustLoadConfig(): SharedConfig` - Load configuration, throws on error

#### Error Handling (`@ai-aas/shared/errors`)

Standardized error types matching Go implementation.

**Types:**
- `SharedError` - Structured error class
- `ErrorResponse` - JSON error payload
- `Actor` - Authenticated subject metadata

**Functions:**
- `new SharedError(code: string, message: string, options?: ErrorOptions)` - Create error
- `toSharedError(err: unknown): SharedError` - Coerce to shared error
- `toErrorResponse(err: SharedError): ErrorResponse` - Serialize to JSON

#### Observability (`@ai-aas/shared/observability`)

OpenTelemetry integration for Node.js services.

**Functions:**
- `startTelemetry(config: TelemetryConfig): Promise<void>` - Initialize OpenTelemetry
- `stopTelemetry(): Promise<void>` - Shutdown and flush
- `createRequestContextHook()` - Fastify/Express hook for request IDs

**Features:**
- Automatic gRPC → HTTP fallback
- Degraded mode support
- Request/trace ID middleware

#### Authorization (`@ai-aas/shared/auth`)

OPA-based authorization middleware.

**Functions:**
- `PolicyEngine.fromFile(path: string): Promise<PolicyEngine>` - Load policy bundle
- `createAuthMiddleware(engine: PolicyEngine): Middleware` - HTTP authorization middleware
- `recordAudit(event: AuditEvent): void` - Emit audit events

#### Data Access (`@ai-aas/shared/dataaccess`)

PostgreSQL connection pooling and health checks.

**Functions:**
- `initDataAccess(config: DatabaseConfig): DataAccess` - Create connection pool
- `databaseProbe(pool: Pool): Probe` - Create health check function

## Release Process

1. Run `make shared-check` and `scripts/shared/upgrade-verify.sh`.
2. Follow the [shared upgrade checklist](../docs/upgrades/shared-libraries.md).
3. Trigger the GitHub workflow `.github/workflows/shared-libraries-release.yml` with the desired semver tag.

## Testing Matrix

- Unit tests: `shared/go`, `shared/ts`, and `tests/ts/unit`
- Contract tests: `tests/go/contract`, `tests/ts/contract` (verify error/telemetry schema compliance)
- Integration tests: `tests/go/integration`, `tests/ts/integration`
- Sample services: `samples/service-template/go`, `samples/service-template/ts`
- Performance benchmarks: `tests/go/perf`, `tests/ts/perf` (see `docs/perf/shared-libraries.md`)

## Upgrade & Compatibility

- See [upgrade checklist](../docs/upgrades/shared-libraries.md) for version upgrade procedures
- Run `scripts/shared/upgrade-verify.sh` to validate compatibility
- Contract tests ensure backward-compatible error/telemetry schemas

## Support

- Quickstart: `specs/004-shared-libraries/quickstart.md`
- Runbook: `docs/runbooks/shared-libraries.md`
- Troubleshooting: `docs/runbooks/shared-libraries.md#troubleshooting`

