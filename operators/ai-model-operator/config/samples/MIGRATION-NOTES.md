# Migration Notes: InferenceService → AIModel CR

## Mistral-7B-Instruct Staging Migration

This document tracks the migration of the Mistral-7B-Instruct staging deployment from a manual InferenceService to an AIModel CR.

### Original InferenceService

**File**: `infra/k8s/kserve/models/mistral-7b-instruct-staging.yaml`

**Key configurations:**
- Namespace: `staging`
- Name: `mistral-7b-instruct-staging`
- Deployment mode: Serverless (Knative)
- Progress deadline: 360s (6 minutes for model loading)
- Autoscaling: KPA with concurrency metric
- Min replicas: 1
- Max replicas: 3
- Container image: `vllm/vllm-openai:v0.6.3`
- GPU: 1x nvidia.com/gpu
- CPU requests: 4 cores, limits: 8 cores
- Memory requests: 16Gi, limits: 32Gi

### New AIModel CR

**File**: `config/samples/mistral-7b-instruct-staging.yaml`

**Mapped configurations:**
- Namespace: `staging` ✓
- Name: `mistral-7b-instruct` (operator adds namespace suffix if needed)
- Runtime: `vllm` (operator selects appropriate image)
- MinReplicas: `1` ✓
- MaxReplicas: `3` ✓
- GPU: `nvidia.com/gpu: "1"` ✓
- CPU/Memory: Same requests/limits ✓
- Tolerations: Identical ✓
- Runtime args: All preserved ✓
- Runtime env: `HF_HOME` preserved ✓

### Operator Responsibilities

The AI Model Operator will handle these InferenceService configurations automatically:

1. **Knative annotations** - Operator adds:
   - `serving.knative.dev/progress-deadline: "360s"`
   - `autoscaling.knative.dev/class: "kpa.autoscaling.knative.dev"`
   - `autoscaling.knative.dev/metric: "concurrency"`
   - Other Knative autoscaling settings

2. **Health probes** - Operator configures:
   - `startupProbe` with 6-minute timeout for model loading
   - `readinessProbe` for traffic management
   - `livenessProbe` for pod health

3. **Container image** - Operator selects:
   - `vllm/vllm-openai:v0.6.3` based on `runtime: vllm`

4. **Port configuration**:
   - Container port 8000 exposed as `http1` (KServe requirement)

### Differences and Rationale

| Aspect | InferenceService | AIModel CR | Rationale |
|--------|------------------|------------|-----------|
| Name | `mistral-7b-instruct-staging` | `mistral-7b-instruct` | Namespace already indicates environment |
| Annotations | Explicit Knative config | Omitted | Operator adds standard Knative config |
| Health probes | Explicit configuration | Omitted | Operator adds standard probes |
| Container image | Explicit version | Via `runtime` field | Operator manages image versions |
| S3 fields | Not present | Empty strings | CRD requires them (validation) |

### Validation Checklist

When the operator creates the InferenceService from this AIModel CR, verify:

- [ ] InferenceService name matches AIModel name
- [ ] InferenceService namespace is `staging`
- [ ] Knative annotations are present (progress-deadline, autoscaling)
- [ ] Health probes are configured (startup, readiness, liveness)
- [ ] Container uses vLLM image (v0.6.3 or operator default)
- [ ] GPU tolerations are applied
- [ ] Resource requests/limits match
- [ ] Runtime args are passed to container
- [ ] Environment variables are set
- [ ] Min/max replicas match
- [ ] InferenceService becomes Ready
- [ ] Model inference endpoint works

### Testing Plan

1. **Apply AIModel CR**:
   ```bash
   kubectl apply -f config/samples/mistral-7b-instruct-staging.yaml
   ```

2. **Check AIModel status**:
   ```bash
   kubectl get aimodel mistral-7b-instruct -n staging -w
   ```

3. **Verify InferenceService created**:
   ```bash
   kubectl get inferenceservice -n staging
   kubectl describe inferenceservice mistral-7b-instruct -n staging
   ```

4. **Check Knative Revision**:
   ```bash
   kubectl get revision -n staging | grep mistral
   ```

5. **Test inference endpoint**:
   ```bash
   ENDPOINT=$(kubectl get aimodel mistral-7b-instruct -n staging -o jsonpath='{.status.inferenceEndpoint}')
   curl -X POST "${ENDPOINT}/v1/chat/completions" \
     -H "Content-Type: application/json" \
     -d '{
       "model": "mistral-7b-instruct",
       "messages": [{"role": "user", "content": "Hello!"}],
       "max_tokens": 50
     }'
   ```

6. **Compare with old endpoint** (before deletion):
   ```bash
   # Old endpoint should produce same results
   kubectl get inferenceservice mistral-7b-instruct-staging -n staging -o jsonpath='{.status.url}'
   ```

7. **Delete old InferenceService** (only after successful validation):
   ```bash
   kubectl delete -f infra/k8s/kserve/models/mistral-7b-instruct-staging.yaml
   ```

### Rollback Plan

If the AIModel CR doesn't work as expected:

1. Keep the old InferenceService YAML file
2. Re-apply the old InferenceService:
   ```bash
   kubectl apply -f infra/k8s/kserve/models/mistral-7b-instruct-staging.yaml
   ```
3. Delete the AIModel CR:
   ```bash
   kubectl delete aimodel mistral-7b-instruct -n staging
   ```
4. Report issues to the operator development team

### Open Questions

1. **Image version management**: Should the operator pin vLLM version or use latest?
   - Currently: Operator uses a default version
   - Consider: Add `runtimeVersion` field to AIModel spec

2. **Knative autoscaling tuning**: Should these be exposed in AIModel spec?
   - `autoscaling.knative.dev/target`
   - `autoscaling.knative.dev/scaleDownDelay`
   - Consider: Add `autoscaling` section to AIModel spec

3. **Name collision**: What if both manual InferenceService and AIModel exist?
   - Operator behavior: Should fail or overwrite?
   - Recommendation: Check for existing InferenceService not owned by operator
