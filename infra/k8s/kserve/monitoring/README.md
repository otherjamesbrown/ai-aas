# KServe Monitoring Resources

This directory contains optional monitoring resources for KServe inference workloads.

## Requirements

These resources require **Prometheus Operator** to be installed in the cluster, which provides the following CRDs:
- `PodMonitor` (monitoring.coreos.com/v1)
- `ServiceMonitor` (monitoring.coreos.com/v1)
- `PrometheusRule` (monitoring.coreos.com/v1)

## Contents

| File | Purpose | Metrics Collected |
|------|---------|-------------------|
| `podmonitor-vllm.yaml` | vLLM inference metrics | TTFT, throughput, cache usage, token counts |
| `podmonitor-queue-proxy.yaml` | Knative queue-proxy metrics | Request latency, queue depth, concurrency |
| `podmonitor-dcgm.yaml` | NVIDIA DCGM GPU metrics | GPU utilization, memory, temperature, power |

## Deployment

### Environments with Prometheus Operator

If Prometheus Operator is installed (check with `kubectl get crd podmonitors.monitoring.coreos.com`):

**Option 1: Include in kserve-config ArgoCD app** (recommended for development)
```yaml
spec:
  source:
    path: infra/k8s/kserve
  # This will deploy both base/ and monitoring/
```

**Option 2: Separate ArgoCD app** (recommended for staging/production)
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kserve-monitoring-<env>
  namespace: argocd
spec:
  project: platform-<env>
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: <branch>
    path: infra/k8s/kserve/monitoring
  destination:
    server: https://kubernetes.default.svc
    namespace: kserve
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### Environments without Prometheus Operator

If Prometheus Operator is NOT installed:
- **Do NOT deploy these resources** - they will cause sync failures
- The kserve-config app should point to `infra/k8s/kserve/base` only
- Alternative: Use ServiceMonitors if supported, or configure Prometheus scrape configs manually

## Checking Prometheus Operator Installation

```bash
# Check if PodMonitor CRD exists
kubectl get crd podmonitors.monitoring.coreos.com

# If installed, check version
kubectl get crd podmonitors.monitoring.coreos.com -o jsonpath='{.spec.versions[*].name}'

# List existing PodMonitors
kubectl get podmonitors -A
```

## Installing Prometheus Operator

If you need to install Prometheus Operator:

**Using Helm (recommended)**:
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus-operator prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

**Note**: Some environments may have Prometheus Operator installed automatically:
- NVIDIA GPU Operator includes Prometheus Operator CRDs
- Some Kubernetes distributions bundle it

## Metrics Usage

Once deployed, these PodMonitors enable:
- **Grafana dashboards** for vLLM performance (see `docs/platform/vllm-observability.md`)
- **Alerting** on inference latency, GPU utilization, OOM conditions
- **Capacity planning** using historical throughput and cache usage trends
- **Cost optimization** correlating GPU utilization with model performance

## Related Documentation

- [vLLM Observability Guide](../../../../docs/platform/vllm-observability.md)
- [Observability Guide](../../../../docs/platform/observability-guide.md)
- [KServe Management](../../../../docs/platform/kserve-management.md)
