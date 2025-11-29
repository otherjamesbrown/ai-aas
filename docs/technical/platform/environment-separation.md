# Environment Separation Strategy for vLLM Deployments

**Feature**: `010-vllm-deployment` (User Story 3 - Safe operations)
**Last Updated**: 2025-01-27

## Overview

This document describes the environment separation strategy for vLLM model deployments to ensure isolation, safety, and controlled promotion between environments.

## Environment Definitions

### Development
- **Purpose**: Rapid iteration, feature development, and initial testing
- **Stability**: Unstable - frequent changes expected
- **Access**: All engineers
- **Data**: Synthetic or anonymized test data only
- **SLOs**: Relaxed (P95 latency < 5s)

### Staging
- **Purpose**: Pre-production validation with production-like configuration
- **Stability**: Stable - changes only after development validation
- **Access**: Restricted to deployment team and QA
- **Data**: Production-like test data (anonymized)
- **SLOs**: Production SLOs enforced (P95 latency < 3s)

### Production
- **Purpose**: Serve production traffic
- **Stability**: Very stable - changes only after staging validation
- **Access**: Highly restricted - deployment team only
- **Data**: Real production data
- **SLOs**: Strict production SLOs (P95 latency < 3s, error rate < 0.1%)

## Isolation Mechanisms

### 1. Namespace Isolation

Each environment uses dedicated Kubernetes namespaces:

```yaml
# Development
namespace: system

# Staging
namespace: system

# Production
namespace: system
```

**Note**: Current implementation uses shared `system` namespace with environment labels. For stronger isolation, consider:

```yaml
# Alternative: Environment-specific namespaces
development:
  namespace: vllm-development

staging:
  namespace: vllm-staging

production:
  namespace: vllm-production
```

**Benefits of separate namespaces:**
- Resource quota isolation
- Network policy enforcement
- RBAC separation
- Billing/cost tracking

### 2. Service Naming Convention

Services use environment-specific names to prevent cross-environment routing:

```
{model-name}-{environment}.{namespace}.svc.cluster.local:8000
```

**Examples:**
```
llama-2-7b-development.system.svc.cluster.local:8000
llama-2-7b-staging.system.svc.cluster.local:8000
llama-2-7b-production.system.svc.cluster.local:8000
```

This ensures:
- No accidental cross-environment traffic
- Clear service identification
- Predictable DNS resolution

### 3. Resource Labels

All resources are labeled with their environment:

```yaml
metadata:
  labels:
    app.kubernetes.io/environment: "development"  # or staging, production
    environment: "development"
    model: "llama-2-7b"
```

Labels enable:
- Environment-specific queries: `kubectl get pods -l environment=production`
- Monitoring and alerting segmentation
- Cost allocation and tracking

### 4. NetworkPolicies

NetworkPolicies enforce traffic isolation between environments:

```yaml
# Example: Production ingress policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vllm-production-ingress
  namespace: system
spec:
  podSelector:
    matchLabels:
      environment: production
  policyTypes:
    - Ingress
  ingress:
    # Allow only from API Router Service
    - from:
      - namespaceSelector:
          matchLabels:
            name: system
        podSelector:
          matchLabels:
            app: api-router-service
      ports:
        - protocol: TCP
          port: 8000
```

**Benefits:**
- Prevents unauthorized access
- Limits blast radius of security issues
- Enforces service mesh boundaries

### 5. Resource Quotas

Environment-specific resource quotas prevent resource exhaustion:

```yaml
# Example: Development resource quota
apiVersion: v1
kind: ResourceQuota
metadata:
  name: vllm-development-quota
  namespace: system
spec:
  hard:
    requests.nvidia.com/gpu: "2"    # Max 2 GPUs for development
    requests.memory: "64Gi"         # Max 64Gi memory
    requests.cpu: "16"              # Max 16 CPUs
    pods: "10"                      # Max 10 pods

---
# Production resource quota (higher limits)
apiVersion: v1
kind: ResourceQuota
metadata:
  name: vllm-production-quota
  namespace: system
spec:
  hard:
    requests.nvidia.com/gpu: "8"    # Max 8 GPUs for production
    requests.memory: "256Gi"        # Max 256Gi memory
    requests.cpu: "64"              # Max 64 CPUs
    pods: "20"                      # Max 20 pods
```

### 6. Database Registry Separation

Model registry entries are separated by environment:

```sql
SELECT * FROM model_registry_entries
WHERE deployment_environment = 'production'
  AND deployment_status = 'ready';
```

**Unique constraint** ensures no duplicate model+environment combinations:
```sql
CONSTRAINT model_registry_entries_unique_deployment
UNIQUE (model_name, deployment_environment)
```

### 7. Configuration Management

Environment-specific values files prevent configuration drift:

```
infra/helm/charts/vllm-deployment/
├── values.yaml                    # Base values
├── values-development.yaml        # Development overrides
├── values-staging.yaml           # Staging overrides
└── values-production.yaml        # Production overrides
```

**Key differences by environment:**

| Configuration | Development | Staging | Production |
|---------------|-------------|---------|------------|
| Replica Count | 1 | 1 | 2-3 |
| GPU Resources | 1 GPU | 1 GPU | 1-2 GPU |
| Memory Limit | 32Gi | 48Gi | 48Gi |
| Startup Timeout | 10 min | 15 min | 20 min |
| Auto-scaling | Disabled | Disabled | Enabled |
| Log Level | DEBUG | INFO | WARN |

## Access Control

### RBAC Policies

Implement role-based access control for each environment:

```yaml
# Development: Engineers can deploy
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vllm-developers
  namespace: system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vllm-developer
subjects:
  - kind: Group
    name: engineering
    apiGroup: rbac.authorization.k8s.io

---
# Production: Only deployment team can deploy
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vllm-production-deployers
  namespace: system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vllm-deployer
subjects:
  - kind: Group
    name: deployment-team
    apiGroup: rbac.authorization.k8s.io
```

### Database Access Control

Separate database credentials per environment:

```bash
# Development
DATABASE_URL="postgres://vllm_dev:***@localhost:5432/ai_aas_dev"

# Staging
DATABASE_URL="postgres://vllm_staging:***@db-staging:5432/ai_aas_staging"

# Production
DATABASE_URL="postgres://vllm_prod:***@db-prod:5432/ai_aas_prod"
```

## Promotion Workflow

### Controlled Promotion Path

Models must progress through environments in order:

```
Development → Staging → Production
```

**Never skip staging** - it's the final validation gate.

### Promotion Gates

Each environment has validation gates before promotion:

**Development → Staging:**
- ✅ All unit tests pass
- ✅ Integration tests pass
- ✅ Code review approved
- ✅ Model outputs validated

**Staging → Production:**
- ✅ All staging tests pass
- ✅ Load testing complete
- ✅ Soak period complete (24-48 hours)
- ✅ Security scan passed
- ✅ Change request approved
- ✅ Rollback plan documented

### Automated Promotion Script

Use the promotion script to enforce validation gates:

```bash
# Validates staging health before promoting
./scripts/vllm/promote-deployment.sh llama-2-7b
```

See [Rollout Workflow](./rollout-workflow.md) for detailed promotion procedures.

## Monitoring and Alerting

### Environment-Specific Dashboards

Create separate Grafana dashboards per environment:

- `vLLM Deployments - Development`
- `vLLM Deployments - Staging`
- `vLLM Deployments - Production`

### Environment-Specific Alerts

Configure different alert thresholds per environment:

```yaml
# Development: Relaxed alerts
groups:
  - name: vllm-development
    rules:
      - alert: HighErrorRate
        expr: rate(vllm_errors_total{environment="development"}[5m]) > 0.10
        for: 30m  # 30 minute window

# Production: Strict alerts
groups:
  - name: vllm-production
    rules:
      - alert: HighErrorRate
        expr: rate(vllm_errors_total{environment="production"}[5m]) > 0.01
        for: 5m  # 5 minute window
```

## Disaster Recovery

### Environment Recovery Priority

In case of cluster-wide issues, recover environments in order:

1. **Production** - Highest priority (customer-facing)
2. **Staging** - Medium priority (blocks deployments)
3. **Development** - Lowest priority (blocks feature work)

### Backup Strategy

**Production:**
- Model registry backed up hourly
- Configuration backed up to Git (automatic)
- Helm release history retained (50 revisions)

**Staging:**
- Model registry backed up daily
- Configuration backed up to Git (automatic)
- Helm release history retained (20 revisions)

**Development:**
- No automatic backups (can be rebuilt from Git)
- Helm release history retained (10 revisions)

## Cost Optimization

### Resource Allocation by Environment

Allocate GPU resources based on environment needs:

| Environment | GPU Nodes | GPU Type | Cost Impact |
|-------------|-----------|----------|-------------|
| Development | 1-2 | Shared pool | Low |
| Staging | 1 | Dedicated | Medium |
| Production | 3-5 | Dedicated | High |

### Autoscaling Configuration

**Development:** No autoscaling (static 1 replica)
**Staging:** No autoscaling (static 1 replica)
**Production:** HPA enabled (min: 2, max: 5 replicas)

### Cost Tags

Tag resources for cost tracking:

```yaml
metadata:
  labels:
    cost-center: "ai-platform"
    environment: "production"
    team: "ml-infrastructure"
```

## Security Considerations

### Secrets Management

Separate secrets per environment:

```bash
# Development
kubectl create secret generic vllm-dev-secrets \
  --from-literal=api-key=dev-key-xxx \
  -n system

# Production
kubectl create secret generic vllm-prod-secrets \
  --from-literal=api-key=prod-key-xxx \
  -n system
```

### Network Segmentation

Use NetworkPolicies to enforce:
- Development models cannot reach production databases
- Production models cannot reach development services
- All environments can only reach their designated API Router

### Audit Logging

Enable audit logging for production deployments:

```yaml
# Production audit policy
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: RequestResponse
    namespaces: ["system"]
    resources:
      - group: ""
        resources: ["pods", "services"]
      - group: "apps"
        resources: ["deployments"]
    omitStages:
      - "RequestReceived"
```

## Best Practices

1. **Always test in development first**
   - Never deploy directly to staging or production

2. **Validate in staging before production**
   - Staging should mirror production configuration
   - Run full test suite in staging

3. **Use GitOps for all environments**
   - All changes go through Git
   - ArgoCD manages synchronization
   - Automatic sync for dev/staging, manual for production

4. **Monitor environment drift**
   - Alert on manual changes in production
   - Require change requests for production
   - Audit all production deployments

5. **Maintain environment parity**
   - Keep staging as close to production as possible
   - Use same GPU types and configurations
   - Mirror resource limits and quotas

6. **Document environment-specific configurations**
   - Maintain values files in Git
   - Document differences between environments
   - Review configurations during promotion

7. **Test rollback procedures**
   - Regularly test rollback in development
   - Validate rollback in staging before production changes
   - Document rollback procedures

8. **Enforce promotion gates**
   - Automate validation checks
   - Require approvals for production
   - Document decision criteria

## See Also

- [Rollout Workflow](./rollout-workflow.md)
- [Rollback Workflow](./rollback-workflow.md)
- [Registration Workflow](./vllm-registration-workflow.md)
- [Deployment Workflow](./deployment-workflow.md)
- [ArgoCD Application Template](../argocd-apps/vllm-deployment-template.yaml)
