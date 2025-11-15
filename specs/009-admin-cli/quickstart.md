# Quickstart: Admin CLI

**Branch**: `009-admin-cli`  
**Date**: 2025-11-08  
**Audience**: Platform administrators and operators

---

## 1. Prerequisites

1. **Confirm platform context**
   - Admin CLI consumes existing service APIs (user-org-service, analytics-service)
   - Services must be running and accessible (locally or remotely)
   - CLI requires authentication credentials (API keys, service account tokens, or OAuth2 tokens)

2. **Install required tooling** (macOS/Linux/WSL2):
   ```bash
   ./scripts/setup/bootstrap.sh --check-only
   ```
   - Verifies Go ≥ 1.24, Git, `make`
   - CLI will be distributed as a single Go binary (no package manager dependencies)

3. **Verify service dependencies**:
   - **user-org-service**: Required for org/user/key lifecycle operations and bootstrap
   - **analytics-service**: Required for usage/membership exports
   - Check service health endpoints:
     ```bash
     curl http://localhost:8081/health  # user-org-service
     curl http://localhost:8084/analytics/v1/status/healthz  # analytics-service
     ```

4. **Obtain authentication credentials**:
   - For bootstrap: No authentication required (creates first admin)
   - For day-2 operations: API key or service account token from existing admin
   - Store credentials securely (environment variables or config file)

## 2. One-time Setup

```bash
cd services/admin-cli
make build                    # Build CLI binary
make install                  # Install to $HOME/.local/bin (or system PATH)
admin-cli --version           # Verify installation
admin-cli --help              # View available commands
```

Expected outcome:
- CLI binary built successfully
- Binary installed in PATH (e.g., `~/.local/bin/admin-cli`)
- Help command responds in under 1 second
- Version information displayed

## 3. Bootstrap First Admin

**Scenario**: Setting up a new platform instance or recovering from access loss.

```bash
# Preview bootstrap operation (dry-run)
admin-cli bootstrap --dry-run \
  --api-endpoint=http://localhost:8081 \
  --email=admin@example.com

# Execute bootstrap (requires confirmation)
admin-cli bootstrap --confirm \
  --api-endpoint=http://localhost:8081 \
  --email=admin@example.com \
  --output-format=json
```

Expected outcome:
- First admin account created
- Credentials displayed securely (masked in logs)
- Audit log entry created with operation details
- Operation completes in ≤2 minutes

**Next steps**: Save credentials securely, configure CLI with API key for subsequent operations.

## 4. Daily Operations

### Configure CLI

Create config file (`~/.admin-cli/config.yaml`):
```yaml
api_endpoints:
  user_org_service: "http://localhost:8081"
  analytics_service: "http://localhost:8084"
defaults:
  output_format: "table"  # or "json" for automation
  confirm: false          # require explicit --confirm for destructive ops
auth:
  api_key: "${ADMIN_CLI_API_KEY}"  # from environment variable
```

Or use environment variables:
```bash
export ADMIN_CLI_USER_ORG_ENDPOINT="http://localhost:8081"
export ADMIN_CLI_ANALYTICS_ENDPOINT="http://localhost:8084"
export ADMIN_CLI_API_KEY="your-api-key-here"
```

### Organization Management

```bash
# List all organizations
admin-cli org list --format=json

# Create organization (dry-run first)
admin-cli org create --name="acme-corp" --dry-run
admin-cli org create --name="acme-corp" --confirm

# Update organization from file
admin-cli org update --file=orgs.yaml --dry-run
admin-cli org update --file=orgs.yaml --confirm --format=json

# Delete organization (requires confirmation + force flag)
admin-cli org delete --org=acme-corp --confirm --force
```

### User Management

```bash
# Create user
admin-cli user create \
  --org=acme-corp \
  --email=user@example.com \
  --role=member \
  --confirm

# List users in organization
admin-cli user list --org=acme-corp --format=json

# Update user (idempotent with --upsert)
admin-cli user update \
  --org=acme-corp \
  --email=user@example.com \
  --role=admin \
  --upsert \
  --confirm
```

### Batch Operations

```bash
# Process batch from file
admin-cli org update --file=batch-orgs.yaml --dry-run
admin-cli org update --file=batch-orgs.yaml --confirm \
  --continue-on-error \
  --format=json \
  --output=batch-results.json

# Resume from checkpoint (if interrupted)
admin-cli org update --file=batch-orgs.yaml \
  --resume-from=checkpoint-2025-01-15.json \
  --confirm
```

### Credential Rotation

```bash
# Rotate API key
admin-cli credentials rotate \
  --org=acme-corp \
  --key-id=key-123 \
  --confirm

# Break-glass recovery (requires special authentication)
admin-cli recovery --break-glass \
  --recovery-token="${RECOVERY_TOKEN}" \
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

# Export membership changes (audit trail)
admin-cli export memberships \
  --org=acme-corp \
  --changes-only \
  --output=audit-2025-01-15.csv

# Verify export reconciliation
admin-cli export usage \
  --org=acme-corp \
  --from=2025-01-01 \
  --to=2025-01-31 \
  --format=csv \
  --verify-reconciliation \
  --tolerance=0.01  # 1% tolerance
```

### Sync Operations

```bash
# Trigger analytics sync
admin-cli sync trigger \
  --component=analytics \
  --confirm

# Check sync status
admin-cli sync status --job-id=job-abc123

# Monitor long-running sync
admin-cli sync status --job-id=job-abc123 --watch
```

## 5. Automation & Scripting

### CI/CD Integration

```bash
#!/bin/bash
set -euo pipefail

# Use JSON output for parsing
ADMIN_CLI_API_KEY="${ADMIN_CLI_API_KEY}"
export ADMIN_CLI_USER_ORG_ENDPOINT="${USER_ORG_SERVICE_URL}"

# Non-interactive mode (all flags provided)
result=$(admin-cli org update \
  --file=orgs.yaml \
  --confirm \
  --format=json \
  --quiet)

# Parse JSON output
orgs_created=$(echo "$result" | jq '.summary.created')
orgs_updated=$(echo "$result" | jq '.summary.updated')

# Exit codes: 0=success, 1=error, 2=usage, 3=service unavailable
if [ $? -eq 0 ]; then
  echo "✓ Batch operation complete: $orgs_created created, $orgs_updated updated"
else
  echo "✗ Batch operation failed"
  exit 1
fi
```

### Monitoring Integration

```bash
# Emit structured logs for log aggregation
admin-cli org list \
  --format=json \
  --verbose \
  | jq -c '. | {timestamp: now, operation: "org_list", result: .}'

# Progress events for long-running operations
admin-cli org update --file=large-batch.yaml --confirm \
  --format=json \
  --progress 2>&1 | while read line; do
    echo "$line" | jq -c '.progress' >> /var/log/admin-cli/progress.log
  done
```

## 6. Metrics & Observability

- **Audit Logs**: All privileged operations are logged with user identity, timestamp, command, parameters (masked), and outcomes. Logs are emitted to standard output (structured JSON format) and can be captured by log aggregation systems.
- **Progress Events**: Long-running operations (>30 seconds) emit progress events that can be consumed by monitoring systems.
- **Operation Duration**: All operations track duration and include it in audit logs for performance analysis.

View audit logs:
```bash
# CLI operations emit structured logs to stdout
admin-cli org create --name=test --confirm 2>&1 | \
  jq 'select(.audit_log != null) | .audit_log'

# Log aggregation example (Loki/Elasticsearch)
# Configure log forwarding to capture CLI output
```

## 7. Troubleshooting

| Scenario | Resolution |
|----------|------------|
| Service unavailable | Run health checks: `curl $USER_ORG_SERVICE_URL/health`. CLI performs upfront health checks and fails fast with clear error messages. |
| Authentication failed | Verify `ADMIN_CLI_API_KEY` is set correctly. Check token expiration. CLI validates tokens before operations. |
| Batch operation partially fails | Use `--continue-on-error` flag for resilient processing. Check `batch-results.json` for failure details. Resume from checkpoint if needed. |
| Export reconciliation fails | Check tolerance (default 1%). Verify time range matches between export and service counters. Use `--verify-reconciliation` flag. |
| Help command slow | Verify CLI binary is in PATH and not via symlink. Target: <1 second response time. |
| Concurrent operations conflict | API-level optimistic locking prevents corruption. CLI displays clear conflict errors with resolution suggestions. |
| Large export timeout | Use streaming output (`--stream`), compression (`--compress=gzip`), or split exports by time range. |

## 8. Performance Benchmarks

**Target Performance** (from spec):
- Bootstrap: ≤2 minutes
- Single org/user operation: ≤5 seconds
- Batch operations: 100 items in ≤2 minutes
- Exports: ≤1 minute per 10k rows
- Help command: <1 second

**Measure CLI Performance**:
```bash
# Time bootstrap operation
time admin-cli bootstrap --confirm --api-endpoint=$URL --email=test@example.com

# Time batch operation
time admin-cli org update --file=100-orgs.yaml --confirm --format=json

# Time export operation
time admin-cli export usage --org=acme --from=2025-01-01 --to=2025-01-31 --format=csv
```

---

**Next Steps**: After following this quickstart, administrators can review `/specs/009-admin-cli/spec.md` for complete requirements and `/specs/009-admin-cli/tasks.md` for implementation work items.

