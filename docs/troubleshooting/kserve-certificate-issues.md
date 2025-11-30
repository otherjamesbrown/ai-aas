# KServe Certificate Troubleshooting

This guide helps diagnose and resolve certificate-related issues with KServe webhooks.

## Symptoms

### ArgoCD Sync Failure

```
CustomResourceDefinition.apiextensions.k8s.io "inferenceservices.serving.kserve.io" is invalid:
spec.conversion.webhookClientConfig.caBundle: Invalid value: "Cg==":
unable to load root certificates: unable to parse bytes as PEM block
```

### Webhook Errors

```
failed calling webhook "inferenceservice.kserve-webhook-server.validator":
failed to call webhook: x509: certificate signed by unknown authority
```

### InferenceService Stuck

```
kubectl get inferenceservice -n development
NAME          URL   READY   AGE
my-model            False   10m
```

## Diagnostic Commands

### 1. Check Certificate Status

```bash
# List certificates in kserve namespace
kubectl get certificate -n kserve

# Expected output:
# NAME           READY   SECRET                       AGE
# serving-cert   True    kserve-webhook-server-cert   1d

# If READY is False, describe for details:
kubectl describe certificate serving-cert -n kserve
```

### 2. Check Issuer Status

```bash
# Check issuer
kubectl get issuer -n kserve

# Expected output:
# NAME                READY   AGE
# selfsigned-issuer   True    1d

# If not Ready:
kubectl describe issuer selfsigned-issuer -n kserve
```

### 3. Check CA Bundle Injection

```bash
# Check if CA was injected into CRD
kubectl get crd inferenceservices.serving.kserve.io \
  -o jsonpath='{.spec.conversion.webhook.clientConfig.caBundle}' | base64 -d | head -3

# Expected output (valid certificate):
# -----BEGIN CERTIFICATE-----
# MIID...
# ...

# If empty or shows just whitespace, CA injection failed
```

### 4. Check Webhook Secret

```bash
# Verify the webhook secret exists and has data
kubectl get secret kserve-webhook-server-cert -n kserve

# Check secret contents
kubectl get secret kserve-webhook-server-cert -n kserve -o yaml

# Should have: tls.crt, tls.key, ca.crt
```

### 5. Check ArgoCD Application Status

```bash
# Check kserve-development app
kubectl get application kserve-development -n argocd

# Get detailed status
kubectl describe application kserve-development -n argocd | grep -A 20 "Operation State:"

# Check sync result for specific resource failures
kubectl get application kserve-development -n argocd \
  -o jsonpath='{.status.operationState.syncResult.resources[?(@.status!="Synced")]}' | jq
```

## Resolution Steps

### Case 1: Certificate Missing

If `kubectl get certificate -n kserve` shows no `serving-cert`:

**Option A: Wait for ArgoCD sync**
```bash
# Trigger a fresh sync
kubectl annotate application kserve-development -n argocd \
  argocd.argoproj.io/refresh=hard --overwrite

# Wait for sync to complete
kubectl get application kserve-development -n argocd -w
```

**Option B: Manually create Certificate (bootstrap)**
```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: serving-cert
  namespace: kserve
spec:
  commonName: kserve-webhook-server-service.kserve.svc
  dnsNames:
  - kserve-webhook-server-service.kserve.svc
  issuerRef:
    kind: Issuer
    name: selfsigned-issuer
  secretName: kserve-webhook-server-cert
EOF

# Verify it becomes Ready
kubectl get certificate serving-cert -n kserve -w
```

### Case 2: Issuer Missing

If `kubectl get issuer -n kserve` shows no `selfsigned-issuer`:

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: kserve
spec:
  selfSigned: {}
EOF
```

### Case 3: CA Bundle Not Injected

If the CA bundle is empty in CRDs/webhooks:

1. **Verify Certificate is Ready**:
   ```bash
   kubectl get certificate serving-cert -n kserve
   # Must show READY: True
   ```

2. **Force CA injection by updating the resource**:
   ```bash
   # Touch the CRD to trigger cert-manager injection
   kubectl annotate crd inferenceservices.serving.kserve.io \
     cert-manager.io/inject-ca-from-secret=kserve/kserve-webhook-server-cert \
     --overwrite

   # Remove the annotation (cert-manager will re-inject from the annotation)
   kubectl annotate crd inferenceservices.serving.kserve.io \
     cert-manager.io/inject-ca-from-secret- --overwrite
   ```

3. **Restart cert-manager** (if injection still fails):
   ```bash
   kubectl rollout restart deployment cert-manager -n cert-manager
   ```

### Case 4: ArgoCD Sync Exhausted Retries

If ArgoCD shows "retried 5 times" and stopped:

```bash
# First fix the underlying issue (usually missing Certificate)
# Then trigger a new sync operation:

kubectl patch application kserve-development -n argocd --type merge -p '{
  "operation": {
    "initiatedBy": {"username": "admin"},
    "sync": {
      "revision": "HEAD",
      "syncOptions": ["CreateNamespace=true"]
    }
  }
}'
```

### Case 5: cert-manager Not Working

```bash
# Check cert-manager pods
kubectl get pods -n cert-manager

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager --tail=100

# Restart cert-manager if needed
kubectl rollout restart deployment -n cert-manager cert-manager
kubectl rollout restart deployment -n cert-manager cert-manager-webhook
kubectl rollout restart deployment -n cert-manager cert-manager-cainjector
```

## Prevention

### ArgoCD Sync Waves

The `kserve.yaml` manifest uses sync waves to ensure proper ordering:

```yaml
# Issuer (Wave -2) -> Certificate (Wave -1) -> CRDs (Wave 1) -> Webhooks (Wave 2)
```

This ensures certificates are created before resources that depend on them.

### CI/CD Checks

Consider adding these checks to your pipeline:

```bash
# Pre-deployment check: Verify cert-manager is healthy
kubectl wait --for=condition=Ready pods -n cert-manager -l app=cert-manager --timeout=60s

# Post-deployment check: Verify KServe certificate
kubectl wait --for=condition=Ready certificate/serving-cert -n kserve --timeout=120s
```

## Related Documentation

- [Certificate Architecture](../platform/certificate-architecture.md)
- [KServe Infrastructure README](../../infra/k8s/kserve/base/README.md)
- [KServe Migration Deployment Guide](../runbooks/kserve-migration-deployment.md)
