# AIModel Validation Webhook

## Overview

The AIModel validation webhook prevents scheduling failures by validating that AIModel resources have the necessary tolerations to run on GPU nodes.

## Problem

GPU nodes are typically tainted to prevent non-GPU workloads from scheduling on them. If an AIModel is created without the required tolerations, it will fail to schedule, leading to:

- Pods stuck in Pending state
- Difficult-to-diagnose "no nodes available" errors
- Wasted time debugging scheduling issues

## Solution

The webhook validates AIModel resources at admission time (before they're created in etcd) and:

1. **Rejects** AIModels missing required tolerations
2. **Warns** when CPU requests exceed the smallest GPU node capacity

## How It Works

### Toleration Validation

The webhook:
1. Lists all GPU nodes (labeled with `nvidia.com/gpu.present=true`)
2. Extracts all taints from those nodes (excluding `PreferNoSchedule`)
3. Validates that the AIModel's `spec.tolerations` covers all required taints
4. Rejects the AIModel if any taint is not tolerated

### Example

**GPU Node Taint:**
```yaml
taints:
- key: gpu-workload
  value: "true"
  effect: NoSchedule
```

**Valid AIModel (will be accepted):**
```yaml
spec:
  tolerations:
  - key: gpu-workload
    operator: Equal
    value: "true"
    effect: NoSchedule
```

**Invalid AIModel (will be rejected):**
```yaml
spec:
  tolerations: []  # Missing required toleration
```

**Error Message:**
```
Error from server: AIModel is missing required tolerations for GPU nodes.
The following taints are not tolerated: gpu-workload=true:NoSchedule.
Add these tolerations to spec.tolerations to allow scheduling on GPU nodes
```

## Toleration Matching Logic

The webhook implements Kubernetes scheduler toleration matching logic:

- **Operator=Equal**: Key, value, and effect must match
- **Operator=Exists**: Key and effect must match (value ignored)
- **Empty effect**: Matches any effect for that key
- **Empty key + Exists**: Matches ALL taints (wildcard toleration)

## CPU Request Warnings

The webhook warns if the CPU request exceeds the smallest GPU node's allocatable CPU:

```
Warning: CPU request 20 exceeds smallest GPU node allocatable capacity 16 -
pod may not be schedulable on all GPU nodes
```

This is a warning, not an error - the AIModel is still created but operators are alerted to potential scheduling constraints.

## Configuration

### Enable/Disable Webhook

The webhook is enabled by default. To disable:

```yaml
# values.yaml
webhook:
  enabled: false
```

### Failure Policy

Controls what happens if the webhook is unreachable:

```yaml
# values.yaml
webhook:
  failurePolicy: Fail  # Reject requests if webhook is down (recommended)
  # OR
  failurePolicy: Ignore  # Allow requests if webhook is down (development)
```

## Deployment

### Prerequisites

1. **cert-manager** must be installed in the cluster:
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
   ```

2. **Node labels**: GPU nodes must be labeled with `nvidia.com/gpu.present=true`

### Install

The webhook is automatically deployed with the operator Helm chart:

```bash
helm install ai-model-operator ./deployments/helm/ai-model-operator
```

### Verify

```bash
# Check webhook configuration
kubectl get validatingwebhookconfigurations | grep ai-model-operator

# Check webhook service
kubectl get svc -n ai-model-system | grep webhook

# Check certificates
kubectl get certificate -n ai-model-system
kubectl get secret -n ai-model-system | grep webhook-server-cert
```

## Testing

### Unit Tests

```bash
cd operators/ai-model-operator
go test ./internal/webhook/... -v
```

### Integration Test (Manual)

1. Create a GPU node with a taint:
   ```bash
   kubectl taint nodes <node-name> gpu-workload=true:NoSchedule
   kubectl label nodes <node-name> nvidia.com/gpu.present=true
   ```

2. Attempt to create an AIModel without tolerations:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: aimodel.ai-aas.io/v1alpha1
   kind: AIModel
   metadata:
     name: test-invalid
   spec:
     modelName: llama-7b
     modelID: meta-llama/Llama-2-7b-hf
   EOF
   ```

   Expected: Rejection with clear error message

3. Create an AIModel with correct tolerations:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: aimodel.ai-aas.io/v1alpha1
   kind: AIModel
   metadata:
     name: test-valid
   spec:
     modelName: llama-7b
     modelID: meta-llama/Llama-2-7b-hf
     tolerations:
     - key: gpu-workload
       operator: Equal
       value: "true"
       effect: NoSchedule
   EOF
   ```

   Expected: Success

## Troubleshooting

### Webhook Not Working

Check operator logs:
```bash
kubectl logs -n ai-model-system deployment/ai-model-operator-controller-manager | grep webhook
```

Expected log:
```
AIModel validating webhook registered	{"port": 9443}
```

### Certificate Issues

If the webhook fails with TLS errors:

```bash
# Check certificate status
kubectl describe certificate -n ai-model-system

# Check secret exists
kubectl get secret ai-model-operator-webhook-server-cert -n ai-model-system

# Restart operator to reload certificates
kubectl rollout restart deployment/ai-model-operator-controller-manager -n ai-model-system
```

### Webhook Blocking Valid AIModels

If the webhook incorrectly rejects valid AIModels:

1. Check GPU node labels:
   ```bash
   kubectl get nodes -l nvidia.com/gpu.present=true
   ```

2. Check node taints:
   ```bash
   kubectl get nodes -o json | jq '.items[] | select(.metadata.labels["nvidia.com/gpu.present"] == "true") | .spec.taints'
   ```

3. Ensure AIModel tolerations match the taints exactly

### Bypass Webhook (Emergency)

To temporarily bypass validation:

```yaml
# Edit ValidatingWebhookConfiguration
kubectl edit validatingwebhookconfiguration ai-model-operator-validating-webhook-configuration

# Change failurePolicy to Ignore
spec:
  webhooks:
  - failurePolicy: Ignore  # Changed from Fail
```

**Warning**: This allows invalid AIModels to be created. Only use for emergency debugging.

## Architecture

```
┌─────────────────────────────────────────┐
│ kubectl apply -f aimodel.yaml           │
└─────────────┬───────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│ Kubernetes API Server                   │
│ ┌─────────────────────────────────────┐ │
│ │ ValidatingWebhookConfiguration      │ │
│ │ - Intercepts CREATE/UPDATE          │ │
│ │ - Calls webhook service             │ │
│ └──────────┬──────────────────────────┘ │
└────────────┼───────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│ ai-model-operator-webhook-service:443   │
│ (routes to port 9443)                   │
└─────────────┬───────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│ AIModel Operator Pod                    │
│ ┌─────────────────────────────────────┐ │
│ │ AIModelValidator.ValidateCreate()   │ │
│ │ 1. List GPU nodes                   │ │
│ │ 2. Extract taints                   │ │
│ │ 3. Check tolerations                │ │
│ │ 4. Return Admit/Deny                │ │
│ └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Future Enhancements

Potential improvements tracked in beads:

- Validate GPU resource requests match node GPU types
- Suggest recommended tolerations based on cluster configuration
- Validate storage requirements against available PV capacity
- Check model compatibility with runtime versions
