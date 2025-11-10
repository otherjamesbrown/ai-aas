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
3. Hydrate environment secrets:
   ```bash
   make secrets-sync WORKSPACE=<your-initials>-dev
   ```
   - Generates `.env.linode` and `.env.local` (gitignored) with masked credentials
4. Connect via secure tunnel:
   ```bash
   linode ssh bastion --workspace <your-initials>-dev
   ```
   - MFA + session recording enforced
   - Idle timeout defaults to 30 minutes
5. Bootstrap remote stack:
   ```bash
   make remote-up WORKSPACE=<your-initials>-dev
   ```
   - Installs system packages, pulls container images, and starts dependencies as systemd services
6. Verify health:
   ```bash
   make remote-status --json | jq '.components[] | {name, state, message}'
   ```
   - Expected states: `healthy`
   - Raw JSON forwarded to centralized logging automatically
7. Develop:
   ```bash
   make remote-shell WORKSPACE=<your-initials>-dev  # interactive shell with env loaded
   devcontainer open --workspace <your-initials>-dev  # optional VS Code/Cursor remote attach
   ```
8. Tear down (manual or wait for TTL automation):
   ```bash
   make remote-destroy WORKSPACE=<your-initials>-dev
   ```
   - Publishes teardown event to audit log stream

## 3. Local Stack Fallback (When Remote Path Unavailable)

1. Ensure Docker is running and ports are free:
   ```bash
   make doctor LOCAL_ONLY=true
   ```
2. Hydrate local env file (reuses secret store):
   ```bash
   make secrets-sync MODE=local
   ```
3. Start dependencies:
   ```bash
   make up
   ```
4. Check status parity:
   ```bash
   make status --json
   ```
5. Point services at local stack:
   ```bash
   export $(grep -v '^#' .env.local | xargs)
   make run SERVICE=user-org-service
   ```
6. Reset or stop as needed:
   ```bash
   make reset  # cleans volumes, reseeds data
   make stop   # stops all containers
   ```

## 4. Lifecycle Command Reference

| Scenario | Remote Command | Local Command |
|----------|----------------|---------------|
| Start stack | `make remote-up` | `make up` |
| Check health | `make remote-status --json` | `make status --json` |
| View logs | `make remote-logs COMPONENT=db` | `make logs COMPONENT=db` |
| Reset state | `make remote-reset` | `make reset` |
| Stop stack | `make remote-stop` | `make stop` |
| Destroy infra | `make remote-destroy` | (n/a) |

## 5. Observability & Metrics

- Remote bootstrap, status, and teardown events ship to centralized logging with workspace correlation IDs
- Local `make status --json` output feeds into `scripts/metrics/publish_local_status.sh` for optional aggregation
- Use `make metrics-report WORKSPACE=<name>` to generate latency and success charts for the last 24 hours

## 6. Troubleshooting

| Symptom | Resolution |
|---------|------------|
| `make remote-provision` fails quota check | Verify Linode quota; clean up old workspaces with `make remote-destroy --all-owned` |
| Tunnel connection timeout | Re-run `linode ssh bastion --workspace <name> --reset`; ensure MFA device available |
| `make remote-up` stuck on image pulls | Run `make remote-logs COMPONENT=bootstrap` and check Linode registry rate limits |
| Local start fails due to ports | Run `make status --diagnose` to view port owners, then free or remap ports via `.specify/local/ports.yaml` |
| Secrets out of date | Run `make secrets-sync --rotate` to fetch fresh credentials and invalidate stale ones |
| Health check reports `degraded` | Inspect component logs (`make remote-logs` / `make logs`), confirm prerequisite services online |

## 7. Next Steps

1. Review `specs/002-local-dev-environment/spec.md` for acceptance criteria alignment
2. Run `/speckit.plan specs/002-local-dev-environment` to generate implementation plan once prerequisites met
3. Update `llms.txt` with links to this quickstart, spec, plan, and relevant Linode documentation during rollout

---

Need deeper guidance? Open an issue in `docs/runbooks/` to track new troubleshooting patterns discovered during implementation.
