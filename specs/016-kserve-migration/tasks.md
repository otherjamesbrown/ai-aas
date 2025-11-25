# Implementation Tasks: KServe Migration

This document outlines the phased implementation plan for migrating from custom vLLM Helm charts to KServe InferenceServices.

## Overview

- **Total Phases**: 5
- **Estimated Duration**: 6-8 weeks
- **Risk Level**: Medium-High (requires careful coordination and validation)

---

## Phase 1: Infrastructure Setup (Week 1-2)

**Goal**: Deploy KServe, Istio, and Knative to the development cluster with GPU support.

### Tasks

#### 1.1 Install Istio (Service Mesh)

- **Owner**: Platform Team
- **Effort**: 2 days
- **Dependencies**: None

**Steps**:
1. Choose Istio profile (recommend `minimal` for reduced overhead)
2. Create Helm values for Istio installation
   - Location: `infra/helm/charts/istio/values-development.yaml`
   - Configure ingress gateway
   - Enable telemetry/metrics
3. Create ArgoCD Application for Istio
   - Location: `gitops/clusters/development/apps/istio.yaml`
4. Deploy via GitOps: `git commit && git push`
5. Verify Istio control plane health:
   ```bash
   kubectl get pods -n istio-system
   kubectl get svc -n istio-system istio-ingressgateway
   ```

**Acceptance Criteria**:
- [X] Istio control plane pods healthy
- [X] Istio ingress gateway accessible
- [ ] Istio sidecar injection working (test with sample app) *(Pending deployment)*

---

#### 1.2 Install Knative Serving

- **Owner**: Platform Team
- **Effort**: 2 days
- **Dependencies**: Istio installed

**Steps**:
1. Install Knative Serving CRDs
   - Version: v1.11+ (check compatibility with KServe)
2. Install Knative Serving core
3. Configure Knative to use Istio as networking layer
   - Edit `config-network` ConfigMap: `ingress.class: istio.ingress.networking.knative.dev`
4. Configure Knative domain
   - Domain: `*.dev.ai-aas.local` for development
5. Create ArgoCD Application
   - Location: `gitops/clusters/development/apps/knative-serving.yaml`
6. Deploy via GitOps

**Acceptance Criteria**:
- [X] Knative controller and webhook pods healthy
- [X] Knative activator and autoscaler running
- [ ] Sample Knative Service deploys successfully *(Pending deployment)*

---

#### 1.3 Install KServe

- **Owner**: Platform Team
- **Effort**: 2 days
- **Dependencies**: Knative Serving installed

**Steps**:
1. Install KServe CRDs
   - Version: v0.11+ (latest stable)
2. Install KServe controller manager
3. Configure KServe for serverless mode
   - Edit `inferenceservice-config` ConfigMap: `deploy.defaultDeploymentMode: "Serverless"`
4. Create ArgoCD Application
   - Location: `gitops/clusters/development/apps/kserve.yaml`
5. Deploy via GitOps
6. Verify KServe controller:
   ```bash
   kubectl get pods -n kserve
   kubectl get crd inferenceservices.serving.kserve.io
   ```

**Acceptance Criteria**:
- [X] KServe controller pod healthy
- [X] InferenceService CRD available
- [ ] Sample InferenceService creates Knative Service *(Pending deployment)*

---

#### 1.4 Configure GPU Support

- **Owner**: Platform Team
- **Effort**: 1 day
- **Dependencies**: KServe installed

**Steps**:
1. Verify NVIDIA GPU Operator installed
   ```bash
   kubectl get pods -n gpu-operator-resources
   ```
2. Label GPU nodes for KServe scheduling
   ```bash
   kubectl label nodes <gpu-node-name> kserve.io/gpu=true
   ```
3. Create GPU-enabled StorageClass (if needed for model caching)
4. Test GPU scheduling with sample InferenceService

**Acceptance Criteria**:
- [X] GPU nodes correctly labeled *(Configuration documented)*
- [ ] Sample GPU InferenceService scheduled on GPU node *(Pending deployment)*
- [ ] `nvidia-smi` accessible in InferenceService pod *(Pending deployment)*

---

#### 1.5 Configure ClusterStorageContainer

- **Owner**: Platform Team
- **Effort**: 1 day
- **Dependencies**: KServe installed

**Steps**:
1. Create Hugging Face token secret
   ```bash
   kubectl create secret generic huggingface-secret \
     --from-literal=token=$HF_TOKEN \
     -n development
   ```
2. Create ClusterStorageContainer for Hugging Face
   - Location: `infra/k8s/kserve/cluster-storage-container-hf.yaml`
3. Create ClusterStorageContainer for S3 (future private models)
   - Location: `infra/k8s/kserve/cluster-storage-container-s3.yaml`
4. Deploy via GitOps

**Acceptance Criteria**:
- [X] ClusterStorageContainer resources created
- [ ] InferenceService with `hf://` URI successfully downloads model *(Pending deployment)*
- [ ] Storage initializer logs show successful download *(Pending deployment)*

---

### Phase 1 Validation

**Steps**:
1. Deploy test InferenceService with small public model (distilgpt2)
   ```yaml
   apiVersion: serving.kserve.io/v1beta1
   kind: InferenceService
   metadata:
     name: test-distilgpt2
     namespace: development
   spec:
     predictor:
       model:
         modelFormat:
           name: pytorch
         storageUri: hf://distilgpt2
       resources:
         requests:
           cpu: "1"
           memory: "2Gi"
         limits:
           cpu: "2"
           memory: "4Gi"
   ```
2. Wait for InferenceService to become ready
3. Send test inference request
4. Verify response

**Success Criteria**:
- [ ] InferenceService reaches `Ready` status within 10 minutes
- [ ] Model pod runs on appropriate node (CPU or GPU)
- [ ] Inference request returns valid response
- [ ] No errors in controller logs

---

## Phase 2: Pilot Model Migration (Week 3)

**Goal**: Migrate Llama-2-7b from custom Helm chart to KServe InferenceService.

### Tasks

#### 2.1 Create InferenceService Template for vLLM

- **Owner**: ML Platform Team
- **Effort**: 2 days
- **Dependencies**: Phase 1 complete

**Steps**:
1. Define vLLM ServingRuntime (optional, or use built-in)
   - Location: `infra/k8s/kserve/serving-runtime-vllm.yaml`
2. Create InferenceService template
   - Location: `infra/k8s/kserve/templates/inference-service-vllm-template.yaml`
3. Include vLLM-specific args:
   ```yaml
   args:
     - --model=/mnt/models
     - --dtype=float16
     - --max-model-len=4096
     - --gpu-memory-utilization=0.9
     - --trust-remote-code
   ```
4. Configure GPU resources and node affinity
5. Set autoscaling parameters:
   - `minReplicas: 1` (no scale-to-zero for pilot)
   - `maxReplicas: 5`
   - `scaleTarget: 5` (5 concurrent requests per pod)

**Acceptance Criteria**:
- [ ] Template is parameterized (model name, version, resources)
- [ ] Template includes all vLLM configuration
- [ ] Template validated with `kubectl --dry-run=client`

---

#### 2.2 Deploy Llama-2-7b via InferenceService

- **Owner**: ML Platform Team
- **Effort**: 1 day
- **Dependencies**: 2.1 complete

**Steps**:
1. Create InferenceService manifest for llama-2-7b
   - Location: `infra/k8s/kserve/models/llama-2-7b.yaml`
   - Storage URI: `hf://meta-llama/Llama-2-7b-hf`
2. Deploy via GitOps
3. Monitor deployment:
   ```bash
   kubectl get inferenceservice llama-2-7b -n development -w
   kubectl get pods -n development -l serving.kserve.io/inferenceservice=llama-2-7b
   ```
4. Check logs for model loading:
   ```bash
   kubectl logs -n development <pod-name> -c kserve-container
   ```

**Acceptance Criteria**:
- [ ] InferenceService reaches `Ready` status within 15 minutes
- [ ] vLLM successfully loads model
- [ ] Pod scheduled on GPU node
- [ ] Readiness probe succeeds

---

#### 2.3 Verify Inference via KServe Endpoint

- **Owner**: ML Platform Team
- **Effort**: 1 day
- **Dependencies**: 2.2 complete

**Steps**:
1. Get InferenceService URL:
   ```bash
   kubectl get inferenceservice llama-2-7b -n development \
     -o jsonpath='{.status.url}'
   ```
2. Port-forward to predictor service:
   ```bash
   kubectl port-forward -n development \
     svc/llama-2-7b-predictor 8080:80
   ```
3. Send test inference request (KServe V2 protocol):
   ```bash
   curl -X POST http://localhost:8080/v2/models/llama-2-7b/infer \
     -H "Content-Type: application/json" \
     -d '{
       "id": "test-123",
       "inputs": [{
         "name": "prompt",
         "shape": [1],
         "datatype": "BYTES",
         "data": ["Once upon a time"]
       }],
       "parameters": {
         "max_tokens": 50,
         "temperature": 0.7
       }
     }'
   ```
4. Verify response contains generated text

**Acceptance Criteria**:
- [ ] Request returns HTTP 200
- [ ] Response contains valid completion
- [ ] Latency < 5 seconds for short prompt
- [ ] No errors in pod logs

---

#### 2.4 Validate Autoscaling

- **Owner**: ML Platform Team
- **Effort**: 1 day
- **Dependencies**: 2.3 complete

**Steps**:
1. Monitor initial pod count:
   ```bash
   kubectl get pods -n development -l serving.kserve.io/inferenceservice=llama-2-7b
   ```
2. Generate load (10-20 concurrent requests):
   ```bash
   for i in {1..20}; do
     curl -X POST http://localhost:8080/v2/models/llama-2-7b/infer \
       -H "Content-Type: application/json" -d '<payload>' &
   done
   ```
3. Monitor autoscaling:
   ```bash
   kubectl get pods -n development -l serving.kserve.io/inferenceservice=llama-2-7b -w
   ```
4. Verify scale-up occurs
5. Stop load and verify scale-down after 5 minutes

**Acceptance Criteria**:
- [ ] Knative autoscaler scales up under load
- [ ] Additional pods become ready within 2 minutes
- [ ] After load stops, pods scale down to `minReplicas`
- [ ] No request failures during scaling

---

### Phase 2 Validation

**Success Criteria**:
- [ ] Llama-2-7b InferenceService stable for 24 hours
- [ ] Inference latency within 10% of baseline (custom Helm deployment)
- [ ] Autoscaling works as expected
- [ ] No OOMKilled or CrashLoopBackOff errors
- [ ] Grafana dashboards show metrics

---

## Phase 3: API Router Integration (Week 4)

**Goal**: Update api-router-service to route requests to KServe endpoint with protocol translation.

### Tasks

#### 3.1 Implement Protocol Translation (OpenAI ↔ KServe V2)

- **Owner**: Backend Team
- **Effort**: 3 days
- **Dependencies**: Phase 2 complete

**Steps**:
1. Create protocol adapter module
   - Location: `services/api-router-service/internal/adapter/kserve/`
   - Files: `translator.go`, `types.go`, `translator_test.go`
2. Implement request translation:
   ```go
   // OpenAI ChatCompletionRequest → KServe InferRequest
   func TranslateOpenAIToKServe(req *openai.ChatCompletionRequest) (*kserve.InferRequest, error)
   ```
3. Implement response translation:
   ```go
   // KServe InferResponse → OpenAI ChatCompletionResponse
   func TranslateKServeToOpenAI(resp *kserve.InferResponse) (*openai.ChatCompletionResponse, error)
   ```
4. Write unit tests for translation logic
5. Handle edge cases:
   - Streaming responses (if supported)
   - Error responses
   - Token counting

**Acceptance Criteria**:
- [ ] Unit tests pass with 100% coverage
- [ ] Translation preserves all OpenAI fields
- [ ] Error handling is robust

---

#### 3.2 Update Backend Configuration Schema

- **Owner**: Backend Team
- **Effort**: 1 day
- **Dependencies**: 3.1 in progress

**Steps**:
1. Update `BackendConfig` struct:
   ```go
   type BackendConfig struct {
       Name              string
       Type              BackendType  // "vllm", "kserve"
       Endpoint          string
       Protocol          ProtocolType // "openai", "kserve-v2"
       ProtocolAdapter   string       // "none", "openai"
       KnativeService    bool
       ColdStartTimeout  time.Duration
       HealthCheckPath   string
       Timeout           time.Duration
   }
   ```
2. Update Helm values schema
   - Location: `services/api-router-service/deployments/helm/api-router-service/values.yaml`
3. Update validation logic

**Acceptance Criteria**:
- [ ] Schema supports both legacy and KServe backends
- [ ] Backward compatibility maintained
- [ ] Helm values validation passes

---

#### 3.3 Configure Llama-2-7b KServe Backend

- **Owner**: Backend Team
- **Effort**: 1 day
- **Dependencies**: 3.2 complete

**Steps**:
1. Update `values-development.yaml`:
   ```yaml
   backends:
     # Keep existing custom vLLM backend (parallel deployment)
     - name: llama-2-7b-helm
       type: vllm
       endpoint: http://llama-2-7b-vllm.system.svc.cluster.local:8000
       protocol: openai
       enabled: true  # Keep active for rollback

     # Add new KServe backend
     - name: llama-2-7b
       type: kserve
       endpoint: http://llama-2-7b-predictor.development.svc.cluster.local/v2/models/llama-2-7b/infer
       protocol: kserve-v2
       protocolAdapter: openai
       knativeService: true
       coldStartTimeout: 60s
       healthCheckPath: /v2/health/ready
       timeout: 30s
       enabled: true
   ```
2. Deploy api-router-service update via GitOps
3. Verify both backends registered:
   ```bash
   kubectl logs -n development -l app.kubernetes.io/name=api-router-service | grep "Registered backend"
   ```

**Acceptance Criteria**:
- [ ] api-router-service starts successfully
- [ ] Both backends appear in `/v1/models` endpoint
- [ ] Health checks pass for both backends

---

#### 3.4 End-to-End Testing via API Router

- **Owner**: Backend Team + QA
- **Effort**: 2 days
- **Dependencies**: 3.3 complete

**Steps**:
1. Send OpenAI API request through api-router-service:
   ```bash
   curl -X POST https://api.dev.ai-aas.local/v1/chat/completions \
     -H "Authorization: Bearer $API_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "model": "llama-2-7b",
       "messages": [
         {"role": "user", "content": "Hello, world!"}
       ],
       "max_tokens": 50
     }'
   ```
2. Verify request routes to KServe backend
3. Verify response is valid OpenAI format
4. Check api-router logs for protocol translation
5. Run load test (50 concurrent users):
   ```bash
   locust -f tests/load/openai_chat.py \
     --host https://api.dev.ai-aas.local \
     --users 50 --spawn-rate 5 --run-time 10m
   ```
6. Verify no errors, latency acceptable

**Acceptance Criteria**:
- [ ] 100% of requests succeed
- [ ] P95 latency < 1.5x baseline
- [ ] Protocol translation transparent to clients
- [ ] No KServe-specific errors exposed to clients

---

#### 3.5 Implement Traffic Splitting (Optional)

- **Owner**: Backend Team
- **Effort**: 1 day (optional)
- **Dependencies**: 3.4 complete

**Steps**:
1. Implement weighted routing in api-router-service:
   ```yaml
   backends:
     - name: llama-2-7b-helm
       weight: 90  # 90% traffic to old backend
       enabled: true

     - name: llama-2-7b
       weight: 10  # 10% traffic to KServe backend
       enabled: true
   ```
2. Deploy and validate traffic distribution
3. Gradually shift traffic: 50/50 → 10/90 → 0/100
4. Monitor error rates and latency during shift

**Acceptance Criteria**:
- [ ] Traffic split matches configuration
- [ ] No increase in error rate during shift
- [ ] Latency remains stable

---

### Phase 3 Validation

**Success Criteria**:
- [ ] End-to-end flow works: Client → API Router → KServe → Client
- [ ] Protocol translation is transparent
- [ ] P95 latency within SLA (<5s for short prompts)
- [ ] Error rate <0.1%
- [ ] Billing and metering work correctly

---

## Phase 4: Bulk Migration (Week 5-6)

**Goal**: Migrate remaining models from custom Helm charts to KServe.

### Tasks

#### 4.1 Create Migration Priority List

- **Owner**: ML Platform Team
- **Effort**: 1 day
- **Dependencies**: Phase 3 complete

**Steps**:
1. List all deployed models:
   ```bash
   helm list -A | grep vllm-deployment
   ```
2. Prioritize by:
   - Traffic volume (low-traffic first)
   - Model size (smaller first)
   - Criticality (non-production first)
3. Create migration schedule (1-2 models per day)

**Suggested Order**:
1. Mistral-7b (medium traffic)
2. CodeLlama-7b (low traffic)
3. Additional models...

---

#### 4.2 Migrate Models One-by-One

- **Owner**: ML Platform Team
- **Effort**: 1-2 days per model
- **Dependencies**: 4.1 complete

**Steps** (per model):
1. Create InferenceService manifest from template
2. Deploy InferenceService via GitOps
3. Wait for `Ready` status
4. Verify inference via kubectl port-forward
5. Update api-router-service backend config
6. Deploy api-router update
7. Run smoke tests
8. Monitor for 24 hours
9. If stable, deprecate Helm release:
   ```bash
   helm uninstall <model-name>-vllm -n system
   ```

**Acceptance Criteria** (per model):
- [ ] InferenceService stable
- [ ] E2E inference via api-router works
- [ ] Latency within SLA
- [ ] No errors after 24 hours

---

### Phase 4 Validation

**Success Criteria**:
- [ ] All models migrated successfully
- [ ] All custom Helm releases removed
- [ ] Platform-wide error rate unchanged
- [ ] No degradation in P95 latency
- [ ] Cost savings observed (scale-to-zero for low-traffic models)

---

## Phase 5: Cleanup and Optimization (Week 7-8)

**Goal**: Remove legacy infrastructure, optimize configuration, and document.

### Tasks

#### 5.1 Remove Custom Helm Charts

- **Owner**: Platform Team
- **Effort**: 1 day
- **Dependencies**: Phase 4 complete, all models migrated

**Steps**:
1. Archive custom Helm chart:
   ```bash
   mkdir -p archive/helm-charts
   mv infra/helm/charts/vllm-deployment archive/helm-charts/
   ```
2. Update documentation to mark as deprecated
3. Remove ArgoCD Applications for Helm-deployed models
4. Clean up orphaned resources

**Acceptance Criteria**:
- [ ] No active Helm releases for vLLM models
- [ ] Custom Helm chart archived
- [ ] Documentation updated

---

#### 5.2 Optimize Autoscaling Configuration

- **Owner**: ML Platform Team
- **Effort**: 2 days
- **Dependencies**: 5.1 complete

**Steps**:
1. Analyze 2 weeks of metrics:
   - Request patterns (peak vs off-peak)
   - Concurrency levels
   - Cold-start frequency
2. Tune autoscaling parameters:
   - Adjust `scaleTarget` based on observed concurrency
   - Adjust `scaleDownDelay` to reduce flapping
   - Consider scale-to-zero for low-traffic models
3. Update InferenceService manifests
4. Deploy and monitor for 1 week

**Acceptance Criteria**:
- [ ] Autoscaling behavior improved
- [ ] Cold-start frequency reduced
- [ ] Resource utilization optimized (cost savings)

---

#### 5.3 Update admin-cli for KServe Management

- **Owner**: CLI Team
- **Effort**: 3 days
- **Dependencies**: Phase 4 complete

**Steps**:
1. Add KServe commands to admin-cli:
   ```bash
   admin-cli model deploy --name llama-2-7b --runtime kserve --storage-uri hf://...
   admin-cli model list --runtime kserve
   admin-cli model describe llama-2-7b
   admin-cli model delete llama-2-7b
   ```
2. Implement InferenceService CRUD operations
3. Add integration tests
4. Update CLI documentation

**Acceptance Criteria**:
- [ ] CLI commands work for InferenceService management
- [ ] Integration tests pass
- [ ] Documentation complete

---

#### 5.4 Documentation and Runbooks

- **Owner**: Platform Team + Technical Writer
- **Effort**: 2 days
- **Dependencies**: All tasks complete

**Steps**:
1. Update platform documentation:
   - `docs/architecture/model-serving.md` - Update architecture diagrams
   - `docs/guides/deploy-model-kserve.md` - New guide for KServe deployment
   - `docs/runbooks/kserve-migration-runbook.md` - Migration procedures
   - `docs/runbooks/troubleshooting-kserve.md` - Common issues and fixes
2. Create Grafana dashboards:
   - KServe InferenceServices overview
   - Knative autoscaling metrics
   - Inference performance by model
3. Record demo video (optional)

**Acceptance Criteria**:
- [ ] All documentation updated
- [ ] Runbooks cover common scenarios
- [ ] Grafana dashboards deployed
- [ ] Knowledge transfer session conducted

---

### Phase 5 Validation

**Success Criteria**:
- [ ] Legacy infrastructure fully removed
- [ ] Platform optimized for KServe
- [ ] Operations team trained on KServe management
- [ ] Documentation complete and accessible
- [ ] Post-migration review conducted

---

## Rollback Procedures

### Rollback from Phase 2 (Pilot)

If Llama-2-7b KServe deployment fails:
1. Disable `llama-2-7b` backend in api-router-service (`enabled: false`)
2. Deploy api-router update
3. Keep custom Helm deployment active
4. Investigate issues, fix, and retry

### Rollback from Phase 3 (API Router Integration)

If protocol translation fails:
1. Revert api-router-service to previous version via GitOps
2. Remove KServe backend from configuration
3. Investigate and fix translation bugs

### Rollback from Phase 4 (Bulk Migration)

If multiple models fail:
1. Pause migration
2. Identify root cause
3. Revert affected models:
   - Disable KServe backends in api-router-service
   - Redeploy Helm releases for affected models
4. Fix issues before continuing

### Emergency Rollback (Full)

If KServe platform has critical issues:
1. Disable all KServe backends in api-router-service
2. Redeploy all models via custom Helm charts
3. Keep KServe infrastructure for investigation
4. Schedule post-incident review

---

## Risk Management

| Risk | Mitigation |
|:-----|:-----------|
| Cold-start latency exceeds SLA | Use `minReplicas: 1` for production; implement model caching |
| Autoscaling thrashing | Tune `scaleDownDelay` and `target` concurrency |
| Protocol translation bugs | Comprehensive unit tests; parallel deployment during transition |
| GPU node exhaustion | Implement resource quotas; monitor node capacity |
| Model loading failures | Test ClusterStorageContainer thoroughly; implement retries |
| Team unfamiliarity with KServe | Training sessions; pair programming; runbooks |

---

## Success Metrics

| Metric | Baseline (Custom Helm) | Target (KServe) | Actual |
|:-------|:-----------------------|:----------------|:-------|
| **Deployment Time** | 10-15 min | 10-15 min | TBD |
| **P95 Inference Latency** | <3s | <3.3s (+10%) | TBD |
| **Error Rate** | <0.1% | <0.1% | TBD |
| **Cost (Idle Models)** | 100% | 20-30% (scale-to-zero) | TBD |
| **Autoscaling Response** | 5-10 min (HPA) | 30-60s (Knative) | TBD |
| **Operational Overhead** | High (custom Helm per model) | Low (standardized CRD) | TBD |

---

## Timeline Summary

| Phase | Duration | Cumulative | Key Deliverable |
|:------|:---------|:-----------|:----------------|
| **Phase 1: Infrastructure** | 2 weeks | Week 2 | KServe, Istio, Knative deployed and validated |
| **Phase 2: Pilot** | 1 week | Week 3 | Llama-2-7b running on KServe |
| **Phase 3: API Integration** | 1 week | Week 4 | E2E flow via api-router-service |
| **Phase 4: Bulk Migration** | 2 weeks | Week 6 | All models migrated |
| **Phase 5: Cleanup** | 2 weeks | Week 8 | Legacy infrastructure removed, documentation complete |

---

## References

- [KServe Quickstart](https://kserve.github.io/website/latest/get_started/)
- [Knative Autoscaling Configuration](https://knative.dev/docs/serving/autoscaling/)
- [vLLM Configuration Options](https://docs.vllm.ai/en/latest/serving/deploying_with_docker.html)
- [KServe V2 Protocol Spec](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)
