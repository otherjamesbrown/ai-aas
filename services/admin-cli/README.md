# Admin CLI

Command-line tool for platform administrators to perform privileged operations quickly and safely: bootstrap the first admin, manage organizations and users, rotate credentials, trigger syncs, and export reports.

## Features

- **Bootstrap Operations**: Initialize platform and perform break-glass recovery with dry-run and confirmation support
- **Organization Management**: Create, read, update, delete organizations with batch operations and structured output
- **User Management**: Manage users with idempotent operations and batch processing
- **Credential Rotation**: Rotate API keys and service account tokens securely with audit logging
- **Sync Operations**: Trigger and monitor analytics sync operations with progress indicators
- **Exports**: Export usage and membership reports with reconciliation verification and streaming support

## Prerequisites

- **Go 1.24+** (for building from source)
- **Access to services**: user-org-service and analytics-service APIs must be running and accessible
- **Authentication credentials**: API keys or service account tokens (not required for bootstrap)

## Quick Start

### Build the CLI

```bash
cd services/admin-cli
make build
```

### Run the CLI

```bash
# View help
./bin/admin-cli --help

# Check version
./bin/admin-cli --version

# Bootstrap first admin (dry-run preview)
./bin/admin-cli bootstrap --dry-run --user-org-endpoint=http://localhost:8081

# Bootstrap first admin (execute)
./bin/admin-cli bootstrap --confirm --user-org-endpoint=http://localhost:8081
```

## Usage Examples

### Bootstrap Operations

```bash
# Preview bootstrap (dry-run)
admin-cli bootstrap --dry-run \
  --user-org-endpoint=http://localhost:8081 \
  --email=admin@example.com

# Execute bootstrap
admin-cli bootstrap --confirm \
  --user-org-endpoint=http://localhost:8081 \
  --email=admin@example.com \
  --format=json
```

### Organization Management

```bash
# List organizations
admin-cli org list --format=json

# Create organization
admin-cli org create \
  --name="Acme Corp" \
  --slug="acme-corp" \
  --confirm

# Update organization from file
admin-cli org update --file=orgs.yaml --dry-run
admin-cli org update --file=orgs.yaml --confirm --format=json

# Delete organization (requires confirmation + force)
admin-cli org delete --org=acme-corp --confirm --force
```

### User Management

```bash
# Create user
admin-cli user create \
  --org=acme-corp \
  --email=user@example.com \
  --confirm

# List users
admin-cli user list --org=acme-corp --format=json

# Update user (idempotent with --upsert)
admin-cli user update \
  --org=acme-corp \
  --email=user@example.com \
  --upsert \
  --confirm
```

### Export Operations

```bash
# Export usage report
admin-cli export usage \
  --org=acme-corp \
  --from=2025-01-01 \
  --to=2025-01-31 \
  --format=csv \
  --output=usage-2025-01.csv

# Export memberships with reconciliation
admin-cli export memberships \
  --org=acme-corp \
  --changes-only \
  --output=audit-2025-01-15.csv
```

### Credential Rotation

```bash
# Rotate API key
admin-cli credentials rotate \
  --org=acme-corp \
  --key-id=key-123 \
  --confirm
```

### Sync Operations

```bash
# Trigger analytics sync
admin-cli sync trigger --component=analytics --confirm

# Check sync status
admin-cli sync status --job-id=job-abc123
```

## Configuration

The CLI supports multiple configuration sources with the following precedence (highest to lowest):

1. **Command-line flags** (take precedence)
2. **Environment variables** (e.g., `ADMIN_CLI_USER_ORG_ENDPOINT`)
3. **Configuration file** (`~/.admin-cli/config.yaml`)
4. **Defaults** (localhost endpoints)

### Environment Variables

```bash
export ADMIN_CLI_USER_ORG_ENDPOINT="http://localhost:8081"
export ADMIN_CLI_ANALYTICS_ENDPOINT="http://localhost:8084"
export ADMIN_CLI_API_KEY="your-api-key-here"
export ADMIN_CLI_DEFAULT_OUTPUT_FORMAT="json"  # table, json, csv
```

### Configuration File

Create `~/.admin-cli/config.yaml`:

```yaml
api_endpoints:
  user_org_service: "http://localhost:8081"
  analytics_service: "http://localhost:8084"

defaults:
  output_format: "table"  # table, json, csv
  verbose: false
  quiet: false
  confirm: false          # require explicit --confirm for destructive ops

auth:
  api_key: "${ADMIN_CLI_API_KEY}"  # from environment variable

retry:
  max_attempts: 3
  timeout: 30  # seconds
```

### Command-Line Flags

All configuration can be overridden via flags:

```bash
admin-cli org list \
  --user-org-endpoint=http://custom:8081 \
  --format=json \
  --verbose
```

## Development

### Build and Test

```bash
# Build the CLI binary
make build

# Run unit tests
make test

# Run integration tests (requires services to be running)
make integration-test

# Run full check (fmt, lint, security, test)
make check

# Clean build artifacts
make clean
```

### Available Make Targets

- `make build` - Build CLI binary to `bin/admin-cli`
- `make test` - Run unit tests (excludes integration tests)
- `make integration-test` - Run integration tests (requires running services)
- `make check` - Run fmt, lint, security, and tests
- `make fmt` - Format Go source files
- `make lint` - Run golangci-lint
- `make security` - Run gosec security scan
- `make clean` - Remove build artifacts

## Integration Testing

Integration tests verify the CLI works correctly against running services. They can run against:
- **Local services** (default: `http://localhost:8081` for user-org-service)
- **Deployed services** (via environment variables)
- **CI environments** (tests skip gracefully if services unavailable)

### Running Integration Tests

```bash
# 1. Start required services (in separate terminals)
cd ../user-org-service
make run  # Starts on port 8081

cd ../analytics-service
make run  # Starts on port 8084

# 2. Run integration tests
cd ../admin-cli
make integration-test

# Or with custom endpoints
ADMIN_CLI_USER_ORG_ENDPOINT=http://custom:8081 make integration-test
```

### Integration Test Requirements

- **user-org-service** running and accessible (for bootstrap, org, user tests)
- **analytics-service** running and accessible (for export tests)
- **Database migrated** (for operations that modify data)

Tests will skip gracefully if services are unavailable (useful for CI where services may not be running).

### Integration Test Structure

- `test/integration/bootstrap_test.go` - Bootstrap and health check tests
- `test/integration/org_user_test.go` - Organization and user lifecycle tests
- `test/integration/export_test.go` - Export command tests

See `test/integration/README.md` for detailed integration test documentation.

## Output Formats

The CLI supports multiple output formats:

- **table** (default): Human-readable table format
- **json**: Machine-readable JSON format (suitable for automation/CI)
- **csv**: Comma-separated values (for exports)

```bash
# Table output (human-readable)
admin-cli org list

# JSON output (machine-readable)
admin-cli org list --format=json

# CSV output (for exports)
admin-cli export usage --org=acme --format=csv --output=usage.csv
```

## Exit Codes

The CLI uses predictable exit codes for scriptability:

- `0`: Success
- `1`: General error (operation failed)
- `2`: Usage error (incorrect command/flag usage)
- `3`: Service unavailable (required service is down)

```bash
# Check exit code in scripts
admin-cli org list --format=json
if [ $? -eq 0 ]; then
  echo "Success"
fi
```

## Safety Features

- **Dry-run mode**: Preview changes before executing (`--dry-run`)
- **Confirmation prompts**: Require explicit `--confirm` for destructive operations
- **Health checks**: Verify service availability before operations
- **Audit logging**: All privileged operations emit structured audit logs
- **Credential masking**: Credentials masked in logs and error messages
- **Non-interactive mode**: All prompts via flags (suitable for CI/CD)

## Performance Targets

- **Bootstrap**: ≤2 minutes
- **Single org/user operation**: ≤5 seconds
- **Batch operations**: 100 items in ≤2 minutes
- **Exports**: ≤1 minute per 10k rows
- **Help command**: <1 second response time

## Troubleshooting

### Service Unavailable

```bash
# Check service health manually
curl http://localhost:8081/healthz  # user-org-service
curl http://localhost:8084/analytics/v1/status/healthz  # analytics-service

# Verify endpoint configuration
admin-cli org list --user-org-endpoint=http://localhost:8081 --verbose
```

### Authentication Failures

```bash
# Verify API key is set
echo $ADMIN_CLI_API_KEY

# Test with explicit API key
admin-cli org list --api-key="${ADMIN_CLI_API_KEY}" --verbose
```

### Command Not Found

```bash
# Ensure binary is built
make build

# Verify binary exists
ls -la bin/admin-cli

# Add to PATH or use full path
./bin/admin-cli --help
```

## Documentation

- **Specification**: [`specs/009-admin-cli/spec.md`](../../specs/009-admin-cli/spec.md) - Complete feature specification
- **Quickstart Guide**: [`specs/009-admin-cli/quickstart.md`](../../specs/009-admin-cli/quickstart.md) - Step-by-step usage guide
- **Implementation Plan**: [`specs/009-admin-cli/plan.md`](../../specs/009-admin-cli/plan.md) - Technical implementation details
- **Tasks**: [`specs/009-admin-cli/tasks.md`](../../specs/009-admin-cli/tasks.md) - Implementation task breakdown
- **Integration Tests**: [`test/integration/README.md`](test/integration/README.md) - Integration test documentation

## Contributing

When contributing to the admin-cli:

1. Follow Go code standards (use `make fmt`, `make lint`)
2. Add tests for new features (unit tests co-located with code, integration tests in `test/integration/`)
3. Update documentation if behavior changes
4. Run `make check` before committing
5. Reference spec/task IDs in commit messages

See [Contributing Guide](../../CONTRIBUTING.md) for more details.

## License

Part of the AI-AAS platform. See project root for license information.
