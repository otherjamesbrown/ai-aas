# Knative Configuration

---
last_updated: 2025-12-08
document_type: guide
last_verified: 2025-12-08
verification_command: "kubectl get deployment -n knative-serving"
---

## Overview

Knative is a Kubernetes-based platform that provides building blocks for serverless workloads. The AI-AAS platform uses **Knative Serving** to power model serving via KServe InferenceServices.

### Why Knative in AI-AAS

Knative Serving provides critical capabilities for ML model serving:

1. **Auto-scaling**: Scale model deployments from zero to handle traffic spikes
2. **Traffic splitting**: Blue-green and canary deployments for model updates
3. **Revision management**: Automatic versioning of model deployments
4. **Request-based routing**: Route traffic based on headers, paths, or traffic splits
5. **Integration with Istio**: Service mesh capabilities for observability and security

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Istio Ingress Gateway                   │
│                     (istio-system namespace)                │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                 Knative Serving (knative-serving)           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Controller   │  │ Autoscaler   │  │ Activator    │      │
│  │ (reconcile)  │  │ (scale pods) │  │ (scale-to-0) │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│  ┌──────────────┐  ┌──────────────┐                        │
│  │ Webhook      │  │ net-istio    │                        │
│  │ (validate)   │  │ (networking) │                        │
│  └──────────────┘  └──────────────┘                        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│          KServe InferenceServices (development)             │
│  ┌──────────────────────────────────────────────────┐       │
│  │ Knative Service (ksvc)                           │       │
│  │  ├─ Revision 00001 (inactive)                    │       │
│  │  └─ Revision 00002 (active, serving traffic)     │       │
│  │      └─ Pod: model-00002-deployment-xyz          │       │
│  │          ├─ storage-initializer (init)           │       │
│  │          ├─ kserve-container (vLLM)              │       │
│  │          └─ queue-proxy (Knative sidecar)        │       │
│  └──────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

## Components

Knative Serving consists of several core components deployed in the `knative-serving` namespace:

| Component | Purpose | Replicas |
|-----------|---------|----------|
| `controller` | Reconciles Knative Service resources | 1 |
| `autoscaler` | Calculates and applies scaling decisions | 1 |
| `activator` | Handles scale-from-zero requests | 1 |
| `webhook` | Validates and mutates Knative resources | 1 |
| `net-istio-controller` | Manages Istio resources for routing | 1 |
| `net-istio-webhook` | Validates Istio networking configuration | 1 |

### Verification

```bash
# Check all Knative components
kubectl get deployment -n knative-serving

# Check component health
kubectl get pods -n knative-serving

# Expected output:
# NAME                                    READY   STATUS    RESTARTS   AGE
# activator-xxx                           1/1     Running   0          13d
# autoscaler-xxx                          1/1     Running   0          13d
# controller-xxx                          1/1     Running   0          13d
# net-istio-controller-xxx                1/1     Running   0          13d
# net-istio-webhook-xxx                   1/1     Running   0          13d
# webhook-xxx                             1/1     Running   0          13d
```

## Configuration

Knative Serving is configured via ConfigMaps in the `knative-serving` namespace.

### Configuration Files Location

**Source of Truth**: All Knative configuration is stored in:
- `infra/k8s/knative-serving/` - Core Knative Serving ConfigMaps
- `infra/k8s/knative/net-istio/` - Istio networking layer

**ArgoCD Applications**:
- `gitops/clusters/<env>/apps/knative-config.yaml` - Deploys core ConfigMaps
- `gitops/clusters/<env>/apps/knative-serving.yaml` - Deploys net-istio networking

### Key ConfigMaps

```bash
# List all Knative ConfigMaps
kubectl get configmap -n knative-serving
```

| ConfigMap | Purpose | Key Settings |
|-----------|---------|--------------|
| `config-domain` | Default domain for Knative Services | `dev.ai-aas.local` |
| `config-network` | Networking configuration | Ingress class: `istio` |
| `config-autoscaler` | Autoscaling behavior | Container concurrency, scale targets |
| `config-deployment` | Deployment settings | Queue proxy resources, timeouts |
| `config-defaults` | Default values for Services | Revision timeout, resource limits |
| `config-features` | Feature flags | Enable/disable experimental features |
| `config-istio` | Istio integration | Gateway configuration |

### Domain Configuration

**File**: `infra/k8s/knative-serving/config-domain.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-domain
  namespace: knative-serving
data:
  # Domain for development environment
  dev.ai-aas.local: ""
```

This sets the default domain suffix for all Knative Services. For example, a Service named `mistral-7b-development-predictor` in namespace `development` will be accessible at:
```
http://mistral-7b-development-predictor.development.dev.ai-aas.local
```

**Customization**:
- Development: `dev.ai-aas.local`
- Staging: `staging.ai-aas.local` (recommended)
- Production: `prod.ai-aas.local` or custom domain

### Network Configuration

**File**: `infra/k8s/knative-serving/config-network.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-network
  namespace: knative-serving
data:
  # Use Istio as the ingress
  ingress-class: "istio.ingress.networking.knative.dev"

  # Enable auto TLS
  auto-tls: "Disabled"
```

**Key Settings**:
- `ingress-class`: Specifies Istio as the networking layer (required for KServe)
- `auto-tls`: Disabled because we manage TLS at the Istio Ingress Gateway level

### Autoscaler Configuration

View current autoscaler settings:

```bash
kubectl get configmap config-autoscaler -n knative-serving -o yaml
```

**Key Autoscaling Settings** (defaults):
- `container-concurrency-target-percentage`: `70` - Target 70% of max concurrency
- `container-concurrency-target-default`: `100` - Default concurrent requests per pod
- `scale-to-zero-grace-period`: `30s` - Grace period before scaling to zero
- `scale-to-zero-pod-retention-period`: `0s` - How long to retain pods at zero
- `stable-window`: `60s` - Window for stable scaling decisions

**Customization Example**:

To prevent scale-to-zero for production models:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-autoscaler
  namespace: knative-serving
data:
  enable-scale-to-zero: "false"  # Disable scale-to-zero globally
  scale-to-zero-grace-period: "5m"  # Or increase grace period
```

### Deployment Configuration

View current deployment settings:

```bash
kubectl get configmap config-deployment -n knative-serving -o yaml
```

**Key Deployment Settings**:
- `progress-deadline`: `600s` - Max time to wait for deployment readiness
- `queue-sidecar-cpu-request`: `25m` - Queue proxy CPU request
- `queue-sidecar-memory-request`: `400Mi` - Queue proxy memory request

**Queue Proxy**: Every Knative Service pod includes a `queue-proxy` sidecar that:
- Routes requests to the user container
- Reports metrics to the autoscaler
- Enforces concurrency limits
- Handles graceful shutdown

## Integration with KServe

KServe InferenceServices create Knative Services under the hood. When you deploy a model via KServe:

1. **KServe Controller** creates an InferenceService resource
2. **InferenceService** creates a Knative Service for the predictor
3. **Knative Service** creates:
   - A Configuration (immutable snapshot)
   - A Revision (versioned deployment)
   - A Route (traffic routing rules)
4. **Knative Controller** creates a Deployment with pods
5. **Knative Autoscaler** monitors and scales the Deployment

### Example: Viewing Knative Resources for a Model

```bash
# List all Knative Services
kubectl get ksvc -A

# View a specific Knative Service
kubectl get ksvc mistral-7b-instruct-development-predictor -n development -o yaml

# View revisions for a service
kubectl get revisions -n development -l serving.knative.dev/service=mistral-7b-instruct-development-predictor

# View routes
kubectl get route mistral-7b-instruct-development-predictor -n development -o yaml
```

### Traffic Splitting

Knative supports traffic splitting across revisions for canary deployments:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-model-predictor
spec:
  traffic:
    - revisionName: my-model-predictor-00001
      percent: 90  # 90% to stable revision
    - revisionName: my-model-predictor-00002
      percent: 10  # 10% to canary revision
```

**Note**: KServe manages this automatically during canary rollouts. See KServe documentation for canary deployment strategies.

## Istio Integration

Knative uses Istio for networking via the `net-istio` controller.

### Istio Gateway

**File**: `infra/k8s/knative/net-istio/net-istio.yaml`

The net-istio installation creates a Knative-specific Istio Gateway:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: knative-ingress-gateway
  namespace: knative-serving
spec:
  selector:
    istio: ingressgateway  # Use Istio's ingress gateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "*"
```

This gateway handles all incoming requests to Knative Services.

### VirtualServices

Knative automatically creates Istio VirtualServices for each Knative Service:

```bash
# List VirtualServices created by Knative
kubectl get virtualservice -n development -l networking.knative.dev/ingress=true
```

These VirtualServices:
- Route traffic based on host headers
- Split traffic across revisions
- Handle retries and timeouts

## Troubleshooting

### Knative Service Not Ready

**Symptoms**: Knative Service shows `Ready=False`

```bash
# Check service status
kubectl get ksvc -n development

# Describe the service
kubectl describe ksvc <service-name> -n development

# Common issues:
# - "RevisionFailed" - Check revision and pod logs
# - "ConfigurationNotReady" - Check configuration spec
# - "RouteNotReady" - Check Istio gateway and routes
```

**Check Revision Status**:

```bash
# List revisions
kubectl get revisions -n development

# Describe failing revision
kubectl describe revision <revision-name> -n development

# Check pod logs
kubectl logs -n development -l serving.knative.dev/revision=<revision-name>
```

### Scale-to-Zero Issues

**Symptoms**: Service doesn't scale down or takes too long

```bash
# Check autoscaler logs
kubectl logs -n knative-serving -l app=autoscaler

# Check if traffic is preventing scale-down
kubectl get podautoscaler -n development
```

**Force scale to zero** (for testing):

```bash
# Annotate the Knative Service
kubectl annotate ksvc <service-name> -n development \
  autoscaling.knative.dev/min-scale=0 --overwrite
```

### Activator Issues

**Symptoms**: Requests fail when service is at zero

The activator component handles requests during scale-from-zero:

```bash
# Check activator logs
kubectl logs -n knative-serving -l app=activator

# Check activator endpoints
kubectl get endpoints activator-service -n knative-serving
```

### Istio Integration Issues

**Symptoms**: Services not accessible, 404 errors

```bash
# Check Istio gateway
kubectl get gateway knative-ingress-gateway -n knative-serving

# Check VirtualServices
kubectl get virtualservice -n development

# Check Istio ingress gateway logs
kubectl logs -n istio-system -l app=istio-ingressgateway
```

### Configuration Changes Not Applied

Knative watches ConfigMaps and reloads automatically, but you can restart components:

```bash
# Restart controller to reload config
kubectl rollout restart deployment controller -n knative-serving

# Restart autoscaler
kubectl rollout restart deployment autoscaler -n knative-serving
```

## Common Operations

### View Knative Service Details

```bash
# Get URL for a service
kubectl get ksvc <service-name> -n development -o jsonpath='{.status.url}'

# Get current traffic split
kubectl get ksvc <service-name> -n development -o jsonpath='{.status.traffic}'

# Get active revision
kubectl get ksvc <service-name> -n development -o jsonpath='{.status.latestReadyRevisionName}'
```

### Check Autoscaling Metrics

```bash
# View PodAutoscaler resources
kubectl get podautoscaler -n development

# Describe autoscaler for a service
kubectl describe podautoscaler <service-name> -n development

# View current pod count and desired replicas
kubectl get podautoscaler <service-name> -n development \
  -o jsonpath='{.status.actualScale} / {.status.desiredScale}'
```

### Debug Networking

```bash
# Test internal service DNS
kubectl run test-pod --rm -it --image=curlimages/curl -- sh
# Inside pod:
curl http://mistral-7b-development-predictor.development.dev.ai-aas.local

# Check if route exists
kubectl get route <service-name> -n development -o yaml

# Verify Istio VirtualService
kubectl get virtualservice -n development -l serving.knative.dev/service=<service-name>
```

### Update Knative Configuration (GitOps)

**NEVER** edit ConfigMaps directly. Always use GitOps:

1. **Edit configuration file**:
   ```bash
   # Edit domain config
   vim infra/k8s/knative-serving/config-domain.yaml

   # Or network config
   vim infra/k8s/knative-serving/config-network.yaml
   ```

2. **Commit and push**:
   ```bash
   git add infra/k8s/knative-serving/
   git commit -m "Update Knative domain configuration"
   git push origin develop
   ```

3. **ArgoCD syncs automatically** (development) or manually sync:
   ```bash
   argocd app sync knative-config-development
   ```

4. **Verify changes applied**:
   ```bash
   kubectl get configmap config-domain -n knative-serving -o yaml
   ```

## Performance Tuning

### For High-Throughput Models

Increase container concurrency and autoscaling targets:

```yaml
# In InferenceService spec (via KServe)
spec:
  predictor:
    containerConcurrency: 10  # Max concurrent requests per pod
    scaleTarget: 10           # Target concurrent requests
    scaleMetric: concurrency
```

### For Large Models (High Memory)

Adjust queue proxy resources to prevent OOM:

```yaml
# In config-deployment ConfigMap
data:
  queue-sidecar-memory-request: "1Gi"
  queue-sidecar-memory-limit: "2Gi"
```

### For Production Stability

Disable scale-to-zero and set minimum replicas:

```yaml
# In InferenceService annotations (via KServe)
metadata:
  annotations:
    autoscaling.knative.dev/min-scale: "1"
    autoscaling.knative.dev/max-scale: "10"
```

## Deployment

Knative is deployed via ArgoCD using two applications per environment:

**Development**:
- `gitops/clusters/development/apps/knative-config.yaml` - Core ConfigMaps
- `gitops/clusters/development/apps/knative-serving.yaml` - net-istio networking

**Staging**:
- `gitops/clusters/staging/apps/knative-config-staging.yaml` - Core ConfigMaps (targetRevision: staging)
- `gitops/clusters/staging/apps/knative-serving.yaml` - net-istio networking (targetRevision: staging)

**Manual Bootstrap** (if needed):

```bash
# Apply ArgoCD applications
kubectl apply -f gitops/clusters/development/apps/knative-config.yaml
kubectl apply -f gitops/clusters/development/apps/knative-serving.yaml

# Wait for Knative to be ready
kubectl wait --for=condition=Ready pods -n knative-serving -l app=controller --timeout=5m

# Verify deployment
kubectl get deployment -n knative-serving
```

## References

- [Knative Documentation](https://knative.dev/docs/)
- [Knative Serving API Reference](https://knative.dev/docs/reference/api/serving-api/)
- [Knative Autoscaling](https://knative.dev/docs/serving/autoscaling/)
- [KServe Documentation](https://kserve.github.io/website/)
- [Istio Documentation](https://istio.io/latest/docs/)

## Related Documentation

- [Infrastructure Overview](infrastructure-overview.md) - Overall platform architecture
- [Certificate Architecture](certificate-architecture.md) - TLS and webhook certificates
- [Endpoints and URLs](endpoints-and-urls.md) - Service endpoint URLs
- [ArgoCD Testing Guide](argocd-testing-guide.md) - ArgoCD sync and troubleshooting
- [KServe Migration Deployment](../runbooks/kserve-migration-deployment.md) - KServe deployment runbook
