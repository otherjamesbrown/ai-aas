# Certificate Architecture

This document describes the certificate management architecture for the AI-AAS platform.

## Overview

The platform uses **two distinct types of certificates**:

1. **External TLS Certificates** - For public-facing HTTPS endpoints
2. **Internal Webhook Certificates** - For Kubernetes internal communication

These are managed separately and serve different purposes.

## External TLS Certificates (Let's Encrypt)

### Purpose

- Secure public-facing endpoints (API, Web Portal, ArgoCD, Grafana)
- Provide trusted HTTPS certificates for external users
- Enable secure communication over the internet

### Management

- **Issuer**: Let's Encrypt (via cert-manager ClusterIssuers)
- **ClusterIssuers**:
  - `letsencrypt-prod` - Production certificates (rate-limited)
  - `letsencrypt-staging` - Testing certificates (not trusted, but no rate limits)
- **Renewal**: Automatic (cert-manager handles renewal before expiry)

### Certificates

| Service | Domain | Certificate |
|---------|--------|-------------|
| API Router | `api.dev.otherjamesbrown.com` | `api-tls` |
| Web Portal | `portal.dev.otherjamesbrown.com` | `portal-tls` |
| Admin API | `admin-api.dev.otherjamesbrown.com` | `admin-api-tls` |
| Analytics | `analytics.dev.otherjamesbrown.com` | `analytics-tls` |
| Grafana | `grafana.dev.otherjamesbrown.com` | `grafana-tls` |
| ArgoCD | `argocd.dev.otherjamesbrown.com` | Managed by ArgoCD |

### Configuration

Certificates are requested via Ingress annotations:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - api.dev.otherjamesbrown.com
      secretName: api-tls
```

### Verification

```bash
# List all certificates
kubectl get certificates -A

# Check certificate status
kubectl describe certificate api-tls -n development

# Check certificate secret
kubectl get secret api-tls -n development -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

## Internal Webhook Certificates (Self-signed)

### Purpose

- Secure communication between Kubernetes API server and admission webhooks
- Required for CRD conversion webhooks (e.g., KServe InferenceService)
- Enable mutating and validating webhooks

### Management

- **Issuer**: Self-signed (per-namespace)
- **Issuers**:
  - `kserve/selfsigned-issuer` - For KServe webhooks
- **Renewal**: Automatic (cert-manager handles rotation)

### KServe Certificates

| Resource | Namespace | Purpose |
|----------|-----------|---------|
| `Issuer/selfsigned-issuer` | kserve | Self-signed certificate issuer |
| `Certificate/serving-cert` | kserve | Webhook TLS certificate |
| `Secret/kserve-webhook-server-cert` | kserve | Contains generated cert/key |

### CA Injection

cert-manager automatically injects the CA bundle into resources with this annotation:

```yaml
metadata:
  annotations:
    cert-manager.io/inject-ca-from: kserve/serving-cert
```

Resources that receive CA injection:
- `CustomResourceDefinition/inferenceservices.serving.kserve.io` (conversion webhook)
- `MutatingWebhookConfiguration/inferenceservice.serving.kserve.io`
- `ValidatingWebhookConfiguration/*` (5 configurations)

### Verification

```bash
# Check KServe certificate
kubectl get certificate serving-cert -n kserve

# Check issuer
kubectl get issuer selfsigned-issuer -n kserve

# Verify CA injection into CRD
kubectl get crd inferenceservices.serving.kserve.io \
  -o jsonpath='{.spec.conversion.webhook.clientConfig.caBundle}' | base64 -d | head -3

# Should show: -----BEGIN CERTIFICATE-----
```

## ArgoCD Sync Wave Ordering

KServe installation requires specific deployment ordering to ensure certificates exist before resources that depend on them.

### Sync Waves

| Wave | Resources | Why |
|------|-----------|-----|
| -2 | `Issuer/selfsigned-issuer` | Must exist before Certificate |
| -1 | `Certificate/serving-cert` | Must exist before CRDs/webhooks |
| 0 | Default resources | Namespace, RBAC, Deployment, Services |
| 1 | CRDs with conversion webhooks | Requires valid CA bundle |
| 2 | Webhook configurations | Requires valid CA bundle |

### Implementation

Sync waves are configured via annotations in `infra/k8s/kserve/install/kserve.yaml`:

```yaml
# Issuer - Wave -2 (first)
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "-2"

# Certificate - Wave -1 (second)
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "-1"

# CRD - Wave 1 (after defaults)
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "1"

# Webhooks - Wave 2 (last)
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "2"
```

## Troubleshooting

### External Certificate Issues

```bash
# Check ClusterIssuer status
kubectl get clusterissuer

# Check certificate status
kubectl describe certificate <name> -n <namespace>

# Check for ACME challenges
kubectl get challenges -A

# Check cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager
```

### Internal Certificate Issues

See [KServe Certificate Troubleshooting](../troubleshooting/kserve-certificate-issues.md)

## References

- [cert-manager Documentation](https://cert-manager.io/docs/)
- [Let's Encrypt](https://letsencrypt.org/)
- [KServe Webhook Configuration](https://kserve.github.io/website/)
- [Kubernetes Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
