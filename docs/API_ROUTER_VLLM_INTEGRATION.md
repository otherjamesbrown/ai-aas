# API Router + vLLM Integration Status

**Last Updated**: 2025-11-20
**Status**: Partially Complete - Authentication Pending

## Overview

This document tracks the integration of the API Router Service with the vLLM deployment to add authentication and routing capabilities to the inference endpoint.

## Current Architecture

```
Internet
   ↓
Ingress (172.232.58.222 - vllm.dev.ai-aas.local)
   ↓
API Router Service (development namespace)
   ↓ (validates API key via user-org-service)
user-org-service (user-org-service namespace)
   ↓ (routes to backend)
vLLM Service (system namespace)
```

## Deployment Status

### ✅ Completed Components

1. **vLLM Deployment**
   - Namespace: `system`
   - Service: `vllm-gpt-oss-20b`
   - Model: `unsloth/gpt-oss-20b` (20B parameters)
   - Endpoint: `http://vllm-gpt-oss-20b.system.svc.cluster.local:8000`
   - Status: Running (1/1 replicas)
   - GPU Node: lke531921-776664-51386eeb0000

2. **API Router Service**
   - Namespace: `development`
   - Image: `ghcr.io/otherjamesbrown/api-router-service:dev`
   - Status: Running (2/2 replicas)
   - Service Port: 8080
   - Backend Configuration: `vllm-gpt-oss-20b:http://vllm-gpt-oss-20b.system.svc.cluster.local:8000/v1/chat/completions`
   - Environment:
     - `USER_ORG_SERVICE_URL`: `http://user-org-service-development-user-org-service.user-org-service.svc.cluster.local:8081`
     - Redis: In-memory stub
     - Kafka: In-memory stub
   - ArgoCD Application: `api-router-service-development`

3. **Ingress Configuration**
   - Name: `vllm-ingress`
   - Namespace: `development`
   - Host: `vllm.dev.ai-aas.local`
   - External IP: `172.232.58.222`
   - Backend: `api-router-service-development-api-router-service:8080`
   - Routes all traffic through API Router (authentication required)

4. **user-org-service**
   - Namespace: `user-org-service`
   - Status: Running (2/2 replicas)
   - Service Port: 8081
   - ArgoCD Application: `user-org-service-development`
   - Branch: `005-user-org-service-upgrade` (needs update)

### ⚠️ Pending Issues

1. **API Key Validation Endpoint Missing**
   - **Issue**: Deployed user-org-service is missing `/v1/auth/validate-api-key` endpoint
   - **Root Cause**: Deployed from old branch (`005-user-org-service-upgrade`)
   - **Code Status**: Endpoint exists in main branch at `services/user-org-service/internal/httpapi/auth/validate_api_key.go`
   - **Impact**: API Router rejects requests without keys but cannot complete full validation
   - **Fix Required**: Deploy updated user-org-service from main branch

2. **user-org-service Build Issues**
   - go.mod has incorrect import path: `github.com/ai-aas/shared-go/logging` (should be internal)
   - Missing go.sum entries for some dependencies
   - Migration init container expects `goose` binary

## Test Data Created

### Organization
- **ID**: `0c432daf-a3a9-480e-8b7a-33840168b027`
- **Name**: Test Organization
- **Slug**: test-org-vllm

### API Key
- **Secret**: `<YOUR_TEST_API_KEY>`
- **Fingerprint**: `<YOUR_API_KEY_FINGERPRINT>`
- **Organization ID**: `0c432daf-a3a9-480e-8b7a-33840168b027`
- **Scopes**: `["inference:read", "inference:write"]`
- **Status**: Active
- **Storage**: Inserted directly into database via Kubernetes Job

## Testing

### Current Behavior

**Without API Key**:
```bash
curl -X POST http://172.232.58.222/v1/chat/completions \
  -H 'Host: vllm.dev.ai-aas.local' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "unsloth/gpt-oss-20b",
    "messages": [{"role": "user", "content": "What is the capital of France?"}]
  }'

# Response: {"error":"authentication required","code":"AUTH_INVALID"}
```

**With API Key** (once endpoint is deployed):
```bash
curl -X POST http://172.232.58.222/v1/chat/completions \
  -H 'Host: vllm.dev.ai-aas.local' \
  -H 'X-API-Key: <YOUR_TEST_API_KEY>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "unsloth/gpt-oss-20b",
    "messages": [{"role": "user", "content": "What is the capital of France?"}],
    "max_tokens": 50,
    "temperature": 0.1
  }'

# Expected: Successful inference response
```

### Direct vLLM Access (for debugging)

```bash
# Port-forward to vLLM service
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl port-forward -n system svc/vllm-gpt-oss-20b 8000:8000

# Test directly (no auth)
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "unsloth/gpt-oss-20b",
    "messages": [{"role": "user", "content": "What is the capital of France?"}],
    "max_tokens": 50
  }'
```

## Next Steps

### Immediate (Required for Authentication)

1. **Fix user-org-service imports**
   - Update `services/user-org-service/internal/logging/logger.go` to use correct import path
   - Run `go mod tidy` to update dependencies
   - Verify code compiles

2. **Build and Deploy user-org-service**
   - Build Docker image from main branch
   - Push to `ghcr.io/otherjamesbrown/user-org-service:latest`
   - Update ArgoCD application to use main branch
   - Verify `/v1/auth/validate-api-key` endpoint is available

3. **Test End-to-End Authentication**
   - Verify API key validation works
   - Test inference with valid API key
   - Test rejection of invalid API keys
   - Document successful test results

### Future Enhancements

1. **Production Configuration**
   - Deploy Redis for production rate limiting
   - Deploy Kafka for usage tracking
   - Configure real budget service
   - Add model registration to model registry

2. **Monitoring**
   - Add Prometheus metrics
   - Set up Grafana dashboards
   - Configure alerting

3. **Additional Models**
   - Deploy additional vLLM instances for different models
   - Configure routing rules in API Router
   - Test model selection and failover

## Files Modified

### API Router Service
- `services/api-router-service/cmd/router/main.go` - Fixed Chi router middleware ordering bug
- `services/api-router-service/go.mod`, `go.sum` - Added missing `lib/pq` dependency
- `services/api-router-service/deployments/helm/api-router-service/values-development.yaml` - Created development values

### Ingress
- `/tmp/vllm-ingress-with-auth.yaml` - New ingress routing through API Router

### ArgoCD Applications
- `api-router-service-development` - Updated with vLLM backend configuration

### Documentation
- `specs/010-vllm-deployment/TESTING_STATUS.md` - Updated with API Router integration status
- `docs/API_ROUTER_VLLM_INTEGRATION.md` - This file

## Key Learnings

1. **Chi Router Middleware Ordering**: All routes must be registered on a router BEFORE calling `Mount()`
2. **Cross-Namespace Services**: Kubernetes services can reference other services across namespaces using FQDN: `service-name.namespace.svc.cluster.local`
3. **ArgoCD Branch Management**: Ensure ArgoCD applications track the correct branch with latest code
4. **API Key Storage**: API keys are stored as SHA-256 fingerprints, not plaintext
5. **Init Container Dependencies**: Migration init containers need proper tooling (goose) installed

## References

- vLLM Deployment Spec: `/home/dev/ai-aas/specs/010-vllm-deployment/`
- API Router Service: `/home/dev/ai-aas/services/api-router-service/`
- user-org-service: `/home/dev/ai-aas/services/user-org-service/`
- Testing Status: `/home/dev/ai-aas/specs/010-vllm-deployment/TESTING_STATUS.md`