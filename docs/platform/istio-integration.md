# Istio Integration

---
last_updated: 2025-12-08
document_type: overview
last_verified: 2025-12-08
verification_command: "kubectl get pods -n istio-system"
---

## Overview

Istio is a service mesh that provides traffic management, security, and observability capabilities for microservices. The AI-AAS platform uses Istio as the **networking layer for Knative Serving**, which in turn powers KServe InferenceServices for ML model deployment.

### Why Istio in AI-AAS

Istio provides critical capabilities for the platform:

1. **Ingress Gateway**: Single entry point for external traffic to Knative Services
2. **Traffic Management**: Intelligent routing, load balancing, and traffic splitting for model deployments
3. **Observability**: Distributed tracing, metrics, and access logs for model serving requests
4. **Security**: mTLS for internal service-to-service communication (when enabled)
5. **Knative Integration**: Required networking layer for Knative Serving and KServe

**Key Characteristic**: The platform uses Istio in **sidecar-less mode** for most workloads. Istio provides centralized ingress and routing without injecting sidecars into every pod, reducing overhead.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    External Traffic (HTTP/HTTPS)                 │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│              Istio Ingress Gateway (istio-system)                │
│  ┌────────────────────────────────────────────────────────┐      │
│  │ istio-ingressgateway-development (LoadBalancer)        │      │
│  │   - Port 80 (HTTP)                                     │      │
│  │   - Port 443 (HTTPS)                                   │      │
│  │   - Port 8081 (Cluster-local traffic)                  │      │
│  │   Label: istio=ingressgateway                          │      │
│  └────────────────────────────────────────────────────────┘      │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                      Istio Control Plane                         │
│  ┌──────────────────────────────────────────────────────┐        │
│  │ istiod (istio-system)                                │        │
│  │   - Pilot: Service discovery, configuration          │        │
│  │   - Citadel: Certificate authority (when mTLS on)    │        │
│  │   - Galley: Configuration validation                 │        │
│  │   - Telemetry v2: Metrics and tracing                │        │
│  └──────────────────────────────────────────────────────┘        │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                  Istio Gateway Resources                         │
│  ┌─────────────────────┐      ┌──────────────────────┐           │
│  │ knative-ingress-    │      │ knative-local-       │           │
│  │ gateway             │      │ gateway              │           │
│  │ (External traffic)  │      │ (Cluster traffic)    │           │
│  │ Port: 80            │      │ Port: 8081           │           │
│  └─────────────────────┘      └──────────────────────┘           │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│            Knative Serving (net-istio controller)                │
│  ┌────────────────────────────────────────────────────────┐      │
│  │ VirtualServices (auto-created by Knative)              │      │
│  │   - Route traffic based on Host headers               │      │
│  │   - Split traffic across revisions                    │      │
│  │   - Support canary deployments                        │      │
│  └────────────────────────────────────────────────────────┘      │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│          KServe InferenceServices (Model Workloads)              │
│  ┌────────────────────────────────────────────────────────┐      │
│  │ Knative Service (ksvc) → Pods                          │      │
│  │   - No Istio sidecar injection                         │      │
│  │   - Queue-proxy sidecar (Knative, not Istio)           │      │
│  │   - Direct access via VirtualServices                  │      │
│  └────────────────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────────────────┘
```

## Components

Istio is deployed in the `istio-system` namespace with three main components:

### 1. istio-base

**ArgoCD Application**: `gitops/clusters/<env>/apps/istio.yaml` (first Application)

**Purpose**: Installs base Istio CRDs (Custom Resource Definitions) and cluster-wide resources.

**Resources**:
- CustomResourceDefinitions: `Gateway`, `VirtualService`, `DestinationRule`, `ServiceEntry`, etc.
- ClusterRoles and ClusterRoleBindings
- ValidatingWebhookConfiguration for Istio resource validation

**Chart**: `istio-release.storage.googleapis.com/charts/base` version `1.19.0`

### 2. istiod (Control Plane)

**ArgoCD Application**: `gitops/clusters/<env>/apps/istio.yaml` (second Application)

**Purpose**: Istio control plane that manages configuration, service discovery, and telemetry.

**Resources**:
- **Deployment**: `istiod` (1 replica)
  - CPU: 100m request, 500m limit
  - Memory: 512Mi request, 2Gi limit
- **Service**: `istiod` (ClusterIP) on ports 15010, 15012, 443, 15014
- **ConfigMaps**: Istio configuration
- **Webhooks**: Sidecar injection (disabled for most namespaces)

**Key Configuration** (from ArgoCD app):

```yaml
# Minimal profile for reduced overhead
pilot:
  resources:
    requests:
      cpu: 100m
      memory: 512Mi
    limits:
      cpu: 500m
      memory: 2Gi

# Enable telemetry and metrics
telemetry:
  enabled: true
  v2:
    enabled: true

meshConfig:
  accessLogFile: /dev/stdout  # Log all requests
  enableTracing: true
  defaultConfig:
    tracing:
      sampling: 1.0  # 100% trace sampling

# Sidecar proxy resources (when injection enabled)
global:
  proxy:
    resources:
      requests:
        cpu: 50m
        memory: 128Mi
      limits:
        cpu: 200m
        memory: 256Mi
```

**Verification**:

```bash
# Check istiod status
kubectl get deployment istiod -n istio-system

# Check istiod logs
kubectl logs -n istio-system -l app=istiod

# Verify istiod is processing configuration
kubectl logs -n istio-system -l app=istiod | grep "Push Status"
```

### 3. istio-ingressgateway (Data Plane)

**ArgoCD Application**: `gitops/clusters/<env>/apps/istio.yaml` (third Application)

**Purpose**: LoadBalancer that receives external traffic and routes to internal services based on Gateway/VirtualService rules.

**Resources**:
- **Deployment**: `istio-ingressgateway-<environment>` (1 replica)
  - CPU: 100m request, 500m limit
  - Memory: 128Mi request, 512Mi limit
- **Service**: `istio-ingressgateway-<environment>` (LoadBalancer)
  - Port 80 (HTTP) → 8080
  - Port 443 (HTTPS) → 8443
  - Port 8081 (cluster-local) → 8081
- **Label**: `istio: ingressgateway` (critical for Gateway selector)

**Key Configuration** (from ArgoCD app):

```yaml
# Critical label for Knative compatibility
labels:
  istio: ingressgateway

service:
  type: LoadBalancer
  ports:
    - name: http
      port: 80
      targetPort: 8080
    - name: https
      port: 443
      targetPort: 8443
    # Port 8081 for knative-local-gateway (internal cluster traffic)
    - name: http-internal
      port: 8081
      targetPort: 8081

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

**External IP**:

```bash
# Get LoadBalancer IP
kubectl get svc istio-ingressgateway-development -n istio-system

# Example output:
# NAME                               TYPE           EXTERNAL-IP     PORT(S)
# istio-ingressgateway-development   LoadBalancer   172.232.48.93   80:32314/TCP,443:31978/TCP,8081:31796/TCP
```

This IP is used for accessing all Knative Services and KServe InferenceServices.

## Integration with Knative Serving

Knative Serving uses Istio as its networking layer via the **net-istio controller**.

### net-istio Components

**Deployment**: `infra/k8s/knative/net-istio/net-istio.yaml`

**ArgoCD Application**: `gitops/clusters/<env>/apps/knative-serving.yaml`

The net-istio installation creates:

1. **ClusterRole**: `knative-serving-istio` - Permissions to manage Istio resources
2. **Gateways**: Two Istio Gateway resources in `knative-serving` namespace
3. **Controllers**: `net-istio-controller` and `net-istio-webhook` deployments
4. **ConfigMap**: `config-istio` - Knative-Istio integration settings
5. **PeerAuthentication**: Webhook mTLS configuration

### Knative Gateways

**File**: `infra/k8s/knative/net-istio/net-istio.yaml`

#### 1. knative-ingress-gateway (External Traffic)

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: knative-ingress-gateway
  namespace: knative-serving
spec:
  selector:
    istio: ingressgateway  # Selects istio-ingressgateway-development pod
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "*"
```

**Purpose**: Routes external HTTP traffic from the Istio ingress gateway to Knative Services.

**Verification**:

```bash
kubectl get gateway knative-ingress-gateway -n knative-serving
```

#### 2. knative-local-gateway (Cluster-Internal Traffic)

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: knative-local-gateway
  namespace: knative-serving
spec:
  selector:
    istio: ingressgateway  # Selects same ingress gateway
  servers:
    - port:
        number: 8081
        name: http
        protocol: HTTP
      hosts:
        - "*"
```

**Purpose**: Routes cluster-internal traffic (pod-to-pod within Kubernetes) to Knative Services without going through external LoadBalancer.

**ClusterIP Service**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: knative-local-gateway
  namespace: istio-system
spec:
  type: ClusterIP
  selector:
    istio: ingressgateway
  ports:
    - name: http2
      port: 80
      targetPort: 8081
```

This allows pods to access Knative Services via `knative-local-gateway.istio-system.svc.cluster.local`.

**Verification**:

```bash
kubectl get gateway knative-local-gateway -n knative-serving
kubectl get svc knative-local-gateway -n istio-system
```

### Configuration: config-istio ConfigMap

**Namespace**: `knative-serving`

This ConfigMap tells Knative where to find the Istio gateways:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-istio
  namespace: knative-serving
data:
  # Gateway for external traffic
  gateway.knative-serving.knative-ingress-gateway: "istio-ingressgateway.istio-system.svc.cluster.local"

  # Gateway for cluster-local traffic
  local-gateway.knative-serving.knative-local-gateway: "knative-local-gateway.istio-system.svc.cluster.local"
```

**Note**: The example data in the ConfigMap shows this format, but actual configuration is usually default.

**View configuration**:

```bash
kubectl get configmap config-istio -n knative-serving -o yaml
```

## Integration with KServe

KServe InferenceServices create Knative Services, which automatically get Istio VirtualServices for routing.

### Traffic Flow for Model Inference

When you deploy a model with KServe:

1. **InferenceService created**: `kubectl apply -f isvc.yaml`
2. **KServe controller** creates a Knative Service (ksvc) named `<model>-predictor`
3. **Knative controller** creates a Configuration, Revision, and Route
4. **net-istio controller** automatically creates an Istio VirtualService
5. **VirtualService** routes traffic from the Knative Gateway to the model pod
6. **External access**: `http://<model-name>.<namespace>.dev.otherjamesbrown.com` → Istio ingress gateway → VirtualService → Pod

### Example: VirtualService for a Model

When you deploy a model like `gpt-oss-20b-development`, Knative automatically creates:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: gpt-oss-20b-development-predictor-ingress
  namespace: development
  labels:
    serving.knative.dev/route: gpt-oss-20b-development-predictor
spec:
  gateways:
    - knative-serving/knative-ingress-gateway  # External gateway
    - knative-serving/knative-local-gateway    # Cluster-local gateway
  hosts:
    - gpt-oss-20b-development-predictor.development
    - gpt-oss-20b-development-predictor.development.dev.otherjamesbrown.com
    - gpt-oss-20b-development-predictor.development.svc
    - gpt-oss-20b-development-predictor.development.svc.cluster.local
  http:
    - match:
        - authority:
            prefix: gpt-oss-20b-development-predictor.development
          gateways:
            - knative-serving/knative-local-gateway
      route:
        - destination:
            host: knative-local-gateway.istio-system.svc.cluster.local
            port:
              number: 80
          weight: 100
```

**Key Features**:
- **Multiple hosts**: Supports both short DNS names and FQDNs
- **Gateway routing**: Different rules for external vs. cluster-local traffic
- **Automatic management**: Created/updated/deleted by Knative, not manually managed

**View VirtualServices**:

```bash
# List all VirtualServices created by Knative
kubectl get virtualservice -n development -l serving.knative.dev/route

# Describe a specific VirtualService
kubectl describe virtualservice gpt-oss-20b-development-predictor-ingress -n development
```

### Namespace Labels: Sidecar Injection

**CRITICAL**: The platform uses **sidecar-less mode** for most namespaces.

```yaml
# KServe namespace - NO sidecar injection
metadata:
  labels:
    istio-injection: disabled
  name: kserve
```

**Namespaces with disabled sidecar injection**:
- `kserve` - KServe controller
- `knative-serving` - Knative controllers
- `development` - Model workloads
- `system` - Platform services

**Why sidecar-less?**
- **Reduced overhead**: No extra proxy container per pod
- **Simpler debugging**: Fewer moving parts
- **Cost efficiency**: Lower CPU/memory usage
- **Knative queue-proxy**: Knative already includes a queue-proxy sidecar for metrics and traffic shaping

**Verification**:

```bash
# Check namespace labels
kubectl get namespace kserve -o jsonpath='{.metadata.labels}'

# Verify no sidecars in pods
kubectl get pods -n development -o jsonpath='{.items[*].spec.containers[*].name}'
# Should NOT show "istio-proxy" container
```

## Gateway and VirtualService Configuration

### When to Create Manual Gateways

**Usually NOT needed**. Knative automatically manages Gateways and VirtualServices.

**Exceptions** (create manual Gateway/VirtualService):
1. **Non-Knative services** that need Istio ingress (e.g., custom API outside Knative)
2. **Advanced traffic rules** not supported by Knative (weighted routing beyond revisions, header-based routing)
3. **Direct TCP/TLS routing** (not HTTP)

### Gateway Example (Manual)

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: custom-gateway
  namespace: development
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "custom.dev.otherjamesbrown.com"
```

### VirtualService Example (Manual)

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: custom-route
  namespace: development
spec:
  hosts:
    - "custom.dev.otherjamesbrown.com"
  gateways:
    - custom-gateway
  http:
    - match:
        - uri:
            prefix: "/v1"
      route:
        - destination:
            host: my-service.development.svc.cluster.local
            port:
              number: 8000
```

### Traffic Splitting (Canary Deployments)

Knative supports traffic splitting via Knative Service spec, which automatically updates VirtualServices:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-model-predictor
spec:
  traffic:
    - revisionName: my-model-predictor-00001
      percent: 90  # 90% to stable
    - revisionName: my-model-predictor-00002
      percent: 10  # 10% to canary
```

**Note**: KServe manages this via `canaryTrafficPercent` in InferenceService spec.

## Service Mesh Features

### Traffic Management

**Capabilities**:
- **Load balancing**: Automatic load balancing across pod replicas
- **Retries**: Configurable retry policies for failed requests
- **Timeouts**: Request timeout management
- **Circuit breaking**: Prevent cascading failures
- **Traffic splitting**: Canary and blue-green deployments

**Configuration**: Managed via VirtualService and DestinationRule resources.

**Current usage**: Primarily load balancing and routing (via Knative-managed VirtualServices).

### mTLS (Mutual TLS)

**Status**: **PERMISSIVE mode** for webhooks only.

**PeerAuthentication resources**:

```yaml
# Knative webhook - allows both mTLS and plaintext
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: webhook
  namespace: knative-serving
spec:
  selector:
    matchLabels:
      app: webhook
  portLevelMtls:
    "8443":
      mode: PERMISSIVE

# net-istio webhook
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: net-istio-webhook
  namespace: knative-serving
spec:
  selector:
    matchLabels:
      app: net-istio-webhook
  portLevelMtls:
    "8443":
      mode: PERMISSIVE
```

**Why PERMISSIVE?** Allows Kubernetes API server (without Istio sidecar) to communicate with webhooks.

**Mesh-wide mTLS**: Not enabled. Services communicate in plaintext for simplicity.

**Check mTLS status**:

```bash
kubectl get peerauthentication -A
```

### Observability

**Enabled Features**:
- **Access logs**: All requests logged to stdout
- **Distributed tracing**: 100% trace sampling enabled
- **Prometheus metrics**: Istio metrics exported to Prometheus

**Configuration** (from ArgoCD app):

```yaml
meshConfig:
  accessLogFile: /dev/stdout
  enableTracing: true
  defaultConfig:
    tracing:
      sampling: 1.0  # 100% sampling
```

**Access logs location**:

```bash
# Ingress gateway logs (all inbound requests)
kubectl logs -n istio-system -l app=istio-ingressgateway

# Example log entry:
# [2025-12-08T10:15:30.123Z] "POST /v1/chat/completions HTTP/1.1" 200 - via_upstream
```

**Metrics**:

```bash
# Check Istio metrics
kubectl exec -n istio-system deploy/istiod -- curl localhost:15014/metrics

# View in Prometheus
# Query: istio_requests_total{destination_service_namespace="development"}
```

**Tracing**: Traces are exported to the platform's observability stack (if Jaeger/Zipkin configured).

## Deployment

Istio is deployed via ArgoCD using the official Istio Helm charts.

### ArgoCD Applications

**File**: `gitops/clusters/development/apps/istio.yaml`

Three Applications in one file:
1. **istio-base-development** - CRDs and base resources
2. **istiod-development** - Control plane
3. **istio-ingressgateway-development** - Ingress gateway

**Branch targeting**:
- Development: `targetRevision: develop`
- Staging: `targetRevision: staging`
- Production: `targetRevision: main`

**Sync policy**: Automated with prune, selfHeal enabled.

### Installation Order

1. **istio-base** (Wave 0) - Must be first (installs CRDs)
2. **istiod** (Wave 1) - Requires CRDs from istio-base
3. **istio-ingressgateway** (Wave 2) - Requires istiod running

ArgoCD handles ordering automatically via Application dependencies (implicit).

### Verification After Deployment

```bash
# Check all Istio pods are running
kubectl get pods -n istio-system

# Expected output:
# NAME                                                READY   STATUS
# istio-ingressgateway-development-xxx                1/1     Running
# istiod-xxx                                          1/1     Running

# Check LoadBalancer IP assigned
kubectl get svc istio-ingressgateway-development -n istio-system

# Check Knative gateways
kubectl get gateway -n knative-serving

# Test Istio configuration
kubectl exec -n istio-system deploy/istiod -- pilot-discovery request GET /debug/configz
```

### GitOps Workflow

**NEVER** manually edit Istio resources. Always use GitOps:

1. **Edit ArgoCD Application**: `vim gitops/clusters/development/apps/istio.yaml`
2. **Modify Helm values**: Change values in the `helm.values` section
3. **Commit and push**:
   ```bash
   git add gitops/clusters/development/apps/istio.yaml
   git commit -m "Update Istio ingress gateway resources"
   git push origin develop
   ```
4. **ArgoCD syncs automatically** (development) or manually sync:
   ```bash
   argocd app sync istio-ingressgateway-development
   ```
5. **Verify changes**:
   ```bash
   kubectl get deployment istio-ingressgateway-development -n istio-system -o yaml
   ```

## Troubleshooting

### Ingress Gateway Not Reachable

**Symptoms**: Cannot access `http://<model>.development.dev.otherjamesbrown.com`

**Debug steps**:

```bash
# 1. Check ingress gateway pod status
kubectl get pods -n istio-system -l app=istio-ingressgateway

# 2. Check LoadBalancer IP
kubectl get svc istio-ingressgateway-development -n istio-system
# Verify EXTERNAL-IP is assigned (not <pending>)

# 3. Check ingress gateway logs
kubectl logs -n istio-system -l app=istio-ingressgateway

# 4. Verify Gateway resource exists
kubectl get gateway knative-ingress-gateway -n knative-serving

# 5. Check DNS resolution
nslookup gpt-oss-20b-development-predictor.development.dev.otherjamesbrown.com
# Should resolve to LoadBalancer IP
```

**Common causes**:
- LoadBalancer IP not assigned (check Linode LKE NodeBalancer)
- DNS not configured (add to `/etc/hosts` for local dev)
- Gateway selector mismatch (verify `istio: ingressgateway` label)

### VirtualService Not Created

**Symptoms**: Knative Service exists but no VirtualService

**Debug steps**:

```bash
# 1. Check Knative Service status
kubectl get ksvc -n development

# 2. Check if Route is ready
kubectl get route -n development

# 3. Check net-istio-controller logs
kubectl logs -n knative-serving -l app=net-istio-controller

# 4. List VirtualServices
kubectl get virtualservice -n development
```

**Common causes**:
- net-istio-controller not running
- Knative Service not ready (check Revision status)
- Istio CRDs not installed (check `kubectl get crd gateways.networking.istio.io`)

### 503 Service Unavailable

**Symptoms**: Request reaches ingress gateway but returns 503

**Debug steps**:

```bash
# 1. Check ingress gateway access logs
kubectl logs -n istio-system -l app=istio-ingressgateway | grep 503

# 2. Verify VirtualService destination
kubectl get virtualservice <name> -n development -o yaml
# Check destination.host matches actual service

# 3. Check backend service exists
kubectl get svc -n development

# 4. Test backend directly (port-forward)
kubectl port-forward -n development svc/<service-name> 8080:80
curl http://localhost:8080
```

**Common causes**:
- Backend pod not ready (check `kubectl get pods -n development`)
- Service name mismatch in VirtualService
- Activator not routing traffic (Knative scaled to zero issue)

### istiod CrashLoopBackOff

**Symptoms**: `istiod` pod restarting repeatedly

**Debug steps**:

```bash
# 1. Check pod status
kubectl get pods -n istio-system -l app=istiod

# 2. Check pod logs
kubectl logs -n istio-system -l app=istiod --previous

# 3. Describe pod for events
kubectl describe pod -n istio-system -l app=istiod

# 4. Check resource limits
kubectl get deployment istiod -n istio-system -o jsonpath='{.spec.template.spec.containers[0].resources}'
```

**Common causes**:
- Insufficient memory (increase limits in ArgoCD app)
- Invalid configuration (check Helm values)
- Certificate issues (check webhook certificates)

### Gateway Label Mismatch

**Symptoms**: Gateway exists but ingress gateway doesn't serve traffic

**Debug steps**:

```bash
# 1. Check Gateway selector
kubectl get gateway knative-ingress-gateway -n knative-serving -o yaml | grep selector

# Should show:
# selector:
#   istio: ingressgateway

# 2. Check ingress gateway pod labels
kubectl get pods -n istio-system -l app=istio-ingressgateway -o jsonpath='{.items[0].metadata.labels}'

# Must include: "istio": "ingressgateway"
```

**Fix**: Ensure ArgoCD Application includes `labels.istio: ingressgateway` in Helm values.

### Sidecar Injection Issues

**Symptoms**: Pods have unexpected `istio-proxy` container (or missing when expected)

**Debug steps**:

```bash
# 1. Check namespace labels
kubectl get namespace <namespace> -o jsonpath='{.metadata.labels}'

# 2. Check pod labels
kubectl get pods <pod-name> -n <namespace> -o jsonpath='{.metadata.labels}'

# 3. Check sidecar injection webhook
kubectl get mutatingwebhookconfigurations | grep istio

# 4. Check istiod logs for injection decisions
kubectl logs -n istio-system -l app=istiod | grep "Sidecar injection"
```

**Fix**: Add `istio-injection: disabled` label to namespace (platform default).

## Common Operations

### View Istio Configuration

```bash
# List all Istio resources
kubectl get gateways,virtualservices,destinationrules -A

# Get Istio version
kubectl get deployment istiod -n istio-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# Check istiod configuration status
kubectl exec -n istio-system deploy/istiod -- pilot-discovery request GET /debug/configz | less
```

### Access Ingress Gateway Logs

```bash
# Follow ingress gateway logs
kubectl logs -n istio-system -l app=istio-ingressgateway -f

# Filter for specific model requests
kubectl logs -n istio-system -l app=istio-ingressgateway | grep "gpt-oss-20b"

# Show only 503 errors
kubectl logs -n istio-system -l app=istio-ingressgateway | grep "503"
```

### Test Gateway Connectivity

```bash
# Get LoadBalancer IP
LB_IP=$(kubectl get svc istio-ingressgateway-development -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Test HTTP connectivity
curl -v -H "Host: gpt-oss-20b-development-predictor.development.dev.otherjamesbrown.com" http://$LB_IP/

# Test cluster-local gateway (from inside cluster)
kubectl run test-pod --rm -it --image=curlimages/curl -- sh
# Inside pod:
curl http://knative-local-gateway.istio-system.svc.cluster.local/
```

### Update Istio Configuration

**Example**: Increase ingress gateway replicas

1. **Edit ArgoCD Application**:
   ```bash
   vim gitops/clusters/development/apps/istio.yaml
   ```

2. **Modify Helm values**:
   ```yaml
   # In istio-ingressgateway-development Application
   helm:
     values: |
       replicaCount: 2  # Increase from 1 to 2
       resources:
         requests:
           cpu: 100m
           memory: 128Mi
   ```

3. **Commit and push**:
   ```bash
   git add gitops/clusters/development/apps/istio.yaml
   git commit -m "Scale Istio ingress gateway to 2 replicas"
   git push origin develop
   ```

4. **Verify ArgoCD sync**:
   ```bash
   argocd app get istio-ingressgateway-development
   argocd app sync istio-ingressgateway-development
   ```

5. **Verify deployment**:
   ```bash
   kubectl get deployment istio-ingressgateway-development -n istio-system
   # DESIRED should show 2
   ```

## Performance Tuning

### For High-Traffic Environments

Increase ingress gateway resources:

```yaml
# In ArgoCD Application helm.values
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi

# Enable autoscaling
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 5
  targetCPUUtilizationPercentage: 80
```

### For Low-Resource Development

Reduce istiod and gateway resources:

```yaml
# istiod
pilot:
  resources:
    requests:
      cpu: 50m
      memory: 256Mi

# Ingress gateway
resources:
  requests:
    cpu: 50m
    memory: 64Mi
```

### Access Logging Configuration

Disable access logs to reduce I/O overhead (not recommended for production):

```yaml
meshConfig:
  accessLogFile: ""  # Empty = disabled
```

Or use sampling:

```yaml
meshConfig:
  accessLogFile: /dev/stdout
  accessLogEncoding: JSON
  accessLogFormat: |
    {"start_time":"%START_TIME%","method":"%REQ(:METHOD)%","path":"%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%","response_code":"%RESPONSE_CODE%"}
```

## Upgrades

### Istio Version Upgrade Process

**Current version**: 1.19.0

**Upgrade steps**:

1. **Check compatibility**: Verify Knative Serving supports new Istio version
   - Knative 1.11 supports Istio 1.16-1.20

2. **Test in development first**: Never upgrade production directly

3. **Update ArgoCD Application** (`gitops/clusters/development/apps/istio.yaml`):
   ```yaml
   source:
     targetRevision: 1.20.0  # New version
   ```

4. **Upgrade order** (ArgoCD handles automatically):
   - `istio-base` first (CRDs)
   - `istiod` second (control plane)
   - `istio-ingressgateway` last (data plane)

5. **Monitor upgrade**:
   ```bash
   argocd app get istio-base-development
   kubectl get pods -n istio-system -w
   ```

6. **Verify functionality**:
   ```bash
   # Test model inference
   curl http://gpt-oss-20b-development-predictor.development.dev.otherjamesbrown.com/v1/chat/completions
   ```

7. **Promote to staging/production** after successful testing

## References

- [Istio Documentation](https://istio.io/latest/docs/)
- [Istio Helm Charts](https://github.com/istio/istio/tree/master/manifests/charts)
- [Knative net-istio Documentation](https://knative.dev/docs/install/yaml-install/serving/install-serving-with-yaml/#install-a-networking-layer)
- [Istio Gateway API](https://istio.io/latest/docs/reference/config/networking/gateway/)
- [Istio VirtualService API](https://istio.io/latest/docs/reference/config/networking/virtual-service/)

## Related Documentation

- [Knative Configuration](knative-configuration.md) - Knative Serving setup and integration with Istio
- [Infrastructure Overview](infrastructure-overview.md) - Overall platform architecture
- [Endpoints and URLs](endpoints-and-urls.md) - Service endpoint URLs and ingress configuration
- [Certificate Architecture](certificate-architecture.md) - TLS certificate management
- [Observability Guide](observability-guide.md) - Metrics, logs, and tracing (includes Istio metrics)
