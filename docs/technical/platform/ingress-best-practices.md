# Ingress Configuration Best Practices

This document outlines best practices for configuring Kubernetes Ingress resources in the AI-AAS platform.

## Architecture Overview

The platform uses a **dual-ingress architecture**:

| Component | Purpose | LoadBalancer IP (Dev) | LoadBalancer IP (Staging) |
|-----------|---------|----------------------|---------------------------|
| **NGINX Ingress** | Primary - All external HTTP/HTTPS traffic | 172.232.58.222 | 172.236.135.55 |
| **Istio Gateway** | Internal only - KServe/Knative routing | 172.232.48.93 | 172.236.132.56 |

### Why Dual Ingress?

1. **NGINX for External Traffic**: Battle-tested, simple configuration, excellent for standard HTTP/HTTPS routing
2. **Istio for KServe/Knative**: Required for ML model serving - KServe uses Knative which requires Istio for internal service mesh routing

### Key Rules

- **All service ingresses MUST use `className: nginx`**
- **Never route external traffic through Istio** - it's only for internal KServe/Knative communication
- **Istio Gateway is used by `knative-local-gateway`** for internal cluster routing between Knative components

### GitOps Management

Both ingress controllers are managed via ArgoCD:
- NGINX: `gitops/clusters/<env>/apps/nginx-ingress.yaml`
- Istio: `gitops/clusters/<env>/apps/istio.yaml`

## Overview

The platform uses NGINX Ingress Controller for routing external traffic to services. Proper ingress configuration ensures:
- Secure HTTPS access
- IP-based and hostname-based routing
- Consistent configuration across environments
- Easy development and testing

## Environment-Specific Patterns

### Development Environment

Development environments should support both hostname-based and IP-based access for flexibility during testing.

**Key Characteristics:**
- HTTPS with self-signed certificates (using `ai-aas-tls` secret)
- Hostname-based access: `api.dev.otherjamesbrown.com` (requires `/etc/hosts` entry)
- DNS-based access: `api.dev.otherjamesbrown.com`
- Force SSL redirect enabled

**Example Configuration:**

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
  hosts:
    - host: api.dev.otherjamesbrown.com
      paths:
        - path: /
          pathType: Prefix
    - host: api.dev.otherjamesbrown.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: api-tls
      hosts:
        - api.dev.otherjamesbrown.com
```

### Staging Environment

Staging should mirror production but use separate DNS and certificates.

**Key Characteristics:**
- Proper DNS (e.g., `router.api.ai-aas.dev`)
- Let's Encrypt staging certificates
- Same security settings as production
- Rate limiting for testing load scenarios

**Example Configuration:**

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-staging
  hosts:
    - host: router.api.ai-aas.dev
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: api-router-service-tls
      hosts:
        - router.api.ai-aas.dev
```

### Production Environment

Production uses fully qualified domain names with production certificates.

**Key Characteristics:**
- Production DNS (e.g., `router.api.ai-aas.prod`)
- Let's Encrypt production certificates
- Rate limiting and security headers
- No IP-based access (security best practice)

**Example Configuration:**

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "1000"
    nginx.ingress.kubernetes.io/rate-limit-window: "1m"
  hosts:
    - host: router.api.ai-aas.prod
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: api-router-service-tls
      hosts:
        - router.api.ai-aas.prod
```

## TLS Configuration

### Development

Development uses a self-signed certificate stored in the `ai-aas-tls` secret. This certificate should:
- Cover all development hostnames (`api.dev.otherjamesbrown.com`, `api.prod.otherjamesbrown.com`)
- Use cert-manager for automatic certificate management
- Be committed to git-crypt for team sharing

### Staging and Production

Use cert-manager with Let's Encrypt for automated certificate management:

1. Install cert-manager in the cluster
2. Create ClusterIssuers for staging and production
3. Annotate ingresses with `cert-manager.io/cluster-issuer`
4. Certificates are automatically provisioned and renewed

## Common Annotations

### Force HTTPS Redirect

Always redirect HTTP to HTTPS:

```yaml
annotations:
  nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
```

### Proxy Timeouts

For long-running requests (streaming, large uploads):

```yaml
annotations:
  nginx.ingress.kubernetes.io/proxy-read-timeout: "300"
  nginx.ingress.kubernetes.io/proxy-send-timeout: "300"
```

### Body Size Limits

For file uploads:

```yaml
annotations:
  nginx.ingress.kubernetes.io/proxy-body-size: "50m"
```

### Rate Limiting

For production APIs:

```yaml
annotations:
  nginx.ingress.kubernetes.io/rate-limit: "1000"
  nginx.ingress.kubernetes.io/rate-limit-window: "1m"
```

## Testing Ingress Configuration

### Test Hostname-Based Access

With `/etc/hosts` configured:

```bash
curl -k https://api.dev.otherjamesbrown.com/v1/status/healthz \
  -H "X-API-Key: YOUR_API_KEY"
```

### Verify TLS Certificate

```bash
openssl s_client -connect api.dev.otherjamesbrown.com:443 -servername api.dev.otherjamesbrown.com
```

## Troubleshooting

### 404 Not Found

**Symptom:** NGINX returns 404 for all requests

**Causes:**
1. Hostname mismatch - ingress configured for `api.dev.otherjamesbrown.com` but accessing via IP
2. Missing ingress rule for the requested host
3. Path not matching ingress path configuration

**Solutions:**
- Use correct hostname in requests
- Use correct hostname in requests
- Verify ingress rules: `kubectl get ingress -n <namespace> -o yaml`

### 308 Permanent Redirect (Redirect Loop)

**Symptom:** HTTP requests get 308 redirect but still don't work

**Causes:**
1. Accessing HTTP when `force-ssl-redirect` is enabled
2. Not following HTTPS redirect

**Solutions:**
- Use HTTPS directly: `https://` instead of `http://`
- Use `-L` flag with curl to follow redirects

### Certificate Errors

**Symptom:** TLS certificate warnings in browser/curl

**Causes:**
1. Self-signed certificate not trusted
2. Certificate doesn't include hostname
3. Certificate expired

**Solutions:**
- For development, use `-k` flag with curl or add certificate to trust store
- Verify certificate includes all hostnames in SAN
- Check certificate expiry: `openssl x509 -in cert.pem -noout -dates`

## Service-Specific Ingress Configurations

### API Router Service

The API Router is the central gateway for all API traffic.

**Access URLs:**
- Hostname: `https://api.dev.otherjamesbrown.com` (requires `/etc/hosts` entry)
- IP-based: `https://api.dev.otherjamesbrown.com` (works without DNS)

**Configuration Location:** `services/api-router-service/deployments/helm/api-router-service/values-development.yaml`

**Testing:**
```bash
curl -k -H "X-API-Key: YOUR_KEY" https://api.dev.otherjamesbrown.com/v1/status/healthz
```

### User-Org Service

The User-Org Service manages user and organization data. This service is accessed by admin-cli and other internal services.

**Access URLs:**
- Hostname: `https://user-org.dev.otherjamesbrown.com` (requires `/etc/hosts` entry)
- IP-based: `https://user-org.dev.otherjamesbrown.com` (works without DNS)

**Configuration Location:** `infra/k8s/development/ingress/user-org-service-ingress.yaml`

**Testing:**
```bash
curl -k https://user-org.dev.otherjamesbrown.com/healthz
```

**Admin CLI Integration:**
The admin-cli defaults to using the public URL. This is configured in `services/admin-cli/internal/config/defaults.go`:

```go
v.SetDefault("api-endpoints.user-org-service", "https://user-org.dev.otherjamesbrown.com")
```

### etcd (Config Service)

etcd stores configuration and routing policies. It's accessed by admin-cli for configuration management.

**Access URLs:**
- HTTP Gateway: `https://etcd.dev.otherjamesbrown.com` (HTTPS, for HTTP API testing)
- gRPC API: `localhost:2379` (requires kubectl port-forward)

**Configuration Location:** `infra/k8s/development/ingress/etcd-ingress.yaml`

**Testing HTTP Gateway:**
```bash
curl -k https://etcd.dev.otherjamesbrown.com/version
```

**Admin CLI Integration:**
The admin-cli uses etcd's gRPC API, which requires port-forward:

```bash
# Required for admin-cli
kubectl port-forward -n development svc/etcd-service 2379:2379
```

The admin-cli defaults to localhost:

```go
v.SetDefault("api-endpoints.config-service", "localhost:2379")
```

**Important Notes:**
- etcd ingress exposes the HTTP gateway for monitoring/testing
- The admin-cli uses etcd's gRPC protocol, which requires direct access via port-forward
- gRPC doesn't work well through standard NGINX ingress without additional configuration
- The ingress uses `backend-protocol: HTTP` annotation because it proxies to etcd's HTTP gateway on port 2379

## Deployment Checklist

When deploying a new service with ingress:

- [ ] Configure hostname-based access for the environment

- [ ] Configure TLS certificate (self-signed for dev, cert-manager for staging/prod)
- [ ] Enable HTTPS redirect
- [ ] Set appropriate timeouts for your use case
- [ ] Configure rate limiting for production
- [ ] Test both hostname and IP-based access
- [ ] Verify TLS certificate is valid
- [ ] Update `/etc/hosts` for local development (if using hostname-based access)
- [ ] Document the access URLs in service README
- [ ] Update admin-cli or client defaults if the service is accessed by CLI tools
- [ ] Create permanent ingress manifest in `infra/k8s/<environment>/ingress/` directory

## Related Documentation

- [API Router Deployment](./deployment-workflow.md)
- [Certificate Management](./certificates.md) (TODO)
- [NGINX Ingress Controller Documentation](https://kubernetes.github.io/ingress-nginx/)
- [Admin CLI Configuration](../services/admin-cli/README.md)
