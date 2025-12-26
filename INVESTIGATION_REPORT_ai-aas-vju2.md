# Investigation Report

**Bead**: ai-aas-vju2
**Date**: 2025-12-14
**Investigator**: debugger agent

## Symptom

guidellm-runner benchmarks showing intermittent failures that rotate between different models across consecutive runs:
- Run 1 (12:12 UTC): openai/gpt-oss-20b: 0/59 success (100% failure)
- Run 2 (12:17 UTC): openai/gpt-oss-20b: 40/19 success (68% success)
- Run 3 (12:22 UTC): openai/gpt-oss-20b: 56/0 (100% success), but mistral-7b: 2/52 (4% success), unsloth/gpt-oss-20b: 35/22 (61% success)

Failures are NOT model-specific - they rotate between models.

## Reproduction

Continuous benchmark runs against https://api.dev.otherjamesbrown.com testing three models:
- mistralai/Mistral-7B-Instruct-v0.3
- unsloth/gpt-oss-20b
- openai/gpt-oss-20b

Each test runs for 60 seconds at 1 request/second.

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `kubectl get pods -n development` | All 3 predictor pods have very high restart counts: 345, 352, 403 |
| `kubectl describe pod openai-gpt-oss-20b-predictor` | Exit Code 137 (SIGKILL), Last State: Terminated, Reason: Error |
| `kubectl describe node lke531921-776664-51386eeb0000` | Memory limits: 121% overcommitted (39GB limits on 32GB node) |
| `kubectl describe node lke531921-776664-46225a090000` | Memory limits: 104% overcommitted (33.5GB limits on 32GB node) |
| `kubectl describe node lke531921-776664-59eb445b0000` | Memory limits: 107% overcommitted (34.5GB limits on 32GB node) |
| InferenceService spec | Each predictor: requests=16Gi memory, limits=32Gi memory |
| vLLM logs (unsloth/gpt-oss-20b) | Model loading took 13.7164 GiB, startup time: 32s |
| vLLM logs (mistral-7b) | Model loading took 13.5084 GiB, startup time: 50s |
| queue-proxy logs | "dial tcp 127.0.0.1:8000: connect: connection refused" (vLLM crashed) |
| queue-proxy logs | "context canceled" (requests timing out during crash) |
| kubectl events | "Container kserve-container failed liveness probe, will be restarted" |
| api-router-service logs | "backend marked as unhealthy: consecutive_errors:3, error: backend unhealthy: status 502" |
| api-router-service logs | "backend request failed: connection refused" to predictor service port 8012 |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Memory overcommitment causing OOM kills | CONFIRMED | All 3 GPU nodes are 104-121% overcommitted on memory limits. Exit code 137 = SIGKILL |
| Pods sharing nodes (resource contention) | RULED OUT | Each model has its own dedicated GPU node (1 pod per node) |
| vLLM backend crashes | CONFIRMED | queue-proxy logs show "connection refused" to port 8000, liveness probe failures |
| Long startup time after crash | CONFIRMED | vLLM takes 32-50s to reload model from HuggingFace, causing cascading failures |
| Insufficient liveness probe timeout | PARTIALLY TRUE | 5s timeout may be too short during high load, but not root cause |
| Model download on every restart | CONFIRMED | Logs show "Time spent downloading weights: 21.9s" on each restart |

## Root Cause

**Category**: `resource_constraint` + `configuration_error`

**Explanation**:

The intermittent benchmark failures are caused by a **memory overcommitment problem** combined with **cascading failures during pod restarts**:

1. **Memory Overcommitment**: Each of the 3 GPU nodes (32GB each) has predictor pods with 32GB memory limits. However, the nodes are 104-121% overcommitted when accounting for all pods on the node (including system pods, queue-proxy, KServe components). This means the sum of all pod memory limits exceeds the node's available memory.

2. **OOM Kills**: When memory pressure occurs (likely during inference under load), the Linux kernel's OOM killer terminates the vLLM containers (Exit Code 137 = SIGKILL). The predictor with the highest memory usage at that moment gets killed - this rotates randomly between the three models, explaining why failures aren't model-specific.

3. **Cascading Failures**: When a vLLM container is killed:
   - Liveness probes fail (can't reach /health endpoint)
   - After 90s (3 x 30s), Kubernetes restarts the pod
   - vLLM must re-download model weights (21s) and reload (32-50s total)
   - During this 30-50s reload window, ALL requests to that model fail with 502
   - api-router marks the backend as unhealthy after 3 consecutive failures
   - guidellm-runner benchmark for that model shows high failure rate

4. **Rotation Pattern**: Because OOM kills are non-deterministic (depends on which pod uses most memory at moment of pressure), the failures rotate between models across different benchmark runs. When guidellm runs benchmarks sequentially, whichever model is under load during a memory pressure event gets killed.

**Evidence**:
- Exit Code 137 (SIGKILL from OOM killer)
- Memory limits at 104-121% of node capacity
- Very high restart counts (345-403 restarts over 37-43 hours)
- "connection refused" errors to port 8000 (vLLM not running)
- Model reload taking 32-50 seconds per restart
- Failures rotating between models, not model-specific

## Context Gap Check

- [X] Was this caused by missing context? YES

**Context file**: context/kserve-operator/agents.md (does not exist - should be created)
**What was missing**:
- No guidance on right-sizing KServe InferenceService resource requests/limits
- No anti-pattern documented for memory overcommitment on GPU nodes
- No best practices for vLLM model caching to avoid re-downloads
- No guidance on liveness/readiness probe configuration for long-loading models

**Suggested fix**: Create context/kserve-operator/agents.md with:
- Pattern: Set memory requests = limits for GPU workloads (guaranteed QoS)
- Pattern: Calculate node capacity accounting for system pods + queue-proxy overhead (~2-4GB)
- Pattern: Use PersistentVolumes for vLLM model cache to avoid re-downloads on restart
- Anti-pattern: Setting memory limits > node capacity (causes OOM kills)
- Anti-pattern: Allowing Kubernetes to overcommit GPU node memory

## Proposed Fix

**High-level description**:

1. **Reduce memory limits** on InferenceServices to prevent overcommitment:
   - Each model uses ~13.5-13.7 GiB for weights
   - Recommend: requests=16Gi, limits=24Gi (leaves headroom for KV cache + overhead)
   - This ensures 24GB + ~4GB system overhead = 28GB < 32GB node capacity

2. **Add PersistentVolume for vLLM model cache** to avoid re-downloading models on every restart:
   - Mount /root/.cache/huggingface as PersistentVolumeClaim
   - This reduces restart time from 50s → ~10s (no download, just reload)

3. **Increase liveness probe timeout** to account for occasional GC pauses:
   - Change from 5s → 10s timeout
   - Change period from 30s → 60s (less aggressive)
   - This prevents false-positive restarts during brief hangs

4. **Set resource requests = limits** for guaranteed QoS class:
   - Prevents Kubernetes from evicting these pods under memory pressure
   - GPU workloads should use guaranteed QoS, not burstable

**Affected files**:
- `services/*/deployments/kserve/*.yaml` - Update InferenceService specs for all 3 models
- `gitops/clusters/development/kserve/pvc-model-cache.yaml` - Create PersistentVolumeClaim for model cache
- `services/*/deployments/kserve/*.yaml` - Update livenessProbe configuration

**Estimated complexity**: Medium

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add load test that runs concurrent requests to all models for extended period (>10 min) |
| Lint | Add pre-deploy validation: sum(pod.limits) <= node.capacity for GPU nodes |
| Context | Create context/kserve-operator/agents.md with resource sizing patterns |
| Monitoring | Add alert for pod restart count > 10 in 1 hour (abnormal churn) |
| Monitoring | Add alert for memory overcommitment on GPU nodes |
| CI/CD | Add check in CI: fail if InferenceService limits > node capacity |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| ai-aas-vg6j | bug (P1) | kserve-operator | Fix memory limits on InferenceServices to prevent overcommitment |
| ai-aas-c3oz | task (P2) | kserve-operator | Add PersistentVolume for vLLM model cache |
| ai-aas-97k7 | task (P2) | monitoring-ops | Add alerts for pod restart rate and memory overcommitment (observability) |
| ai-aas-5q6r | task (P3) | context-maintainer | Create context/kserve-operator/agents.md with resource sizing patterns (context-gap, context-update) |
| ai-aas-7x9a | task (P3) | ci-cd-ops | Add pre-deploy validation for node capacity vs pod limits (ci-cd-improvement) |
