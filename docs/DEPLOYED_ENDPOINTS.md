# Deployed Endpoints Reference

**Last Updated**: 2025-11-20

This document provides quick reference for all deployed services and how to access them.

## Development Cluster (LKE 531921)

### Cluster Information
- **Name**: lke531921
- **Location**: Linode (fr-par-2)
- **Kubeconfig**: `~/kubeconfigs/kubeconfig-development.yaml`
- **API Server**: `https://ab5a4a7c-b8fd-4f8c-9479-7260d815f967.fr-par-2-gw.linodelke.net`

### Nodes
| Node | Type | GPU | Purpose |
|------|------|-----|---------|
| lke531921-770211-1b59efcf0000 | g6-standard-4 | ❌ | CPU workloads |
| lke531921-770211-3813f3520000 | g6-standard-4 | ❌ | CPU workloads |
| lke531921-770211-611c01ef0000 | g6-standard-4 | ❌ | CPU workloads |
| lke531921-776664-51386eeb0000 | GPU instance | ✅ 1x NVIDIA | vLLM inference |

### Deployed Services

#### vLLM Inference Service

**Service**: `vllm-gpt-oss-20b`
- **Namespace**: `system`
- **Type**: ClusterIP
- **Internal IP**: 10.128.254.198
- **Port**: 8000
- **Model**: unsloth/gpt-oss-20b (20B parameters)
- **Status**: ✅ Running (21+ hours uptime)
- **Pod**: vllm-gpt-oss-20b-7ccc4c947b-lg2h9

**How to Access**:

```bash
# Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Option 1: Port-forward (recommended for testing)
kubectl port-forward -n system svc/vllm-gpt-oss-20b 8000:8000

# Option 2: Direct pod access (debugging)
kubectl port-forward -n system pod/vllm-gpt-oss-20b-7ccc4c947b-lg2h9 8000:8000
```

**Test the Endpoint**:

```bash
# Health check
curl http://localhost:8000/health

# Models endpoint
curl http://localhost:8000/v1/models

# Chat completion (simple)
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "unsloth/gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'

# Chat completion (test question - capital of France)
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "unsloth/gpt-oss-20b",
    "messages": [{"role": "user", "content": "What is the capital of France? Answer in one word only."}],
    "max_tokens": 50,
    "temperature": 0.1
  }' | jq '.choices[0].message.content'

# Expected output: "Paris"
```

**OpenAI-Compatible API**:

This endpoint implements the OpenAI Chat Completions API format:
- `/v1/chat/completions` - Chat completions
- `/v1/completions` - Text completions
- `/v1/models` - List available models
- `/health` - Health check
- `/ready` - Readiness check

## Troubleshooting

### Cannot Connect to Cluster

```bash
# Verify kubeconfig
kubectl cluster-info

# Check nodes are accessible
kubectl get nodes

# Expected output: 4 nodes (3 CPU, 1 GPU)
```

### Port-Forward Fails

```bash
# Check if service exists
kubectl get svc -n system vllm-gpt-oss-20b

# Check if pod is running
kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment

# View pod logs
kubectl logs -n system vllm-gpt-oss-20b-7ccc4c947b-lg2h9
```

### Inference Request Fails

```bash
# Check pod health
kubectl describe pod -n system vllm-gpt-oss-20b-7ccc4c947b-lg2h9

# Check recent logs
kubectl logs -n system vllm-gpt-oss-20b-7ccc4c947b-lg2h9 --tail=50

# Verify GPU is allocated
kubectl get pod -n system vllm-gpt-oss-20b-7ccc4c947b-lg2h9 -o json | jq '.spec.containers[0].resources'
```

## Next Steps

### Pending Deployments

1. **API Router Service** - Public inference gateway
   - Needs deployment to dev cluster
   - Will provide authenticated access to vLLM
   - Status: Implementation complete, deployment pending

2. **Model Registry** - Service discovery
   - Register vLLM deployment
   - Enable dynamic routing
   - Status: Schema ready, needs registration

### Future Environments

- **Staging**: Not yet provisioned
- **Production**: Not yet provisioned

## See Also

- [vLLM Deployment Guide](./GPU_DEPLOYMENT_QUICKSTART.md)
- [Development Cluster Configuration](../specs/010-vllm-deployment/DEV_CLUSTER_NODES.md)
- [Testing Status](../specs/010-vllm-deployment/TESTING_STATUS.md)
- [Architecture Overview](./ARCHITECTURE.md)
