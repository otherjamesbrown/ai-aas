# Local Development Environment Contracts

**Branch**: `002-local-dev-environment`  
**Date**: 2025-11-10

## CLI Contracts

| Command | Inputs | Outputs | Guarantees |
|---------|--------|---------|------------|
| `make remote-provision` | `WORKSPACE` (kebab-case), `REGION`, optional `TTL_HOURS` | Terraform plan/apply output, workspace ID on stdout, audit log ID | Idempotent; creates/updates Terraform workspace resources only; fails safe on quota errors |
| `make secrets-sync` | `MODE` (`remote`\|`local`), optional `WORKSPACE` | `.env.linode` and/or `.env.local` written with 0600 perms; masked summary | No secret printed to stdout/stderr; exits non-zero if `.gitignore` missing entries |
| `make remote-up` | `WORKSPACE` | Streams bootstrap logs, returns 0 when Docker Compose stack healthy | Restarts systemd units if already running; ensures compose file hash applied |
| `make remote-status --json` | `WORKSPACE`, optional `--component` filter | JSON payload with per-component health, latency, message | Returns exit 1 when any component unhealthy; emits log event with correlation ID |
| `make remote-reset` | `WORKSPACE`, optional `--preserve-volumes` | Confirmation prompt; resets volumes unless preserved; status JSON post-reset | Blocks until health restored or timeout (default 180s); cleanup actions logged |
| `make remote-logs COMPONENT=<name>` | `WORKSPACE`, `COMPONENT` enum (`postgres`, `redis`, `nats`, `minio`, `mock-inference`, `bootstrap`) | Streams logs with timestamps | Supports `--since` (duration) and `--follow`; enforces 15-minute session timeout |
| `make remote-destroy` | `WORKSPACE` | Terraform destroy output, teardown confirmation | Verifies workspace idle; forces destroy when TTL exceeded; logs audit trail |
| `make up` | optional `PROFILE` (e.g., `default`) | Compose stack running locally | Honors `.specify/local/ports.yaml` overrides; prints next steps |
| `make status --json` | optional `--component` | JSON payload identical to remote contract | Mirrors remote exit semantics; includes mode=`local` in payload |
| `make reset` | optional `--preserve-volumes` | Resets local volumes, seeds data, prints status summary | Commands executed with `docker compose`; handles port cleanup |
| `make logs COMPONENT=<name>` | `COMPONENT` enum | Streams container logs locally | Honors `--since`, `--follow`; colorizes output |
| `make stop` | none | Stops local compose stack | Leaves volumes intact unless `make reset` invoked |

## Terraform Module Interface (`modules/dev-workspace`)

| Variable | Type | Description | Default |
|----------|------|-------------|---------|
| `workspace_name` | string | Kebab-case identifier; used for tags, hostname prefix | n/a |
| `region` | string | Linode region slug | `us-east` |
| `ttl_hours` | number | Hours before TTL automation destroys workspace | `24` |
| `stackscript_id` | number | StackScript responsible for bootstrap | Provided via locals |
| `enable_private_vlan` | bool | Attach private VLAN and security groups | `true` |
| `tags` | list(string) | Additional tags merged with defaults (`env:dev`, `feature:002`) | `[]` |
| `log_agent_config` | string | Base64 cloud-init for Vector/Loki credentials | n/a |
| `vault_role_id` | string | Vault AppRole ID used by secrets sync helper | n/a |
| `vault_secret_id` | string | Vault Secret ID with TTL ≤ 24h | n/a |

| Output | Type | Description |
|--------|------|-------------|
| `linode_instance_id` | number | Numeric Linode ID |
| `private_ip` | string | RFC1918 IP used by tunnel and services |
| `ssh_command` | string | Convenience command `linode ssh bastion --workspace <name>` |
| `workspace_labels` | map(string) | Tags (owner, ttl, feature) for observability |

## Secrets Sync Contract (`cmd/secrets-sync`)

```json
{
  "workspace": "dev-jab",
  "mode": "remote",
  "generated_at": "2025-11-10T15:20:45Z",
  "files": [
    {
      "path": ".env.linode",
      "keys": ["POSTGRES_HOST", "POSTGRES_USER", "POSTGRES_PASSWORD", "REDIS_URL", "NATS_URL", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MOCK_INFERENCE_URL"],
      "expires_at": "2025-11-11T15:20:45Z"
    },
    {
      "path": ".env.local",
      "keys": ["POSTGRES_HOST", "REDIS_URL", "NATS_URL", "MINIO_ENDPOINT", "MOCK_INFERENCE_URL"],
      "expires_at": null
    }
  ],
  "audit_id": "b4c6f9d0-6e4f-4c20-b3da-8d68ac2f9a79"
}
```

- Vault path: `kv/data/dev-workspaces/<workspace>` (remote) and `kv/data/local-stack/default` (local).  
- AppRole tokens must have TTL ≤ 24h and renewable.  
- Command exits with code `42` when `.gitignore` missing required entries (treat as actionable error).

## Health Status Contract (`cmd/dev-status` output)

```json
{
  "mode": "remote",
  "workspace": "dev-jab",
  "timestamp": "2025-11-10T15:22:10Z",
  "components": [
    { "name": "postgres", "state": "healthy", "latency_ms": 45, "message": "ready" },
    { "name": "redis", "state": "healthy", "latency_ms": 12, "message": "pong" },
    { "name": "nats", "state": "healthy", "latency_ms": 18, "message": "connected" },
    { "name": "minio", "state": "healthy", "latency_ms": 32, "message": "list-buckets" },
    { "name": "mock-inference", "state": "healthy", "latency_ms": 67, "message": "200 OK" }
  ],
  "summary": {
    "state": "healthy",
    "latency_p95_ms": 70,
    "unhealthy_components": []
  }
}
```

- `state` ∈ {`healthy`, `degraded`, `unhealthy`}.  
- When any component `unhealthy`, `summary.state` must reflect that and `unhealthy_components` list details.  
- Command accepts `--output=/path/to/file` for automation to persist payload.

## Observability Events

| Event | Producer | Fields | Destination |
|-------|----------|--------|-------------|
| `workspace.provisioned` | `make remote-provision` | `workspace_id`, `owner`, `timestamp`, `linode_instance_id`, `region`, `ttl_hours` | Loki (`pipeline=dev-env`) |
| `workspace.teardown` | TTL automation or `make remote-destroy` | `workspace_id`, `owner`, `timestamp`, `reason` (`ttl_expired\|manual`), `duration_minutes` | Loki |
| `secrets.hydrated` | `cmd/secrets-sync` | `workspace_id`, `mode`, `generated_at`, `audit_id`, `key_count` | Loki + Prometheus counter `dev_env_secrets_sync_total` |
| `status.report` | `cmd/dev-status` | `workspace_id`, `mode`, `summary.state`, `duration_ms` | Prometheus pushgateway (`job=dev-env-status`) |

## Security & Compliance Requirements

- All CLI commands must respect `XDG_CONFIG_HOME` overrides for credentials.  
- `make remote-*` commands store transient SSH keys under `~/.cache/ai-aas/workspaces/<id>` and purge after command completion.  
- Logs must redact secrets using regex patterns defined in `configs/log-redaction.yaml`.  
- Session recordings stored in security account for 90 days; metadata streamed as `workspace.session.recorded` events.

## Error Codes

| Code | Scenario | Remediation |
|------|----------|-------------|
| `31` | Vault authentication failure | Re-run `make secrets-sync` after renewing AppRole secret ID |
| `32` | Health probe timeout | Inspect component logs; re-run `make remote-status --diagnose` |
| `33` | Port conflict detected locally | Review `make status --diagnose` output; update `.specify/local/ports.yaml` |
| `41` | Terraform apply partial failure | Run `make remote-provision --plan` to inspect drift, then retry |
| `42` | `.gitignore` missing secret file entries | Add `.env.linode` and `.env.local` to `.gitignore` and re-run |

