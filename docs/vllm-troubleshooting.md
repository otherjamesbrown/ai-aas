# vLLM Deployment Troubleshooting Guide

This guide provides solutions for common issues encountered when deploying and operating vLLM models.

## Quick Diagnosis

```bash
# Check deployment status
kubectl get deployment -n system <model-name>-vllm-deployment

# Check pod status and events
kubectl describe pod -n system <pod-name>

# View pod logs
kubectl logs -n system <pod-name> --tail=100 --follow

# Check GPU availability
kubectl get nodes -l node-type=gpu
kubectl describe node <gpu-node-name> | grep -A 10 "Allocated resources"
```

## Common Issues

### 1. Pod Stuck in Pending State

**Symptoms:**
- Pod remains in `Pending` status for >5 minutes
- Event: `0/N nodes are available: N Insufficient nvidia.com/gpu`

**Causes & Solutions:**

**No GPU nodes available:**
```bash
# Check GPU node pool
kubectl get nodes -l node-type=gpu

# If no nodes, scale up GPU node pool
linode-cli lke pools-list <cluster-id>
```

**GPU already allocated:**
```bash
# Check GPU resource allocation
kubectl describe node <gpu-node> | grep nvidia.com/gpu

# List pods using GPUs
kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[].resources.limits."nvidia.com/gpu" != null) | {name:.metadata.name, namespace:.metadata.namespace, gpu:.spec.containers[].resources.limits."nvidia.com/gpu"}'

# Scale down or delete unused GPU pods if needed
```

**Taints preventing scheduling:**
```bash
# Check node taints
kubectl describe node <gpu-node> | grep Taints

# Ensure deployment has matching tolerations
helm get values <release-name> | grep -A 5 tolerations
```

### 2. Pod Crash Looping (CrashLoopBackOff)

**Symptoms:**
- Pod repeatedly restarts
- Status shows `CrashLoopBackOff` or `Error`

**Common Causes:**

**GPU Out of Memory (OOM):**
```bash
# Check logs for OOM errors
kubectl logs -n system <pod-name> | grep -i "out of memory\|oom\|cuda"

# Solutions:
# 1. Reduce max_num_seqs in values.yaml
# 2. Use quantization (--quantization awq or gptq)
# 3. Reduce max_model_len
# 4. Use larger GPU (upgrade node pool)
```

**Model download failure:**
```bash
# Check logs for download errors
kubectl logs -n system <pod-name> | grep -i "download\|huggingface\|error"

# Solutions:
# 1. Verify model path is correct
# 2. Check HuggingFace authentication (if private model)
# 3. Check network connectivity from cluster
```

**Incorrect vLLM arguments:**
```bash
# Review vLLM arguments
kubectl get deployment <name> -o yaml | grep -A 20 args

# Common issues:
# - Incompatible quantization method for model
# - Invalid tensor_parallel_size (must match GPU count)
# - Incorrect dtype for model architecture
```

### 3. Health Check Failures

**Symptoms:**
- Pod shows `0/1` ready
- Liveness/readiness probes failing

**Diagnosis:**
```bash
# Check probe failures
kubectl describe pod <pod-name> | grep -A 10 "Liveness\|Readiness"

# Test health endpoint manually (port-forward)
kubectl port-forward -n system <pod-name> 8000:8000
curl http://localhost:8000/health
curl http://localhost:8000/v1/models
```

**Solutions:**

**Model still loading:**
- Increase `startupProbe.initialDelaySeconds` for large models (70B+)
- Default: 300s for small models, 600s for medium, 900s for large

**Port not accessible:**
```bash
# Verify container port
kubectl get pod <pod-name> -o yaml | grep containerPort

# Check service
kubectl get svc <service-name>
```

### 4. High Latency

**Symptoms:**
- P95 latency > 10 seconds
- Alert: `VLLMHighRequestLatency`

**Diagnosis:**
```bash
# Check GPU utilization
kubectl exec -n system <pod-name> -- nvidia-smi

# Check request queue
kubectl logs -n system <pod-name> | grep "request queue\|waiting"

# View Grafana dashboard metrics
```

**Solutions:**

**GPU underutilized (<50%):**
- Increase `max_num_seqs` to allow more concurrent requests
- Increase batch size

**GPU overutilized (>95%):**
- Reduce `max_num_seqs`
- Add more GPU replicas: `helm upgrade --set replicaCount=2`

**Large prompts:**
- Set `max_model_len` appropriately for your use case
- Consider prompt caching if using repeated system prompts

### 5. High Error Rate

**Symptoms:**
- Alert: `VLLMHighErrorRate`
- 5xx errors in logs

**Common Errors:**

**CUDA Out of Memory:**
```bash
kubectl logs <pod-name> | grep "CUDA out of memory"

# Solutions:
# 1. Reduce max_num_seqs
# 2. Reduce max_model_len
# 3. Enable quantization
# 4. Upgrade to larger GPU
```

**Invalid request format:**
```bash
kubectl logs <pod-name> | grep "400\|invalid"

# Check request format matches OpenAI API spec
# Verify model name in requests matches deployed model
```

### 6. No Requests Received

**Symptoms:**
- Alert: `VLLMNoRequests`
- Zero traffic despite deployment being healthy

**Diagnosis:**
```bash
# Check model registration in database
psql $DATABASE_URL -c "SELECT * FROM model_registry_entries WHERE model_name='<model-name>';"

# Check API Router configuration
kubectl logs -n development deployment/api-router-service | grep <model-name>

# Verify service is accessible
kubectl get svc -n system <service-name>
```

**Solutions:**

**Model not registered:**
```bash
# Run registration script
./scripts/register-model.sh <model-name> <environment> system
```

**Routing misconfiguration:**
- Verify `backend_endpoint` in model registry matches service DNS
- Format: `http://<service-name>.<namespace>.svc.cluster.local:8000`

**NetworkPolicy blocking traffic:**
```bash
# Check NetworkPolicies
kubectl get networkpolicy -n system

# Verify ingress rules allow traffic from API Router namespace
```

### 7. Pod Eviction

**Symptoms:**
- Pod status: `Evicted`
- Event: `The node was low on resource`

**Solutions:**

**Memory pressure:**
```bash
# Check node memory
kubectl top nodes

# Reduce memory request/limit in values.yaml
# Or upgrade node pool to larger instances
```

**Disk pressure:**
```bash
# Check node disk usage
kubectl describe node <node-name> | grep "DiskPressure"

# Clean up unused images and containers
kubectl delete pod <evicted-pod> --grace-period=0 --force
```

## Performance Tuning

### GPU Memory Optimization

```yaml
# values.yaml optimizations for different scenarios

# High throughput (many concurrent requests)
vllm:
  args:
    - --max-num-seqs=256
    - --max-model-len=2048
    - --gpu-memory-utilization=0.95

# Low latency (priority on response time)
vllm:
  args:
    - --max-num-seqs=64
    - --max-model-len=4096
    - --gpu-memory-utilization=0.90
    - --enable-prefix-caching

# Memory constrained (smaller GPU)
vllm:
  args:
    - --max-num-seqs=32
    - --max-model-len=2048
    - --quantization=awq  # or gptq
    - --gpu-memory-utilization=0.85
```

### Latency Optimization

1. **Enable KV cache**: Already enabled by default in vLLM
2. **Use prefix caching**: `--enable-prefix-caching` for repeated system prompts
3. **Optimize batch size**: Balance between throughput and latency
4. **Reduce max_model_len**: Set to actual use case requirements
5. **Use tensor parallelism**: For multi-GPU setups: `--tensor-parallel-size=N`

## Monitoring Commands

```bash
# Watch pod status
watch kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment

# Monitor GPU usage
watch 'kubectl exec -n system <pod-name> -- nvidia-smi'

# Stream logs
kubectl logs -n system <pod-name> --follow | grep -v "GET /health"

# Check alerts
kubectl get prometheusrules -A | grep vllm
```

## Getting Help

1. **Check Grafana Dashboard**: `docs/dashboards/vllm-deployment-dashboard.json`
2. **Review Prometheus Alerts**: Configure alert notifications in Prometheus
3. **Examine Pod Events**: `kubectl describe pod <pod-name>`
4. **Collect Logs**: `kubectl logs -n system <pod-name> --previous` (for crashed pods)

## Related Documentation

- `docs/deployment-workflow.md` - Deployment procedures
- `docs/rollback-workflow.md` - Rollback procedures
- `specs/010-vllm-deployment/` - Feature specifications
- `infra/helm/charts/vllm-deployment/README.md` - Helm chart documentation
