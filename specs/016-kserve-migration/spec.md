# Feature Specification: KServe Migration for Model Serving

**Feature Branch**: `016-kserve-migration`
**Created**: 2025-01-27
**Status**: Draft
**Input**: Migrate custom vLLM Helm chart deployments to KServe InferenceServices for standardized model serving with autoscaling, scale-to-zero capabilities, and reduced operational overhead.

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.

  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Deploy KServe infrastructure (Priority: P1)

As a platform engineer, I can install and configure KServe, Istio, and Knative on the development cluster with GPU support, establishing the foundation for model serving.

**Why this priority**: Provides the foundational infrastructure required for all subsequent KServe functionality.

**Independent Test**: Can be tested by verifying KServe controller pods are running, Istio components are healthy, Knative serving is operational, and GPU nodes are correctly labeled.

**Acceptance Scenarios**:

1. **Given** a clean Kubernetes cluster, **When** I deploy KServe infrastructure using GitOps, **Then** all KServe, Istio, and Knative components reach ready state within 10 minutes.
2. **Given** GPU nodes in the cluster, **When** I configure GPU support for KServe, **Then** GPU resources are correctly exposed and schedulable for InferenceService pods.
3. **Given** deployed infrastructure, **When** I verify component health, **Then** all control plane pods show healthy status and resource usage is within expected limits.

---

### User Story 2 - Deploy first model with InferenceService (Priority: P1)

As a platform engineer, I can deploy a vLLM model (llama-2-7b) using KServe InferenceService CRD in the development environment and verify successful inference.

**Why this priority**: Validates end-to-end model serving through KServe with a known working model.

**Independent Test**: Can be tested by creating an InferenceService resource, waiting for it to become ready, and successfully making an inference request via the KServe endpoint.

**Acceptance Scenarios**:

1. **Given** KServe infrastructure is deployed, **When** I create an InferenceService for llama-2-7b, **Then** the model pod starts on a GPU node and reaches ready state within 15 minutes.
2. **Given** a ready InferenceService, **When** I send a test inference request to the KServe endpoint, **Then** I receive a valid completion response within 5 seconds.
3. **Given** a running InferenceService, **When** I inspect the pod logs, **Then** vLLM successfully loads the model and reports healthy status.

---

### User Story 3 - Integrate KServe with API Router (Priority: P2)

As a platform engineer, I can update the api-router-service to route inference requests to KServe endpoints instead of custom vLLM services, maintaining backward compatibility with the OpenAI-compatible API.

**Why this priority**: Enables existing clients to transparently use KServe-backed models without API changes.

**Independent Test**: Can be tested by sending requests through api-router-service to a KServe-backed model and verifying successful inference with proper billing/metering.

**Acceptance Scenarios**:

1. **Given** a model deployed via KServe, **When** I configure api-router-service backend to point to KServe endpoint, **Then** client requests successfully route through api-router to KServe and return valid completions.
2. **Given** KServe's V2 protocol, **When** api-router-service proxies requests, **Then** the internal "OpenAI-to-KServe Adapter" translates the protocol (OpenAI ↔ V2) correctly without data loss.
3. **Given** a hybrid environment, **When** requests are made to legacy and KServe models, **Then** the router correctly directs traffic to the appropriate backend based on the model registry's `backend_type`.
4. **Given** KServe endpoint failure, **When** api-router-service attempts to route, **Then** proper error handling occurs with client-friendly error messages and circuit breaking.

---

### User Story 4 - Implement autoscaling and scale-to-zero (Priority: P2)

As a platform engineer, I can configure Knative autoscaling policies for InferenceServices to scale up under load and scale down to zero during idle periods, optimizing resource utilization.

**Why this priority**: Delivers key value proposition of KServe - dynamic resource management based on demand.

**Independent Test**: Can be tested by monitoring pod count during periods of no traffic (should scale to zero if configured, or to minReplicas) and under sustained load (should scale up based on concurrency).

**Acceptance Scenarios**:

1. **Given** an InferenceService with scale-to-zero enabled, **When** no requests are made for 5 minutes, **Then** the model pod scales down to zero and resources are released.
2. **Given** a scaled-to-zero model, **When** a new request arrives, **Then** Knative cold-starts the pod and serves the request within acceptable SLA (acknowledging cold-start latency).
3. **Given** high concurrent load, **When** request concurrency exceeds configured target, **Then** KServe autoscaler creates additional pods up to maxReplicas and distributes load.

---

### User Story 5 - Migrate remaining models from custom Helm charts (Priority: P3)

As a platform engineer, I can systematically migrate all remaining vLLM model deployments from custom Helm charts to KServe InferenceServices, following a phased rollout approach.

**Why this priority**: Completes the migration and allows deprecation of custom Helm charts.

**Independent Test**: Can be tested by migrating one model at a time, running parallel deployments briefly, and verifying feature parity before decommissioning the old deployment.

**Acceptance Scenarios**:

1. **Given** a model deployed via custom Helm chart, **When** I deploy equivalent InferenceService, **Then** both deployments coexist and serve traffic successfully during migration window.
2. **Given** parallel deployments, **When** I switch api-router-service backend to KServe endpoint, **Then** traffic shifts smoothly without errors or dropped requests.
3. **Given** successful KServe deployment, **When** I decomm Helm release, **Then** old resources are cleanly removed and no orphaned pods or services remain.

---

### User Story 6 - Update admin-cli for KServe management (Priority: P3)

As an operator, I can use admin-cli to create, update, list, and delete model deployments via KServe InferenceServices instead of Helm releases.

**Why this priority**: Provides consistent operational tooling for the new deployment model.

**Independent Test**: Can be tested by using admin-cli commands to manage InferenceService lifecycle and verifying operations succeed.

**Acceptance Scenarios**:

1. **Given** admin-cli with KServe support, **When** I run `admin-cli model deploy --name llama-2-7b --runtime kserve`, **Then** an InferenceService is created and becomes ready.
2. **Given** deployed models, **When** I run `admin-cli model list`, **Then** I see all InferenceServices with status, endpoints, and resource usage.
3. **Given** a model to remove, **When** I run `admin-cli model delete --name llama-2-7b`, **Then** the InferenceService and all related resources are cleanly deleted.

---

### User Story 7 - Evaluate and integrate model registry (Priority: P4)

As a platform engineer, I can evaluate MLflow or Kubeflow Model Registry as a replacement for the custom model registry, providing standardized model versioning and lineage tracking.

**Why this priority**: Improves model governance but not critical for initial KServe functionality.

**Independent Test**: Can be tested by deploying a model registry, registering a model, and deploying an InferenceService that references the registered model.

**Acceptance Scenarios**:

1. **Given** MLflow deployed, **When** I register a model version, **Then** model artifacts are stored with metadata and lineage information.
2. **Given** a registered model, **When** I create InferenceService referencing model URI from registry, **Then** KServe pulls model artifacts and deploys successfully.
3. **Given** model registry integration, **When** api-router-service queries available models, **Then** it retrieves model list and versions from registry instead of database table.
4. **Given** existing legacy models, **When** the registry schema is updated, **Then** all existing records are backfilled with `backend_type='legacy_helm'` to ensure uninterrupted routing.

---

### Edge Cases

- **Cold start latency**: First request after scale-to-zero experiences high latency; configure minReplicas for production or implement model caching.
- **GPU scheduling conflicts**: Multiple InferenceServices competing for limited GPU nodes; implement node affinity and resource quotas.
- **Protocol incompatibility**: OpenAI API clients incompatible with KServe V2 protocol; implement protocol adapter in api-router-service.
- **Network policy conflicts**: Istio ingress blocked by existing NetworkPolicies; update policies to allow Istio gateway traffic.
- **Model loading failures**: Large models exceeding ephemeral storage or memory limits; adjust resource requests/limits and consider PVC-backed storage.
- **Concurrent updates**: Multiple InferenceService revisions during rolling updates causing traffic split issues; implement blue-green deployment strategy.
- **Scale-to-zero during high load**: Aggressive scale-down causing unnecessary cold starts; tune Knative autoscaler parameters (stable-window, scale-down-delay).
- **Cross-namespace routing**: api-router-service in different namespace unable to reach InferenceService; configure proper service mesh policies.
- **Metrics collection gaps**: Custom vLLM metrics not exposed through KServe; implement custom metrics exporter or extend ServingRuntime.
- **License validation**: Commercial models requiring license checks on startup; implement init containers for license validation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide Helm charts or GitOps manifests for deploying KServe, Istio, and Knative to development and production clusters.
- **FR-002**: Provide GPU node labeling and configuration for KServe to schedule InferenceServices on GPU-enabled nodes.
- **FR-003**: Provide InferenceService templates for vLLM models supporting Hugging Face model URIs.
- **FR-004**: Provide Knative autoscaling configuration for request-based scaling and scale-to-zero capabilities.
- **FR-005**: Provide ClusterStorageContainer configuration for accessing Hugging Face models from object storage or model hub.
- **FR-006**: Provide api-router-service updates to implement an "OpenAI-to-KServe Adapter" for protocol translation (OpenAI ↔ KServe V2) without modifying the external API contract.
- **FR-007**: Provide model registration mechanism to map client model names to KServe InferenceService endpoints.
- **FR-008**: Provide admin-cli commands for managing InferenceService lifecycle (create, update, delete, list, describe).
- **FR-009**: Provide migration runbook with step-by-step procedures for moving models from Helm charts to KServe.
- **FR-010**: Provide health checks and readiness probes for InferenceServices compatible with existing monitoring.
- **FR-011**: Provide metrics collection and export from KServe InferenceServices to Prometheus/Grafana.
- **FR-012**: Provide canary deployment support for gradual model rollouts with traffic splitting.
- **FR-013**: Provide integration with existing authentication and authorization mechanisms (API keys, JWT).
- **FR-014**: Provide model versioning support to deploy multiple versions of same model simultaneously.
- **FR-015**: Provide resource quotas and limits per InferenceService to prevent resource exhaustion.
- **FR-016**: Provide hybrid routing logic in api-router-service to support both legacy vLLM URLs and KServe DNS names based on model registry lookup.
- **FR-017**: Update model registry schema to include `backend_type` (enum: `legacy_helm`, `kserve`) to support coexistence.

### Non-Functional Requirements

- **NFR-001**: InferenceService deployment must reach ready state within 15 minutes for models <10GB.
- **NFR-002**: Cold start latency for scaled-to-zero models must be ≤30 seconds for models <5GB.
- **NFR-003**: Inference request latency through KServe must be within 10% of custom vLLM deployment latency.
- **NFR-004**: Knative autoscaler must scale up within 30 seconds when concurrent requests exceed target.
- **NFR-005**: KServe control plane overhead must not exceed 2GB memory and 1 CPU core per cluster.
- **NFR-006**: Model serving availability must be ≥99.5% during normal operations (excluding cold starts).
- **NFR-007**: Protocol translation overhead in api-router-service must add ≤5ms latency.
- **NFR-008**: Migration of a single model must complete within 1 hour including validation.
- **NFR-009**: KServe infrastructure must support at least 10 concurrent InferenceServices per cluster.
- **NFR-010**: Documentation must include troubleshooting guides for common KServe issues.
- **NFR-011**: KServe metrics must be mapped to existing observability standards (Spec 011) to ensure continuity of dashboards.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: KServe infrastructure deployment completes successfully in development cluster with all components healthy.
- **SC-002**: Llama-2-7b model deploys via InferenceService and serves successful inference requests with <5s latency.
- **SC-003**: api-router-service successfully routes 100% of test requests to KServe endpoint with proper protocol translation.
- **SC-004**: Autoscaling demonstrates scale-up under load (10→50 concurrent requests) and scale-down during idle (5min no traffic).
- **SC-005**: At least 3 existing vLLM models migrate from custom Helm charts to KServe with feature parity.
- **SC-006**: admin-cli commands successfully manage InferenceService lifecycle with 100% success rate.
- **SC-007**: P95 inference latency through KServe is within 10% of baseline custom vLLM deployment latency.
- **SC-008**: Zero production incidents caused by KServe migration during 30-day post-migration period.
- **SC-009**: Cost savings of ≥20% achieved through scale-to-zero for low-traffic models.
- **SC-010**: Documentation covers 100% of operational procedures with runbooks and troubleshooting guides.
- **SC-011**: Autoscaling and scale-to-zero capabilities verified using `specs/014-load-testing-harness` with <5% error rate during scale-up.

## Architecture Overview

### Current Architecture (Custom vLLM Deployments)

```
┌─────────────────────────────────────────────────────────────────┐
│                     Current vLLM Architecture                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌────────────────┐      ┌──────────────────────────────────┐   │
│  │   API Router   │─────▶│   vLLM Model Deployments         │   │
│  │   Service      │      │   (Custom Helm Charts)           │   │
│  │                │      │                                  │   │
│  │  - OpenAI API  │      │  ┌─────────────────────────┐    │   │
│  │  - Auth        │      │  │  llama-2-7b             │    │   │
│  │  - Billing     │      │  │  Deployment + Service   │    │   │
│  │  - Routing     │      │  │  Manual HPA             │    │   │
│  │                │      │  │  GPU NodeSelector       │    │   │
│  └────────────────┘      │  └─────────────────────────┘    │   │
│         │                │                                  │   │
│         │                │  ┌─────────────────────────┐    │   │
│         │                │  │  mistral-7b             │    │   │
│         └────────────────┼─▶│  Deployment + Service   │    │   │
│                          │  │  Manual HPA             │    │   │
│                          │  │  GPU NodeSelector       │    │   │
│                          │  └─────────────────────────┘    │   │
│                          │                                  │   │
│                          │  ┌─────────────────────────┐    │   │
│                          │  │  More Models...         │    │   │
│                          │  │  (Each w/ Helm chart)   │    │   │
│                          │  └─────────────────────────┘    │   │
│                          └──────────────────────────────────┘   │
│                                                                   │
│  Limitations:                                                     │
│  ✗ Manual autoscaling (HPA on CPU/Memory, not request-based)     │
│  ✗ No scale-to-zero (wasted resources during idle)               │
│  ✗ Complex Helm chart maintenance per model                      │
│  ✗ No built-in canary deployments or traffic splitting           │
│  ✗ Manual model registry synchronization                         │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

### Target Architecture (KServe)

```
┌───────────────────────────────────────────────────────────────────────┐
│                     Target KServe Architecture                         │
├───────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌────────────────┐      ┌──────────────────────────────────────┐     │
│  │   API Router   │      │   KServe Control Plane               │     │
│  │   Service      │      │                                      │     │
│  │                │      │  ┌────────────────────────────────┐  │     │
│  │  - OpenAI API  │      │  │  KServe Controller             │  │     │
│  │  - Auth        │      │  │  - Manages InferenceServices   │  │     │
│  │  - Billing     │      │  │  - Creates Knative Services    │  │     │
│  │  - Routing     │      │  │  - Handles model revisions     │  │     │
│  │  - Protocol    │      │  └────────────────────────────────┘  │     │
│  │    Translation │      │                                      │     │
│  └────────┬───────┘      │  ┌────────────────────────────────┐  │     │
│           │              │  │  Knative Serving               │  │     │
│           │              │  │  - Request-based autoscaling   │  │     │
│           │              │  │  - Scale-to-zero               │  │     │
│           │              │  │  - Revision management         │  │     │
│           │              │  └────────────────────────────────┘  │     │
│           │              │                                      │     │
│           │              │  ┌────────────────────────────────┐  │     │
│           │              │  │  Istio Service Mesh            │  │     │
│           │              │  │  - Ingress / Egress            │  │     │
│           │              │  │  - Traffic splitting           │  │     │
│           │              │  │  - Observability               │  │     │
│           │              │  └────────────────────────────────┘  │     │
│           │              └──────────────────────────────────────┘     │
│           │                                                            │
│           │              ┌──────────────────────────────────────┐     │
│           └─────────────▶│   InferenceServices (CRDs)           │     │
│                          │                                      │     │
│                          │  ┌────────────────────────────────┐  │     │
│                          │  │  llama-2-7b InferenceService   │  │     │
│                          │  │                                │  │     │
│                          │  │  apiVersion: serving.kserve.io │  │     │
│                          │  │  spec:                         │  │     │
│                          │  │    predictor:                  │  │     │
│                          │  │      model: vllm               │  │     │
│                          │  │      runtime: huggingface      │  │     │
│                          │  │      storageUri: hf://...      │  │     │
│                          │  │      resources:                │  │     │
│                          │  │        limits:                 │  │     │
│                          │  │          nvidia.com/gpu: 1     │  │     │
│                          │  │      autoscaling:              │  │     │
│                          │  │        target: 5               │  │     │
│                          │  │        minReplicas: 0 or 1     │  │     │
│                          │  │                                │  │     │
│                          │  │  ↓ Creates ↓                   │  │     │
│                          │  │                                │  │     │
│                          │  │  Knative Service (auto)        │  │     │
│                          │  │  │                             │  │     │
│                          │  │  ├─ Revision v1 (90% traffic)  │  │     │
│                          │  │  │  └─ vLLM Pods (0-N)         │  │     │
│                          │  │  │                             │  │     │
│                          │  │  └─ Revision v2 (10% traffic)  │  │     │
│                          │  │     └─ vLLM Pods (0-N)         │  │     │
│                          │  │                                │  │     │
│                          │  └────────────────────────────────┘  │     │
│                          │                                      │     │
│                          │  ┌────────────────────────────────┐  │     │
│                          │  │  mistral-7b InferenceService   │  │     │
│                          │  │  (Similar structure)           │  │     │
│                          │  └────────────────────────────────┘  │     │
│                          │                                      │     │
│                          │  ┌────────────────────────────────┐  │     │
│                          │  │  More Models...                │  │     │
│                          │  │  (Standardized CRDs)           │  │     │
│                          │  └────────────────────────────────┘  │     │
│                          └──────────────────────────────────────┘     │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────┐       │
│  │  Model Storage                                              │       │
│  │                                                              │       │
│  │  ┌──────────────────┐     ┌──────────────────────────┐     │       │
│  │  │  Hugging Face    │     │  S3 / MinIO              │     │       │
│  │  │  Model Hub       │     │  (Private models)        │     │       │
│  │  │  (Public models) │     │                          │     │       │
│  │  └──────────────────┘     └──────────────────────────┘     │       │
│  │                                                              │       │
│  │  ClusterStorageContainer: Configures access credentials     │       │
│  └────────────────────────────────────────────────────────────┘       │
│                                                                         │
│  Benefits:                                                              │
│  ✓ Request-based autoscaling (Knative)                                 │
│  ✓ Scale-to-zero for cost savings                                      │
│  ✓ Standardized InferenceService CRDs (no custom Helm per model)       │
│  ✓ Built-in canary deployments and traffic splitting                   │
│  ✓ Model versioning and revision management                            │
│  ✓ Integrated observability (Istio metrics)                            │
│                                                                         │
└───────────────────────────────────────────────────────────────────────┘
```

### Component Replacement Matrix

| Component | Current Implementation | KServe Replacement | Migration Effort |
|:----------|:----------------------|:-------------------|:----------------|
| **Inference Engine** | vLLM (Custom Pod) | vLLM (via KServe HuggingFace Runtime) | Low - Same engine, different packaging |
| **Deployment Manifest** | Custom Helm Chart (`infra/helm/charts/vllm-deployment`) | InferenceService CRD | Medium - Template conversion |
| **Autoscaling** | Kubernetes HPA (CPU/Memory) | Knative Pod Autoscaler (Request-based/Concurrency) | Medium - Configuration change |
| **Scaling Trigger** | CPU/Memory thresholds | Request concurrency target | Medium - Metric change |
| **Scale-to-Zero** | Not supported (minReplicas ≥ 1) | Knative scale-to-zero | Low - Configuration flag |
| **Routing/Ingress** | Kubernetes Service + NodePort | KServe Predictor Service + Istio Ingress | High - Network architecture change |
| **Load Balancing** | K8s Service (round-robin) | Istio Envoy (advanced routing) | Medium - Transparent |
| **Model Registry** | Custom PostgreSQL Table | MLflow or Kubeflow Model Registry (Future) | High - Data migration and API change |
| **API Gateway** | `api-router-service` (OpenAI API) | Keep `api-router-service` + "OpenAI-to-KServe Adapter" | Medium - Protocol translation layer |
| **Model Versioning** | Helm release versions | InferenceService revisions | Low - Built-in |
| **Traffic Splitting** | Manual (deploy multiple services) | Istio VirtualService (automatic) | Medium - Configuration |
| **Health Checks** | Custom readiness/liveness probes | KServe predictor probes | Low - Similar implementation |
| **Metrics** | Custom vLLM metrics | KServe + Knative + Istio metrics | Medium - Metrics adapter |
| **Model Loading** | initContainer or entrypoint script | ClusterStorageContainer + runtime | Medium - Storage abstraction |
| **GPU Assignment** | NodeSelector + tolerations | Resource limits (`nvidia.com/gpu`) | Low - Same mechanism |
| **Admin Operations** | `admin-cli` (Helm release mgmt) | `admin-cli` (InferenceService mgmt) | High - CLI rewrite |

### Network Flow Changes

**Current Flow (Custom vLLM)**:
```
Client Request
  │
  ├─▶ API Router Service (NodePort or LoadBalancer)
  │    - Authenticates request
  │    - Meters usage
  │    - Routes to backend
  │
  └─▶ vLLM Service (ClusterIP)
       │
       └─▶ vLLM Pod (on GPU node)
            └─▶ Response
```

**New Flow (KServe)**:
```
Client Request
  │
  ├─▶ API Router Service (NodePort or LoadBalancer)
  │    - Authenticates request
  │    - Meters usage
  │    - Translates protocol (OpenAI ↔ KServe V2)
  │    - Routes to KServe endpoint
  │
  ├─▶ Istio Ingress Gateway
  │    - Applies VirtualService rules
  │    - Traffic splitting (canary)
  │    - Collects metrics
  │
  └─▶ Knative Service (managed by InferenceService)
       │
       ├─▶ Knative Activator (if scaled-to-zero, cold-start)
       │
       └─▶ vLLM Pod (on GPU node, managed by Knative)
            │
            └─▶ Response (via Istio)
                 │
                 └─▶ API Router (translates back to OpenAI)
                      │
                      └─▶ Client
```

### Data Flow: Model Loading

**Current (Custom Helm)**:
```
Helm Install
  │
  ├─▶ Deployment created
  │    - initContainer: download model from HF
  │    - mainContainer: vLLM with --model /models/<name>
  │
  └─▶ Pod scheduled on GPU node
       │
       ├─▶ initContainer runs: huggingface-cli download
       │    - Downloads to ephemeral volume or emptyDir
       │    - ~5-10min for 7B models
       │
       └─▶ mainContainer starts: vLLM loads model
            - Reads from volume
            - ~2-5min GPU initialization
            - Ready after ~7-15min total
```

**New (KServe + ClusterStorageContainer)**:
```
InferenceService Created
  │
  ├─▶ KServe Controller
  │    - Reads InferenceService spec
  │    - Resolves storageUri (hf://, s3://, etc)
  │    - Creates Knative Service
  │
  └─▶ Knative Service Reconcile
       │
       ├─▶ Storage Initializer (sidecar or init)
       │    - Uses ClusterStorageContainer credentials
       │    - Downloads model to PVC or emptyDir
       │    - ~5-10min for 7B models
       │
       └─▶ vLLM ServingRuntime (main container)
            - Mounts model from PVC
            - vLLM loads model
            - ~2-5min GPU initialization
            - Readiness probe succeeds
            - Knative marks Service ready
            - ~7-15min total (similar to current)
```

**Optimization (Model Caching)**:
```
First Deployment: 7-15min (download + load)
Subsequent Deploys: 2-5min (load from cache)
  │
  └─▶ PersistentVolumeClaim (shared across revisions)
       - modelCache: true in InferenceService
       - Model downloaded once, reused
       - Reduces cold-start significantly
```

## Data Model

See `data-model.md` for detailed schema definitions including:
- InferenceService CRD structure
- ClusterStorageContainer configuration
- ServingRuntime definitions
- Model registry schema (current vs future)
- Autoscaling parameters
- Metrics data model

## API Contracts

See `contracts/` directory for:
- `inference-service-template.yaml` - InferenceService CRD template for vLLM models
- `cluster-storage-container.yaml` - Storage configuration for Hugging Face and S3
- `serving-runtime-vllm.yaml` - Custom ServingRuntime for vLLM
- `kserve-v2-protocol.yaml` - KServe V2 Inference Protocol specification
- `openai-to-kserve-mapping.yaml` - Protocol translation mapping
- `autoscaling-policy.yaml` - Knative autoscaling configuration examples

## Implementation Tasks

See `tasks.md` for phased implementation plan including:
- Phase 1: Infrastructure setup (KServe, Istio, Knative installation)
- Phase 2: Pilot deployment (single model migration)
- Phase 3: API integration (api-router-service updates)
- Phase 4: Bulk migration (remaining models)
- Phase 5: Deprecation (remove custom Helm charts)

## Security Considerations

- **API Authentication**: Maintain existing API key and JWT authentication in api-router-service; KServe endpoints accessed internally only.
- **Network Isolation**: Use Kubernetes NetworkPolicies to restrict direct access to InferenceService pods; all external traffic must route through api-router-service.
- **Model Access Control**: Secure ClusterStorageContainer credentials using Kubernetes Secrets; restrict access to model storage (S3, Hugging Face tokens).
- **Multi-Tenancy**: Ensure InferenceServices deployed in separate namespaces per environment; use resource quotas to prevent resource monopolization.
- **TLS Encryption**: Enable Istio mutual TLS (mTLS) for inter-service communication; terminate external TLS at api-router-service or Istio ingress.
- **RBAC**: Restrict InferenceService management permissions; only authorized service accounts can create/update InferenceServices.
- **Secrets Management**: Rotate storage credentials regularly; avoid hardcoding secrets in manifests (use SealedSecrets or External Secrets Operator).
- **Model Validation**: Implement checksum verification for downloaded models; prevent model poisoning or tampering.
- **Rate Limiting**: Apply Istio rate limits at ingress; protect InferenceServices from DoS attacks.
- **Audit Logging**: Enable Kubernetes audit logs for InferenceService operations; track creation, updates, and deletions.

## Observability

### Metrics

**KServe Control Plane Metrics**:
- `kserve_reconcile_duration_seconds` - Time taken to reconcile InferenceService
- `kserve_inference_service_status` - Current status of InferenceServices (Ready, NotReady, Failed)
- `kserve_model_revision_active` - Active revision per model

**Knative Autoscaling Metrics**:
- `knative_serving_autoscaler_actual_pods` - Current pod count per InferenceService
- `knative_serving_autoscaler_desired_pods` - Desired pod count based on load
- `knative_serving_autoscaler_panic_mode` - Autoscaler in panic mode (rapid scale-up)
- `knative_serving_activator_request_count` - Requests handled during cold-start
- `knative_serving_revision_request_latencies` - Request latency per revision

**Istio Service Mesh Metrics**:
- `istio_requests_total{destination_service=~".*predictor.*"}` - Request count to InferenceService predictors
- `istio_request_duration_milliseconds{destination_service=~".*predictor.*"}` - Request duration
- `istio_request_bytes_sum` - Request payload size
- `istio_response_bytes_sum` - Response payload size

**vLLM Runtime Metrics** (custom exporter if needed):
- `vllm_num_requests_running` - Active inference requests
- `vllm_num_requests_waiting` - Queued requests
- `vllm_gpu_cache_usage_perc` - KV cache utilization
- `vllm_time_to_first_token_seconds` - TTFT latency
- `vllm_time_per_output_token_seconds` - Token generation speed
- `vllm_e2e_request_latency_seconds` - End-to-end latency

**API Router Service Metrics** (updated):
- `api_router_kserve_requests_total` - Requests routed to KServe backends
- `api_router_protocol_translation_duration_seconds` - Protocol conversion latency
- `api_router_backend_health{backend="kserve"}` - Health status of KServe endpoints

**Observability Mapping (Spec 011 Compliance)**:
- **Throughput**: Map `istio_requests_total` to Spec 011 `service_throughput`.
- **Latency**: Map `istio_request_duration_milliseconds` to Spec 011 `service_latency`.
- **Errors**: Map `istio_requests_total{response_code=~"5.*"}` to Spec 011 `service_error_rate`.
- **Saturation**: Map `knative_serving_autoscaler_actual_pods` / `maxReplicas` to Spec 011 `resource_saturation`.

### Dashboards

**Grafana Dashboards**:

1. **KServe Overview**
   - Total InferenceServices by status (Ready, NotReady, Failed)
   - InferenceService reconciliation errors
   - Model revision rollout status
   - Storage initializer duration

2. **Knative Autoscaling**
   - Current vs desired pod count per model
   - Autoscaling events (scale-up, scale-down, scale-to-zero)
   - Cold-start frequency and latency
   - Panic mode activations

3. **Inference Performance**
   - Request rate per model (via Istio)
   - P50/P90/P95/P99 latency per model
   - Error rate by model and HTTP status code
   - Request/response payload sizes

4. **Resource Utilization**
   - GPU utilization per InferenceService pod
   - CPU and memory usage per pod
   - Network ingress/egress per model
   - Storage I/O for model loading

5. **Traffic Splitting (Canary)**
   - Traffic distribution across revisions (v1: 90%, v2: 10%)
   - Error rate comparison between revisions
   - Latency comparison between revisions
   - Success rate per revision

6. **API Router Integration**
   - Protocol translation success rate
   - Backend routing decisions (KServe vs legacy)
   - End-to-end latency (client → api-router → KServe → client)
   - Authentication and billing events

### Logging

**Structured Logs (JSON format)**:

**KServe Controller Logs**:
```json
{
  "level": "info",
  "ts": "2025-01-27T10:30:00Z",
  "logger": "inferenceservice-controller",
  "msg": "Reconciling InferenceService",
  "inferenceservice": "llama-2-7b",
  "namespace": "development",
  "revision": "v2",
  "status": "Ready"
}
```

**Knative Autoscaler Logs**:
```json
{
  "level": "info",
  "ts": "2025-01-27T10:30:05Z",
  "logger": "autoscaler",
  "msg": "Scaling decision",
  "revision": "llama-2-7b-v1",
  "currentPods": 1,
  "desiredPods": 3,
  "avgConcurrency": 15.2,
  "targetConcurrency": 5
}
```

**vLLM Runtime Logs** (from pod):
```json
{
  "level": "info",
  "timestamp": "2025-01-27T10:30:10Z",
  "message": "Model loaded successfully",
  "model_name": "llama-2-7b",
  "model_size_gb": 13.5,
  "load_time_seconds": 178.3,
  "gpu_id": 0,
  "kv_cache_size_gb": 12.0
}
```

**Istio Access Logs**:
```json
{
  "start_time": "2025-01-27T10:30:15Z",
  "method": "POST",
  "path": "/v2/models/llama-2-7b/infer",
  "protocol": "HTTP/1.1",
  "response_code": 200,
  "duration_ms": 1250,
  "upstream_cluster": "outbound|8080||llama-2-7b-predictor.development.svc.cluster.local",
  "user_agent": "api-router-service/1.0"
}
```

**Log Aggregation**:
- All logs forwarded to centralized logging (Loki, Elasticsearch)
- Correlation IDs (`X-Request-ID`) propagated through entire request chain
- Log retention: 30 days for operational logs, 90 days for audit logs

## Testing Strategy

### Unit Tests

- **KServe Operator**: Not typically unit tested (Go controller), but can validate InferenceService CRD schema
- **API Router Protocol Translation**: Unit test OpenAI ↔ KServe V2 protocol conversion logic
- **Admin CLI**: Unit test InferenceService CRUD operations against mock Kubernetes API

### Integration Tests

- **KServe Deployment**: Deploy test InferenceService with small model (e.g., distilgpt2), verify readiness
- **Autoscaling**: Simulate load with concurrent requests, verify scale-up and scale-down behavior
- **API Router Integration**: Send OpenAI API requests through api-router-service to KServe backend, verify successful completion
- **Model Loading**: Test ClusterStorageContainer with Hugging Face public model, verify successful download and initialization
- **Health Checks**: Verify InferenceService readiness probes correctly reflect vLLM health

### End-to-End Tests

- **Full Migration Scenario**: Deploy model via custom Helm, create equivalent InferenceService, switch api-router backend, verify traffic, decomm Helm release
- **Canary Rollout**: Deploy InferenceService v1, deploy v2 with traffic split (90/10), validate both versions serve traffic, promote v2 to 100%
- **Scale-to-Zero Recovery**: Deploy InferenceService with minReplicas=0, wait for scale-down, send request, verify cold-start and successful response
- **Multi-Model Load**: Deploy 5 InferenceServices simultaneously, verify all reach ready state, send concurrent requests to all models
- **Failure Recovery**: Simulate node failure (drain GPU node), verify InferenceService pods reschedule to available GPU nodes

### Performance Tests

- **Latency Comparison**: Baseline current vLLM deployment latency, measure KServe deployment latency, verify <10% regression
- **Throughput**: Send sustained load (50 concurrent users), measure requests/sec for both deployments
- **Cold-Start Latency**: Measure time from scale-to-zero to first successful response for models of varying sizes
- **Autoscaling Response Time**: Measure time from load increase to additional pods becoming ready
- **Load Testing Harness**: Use `specs/014-load-testing-harness` to validate User Story 1 (Autoscaling) and User Story 2 (Scale-to-Zero) under realistic load patterns.

### Chaos Tests (Optional)

- **Network Partition**: Simulate Istio ingress failure, verify api-router circuit breaker activates
- **Storage Failure**: Simulate ClusterStorageContainer credentials expiry, verify graceful error handling
- **GPU Exhaustion**: Deploy more InferenceServices than available GPU capacity, verify proper queuing and error messages
- **Control Plane Failure**: Restart KServe controller during active reconciliation, verify InferenceServices eventually reach desired state

## Migration Runbook

See `docs/runbooks/kserve-migration-runbook.md` for detailed step-by-step procedures including:
- Pre-migration checklist
- Infrastructure deployment steps
- Model-by-model migration procedures
- Rollback procedures
- Validation checkpoints
- Post-migration verification

## Risk Mitigation

| Risk | Impact | Probability | Mitigation |
|:-----|:-------|:-----------|:-----------|
| Cold-start latency exceeds SLA | High | Medium | Configure `minReplicas: 1` for production models; implement model caching |
| KServe V2 protocol incompatibility with clients | Medium | Low | Implement robust protocol adapter in api-router-service; maintain backward compatibility |
| Istio resource overhead impacts performance | Medium | Medium | Right-size Istio resource requests; monitor control plane metrics; use Istio minimal profile |
| GPU scheduling conflicts during migration | High | Medium | Migrate models during low-traffic periods; maintain parallel deployments briefly |
| Model loading failures due to storage issues | High | Low | Test ClusterStorageContainer thoroughly in dev; implement retries and timeouts |
| Autoscaler thrashing (rapid scale up/down) | Medium | Medium | Tune Knative autoscaler parameters (`stable-window`, `scale-down-delay`); set appropriate `target` concurrency |
| Loss of custom vLLM metrics | Low | High | Implement custom metrics exporter sidecar; extend ServingRuntime if needed |
| Admin CLI rewrite delays migration | Medium | Medium | Prioritize core CRUD operations; maintain kubectl access as fallback |
| Team unfamiliarity with KServe/Knative | Medium | High | Conduct training sessions; provide comprehensive documentation; pair with experienced engineer |
| Rollback complexity if migration fails | High | Low | Maintain custom Helm charts until migration fully validated; test rollback procedure in dev |

## Open Questions

- **Q1**: Should we use scale-to-zero (`minReplicas: 0`) for production models, or set `minReplicas: 1` to avoid cold-start latency?
  - **Recommendation**: Start with `minReplicas: 1` for production, evaluate cost savings vs latency trade-off after observing usage patterns

- **Q2**: Which model registry should we evaluate - MLflow or Kubeflow Model Registry?
  - **Recommendation**: Start with MLflow (more mature, better community support); evaluate Kubeflow if deeper Kubeflow integration is needed

- **Q3**: Should api-router-service handle protocol translation, or should we deploy a separate protocol adapter service?
  - **Recommendation**: Implement protocol translation directly in api-router-service to minimize additional hops and latency

- **Q4**: What is the target concurrency value for Knative autoscaler (`target` parameter)?
  - **Recommendation**: Start with `target: 5` (5 concurrent requests per pod), tune based on observed GPU utilization and latency

- **Q5**: Should we use Istio's full installation or minimal profile?
  - **Recommendation**: Use minimal profile (Istio core + ingress gateway only) to reduce resource overhead; add components as needed

- **Q6**: How to handle commercial models requiring license validation?
  - **Recommendation**: Implement initContainer in ServingRuntime to validate licenses before model loading; fail fast on invalid license

## Future Enhancements

- **Multi-Model Serving**: Deploy multiple small models in a single InferenceService pod for resource efficiency
- **Model Ensemble**: Combine multiple models (e.g., embedding + LLM + reranker) in a single InferenceService pipeline
- **Advanced Traffic Management**: A/B testing with user segmentation, shadow traffic for safe validation
- **Cost Optimization**: Implement intelligent model caching, spot instance support for non-production workloads
- **Model Warmup**: Pre-warm models before accepting traffic to reduce perceived cold-start latency
- **Custom Metrics Autoscaling**: Scale based on custom metrics (e.g., GPU utilization, queue depth) instead of concurrency
- **Cross-Cluster Serving**: Deploy InferenceServices across multiple clusters with global load balancing
- **Model Registry UI**: Web interface for browsing available models, versions, and deployment status
- **Automated Canary Analysis**: Integrate with Flagger for automated progressive delivery with rollback on errors

## References

- [KServe Documentation](https://kserve.github.io/website/)
- [Knative Serving Documentation](https://knative.dev/docs/serving/)
- [Istio Documentation](https://istio.io/latest/docs/)
- [vLLM Documentation](https://docs.vllm.ai/)
- [KServe V2 Inference Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)
- [Knative Autoscaling](https://knative.dev/docs/serving/autoscaling/)
- [GPU Operator for Kubernetes](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/getting-started.html)
