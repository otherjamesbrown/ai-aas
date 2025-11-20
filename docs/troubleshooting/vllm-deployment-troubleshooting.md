# vLLM Deployment Troubleshooting Guide

**Feature**: `010-vllm-deployment`
**Last Updated**: 2025-01-27

## Overview

This comprehensive troubleshooting guide provides step-by-step diagnosis and resolution procedures for common issues with vLLM model deployments.

## Quick Diagnostic Commands

```bash
# Essential commands for quick diagnosis
# Copy and run these first when investigating issues

# 1. Check pod status
kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment

# 2. Check deployment status
kubectl get deployment -n system | grep vllm

# 3. Check pod logs (latest pod)
kubectl logs -n system -l app.kubernetes.io/name=vllm-deployment --tail=100

# 4. Check events
kubectl get events -n system --sort-by='.lastTimestamp' | tail -20

# 5. Check registry status
admin-cli registry list --environment production

# 6. Check resource availability
kubectl describe nodes | grep -A 10 "Allocated resources"
```

## Table of Contents

1. [Pod Issues](#pod-issues)
2. [Model Loading Issues](#model-loading-issues)
3. [Performance Issues](#performance-issues)
4. [Network and Routing Issues](#network-and-routing-issues)
5. [Resource Issues](#resource-issues)
6. [Registry Issues](#registry-issues)
7. [Helm Deployment Issues](#helm-deployment-issues)
8. [GPU Issues](#gpu-issues)

---

## Pod Issues

### Issue: Pods Stuck in Pending State

**Symptoms:**
```bash
$ kubectl get pods -n system
NAME                           READY   STATUS    RESTARTS   AGE
llama-2-7b-production-abc123   0/1     Pending   0          10m
```

**Common Causes:**
1. Insufficient GPU resources
2. Node selector mismatch
3. Resource quota exceeded
4. Pod affinity/anti-affinity constraints
5. Volume mount failures

**Diagnosis:**

```bash
# Step 1: Check pod description for events
kubectl describe pod <pod-name> -n system | grep -A 20 Events

# Step 2: Check GPU resource availability
kubectl describe nodes | grep -A 5 "nvidia.com/gpu"

# Step 3: Check resource quotas
kubectl get resourcequota -n system

# Step 4: Check node labels
kubectl get nodes --show-labels | grep gpu
```

**Resolution:**

**For GPU shortage:**
```bash
# Option 1: Scale down non-critical GPU workloads
kubectl scale deployment <other-deployment> --replicas=0 -n <namespace>

# Option 2: Reduce replica count
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set replicaCount=1 \
  --reuse-values \
  -n system

# Option 3: Add GPU nodes to cluster (infrastructure team)
```

**For node selector mismatch:**
```bash
# Check node labels
kubectl get nodes --show-labels

# Update deployment node selector
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set nodeSelector."node-type"=gpu \
  --reuse-values \
  -n system
```

**For volume mount issues:**
```bash
# Check PVC status
kubectl get pvc -n system

# Check storage class
kubectl get storageclass

# Recreate PVC if needed
kubectl delete pvc <pvc-name> -n system
helm upgrade <release-name> infra/helm/charts/vllm-deployment --reuse-values -n system
```

---

### Issue: Pods in CrashLoopBackOff

**Symptoms:**
```bash
$ kubectl get pods -n system
NAME                           READY   STATUS             RESTARTS   AGE
llama-2-7b-production-abc123   0/1     CrashLoopBackOff   5          10m
```

**Common Causes:**
1. Out of memory (OOM)
2. Model loading failure
3. Configuration errors
4. Missing dependencies
5. GPU driver issues

**Diagnosis:**

```bash
# Step 1: Check current pod logs
kubectl logs <pod-name> -n system --tail=100

# Step 2: Check previous pod logs (before crash)
kubectl logs <pod-name> -n system --previous --tail=100

# Step 3: Check for OOM kills
kubectl describe pod <pod-name> -n system | grep -i "OOMKilled"

# Step 4: Check resource usage before crash
kubectl top pod <pod-name> -n system
```

**Resolution:**

**For OOM errors:**
```bash
# Increase memory limits
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set resources.limits.memory=64Gi \
  --set resources.requests.memory=48Gi \
  --reuse-values \
  -n system
```

**For model loading failures:**
```bash
# Check model path is accessible
kubectl exec -it <pod-name> -n system -- ls -la /models

# Verify HuggingFace token (if using private models)
kubectl get secret <secret-name> -n system -o yaml

# Test model access manually
kubectl run test-model --rm -i --restart=Never -n system \
  --image=vllm/vllm-openai:latest -- \
  python -m vllm.entrypoints.openai.api_server \
  --model <model-name> --dry-run
```

**For configuration errors:**
```bash
# Validate Helm values
helm get values <release-name> -n system

# Test configuration with dry-run
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --values values-production.yaml \
  --dry-run --debug \
  -n system
```

---

### Issue: Pods Restarting Frequently

**Symptoms:**
- High restart count (> 5)
- Intermittent service availability
- Health check failures in logs

**Common Causes:**
1. Liveness probe too aggressive
2. Memory leaks
3. GPU errors
4. Application bugs

**Diagnosis:**

```bash
# Check restart count
kubectl get pods -n system -o wide

# Check pod events for restart reasons
kubectl describe pod <pod-name> -n system | grep -A 10 "Last State"

# Monitor restart rate
watch -n 5 "kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment"

# Check liveness probe configuration
kubectl get deployment <deployment-name> -n system -o yaml | grep -A 10 livenessProbe
```

**Resolution:**

**For aggressive health checks:**
```bash
# Increase liveness probe timeout and failure threshold
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set livenessProbe.timeoutSeconds=10 \
  --set livenessProbe.failureThreshold=5 \
  --set livenessProbe.periodSeconds=60 \
  --reuse-values \
  -n system
```

**For memory leaks:**
```bash
# Monitor memory usage over time
kubectl top pod <pod-name> -n system

# Enable heap profiling (if supported)
# Add env var: VLLM_DEBUG_MEMORY=1

# Restart pods to clear leaked memory (temporary fix)
kubectl delete pod <pod-name> -n system
```

---

## Model Loading Issues

### Issue: Model Loading Timeout

**Symptoms:**
- Startup probe failing
- "Model not ready" errors in logs
- Pod restarts after 10-20 minutes

**Common Causes:**
1. Model too large for startup timeout
2. Slow network (downloading model)
3. Insufficient GPU memory
4. Model cache miss

**Diagnosis:**

```bash
# Check startup probe configuration
kubectl get deployment <deployment-name> -n system -o yaml | grep -A 10 startupProbe

# Monitor model loading progress
kubectl logs <pod-name> -n system --follow | grep -i "loading\|download"

# Check GPU memory during loading
kubectl exec -it <pod-name> -n system -- nvidia-smi
```

**Resolution:**

**For large models (70B+):**
```bash
# Increase startup probe failure threshold
# Each failure = 30 seconds, so 40 failures = 20 minutes
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set startupProbe.failureThreshold=40 \
  --set startupProbe.periodSeconds=30 \
  --reuse-values \
  -n system
```

**For slow downloads:**
```bash
# Use model caching (mount persistent volume)
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set persistence.enabled=true \
  --set persistence.size=200Gi \
  --reuse-values \
  -n system

# Or pre-download model to node
# SSH to node and pull model:
# docker pull vllm/vllm-openai:<model-tag>
```

**For GPU memory issues:**
```bash
# Check GPU memory size
kubectl exec -it <pod-name> -n system -- nvidia-smi --query-gpu=memory.total --format=csv

# Reduce max_model_len to lower memory usage
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set vllm.maxModelLen=2048 \
  --reuse-values \
  -n system
```

---

### Issue: Model Inference Errors

**Symptoms:**
- 500 errors from inference endpoint
- "CUDA out of memory" errors
- Timeout errors on inference

**Diagnosis:**

```bash
# Test inference endpoint
curl -X POST http://<endpoint>:8000/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{"model": "test", "prompt": "Hello", "max_tokens": 10}'

# Check inference logs
kubectl logs <pod-name> -n system | grep -i "error\|exception\|cuda"

# Monitor GPU usage during inference
kubectl exec -it <pod-name> -n system -- watch -n 1 nvidia-smi
```

**Resolution:**

**For CUDA OOM:**
```bash
# Reduce batch size
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set vllm.maxBatchSize=8 \
  --reuse-values \
  -n system

# Use tensor parallelism (multi-GPU)
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set vllm.tensorParallelSize=2 \
  --set resources.requests."nvidia\.com/gpu"=2 \
  --reuse-values \
  -n system
```

---

## Performance Issues

### Issue: High Latency

**Symptoms:**
- P95 latency > 3 seconds
- Slow responses
- Timeout errors

**Common Causes:**
1. GPU resource contention
2. High traffic volume
3. Large prompts or output lengths
4. CPU bottleneck

**Diagnosis:**

```bash
# Check current latency
kubectl logs <pod-name> -n system | grep -i "latency\|duration"

# Check GPU utilization
kubectl exec -it <pod-name> -n system -- nvidia-smi dmon -s u

# Check CPU usage
kubectl top pod <pod-name> -n system

# Monitor request rate
# (requires Prometheus)
# rate(vllm_requests_total[5m])
```

**Resolution:**

**For high GPU utilization:**
```bash
# Scale up replicas
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set replicaCount=3 \
  --reuse-values \
  -n system
```

**For CPU bottleneck:**
```bash
# Increase CPU allocation
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set resources.requests.cpu=8 \
  --set resources.limits.cpu=16 \
  --reuse-values \
  -n system
```

**For network latency:**
```bash
# Test network latency from API router
kubectl run test-network --rm -i --restart=Never -n system \
  --image=nicolaka/netshoot -- \
  ping <service-name>.system.svc.cluster.local

# Check service endpoints
kubectl get endpoints <service-name> -n system
```

---

### Issue: Low Throughput

**Symptoms:**
- Requests/second lower than expected
- Long queue times
- Backlog of requests

**Diagnosis:**

```bash
# Check request rate
# (requires metrics endpoint)
curl http://<endpoint>:8000/metrics | grep vllm_requests_total

# Check for request queuing
kubectl logs <pod-name> -n system | grep -i "queue\|waiting"

# Check concurrent requests
kubectl exec -it <pod-name> -n system -- ps aux | grep vllm
```

**Resolution:**

```bash
# Increase concurrent requests (vLLM worker threads)
# Note: This is typically auto-configured, but can be tuned

# Scale horizontally
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set replicaCount=5 \
  --reuse-values \
  -n system

# Optimize batch size for throughput
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set vllm.maxBatchSize=32 \
  --reuse-values \
  -n system
```

---

## Network and Routing Issues

### Issue: Service Not Accessible

**Symptoms:**
- Connection refused errors
- DNS resolution failures
- 404 errors from API router

**Diagnosis:**

```bash
# Check service exists
kubectl get service <service-name> -n system

# Check service endpoints
kubectl get endpoints <service-name> -n system

# Test DNS resolution
kubectl run test-dns --rm -i --restart=Never -n system \
  --image=busybox:1.28 -- \
  nslookup <service-name>.system.svc.cluster.local

# Test connectivity from API router namespace
kubectl run test-connect --rm -i --restart=Never -n system \
  --image=curlimages/curl:latest -- \
  curl -f http://<service-name>.system.svc.cluster.local:8000/health
```

**Resolution:**

**For missing endpoints:**
```bash
# Check pod readiness
kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment

# Verify service selector matches pod labels
kubectl get service <service-name> -n system -o yaml | grep selector -A 5
kubectl get pods <pod-name> -n system -o yaml | grep labels -A 10
```

**For DNS issues:**
```bash
# Check CoreDNS pods
kubectl get pods -n kube-system -l k8s-app=kube-dns

# Restart CoreDNS if needed
kubectl rollout restart deployment coredns -n kube-system
```

**For NetworkPolicy blocking:**
```bash
# Check NetworkPolicies
kubectl get networkpolicy -n system

# Temporarily disable for testing
kubectl delete networkpolicy <policy-name> -n system

# Re-create after testing
kubectl apply -f <networkpolicy-file>
```

---

### Issue: Model Not Found in Registry

**Symptoms:**
- "Model not found" errors from API router
- 404 responses for model requests
- Registry shows model as disabled

**Diagnosis:**

```bash
# Check registry status
admin-cli registry list --environment production

# Check database directly
kubectl exec -it <postgres-pod> -n system -- \
  psql -U postgres -d ai_aas_operational \
  -c "SELECT * FROM model_registry_entries WHERE deployment_environment='production';"

# Check Redis cache
kubectl exec -it <redis-pod> -n system -- \
  redis-cli KEYS "model:*"
```

**Resolution:**

```bash
# Re-register model
admin-cli registry register \
  --model-name <model-name> \
  --endpoint <endpoint> \
  --environment production \
  --namespace system

# Enable if disabled
admin-cli registry enable \
  --model-name <model-name> \
  --environment production

# Clear Redis cache
kubectl exec -it <redis-pod> -n system -- redis-cli FLUSHDB
```

---

## Resource Issues

### Issue: GPU Not Detected

**Symptoms:**
- "No GPUs found" errors
- CUDA initialization failures
- Pod scheduled on non-GPU node

**Diagnosis:**

```bash
# Check GPU availability on nodes
kubectl describe nodes | grep -A 5 "nvidia.com/gpu"

# Check NVIDIA device plugin
kubectl get pods -n kube-system -l name=nvidia-device-plugin-ds

# Check GPU from inside pod
kubectl exec -it <pod-name> -n system -- nvidia-smi
```

**Resolution:**

**For missing device plugin:**
```bash
# Install NVIDIA device plugin
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml

# Verify installation
kubectl get pods -n kube-system -l name=nvidia-device-plugin-ds
```

**For node selector issues:**
```bash
# Check node labels
kubectl get nodes --show-labels | grep gpu

# Label nodes if needed
kubectl label nodes <node-name> node-type=gpu

# Verify deployment node selector
helm get values <release-name> -n system | grep nodeSelector
```

---

### Issue: Out of Memory (OOM)

**Symptoms:**
- Pod terminated with OOMKilled
- "Cannot allocate memory" errors
- Slow performance before crash

**Diagnosis:**

```bash
# Check pod termination reason
kubectl describe pod <pod-name> -n system | grep -A 5 "Last State"

# Check memory limits
kubectl get pod <pod-name> -n system -o yaml | grep -A 5 "limits:"

# Monitor memory usage
kubectl top pod <pod-name> -n system
```

**Resolution:**

```bash
# Increase memory limits
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set resources.limits.memory=96Gi \
  --set resources.requests.memory=64Gi \
  --reuse-values \
  -n system

# Use smaller model variant
# e.g., llama-2-7b instead of llama-2-13b

# Reduce max_model_len
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set vllm.maxModelLen=2048 \
  --reuse-values \
  -n system
```

---

## Registry Issues

### Issue: Registry Database Connection Failures

**Symptoms:**
- "Connection refused" errors in API router logs
- Registry commands fail
- Stale data in responses

**Diagnosis:**

```bash
# Check PostgreSQL pod
kubectl get pods -n system -l app=postgresql

# Test database connection
kubectl run test-db --rm -i --restart=Never -n system \
  --image=postgres:15 -- \
  psql -h <postgres-host> -U postgres -d ai_aas_operational -c "SELECT 1;"

# Check database URL configuration
kubectl get configmap <api-router-config> -n system -o yaml | grep DATABASE_URL
```

**Resolution:**

```bash
# Restart PostgreSQL pod
kubectl delete pod <postgres-pod> -n system

# Verify connection string
# Should be: postgres://user:pass@host:5432/dbname

# Update configuration if needed
helm upgrade api-router-service \
  --set database.url="postgres://..." \
  --reuse-values \
  -n system
```

---

### Issue: Redis Cache Not Working

**Symptoms:**
- High database load
- Slow registry lookups
- Cache miss rate 100%

**Diagnosis:**

```bash
# Check Redis pod
kubectl get pods -n system -l app=redis

# Test Redis connection
kubectl exec -it <redis-pod> -n system -- redis-cli PING

# Check cache hit rate
kubectl exec -it <redis-pod> -n system -- redis-cli INFO stats | grep hits

# Monitor cache keys
kubectl exec -it <redis-pod> -n system -- redis-cli KEYS "*"
```

**Resolution:**

```bash
# Restart Redis pod
kubectl delete pod <redis-pod> -n system

# Verify Redis URL in API router config
kubectl get deployment api-router-service -n system -o yaml | grep REDIS_URL

# Flush cache and rebuild
kubectl exec -it <redis-pod> -n system -- redis-cli FLUSHDB
```

---

## Helm Deployment Issues

### Issue: Helm Install/Upgrade Fails

**Symptoms:**
- Helm command returns error
- Resources not created
- Partial deployment

**Diagnosis:**

```bash
# Check Helm release status
helm status <release-name> -n system

# View release history
helm history <release-name> -n system

# Get detailed error
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --values values-production.yaml \
  --debug \
  -n system
```

**Resolution:**

**For validation errors:**
```bash
# Validate chart
helm lint infra/helm/charts/vllm-deployment

# Test template rendering
helm template test infra/helm/charts/vllm-deployment \
  --values values-production.yaml \
  --debug
```

**For failed release:**
```bash
# Rollback to previous version
helm rollback <release-name> -n system

# Or uninstall and reinstall
helm uninstall <release-name> -n system
helm install <release-name> infra/helm/charts/vllm-deployment \
  --values values-production.yaml \
  -n system
```

---

## GPU Issues

### Issue: GPU Drivers Not Found

**Symptoms:**
- "NVIDIA driver not found" errors
- "libcuda.so: cannot open shared object file"
- GPU pods stuck in Init state

**Diagnosis:**

```bash
# Check GPU driver on node
kubectl debug node/<node-name> -it --image=ubuntu -- \
  chroot /host nvidia-smi

# Check driver version
kubectl debug node/<node-name> -it --image=ubuntu -- \
  chroot /host cat /proc/driver/nvidia/version
```

**Resolution:**

```bash
# Ensure NVIDIA drivers installed on nodes
# This is typically an infrastructure/node setup task

# For NVIDIA GPU Operator (automated driver installation)
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/gpu-operator/v23.9.0/deployments/gpu-operator/values.yaml

# Verify driver installation
kubectl get pods -n gpu-operator-resources
```

---

## Advanced Debugging

### Enable Debug Logging

```bash
# Enable vLLM debug logging
helm upgrade <release-name> infra/helm/charts/vllm-deployment \
  --set env[0].name=VLLM_LOG_LEVEL \
  --set env[0].value=DEBUG \
  --reuse-values \
  -n system

# View debug logs
kubectl logs <pod-name> -n system --tail=500 | grep DEBUG
```

### Interactive Debugging

```bash
# Get shell in running pod
kubectl exec -it <pod-name> -n system -- /bin/bash

# Run manual inference test
python3 -c "
from vllm import LLM
llm = LLM(model='<model-name>')
output = llm.generate(['Hello'])
print(output)
"

# Check GPU from inside pod
nvidia-smi
nvidia-smi dmon -s u

# Check network from inside pod
curl http://localhost:8000/health
```

### Collect Diagnostic Bundle

```bash
#!/bin/bash
# collect-diagnostics.sh
RELEASE_NAME=$1
NAMESPACE=${2:-system}
OUTPUT_DIR="diagnostics-$(date +%Y%m%d-%H%M%S)"

mkdir -p $OUTPUT_DIR

# Collect pod information
kubectl get pods -n $NAMESPACE -l app.kubernetes.io/instance=$RELEASE_NAME -o yaml > $OUTPUT_DIR/pods.yaml

# Collect logs
for pod in $(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/instance=$RELEASE_NAME -o name); do
  kubectl logs -n $NAMESPACE $pod > $OUTPUT_DIR/$(basename $pod).log
  kubectl logs -n $NAMESPACE $pod --previous > $OUTPUT_DIR/$(basename $pod)-previous.log 2>/dev/null || true
done

# Collect events
kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp' > $OUTPUT_DIR/events.txt

# Collect Helm release
helm get all $RELEASE_NAME -n $NAMESPACE > $OUTPUT_DIR/helm-release.yaml

# Collect node information
kubectl describe nodes > $OUTPUT_DIR/nodes.txt

# Compress
tar -czf $OUTPUT_DIR.tar.gz $OUTPUT_DIR
echo "Diagnostics collected: $OUTPUT_DIR.tar.gz"
```

## Getting Help

### Escalation Path

1. **Check this troubleshooting guide**
2. **Check runbooks**: [docs/runbooks/](../runbooks/)
3. **Check Helm chart README**: [infra/helm/charts/vllm-deployment/README.md](../../infra/helm/charts/vllm-deployment/README.md)
4. **Review logs and metrics**
5. **Collect diagnostic bundle**
6. **Contact on-call engineer** (for production issues)
7. **File GitHub issue** with diagnostic bundle attached

### Useful Resources

- [Rollback Workflow](../rollback-workflow.md)
- [Rollout Workflow](../rollout-workflow.md)
- [Partial Failure Remediation](../runbooks/partial-failure-remediation.md)
- [Performance SLO Tracking](../monitoring/performance-slo-tracking.md)
- [vLLM Documentation](https://vllm.readthedocs.io/)
- [Kubernetes Troubleshooting](https://kubernetes.io/docs/tasks/debug/)
