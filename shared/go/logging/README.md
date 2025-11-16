# Logging Package

Unified logging package for all Go services using zap with standardized configuration, OpenTelemetry integration, and log redaction.

## Features

- **Standardized Configuration**: Consistent logger setup across all services
- **OpenTelemetry Integration**: Automatic trace context propagation
- **Structured Logging**: JSON output with standardized field names
- **Log Redaction**: Built-in patterns for masking sensitive data
- **Environment-Aware**: Different encoder configs for development vs production

## Usage

### Basic Usage

```go
import "github.com/ai-aas/shared-go/logging"

// Create logger with config
cfg := logging.Config{
    ServiceName: "my-service",
    Environment: "development",
    LogLevel:    "info",
    OutputPath:  "stdout",
}

logger, err := logging.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer logger.Sync()

// Use logger
logger.Info("service started", zap.String("port", "8080"))
```

### With Default Config

```go
cfg := logging.DefaultConfig().WithServiceName("my-service")
logger := logging.MustNew(cfg)
```

### With OpenTelemetry Context

```go
ctx := context.Background()
loggerWithCtx := logger.WithContext(ctx)
loggerWithCtx.Info("request processed")
```

### With Request/User/Org IDs

```go
logger := logger.WithRequestID("req-123")
logger = logger.WithUserID("user-456")
logger = logger.WithOrgID("org-789")
logger.Info("user action")
```

### Log Redaction

```go
import "github.com/ai-aas/shared-go/logging"

// Redact sensitive strings
safe := logging.RedactString("password=secret123")
// Returns: "password=***REDACTED***"

// Redact sensitive fields in a map
fields := map[string]interface{}{
    "password": "secret",
    "username": "user",
}
safeFields := logging.RedactFields(fields)
```

## Standardized Fields

All loggers automatically include:
- `service`: Service name
- `environment`: Deployment environment
- `timestamp`: ISO8601 formatted timestamp
- `level`: Log level (debug, info, warn, error)
- `caller`: Source file and line number

Optional fields (via helper methods):
- `trace_id`: OpenTelemetry trace ID
- `span_id`: OpenTelemetry span ID
- `request_id`: Request correlation ID
- `user_id`: User identifier
- `org_id`: Organization identifier

## Configuration

### Log Levels

- `debug`: Verbose debugging information
- `info`: General informational messages (default)
- `warn`: Warning messages
- `error`: Error messages

Set via `LOG_LEVEL` environment variable or `Config.LogLevel`.

### Output Paths

- `stdout`: Standard output (default)
- `stderr`: Standard error
- File path: Write to file (e.g., `/var/log/service.log`)

### Environment Detection

The logger automatically detects environment from `ENVIRONMENT` environment variable:
- `development`: Development encoder config (more readable)
- `production`: Production encoder config (optimized JSON)

## Integration with OpenTelemetry

The logger automatically extracts trace context from `context.Context`:

```go
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

logger := logger.WithContext(ctx)
logger.Info("operation completed")
// Logs include trace_id and span_id fields
```

## Redaction Patterns

The redaction package masks:
- Passwords: `password=***REDACTED***`
- Tokens: `Bearer ***REDACTED***`
- Connection strings: `postgres://***REDACTED***@host/db`
- API keys: `X-API-Key: ***REDACTED***`
- Secrets: Environment variables containing "SECRET"

Patterns are based on `configs/log-redaction.yaml`.

## Best Practices

1. **Always use structured fields**: Use `zap.String()`, `zap.Int()`, etc.
2. **Include context**: Use `WithContext()` for request-scoped logs
3. **Redact sensitive data**: Use redaction helpers for user input or config values
4. **Set appropriate levels**: Use `debug` for development, `info` for production
5. **Sync on shutdown**: Call `logger.Sync()` before application exit

## Migration from Service-Specific Loggers

### From zerolog

```go
// Before
import "github.com/rs/zerolog"
logger := zerolog.New(os.Stdout).With().Str("service", "svc").Logger()

// After
import "github.com/ai-aas/shared-go/logging"
cfg := logging.DefaultConfig().WithServiceName("svc")
logger := logging.MustNew(cfg)
```

### From zap (service-specific)

```go
// Before
import "go.uber.org/zap"
logger, _ := zap.NewProduction()

// After
import "github.com/ai-aas/shared-go/logging"
cfg := logging.DefaultConfig().WithServiceName("svc").WithLogLevel("info")
logger := logging.MustNew(cfg)
```

