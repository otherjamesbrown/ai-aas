# KServe Infrastructure

This directory contains KServe infrastructure configuration for the AI-AAS platform.

## Contents

- `cluster-serving-runtime-vllm.yaml` - vLLM ClusterServingRuntime for model inference
- `cluster-storage-container-hf.yaml` - Hugging Face model storage backend
- `cluster-storage-container-s3.yaml` - S3-compatible storage backend
- `secret-templates.yaml` - Templates for storage credentials
- `gpu-node-setup.md` - GPU node configuration guide
- `podmonitor-*.yaml` - Prometheus PodMonitors for metrics collection

## Architecture

### Certificate Management

KServe uses internal webhook certificates for CRD validation and conversion. These are **separate from external TLS certificates** (Let's Encrypt).

**Internal Certificates (Self-signed):**
- Managed by cert-manager with a self-signed Issuer in the `kserve` namespace
- Used for webhook communication between Kubernetes API server and KServe controller
- Automatically rotated by cert-manager

**Certificate Resources:**
- `Issuer/selfsigned-issuer` - Self-signed certificate issuer
- `Certificate/serving-cert` - Generates the webhook TLS certificate
- Secret `kserve-webhook-server-cert` - Contains the generated certificate

**CA Injection:**
CRDs and webhook configurations have the annotation `cert-manager.io/inject-ca-from: kserve/serving-cert` which tells cert-manager to inject the CA bundle automatically.

### Deployment Order (Sync Waves)

ArgoCD sync waves ensure proper deployment order:

| Wave | Resources | Purpose |
|------|-----------|---------|
| -2 | Issuer | Create certificate issuer first |
| -1 | Certificate | Generate webhook certificate |
| 0 | Default resources | Namespace, RBAC, Deployment, Services |
| 1 | CRDs with conversion webhooks | Requires valid CA bundle |
| 2 | Webhook configurations | Requires valid CA bundle |

## Deployment

These resources are deployed via ArgoCD using the `kserve-config-development` application.

```bash
# Apply manually (not recommended, use GitOps instead)
kubectl apply -f cluster-serving-runtime-vllm.yaml
kubectl apply -f cluster-storage-container-hf.yaml
kubectl apply -f cluster-storage-container-s3.yaml
```

## Creating Secrets

Before deploying InferenceServices, create the required secrets:

```bash
# Hugging Face token (for private models)
kubectl create secret generic huggingface-secret \
  --from-literal=token=$HF_TOKEN \
  -n development

# S3 credentials (if using private S3 storage)
kubectl create secret generic s3-credentials \
  --from-literal=access_key=$AWS_ACCESS_KEY_ID \
  --from-literal=secret_key=$AWS_SECRET_ACCESS_KEY \
  -n development
```

## Testing the Infrastructure

Deploy a test model using the CLI:

```bash
# Register a model
ai-aas-cli model registry add mistralai/Mistral-7B-Instruct-v0.3 \
  --name mistral-7b-instruct

# Deploy to development
ai-aas-cli model deploy create mistral-7b-instruct -e development

# Check status
kubectl get inferenceservice -n development
```

## Troubleshooting

### Certificate Issues

If you see errors like `unable to load root certificates` or `caBundle invalid`:

```bash
# Check if the Certificate exists and is Ready
kubectl get certificate serving-cert -n kserve

# Check if the Issuer is Ready
kubectl get issuer selfsigned-issuer -n kserve

# Check if CA was injected into CRDs
kubectl get crd inferenceservices.serving.kserve.io \
  -o jsonpath='{.spec.conversion.webhook.clientConfig.caBundle}' | base64 -d | head -3

# If Certificate is missing, check ArgoCD sync status
kubectl get application kserve-development -n argocd
```

See [Certificate Troubleshooting Guide](../../../docs/troubleshooting/kserve-certificate-issues.md) for more details.

### InferenceService not becoming Ready

```bash
# Check InferenceService status
kubectl describe inferenceservice <name> -n development

# Check controller logs
kubectl logs -n kserve -l control-plane=kserve-controller-manager

# Check predictor pod logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=<name> -c kserve-container
```

### Storage initializer failures

```bash
# Check storage initializer logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=<name> -c storage-initializer
```

## References

- [KServe Documentation](https://kserve.github.io/website/)
- [Knative Serving](https://knative.dev/docs/serving/)
- [Istio Documentation](https://istio.io/latest/docs/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
