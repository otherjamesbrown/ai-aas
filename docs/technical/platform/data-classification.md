# Data Classification & Retention for Development Environments

**Last Updated**: 2025-01-20  
**Applies To**: Local and remote development workspaces

## Overview

This document defines data classification levels and retention policies for artifacts generated in development environments (local stacks and remote workspaces). These policies ensure compliance with security requirements while supporting developer productivity.

## Classification Levels

### Internal
**Definition**: Non-sensitive operational data that is safe for development environments.

**Examples**:
- Service configuration (non-secret)
- Logs without PII or credentials
- Test data (synthetic)
- Metrics and performance data
- Container image metadata

**Retention**: 30 days (development), 90 days (staging), permanent (production)
**Encryption**: Not required for at-rest storage
**Access**: Development team members

### Confidential
**Definition**: Data that requires protection but is acceptable in development with appropriate controls.

**Examples**:
- Service connection strings (with masked credentials)
- API response schemas
- Test fixtures with non-production data
- Build artifacts
- Deployment manifests (without secrets)

**Retention**: 90 days (development), 180 days (staging), per operational policy (production)
**Encryption**: Recommended for at-rest storage
**Access**: Restricted to authorized developers

### Restricted
**Definition**: Sensitive data that must be handled with strict controls, even in development.

**Examples**:
- Credentials (passwords, API keys, tokens)
- PII (personally identifiable information)
- Production database dumps
- Real user data
- SSH private keys
- Service account keys

**Retention**: 24 hours (remote workspaces), 7 days (local), per compliance requirements (production)
**Encryption**: Required for at-rest and in-transit
**Access**: Limited to specific authorized individuals
**Handling**: Must be redacted in logs, never committed to version control

## Retention Policies

### Remote Workspaces

**Operational Artifacts** (Internal):
- System logs: 90 days
- Application logs: 90 days
- Metrics: 30 days
- Container logs: 90 days

**Development Artifacts** (Confidential):
- Workspace state: 24 hours (TTL enforced)
- Docker volumes: 24 hours (ephemeral)
- Build artifacts: 7 days
- Test outputs: 7 days

**Secrets & Credentials** (Restricted):
- Environment files (`.env.linode`): 24 hours (automatic cleanup)
- SSH keys: Per workspace lifecycle
- API tokens: Per PAT TTL (max 24 hours)

### Local Development

**Operational Artifacts** (Internal):
- Local logs: 30 days (optional cleanup)
- Metrics: Optional upload to centralized system
- Container logs: 30 days (rotated)

**Development Artifacts** (Confidential):
- Docker volumes: Per developer preference (recommended: cleanup on `make reset`)
- Build artifacts: Per developer preference
- Test outputs: Per developer preference

**Secrets & Credentials** (Restricted):
- Environment files (`.env.local`): Per developer (must be `.gitignore`d)
- Local credentials: Stored securely, never in VCS
- API tokens: Per PAT TTL

## Log Retention

### Remote Workspace Logs

**Classification**: Internal (with redaction for Restricted fields)

**Retention**: 90 days

**Sources**:
- Systemd journal logs
- Docker Compose service logs
- Application logs from `/opt/ai-aas/dev-stack/logs/`
- Vector agent logs

**Redaction Requirements**:
- Passwords: `password=***REDACTED***`
- Tokens: `LINODE_TOKEN=***REDACTED***`
- Connection strings: `postgres://***REDACTED***@host/db`
- SSH keys: `***REDACTED SSH KEY***`

**Configuration**: See `scripts/dev/vector-agent.toml` for redaction patterns.

### Local Development Logs

**Classification**: Internal

**Retention**: 30 days (optional)

**Cleanup**: Manual or via `make reset`

## Workspace Lifecycle

### Remote Workspace TTL

- **Default**: 24 hours
- **Enforcement**: Automatic teardown via Terraform tags and cleanup automation
- **Extension**: Manual via `make remote-provision` with extended TTL
- **Grace Period**: 1 hour warning before teardown

### Data Cleanup on Teardown

**Automatic**:
- Docker volumes removed
- Container images pruned (optional)
- Environment files deleted
- SSH keys revoked
- Logs shipped to centralized storage (Loki)

**Manual** (if needed):
- `make remote-destroy` explicitly removes all resources
- Terraform state cleaned

### Local Development Cleanup

**Commands**:
- `make reset` - Stops stack, removes volumes, restarts
- `make stop` - Stops stack, preserves volumes
- Manual cleanup - Developer responsibility

## Artifact Storage

### Remote Workspaces

**Local Storage** (ephemeral):
- `/opt/ai-aas/dev-stack/data/` - Docker volumes (24h TTL)
- `/var/log/ai-aas/` - Application logs (90d retention)
- `/var/lib/docker/` - Container images and layers

**Centralized Storage**:
- **Loki**: Log aggregation (90d retention)
- **Prometheus/Grafana**: Metrics (30d retention)
- **MinIO S3**: Artifacts (7d retention)

### Local Development

**Storage**:
- Docker volumes: `~/.docker/volumes/` or project-specific
- Logs: Project directory or system logs
- Artifacts: Project-specific directories

**Recommendation**: Clean up artifacts regularly to free disk space.

## Security Controls

### Access Control

**Remote Workspaces**:
- SSH access via Linode bastion with MFA
- Session recording enabled
- Audit logs for all lifecycle operations
- Workspace owner identification via tags

**Local Development**:
- Developer machine access controls
- `.env.*` files excluded from version control
- Secrets management via GitHub environment secrets

### Secret Management

**Remote**:
- Secrets synced from GitHub repository environment
- 24-hour TTL on `.env.linode`
- Automatic redaction in logs
- Vector agent configured for log sanitization

**Local**:
- Secrets synced via `make remote-secrets` or manual configuration
- `.env.local` must be in `.gitignore`
- Developer responsibility to protect local secrets

## Compliance

### Development Environment Data

**GDPR/PII**:
- No real user data in development workspaces
- Synthetic test data only
- PII redaction in logs

**Audit Requirements**:
- All remote workspace lifecycle events logged
- Audit trail in `${HOME}/.ai-aas/workspace-audit.log`
- Correlation IDs for traceability

### Data Sovereignty

**Storage Locations**:
- Remote workspaces: Linode region (configurable, default: fr-par)
- Local development: Developer machine
- Centralized logs: Observability stack (per observability spec)

## Implementation

### Automated Enforcement

**Remote Workspaces**:
- Terraform tags include TTL metadata
- Cleanup scripts check TTL and trigger teardown
- Vector agent ships logs with retention tags
- Systemd timers for periodic cleanup checks

**Scripts**:
- `scripts/dev/remote_lifecycle.sh` - Lifecycle management with audit logging
- `scripts/dev/vector-agent.toml` - Log redaction and shipping
- `scripts/dev/remote_provision.sh` - Provisioning with TTL enforcement

### Monitoring

**Metrics**:
- Workspace creation/destruction events
- TTL compliance (time until teardown)
- Log retention compliance
- Secret exposure incidents (via log scanning)

**Alerts**:
- Workspace exceeding TTL grace period
- Secrets detected in logs (pattern matching)
- Audit log failures

## References

- `configs/log-redaction.yaml` - Log redaction patterns
- `scripts/dev/vector-agent.toml` - Vector agent configuration
- `specs/002-local-dev-environment/` - Development environment specification
- `specs/011-observability/` - Observability and logging specification

## Updates

This policy is reviewed and updated as needed. Last update: 2025-01-20

Changes:
- Initial version: Defines classification levels and retention for dev environments
- Integrates with remote workspace TTL enforcement
- Aligns with observability stack requirements

