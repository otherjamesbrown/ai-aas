# Research: Model Inference Deployment

**Feature**: `010-vllm-deployment`  
**Date**: 2025-01-27  
**Purpose**: Document technical decisions, alternatives considered, and rationale for vLLM deployment architecture

## Technology Choices

### 1. Deployment Method: Helm Charts + ArgoCD

**Decision**: Use Helm charts for declarative vLLM deployment with ArgoCD GitOps reconciliation.

**Rationale**:
- Aligns with constitution principle: "Declarative Infrastructure & GitOps"
- Existing platform pattern (all services use Helm charts)
- Git as source of truth enables auditability and rollback
- ArgoCD provides automatic reconciliation and drift detection
- Environment-specific values via `values-{environment}.yaml` files

**Alternatives Considered**:
- **Direct kubectl apply**: Rejected - violates GitOps principle, no drift detection
- **Kustomize overlays**: Considered but Helm provides better templating and dependency management
- **Terraform Kubernetes provider**: Rejected - Terraform is for infrastructure, not application deployment
- **Custom operator**: Rejected - over-engineering for initial implementation, can be added later if needed

**Implementation Notes**:
- Chart location: `infra/helm/charts/vllm-deployment/`
- Environment values: `values-development.yaml`, `values-staging.yaml`, `values-production.yaml`
- ArgoCD Application manifests reference Git repository

---

### 2. Health Check Strategy

**Decision**: Use vLLM's built-in `/health` and `/ready` endpoints with Kubernetes liveness/readiness probes.

**Rationale**:
- vLLM provides native health endpoints (no custom implementation needed)
- Kubernetes probes enable automatic pod restart and traffic routing
- Aligns with constitution observability requirements
- Matches existing service patterns (API Router, User-Org Service use similar probes)

**Probe Configuration**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 60  # Allow model loading time
  periodSeconds: 30
  timeoutSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8000
  initialDelaySeconds: 30  # Shorter delay for readiness
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

**Alternatives Considered**:
- **Custom health check script**: Rejected - vLLM endpoints are sufficient
- **TCP probe**: Rejected - HTTP probe provides more meaningful health status
- **Exec probe**: Rejected - adds complexity, HTTP is cleaner

**Implementation Notes**:
- Initial delay accounts for model loading (5-10 minutes for large models)
- Readiness probe has shorter delay to enable faster traffic routing once model is loaded
- Failure thresholds prevent premature restarts during initialization

---

### 3. Model Registry Integration

**Decision**: Extend existing `model_registry_entries` table with deployment metadata (endpoint URL, health status, environment).

**Rationale**:
- Reuses existing database schema and infrastructure
- Maintains single source of truth for model metadata
- Enables API Router to query deployment status for routing decisions
- Aligns with existing data model patterns

**Schema Extension**:
```sql
ALTER TABLE model_registry_entries ADD COLUMN IF NOT EXISTS deployment_endpoint TEXT;
ALTER TABLE model_registry_entries ADD COLUMN IF NOT EXISTS deployment_status TEXT CHECK (deployment_status IN ('pending', 'deploying', 'ready', 'degraded', 'failed'));
ALTER TABLE model_registry_entries ADD COLUMN IF NOT EXISTS deployment_environment TEXT;
ALTER TABLE model_registry_entries ADD COLUMN IF NOT EXISTS deployment_namespace TEXT;
ALTER TABLE model_registry_entries ADD COLUMN IF NOT EXISTS last_health_check_at TIMESTAMPTZ;
```

**Alternatives Considered**:
- **Separate deployment table**: Rejected - creates data duplication and sync complexity
- **Kubernetes-only state**: Rejected - need database for API Router queries and cross-service access
- **ConfigMap/Secret storage**: Rejected - not queryable, violates stateless service principle

**Implementation Notes**:
- Migration adds nullable columns (backward compatible)
- Deployment status updated via health check service or management API
- API Router queries this table to determine available backends

---

### 4. Endpoint Naming Convention

**Decision**: Use predictable endpoint pattern: `{model-name}-{environment}.{namespace}.svc.cluster.local:8000`

**Rationale**:
- Predictable naming enables API Router to construct endpoint URLs
- Environment separation via namespace
- Kubernetes DNS resolution (no external DNS required for internal routing)
- Aligns with FR-002: "predictable endpoints per environment"

**Pattern**:
- Development: `llama-7b-development.system.svc.cluster.local:8000`
- Staging: `llama-7b-staging.system.svc.cluster.local:8000`
- Production: `llama-7b-production.system.svc.cluster.local:8000`

**Alternatives Considered**:
- **Random service names**: Rejected - not predictable, violates FR-002
- **External DNS**: Rejected - adds complexity, internal routing doesn't need it
- **Service mesh (Istio)**: Rejected - over-engineering for initial implementation

**Implementation Notes**:
- Service name in Helm chart: `{{ .Values.modelName }}-{{ .Values.environment }}`
- Namespace: `system` (GPU workloads) or environment-specific
- Port: 8000 (vLLM default)

---

### 5. Rollout/Rollback Strategy

**Decision**: Use Helm release versioning with ArgoCD sync waves and manual approval gates for production.

**Rationale**:
- Helm maintains release history (enables rollback via `helm rollback`)
- ArgoCD sync waves allow staged rollouts (deploy → validate → promote)
- Manual approval gates for production reduce risk
- Git-based rollback (revert commit → ArgoCD syncs)

**Rollout Flow**:
1. Deploy to `development` namespace
2. Validate health and test completion endpoint
3. Promote to `staging` (same validation)
4. Manual approval for `production`
5. ArgoCD syncs production deployment

**Rollback Flow**:
1. Identify previous stable release: `helm history {release-name}`
2. Rollback: `helm rollback {release-name} {revision}` OR revert Git commit
3. ArgoCD reconciles to previous state
4. Verify previous version is serving traffic

**Alternatives Considered**:
- **Blue-green deployment**: Considered but adds complexity (separate service sets)
- **Canary deployment**: Considered but requires traffic splitting (future enhancement)
- **Manual kubectl rollback**: Rejected - not GitOps-compliant, no audit trail

**Implementation Notes**:
- Helm release names: `{model-name}-{environment}`
- ArgoCD sync policy: `manual` for production, `auto` for development/staging
- Rollback target: ≤5 minutes per SC-004

---

### 6. Environment Separation

**Decision**: Use Kubernetes namespaces (`development`, `staging`, `production`) with NetworkPolicies for isolation.

**Rationale**:
- Namespace isolation prevents cross-environment access
- NetworkPolicies enforce zero-trust boundaries (constitution requirement)
- Resource quotas per namespace prevent resource contention
- Matches existing platform pattern

**NetworkPolicy Pattern**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vllm-deployment-policy
spec:
  podSelector:
    matchLabels:
      app: vllm-deployment
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: system  # API Router namespace
    ports:
    - protocol: TCP
      port: 8000
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: system  # Allow egress to system services
```

**Alternatives Considered**:
- **Separate clusters**: Rejected - adds operational overhead, single cluster is sufficient
- **Virtual clusters**: Rejected - over-engineering for current scale
- **No isolation**: Rejected - violates security and environment separation requirements

**Implementation Notes**:
- Namespace labels: `environment: {development|staging|production}`
- NetworkPolicies deny by default, allow explicit ingress/egress
- Resource quotas prevent GPU resource exhaustion

---

### 7. Resource Management

**Decision**: Use Kubernetes resource requests/limits with node selectors for GPU node pool.

**Rationale**:
- GPU node pool (`g1-gpu-rtx6000`) dedicated for vLLM workloads
- Resource limits prevent resource exhaustion
- Node selectors ensure pods schedule on GPU nodes
- HPA (Horizontal Pod Autoscaler) for scaling (future enhancement)

**Resource Configuration**:
```yaml
resources:
  requests:
    nvidia.com/gpu: 1
    memory: "32Gi"
    cpu: "8"
  limits:
    nvidia.com/gpu: 1
    memory: "48Gi"
    cpu: "16"

nodeSelector:
  node-type: gpu

tolerations:
- key: gpu-workload
  operator: Equal
  value: "true"
  effect: NoSchedule
```

**Alternatives Considered**:
- **CPU-only nodes**: Rejected - vLLM requires GPU for inference
- **Shared GPU nodes**: Considered but dedicated pool provides better isolation
- **Dynamic GPU allocation**: Future enhancement (multi-model per GPU)

**Implementation Notes**:
- GPU requests: 1 GPU per model instance (7B-70B parameter models)
- Memory limits account for model weights + KV cache
- Node selectors ensure scheduling on GPU pool

---

### 8. Model Registration for Routing

**Decision**: Extend API Router's routing configuration to query `model_registry_entries` for active deployments.

**Rationale**:
- Reuses existing API Router routing logic
- Database query provides real-time deployment status
- Enables enable/disable states (FR-003 requirement)
- No new service needed (admin-cli can manage registrations)

**Registration Flow**:
1. Deploy model via Helm chart
2. Update `model_registry_entries` with endpoint and status
3. API Router queries registry on routing decisions
4. Disable: Set `deployment_status = 'disabled'` in database

**Alternatives Considered**:
- **Separate routing service**: Rejected - adds complexity, API Router already handles routing
- **Kubernetes Service discovery**: Considered but database provides richer metadata (cost, status)
- **ConfigMap-based routing**: Rejected - not dynamic, requires pod restart for changes

**Implementation Notes**:
- API Router queries: `SELECT * FROM model_registry_entries WHERE deployment_status = 'ready' AND model_name = $1`
- Registration propagation: ≤2 minutes per SC-003 (database update + API Router cache TTL)
- Disabled models: API Router returns 404 or 503 with clear error message

---

## Open Questions Resolved

### Q1: Should we use a separate management service for deployments?

**Answer**: No, initially. Admin CLI can manage Helm deployments and database updates. If management APIs are needed later, create `services/model-deployment-service/` following existing patterns.

### Q2: How to handle model versioning/revisions?

**Answer**: Use `revision` field in `model_registry_entries` table. Each revision can have separate deployment. API Router routes to latest revision by default, or specific revision via request parameter.

### Q3: Should deployments be automated or manual?

**Answer**: Hybrid approach:
- Development: Automated via ArgoCD auto-sync
- Staging: Automated with validation gates
- Production: Manual approval required (ArgoCD manual sync)

### Q4: How to handle GPU resource contention?

**Answer**: 
- Resource quotas per namespace
- Node selectors ensure GPU pool scheduling
- Manual scaling of GPU node pool if needed
- Future: Implement queue system for deployment requests

---

## Dependencies & Integration Points

### Existing Services
- **API Router Service**: Queries model registry for routing decisions
- **User-Org Service**: Manages organizations (model registry entries can be org-scoped)
- **PostgreSQL**: Stores model registry and deployment metadata
- **Redis**: Optional caching for health check results

### Infrastructure
- **Kubernetes (LKE)**: Container orchestration
- **Helm**: Package management
- **ArgoCD**: GitOps reconciliation
- **NGINX Ingress**: Optional external access (most access via API Router)

### External Dependencies
- **vLLM**: Python inference engine (container image)
- **HuggingFace**: Model repository (for model downloads)
- **GPU Node Pool**: Hardware resource

---

## Performance Considerations

### Model Loading Time
- **7B models**: ~2-3 minutes
- **13B models**: ~4-5 minutes
- **70B models**: ~8-10 minutes
- **Mitigation**: Readiness probe `initialDelaySeconds` accounts for load time

### Inference Latency
- **Target**: P50 <100ms, P95 <300ms (constitution requirement)
- **Factors**: Model size, request length, GPU memory bandwidth
- **Monitoring**: Prometheus metrics from vLLM

### Deployment Time
- **Target**: ≤10 minutes (95th percentile) per SC-001
- **Components**: Helm install, pod scheduling, model download, model loading
- **Optimization**: Pre-pull images, use node-local storage for models (future)

---

## Security Considerations

### Network Isolation
- NetworkPolicies enforce zero-trust boundaries
- API Router → vLLM pod communication explicitly allowed
- No external ingress by default (internal service only)

### Secrets Management
- Model access tokens (HuggingFace) in Kubernetes Secrets
- Secrets synced via ArgoCD Sealed Secrets
- Never stored in Git

### RBAC
- Deployment operations require admin role
- Model registration requires operator role
- API Router queries are read-only (no RBAC needed for queries)

---

## Monitoring & Observability

### Metrics
- vLLM exposes Prometheus metrics at `/metrics`
- Deployment status metrics (ready, degraded, failed)
- Model inference metrics (latency, throughput, error rate)

### Logs
- vLLM logs to stdout (captured by Loki)
- Deployment logs (Helm, ArgoCD)
- Health check logs

### Dashboards
- Model deployment status dashboard
- Inference performance dashboard
- GPU utilization dashboard

### Alerts
- Deployment failure alerts
- Health check failure alerts
- GPU resource exhaustion alerts

---

## Future Enhancements

1. **Multi-model per GPU**: Share GPU across multiple smaller models
2. **Auto-scaling**: HPA based on inference request queue depth
3. **Canary deployments**: Gradual traffic shifting for new model versions
4. **Model caching**: Node-local storage for faster model loading
5. **Management API service**: Dedicated service for deployment operations
6. **Model versioning UI**: Web UI for model deployment management

---

## References

- [vLLM Documentation](https://docs.vllm.ai/)
- [Kubernetes Health Checks](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Helm Best Practices](https://helm.sh/docs/chart_best_practices/)
- [ArgoCD GitOps](https://argo-cd.readthedocs.io/)
- Platform Constitution: `memory/constitution.md`
- Existing Helm Charts: `services/*/deployments/helm/`

