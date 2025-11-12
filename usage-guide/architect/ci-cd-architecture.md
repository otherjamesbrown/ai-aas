# CI/CD Architecture

## Overview

This document describes the CI/CD (Continuous Integration/Continuous Deployment) architecture of the AIaaS platform, including how code flows from development to production through automated pipelines.

## CI/CD Pipeline Architecture

### High-Level Flow

```
┌─────────────────┐
│  Developer      │
│  (Git Push/PR)  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────┐
│     GitHub Actions CI           │
│                                 │
│  • Build                        │
│  • Test                         │
│  • Lint                         │
│  • Security Scan                │
│  • Contract Tests               │
└────────┬────────────────────────┘
         │
         ▼ (on merge to main)
┌─────────────────────────────────┐
│  Infrastructure CI (Terraform)  │
│                                 │
│  • Terraform Validate            │
│  • Terraform Plan               │
│  • Security Scan (tfsec)        │
│  • Apply (via GitHub Actions)   │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│     Git Repository              │
│     (Source of Truth)            │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│     ArgoCD (GitOps)             │
│                                 │
│  • Watches Git                  │
│  • Syncs Helm Charts            │
│  • Deploys to Kubernetes        │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│     Kubernetes Cluster          │
│     (Production)                 │
└─────────────────────────────────┘
```

## CI Pipeline (GitHub Actions)

### Main CI Workflow (`ci.yml`)

**Triggers**:
- Push to `main` branch
- Pull requests

**Jobs**:

1. **Setup & Discover Services**
   - Discovers all services in `services/` directory
   - Outputs service list as JSON
   - Shows Make help for validation

2. **Build** (Matrix Strategy)
   - Builds each service in parallel
   - Uses reusable workflow `reusable-build.yml`
   - Outputs: Build artifacts

3. **Test** (Matrix Strategy)
   - Runs tests for each service
   - Depends on: Build
   - Outputs: Test results

4. **Lint**
   - Runs `golangci-lint` across all services
   - Depends on: Test
   - Validates code quality

5. **Metrics Upload**
   - Collects build/test metrics
   - Uploads to S3 (Linode Object Storage)
   - Runs even if previous jobs fail (`if: always()`)

### Remote CI Workflow (`ci-remote.yml`)

**Purpose**: Allow contributors on restricted machines to run CI via GitHub Actions

**Triggers**:
- Manual dispatch (`workflow_dispatch`)

**Inputs**:
- `service`: Target service (default: "all")
- `notes`: Additional notes for audit trail

**Jobs**: Same as main CI workflow

**Usage**:
```bash
make ci-remote SERVICE=api-router-service NOTES="testing fix"
```

### Reusable Build Workflow (`reusable-build.yml`)

**Purpose**: Shared workflow for building and testing services

**Inputs**:
- `service`: Service name or "all"
- `target`: Build target (`build` or `test`)
- `go-version`: Go version to use

**Steps**:
1. Checkout code
2. Setup Go
3. Cache Go modules
4. Run Make target (`make build` or `make test`)

## CD Pipeline (GitOps with ArgoCD)

### GitOps Flow

```
Git Repository (main branch)
  ↓
ArgoCD Application (watches Git)
  ↓
Helm Chart Rendering
  ↓
Kubernetes API (applies manifests)
  ↓
Pods Running
```

### Infrastructure CD (Terraform)

**Workflow**: `infra-terraform.yml`

**Triggers**:
- Push to `main` with changes to `infra/terraform/`
- Manual dispatch

**Jobs**:

1. **Terraform Validate**
   - `terraform fmt -check`
   - `terraform validate`
   - `tflint` (linting)
   - `tfsec` (security scanning)

2. **Terraform Plan**
   - Generates plan for each environment
   - Validates changes

3. **Terraform Apply** (Production requires approval)
   - Applies infrastructure changes
   - Uses OIDC for authentication
   - Stores state in Linode Object Storage

**Environments**:
- `development`: Auto-apply
- `staging`: Auto-apply
- `production`: Requires manual approval

### Application CD (Helm + ArgoCD)

**Flow**:

1. **Helm Chart Changes**
   - Changes to `infra/helm/charts/*` or `configs/kustomize/*`
   - Committed to Git

2. **ArgoCD Sync**
   - ArgoCD detects changes
   - Syncs Helm charts to Kubernetes
   - Production: Manual sync required

3. **Deployment**
   - Helm renders manifests
   - Kubernetes applies changes
   - Health checks verify deployment

**ArgoCD Applications**:
- `platform-<env>-infrastructure`: Infrastructure components
- `platform-<env>-<service>`: Service deployments

## Security & Quality Gates

### Security Scanning

**CI Pipeline**:
- **CodeQL**: Code security analysis
- **gitleaks**: Secret detection
- **trivy**: Container image scanning
- **tfsec**: Terraform security scanning
- **golangci-lint**: Security-focused linting

**Enforcement**:
- Security scans must pass before merge
- Secrets detection blocks commits
- Container scanning blocks deployment

### Quality Gates

**Required Checks**:
- ✅ All tests pass
- ✅ Linting passes
- ✅ Security scans pass
- ✅ Contract tests pass
- ✅ Terraform validation passes (infrastructure changes)

**Blocking**:
- PR cannot merge if any check fails
- Production deployments require approvals

## Service-Specific CI/CD

### Service Workflows

Each service can have its own workflow (e.g., `api-router-service.yml`):

**Purpose**:
- Service-specific validation
- Contract tests
- Integration tests
- Performance benchmarks

**Pattern**:
```yaml
on:
  push:
    paths:
      - 'services/api-router-service/**'
  pull_request:
    paths:
      - 'services/api-router-service/**'
```

### Database Migrations

**Workflow**: `db-guardrails.yml`

**Purpose**:
- Validates database migrations
- Checks migration syntax
- Ensures migrations are reversible

**Triggers**:
- Changes to `db/migrations/**`

## Deployment Strategies

### Blue-Green Deployment

**Status**: Not currently implemented

**Future**: Consideration for zero-downtime deployments

### Rolling Updates

**Current**: Kubernetes rolling updates

**Configuration**: Helm chart `deploymentStrategy`

### Canary Releases

**Status**: Not currently implemented

**Future**: Consideration for gradual traffic shift

## Environment Promotion

### Promotion Flow

```
Development → Staging → Production
```

**Process**:

1. **Development**
   - Auto-deploy on merge to `main`
   - Fast feedback loop
   - No approval required

2. **Staging**
   - Auto-deploy on merge to `main`
   - Validation environment
   - Smoke tests run

3. **Production**
   - Manual ArgoCD sync required
   - Requires approval
   - Health checks mandatory

### Promotion Criteria

**Development → Staging**:
- ✅ All CI checks pass
- ✅ No blocking issues

**Staging → Production**:
- ✅ Staging validation successful
- ✅ Manual approval
- ✅ Health checks pass

## Monitoring & Observability

### CI/CD Metrics

**Collected**:
- Build duration
- Test duration
- Deployment duration
- Success/failure rates

**Storage**: Linode Object Storage (`ai-aas-build-metrics`)

**Usage**: Performance tracking and optimization

### Deployment Monitoring

**ArgoCD**:
- Sync status
- Health status
- Resource status

**Kubernetes**:
- Pod status
- Health checks
- Resource utilization

**Alerting**: Prometheus alerts for failed deployments

## Rollback Procedures

### Application Rollback

**Via ArgoCD**:
1. Identify previous working revision
2. Sync to previous revision: `argocd app sync <app> --revision <rev>`
3. Verify health checks

**Via Git**:
1. Revert commit in Git
2. ArgoCD auto-syncs (or manual sync)
3. Verify deployment

### Infrastructure Rollback

**Via Terraform**:
1. Revert Terraform changes in Git
2. Run `terraform apply` with previous state
3. Verify infrastructure health

**Documentation**: `docs/runbooks/infrastructure-rollback.md`

## CI/CD Best Practices

### Workflow Design

- **Reusable Workflows**: Share common patterns
- **Matrix Strategy**: Parallel execution
- **Job Dependencies**: Explicit `needs:` declarations
- **Conditional Execution**: `if:` for optional steps

### Security

- **Secrets**: Never in Git, use GitHub Secrets
- **OIDC**: Use OIDC for cloud authentication
- **Least Privilege**: Minimal permissions for workflows
- **Secret Rotation**: Regular rotation of credentials

### Performance

- **Caching**: Cache Go modules, dependencies
- **Parallel Execution**: Matrix strategy for services
- **Early Failure**: Fail fast on critical checks
- **Resource Optimization**: Efficient resource usage

## Troubleshooting

### Common Issues

1. **Workflow Not Triggering**
   - Check workflow file exists on `main` branch
   - Verify trigger conditions
   - Check GitHub Actions permissions

2. **Build Failures**
   - Check Go version compatibility
   - Verify dependencies
   - Review build logs

3. **Deployment Failures**
   - Check ArgoCD sync status
   - Verify Kubernetes resources
   - Review health checks

### Debugging

**CI Pipeline**:
```bash
# View workflow runs
gh run list --workflow ci.yml

# View specific run
gh run view <run-id>

# View logs
gh run view <run-id> --log
```

**ArgoCD**:
```bash
# Check application status
argocd app get <app-name>

# View sync history
argocd app history <app-name>

# View resource status
argocd app resources <app-name>
```

## Related Documentation

- [GitHub Actions Guide](../../docs/platform/github-actions-guide.md) - Workflow best practices
- [Deployment Architecture](./deployment-architecture.md) - Infrastructure deployment
- [Architectural Principles](./architectural-principles.md) - CI/CD enforcement of principles
- [CI Remote Runbook](../../docs/runbooks/ci-remote.md) - Remote CI usage

