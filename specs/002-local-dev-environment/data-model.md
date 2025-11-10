# Data Model: Local Development Environment

**Branch**: `002-local-dev-environment`  
**Date**: 2025-11-10  
**Spec**: `/specs/002-local-dev-environment/spec.md`

## Entities

### RemoteWorkspace
- **description**: Ephemeral Linode instance hosting the remote development stack with security controls and observability agents.
- **fields**:
  - `workspace_id`: string (tag applied during provisioning, e.g., `dev-jab`)
  - `linode_instance_id`: integer
  - `region`: enum (Linode regions supported by platform)
  - `ttl_hours`: integer (default 24)
  - `private_ip`: string (RFC1918 address)
  - `provisioner`: string (`terraform` module identifier + version)
  - `stackscript_version`: semantic version of bootstrap script
- **relationships**:
  - Owns one **SecretsBundle** (remote scope)
  - Runs one **DependencySuite**
  - Emits events to **ObservabilityStream**
- **rules**:
  - Must attach to private VLAN and logging collector during creation
  - Must be tagged with owner, region, ttl metadata for cleanup automation

### LocalStack
- **description**: Developer machine runtime using Docker Compose to mirror production dependencies.
- **fields**:
  - `compose_file`: path (`dev/compose.local.yaml`)
  - `override_file`: path (`dev/compose.override.yaml` optional)
  - `ports`: map from service to host port (default from `.specify/local/ports.yaml`)
  - `resource_limits`: CPU/RAM recommendations per dependency
- **relationships**:
  - Consumes **SecretsBundle** (local scope)
  - Exposes health via **HealthProbe**
- **rules**:
  - Must support configurable ports for conflict resolution
  - Startup/stop/reset commands map to Make targets (`make up`, `make stop`, `make reset`)

### DependencySuite
- **description**: Collection of containerized dependencies shared by remote and local environments.
- **fields**:
  - `postgres`: configuration (image `postgres:15`, volume mount, init SQL path)
  - `redis`: configuration (image `redis:7`, persistence toggle)
  - `nats`: configuration (image `nats:2.10`, cluster disabled)
  - `minio`: configuration (image `minio/minio:RELEASE`, access/secret keys)
  - `mock_inference`: configuration (image `ghcr.io/ai-aas/mock-inference:<tag>`)
  - `network`: docker network name (`ai-aas-dev-net`)
- **relationships**:
  - Referenced by **RemoteWorkspace** and **LocalStack**
  - Surfaces states via **HealthProbe**
- **rules**:
  - All containers expose readiness endpoints for polling
  - Remote deployment uses systemd units to ensure restart on failure

### SecretsBundle
- **description**: Generated environment files containing connection strings and credentials for both modes.
- **fields**:
  - `path_remote`: `.env.linode`
  - `path_local`: `.env.local`
  - `generated_at`: timestamp
  - `source`: string (Vault namespace + role)
  - `masked_fields`: list of keys that must never print in logs (e.g., `POSTGRES_PASSWORD`)
- **relationships**:
  - Produced by **SecretsSyncCommand**
  - Consumed by services and tooling during startup
- **rules**:
  - Files must be `.gitignore`d and have `0600` permissions
  - Values expire within 24 hours for remote usage

### SecretsSyncCommand
- **description**: Automation that hydrates secrets bundle from Vault using short-lived tokens.
- **fields**:
  - `binary`: path (`cmd/secrets-sync`)
  - `inputs`: Vault address, AppRole ID, Secret ID, workspace/local mode
  - `outputs`: `.env.linode`, `.env.local`
  - `audit_id`: correlation ID logged to observability stream
- **relationships**:
  - Writes **SecretsBundle**
  - Triggered by `make secrets-sync`
- **rules**:
  - Must redact secrets in stdout/stderr
  - Must validate `.gitignore` before writing files

### HealthProbe
- **description**: Go binary that checks dependency health and prints JSON consumed by tooling.
- **fields**:
  - `binary`: path (`cmd/dev-status`)
  - `components`: array of checks (postgres, redis, nats, minio, mock_inference)
  - `thresholds`: map of acceptable response times (≤ 2s each)
  - `output_format`: JSON (component name, state, latency_ms, message)
- **relationships**:
  - Invoked by `make status` and `make remote-status`
  - Publishes results to **ObservabilityStream** optionally
- **rules**:
  - Must exit non-zero when any component reports `unhealthy`
  - Supports `--json` (default) and `--human` output modes

### ObservabilityStream
- **description**: Aggregated logging and metrics destination for both remote and local stacks.
- **fields**:
  - `logs_endpoint`: URL (Loki push API)
  - `metrics_endpoint`: Prometheus pushgateway or OTLP endpoint
  - `workspace_labels`: map (workspace_id, owner, region)
  - `retention_days`: integer (≥ 30 for logs, 7 for debug-level)
- **relationships**:
  - Receives events from **RemoteWorkspace**, **SecretsSyncCommand**, **HealthProbe**
  - Integrates with observability spec dashboards
- **rules**:
  - All events must include correlation ID
  - Local mode optionally batches and uploads metrics when `METRICS=true`

### LifecycleCommand
- **description**: Make targets providing parity between remote and local workflows.
- **fields**:
  - `name`: enum (`remote-up`, `remote-status`, `remote-reset`, `remote-logs`, `remote-destroy`, `up`, `status`, `reset`, `logs`, `stop`)
  - `runner`: `bash` script path under `scripts/dev/`
  - `requires`: prerequisites (e.g., Terraform, Docker, Vault login)
  - `side_effects`: e.g., `remote-up` creates systemd units, `reset` wipes volumes
- **relationships**:
  - Operate on **RemoteWorkspace** or **LocalStack**
  - Call **SecretsSyncCommand** and **HealthProbe**
- **rules**:
  - Must print actionable errors and remediation steps
  - Remote variants log lifecycle events with timestamp and actor metadata

### DeveloperServiceConfig
- **description**: Environment-specific configuration for application services to connect to dev stack.
- **fields**:
  - `service_name`: string
  - `config_template`: path under `configs/dev/<service>.env.tpl`
  - `placeholders`: map of required substitutions (e.g., `${POSTGRES_HOST}`)
  - `mode`: enum (`remote`, `local`)
- **relationships**:
  - Populated using **SecretsBundle** values
  - Referenced by service quickstarts and README instructions
- **rules**:
  - Templates must document toggles between remote/local by referencing shared variables only
  - Configs must avoid embedding secret values directly; rely on `.env.*` injections
