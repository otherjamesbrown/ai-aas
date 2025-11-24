# Spec 016: Migrate vLLM Deployments to KServe

## 1. Background

Currently, the AI-as-a-Service platform deploys vLLM models using custom Helm charts (`infra/helm/charts/vllm-deployment`). While this works, it requires significant maintenance for features like autoscaling, scale-to-zero, and model caching.

**KServe** is a standard Kubernetes-native platform for model serving that solves these problems out of the box. It supports vLLM natively (via the Hugging Face runtime) and provides advanced features like:
- Serverless autoscaling (including scale-to-zero) via Knative.
- Standardized V2 Inference Protocol.
- Model caching to reduce cold start times.
- Canary rollouts and traffic splitting.

## 2. Goals

1.  Replace custom vLLM Helm charts with KServe `InferenceService` resources.
2.  Integrate KServe with the existing `api-router-service`.
3.  Evaluate and potentially replace the custom Model Registry with an open-source alternative (e.g., MLflow or Kubeflow Model Registry).
4.  Maintain parity with existing features (GPU acceleration, metrics, health checks).

## 3. Architecture Changes

### 3.1. Deployment Model

**Current:**
- Custom `Deployment` + `Service` + `HPA` managed by Helm.
- Manual node tainting/labeling (as per `vLLM_LKE_GUIDE.md`).

**New:**
- **KServe `InferenceService`**: A single CRD that defines the model, runtime, and scaling policies.
- **Knative Serving**: Handles request-based autoscaling and scale-to-zero.
- **Istio**: Handles ingress and traffic routing.

### 3.2. Component Replacements

| Component | Current Implementation | Proposed Replacement |
| :--- | :--- | :--- |
| **Inference Engine** | vLLM (Custom Pod) | vLLM (via KServe HuggingFace Runtime) |
| **Autoscaling** | Kubernetes HPA (CPU/Memory) | Knative Pod Autoscaler (Request-based/Concurrency) |
| **Routing/Ingress** | Kubernetes Service | KServe Network (Istio/Envoy) |
| **Model Registry** | Custom PostgreSQL Table | **MLflow** or **Kubeflow Model Registry** (Optional Phase 2) |
| **API Gateway** | `api-router-service` | Keep `api-router-service` for auth/billing, but route to KServe URLs |

## 4. Implementation Plan

### Phase 1: Infrastructure Setup
1.  Install **KServe**, **Istio**, and **Knative** on the cluster.
2.  Configure GPU support for KServe.
3.  Set up `ClusterStorageContainer` for Hugging Face model access.

### Phase 2: Migration of vLLM Deployments
1.  Create a reusable `InferenceService` template for vLLM models.
2.  Migrate one model (e.g., `llama-2-7b`) to KServe in the Development environment.
3.  Verify inference using the standard V2 protocol.

### Phase 3: Integration
1.  Update `api-router-service` to route requests to KServe endpoints.
    - KServe endpoints follow the pattern: `http://<model-name>.<namespace>.svc.cluster.local/v2/models/<model-name>/infer`
2.  Update `admin-cli` to manage KServe resources instead of Helm releases.

### Phase 4: Cleanup
1.  Deprecate and remove `infra/helm/charts/vllm-deployment`.
2.  Update documentation and guides.

## 5. Open Questions / Risks

- **Cold Starts**: Scale-to-zero causes latency on the first request. We need to configure `minReplicas: 1` for production models or use KServe's model caching.
- **Resource Overhead**: Running Istio + Knative + KServe adds control plane overhead.
- **Protocol Change**: KServe uses the V2 Inference Protocol. We need to ensure `api-router-service` or the vLLM runtime adapts the OpenAI-compatible API correctly. (Note: KServe's vLLM runtime *can* expose OpenAI endpoints, we need to verify configuration).

## 6. Success Metrics

- Successful deployment of Llama-2-7b using `InferenceService`.
- Autoscaling works (scales up under load, scales down when idle).
- `api-router-service` successfully proxies requests to the new KServe endpoint.
