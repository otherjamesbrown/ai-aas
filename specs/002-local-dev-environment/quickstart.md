# Quickstart: Local Development Environment

**Branch**: `002-local-dev-environment`  
**Date**: 2025-11-10  
**Audience**: Platform engineers and service developers with corporate-managed devices

---

## 1. Prerequisites

### Remote Workspace (default path)
- Akamai Linode account access with Terraform + StackScript permissions
- Corporate SSO credentials with MFA (enforced via bastion tunnel)
- `linode-cli`, `terraform`, `make`, `ssh`, and `gh` installed (see `configs/tool-versions.mk` for versions)
- Access to approved secret store (1Password, Vault, or equivalent) for environment hydration

### Local Fallback (conditional)
- macOS 13+/Linux with Docker Desktop or containerd (WSL2 supported)
- Hardware: 8 vCPU (logical), 16 GB RAM, 100 GB free SSD
- Ports 5432, 6379, 4222, 8080 available (or configurable alternatives)
- Corporate policy allowing hypervisor / container runtime; otherwise use remote workspace

Check tooling quickly:
```bash
make doctor  # validates terraform, linode-cli, docker, kubectl, etc.
```

## 2. Provision Remote Workspace (Primary Workflow)

1. Authenticate and prepare context:
   ```bash
   gh auth status || gh auth login
   linode-cli account view >/dev/null  # validates token
   terraform -chdir=infrastructure/plans/dev init -upgrade
   ```
2. Provision ephemeral workspace:
   ```bash
   make remote-provision \
     WORKSPACE=<your-initials>-dev \
     REGION=us-east \
     TTL_HOURS=24
   ```
   - Outputs Linode instance ID, private IP, and audit log link
   - Automatic tags: `env:dev`, `owner:<initials>`, `ttl:24h`
3. Activate environment profile:
   ```bash
   make env-activate ENVIRONMENT=remote-dev
   ```
   - Sets active environment profile and generates environment-specific configuration
   - Validates profile configuration and reports any issues
4. Hydrate environment secrets:
   ```bash
   gh auth login --scopes "repo, read:actions"
   make secrets-sync WORKSPACE=<your-initials>-dev
   ```
   - Generates `.env.linode` and `.env.local` (gitignored) with masked credentials
   - Secrets are merged with active environment profile configuration
5. Connect via secure tunnel:
   ```bash
   linode ssh bastion --workspace <your-initials>-dev
   ```
   - MFA + session recording enforced
   - Idle timeout defaults to 30 minutes
6. Bootstrap remote stack:
   ```bash
   make remote-up WORKSPACE=<your-initials>-dev
   ```
   - Installs system packages, pulls container images, and starts dependencies as systemd services
7. Verify health:
   ```bash
   make remote-status --json | jq '.components[] | {name, state, message}'
   ```
   - Expected states: `healthy`
   - Raw JSON forwarded to centralized logging automatically
8. Develop:
   ```bash
   make remote-shell WORKSPACE=<your-initials>-dev  # interactive shell with env loaded
   devcontainer open --workspace <your-initials>-dev     # optional VS Code/Cursor remote attach
   ```
9. Tear down (manual or wait for TTL automation):
   ```bash
   make remote-destroy WORKSPACE=<your-initials>-dev
   ```
   - Publishes teardown event to audit log stream

## 3. Local Stack Fallback (When Remote Path Unavailable)

1. Ensure Docker is running and ports are free:
   ```bash
   make doctor LOCAL_ONLY=true
   ```
2. Activate local development environment profile:
   ```bash
   make env-activate ENVIRONMENT=local-dev
   ```
   - Sets local development configuration (localhost, default ports)
   - Validates configuration and reports any conflicts
3. Hydrate local env file (reuses secret store):
   ```bash
   make secrets-sync MODE=local
   ```
   - Generates `.env.local` with secrets merged into active profile configuration
4. Start dependencies:
   ```bash
   make up
   ```
5. Check status parity:
   ```bash
   make status --json
   ```
6. Point services at local stack:
   ```bash
   # Environment variables are automatically loaded from active profile
   # No need to manually export - profile manager handles this
   make run SERVICE=user-org-service
   ```
7. Reset or stop as needed:
   ```bash
   make reset  # cleans volumes, reseeds data
   make stop   # stops all containers
   ```

## 4. Environment Profile Management

Environment profiles centralize configuration management across environments. Each profile defines component locations, ports, connection strings, and environment-specific settings.

### Available Environments

- **local-dev**: Local development on your machine (localhost, default dev ports)
- **remote-dev**: Remote Linode workspace (private networking, Linode endpoints)
- **production**: Production environment (production hosts, SSL required, Vault secrets)

### Environment Profile Commands

```bash
# Activate an environment profile
make env-activate ENVIRONMENT=local-dev

# Show current environment configuration
make env-show

# Show configuration for a specific component
make env-show COMPONENT=user_org_service

# Validate current environment profile
make env-validate

# Compare two environment profiles
make env-diff ENV1=local-dev ENV2=remote-dev

# Export environment variables
make env-export FORMAT=env > .env.export

# Check status of all components in current environment
make env-component-status

# Generate .env file from active profile
make env-generate-env-file
```

### Switching Environments

To switch between environments:

```bash
# Switch to remote development
make env-activate ENVIRONMENT=remote-dev
make secrets-sync  # Refresh secrets for remote environment

# Switch back to local
make env-activate ENVIRONMENT=local-dev
make secrets-sync MODE=local  # Refresh secrets for local environment

# Switch to production (requires credentials)
make env-activate ENVIRONMENT=production
make secrets-sync  # Sync from Vault or production secrets store
```

### Profile Configuration

Environment profiles are stored in `configs/environments/*.yaml`:

- `base.yaml`: Common defaults shared across all environments
- `local-dev.yaml`: Local development configuration (extends base)
- `remote-dev.yaml`: Remote workspace configuration (extends base)
- `production.yaml`: Production configuration (extends base)

Profiles define:
- **Components**: Service locations (hosts, ports), connection strings, endpoints
- **Environment Variables**: Variable definitions with component references
- **Secrets**: Secret references (source, path) - never hardcoded values

See `docs/platform/component-registry.md` for complete component documentation.

## 5. Lifecycle Command Reference

| Scenario | Remote Command | Local Command |
|----------|----------------|---------------|
| Start stack | `make remote-up` | `make up` |
| Check health | `make remote-status --json` | `make status --json` |
| View logs | `make remote-logs COMPONENT=db` | `make logs COMPONENT=db` |
| Reset state | `make remote-reset` | `make reset` |
| Stop stack | `make remote-stop` | `make stop` |
| Destroy infra | `make remote-destroy` | (n/a) |

## 6. Observability & Metrics

- Remote bootstrap, status, and teardown events ship to centralized logging with workspace correlation IDs
- Local `make status --json` output feeds into `scripts/metrics/publish_local_status.sh` for optional aggregation
- Use `make metrics-report WORKSPACE=<name>` to generate latency and success charts for the last 24 hours

## 7. TTL Automation & Secrets Rotation

### TTL Enforcement

Remote workspaces have a default TTL of 24 hours. TTL enforcement:

- **Automatic teardown**: Workspaces automatically terminate when TTL expires
- **Warning signs**: `make remote-status` shows TTL warnings when workspace age approaches expiration
- **Extension**: Reprovision with extended TTL:
  ```bash
  make remote-provision \
    WORKSPACE=<your-initials>-dev \
    WORKSPACE_TTL_HOURS=48  # Extend to 48 hours
  ```
- **TTL checks**: All lifecycle commands (`remote-up`, `remote-reset`, `remote-destroy`) verify TTL before execution
- **Audit logging**: TTL warnings are logged to `~/.ai-aas/workspace-audit.log`

### Secrets Rotation

Secrets are automatically rotated based on PAT (Personal Access Token) TTL:

- **Sync secrets**: `make remote-secrets WORKSPACE=<name>` syncs from GitHub secrets
- **Rotate credentials**: When PAT expires, re-run `gh auth login` then `make remote-secrets`
- **Local secrets**: Use `make remote-secrets` to generate `.env.local` for local development
- **Secrets validation**: The `secrets-sync` command validates PAT scopes and enforces `.gitignore` rules

### Lifecycle Troubleshooting

**Workspace TTL warnings:**
```bash
# Check workspace age and TTL
make dev-status --mode remote --host <ip> --diagnose

# Reprovision if TTL expired
make remote-destroy WORKSPACE=<name> WORKSPACE_HOST=<ip>
make remote-provision WORKSPACE=<name> WORKSPACE_OWNER=<owner>
```

**Port conflicts (local):**
```bash
# Diagnose port conflicts with remediation guidance
make diagnose

# Override ports via environment variables
export POSTGRES_PORT=5433
export REDIS_PORT=6380
export NATS_CLIENT_PORT=4223
export MINIO_API_PORT=9002
export MOCK_INFERENCE_PORT=8001
make up
```

**Component health issues:**
```bash
# Check detailed component status
make status JSON=true | jq '.components[] | select(.state != "healthy")'

# View component-specific logs
make logs COMPONENT=postgres  # or redis, nats, minio, mock-inference

# Reset stack if needed
make reset  # Local: stops, removes volumes, restarts, re-seeds
make remote-reset WORKSPACE_HOST=<ip>  # Remote: same, with 90-day log retention
```

**Secrets sync failures:**
```bash
# Verify GitHub CLI authentication
gh auth status

# Re-authenticate if needed
gh auth login --scopes "repo, read:actions"

# Retry secrets sync
make remote-secrets WORKSPACE=<name>
```

## 8. Troubleshooting Reference

| Symptom | Resolution |
|---------|------------|
| `make remote-provision` fails quota check | Verify Linode quota; clean up old workspaces with `make remote-destroy` |
| TTL warning in lifecycle commands | Workspace age exceeds TTL; reprovision with extended TTL or destroy/recreate |
| Tunnel connection timeout | Re-run `linode ssh bastion --workspace <name> --reset`; ensure MFA device available |
| `make remote-up` stuck on image pulls | Run `make remote-logs COMPONENT=bootstrap` and check Linode registry rate limits |
| Local start fails due to ports | Run `make diagnose` for port conflict detection and remediation guidance |
| Port conflicts detected | Use `make diagnose` to see remediation steps; override ports via environment variables |
| Secrets out of date | Run `gh auth login` then `make secrets-sync` to fetch fresh credentials |
| Environment profile not found | Verify profile exists in `configs/environments/`; check profile name spelling |
| Configuration validation fails | Run `make env-validate` for detailed error messages; check component registry for required fields |
| Wrong environment active | Check active profile with `make env-show`; activate correct environment with `make env-activate` |
| Health check reports `degraded` | Inspect component logs (`make remote-logs` / `make logs`), confirm prerequisite services online |
| Component not starting | Check logs: `make logs COMPONENT=<name>`, verify dependencies, try `make reset` |
| Remote workspace unresponsive | Check TTL status: `make dev-status --mode remote --host <ip> --diagnose`, reprovision if expired |

## 9. Validation & Verification

### Quick Validation

Run the validation script to verify your setup:
```bash
./scripts/dev/validate_quickstart.sh
```

This script:
- Verifies local stack health (`make up`, `make status`)
- Tests component connectivity (PostgreSQL, Redis, NATS, MinIO, Mock Inference)
- Validates service template integration
- Reports any configuration issues

### CI Validation

The dev environment CI workflow (`.github/workflows/dev-environment-ci.yml`) automatically:
- Validates Go code formatting and tests
- Checks Terraform configuration (fmt/validate)
- Lints shell scripts (`scripts/dev/*.sh`)
- Runs startup measurement scripts (`measure_local_startup.sh`, `measure_remote_startup.sh`)

## 10. Next Steps

1. Review `specs/002-local-dev-environment/spec.md` for acceptance criteria alignment
2. Review `specs/002-local-dev-environment/plan.md` for implementation approach
3. Review `docs/runbooks/service-dev-connect.md` for detailed connectivity troubleshooting
4. Review `docs/platform/data-classification.md` for data retention policies
5. Run validation script: `./scripts/dev/validate_quickstart.sh`

---

Need deeper guidance? Open an issue in `docs/runbooks/` to track new troubleshooting patterns discovered during implementation.
