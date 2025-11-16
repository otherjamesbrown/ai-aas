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
  - `source`: string (GitHub repository environment + secret prefix)
  - `masked_fields`: list of keys that must never print in logs (e.g., `POSTGRES_PASSWORD`)
- **relationships**:
  - Produced by **SecretsSyncCommand**
  - Consumed by services and tooling during startup
- **rules**:
  - Files must be `.gitignore`d and have `0600` permissions
  - Values expire within 24 hours for remote usage

### SecretsSyncCommand
- **description**: Automation that hydrates secrets bundle from GitHub repository environment secrets using `gh api`.
- **fields**:
  - `binary`: path (`cmd/secrets-sync`)
  - `inputs`: GitHub owner/repo, environment name, personal access token with `actions:read`, workspace/local mode
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

### EnvironmentProfile
- **description**: Centralized configuration profile defining component locations, connection strings, and environment-specific settings for a target environment (local-dev, remote-dev, production).
- **fields**:
  - `name`: string (e.g., `local-dev`, `remote-dev`, `production`)
  - `description`: string
  - `extends`: string (optional base profile name for common defaults)
  - `components`: map of component configurations (postgres, redis, user_org_service, api_router_service, web_portal, etc.)
    - Each component defines: `host`, `port`, `protocol`, `database` (if applicable), `connection_string_template`, `endpoints`
  - `environment_variables`: list of environment variable definitions (name, value, component reference, from_secret)
  - `secrets`: list of secret references (name, source: `env_file`|`vault`|`github`, path/file, expires_at)
- **relationships**:
  - Referenced by **DeveloperServiceConfig**
  - Produced by **EnvironmentProfileManager**
  - Consumed by services during startup via generated `.env.*` files
- **rules**:
  - Must validate component dependencies before activation
  - Secrets must reference secure sources (env files, Vault, GitHub secrets) never hardcode values
  - Profiles can extend base profiles for common defaults (e.g., `local-dev` extends `base`)
  - Component configurations must align with **ComponentRegistry** definitions

### ComponentRegistry
- **description**: Centralized registry tracking all platform components, their ports, dependencies, and configurations across environments.
- **fields**:
  - `components`: list of component definitions
    - `name`: string (e.g., `postgres`, `redis`, `user_org_service`, `api_router_service`, `web_portal`)
    - `description`: string
    - `ports`: list of port definitions (port number, protocol: `tcp`|`http`|`https`, description)
    - `dependencies`: list of required component names (e.g., `user_org_service` depends on `postgres`, `redis`)
    - `environment_variables`: list of required environment variable names (e.g., `DATABASE_URL`, `REDIS_ADDR`)
- **relationships**:
  - Referenced by **EnvironmentProfile** for validation
  - Used by **HealthProbe** for component discovery and status checking
  - Validated by **EnvironmentProfileManager** during profile validation
- **rules**:
  - Must be version-controlled and updated when new components are added
  - Components must define their dependencies for validation and startup ordering
  - Port definitions must specify protocol and include documentation

### EnvironmentProfileManager
- **description**: Tooling for managing environment profiles, activation, validation, and configuration generation.
- **fields**:
  - `binary`: path (`configs/manage-env.sh` or `cmd/env-manager/main.go`)
  - `commands`: list of commands (`activate`, `show`, `validate`, `diff`, `export`, `component-status`, `generate-env-file`)
  - `current_profile`: path to file tracking active environment (`.current-env`)
  - `profile_directory`: path to environment profiles (`configs/environments/`)
- **relationships**:
  - Manages **EnvironmentProfile** entities (read, validate, activate)
  - Generates **SecretsBundle** files (`.env.*`) from active profile
  - References **ComponentRegistry** for validation
- **rules**:
  - Must validate profile YAML syntax and schema before activation
  - Must validate component dependencies and secret availability
  - Must preserve existing secrets during profile switches
  - Must generate `.env.*` files with correct permissions (0600) and ensure `.gitignore` protection
  - Commands must complete within SLAs (validation < 5s, activation < 2s)

### DeveloperServiceConfig
- **description**: Environment-specific configuration for application services to connect to dev stack.
- **fields**:
  - `service_name`: string
  - `config_template`: path under `configs/dev/<service>.env.tpl`
  - `environment_profile`: string (references **EnvironmentProfile** name, e.g., `local-dev`, `remote-dev`)
  - `placeholders`: map of required substitutions (e.g., `${POSTGRES_HOST}`, `${DATABASE_URL}`)
  - `mode`: enum (`local-dev`, `remote-dev`, `production`)
- **relationships**:
  - Populated using **EnvironmentProfile** values (component configs, environment variables)
  - References **SecretsBundle** for secret injection
  - Referenced by service quickstarts and README instructions
- **rules**:
  - Templates must document environment profile usage and reference profile variables
  - Configs must reference environment profiles, not hardcode values
  - Profile activation generates correct service configuration automatically via template substitution
  - Placeholders must map to **EnvironmentProfile** component definitions or environment variables
