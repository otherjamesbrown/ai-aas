# Ingress Configuration Best Practices

This document outlines best practices for configuring Kubernetes Ingress resources in the AI-AAS platform.

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
- Hostname-based access: `api.dev.ai-aas.local` (requires `/etc/hosts` entry)
- IP-based access: `api.<LOAD_BALANCER_IP>.nip.io` (works without DNS/hosts configuration)
- Force SSL redirect enabled

**Example Configuration:**

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
  hosts:
    - host: api.dev.ai-aas.local
      paths:
        - path: /
          pathType: Prefix
    # IP-based access using nip.io
    - host: api.172.232.58.222.nip.io
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: ai-aas-tls
      hosts:
        - api.dev.ai-aas.local
        - api.172.232.58.222.nip.io
```

**Why nip.io?**
- nip.io is a wildcard DNS service that maps `<anything>.<IP>.nip.io` to `<IP>`
- Eliminates need to configure `/etc/hosts` or DNS for development
- Works immediately after deployment with LoadBalancer IP
- Supports HTTPS with wildcard certificates

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
- Cover all development hostnames (`api.dev.ai-aas.local`, `api.prod.ai-aas.local`)
- Include wildcard for nip.io if possible, or specific IP-based hostnames
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
curl -k https://api.dev.ai-aas.local/v1/status/healthz \
  -H "X-API-Key: YOUR_API_KEY"
```

### Test IP-Based Access

Using nip.io (works without `/etc/hosts`):

```bash
LOAD_BALANCER_IP=$(kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

curl -k https://api.$LOAD_BALANCER_IP.nip.io/v1/status/healthz \
  -H "X-API-Key: YOUR_API_KEY"
```

### Verify TLS Certificate

```bash
openssl s_client -connect api.172.232.58.222.nip.io:443 -servername api.172.232.58.222.nip.io
```

## Troubleshooting

### 404 Not Found

**Symptom:** NGINX returns 404 for all requests

**Causes:**
1. Hostname mismatch - ingress configured for `api.dev.ai-aas.local` but accessing via IP
2. Missing ingress rule for the requested host
3. Path not matching ingress path configuration

**Solutions:**
- Add nip.io hostname to ingress hosts
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
- Hostname: `https://api.dev.ai-aas.local` (requires `/etc/hosts` entry)
- IP-based: `https://api.172.232.58.222.nip.io` (works without DNS)

**Configuration Location:** `services/api-router-service/deployments/helm/api-router-service/values-development.yaml`

**Testing:**
```bash
curl -k -H "X-API-Key: YOUR_KEY" https://api.172.232.58.222.nip.io/v1/status/healthz
```

### User-Org Service

The User-Org Service manages user and organization data. This service is accessed by admin-cli and other internal services.

**Access URLs:**
- Hostname: `https://user-org.dev.ai-aas.local` (requires `/etc/hosts` entry)
- IP-based: `https://user-org.172.232.58.222.nip.io` (works without DNS)

**Configuration Location:** `infra/k8s/development/ingress/user-org-service-ingress.yaml`

**Testing:**
```bash
curl -k https://user-org.172.232.58.222.nip.io/healthz
```

**Admin CLI Integration:**
The admin-cli defaults to using the nip.io URL for remote access without requiring kubectl. This is configured in `services/admin-cli/internal/config/defaults.go`:

```go
v.SetDefault("api-endpoints.user-org-service", "https://user-org.172.232.58.222.nip.io")
```

### etcd (Config Service)

etcd stores configuration and routing policies. It's accessed by admin-cli for configuration management.

**Access URLs:**
- HTTP Gateway: `https://etcd.172.232.58.222.nip.io` (HTTPS, for HTTP API testing)
- gRPC API: `localhost:2379` (requires kubectl port-forward)

**Configuration Location:** `infra/k8s/development/ingress/etcd-ingress.yaml`

**Testing HTTP Gateway:**
```bash
curl -k https://etcd.172.232.58.222.nip.io/version
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
- [ ] Add nip.io hostname for development
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
