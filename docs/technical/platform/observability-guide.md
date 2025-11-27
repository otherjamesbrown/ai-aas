# Platform Observability Guide

**Feature**: `016-kserve-migration`
**Last Updated**: 2025-11-27
**Owner**: Platform Engineering

## Stack Components

- **Prometheus**: For metrics collection, using `kube-prometheus-stack`.
- **Grafana**: For visualization and dashboards.
- **Alertmanager**: For routing alerts to Slack and PagerDuty.
- **Loki**: For log aggregation.
- **Tempo**: For distributed tracing.
- **DCGM Exporter**: For NVIDIA GPU hardware metrics.

## Logging Standards

All Go services use the `shared/go/logging` package for consistent structured JSON logging. Key fields include `service`, `environment`, `trace_id`, `request_id`, `user_id`, and `org_id`.

## Metrics

### KServe Control Plane Metrics
- `kserve_reconcile_duration_seconds`: Time to reconcile `InferenceService`.
- `kserve_inference_service_status`: Status of `InferenceService`s.

### Knative Autoscaling Metrics
- `knative_serving_autoscaler_actual_pods`: Current pod count per `InferenceService`.
- `knative_serving_autoscaler_desired_pods`: Desired pod count based on load.
- `knative_serving_revision_request_latencies`: Request latency per revision.

### Istio Service Mesh Metrics
- `istio_requests_total`: Request count to `InferenceService` predictors.
- `istio_request_duration_milliseconds`: Request duration.

### vLLM Runtime Metrics
- `vllm_num_requests_running`: Active inference requests.
- `vllm_gpu_cache_usage_perc`: KV cache utilization.

### DCGM GPU Hardware Metrics
- `DCGM_FI_DEV_GPU_UTIL`: GPU utilization.
- `DCGM_FI_DEV_FB_USED`: GPU framebuffer memory used.

## Dashboards

Dashboards are stored as code in `infra/helm/charts/observability-stack/dashboards/`.

### 1. KServe Overview
- `InferenceService` status, reconciliation errors, model revision rollouts.

### 2. Knative Autoscaling
- Current vs. desired pod counts, scaling events, cold start frequency.

### 3. Inference Performance
- Request rate, latency (p50/p90/p99), and error rate per model.

### 4. Resource Utilization
- GPU, CPU, and memory usage per `InferenceService` pod.

## Alert Policies

Alerts are defined in `infra/helm/charts/observability-stack/templates/alerts/*.yaml`.

- **KServeHighLatency**: P95 latency for an `InferenceService` is too high.
- **KServeHighErrorRate**: Error rate for an `InferenceService` is too high.
- **KnativeNotScaling**: Knative is not scaling up pods under load.

## Querying Prometheus

### Common Query Patterns

#### List All Models
```promql
count by (inferenceservice) (kserve_inference_service_status)
```

#### GPU Utilization for a Specific Model
```promql
DCGM_FI_DEV_GPU_UTIL{pod=~"<isvc-name>-predictor.*"}
```

#### Total Throughput Across Fleet
```promql
sum(rate(istio_requests_total[5m]))
```