# Manual Testing Checklist: Failure Mode Scenarios

This checklist provides step-by-step instructions for manually testing all failure mode scenarios before releases. Each test verifies that the AI-AAS platform correctly detects, reports, and surfaces deployment failures to operators.

---

## Prerequisites

Before running these tests, ensure the following:

### Cluster Access
- [ ] Kubernetes cluster access configured (`kubectl` working)
- [ ] Correct kubeconfig set: `export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml`
- [ ] Can access the `development` namespace: `kubectl get pods -n development`

### CLI Installed
- [ ] `ai-aas-cli` installed and in PATH: `which ai-aas-cli`
- [ ] CLI configured with valid credentials: `ai-aas-cli status`
- [ ] Rebuild if needed: `./scripts/build-clis.sh --install && hash -r`

### Operator Deployed
- [ ] AIModel operator is running: `kubectl get pods -n ai-model-system | grep controller`
- [ ] AIModel CRD is installed: `kubectl get crd aimodels.aimodel.ai-aas.io`
- [ ] ModelRecipe CRD is installed: `kubectl get crd modelrecipes.aimodel.ai-aas.io`

### Test Environment
- [ ] No conflicting test models deployed: `kubectl get aimodel -n development | grep test-`
- [ ] Sufficient quota available in namespace
- [ ] Cluster autoscaler disabled or behavior understood (for scheduling tests)

---

## Test Matrix Summary

| # | Scenario | Config File | Expected Phase | Expected Message Pattern | Category | Est. Time |
|---|----------|-------------|----------------|-------------------------|----------|-----------|
| 1 | Insufficient CPU | test-insufficient-cpu.yaml | Scheduling | "Insufficient cpu" | Scheduling | 2-3 min |
| 2 | Insufficient Memory | test-insufficient-memory.yaml | Scheduling | "Insufficient memory" | Scheduling | 2-3 min |
| 3 | Missing GPU Toleration | test-missing-toleration.yaml | Scheduling | "untolerated taint" | Scheduling | 2-3 min |
| 4 | No GPU Available | test-no-gpu-available.yaml | Scheduling | "Insufficient nvidia.com/gpu" | Scheduling | 2-3 min |
| 5 | Needs Cluster Scale-up | test-needs-scaleup.yaml | Scheduling | "waiting for" or "autoscaler" | Scheduling | 5-10 min |
| 6 | At Maximum Scale | test-at-max-scale.yaml | Scheduling | "resource exhaustion" | Scheduling | 5-10 min |
| 7 | Invalid S3 Path | test-invalid-s3-path.yaml | Downloading/Failed | "not found", "404", "NoSuchKey" | Download | 3-5 min |
| 8 | Invalid HuggingFace Model | test-invalid-hf-model.yaml | Downloading/Failed | "not found", "404" | Download | 3-5 min |
| 9 | OOM Kill | test-oom-kill.yaml | Initializing/Failed | "OOMKilled", "out of memory" | Initialization | 5-10 min |
| 10 | Crash Loop | test-crash-loop.yaml | Initializing/Failed | "CrashLoopBackOff", "error" | Initialization | 3-5 min |
| 11 | Invalid Recipe Reference | test-invalid-recipe.yaml | Validating/Failed | "recipe not found" | Validation | 1-2 min |

**Total Estimated Time: 35-60 minutes**

---

## Detailed Test Instructions

### Test 1: Insufficient CPU

**Category:** Scheduling
**Purpose:** Verify detection of scheduling failures due to excessive CPU requests (64 CPUs)

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-insufficient-cpu.yaml
   ```

2. **Enable the model (triggers deployment):**
   ```bash
   kubectl patch aimodel test-insufficient-cpu -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status via CLI:**
   ```bash
   ai-aas-cli model deploy status test-insufficient-cpu -e development
   ```

4. **Alternative: Watch via kubectl:**
   ```bash
   kubectl get aimodel test-insufficient-cpu -n development -w
   ```

5. **Check events for scheduling details:**
   ```bash
   ai-aas-cli model troubleshoot events test-insufficient-cpu -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Scheduling` (not stuck at `Pending` without context)
- [ ] Message contains "Insufficient cpu" or similar scheduling failure text
- [ ] Events show pod scheduling failure with resource details
- [ ] CLI output is human-readable and actionable

**Cleanup:**
```bash
kubectl delete aimodel test-insufficient-cpu -n development
```

---

### Test 2: Insufficient Memory

**Category:** Scheduling
**Purpose:** Verify detection of scheduling failures due to excessive memory requests (512Gi)

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-insufficient-memory.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-insufficient-memory -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-insufficient-memory -e development
   ```

4. **Check events:**
   ```bash
   ai-aas-cli model troubleshoot events test-insufficient-memory -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Scheduling`
- [ ] Message contains "Insufficient memory" or similar
- [ ] Events detail the memory requirement that cannot be satisfied

**Cleanup:**
```bash
kubectl delete aimodel test-insufficient-memory -n development
```

---

### Test 3: Missing GPU Toleration

**Category:** Scheduling
**Purpose:** Verify detection when pod lacks toleration for GPU node taints (`gpu-workload=true:NoSchedule`)

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-missing-toleration.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-missing-toleration -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-missing-toleration -e development
   ```

4. **Check events:**
   ```bash
   ai-aas-cli model troubleshoot events test-missing-toleration -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Scheduling`
- [ ] Message contains "untolerated taint" or mentions "gpu-workload"
- [ ] Message is actionable (tells operator which toleration is needed)

**Cleanup:**
```bash
kubectl delete aimodel test-missing-toleration -n development
```

---

### Test 4: No GPU Available

**Category:** Scheduling
**Purpose:** Verify detection when requesting more GPUs than any node can provide (4 GPUs)

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-no-gpu-available.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-no-gpu-available -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-no-gpu-available -e development
   ```

4. **Check events:**
   ```bash
   ai-aas-cli model troubleshoot events test-no-gpu-available -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Scheduling`
- [ ] Message contains "Insufficient nvidia.com/gpu" or similar GPU shortage message
- [ ] Message indicates how many GPUs were requested vs available

**Cleanup:**
```bash
kubectl delete aimodel test-no-gpu-available -n development
```

---

### Test 5: Needs Cluster Scale-up

**Category:** Scheduling
**Purpose:** Verify detection when cluster autoscaler needs to provision new nodes

**Note:** This test may take longer as the autoscaler needs time to evaluate. Results vary based on autoscaler configuration.

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-needs-scaleup.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-needs-scaleup -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status (allow 5+ minutes):**
   ```bash
   ai-aas-cli model deploy status test-needs-scaleup -e development
   ```

4. **Check events for autoscaler activity:**
   ```bash
   ai-aas-cli model troubleshoot events test-needs-scaleup -e development
   ```

5. **Check cluster autoscaler status (if available):**
   ```bash
   kubectl get events -n kube-system --field-selector reason=ScaledUpGroup
   ```

**Validation Criteria:**
- [ ] Phase shows `Scheduling`
- [ ] Message indicates waiting for node provisioning or autoscaler activity
- [ ] If autoscaler is disabled, message indicates no available nodes

**Cleanup:**
```bash
kubectl delete aimodel test-needs-scaleup -n development
```

---

### Test 6: At Maximum Scale

**Category:** Scheduling
**Purpose:** Verify detection when node pool is at maximum capacity and cannot scale further

**Note:** This test simulates exhausted node pools. Behavior depends on cluster configuration.

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-at-max-scale.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-at-max-scale -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status (allow 5+ minutes):**
   ```bash
   ai-aas-cli model deploy status test-at-max-scale -e development
   ```

4. **Check events:**
   ```bash
   ai-aas-cli model troubleshoot events test-at-max-scale -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Scheduling`
- [ ] Message indicates resource exhaustion or inability to schedule
- [ ] Differentiates from "waiting for scale-up" scenarios

**Cleanup:**
```bash
kubectl delete aimodel test-at-max-scale -n development
```

---

### Test 7: Invalid S3 Path

**Category:** Download
**Purpose:** Verify detection of model download failures from non-existent S3 paths

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-invalid-s3-path.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-invalid-s3-path -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-invalid-s3-path -e development
   ```

4. **Check logs for download errors:**
   ```bash
   ai-aas-cli model troubleshoot logs test-invalid-s3-path -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Downloading` or `Failed`
- [ ] Message contains "not found", "404", "NoSuchKey", or S3 error
- [ ] Message identifies which S3 path failed
- [ ] Init container logs show clear error

**Cleanup:**
```bash
kubectl delete aimodel test-invalid-s3-path -n development
```

---

### Test 8: Invalid HuggingFace Model

**Category:** Download
**Purpose:** Verify detection of download failures from non-existent HuggingFace model IDs

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-invalid-hf-model.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-invalid-hf-model -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-invalid-hf-model -e development
   ```

4. **Check logs:**
   ```bash
   ai-aas-cli model troubleshoot logs test-invalid-hf-model -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Downloading` or `Failed`
- [ ] Message contains "not found", "404", or HuggingFace error
- [ ] Message identifies which model ID was invalid

**Cleanup:**
```bash
kubectl delete aimodel test-invalid-hf-model -n development
```

---

### Test 9: OOM Kill

**Category:** Initialization
**Purpose:** Verify detection of out-of-memory kills during model loading (70B model with 8Gi limit)

**Note:** This test requires a GPU node and may take longer as it attempts to load a large model.

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-oom-kill.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-oom-kill -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status (may take 5-10 minutes to trigger OOM):**
   ```bash
   ai-aas-cli model deploy status test-oom-kill -e development
   ```

4. **Check pod status for OOMKilled:**
   ```bash
   kubectl get pods -n development -l aimodel=test-oom-kill -o wide
   kubectl describe pod -n development -l aimodel=test-oom-kill | grep -A5 "Last State"
   ```

5. **Check events:**
   ```bash
   ai-aas-cli model troubleshoot events test-oom-kill -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Initializing` or `Failed`
- [ ] Message contains "OOMKilled", "out of memory", or memory-related failure
- [ ] Pod status shows `OOMKilled` in last terminated state
- [ ] Message is actionable (suggests increasing memory limit)

**Cleanup:**
```bash
kubectl delete aimodel test-oom-kill -n development
```

---

### Test 10: Crash Loop

**Category:** Initialization
**Purpose:** Verify detection of CrashLoopBackOff due to invalid runtime arguments

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-crash-loop.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-crash-loop -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-crash-loop -e development
   ```

4. **Check pod status:**
   ```bash
   kubectl get pods -n development -l aimodel=test-crash-loop
   ```

5. **Check logs for error details:**
   ```bash
   ai-aas-cli model troubleshoot logs test-crash-loop -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Initializing` or `Failed`
- [ ] Message contains "CrashLoopBackOff", "error", or invalid argument details
- [ ] Pod shows restart count > 0
- [ ] Logs identify which argument caused the crash

**Cleanup:**
```bash
kubectl delete aimodel test-crash-loop -n development
```

---

### Test 11: Invalid Recipe Reference

**Category:** Validation
**Purpose:** Verify detection when AIModel references a non-existent ModelRecipe

**Steps:**

1. **Apply the test configuration:**
   ```bash
   kubectl apply -f tests/failure-modes/configs/test-invalid-recipe.yaml
   ```

2. **Enable the model:**
   ```bash
   kubectl patch aimodel test-invalid-recipe -n development \
     --type=merge -p '{"spec":{"enabled":true}}'
   ```

3. **Watch the status:**
   ```bash
   ai-aas-cli model deploy status test-invalid-recipe -e development
   ```

4. **Check events:**
   ```bash
   ai-aas-cli model troubleshoot events test-invalid-recipe -e development
   ```

**Validation Criteria:**
- [ ] Phase shows `Validating` or `Failed`
- [ ] Message contains "recipe not found" or references the missing recipe name
- [ ] Failure occurs quickly (validation phase, not scheduling)
- [ ] Message identifies which recipe was not found

**Cleanup:**
```bash
kubectl delete aimodel test-invalid-recipe -n development
```

---

## Cleanup All Tests

Run this to remove all test models at once:

```bash
kubectl delete aimodel -n development -l test-type=failure-mode
```

Verify cleanup:
```bash
kubectl get aimodel -n development | grep test-
```

---

## Troubleshooting Test Issues

### Test model stuck in unexpected state

```bash
# Get full description
ai-aas-cli model troubleshoot describe <model-name> -e development

# Get all events
kubectl get events -n development --field-selector involvedObject.name=<model-name>

# Get operator logs
kubectl logs -n ai-model-system -l app=aimodel-controller --tail=100
```

### Cannot apply test config

```bash
# Validate YAML
kubectl apply -f tests/failure-modes/configs/<config>.yaml --dry-run=client

# Check CRD is installed
kubectl get crd aimodels.aimodel.ai-aas.io
```

### CLI not showing expected output

```bash
# Rebuild CLI
./scripts/build-clis.sh --install && hash -r

# Use kubectl directly
kubectl get aimodel <model-name> -n development -o yaml
```

---

## Sign-Off

### Test Execution Record

| Field | Value |
|-------|-------|
| **Tester Name** | |
| **Test Date** | |
| **Release Version** | |
| **Cluster** | development / staging / production |
| **CLI Version** | `ai-aas-cli --version` |
| **Operator Version** | |

### Test Results Summary

| Test | Pass | Fail | Skip | Notes |
|------|------|------|------|-------|
| 1. Insufficient CPU | [ ] | [ ] | [ ] | |
| 2. Insufficient Memory | [ ] | [ ] | [ ] | |
| 3. Missing Toleration | [ ] | [ ] | [ ] | |
| 4. No GPU Available | [ ] | [ ] | [ ] | |
| 5. Needs Scale-up | [ ] | [ ] | [ ] | |
| 6. At Max Scale | [ ] | [ ] | [ ] | |
| 7. Invalid S3 Path | [ ] | [ ] | [ ] | |
| 8. Invalid HF Model | [ ] | [ ] | [ ] | |
| 9. OOM Kill | [ ] | [ ] | [ ] | |
| 10. Crash Loop | [ ] | [ ] | [ ] | |
| 11. Invalid Recipe | [ ] | [ ] | [ ] | |

### Sign-Off

| Role | Name | Signature | Date |
|------|------|-----------|------|
| QA Tester | | | |
| Release Manager | | | |

### Issues Found

Document any issues discovered during testing:

| Issue # | Test | Description | Severity | Bead ID |
|---------|------|-------------|----------|---------|
| | | | | |

---

## Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | | | Initial checklist creation |
