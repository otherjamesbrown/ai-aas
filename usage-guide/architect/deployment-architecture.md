# Deployment Architecture

## Overview

This document describes the deployment architecture, infrastructure components, and deployment patterns used in the AIaaS platform.

## Infrastructure Provider

### Akamai Linode Kubernetes Engine (LKE)

- **Provider**: Akamai Linode
- **Region**: `fr-par` (configurable via Terraform)
- **Cluster Type**: Managed Kubernetes
- **Control Plane**: Managed by Linode

### Environments

All environments share a single Kubernetes control plane with dedicated namespaces:

- **development**: Development and testing
- **staging**: Pre-production validation
- **production**: Production workloads
- **system**: Platform infrastructure (monitoring, ingress)

### Node Pools

#### Baseline Pool
- **Instance Type**: `g6-standard-8`
- **Nodes**: 3-6 per environment (autoscaling to 10)
- **Purpose**: Application workloads
- **Scaling**: Horizontal Pod Autoscaler (HPA)

#### GPU Pool
- **Instance Type**: `g1-gpu-rtx6000`
- **Nodes**: 2 nodes (system namespace)
- **Purpose**: vLLM workloads
- **Scaling**: Manual scaling

## Networking Architecture

### Ingress

- **Controller**: NGINX Ingress Controller
- **TLS**: cert-manager with Let's Encrypt
- **DNS**: Linode DNS for external records
- **Load Balancing**: Linode NodeBalancer

### Network Policies

- **CNI**: Calico
- **Policy**: Default-deny stance
- **Isolation**: Namespace-level isolation
- **Zero-Trust**: Explicit allow rules required

### Service Mesh

- **Current**: Not implemented
- **Future**: Consideration for Istio/Linkerd

## Deployment Patterns

### GitOps with ArgoCD

**Flow**:
```
Git Repository (source of truth)
  ↓
ArgoCD (watches Git)
  ↓
Kubernetes API (applies manifests)
  ↓
Pods running workloads
```

**Components**:
- **Terraform**: Provisions infrastructure
- **Helm Charts**: Application definitions
- **ArgoCD**: GitOps reconciliation
- **Kustomize**: Environment-specific overlays

### Helm Charts

**Location**: `infra/helm/charts/*`

**Structure**:
- Service-specific charts
- Shared dependencies (Redis, PostgreSQL, RabbitMQ)
- Environment overlays (`development`, `staging`, `production`)

**Values**:
- Environment-specific configuration
- Resource limits and requests
- Replica counts
- Service endpoints

### Kustomize Overlays

**Location**: `configs/kustomize/`

**Purpose**: Environment-specific configuration

**Overlays**:
- `development/`: Dev-specific settings
- `staging/`: Staging-specific settings
- `production/`: Production-specific settings

## Secrets Management

### Linode Secret Manager

- **Source of Truth**: Linode Secret Manager
- **Bundles**: Defined in `infra/secrets/bundles/*.yaml`
- **Encryption**: Sealed Secrets controller
- **GitOps**: ArgoCD applies encrypted manifests

### Sealed Secrets

- **Controller**: Sealed Secrets controller in cluster
- **Encryption**: Public key encryption
- **Storage**: Encrypted secrets in Git
- **Decryption**: Controller decrypts at apply time

### Secret Rotation

- **Frequency**: Quarterly rotation checks
- **Process**: Automated rotation scripts
- **Validation**: Post-rotation verification

## Database Architecture

### PostgreSQL

**Deployment**: Managed PostgreSQL instances per service

**Services**:
- **User & Organization Service**: Primary database
- **Analytics Service**: TimescaleDB extension

**Configuration**:
- Connection pooling per service
- Read replicas (future consideration)
- Automated backups

### Redis

**Deployment**: Shared Redis cluster

**Usage**:
- API Router Service: Rate limiting
- User & Organization Service: Session cache
- Analytics Service: Freshness cache

**Configuration**:
- Database separation per service
- TTL-based expiration
- Persistence enabled

### RabbitMQ

**Deployment**: RabbitMQ cluster

**Usage**:
- Usage event queue (API Router → Analytics)
- Streams support enabled

**Configuration**:
- Durable queues
- At-least-once delivery
- Dead-letter queues

## Observability Stack

### Prometheus/Grafana

**Deployment**: `kube-prometheus-stack` Helm chart

**Components**:
- Prometheus (metrics collection)
- Grafana (visualization)
- Alertmanager (alert routing)

**Dashboards**:
- Environment-tagged dashboards
- Service-specific dashboards
- Infrastructure health dashboards

### Loki

**Deployment**: Loki cluster

**Purpose**: Log aggregation

**Configuration**:
- Tenant per environment
- Retention policies
- Query performance optimization

### Tempo

**Deployment**: Tempo cluster (optional)

**Purpose**: Distributed tracing

**Configuration**:
- OpenTelemetry integration
- Trace retention
- Query performance

## Storage Architecture

### Persistent Volumes

**Provisioner**: Linode Block Storage CSI

**Usage**:
- Database data
- Application state (if needed)

**Backup**: Automated snapshots

### Object Storage

**Provider**: Linode Object Storage (S3-compatible)

**Usage**:
- Analytics exports
- Terraform state
- Backup artifacts

**Access**: Pre-signed URLs for secure access

## Deployment Process

### Infrastructure Changes

1. **Plan**: PR modifying `infra/terraform`
2. **Validate**: GitHub Actions (terraform fmt/validate/plan)
3. **Review**: Two approvals required for production
4. **Apply**: GitHub Actions executes `terraform apply`
5. **Verify**: Automated smoke tests
6. **Audit**: Events logged to Loki

### Application Changes

1. **Plan**: PR modifying Helm charts or manifests
2. **Validate**: Helm lint, ArgoCD validation
3. **Review**: Code review process
4. **Merge**: Changes merged to main branch
5. **Sync**: ArgoCD syncs changes (manual for production)
6. **Verify**: Health checks and smoke tests

### Rollback Process

1. **Identify**: Issue detected via monitoring
2. **Revert**: Git revert or ArgoCD rollback
3. **Apply**: ArgoCD syncs previous version
4. **Verify**: Health checks confirm recovery
5. **Document**: Post-mortem and lessons learned

## Scaling Architecture

### Horizontal Pod Autoscaling (HPA)

**Metrics**:
- CPU utilization (target: 70%)
- Memory utilization (target: 80%)
- Custom metrics (future)

**Scaling**:
- Min replicas: 2
- Max replicas: 10
- Scale-up: Aggressive (fast response)
- Scale-down: Conservative (stability)

### Vertical Pod Autoscaling (VPA)

**Status**: Not currently implemented

**Future**: Consideration for resource optimization

### Cluster Autoscaling

**Status**: Enabled

**Configuration**:
- Min nodes: 3 per environment
- Max nodes: 10 per environment
- Scale triggers: Pod scheduling failures

## High Availability

### Service Replicas

- **Minimum**: 2 replicas per service
- **Target**: 3 replicas per service
- **Maximum**: 10 replicas per service (HPA)

### Pod Disruption Budgets

- **Purpose**: Ensure availability during updates
- **Configuration**: Min available: 1 pod
- **Application**: All production services

### Database High Availability

- **PostgreSQL**: Managed high-availability setup
- **Redis**: Cluster mode with replication
- **RabbitMQ**: Cluster with mirrored queues

## Disaster Recovery

### Backup Strategy

- **Databases**: Automated daily backups
- **State**: Terraform state in object storage
- **Secrets**: Linode Secret Manager (replicated)

### Recovery Procedures

- **RTO**: 4 hours (Recovery Time Objective)
- **RPO**: 24 hours (Recovery Point Objective)
- **Runbooks**: Documented recovery procedures

### Testing

- **Frequency**: Quarterly disaster recovery drills
- **Scenarios**: Database failure, cluster failure
- **Documentation**: Post-drill reports

## Security Architecture

### Network Security

- **Network Policies**: Calico default-deny
- **Ingress**: TLS termination at ingress
- **Egress**: Controlled via network policies

### Pod Security

- **Standards**: Pod Security Standards (restricted)
- **RBAC**: Service accounts with minimal permissions
- **Secrets**: Mounted as volumes (never in environment)

### Image Security

- **Scanning**: Trivy scans in CI/CD
- **Base Images**: Minimal base images
- **Updates**: Regular security updates

## Related Documentation

- [Infrastructure Overview](../../docs/platform/infrastructure-overview.md) - Detailed infrastructure docs
- [Architecture Overview](./architecture-overview.md) - High-level architecture
- [System Components](./system-components.md) - Component details

