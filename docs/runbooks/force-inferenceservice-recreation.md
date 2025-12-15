# Runbook: Force InferenceService Recreation

**Feature**: Operator maintenance
**Last Updated**: 2025-12-15
**Audience**: Platform Operators

## Overview

This runbook documents how to force recreation of KServe InferenceServices when the AI Model Operator code is updated. Existing InferenceServices do not automatically pick up changes to operator configuration (such as probe configs, runtime args, or image updates) until they are recreated.

## When to Use This Runbook

Use this procedure when:

1. **Operator probe configuration updated**: Changed startup/liveness/readiness probe settings
2. **Runtime image updated**: Changed default vLLM/TGI/Triton image versions
3. **Runtime args changed**: Modified default arguments passed to inference runtimes
4. **Resource defaults updated**: Changed default CPU/memory/GPU allocations
5. **Annotation updates**: Added or modified Knative annotations (e.g., progress-deadline)
6. **Scaling policy changes**: Updated autoscaling behavior

**Note**: AIModel spec changes (like `runtimeArgs` or `resources`) DO trigger updates automatically. This runbook is for operator-level config changes that affect how InferenceServices are created.

---

## Prerequisites

- kubectl access to the target cluster
- Confirm which AIModels need InferenceService recreation:
  ```bash
  # List all AIModels in a namespace
  kubectl get aimodel -n development
  
  # View specific AIModel details
  kubectl get aimodel <model-name> -n development -o yaml
  ```

---

## Identify Affected InferenceServices

### Step 1: Find InferenceServices Managed by Specific AIModels

```bash
# List all InferenceServices with their owner references
kubectl get inferenceservice -n development -o json | \
  jq -r '.items[] | select(.metadata.ownerReferences[].kind=="AIModel") | 
  "\(.metadata.name) (AIModel: \(.metadata.ownerReferences[0].name))"'
```

### Step 2: Check InferenceService Configuration

Compare current InferenceService config against what the updated operator would create:

```bash
# View current InferenceService spec
kubectl get inferenceservice <isvc-name> -n development -o yaml

# Look for:
# - .metadata.annotations (progress-deadline, etc.)
# - .spec.predictor.containers[].image (runtime version)
# - .spec.predictor.containers[].args (runtime args)
# - .spec.predictor.containers[].resources (CPU/memory/GPU)
```

### Step 3: Identify Models That Need Recreation

If the InferenceService configuration does not match the updated operator defaults, it needs recreation.

---

## Recreation Procedures

### Option A: Delete AIModel (Recommended)

This is the safest approach as it uses the operator's built-in cleanup logic.

**Advantages**:
- Cleanest state management
- Operator handles finalizers and cleanup
- Guaranteed fresh deployment

**Disadvantages**:
- Brief downtime during recreation
- Model must be re-downloaded if using ephemeral storage

**Procedure**:

```bash
# 1. Backup the AIModel manifest
kubectl get aimodel <model-name> -n development -o yaml > /tmp/aimodel-<model-name>.yaml

# 2. Delete the AIModel (operator will clean up InferenceService)
kubectl delete aimodel <model-name> -n development

# 3. Wait for cleanup to complete
kubectl get inferenceservice -n development -w
# Wait until InferenceService is gone

# 4. Recreate from manifest
kubectl apply -f /tmp/aimodel-<model-name>.yaml

# 5. Monitor recreation
kubectl get aimodel <model-name> -n development -w
# Wait for Phase: Ready
```

**Expected timeline**:
- Deletion: 30-60 seconds
- Recreation: 2-15 minutes (depending on model size)

---

### Option B: Delete InferenceService Only

This approach keeps the AIModel and triggers operator reconciliation.

**Advantages**:
- AIModel stays in etcd
- Faster for troubleshooting/testing

**Disadvantages**:
- May leave orphaned resources if operator reconciliation fails
- Not as clean as full AIModel recreation

**Procedure**:

```bash
# 1. Delete the InferenceService directly
kubectl delete inferenceservice <isvc-name> -n development

# 2. Trigger operator reconciliation by annotating the AIModel
kubectl annotate aimodel <model-name> -n development \
  force-reconcile="$(date +%s)" --overwrite

# 3. Monitor reconciliation
kubectl get inferenceservice -n development -w
# Wait for InferenceService to be recreated

# 4. Check new InferenceService has updated config
kubectl get inferenceservice <isvc-name> -n development -o yaml
```

**Expected timeline**:
- InferenceService recreation: 2-15 minutes

---

### Option C: Annotation-Triggered Reconciliation (Experimental)

Force the operator to reconcile without deletion by changing AIModel annotations.

**Advantages**:
- No deletion required
- Can work if operator supports annotation-based updates

**Disadvantages**:
- May not work for all configuration changes
- Depends on operator implementation details

**Procedure**:

```bash
# 1. Add reconciliation annotation
kubectl annotate aimodel <model-name> -n development \
  aimodel.ai-aas.io/force-reconcile="$(date +%s)" --overwrite

# 2. Monitor for changes
kubectl get inferenceservice <isvc-name> -n development -o yaml

# 3. If no changes occur after 60 seconds, use Option A or B
```

**Note**: As of operator version 0.2.0, this may not trigger InferenceService updates. Use Option A or B for guaranteed recreation.

---

### Option D: Scale to Zero, Then Recreate

For production environments where you want to minimize disruption.

**Advantages**:
- Drains traffic gracefully before deletion
- Good for production with multiple replicas

**Disadvantages**:
- Takes longer (scale down -> delete -> recreate -> scale up)
- More complex procedure

**Procedure**:

```bash
# 1. Scale AIModel to zero replicas
kubectl patch aimodel <model-name> -n development --type='json' \
  -p='[{"op": "replace", "path": "/spec/minReplicas", "value": 0}]'

# 2. Wait for pods to terminate
kubectl get pods -n development -l serving.kserve.io/inferenceservice=<isvc-name> -w
# Wait until no pods remain

# 3. Delete InferenceService
kubectl delete inferenceservice <isvc-name> -n development

# 4. Restore original minReplicas (triggers recreation)
kubectl patch aimodel <model-name> -n development --type='json' \
  -p='[{"op": "replace", "path": "/spec/minReplicas", "value": 1}]'

# 5. Monitor recreation
kubectl get aimodel <model-name> -n development -w
# Wait for Phase: Ready
```

**Expected timeline**:
- Scale to zero: 1-2 minutes
- Recreation: 2-15 minutes

---

## Verification Steps

After recreation, verify the InferenceService picked up the new configuration:

### 1. Check InferenceService Status

```bash
# Verify InferenceService exists and is ready
kubectl get inferenceservice <isvc-name> -n development

# Expected output:
# NAME       URL                                              READY   PREV   LATEST   AGE
# model-xyz  http://model-xyz.development.svc.cluster.local   True    100           2m
```

### 2. Verify Configuration Updates

```bash
# Check annotations (e.g., progress-deadline)
kubectl get inferenceservice <isvc-name> -n development \
  -o jsonpath='{.metadata.annotations}' | jq

# Check runtime args
kubectl get inferenceservice <isvc-name> -n development \
  -o jsonpath='{.spec.predictor.containers[0].args}' | jq

# Check runtime image
kubectl get inferenceservice <isvc-name> -n development \
  -o jsonpath='{.spec.predictor.containers[0].image}'

# Check resource limits
kubectl get inferenceservice <isvc-name> -n development \
  -o jsonpath='{.spec.predictor.containers[0].resources}' | jq
```

### 3. Verify Pod Health

```bash
# Check pods are running
kubectl get pods -n development -l serving.kserve.io/inferenceservice=<isvc-name>

# Expected: 2/2 Running after model loads

# Check pod events for probe failures
kubectl get events -n development --field-selector involvedObject.name=<pod-name> \
  | grep -E "Liveness|Readiness|Startup"

# Expected: No probe failures
```

### 4. Test Inference Endpoint

```bash
# Port-forward to the InferenceService
kubectl port-forward -n development svc/<isvc-name>-predictor-default 8080:80

# Test inference (in another terminal)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "model-xyz",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 10
  }'

# Expected: HTTP 200 OK with completion response
```

### 5. Check Operator Logs

```bash
# View operator logs for reconciliation events
kubectl logs -n ai-model-system -l app=ai-model-operator --tail=100

# Look for:
# - "Reconciling AIModel" events
# - "Created InferenceService" or "Updated InferenceService"
# - No error messages
```

---

## Rollback Procedure

If recreation causes issues, revert to the previous state:

### Rollback Method 1: Restore from Backup

```bash
# 1. Delete the broken AIModel
kubectl delete aimodel <model-name> -n development

# 2. Restore from backup manifest
kubectl apply -f /tmp/aimodel-<model-name>.yaml

# 3. If operator code was updated, revert the operator deployment
kubectl rollout undo deployment ai-model-operator -n ai-model-system

# 4. Verify rollback
kubectl get aimodel <model-name> -n development -w
```

### Rollback Method 2: Manual InferenceService Creation

If AIModel is stuck, create InferenceService manually as a temporary fix:

```bash
# 1. Create InferenceService from known-good template
kubectl apply -f - <<EOF
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: <isvc-name>
  namespace: development
  annotations:
    serving.knative.dev/progress-deadline: "600s"
spec:
  predictor:
    model:
      modelFormat:
        name: vllm
      storageUri: s3://ai-models/<model-path>
    minReplicas: 1
    maxReplicas: 3
EOF

# 2. Monitor status
kubectl get inferenceservice <isvc-name> -n development -w
```

---

## Bulk Recreation

For updating multiple models at once after major operator upgrade:

```bash
# 1. List all AIModels
kubectl get aimodel -n development -o name > /tmp/aimodels.txt

# 2. Backup all manifests
while read -r aimodel; do
  kubectl get "$aimodel" -n development -o yaml > "/tmp/$(echo "$aimodel" | tr '/' '_').yaml"
done < /tmp/aimodels.txt

# 3. Delete all AIModels
cat /tmp/aimodels.txt | xargs kubectl delete -n development

# 4. Wait for cleanup
kubectl get inferenceservice -n development -w
# Wait until all InferenceServices are gone

# 5. Recreate all AIModels
for manifest in /tmp/aimodel.ai-aas.io_*.yaml; do
  kubectl apply -f "$manifest"
  sleep 5  # Stagger recreation to avoid resource contention
done

# 6. Monitor recreation
kubectl get aimodel -n development -w
```

**Note**: This will cause downtime for all models. Use during maintenance windows only.

---

## Risks and Considerations

### Downtime

- **Option A (Delete AIModel)**: 2-15 minutes downtime per model
- **Option D (Scale to Zero)**: 3-17 minutes downtime (scale + recreate)
- **Mitigation**: Schedule during maintenance windows or use blue/green deployment

### Model Re-download

- If using ephemeral storage, deleting AIModel may trigger model re-download from HuggingFace
- **Mitigation**: Ensure `storageUri` points to persistent S3 bucket with cached models

### Resource Contention

- Recreating multiple large models simultaneously can exhaust GPU nodes
- **Mitigation**: Stagger recreation with `sleep` between deletions (see Bulk Recreation)

### Autoscaling Disruption

- Deleting InferenceService removes autoscaling history and metrics
- **Mitigation**: Monitor for cold start latency spikes after recreation

### Production Impact

- Production models should use `minReplicas: 1` to avoid cold starts
- **Mitigation**: Verify `minReplicas` before recreation and test in staging first

---

## Troubleshooting

### Problem: InferenceService Not Recreated After Deletion

**Cause**: AIModel controller not running or failed reconciliation.

**Solution**:
```bash
# Check operator logs
kubectl logs -n ai-model-system -l app=ai-model-operator --tail=100

# Restart operator if needed
kubectl rollout restart deployment ai-model-operator -n ai-model-system

# Verify operator is running
kubectl get pods -n ai-model-system -l app=ai-model-operator
```

### Problem: New InferenceService Still Has Old Configuration

**Cause**: Operator cache not updated or change not in deployed operator version.

**Solution**:
```bash
# Check operator version
kubectl get deployment ai-model-operator -n ai-model-system \
  -o jsonpath='{.spec.template.spec.containers[0].image}'

# Force operator restart to clear cache
kubectl rollout restart deployment ai-model-operator -n ai-model-system
```

### Problem: AIModel Stuck in "Deploying" Phase

**Cause**: InferenceService creation failed or probe configuration invalid.

**Solution**:
```bash
# Check AIModel status
kubectl get aimodel <model-name> -n development -o yaml | grep -A 20 status

# Check InferenceService events
kubectl get events -n development --field-selector involvedObject.kind=InferenceService

# Check KServe controller logs
kubectl logs -n kserve -l control-plane=kserve-controller-manager --tail=100
```

### Problem: Pod Stuck in CrashLoopBackOff After Recreation

**Cause**: Invalid runtime args or probe config too aggressive.

**Solution**:
```bash
# Check pod logs
kubectl logs -n development <pod-name> -c kserve-container --tail=100

# Check probe configuration
kubectl get pod <pod-name> -n development -o yaml | grep -A 10 Probe

# Adjust probe config if needed (edit AIModel or operator defaults)
```

---

## Related Documentation

- [AI Model Operator Guide](../operators/ai-model-operator-guide.md)
- [Enable Model Readiness Probes](./enable-model-readiness-probes.md)
- [KServe Migration Deployment](./kserve-migration-deployment.md)
- [Infrastructure Rollback](./infrastructure-rollback.md)

---

## Checklist

- [ ] Identified AIModels/InferenceServices needing recreation
- [ ] Backed up AIModel manifests to `/tmp`
- [ ] Selected recreation method (A/B/C/D) based on environment
- [ ] Executed recreation procedure
- [ ] Verified InferenceService has new configuration
- [ ] Verified pods are healthy (2/2 Running)
- [ ] Tested inference endpoint
- [ ] Monitored for probe failures and errors
- [ ] Documented any issues or unexpected behavior
- [ ] Updated this runbook if new edge cases discovered
